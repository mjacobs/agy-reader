#!/usr/bin/env python3
"""Run an interactive agy and an authenticated reader watcher together."""

import argparse
import http.client
import math
import os
from pathlib import Path
import re
import secrets
import shlex
import shutil
import signal
import subprocess
import sys
import tempfile
import termios
import time


class Stopped(Exception):
    def __init__(self, signum):
        self.signum = signum


def positive_seconds(value):
    seconds = float(value)
    if not math.isfinite(seconds) or seconds <= 0:
        raise argparse.ArgumentTypeError("must be a finite positive number")
    return seconds


def arguments():
    parser = argparse.ArgumentParser(
        description=__doc__,
        epilog="Single-user testing only: agy's launch token is visible in process "
        "arguments. Put agy arguments after --. Exiting agy stops the watcher.",
    )
    modes = parser.add_subparsers(dest="mode", required=True)
    run_parser = modes.add_parser("run", help="run the pair in the foreground")
    start_parser = modes.add_parser("start", help="start a detached tmux pair")
    start_parser.add_argument("--session", default="test", help="pair name (default: test)")
    for mode in ("attach", "status", "stop", "restart"):
        control = modes.add_parser(mode, help=f"{mode} a tmux pair")
        control.add_argument("--session", default="test", help="pair name (default: test)")
        if mode in ("stop", "restart"):
            control.add_argument("--timeout", type=positive_seconds, default=15,
                                 help="seconds to wait for shutdown (default: 15)")
    for launcher in (run_parser, start_parser):
        launch_arguments(launcher)
    args = parser.parse_args()
    if args.mode != "run":
        if not re.fullmatch(r"[A-Za-z0-9_-]+", args.session):
            parser.error("session names may contain only letters, digits, underscores, and hyphens")
        if not shutil.which("tmux"):
            parser.error("tmux is required for detached pairs")
    if args.mode not in ("run", "start"):
        return args
    if args.agy_args[:1] == ["--"]:
        args.agy_args.pop(0)
    # These options belong to the pair. Never allow a second token or a log
    # redirect to silently defeat authentication or endpoint selection.
    for arg in args.agy_args:
        if arg.split("=", 1)[0] in {"--csrf_token", "-csrf_token", "--log-file", "-log-file"}:
            parser.error("the helper manages agy's CSRF token and log file")
    for name in ("agy", "reader"):
        executable = shutil.which(getattr(args, name))
        if not executable:
            parser.error(f"{name} executable not found; use --{name}" +
                         (" or run make build" if name == "reader" else ""))
        setattr(args, name, str(Path(executable).resolve()))
    args.root = str(Path(args.root).expanduser().resolve())
    return args


def launch_arguments(parser):
    parser.add_argument("--agy", default="agy", help="agy executable (default: PATH)")
    parser.add_argument(
        "--reader", default=str(Path(__file__).resolve().parents[1] / "bin/agy-reader"),
        help="reader executable (default: this checkout's bin/agy-reader; run make build)",
    )
    parser.add_argument(
        "--root", default=str(Path.home() / ".gemini/antigravity-cli"),
        help="store to watch; does not relocate agy's store (default: ~/.gemini/antigravity-cli)",
    )
    parser.add_argument("--watch-interval", default="1s", help="reader polling interval (default: 1s)")
    parser.add_argument("--startup-timeout", type=positive_seconds, default=30,
                        help="seconds to wait for authenticated HTTP readiness (default: 30)")
    parser.add_argument("--stop-timeout", type=positive_seconds, default=5,
                        help="seconds to allow graceful shutdown before SIGKILL (default: 5)")
    parser.add_argument("--log-dir", type=Path,
                        help="parent directory for private per-run logs (default: system temporary directory)")
    parser.add_argument("agy_args", nargs=argparse.REMAINDER, help="arguments following -- go to agy")


def tmux(*args, check=True):
    result = subprocess.run(
        ["tmux", "-L", "agy-reader-pairs", "-f", "/dev/null", *args],
        text=True, capture_output=True,
    )
    if check and result.returncode:
        raise RuntimeError(result.stderr.strip() or "tmux command failed")
    return result


def pane_status(target):
    status = tmux("display-message", "-p", "-t", target,
                  "#{pane_id}\t#{pane_dead}\t#{pane_pid}\t#{pane_dead_status}").stdout.strip().split("\t")
    if len(status) < 3 or not status[2].isdigit():
        raise RuntimeError("the pair's original tmux pane is missing")
    return status


def detached(args):
    if args.mode == "start":
        command = [sys.executable, str(Path(__file__).resolve()), "run",
                   "--agy", args.agy, "--reader", args.reader, "--root", args.root,
                   f"--watch-interval={args.watch_interval}",
                   f"--startup-timeout={args.startup_timeout}", f"--stop-timeout={args.stop_timeout}"]
        if args.log_dir:
            command += ["--log-dir", str(args.log_dir.expanduser().resolve())]
        command += ["--", *args.agy_args]
        # Hold the empty pane until remain-on-exit is set, including for fast
        # startup failures. The real command stays available for restart.
        target = tmux("new-session", "-d", "-P", "-F", "#{session_id}",
                      "-s", args.session, "-c", os.getcwd(), "sleep 86400").stdout.strip()
        try:
            tmux("set-option", "-t", target, "@agy_pair", "1")
            pane = pane_status(target)[0]
            tmux("set-option", "-t", target, "@agy_pair_pane", pane)
            tmux("set-option", "-w", "-t", target, "remain-on-exit", "on")
            tmux("respawn-pane", "-k", "-t", pane, "exec " + shlex.join(command))
        except BaseException:
            tmux("kill-session", "-t", target, check=False)
            raise
        print(f"agy-pair: starting {args.session}; attach with: {Path(__file__).name} attach --session {args.session}")
        return 0
    # Resolve the exact name to an ID; tmux's usual name prefix matching must
    # never let 'stop test' terminate a differently named pair like 'test2'.
    sessions = tmux("list-sessions", "-F", "#{session_name}\t#{session_id}", check=False)
    target = next((line.split("\t")[1] for line in sessions.stdout.splitlines()
                   if line.split("\t")[0] == args.session), None)
    if target is None:
        raise RuntimeError(f"no pair named {args.session}")
    if tmux("show-options", "-t", target, "-v", "@agy_pair").stdout.strip() != "1":
        raise RuntimeError("session is not managed by agy-pair")
    if args.mode == "attach":
        return subprocess.call(["tmux", "-L", "agy-reader-pairs", "attach-session", "-t", target])
    pane = tmux("show-options", "-t", target, "-v", "@agy_pair_pane").stdout.strip()
    if not re.fullmatch(r"%\d+", pane):
        raise RuntimeError("session is missing its managed pane ID")
    status = pane_status(pane)
    pane, dead = status[:2]
    if args.mode == "status":
        state = f"exited (status {status[3] if len(status) > 3 else 'unknown'})" if dead == "1" else "running"
        print(f"agy-pair: {args.session}: {state}")
        return 1 if dead == "1" else 0
    if dead != "1":
        # A TUI may treat Ctrl-C as cancel-turn, not exit. The pane execs the
        # supervisor directly, so signal that owned process to stop the pair.
        try:
            os.kill(int(status[2]), signal.SIGTERM)
        except ProcessLookupError:
            pass
        deadline = time.monotonic() + args.timeout
        while pane_status(pane)[1] != "1":
            if time.monotonic() >= deadline:
                raise RuntimeError("pair has not stopped; session retained for inspection; retry with --timeout")
            time.sleep(0.05)
    if args.mode == "restart":
        # No -k: the supervisor must have completed its child cleanup first.
        tmux("respawn-pane", "-t", pane)
        print(f"agy-pair: restarting {args.session}")
    else:
        tmux("kill-session", "-t", target)
        print(f"agy-pair: stopped {args.session}")
    return 0


def daemon_port(log_path):
    try:
        log = log_path.read_text(errors="replace")
    except FileNotFoundError:
        return None
    ports = re.findall(r"listening on random port at (\d+) for HTTP\b", log)
    if ports and 0 < int(ports[-1]) < 65536:
        return int(ports[-1])
    return None


def authenticated(port, token, timeout):
    # Direct loopback connection: proxy environment variables must not receive
    # the token. Do not follow redirects or print response bodies.
    connection = http.client.HTTPConnection("127.0.0.1", port, timeout=timeout)
    try:
        connection.request(
            "POST", "/exa.language_server_pb.LanguageServerService/GetAllCascadeTrajectories",
            body=b"{}", headers={"Content-Type": "application/json", "x-codeium-csrf-token": token},
        )
        response = connection.getresponse()
        if response.status in (401, 403):
            raise RuntimeError("agy rejected the pair's CSRF token")
        return response.status == 200
    except (OSError, http.client.HTTPException):
        return False
    finally:
        connection.close()


def exit_code(code):
    return code if code >= 0 else 128 - code


def stop_children(children, timeout):
    # Signal only processes we launched and still own; no process-name scans,
    # stale PID files, or interaction with an installed watcher service.
    for child in children:
        if child.poll() is None:
            try:
                child.terminate()
            except ProcessLookupError:
                pass
    deadline = time.monotonic() + timeout
    for child in children:
        try:
            child.wait(timeout=max(0, deadline - time.monotonic()))
        except subprocess.TimeoutExpired:
            child.kill()
            child.wait()


def run(args):
    if args.log_dir:
        args.log_dir = args.log_dir.expanduser().resolve()
        args.log_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    logs = Path(tempfile.mkdtemp(prefix="agy-pair-", dir=args.log_dir))
    # Create private files before passing them to either child. Keep them for
    # postmortem inspection, but never persist the token ourselves.
    for name in ("agy.log", "reader.log"):
        (logs / name).touch(mode=0o600)
    print(f"agy-pair: logs: {logs}", file=sys.stderr, flush=True)
    token = secrets.token_urlsafe(32)
    env = os.environ.copy()
    for key in ("ANTIGRAVITY_DAEMON_URL", "ANTIGRAVITY_LS_ADDRESS",
                "ANTIGRAVITY_LS_VERSION", "ANTIGRAVITY_CLI_ROOT", "ANTIGRAVITY_CSRF_TOKEN"):
        env.pop(key, None)
    env["ANTIGRAVITY_CSRF_TOKEN"] = token
    children = []
    stop_signal = None

    def interrupted(signum, _frame):
        # Do not raise during Popen: a signal between fork and append must not
        # lose a child before it is registered for cleanup.
        nonlocal stop_signal
        stop_signal = signum

    def check_signal():
        if stop_signal is not None:
            raise Stopped(stop_signal)

    old_handlers = {sig: signal.signal(sig, interrupted)
                    for sig in (signal.SIGINT, signal.SIGTERM, signal.SIGHUP)}
    terminal = termios.tcgetattr(sys.stdin.fileno()) if sys.stdin.isatty() else None
    try:
        # Inherit the terminal and foreground process group so agy's TUI and
        # stdin work normally. Reader output goes to a separate log.
        agy = subprocess.Popen(
            [args.agy, f"--csrf_token={token}", f"--log-file={logs / 'agy.log'}", *args.agy_args],
            env=env,
        )
        children.append(agy)
        deadline = time.monotonic() + args.startup_timeout
        port = None
        while time.monotonic() < deadline:
            check_signal()
            if agy.poll() is not None:
                if agy.returncode:
                    return exit_code(agy.returncode)
                raise RuntimeError("agy exited before the pair was ready")
            port = daemon_port(logs / "agy.log")
            remaining = deadline - time.monotonic()
            if port and authenticated(port, token, max(0.01, min(0.5, remaining))):
                break
            time.sleep(0.05)
        else:
            check_signal()
            raise RuntimeError("timed out waiting for agy's authenticated HTTP listener")

        check_signal()
        env["ANTIGRAVITY_DAEMON_URL"] = f"http://127.0.0.1:{port}"
        with (logs / "reader.log").open("ab") as output:
            reader = subprocess.Popen(
                [args.reader, "--root", args.root, "--watch",
                 f"--watch-interval={args.watch_interval}", "--watch-idle-timeout=0"],
                env=env, stdin=subprocess.DEVNULL, stdout=output, stderr=output,
                start_new_session=True,
            )
        children.append(reader)
        print(f"agy-pair: authenticated at {env['ANTIGRAVITY_DAEMON_URL']}; "
              f"agy PID {agy.pid}, reader PID {reader.pid}; watching {args.root}",
              file=sys.stderr, flush=True)
        while True:
            check_signal()
            if reader.poll() is not None:
                raise RuntimeError(f"reader exited with status {exit_code(reader.returncode)}; see reader.log")
            if agy.poll() is not None:
                return exit_code(agy.returncode)
            time.sleep(0.05)
    except Stopped as stopped:
        return 128 + stopped.signum
    finally:
        # A second Ctrl-C must not interrupt reaping the owned children.
        for sig in old_handlers:
            signal.signal(sig, signal.SIG_IGN)
        stop_children(children, args.stop_timeout)
        if terminal is not None:
            # A killed TUI cannot restore its own raw-mode settings.
            try:
                termios.tcsetattr(sys.stdin.fileno(), termios.TCSANOW, terminal)
            except termios.error:
                pass  # The terminal may already have disappeared on SIGHUP.
        for sig, handler in old_handlers.items():
            signal.signal(sig, handler)
        print("agy-pair: stopped", file=sys.stderr, flush=True)


def main():
    try:
        args = arguments()
        return run(args) if args.mode == "run" else detached(args)
    except (OSError, RuntimeError) as error:
        print(f"agy-pair: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
