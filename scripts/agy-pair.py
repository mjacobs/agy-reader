#!/usr/bin/env python3
"""Run an interactive agy and an authenticated reader watcher together."""

import argparse
import http.client
import math
import mmap
import os
from pathlib import Path
import re
import secrets
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import termios
import time


TOKEN_MODES = ("argv", "no-flag", "shim")
SHIM_SLOT = "--pair_token_slot"
TOKEN_MODE_HELP = """\
token modes (--token-mode):
  argv     default. agy gets a fresh token as its first argument, so sync works
           at once, even while agy is idle. Any local account can read process
           arguments, so use it only on single-user machines.
  no-flag  agy picks its own token. The helper reads it from the environment of
           agy's first tool process, which only your account can see. Nothing
           syncs until agy runs a tool; a session without tool calls is not
           synced by this pair. A watcher you start separately, such as the
           systemd service, also authenticates only once it sees a tool
           process. Linux only.
  shim     opt-in. A preload library (make shim) hands agy the token before it
           reads its arguments; process arguments show only a placeholder. Sync
           works at once. Linux and glibc only; relies on agy's hidden
           --csrf_token flag. If the library does not load, agy exits 2.
           --agy must name agy itself: the helper refuses a wrapper script or
           a launcher such as a mise shim, which would pass the token to agy
           in its arguments.
           agy's LD_PRELOAD is the library alone: an LD_PRELOAD you already
           had does not apply to agy or its tools (the reader keeps it). The
           token stays in agy's own /proc/<pid>/environ as
           ANTIGRAVITY_PAIR_TOKEN, where a watcher you start separately, such
           as the systemd service, reads it; agy's tools do not inherit it.
           That file also lists LD_PRELOAD and ANTIGRAVITY_PAIR_EXE.

agy writes its log to log/cli-pair-*.log under --root, where agy-reader's
discovery looks, so a watcher you start separately can find this agy; the
helper's log directory links to it as agy.log. If another account could
change that log directory or one above it, or the helper cannot create its log
there, agy's log stays in the helper's private log directory instead, and only
this pair's reader finds agy. The
helper sends a token only to a listener agy itself holds. In every mode the
reader gets the token through its environment, and processes running as your
own user can still read it. Put agy arguments after --. Exiting agy stops the
watcher."""


class Stopped(Exception):
    def __init__(self, signum):
        self.signum = signum


def defines_csrf_flag(path):
    # Shim mode must start agy itself. A wrapper script, or a compiled launcher
    # such as a mise shim, would get the real flag from the shim and pass it to
    # agy as an exec argument, readable by every local account. agy is a native
    # executable that contains its hidden flag's name; launchers do not.
    try:
        with open(path, "rb") as file:
            if file.read(4) != b"\x7fELF":
                return False
            with mmap.mmap(file.fileno(), 0, access=mmap.ACCESS_READ) as image:
                return image.find(b"csrf_token") >= 0
    except (OSError, ValueError):
        return False


def positive_seconds(value):
    seconds = float(value)
    if not math.isfinite(seconds) or seconds <= 0:
        raise argparse.ArgumentTypeError("must be a finite positive number")
    return seconds


def arguments():
    parser = argparse.ArgumentParser(
        description=__doc__, epilog=TOKEN_MODE_HELP,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    modes = parser.add_subparsers(dest="mode", required=True)
    run_parser = modes.add_parser("run", help="run the pair in the foreground", epilog=TOKEN_MODE_HELP,
                                  formatter_class=argparse.RawDescriptionHelpFormatter)
    start_parser = modes.add_parser("start", help="start a detached tmux pair", epilog=TOKEN_MODE_HELP,
                                    formatter_class=argparse.RawDescriptionHelpFormatter)
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
        if arg.split("=", 1)[0] in {"--csrf_token", "-csrf_token", "--log-file", "-log-file",
                                    SHIM_SLOT, SHIM_SLOT[1:]}:
            parser.error("the helper manages agy's CSRF token and log file")
    if args.token_mode != "argv" and not sys.platform.startswith("linux"):
        parser.error(f"--token-mode={args.token_mode} requires Linux")
    if args.token_mode == "shim":
        shim = Path(args.shim).expanduser()
        if not shim.is_file():
            parser.error("preload shim not found; run make shim or use --shim")
        args.shim = str(shim.resolve())
        # ld.so splits LD_PRELOAD at spaces and colons and cannot escape
        # them, so the library would never load and agy would reject the
        # placeholder flag.
        if any(separator in args.shim for separator in " :"):
            parser.error(f"the preload shim's path {args.shim!r} contains a space or colon, which "
                         "LD_PRELOAD cannot hold; copy the library to a path without them and "
                         "pass it with --shim")
    for name in ("agy", "reader"):
        executable = shutil.which(getattr(args, name))
        if not executable:
            parser.error(f"{name} executable not found; use --{name}" +
                         (" or run make build" if name == "reader" else ""))
        if name == "agy":
            # agy runs from the resolved file, the one checked below, but keeps
            # the path it was named by as its first argument: agy-reader's
            # discovery recognizes agy by that name, so a symlink such as
            # agy -> agy-1.3.0 must still read as agy there. Do not normalize
            # it: after a directory symlink, "dir/../agy" and its collapsed
            # form can name different files, and detached mode reruns the
            # helper with this path.
            args.agy_name = os.path.join(os.getcwd(), executable)
        setattr(args, name, str(Path(executable).resolve()))
    if args.token_mode == "shim" and not defines_csrf_flag(args.agy):
        parser.error("--token-mode=shim needs --agy to name the agy executable itself, not a "
                     "wrapper script or launcher, which would put the token in process arguments")
    args.root = str(Path(args.root).expanduser().resolve())
    return args


def launch_arguments(parser):
    parser.add_argument("--agy", default="agy", help="agy executable (default: PATH)")
    parser.add_argument("--token-mode", choices=TOKEN_MODES, default="argv",
                        help="how agy gets its token; see below (default: argv)")
    parser.add_argument(
        "--shim", default=str(Path(__file__).resolve().parents[1] / "bin/agy-pair-shim.so"),
        help="preload library for --token-mode=shim (default: this checkout's "
        "bin/agy-pair-shim.so; run make shim)",
    )
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
                        help="seconds to wait for authenticated HTTP readiness; in no-flag mode, "
                        "only for agy's HTTP listener, then without limit for its first tool (default: 30)")
    parser.add_argument("--stop-timeout", type=positive_seconds, default=5,
                        help="seconds to allow graceful shutdown before SIGKILL (default: 5)")
    parser.add_argument("--log-dir", type=Path,
                        help="parent directory for private per-run logs, which no other account "
                        "may be able to change; agy's own log is under --root and linked from "
                        "there (default: system temporary directory)")
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


def tmux_argument(arg):
    # tmux reads an argument ending in ";" as the end of its command, so a
    # prompt like "SELECT 1;" would lose the semicolon and tmux would run the
    # next argument as a command of its own. A backslash before that final
    # semicolon keeps it; tmux removes the backslash.
    return arg[:-1] + "\\;" if arg.endswith(";") else arg


def detached(args):
    if args.mode == "start":
        command = [sys.executable, str(Path(__file__).resolve()), "run",
                   "--agy", args.agy_name, "--reader", args.reader, "--root", args.root,
                   f"--token-mode={args.token_mode}",
                   f"--watch-interval={args.watch_interval}",
                   f"--startup-timeout={args.startup_timeout}", f"--stop-timeout={args.stop_timeout}"]
        if args.token_mode == "shim":
            command += ["--shim", args.shim]
        if args.log_dir:
            command += ["--log-dir", str(args.log_dir.expanduser().resolve())]
        command += ["--", *args.agy_args]
        # Hold the empty pane until remain-on-exit is set, including for fast
        # startup failures. The real command stays available for restart.
        # tmux runs a command given as one string with the user's login
        # shell, which may not be POSIX sh (fish reads backslashes in single
        # quotes as escapes). Given as several arguments, it runs the command
        # directly, so pass every command that way.
        target = tmux("new-session", "-d", "-P", "-F", "#{session_id}",
                      "-s", args.session, "-c", os.getcwd(), "sleep", "86400").stdout.strip()
        try:
            tmux("set-option", "-t", target, "@agy_pair", "1")
            pane = pane_status(target)[0]
            tmux("set-option", "-t", target, "@agy_pair_pane", pane)
            tmux("set-option", "-w", "-t", target, "remain-on-exit", "on")
            tmux("respawn-pane", "-k", "-t", pane, *map(tmux_argument, command))
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


def others_can_change(directory):
    # Whether an account other than yours (or root) could rename, replace or
    # add entries in this absolute directory: it, or a directory above it, is
    # a symlink, belongs to someone else, or is group- or world-writable
    # without the sticky bit. ACL grants show up in the group bits. In such a
    # directory another account could swap agy's log for a file of its own
    # and read what agy writes, or turn the name into a symlink to one of
    # your files. A sticky directory such as /tmp lets nobody else rename or
    # remove your entries.
    for path in (directory, *directory.parents):
        try:
            info = os.lstat(path)
        except OSError:
            return True
        if (not stat.S_ISDIR(info.st_mode) or info.st_uid not in (os.geteuid(), 0)
                or (info.st_mode & 0o022 and not info.st_mode & stat.S_ISVTX)):
            return True
    return False


def daemon_port(log):
    # Read through the descriptor the helper created the log with, never by
    # its name: only processes of your own user can write to that 0600 file,
    # wherever its name points by now.
    data = os.pread(log, os.fstat(log).st_size, 0).decode(errors="replace")
    ports = re.findall(r"listening on random port at (\d+) for HTTP\b", data)
    if ports and 0 < int(ports[-1]) < 65536:
        return int(ports[-1])
    return None


def listener(pid, port, owner=False):
    # The inode of a socket listening on 127.0.0.1:port that pid, or a
    # process under it, holds, or None. Other accounts can listen on loopback
    # ports too, so the helper sends a token only to a listener of the agy it
    # started; with a wrapper that keeps running, agy is a process under it.
    # The table is the helper's own (/proc/self/net), because that namespace
    # is where its connection goes: in another network namespace, agy's
    # loopback port is a different port. Only Linux has /proc; elsewhere only
    # argv mode runs, whose token is in agy's process arguments anyway, so the
    # check is skipped there.
    if not sys.platform.startswith("linux"):
        return "unchecked"
    sockets = {}
    for process in (pid, *descendants(pid)):
        try:
            fds = os.listdir(f"/proc/{process}/fd")
        except OSError:
            continue
        for fd in fds:
            try:
                target = os.readlink(f"/proc/{process}/fd/{fd}")
            except OSError:
                continue
            if target.startswith("socket:[") and target.endswith("]"):
                sockets[target[len("socket:["):-1]] = process
    for table in ("tcp", "tcp6"):
        try:
            rows = Path(f"/proc/self/net/{table}").read_text().splitlines()[1:]
        except OSError:
            continue
        for row in rows:
            # As in agy-reader's discovery (listeningPorts): state 0A is
            # LISTEN, on IPv4 loopback or its IPv6-mapped form.
            fields = row.split()
            if len(fields) > 9 and fields[3] == "0A" and fields[9] in sockets:
                host, _, hex_port = fields[1].partition(":")
                if (host in ("0100007F", "0000000000000000FFFF00000100007F")
                        and int(hex_port, 16) == port):
                    return sockets[fields[9]] if owner else fields[9]
    return None


def authenticated(pid, port, token, timeout):
    # Direct loopback connection: proxy environment variables must not receive
    # the token. Do not follow redirects or print response bodies.
    # Send the token only to agy's own listener. While agy holds 127.0.0.1:port
    # no other account can bind it, and a closed socket never comes back, so
    # the same listener before and after connecting means this connection
    # reached agy.
    owner = listener(pid, port)
    if owner is None:
        return False
    connection = http.client.HTTPConnection("127.0.0.1", port, timeout=timeout)
    try:
        connection.connect()
        if listener(pid, port) != owner:
            return False
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


def descendants(pid):
    # Only this agy's own process tree: never adopt a token from another
    # session. Prefer the kernel's per-thread child lists; fall back to a
    # parent-PID scan when the kernel does not provide them.
    if Path(f"/proc/{pid}/task/{pid}/children").exists():
        found, pending = [], [pid]
        while pending:
            parent = pending.pop()
            try:
                threads = os.listdir(f"/proc/{parent}/task")
            except OSError:
                continue
            for thread in threads:
                try:
                    children = Path(f"/proc/{parent}/task/{thread}/children").read_text().split()
                except OSError:
                    continue
                for child in map(int, children):
                    if child not in found:
                        found.append(child)
                        pending.append(child)
        return found
    parents = {}
    for entry in os.listdir("/proc"):
        if entry.isdigit():
            try:
                stat = Path(f"/proc/{entry}/stat").read_text()
            except OSError:
                continue
            # The command name may contain spaces or parentheses.
            parents.setdefault(int(stat.rsplit(")", 1)[1].split()[1]), []).append(int(entry))
    found, pending = [], [pid]
    while pending:
        for child in parents.get(pending.pop(), []):
            if child not in found:
                found.append(child)
                pending.append(child)
    return found


def tool_token(pid, port):
    # agy exports its own token and HTTP address only to the tools it runs.
    try:
        data = Path(f"/proc/{pid}/environ").read_bytes()
    except OSError:
        return None
    env = dict(entry.split(b"=", 1) for entry in data.split(b"\0") if b"=" in entry)
    host, _, address_port = env.get(b"ANTIGRAVITY_LS_ADDRESS", b"").decode(errors="replace").rpartition(":")
    token = env.get(b"ANTIGRAVITY_CSRF_TOKEN", b"").decode(errors="replace")
    if host in ("localhost", "127.0.0.1") and address_port == str(port) and token:
        return token
    return None


def visible_in_arguments(pids, token):
    for pid in pids:
        try:
            if token.encode() in Path(f"/proc/{pid}/cmdline").read_bytes():
                return True
        except OSError:
            pass
    return False


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
    # The run directory is private only while no other account can rename it
    # and put its own in its place.
    log_parent = args.log_dir or Path(tempfile.gettempdir()).resolve()
    if others_can_change(log_parent):
        raise RuntimeError(f"another account could change {log_parent} or a directory above it; "
                           "choose a private --log-dir")
    logs = Path(tempfile.mkdtemp(prefix="agy-pair-", dir=log_parent))
    # Create private files before passing them to either child. Keep them for
    # postmortem inspection, but never persist the token ourselves.
    # agy's log goes where agy-reader's discovery matches an agy to its root
    # (log/cli-*.log), so a watcher started separately, such as the systemd
    # service, can find this agy. With the log elsewhere, discovery skips it
    # in every token mode. The run directory links to it. O_EXCL protects the
    # file but not its name, so use that directory only if no other account
    # can change it. The reader needs only the conversations directory, so a
    # store that cannot hold the log, such as one you made read-only, still
    # pairs: agy's log then stays in the run directory too.
    # O_EXCL also refuses a symlink. Keep the descriptor: agy's port is read
    # through it, not by the file's name.
    log_flags = os.O_RDONLY | os.O_CREAT | os.O_EXCL
    agy_log_dir = Path(args.root) / "log"
    agy_log_fd = None
    try:
        agy_log_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    except OSError as error:
        fallback = f"cannot create {agy_log_dir} ({error.strerror or error})"
    else:
        if others_can_change(agy_log_dir):
            fallback = (f"another account could change {agy_log_dir} or a directory above it, "
                        "or it is a symlink")
        else:
            agy_log = agy_log_dir / f"cli-pair-{time.strftime('%Y%m%d_%H%M%S')}-{secrets.token_hex(4)}.log"
            try:
                agy_log_fd = os.open(agy_log, log_flags, 0o600)
            except OSError as error:
                fallback = f"cannot create agy's log in {agy_log_dir} ({error.strerror or error})"
    if agy_log_fd is None:
        print(f"agy-pair: {fallback}; agy logs to this run's private directory instead, so a "
              "watcher started separately will not find this agy", file=sys.stderr, flush=True)
        agy_log = logs / "agy.log"
        agy_log_fd = os.open(agy_log, log_flags, 0o600)
    else:
        (logs / "agy.log").symlink_to(agy_log)
    (logs / "reader.log").touch(mode=0o600)
    print(f"agy-pair: logs: {logs}", file=sys.stderr, flush=True)
    token = None if args.token_mode == "no-flag" else secrets.token_urlsafe(32)
    env = os.environ.copy()
    for key in ("ANTIGRAVITY_DAEMON_URL", "ANTIGRAVITY_LS_ADDRESS", "ANTIGRAVITY_LS_VERSION",
                "ANTIGRAVITY_CLI_ROOT", "ANTIGRAVITY_CSRF_TOKEN", "ANTIGRAVITY_PAIR_TOKEN",
                "ANTIGRAVITY_PAIR_EXE"):
        env.pop(key, None)
    log_flag = f"--log-file={agy_log}"
    agy_env = env
    if args.token_mode == "argv":
        env["ANTIGRAVITY_CSRF_TOKEN"] = token
        agy_command = [args.agy_name, f"--csrf_token={token}", log_flag]
    elif args.token_mode == "shim":
        # The shim swaps the placeholder for the token, only in the process
        # running the checked agy file, and removes its variables before agy
        # starts any tool. agy's own /proc/<pid>/environ still lists them,
        # token included: agy-reader's discovery reads it there, so a watcher
        # started separately can authenticate to this agy while it is idle.
        # LD_PRELOAD holds the shim alone, so dropping the whole variable
        # leaves no tool loading the shim, and no other preloaded code starts
        # inside agy alongside the token. The reader keeps an inherited value.
        if env.get("LD_PRELOAD"):
            print("agy-pair: --token-mode=shim replaces your LD_PRELOAD for agy and its tools; "
                  "the reader keeps it", file=sys.stderr, flush=True)
        agy_env = dict(env, LD_PRELOAD=args.shim, ANTIGRAVITY_PAIR_TOKEN=token,
                       ANTIGRAVITY_PAIR_EXE=args.agy)
        agy_command = [args.agy_name, f"{SHIM_SLOT}=1", log_flag]
    else:
        # agy must not inherit a token variable: its early children would
        # carry the stale value and look like a tool's credentials.
        agy_command = [args.agy_name, log_flag]
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
        agy = subprocess.Popen([*agy_command, *args.agy_args], executable=args.agy, env=agy_env)
        children.append(agy)
        deadline = time.monotonic() + args.startup_timeout
        port = None
        while time.monotonic() < deadline:
            check_signal()
            if agy.poll() is not None:
                if agy.returncode == 2 and args.token_mode == "shim":
                    print("agy-pair: agy rejected the placeholder flag; the preload shim "
                          "did not load, received no token, or did not run in the checked agy "
                          "executable", file=sys.stderr, flush=True)
                if agy.returncode:
                    return exit_code(agy.returncode)
                raise RuntimeError("agy exited before the pair was ready")
            port = daemon_port(agy_log_fd)
            remaining = deadline - time.monotonic()
            if port and listener(agy.pid, port) and (
                    token is None or authenticated(agy.pid, port, token, max(0.01, min(0.5, remaining)))):
                break
            time.sleep(0.05)
        else:
            check_signal()
            raise RuntimeError("timed out waiting for agy's " +
                               ("HTTP listener" if token is None else "authenticated HTTP listener") +
                               (f"; agy is not listening on port {port}, which its log names"
                                if port and not listener(agy.pid, port) else ""))

        if token is None:
            print(f"agy-pair: agy is listening on port {port}; sync is pending until agy "
                  "runs its first tool (--token-mode=no-flag)", file=sys.stderr, flush=True)
            # Keep every token found so far: the tool that exported it may exit
            # before agy answers. Only an explicit refusal (401 or 403) rules a
            # token out; after a timeout or another error, try it again.
            candidates, rejected = [], set()
            while token is None:
                check_signal()
                if agy.poll() is not None:
                    print("agy-pair: agy exited before running a tool; this pair synced nothing",
                          file=sys.stderr, flush=True)
                    return exit_code(agy.returncode)
                for pid in descendants(agy.pid):
                    candidate = tool_token(pid, port)
                    if candidate and candidate not in rejected and candidate not in candidates:
                        candidates.append(candidate)
                for candidate in list(candidates):
                    try:
                        if authenticated(agy.pid, port, candidate, 0.5):
                            token = candidate
                            break
                    except RuntimeError:
                        candidates.remove(candidate)
                        rejected.add(candidate)
                else:
                    # Short commands can finish between polls; a later tool
                    # exports the same token.
                    time.sleep(0.02)
        env["ANTIGRAVITY_CSRF_TOKEN"] = token
        if args.token_mode == "shim" and visible_in_arguments([agy.pid, *descendants(agy.pid)], token):
            # The shim must never write the token where /proc/<pid>/cmdline
            # reads it, and agy must not re-execute itself with the real flag.
            raise RuntimeError("agy's token is visible in process arguments; stopping the pair")

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
        # Stop the owned listener before its wrapper, which may not forward
        # signals. Leave unrelated tools alone; only the socket holder is owned.
        if children and children[0].poll() is None and sys.platform.startswith("linux"):
            owned = listener(children[0].pid, daemon_port(agy_log_fd), owner=True)
            if owned and owned != children[0].pid:
                try:
                    os.kill(owned, signal.SIGTERM)
                    deadline = time.monotonic() + args.stop_timeout
                    while time.monotonic() < deadline:
                        try:
                            os.kill(owned, 0)
                        except ProcessLookupError:
                            break
                        time.sleep(0.02)
                    else:
                        os.kill(owned, signal.SIGKILL)
                except ProcessLookupError:
                    pass
        stop_children(children, args.stop_timeout)
        os.close(agy_log_fd)
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
