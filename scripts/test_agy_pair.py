"""Process-level tests: real child processes and an authenticated loopback RPC."""

import contextlib
import errno
import hashlib
import http.client
import importlib.util
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import io
import itertools
import pty
import runpy
import select
import shutil
import signal
import subprocess
import sys
import sysconfig
import tempfile
import termios
import threading
import time
import tty
import unittest
from unittest import mock
import uuid


HELPER = Path(__file__).with_name("agy-pair.py")
SHIM_SOURCE = HELPER.with_name("agy-pair-shim.c")

# Process details come from procfs, which only Linux has. Pointing
# AGY_PAIR_TEST_PROC_ROOT at a missing directory simulates a platform without
# it, such as macOS: the argv-mode tests must still pass there.
PROC = Path(os.environ.get("AGY_PAIR_TEST_PROC_ROOT", "/proc"))
HAVE_PROC = sys.platform.startswith("linux") and PROC.is_dir()
CC = shutil.which("cc")
PY_INCLUDE = Path(sysconfig.get_paths()["include"])
PY_EMBED = bool(sysconfig.get_config_var("Py_ENABLE_SHARED")) and (PY_INCLUDE / "Python.h").is_file()

linux_only = unittest.skipUnless(HAVE_PROC, "no-flag and shim modes need Linux and /proc")
needs_cc = unittest.skipUnless(HAVE_PROC and CC,
                               "the preload shim needs Linux with /proc, glibc and a C compiler")
needs_native_agy = unittest.skipUnless(
    HAVE_PROC and CC and PY_EMBED,
    "shim-mode pairs need Linux with /proc, a C compiler, and Python's embedding headers "
    "for a native test agy")

# Shim mode accepts only a native executable that names --csrf_token, and the
# shim swaps the token in only inside that executable. So this test agy runs
# the Python fake in its own process: Python reads the arguments as agy's Go
# runtime would, after the shim's swap, and its /proc entries are agy's.
NATIVE_AGY_C = r"""
#define PY_SSIZE_T_CLEAN
#include <Python.h>
#include <stdlib.h>

__attribute__((used)) static const char flag_name[] = "csrf_token";

int main(int argc, char **argv) {
    char **args = calloc((size_t)argc + 2, sizeof *args);
    if (!args) return 70;
    args[0] = argv[0];
    args[1] = FIXTURE;
    for (int i = 1; i < argc; i++) args[i + 1] = argv[i];
    PyConfig config;
    PyConfig_InitPythonConfig(&config);
    PyStatus status = PyConfig_SetBytesString(&config, &config.executable, PYTHON);
    if (!PyStatus_Exception(status)) status = PyConfig_SetBytesArgv(&config, argc + 1, args);
    if (!PyStatus_Exception(status)) status = Py_InitializeFromConfig(&config);
    PyConfig_Clear(&config);
    if (PyStatus_Exception(status)) Py_ExitStatusException(status);
    return Py_RunMain();
}
"""

# A compiled launcher, like a mise shim: it runs another program with its own
# arguments.
LAUNCHER_C = r"""
#include <unistd.h>
int main(int argc, char **argv) { (void)argc; execv(TARGET, argv); return 127; }
"""

# Writes the arguments its process received, which shim variables it
# inherited, and whether its original environment block, as
# /proc/<pid>/environ shows it, still holds ANTIGRAVITY_PAIR_TOKEN with the
# token in REPORT_TOKEN.
REPORT_C = r"""
#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
int main(int argc, char **argv) {
    FILE *out = fopen(getenv("REPORT"), "w");
    if (!out) return 70;
    for (int i = 0; i < argc; i++) fprintf(out, "%s\n", argv[i]);
    char block[65536];
    FILE *environ_file = fopen("/proc/self/environ", "r");
    size_t n = environ_file ? fread(block, 1, sizeof block, environ_file) : 0;
    if (environ_file) fclose(environ_file);
    /* REPORT_TOKEN holds the value too, so look for the pair entry itself. */
    char needle[512];
    const char *token = getenv("REPORT_TOKEN");
    int len = token ? snprintf(needle, sizeof needle, "ANTIGRAVITY_PAIR_TOKEN=%s", token) : -1;
    fprintf(out, "environ:%d\n", len > 0 && memmem(block, n, needle, (size_t)len) != NULL);
    fprintf(out, "inherited:%d%d%d\n", getenv("LD_PRELOAD") != NULL,
            getenv("ANTIGRAVITY_PAIR_TOKEN") != NULL, getenv("ANTIGRAVITY_PAIR_EXE") != NULL);
    return fclose(out) ? 70 : 0;
}
"""


def proc_contains(entry, text):
    # None where procfs is missing, as on macOS.
    if not PROC.is_dir():
        return None
    return text.encode() in (PROC / entry).read_bytes()


def pidfd_pid(fd):
    # The process a pidfd refers to, as the kernel reports it.
    with open(f"/proc/self/fdinfo/{fd}") as info:
        for line in info:
            if line.startswith("Pid:"):
                return int(line.split()[1])
    return None


def pidfd_open_patch(testcase, pidfds):
    # Patches os.pidfd_open to wrap the real call (pidfds) or to fail as a
    # kernel without pidfds does. A Python without os.pidfd_open can only run
    # the second; the first skips the test or subtest that asks for it.
    if not pidfds:
        return mock.patch.object(os, "pidfd_open", side_effect=OSError, create=True)
    if not hasattr(os, "pidfd_open"):
        testcase.skipTest("os.pidfd_open is not available")
    return mock.patch.object(os, "pidfd_open", wraps=os.pidfd_open)


def refusing_signals(refuse):
    # Stand-ins for os.kill and signal.pidfd_send_signal that fail with EPERM,
    # as the kernel does for a process running as another user (one started
    # with sudo, say), whenever refuse(pid, signum) is true. Otherwise they
    # send the signal. Tests never need another account for this. Without
    # signal.pidfd_send_signal, there is no stand-in for it (None).
    real_kill = os.kill
    real_pidfd_send_signal = getattr(signal, "pidfd_send_signal", None)

    def kill(pid, signum):
        if refuse(pid, signum):
            raise PermissionError(errno.EPERM, os.strerror(errno.EPERM))
        return real_kill(pid, signum)

    def pidfd_send_signal(fd, signum, *args):
        if refuse(pidfd_pid(fd), signum):
            raise PermissionError(errno.EPERM, os.strerror(errno.EPERM))
        return real_pidfd_send_signal(fd, signum, *args)

    return kill, (pidfd_send_signal if real_pidfd_send_signal else None)


def refusing_helper():
    # Runs agy-pair.py with every signal to a process whose arguments hold
    # the marker in sys.argv[1] refused, then the helper's own arguments.
    marker = sys.argv[1].encode()

    def refuse(pid, _signum):
        try:
            with open(f"/proc/{pid}/cmdline", "rb") as cmdline:
                return marker in cmdline.read()
        except OSError:
            return False

    os.kill, pidfd_send_signal = refusing_signals(refuse)
    if pidfd_send_signal:
        signal.pidfd_send_signal = pidfd_send_signal
    sys.argv = sys.argv[2:]
    runpy.run_path(sys.argv[0], run_name="__main__")


def fake_agy():
    options = dict(arg.split("=", 1) for arg in sys.argv[1:] if "=" in arg)
    if "--pair_token_slot" in options:
        # Like agy: an unknown flag stops the launch before anything starts.
        print("flag provided but not defined: -pair_token_slot", file=sys.stderr)
        return 2
    root = Path(options["--fixture"])
    config = json.loads((root / "config.json").read_text())
    # Without the launch flag, agy picks its own token and exports it only to
    # the tools it runs. The test reads this synthetic value to scan for it.
    token = options.get("--csrf_token") or "fake-agy-" + uuid.uuid4().hex
    (root / "agy-token").write_text(token)
    if config.get("ignore_term"):
        signal.signal(signal.SIGTERM, signal.SIG_IGN)
    if config.get("raw_terminal"):
        tty.setraw(sys.stdin.fileno())
    child = subprocess.run(
        [sys.executable, "-c", "import json, os; print(json.dumps([name in os.environ for name in "
         "('LD_PRELOAD', 'ANTIGRAVITY_PAIR_TOKEN', 'ANTIGRAVITY_PAIR_EXE')]))"],
        capture_output=True, text=True, check=True,
    )
    log_index = next(i for i, arg in enumerate(sys.argv) if arg.startswith("--log-file="))
    (root / "agy.json").write_text(json.dumps({
        "pid": os.getpid(), "token_hash": hashlib.sha256(token.encode()).hexdigest(),
        "first_arg": sys.argv[1].split("=")[0], "args": sys.argv[log_index + 1:],
        "log_file": options["--log-file"],
        "same_env": token == os.environ.get("ANTIGRAVITY_CSRF_TOKEN"),
        "env_token": "ANTIGRAVITY_CSRF_TOKEN" in os.environ,
        "url": os.environ.get("ANTIGRAVITY_DAEMON_URL"),
        "cmdline_has_token": proc_contains("self/cmdline", token),
        "environ_has_token": proc_contains("self/environ", token),
        "preload_env": "LD_PRELOAD" in os.environ,
        "pair_env": "ANTIGRAVITY_PAIR_TOKEN" in os.environ,
        "pair_exe_env": "ANTIGRAVITY_PAIR_EXE" in os.environ,
        "child_env": json.loads(child.stdout),
    }))
    if config.get("early_exit") is not None:
        return config["early_exit"]

    time.sleep(config.get("log_delay", 0))
    tools, lock, right_tries = [], threading.Lock(), [0]

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            good = self.headers.get("x-codeium-csrf-token") == token
            status = 200 if good and not config.get("reject") else 401
            flaky = config.get("flaky_auth")
            with lock:
                right_tries[0] += good
                first_right = good and right_tries[0] == 1
            if flaky and first_right:
                # The first check with the right token fails transiently, and
                # only after every tool that exported the token has exited.
                deadline = time.monotonic() + 5
                while len(tools) < len(config.get("tools", [])) and time.monotonic() < deadline:
                    time.sleep(0.01)  # a tool can export its token before it is listed
                for tool in tools:
                    tool.terminate()
                    tool.wait()
                (root / "flaky.json").write_text(json.dumps({
                    "tools": len(tools), "running": sum(tool.poll() is None for tool in tools)}))
                time.sleep(flaky.get("delay", 0))
                status = flaky.get("status", 200)
            try:
                self.send_response(status)
                self.end_headers()
                self.wfile.write(b"{}")
            except OSError:
                pass  # The helper stopped waiting for this answer.

        def log_message(self, *_args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    if not config.get("no_listener_log"):
        log_path = Path(options["--log-file"])
        with log_path.open("a") as log:
            if config.get("swap_log_port"):
                # Another account that can write the log's directory swaps
                # the name for a file of its own after agy opened it. agy
                # keeps writing the file it opened.
                log_path.unlink()
                log_path.write_text(f"listening on random port at {config['swap_log_port']} for HTTP\n")
            # log_port: agy's own log names a port agy does not listen on.
            log.write(f"listening on random port at {config.get('log_port', server.server_port)} for HTTP\n"
                      "listening on random port at 1 for HTTPS (gRPC)\n")

    def run_tool(tool):
        time.sleep(tool.get("delay", 0))
        env = dict(os.environ)
        if tool.get("token"):
            port = server.server_port if tool.get("port", "right") == "right" else 1
            env.update(ANTIGRAVITY_LS_ADDRESS=f"localhost:{port}", ANTIGRAVITY_LS_VERSION="cli-test",
                       ANTIGRAVITY_CSRF_TOKEN=token if tool["token"] == "right" else "wrong-token")
        tools.append(subprocess.Popen(
            [str(root / "tool"), str(tool.get("depth", 1)), str(tool.get("lifetime", 5))],
            env=env, stdin=subprocess.DEVNULL,
        ))

    for tool in config.get("tools", []):
        threading.Thread(target=run_tool, args=(tool,), daemon=True).start()
    while True:
        ready, _, _ = select.select([sys.stdin], [], [], 0.05)
        if ready:
            line = sys.stdin.readline().strip()
            if line.startswith("exit "):
                return int(line.split()[1])
        if (root / "exit").exists():
            return int((root / "exit").read_text())
        time.sleep(0.02)


def fake_tool():
    # A tool process agy runs, optionally below an intermediate shell.
    depth, lifetime = int(sys.argv[1]), float(sys.argv[2])
    parent = os.getppid()
    if depth > 1:
        subprocess.Popen([sys.argv[0], str(depth - 1), str(lifetime)], stdin=subprocess.DEVNULL)
    deadline = time.monotonic() + lifetime
    while time.monotonic() < deadline and os.getppid() == parent:
        time.sleep(0.02)


def visible_in_any_arguments(text):
    for entry in os.listdir(PROC):
        try:
            if entry.isdigit() and text.encode() in (PROC / entry / "cmdline").read_bytes():
                return True
        except OSError:
            pass
    return False


def environment_block(pid, names, texts):
    # /proc/<pid>/environ as another process of the same user sees it: the
    # environment the process started with, not its live environment after
    # unsetenv. Return only the entries for `names` and whether each of
    # `texts` appears, so a failing assertion never prints the rest of the
    # environment, which may hold the user's own secrets.
    data = (PROC / str(pid) / "environ").read_bytes()
    listed = sorted(entry for entry in data.split(b"\0") if entry.split(b"=", 1)[0] in names)
    return listed, [text.encode() in data for text in texts]


class Decoy:
    # Another account's loopback listener. It answers 200 to anything, so a
    # helper that sent it agy's token would take that for agy's acceptance.
    def __init__(self, test):
        self.connections = 0
        decoy = self

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                self.send_response(200)
                self.end_headers()
                self.wfile.write(b"{}")

            def log_message(self, *_args):
                pass

        class Server(ThreadingHTTPServer):
            def verify_request(self, request, client_address):
                decoy.connections += 1
                return True

        self.server = Server(("127.0.0.1", 0), Handler)
        self.port = self.server.server_port
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        test.addCleanup(self.server.server_close)
        test.addCleanup(self.server.shutdown)


def fake_reader():
    root = Path(sys.argv[sys.argv.index("--root") + 1]).parent
    config = json.loads((root / "config.json").read_text())
    if config.get("ignore_term"):
        signal.signal(signal.SIGTERM, signal.SIG_IGN)
    token = os.environ["ANTIGRAVITY_CSRF_TOKEN"]
    url = os.environ["ANTIGRAVITY_DAEMON_URL"]
    connection = http.client.HTTPConnection(url.removeprefix("http://"), timeout=1)
    connection.request("POST", "/exa.language_server_pb.LanguageServerService/GetAllCascadeTrajectories",
                       b"{}", {"x-codeium-csrf-token": token})
    status = connection.getresponse().status
    connection.close()
    (root / "reader.json").write_text(json.dumps({
        "pid": os.getpid(), "token_hash": hashlib.sha256(token.encode()).hexdigest(),
        "status": status, "url": url, "args": sys.argv[1:], "preload": os.environ.get("LD_PRELOAD"),
    }))
    if config.get("reader_exit") is not None:
        return config["reader_exit"]
    while True:
        time.sleep(0.05)


class PairLifecycle(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="agy-pair-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        # Whatever the umask, no other account may change the store, or the
        # helper keeps agy's log out of it.
        (self.root / "store").mkdir(mode=0o700)
        self.configure()
        self.agy = self.fixture("agy", "fake_agy")
        self.fixture("tool", "fake_tool")
        self.reader = self.fixture("reader", "fake_reader")
        self.logs = self.root / "logs"
        self.base = ["--agy", str(self.agy), "--reader", str(self.reader),
                     "--root", str(self.root / "store"), "--log-dir", str(self.logs),
                     "--startup-timeout=2", "--stop-timeout=0.2"]
        self.agy_args = ["--", f"--fixture={self.root}", "--print", "spaces ; $ are literal"]

    def configure(self, **config):
        (self.root / "config.json").write_text(json.dumps(config))

    def fixture(self, name, mode):
        path = self.root / name
        path.write_text(f"#!{sys.executable}\nimport runpy\n"
                        f"runpy.run_path({str(Path(__file__).resolve())!r}, run_name={mode!r})\n")
        path.chmod(0o700)
        return path

    def launch(self, *extra, env_overrides=None, helper=(sys.executable, str(HELPER))):
        env = os.environ.copy()
        env.update(ANTIGRAVITY_CSRF_TOKEN="stale-token",
                   ANTIGRAVITY_DAEMON_URL="http://127.0.0.1:1",
                   ANTIGRAVITY_CLI_ROOT="/wrong-root")
        for key, value in (env_overrides or {}).items():
            if value is None:
                env.pop(key, None)
            else:
                env[key] = value
        process = subprocess.Popen(
            [*helper, "run", *self.base, *extra, *self.agy_args],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            text=True, env=env, start_new_session=True,
        )
        self.addCleanup(self.cleanup_process, process)
        return process

    def cleanup_process(self, process):
        if process.poll() is None:
            process.terminate()
            process.wait(timeout=5)
        for stream in (process.stdin, process.stdout, process.stderr):
            if stream is not None:
                stream.close()

    def record(self, name):
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            try:
                return json.loads((self.root / f"{name}.json").read_text())
            except (FileNotFoundError, json.JSONDecodeError):
                time.sleep(0.02)
        self.fail(f"{name} did not start")

    def assert_stopped(self, *records):
        for record in records:
            with self.assertRaises(ProcessLookupError):
                os.kill(record["pid"], 0)

    def build_library(self, name, source):
        library = self.root / name
        subprocess.run([CC, "-shared", "-fPIC", "-o", str(library), str(source)], check=True)
        return library.resolve()

    def build_program(self, name, code, *flags):
        source = self.root / f"{name}.c"
        source.write_text(code)
        program = self.root / name
        subprocess.run([CC, "-o", str(program), str(source), *flags], check=True)
        return program.resolve()

    def build_native_agy(self):
        libdir = sysconfig.get_config_var("LIBDIR")
        return self.build_program(
            "native-agy", NATIVE_AGY_C, f"-I{PY_INCLUDE}", f'-DFIXTURE="{self.agy}"',
            f'-DPYTHON="{sys.executable}"', f"-L{libdir}",
            f"-lpython{sysconfig.get_config_var('LDVERSION')}", f"-Wl,-rpath,{libdir}",
        )

    def wrappers(self, target):
        # A wrapper script and a compiled launcher, both running target with
        # their own arguments.
        script = self.root / "wrapper.sh"
        script.write_text(f"#!/bin/sh\nexec '{target}' \"$@\"\n")
        script.chmod(0o700)
        launcher = self.build_program("launcher", LAUNCHER_C, f'-DTARGET="{target}"')
        return {"script": script, "launcher": launcher}

    def test_authenticated_pair_and_agy_exit_code(self):
        process = self.launch()
        agy, reader = self.record("agy"), self.record("reader")
        self.assertEqual(agy["first_arg"], "--csrf_token")
        self.assertTrue(agy["same_env"])
        # The documented argv-mode exposure, observable only where procfs exists.
        self.assertIs(agy["cmdline_has_token"], True if HAVE_PROC else None)
        self.assertIsNone(agy["url"])
        self.assertEqual(agy["token_hash"], reader["token_hash"])
        self.assertNotEqual(agy["token_hash"], hashlib.sha256(b"stale-token").hexdigest())
        self.assertEqual(reader["status"], 200)
        self.assertIn("spaces ; $ are literal", agy["args"])
        stdout, stderr = process.communicate("exit 7\n", timeout=5)
        self.assertEqual(process.returncode, 7, stderr)
        self.assertNotIn("stale-token", stdout + stderr)
        self.assert_stopped(agy, reader)
        log_dir, = self.logs.iterdir()
        self.assertEqual(log_dir.stat().st_mode & 0o777, 0o700)
        self.assertEqual(sorted(path.name for path in log_dir.iterdir()), ["agy.log", "reader.log"])
        for path in log_dir.iterdir():
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
        self.assert_agy_log_under_root(agy, log_dir)

    def assert_agy_log_under_root(self, agy, log_dir):
        # agy-reader's discovery matches an agy to a root only through an
        # open log in that root's log/cli-*.log. agy writes there, so a
        # watcher started separately can find it; the run directory links to it.
        agy_log = Path(agy["log_file"])
        self.assertEqual(agy_log.parent, (self.root / "store" / "log").resolve())
        self.assertRegex(agy_log.name, r"^cli-pair-\d{8}_\d{6}-[0-9a-f]{8}\.log$")
        self.assertEqual(agy_log.stat().st_mode & 0o777, 0o600)
        self.assertIn("listening on random port at", agy_log.read_text())
        self.assertTrue((log_dir / "agy.log").is_symlink())
        self.assertEqual((log_dir / "agy.log").resolve(), agy_log)

    def test_signals_stop_both_children(self):
        for sig in (signal.SIGINT, signal.SIGTERM, signal.SIGHUP):
            with self.subTest(signal=sig):
                for name in ("agy.json", "reader.json"):
                    (self.root / name).unlink(missing_ok=True)
                process = self.launch()
                agy, reader = self.record("agy"), self.record("reader")
                process.send_signal(sig)
                _, stderr = process.communicate(timeout=5)
                self.assertEqual(process.returncode, 128 + sig, stderr)
                self.assert_stopped(agy, reader)

    def test_forced_cleanup_is_bounded(self):
        self.configure(ignore_term=True)
        process = self.launch()
        agy, reader = self.record("agy"), self.record("reader")
        process.terminate()
        process.communicate(timeout=3)
        self.assertEqual(process.returncode, 143)
        self.assert_stopped(agy, reader)

    def test_terminal_is_restored_after_forced_shutdown(self):
        self.configure(ignore_term=True, raw_terminal=True)
        master, slave = pty.openpty()
        self.addCleanup(os.close, master)
        self.addCleanup(os.close, slave)
        initial = termios.tcgetattr(slave)
        process = subprocess.Popen(
            [sys.executable, str(HELPER), "run", *self.base, *self.agy_args],
            stdin=slave, stdout=slave, stderr=slave, start_new_session=True,
        )
        self.addCleanup(self.cleanup_process, process)
        agy, reader = self.record("agy"), self.record("reader")
        self.assertNotEqual(termios.tcgetattr(slave), initial)
        process.terminate()
        process.wait(timeout=3)
        self.assertEqual(termios.tcgetattr(slave), initial)
        self.assert_stopped(agy, reader)

    def test_reader_failure_stops_agy(self):
        self.configure(reader_exit=23)
        process = self.launch()
        agy, reader = self.record("agy"), self.record("reader")
        _, stderr = process.communicate(timeout=5)
        self.assertEqual(process.returncode, 1, stderr)
        self.assertIn("reader exited with status 23", stderr)
        self.assert_stopped(agy, reader)

    def test_startup_timeout_and_auth_rejection_clean_up(self):
        for config, expected in [({"no_listener_log": True}, "timed out"),
                                 ({"reject": True}, "rejected")]:
            with self.subTest(config=config):
                (self.root / "agy.json").unlink(missing_ok=True)
                self.configure(**config)
                process = self.launch("--startup-timeout=0.4")
                agy = self.record("agy")
                _, stderr = process.communicate(timeout=5)
                self.assertEqual(process.returncode, 1, stderr)
                self.assertIn(expected, stderr)
                self.assertFalse((self.root / "reader.json").exists())
                self.assert_stopped(agy)

    def test_early_agy_exit_preserves_failure(self):
        self.configure(early_exit=12)
        process = self.launch()
        _, stderr = process.communicate(timeout=5)
        self.assertEqual(process.returncode, 12, stderr)
        self.assertFalse((self.root / "reader.json").exists())

    def test_a_swapped_log_name_never_sends_the_token_elsewhere(self):
        # After agy opens its log, another account swaps the name for a file
        # naming its own listener. The helper reads the file it created, which
        # agy still writes, so it finds agy's real port.
        decoy = Decoy(self)
        self.configure(swap_log_port=decoy.port)
        process = self.launch()
        agy, reader = self.record("agy"), self.record("reader")
        self.assertEqual(reader["status"], 200)
        self.assertNotEqual(reader["url"], f"http://127.0.0.1:{decoy.port}")
        _, stderr = process.communicate("exit 0\n", timeout=5)
        self.assertEqual(process.returncode, 0, stderr)
        self.assertEqual(decoy.connections, 0)
        self.assert_stopped(agy, reader)

    @linux_only
    def test_the_token_goes_only_to_a_listener_agy_holds(self):
        # agy's log names a port where another account listens. The helper
        # never connects there, in any mode, and gives up at the timeout.
        decoy = Decoy(self)
        for mode in ("argv", "no-flag"):
            with self.subTest(mode=mode):
                (self.root / "agy.json").unlink(missing_ok=True)
                self.configure(log_port=decoy.port, tools=[{"token": "right"}])
                process = self.launch(f"--token-mode={mode}", "--startup-timeout=0.5")
                agy = self.record("agy")
                _, stderr = process.communicate(timeout=5)
                self.assertEqual(process.returncode, 1, stderr)
                self.assertIn(f"agy is not listening on port {decoy.port}, which its log names", stderr)
                self.assertEqual(decoy.connections, 0)
                self.assertFalse((self.root / "reader.json").exists())
                self.assert_stopped(agy)

    @linux_only
    def test_a_wrapper_that_runs_agy_as_its_child_still_pairs(self):
        # In argv and no-flag modes --agy may name a wrapper that keeps
        # running while agy runs under it. agy's listener then belongs to a
        # process under the one the helper started.
        wrapper = self.root / "agy-wrapper"
        wrapper.write_text(f"#!/bin/sh\n'{self.agy}' \"$@\"\nexit $?\n")
        wrapper.chmod(0o700)
        for mode, config in (("argv", {}), ("no-flag", {"tools": [{"delay": 0.3, "token": "right"}]})):
            with self.subTest(mode=mode):
                for name in ("agy.json", "reader.json"):
                    (self.root / name).unlink(missing_ok=True)
                self.configure(**config)
                process = self.launch(f"--token-mode={mode}", f"--agy={wrapper}")
                agy, reader = self.record("agy"), self.record("reader")
                self.assertEqual(reader["status"], 200)
                self.assertEqual(reader["token_hash"], agy["token_hash"])
                _, stderr = process.communicate("exit 0\n", timeout=5)
                self.assertEqual(process.returncode, 0, stderr)
                self.assert_stopped(agy, reader)

    @linux_only
    def test_wrapper_listener_stops_on_supervisor_or_reader_exit(self):
        wrapper = self.root / "agy-wrapper"
        wrapper.write_text(f"#!/bin/sh\n'{self.agy}' \"$@\"\nexit $?\n")
        wrapper.chmod(0o700)
        for failure in ("supervisor", "reader"):
            with self.subTest(failure=failure):
                for name in ("agy.json", "reader.json"):
                    (self.root / name).unlink(missing_ok=True)
                self.configure()
                process = self.launch(f"--agy={wrapper}")
                agy, reader = self.record("agy"), self.record("reader")
                if failure == "supervisor":
                    process.terminate()
                else:
                    os.kill(reader["pid"], signal.SIGTERM)
                _, stderr = process.communicate(timeout=5)
                self.assertNotEqual(process.returncode, 0, stderr)
                self.assert_stopped(agy, reader)

    @linux_only
    def test_wrapper_startup_timeout_stops_agy_before_listener_ready(self):
        wrapper = self.root / "agy-wrapper"
        wrapper.write_text(f"#!/bin/sh\n'{self.agy}' \"$@\"\nexit $?\n")
        wrapper.chmod(0o700)
        self.configure(log_delay=10)
        process = self.launch(f"--agy={wrapper}", "--startup-timeout=1.5")
        agy = self.record("agy")
        _, stderr = process.communicate(timeout=5)
        self.assertEqual(process.returncode, 1, stderr)
        self.assert_stopped(agy)

    @linux_only
    def test_wrapper_preparation_cannot_start_agy_during_cleanup(self):
        wrapper = self.root / "agy-wrapper"
        pidfile = self.root / "wrapper.pid"
        wrapper.write_text(f"#!/bin/sh\necho $$ > '{pidfile}'\nsleep 10\n'{self.agy}' \"$@\"\nexit $?\n")
        wrapper.chmod(0o700)
        process = self.launch(f"--agy={wrapper}", "--startup-timeout=0.2", "--stop-timeout=0.2")
        _, stderr = process.communicate(timeout=5)
        self.assertEqual(process.returncode, 1, stderr)
        self.assertFalse((self.root / "agy.json").exists())
        with self.assertRaises(ProcessLookupError):
            os.kill(int(pidfile.read_text()), 0)

    def survivors(self):
        # Processes still carrying this test's fixture path in their
        # arguments: the wrapper and agy both do, and in argv mode agy's
        # arguments also hold the pair's token.
        found = []
        for entry in os.listdir(PROC):
            try:
                if entry.isdigit() and str(self.root).encode() in (PROC / entry / "cmdline").read_bytes():
                    found.append(int(entry))
            except OSError:
                pass
        return found

    def kill_survivors(self):
        # A failing run must not leave its processes behind.
        for pid in self.survivors():
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass

    def finish(self, process, timeout, settle=0):
        # Wait for the helper alone, then list what it left running. A
        # survivor still holds the helper's output pipes, so stop it before
        # reading them.
        process.wait(timeout=timeout)
        time.sleep(settle)  # anything started late would show by now
        survivors = self.survivors()
        self.kill_survivors()
        stdout, stderr = process.communicate(timeout=5)
        return stdout, stderr, survivors

    @linux_only
    def test_a_wrapper_term_trap_cannot_leave_agy_running_after_cleanup(self):
        # A wrapper whose TERM trap does not exit runs its trap and carries on
        # to start agy. Cleanup must stop that agy too: nothing carrying the
        # pair's token may outlive the helper.
        bash = shutil.which("bash")
        if bash is None:
            self.skipTest("needs bash")
        self.addCleanup(self.kill_survivors)
        wrapper = self.root / "agy-wrapper"
        pidfile = self.root / "wrapper.pid"
        wrapper.write_text(f"#!{bash}\necho $$ > '{pidfile}'\ntmp=$(mktemp -p '{self.root}')\n"
                           "trap 'rm -f \"$tmp\"' EXIT TERM\n"
                           f"sleep 10\n'{self.agy}' \"$@\"\n")
        wrapper.chmod(0o700)
        process = self.launch(f"--agy={wrapper}", "--startup-timeout=1")
        _, stderr, survivors = self.finish(process, timeout=10, settle=0.3)
        self.assertEqual(survivors, [], stderr)
        self.assertEqual(process.returncode, 1, stderr)
        self.assertIn("timed out", stderr)
        with self.assertRaises(ProcessLookupError):
            os.kill(int(pidfile.read_text()), 0)
        if (self.root / "agy.json").exists():
            self.assert_stopped(self.record("agy"))

    @linux_only
    def test_an_orphaned_inner_wrapper_cannot_leave_agy_running_after_cleanup(self):
        # The outer wrapper dies of SIGTERM at once; the inner one, whose TERM
        # trap does not exit, outlives it, leaves the launched tree, and only
        # then starts agy. Cleanup must still find that agy under the inner
        # wrapper it already holds and stop it.
        bash = shutil.which("bash")
        if bash is None:
            self.skipTest("needs bash")
        self.addCleanup(self.kill_survivors)
        inner = self.root / "agy-inner"
        outer = self.root / "agy-wrapper"
        inner_pid, outer_pid = self.root / "inner.pid", self.root / "outer.pid"
        launched = self.root / "launched"
        inner.write_text(f"#!{bash}\necho $$ > '{inner_pid}'\ntmp=$(mktemp -p '{self.root}')\n"
                         "trap 'rm -f \"$tmp\"' EXIT TERM\n"
                         f"sleep 10\ntouch '{launched}'\n'{self.agy}' \"$@\"\n")
        outer.write_text(f"#!/bin/sh\necho $$ > '{outer_pid}'\n'{inner}' \"$@\"\nexit $?\n")
        for wrapper in (inner, outer):
            wrapper.chmod(0o700)
        process = self.launch(f"--agy={outer}", "--startup-timeout=1", "--stop-timeout=2")
        _, stderr, survivors = self.finish(process, timeout=10, settle=0.3)
        self.assertEqual(survivors, [], stderr)
        self.assertEqual(process.returncode, 1, stderr)
        self.assertIn("timed out", stderr)
        for pidfile in (outer_pid, inner_pid):
            with self.assertRaises(ProcessLookupError):
                os.kill(int(pidfile.read_text()), 0)
        # The inner wrapper went on to start agy, so the late start happened.
        self.assertTrue(launched.exists(), stderr)
        if (self.root / "agy.json").exists():
            self.assert_stopped(self.record("agy"))

    @linux_only
    def test_a_process_that_refuses_signals_keeps_the_startup_timeout_and_stops_agy(self):
        # A wrapper starts something that runs as another user, as with
        # `sudo -n sleep 30 &`, before it starts agy. Every signal to that
        # process fails with EPERM. The startup timeout must stay the reason
        # reported, agy must still be stopped, and cleanup must finish.
        self.addCleanup(self.kill_survivors)
        marker = f"refuse-{uuid.uuid4().hex}"
        pidfile = self.root / "refused.pid"
        wrapper = self.root / "agy-wrapper"
        wrapper.write_text(
            "#!/bin/sh\n"
            f"'{sys.executable}' -c 'import os, sys, time; open(sys.argv[1], \"w\").write(str(os.getpid())); "
            f"time.sleep(300)' '{pidfile}' {marker} &\n"
            f"'{self.agy}' \"$@\"\nexit $?\n")
        wrapper.chmod(0o700)
        self.configure(no_listener_log=True)
        helper = self.fixture("refusing-helper", "refusing_helper")
        process = self.launch(f"--agy={wrapper}", "--startup-timeout=1",
                              helper=(str(helper), marker, str(HELPER)))
        agy = self.record("agy")
        _, stderr, survivors = self.finish(process, timeout=10, settle=0.3)
        refused = int(pidfile.read_text())
        self.assertEqual(process.returncode, 1, stderr)
        self.assertEqual(stderr.strip().splitlines()[-1][:len("agy-pair: timed out")],
                         "agy-pair: timed out", stderr)
        self.assertNotIn("Operation not permitted", stderr)
        self.assertIn("agy-pair: stopped", stderr)
        self.assertEqual(stderr.count(f"not allowed to signal process {refused},"), 1, stderr)
        # Only the process that refused is left; agy is not.
        self.assertEqual(survivors, [refused], stderr)
        self.assert_stopped(agy)

    @linux_only
    def test_wrapped_agy_stops_when_its_logged_port_is_not_its_listener(self):
        # agy's log names a port where another account listens, so the
        # helper finds no listener of its own and times out. With no socket
        # to identify agy by, it must stop everything under the wrapper, not
        # only the wrapper, which does not forward signals.
        self.addCleanup(self.kill_survivors)
        decoy = Decoy(self)
        wrapper = self.root / "agy-wrapper"
        wrapper.write_text(f"#!/bin/sh\n'{self.agy}' \"$@\"\nexit $?\n")
        wrapper.chmod(0o700)
        self.configure(log_port=decoy.port)
        process = self.launch(f"--agy={wrapper}", "--startup-timeout=0.5")
        agy = self.record("agy")
        _, stderr, survivors = self.finish(process, timeout=5)
        self.assertEqual(survivors, [], stderr)
        self.assertEqual(process.returncode, 1, stderr)
        self.assertIn(f"agy is not listening on port {decoy.port}, which its log names", stderr)
        self.assertEqual(decoy.connections, 0)
        self.assert_stopped(agy)

    @linux_only
    def test_stopping_a_preparing_wrapper_shares_one_stop_deadline(self):
        # Each of the wrapper's commands exits at once on SIGTERM. Cleanup
        # must count each as finished when it exits, even before the wrapper
        # reaps it, and all of them share one --stop-timeout: with the default
        # five seconds, stopping takes far less than one timeout, let alone
        # one per child.
        self.addCleanup(self.kill_survivors)
        wrapper = self.root / "agy-wrapper"
        pidfile = self.root / "wrapper.pid"
        wrapper.write_text(f"#!/bin/sh\necho $$ > '{pidfile}'\nsleep 10 & sleep 10 & wait\n"
                           f"exec '{self.agy}' \"$@\"\n")
        wrapper.chmod(0o700)
        startup, stop = 1.0, 5.0
        started = time.monotonic()
        process = self.launch(f"--agy={wrapper}", f"--startup-timeout={startup}", f"--stop-timeout={stop}")
        process.wait(timeout=30)
        elapsed = time.monotonic() - started
        _, stderr, survivors = self.finish(process, timeout=5)
        self.assertEqual(survivors, [], stderr)
        self.assertEqual(process.returncode, 1, stderr)
        self.assertLess(elapsed - startup, stop / 2, stderr)
        self.assertFalse((self.root / "agy.json").exists())
        with self.assertRaises(ProcessLookupError):
            os.kill(int(pidfile.read_text()), 0)

    def reset_store(self, mode=0o700):
        store = self.root / "store"
        for path in (store, self.logs):
            shutil.rmtree(path, ignore_errors=True)
        for name in ("agy.json", "reader.json"):
            (self.root / name).unlink(missing_ok=True)
        store.mkdir()
        store.chmod(mode)
        return store

    def test_agy_log_stays_private_where_another_account_could_change_it(self):
        def group_writable_log(store):
            (store / "log").mkdir()
            (store / "log").chmod(0o770)

        def world_writable_store(store):
            store.chmod(0o777)

        def symlinked_log(store):
            (store / "elsewhere").mkdir(mode=0o700)
            (store / "log").symlink_to(store / "elsewhere")

        for prepare in (group_writable_log, world_writable_store, symlinked_log):
            with self.subTest(prepare.__name__):
                store = self.reset_store()
                prepare(store)
                process = self.launch()
                agy, reader = self.record("agy"), self.record("reader")
                self.assertEqual(reader["status"], 200)
                _, stderr = process.communicate("exit 0\n", timeout=5)
                self.assertEqual(process.returncode, 0, stderr)
                self.assertIn("agy logs to this run's private directory instead", stderr)
                log_dir, = self.logs.iterdir()
                agy_log = log_dir / "agy.log"
                self.assertEqual(Path(agy["log_file"]).resolve(), agy_log.resolve())
                self.assertFalse(agy_log.is_symlink())
                self.assertEqual(agy_log.stat().st_mode & 0o777, 0o600)
                self.assertIn("listening on random port at", agy_log.read_text())
                self.assertEqual(list(store.rglob("cli-pair-*")), [])
                self.assert_stopped(agy, reader)

    def test_a_sticky_store_still_holds_agy_log(self):
        # Like /tmp: others may add entries, but not rename or remove yours.
        self.reset_store(0o1777)
        process = self.launch()
        agy, reader = self.record("agy"), self.record("reader")
        _, stderr = process.communicate("exit 0\n", timeout=5)
        self.assertEqual(process.returncode, 0, stderr)
        self.assertNotIn("private directory instead", stderr)
        self.assert_agy_log_under_root(agy, next(self.logs.iterdir()))
        self.assert_stopped(agy, reader)

    def test_a_store_that_cannot_hold_agy_log_still_pairs(self):
        # The reader writes only under conversations/, so a store whose top
        # level, or whose log directory, you made read-only must not stop the
        # pair. agy's log stays in the run directory, as it does where another
        # account could change the store.
        if os.geteuid() == 0:
            self.skipTest("root may write to any directory")

        def read_only_store(store):
            (store / "conversations").mkdir(mode=0o700)
            store.chmod(0o555)
            return f"cannot create {store.resolve() / 'log'} ("

        def read_only_log(store):
            (store / "log").mkdir(mode=0o500)
            return f"cannot create agy's log in {store.resolve() / 'log'} ("

        for prepare in (read_only_store, read_only_log):
            with self.subTest(prepare.__name__):
                store = self.reset_store()
                try:
                    cause = prepare(store)
                    process = self.launch()
                    agy, reader = self.record("agy"), self.record("reader")
                    self.assertEqual(reader["status"], 200)
                    _, stderr = process.communicate("exit 0\n", timeout=5)
                    self.assertEqual(process.returncode, 0, stderr)
                    self.assertIn(cause, stderr)
                    self.assertIn("agy logs to this run's private directory instead", stderr)
                    log_dir, = self.logs.iterdir()
                    agy_log = log_dir / "agy.log"
                    self.assertEqual(Path(agy["log_file"]).resolve(), agy_log.resolve())
                    self.assertFalse(agy_log.is_symlink())
                    self.assertEqual(agy_log.stat().st_mode & 0o777, 0o600)
                    self.assertIn("listening on random port at", agy_log.read_text())
                    self.assertEqual(list(store.rglob("cli-pair-*")), [])
                    self.assert_stopped(agy, reader)
                finally:
                    # Let reset_store and the temporary directory remove it.
                    store.chmod(0o700)
                    if (store / "log").is_dir():
                        (store / "log").chmod(0o700)

    def test_a_log_dir_another_account_could_change_is_refused(self):
        # The helper's mkdir leaves an existing directory's mode alone.
        self.logs.mkdir()
        self.logs.chmod(0o777)
        process = self.launch()
        _, stderr = process.communicate(timeout=5)
        self.assertEqual(process.returncode, 1, stderr)
        self.assertIn("choose a private --log-dir", stderr)
        self.assertFalse((self.root / "agy.json").exists())
        self.assertEqual(list(self.logs.iterdir()), [])

    @linux_only
    def test_no_flag_mode_catches_the_first_tool_token(self):
        self.configure(tools=[
            {"token": None},                                   # launch-time child, no credentials
            {"delay": 0.3, "token": "wrong"},                  # agy rejects it
            {"delay": 0.3, "token": "right", "port": "other"},  # another daemon's address
            {"delay": 0.8, "token": "right", "depth": 2},      # first real tool, a grandchild
        ])
        # The first tool arrives after the startup timeout, which bounds only
        # agy's listener in this mode.
        process = self.launch("--token-mode=no-flag", "--startup-timeout=0.4")
        agy, reader = self.record("agy"), self.record("reader")
        token = (self.root / "agy-token").read_text()
        self.assertEqual(agy["first_arg"], "--log-file")
        self.assertFalse(agy["env_token"])
        self.assertEqual(reader["token_hash"], agy["token_hash"])
        self.assertEqual(reader["status"], 200)
        self.assertIn("spaces ; $ are literal", agy["args"])
        self.assertFalse(visible_in_any_arguments(token))
        stdout, stderr = process.communicate("exit 0\n", timeout=5)
        self.assertEqual(process.returncode, 0, stderr)
        self.assertIn("sync is pending until agy runs its first tool", stderr)
        self.assertNotIn(token, stdout + stderr)
        self.assert_stopped(agy, reader)
        self.assert_agy_log_under_root(agy, next(self.logs.iterdir()))

    @linux_only
    def test_no_flag_mode_retries_a_found_token_after_a_transient_failure(self):
        # agy's first answer to the right token is a 503, or comes too late
        # for the helper's 0.5 s timeout, and arrives only after the tool that
        # exported the token has exited. No other tool runs. The helper must
        # keep the token it found and try it again.
        for flaky in ({"status": 503}, {"delay": 0.8}):
            with self.subTest(flaky=flaky):
                for name in ("agy.json", "reader.json", "flaky.json"):
                    (self.root / name).unlink(missing_ok=True)
                self.configure(tools=[{"delay": 0.2, "token": "right"}], flaky_auth=flaky)
                process = self.launch("--token-mode=no-flag")
                agy, reader = self.record("agy"), self.record("reader")
                flaky_record = json.loads((self.root / "flaky.json").read_text())
                self.assertEqual(flaky_record, {"tools": 1, "running": 0})
                self.assertEqual(reader["token_hash"], agy["token_hash"])
                self.assertEqual(reader["status"], 200)
                _, stderr = process.communicate("exit 0\n", timeout=5)
                self.assertEqual(process.returncode, 0, stderr)
                self.assert_stopped(agy, reader)

    @linux_only
    def test_no_flag_mode_without_a_tool_never_starts_the_reader(self):
        process = self.launch("--token-mode=no-flag")
        agy = self.record("agy")
        time.sleep(0.3)
        _, stderr = process.communicate("exit 5\n", timeout=5)
        self.assertEqual(process.returncode, 5, stderr)
        self.assertIn("sync is pending", stderr)
        self.assertIn("before running a tool", stderr)
        self.assertFalse((self.root / "reader.json").exists())
        self.assert_stopped(agy)

    @needs_native_agy
    def test_shim_mode_hides_the_token_from_process_arguments(self):
        shim = self.build_library("shim.so", SHIM_SOURCE)
        native = self.build_native_agy()
        process = self.launch("--token-mode=shim", f"--agy={native}", f"--shim={shim}",
                              env_overrides={"LD_PRELOAD": None})
        agy, reader = self.record("agy"), self.record("reader")
        token = (self.root / "agy-token").read_text()
        # agy parsed the real flag, but its visible arguments hold only the
        # placeholder, and neither agy's live environment nor its tools keep
        # the shim or the token. Its original environment block keeps the
        # token on purpose (see below).
        self.assertEqual(agy["first_arg"], "--csrf_token")
        self.assertFalse(agy["cmdline_has_token"])
        self.assertTrue(agy["environ_has_token"])
        self.assertFalse(agy["env_token"])
        self.assertFalse(agy["preload_env"])
        self.assertFalse(agy["pair_env"])
        self.assertFalse(agy["pair_exe_env"])
        self.assertEqual(agy["child_env"], [False, False, False])
        # agy's /proc/<pid>/environ, read from outside as the same user,
        # shows the block agy started with. README documents exactly this:
        # LD_PRELOAD with the library's path, ANTIGRAVITY_PAIR_EXE with agy's
        # path, and the token itself, which agy-reader's discovery reads there
        # (cliPairToken) so a watcher started separately can authenticate.
        # Update both together.
        listed, found = environment_block(
            agy["pid"], (b"LD_PRELOAD", b"ANTIGRAVITY_PAIR_TOKEN", b"ANTIGRAVITY_PAIR_EXE"), [token])
        self.assertEqual(found, [True])
        listed = [entry.replace(token.encode(), b"<token>") for entry in listed]
        self.assertEqual(listed, [b"ANTIGRAVITY_PAIR_EXE=" + str(native).encode(),
                                  b"ANTIGRAVITY_PAIR_TOKEN=<token>", b"LD_PRELOAD=" + str(shim).encode()])
        self.assertIn("spaces ; $ are literal", agy["args"])
        self.assertEqual(reader["token_hash"], agy["token_hash"])
        self.assertEqual(reader["status"], 200)
        self.assertFalse(visible_in_any_arguments(token))
        stdout, stderr = process.communicate("exit 3\n", timeout=5)
        self.assertEqual(process.returncode, 3, stderr)
        self.assertNotIn(token, stdout + stderr)
        self.assertNotIn("LD_PRELOAD", stderr)
        self.assert_stopped(agy, reader)
        self.assert_agy_log_under_root(agy, next(self.logs.iterdir()))

    @needs_native_agy
    def test_agy_keeps_the_name_it_was_launched_by(self):
        # agy-reader's discovery recognizes agy by the name in its first
        # argument. Through a symlink named agy whose target has a versioned
        # name, agy must still show that name, while the helper runs and the
        # shim checks the resolved file.
        shim = self.build_library("shim.so", SHIM_SOURCE)
        native = self.build_native_agy()
        link = self.root / "links" / "agy"
        link.parent.mkdir()
        link.symlink_to(native)
        for mode in ("argv", "shim"):
            with self.subTest(mode=mode):
                for name in ("agy.json", "reader.json"):
                    (self.root / name).unlink(missing_ok=True)
                process = self.launch(f"--token-mode={mode}", f"--agy={link}", f"--shim={shim}",
                                      env_overrides={"LD_PRELOAD": None})
                agy, reader = self.record("agy"), self.record("reader")
                first = (PROC / str(agy["pid"]) / "cmdline").read_bytes().split(b"\0")[0]
                self.assertEqual(first, str(link).encode())
                self.assertEqual(os.readlink(PROC / str(agy["pid"]) / "exe"), str(native))
                self.assertEqual(agy["first_arg"], "--csrf_token")
                self.assertEqual(reader["status"], 200)
                _, stderr = process.communicate("exit 0\n", timeout=5)
                self.assertEqual(process.returncode, 0, stderr)
                self.assert_stopped(agy, reader)

    @needs_native_agy
    def test_shim_mode_replaces_an_inherited_ld_preload(self):
        shim = self.build_library("shim.so", SHIM_SOURCE)
        native = self.build_native_agy()
        source = self.root / "user-preload.c"
        source.write_text("int agy_pair_test_user_preload;\n")
        user_library = self.build_library("user-preload.so", source)
        process = self.launch("--token-mode=shim", f"--agy={native}", f"--shim={shim}",
                              env_overrides={"LD_PRELOAD": str(user_library)})
        agy, reader = self.record("agy"), self.record("reader")
        # agy started with the shim alone, so neither agy nor its tools load
        # the user's library or inherit LD_PRELOAD. The reader keeps it.
        listed, found = environment_block(agy["pid"], (b"LD_PRELOAD",), [str(user_library)])
        self.assertEqual(listed, [b"LD_PRELOAD=" + str(shim).encode()])
        self.assertEqual(found, [False])
        self.assertFalse(agy["preload_env"])
        self.assertEqual(agy["child_env"], [False, False, False])
        self.assertEqual(reader["preload"], str(user_library))
        self.assertEqual(reader["status"], 200)
        _, stderr = process.communicate("exit 0\n", timeout=5)
        self.assertEqual(process.returncode, 0, stderr)
        self.assertIn("replaces your LD_PRELOAD for agy and its tools; the reader keeps it", stderr)
        self.assert_stopped(agy, reader)

    @linux_only
    def test_shim_mode_fails_closed(self):
        result = subprocess.run([sys.executable, str(HELPER), "run", *self.base, "--token-mode=shim",
                                 f"--shim={self.root / 'missing.so'}", *self.agy_args],
                                text=True, capture_output=True)
        self.assertEqual(result.returncode, 2)
        self.assertIn("make shim", result.stderr)
        self.assertFalse((self.root / "agy-token").exists())

    @linux_only
    def test_shim_mode_refuses_a_library_path_ld_so_would_split(self):
        # ld.so splits LD_PRELOAD at spaces and colons, with no escaping, so
        # the library could never load from such a path. The helper must say
        # why before it starts anything, not blame the library afterwards.
        for directory in ("with space", "with:colon"):
            with self.subTest(directory=directory):
                shim = self.root / directory / "shim.so"
                shim.parent.mkdir()
                shim.write_bytes(b"")
                result = subprocess.run([sys.executable, str(HELPER), "run", *self.base,
                                         "--token-mode=shim", f"--shim={shim}", *self.agy_args],
                                        text=True, capture_output=True, timeout=10)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertIn("contains a space or colon, which LD_PRELOAD cannot hold", result.stderr)
                self.assertIn("copy the library to a path without them", result.stderr)
                self.assertFalse((self.root / "agy-token").exists())
                self.assertFalse((self.root / "reader.json").exists())

    @needs_native_agy
    def test_shim_mode_fails_closed_when_the_library_does_not_load(self):
        # A library that does not load leaves agy with the unknown placeholder.
        native = self.build_native_agy()
        broken = self.root / "broken.so"
        broken.write_bytes(b"not a shared object")
        process = self.launch("--token-mode=shim", f"--agy={native}", f"--shim={broken}")
        _, stderr = process.communicate(timeout=5)
        self.assertEqual(process.returncode, 2, stderr)
        self.assertIn("placeholder flag", stderr)
        self.assertFalse((self.root / "agy.json").exists())
        self.assertFalse((self.root / "reader.json").exists())

    @needs_cc
    def test_shim_mode_refuses_a_wrapper_before_starting_anything(self):
        # The shim would load into a wrapper first and hand it the real flag,
        # which the wrapper would pass to agy in its exec arguments. The
        # helper must refuse before it starts any process.
        shim = self.build_library("shim.so", SHIM_SOURCE)
        marker = self.root / "wrapper-ran"
        target = self.root / "mark"
        target.write_text(f"#!/bin/sh\n: > '{marker}'\n")
        target.chmod(0o700)
        for kind, wrapper in self.wrappers(target).items():
            with self.subTest(wrapper=kind):
                result = subprocess.run(
                    [sys.executable, str(HELPER), "run", *self.base, "--token-mode=shim",
                     f"--agy={wrapper}", f"--shim={shim}", *self.agy_args],
                    text=True, capture_output=True, timeout=10,
                )
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertIn("agy executable itself", result.stderr)
                self.assertFalse(marker.exists())
                self.assertFalse((self.root / "agy-token").exists())
                self.assertFalse((self.root / "reader.json").exists())

    @needs_cc
    def test_shim_swaps_the_token_only_in_the_checked_executable(self):
        # Even if a wrapper got past the helper's check, the shim acts only
        # in the file named by ANTIGRAVITY_PAIR_EXE. Here that file is the
        # program at the end of the chain, so a swap in the wrapper would
        # reach it as an exec argument; the report shows what it received.
        shim = self.build_library("shim.so", SHIM_SOURCE)
        report = self.build_program("report", REPORT_C)
        output = self.root / "report.txt"
        token = "dummy-" + uuid.uuid4().hex
        env = {"PATH": "/usr/bin:/bin", "LD_PRELOAD": str(shim), "ANTIGRAVITY_PAIR_TOKEN": token,
               "ANTIGRAVITY_PAIR_EXE": str(report), "REPORT": str(output), "REPORT_TOKEN": token}

        def received(program, *args):
            output.unlink(missing_ok=True)
            subprocess.run([str(program), *args], env=env, check=True, timeout=10)
            lines = output.read_text().splitlines()
            return {"token": any(token in line for line in lines[:-2]),
                    "placeholder": "--pair_token_slot=1" in lines,
                    "environ": lines[-2], "inherited": lines[-1]}

        # Positive control: run directly, the checked program gets the real
        # flag in its own memory, and inherits none of the shim's variables.
        # Its original environment block keeps the token, which is where
        # agy-reader's discovery reads it for a separately started watcher.
        self.assertEqual(received(report, "--pair_token_slot=1", "x"),
                         {"token": True, "placeholder": False, "environ": "environ:1",
                          "inherited": "inherited:000"})
        # Without the placeholder nothing is swapped, so the token's bytes are
        # cleared even in the checked program.
        self.assertEqual(received(report, "x"),
                         {"token": False, "placeholder": False, "environ": "environ:0",
                          "inherited": "inherited:000"})
        for kind, wrapper in self.wrappers(report).items():
            with self.subTest(wrapper=kind):
                self.assertEqual(received(wrapper, "--pair_token_slot=1", "x"),
                                 {"token": False, "placeholder": True, "environ": "environ:0",
                                  "inherited": "inherited:000"})

    def test_managed_arguments_cannot_be_overridden(self):
        for arg in ("--csrf_token=bad", "--log-file=/tmp/wrong", "--pair_token_slot=bad"):
            result = subprocess.run([sys.executable, str(HELPER), "run", *self.base, "--", arg],
                                    text=True, capture_output=True)
            self.assertEqual(result.returncode, 2)
            self.assertNotIn("=bad", result.stderr)
            self.assertFalse((self.root / "agy.json").exists())

    @unittest.skipUnless(shutil.which("tmux"), "tmux is optional")
    def test_detached_start_restart_status_and_stop(self):
        session = "test-" + uuid.uuid4().hex

        def control(mode, *extra):
            return subprocess.run([sys.executable, str(HELPER), mode, "--session", session, *extra],
                                  text=True, capture_output=True, timeout=10)

        self.addCleanup(control, "stop")
        result = control("start", *self.base, *self.agy_args)
        self.assertEqual(result.returncode, 0, result.stderr)
        agy, reader = self.record("agy"), self.record("reader")
        self.assertIn("running", control("status").stdout)
        self.assertNotEqual(control("start", *self.base, *self.agy_args).returncode, 0)
        prefix = subprocess.run(
            [sys.executable, str(HELPER), "stop", "--session", session[:-1]],
            text=True, capture_output=True, timeout=10,
        )
        self.assertNotEqual(prefix.returncode, 0)
        self.assertIn("running", control("status").stdout)
        # Attaching and opening another window must not change which process
        # stop/restart signals. The extra window belongs to this test session.
        subprocess.run(["tmux", "-L", "agy-reader-pairs", "new-window", "-t", session,
                        "sleep 86400"], check=True, capture_output=True)
        for name in ("agy.json", "reader.json"):
            (self.root / name).unlink()
        result = control("restart")
        self.assertEqual(result.returncode, 0, result.stderr)
        next_agy, next_reader = self.record("agy"), self.record("reader")
        self.assert_stopped(agy, reader)
        self.assertNotEqual(agy["token_hash"], next_agy["token_hash"])
        self.assertEqual(next_reader["status"], 200)
        result = control("stop")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_stopped(next_agy, next_reader)
        self.assertNotEqual(control("status").returncode, 0)

    @linux_only
    @unittest.skipUnless(shutil.which("tmux"), "tmux is optional")
    def test_detached_start_keeps_the_token_mode(self):
        self.configure(tools=[{"delay": 0.2, "token": "right"}])
        session = "test-" + uuid.uuid4().hex

        def control(mode, *extra):
            return subprocess.run([sys.executable, str(HELPER), mode, "--session", session, *extra],
                                  text=True, capture_output=True, timeout=10)

        self.addCleanup(control, "stop")
        result = control("start", *self.base, "--token-mode=no-flag", *self.agy_args)
        self.assertEqual(result.returncode, 0, result.stderr)
        agy, reader = self.record("agy"), self.record("reader")
        self.assertEqual(agy["first_arg"], "--log-file")
        self.assertEqual(reader["token_hash"], agy["token_hash"])
        self.assertEqual(control("stop").returncode, 0)
        self.assert_stopped(agy, reader)

    @unittest.skipUnless(shutil.which("tmux"), "tmux is optional")
    def test_detached_start_keeps_an_agy_path_through_a_directory_symlink(self):
        # n/tools is a symlink to releases/v1/tools, so n/tools/../agy names
        # releases/v1/agy. Collapsing ".." first would name n/agy instead,
        # which does not exist, and the detached supervisor would not start.
        release = self.root / "releases" / "v1"
        (release / "tools").mkdir(parents=True)
        (release / "agy").symlink_to(self.agy)
        (self.root / "n").mkdir()
        (self.root / "n" / "tools").symlink_to(release / "tools")
        agy_path = f"{self.root}/n/tools/../agy"
        base = list(self.base)
        base[base.index("--agy") + 1] = agy_path
        session = "test-" + uuid.uuid4().hex

        def control(mode, *extra):
            return subprocess.run([sys.executable, str(HELPER), mode, "--session", session, *extra],
                                  text=True, capture_output=True, timeout=10)

        self.addCleanup(control, "stop")
        result = control("start", *base, *self.agy_args)
        self.assertEqual(result.returncode, 0, result.stderr)
        agy, reader = self.record("agy"), self.record("reader")
        self.assertEqual(reader["status"], 200)
        self.assertIn("running", control("status").stdout)
        self.assertEqual(control("stop").returncode, 0)
        self.assert_stopped(agy, reader)

    @unittest.skipUnless(shutil.which("tmux"), "tmux is optional")
    def test_detached_pair_runs_without_the_login_shell(self):
        # tmux runs a single command string with the login shell from SHELL,
        # which may be fish or another shell that parses POSIX quoting
        # differently. agy's arguments must reach it unchanged, through start
        # and restart, without any shell. This tmux server is private to the
        # test, so its default shell is the stand-in below, which refuses to
        # run anything. Its socket goes in a short directory of its own: the
        # test's temporary directory can be too deep for a socket path, as
        # macOS's per-user one is.
        tmux_dir = tempfile.mkdtemp(prefix="agyt-", dir="/tmp" if os.path.isdir("/tmp") else None)
        self.addCleanup(shutil.rmtree, tmux_dir, ignore_errors=True)
        shell = self.root / "login-shell"
        shell.write_text(f"#!/bin/sh\necho \"$*\" >> '{self.root / 'shell-ran'}'\nexit 97\n")
        shell.chmod(0o700)
        env = dict(os.environ, TMUX_TMPDIR=tmux_dir, SHELL=str(shell))
        env.pop("TMUX", None)
        self.addCleanup(subprocess.run, ["tmux", "-L", "agy-reader-pairs", "kill-server"],
                        env=env, capture_output=True)
        session = "test-" + uuid.uuid4().hex
        # tmux itself ends a command at an argument ending in ";", so those
        # arguments must reach agy whole too.
        agy_args = ["--", f"--fixture={self.root}", "--print", "match \\\\d+ here", "C:\\dir\\",
                    "it's \"quoted\"", "Explain SELECT 1;", ";", "a\\;", "--sandbox"]

        def control(mode, *extra):
            return subprocess.run([sys.executable, str(HELPER), mode, "--session", session, *extra],
                                  text=True, capture_output=True, timeout=10, env=env)

        self.addCleanup(control, "stop")
        result = control("start", *self.base, *agy_args)
        self.assertEqual(result.returncode, 0, result.stderr)
        agy, reader = self.record("agy"), self.record("reader")
        self.assertEqual(agy["args"], agy_args[1:])
        self.assertEqual(reader["status"], 200)
        for name in ("agy.json", "reader.json"):
            (self.root / name).unlink()
        result = control("restart")
        self.assertEqual(result.returncode, 0, result.stderr)
        next_agy, next_reader = self.record("agy"), self.record("reader")
        self.assertEqual(next_agy["args"], agy_args[1:])
        self.assertEqual(control("stop").returncode, 0)
        self.assert_stopped(agy, reader, next_agy, next_reader)
        self.assertFalse((self.root / "shell-ran").exists())

    @unittest.skipUnless(shutil.which("tmux"), "tmux is optional")
    def test_detached_failure_remains_inspectable(self):
        self.configure(reader_exit=23)
        session = "test-" + uuid.uuid4().hex

        def control(mode, *extra):
            return subprocess.run([sys.executable, str(HELPER), mode, "--session", session, *extra],
                                  text=True, capture_output=True, timeout=10)

        self.addCleanup(control, "stop")
        result = control("start", *self.base, *self.agy_args)
        self.assertEqual(result.returncode, 0, result.stderr)
        agy, reader = self.record("agy"), self.record("reader")
        deadline = time.monotonic() + 5
        while True:
            status = control("status")
            if "exited" in status.stdout or time.monotonic() > deadline:
                break
            time.sleep(0.02)
        self.assertIn("exited (status 1)", status.stdout)
        self.assertNotEqual(status.returncode, 0)
        self.assert_stopped(agy, reader)
        self.assertEqual(control("stop").returncode, 0)


def load_helper():
    spec = importlib.util.spec_from_file_location("agy_pair", HELPER)
    helper = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(helper)
    return helper


class PaneStatus(unittest.TestCase):
    def test_live_and_dead_panes_preserve_empty_fields(self):
        helper = load_helper()
        for output, expected in (
            ("%3\t0\t1234\t\n", ["%3", "0", "1234", ""]),
            ("%3\t1\t1234\t1\n", ["%3", "1", "1234", "1"]),
            ("%3\t1\t\t1\n", ["%3", "1", "", "1"]),
            ("%3\t1\t\t\n", ["%3", "1", "", ""]),
        ):
            with self.subTest(output=output), mock.patch.object(helper, "tmux") as tmux:
                tmux.return_value.stdout = output
                self.assertEqual(helper.pane_status("%3"), expected)

    def test_malformed_or_live_pidless_panes_are_refused(self):
        helper = load_helper()
        for output in ("", "%3\t1\n", "%3\t1\t\n", "%3\t0\t\t\n",
                       "%3\t0\tbad\t\n", "%3\t0\t0\t\n", "%3\t0\t-1\t\n",
                       "%3\t1\tbad\t1\n", "bad\t1\t\t1\n", "%3\t2\t1234\t\n",
                       "%3\t1\t\t1\textra\n"):
            with self.subTest(output=output), mock.patch.object(helper, "tmux") as tmux:
                tmux.return_value.stdout = output
                with self.assertRaisesRegex(RuntimeError, "original tmux pane is missing"):
                    helper.pane_status("%3")

    def test_dead_pane_controls_never_signal_an_empty_pid(self):
        helper = load_helper()
        for mode in ("status", "stop", "restart"):
            with self.subTest(mode=mode), mock.patch.object(helper, "tmux") as tmux, \
                    mock.patch.object(helper.os, "kill") as kill, \
                    contextlib.redirect_stdout(io.StringIO()) as output:
                tmux.side_effect = [
                    subprocess.CompletedProcess([], 0, stdout="test\t$1\n"),
                    subprocess.CompletedProcess([], 0, stdout="1\n"),
                    subprocess.CompletedProcess([], 0, stdout="%3\n"),
                    subprocess.CompletedProcess([], 0, stdout="%3\t1\t\t\n"),
                    subprocess.CompletedProcess([], 0, stdout=""),
                ]
                result = helper.detached(helper.argparse.Namespace(mode=mode, session="test"))
                self.assertEqual(result, 1 if mode == "status" else 0)
                kill.assert_not_called()
                if mode == "status":
                    self.assertIn("exited (status unknown)", output.getvalue())
                elif mode == "stop":
                    tmux.assert_called_with("kill-session", "-t", "$1")
                else:
                    tmux.assert_called_with("respawn-pane", "-t", "%3")


# Stands in for agy in StopTree: it ignores SIGTERM like a busy agy, and
# starts a tool, which records any SIGTERM it gets, each time a request file
# appears.
STOP_TREE_AGY = r"""
import os, signal, subprocess, sys, time
signal.signal(signal.SIGTERM, signal.SIG_IGN)
root, started = sys.argv[1], 0
# The tool writes its PID under a temporary name and renames it into place,
# so a ready file is always complete.
tool = ("import os, signal, sys, time\n"
        "signal.signal(signal.SIGTERM, lambda *_: open(sys.argv[1], 'w').close())\n"
        "with open(sys.argv[2] + '.tmp', 'w') as out: out.write(str(os.getpid()))\n"
        "os.rename(sys.argv[2] + '.tmp', sys.argv[2])\n"
        "while True: time.sleep(1)\n")
while True:
    request = os.path.join(root, f"request-{started}")
    if os.path.exists(request):
        subprocess.Popen([sys.executable, "-c", tool, os.path.join(root, f"term-{started}"),
                          os.path.join(root, f"ready-{started}")], start_new_session=True)
        started += 1
    time.sleep(0.005)
"""


class StopTree(unittest.TestCase):
    # stop_tree run in this process against real processes, with the race
    # made certain: agy starts a tool while the helper is looking at the tree.

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="agy-pair-stop-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.tools = []
        self.addCleanup(self.kill_tools)

    def stop(self, process):
        if process.poll() is None:
            process.kill()
            process.wait()

    def kill_tools(self):
        for pid in self.tools:
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass

    def start_tool(self, agy):
        # Ask agy for one more tool and wait until it is agy's child and has
        # its SIGTERM handler, so a SIGTERM it gets is recorded, not fatal.
        index = len(self.tools)
        (self.root / f"request-{index}").touch()
        ready = self.root / f"ready-{index}"
        deadline = time.monotonic() + 5
        while not ready.exists():
            self.assertLess(time.monotonic(), deadline, "agy did not start its tool")
            time.sleep(0.005)
        self.tools.append(int(ready.read_text()))
        return self.tools[-1]

    def assert_tools_untouched(self):
        self.assertTrue(self.tools)
        for index, pid in enumerate(self.tools):
            self.assertFalse((self.root / f"term-{index}").exists(), f"tool {index} got SIGTERM")
            try:
                state = (PROC / str(pid) / "stat").read_text().rsplit(")", 1)[1].split()[0]
            except FileNotFoundError:
                state = "gone"
            self.assertNotIn(state, ("T", "t", "Z", "X", "gone"), f"tool {index} was stopped or killed")

    def run_stop_tree(self, root, agy):
        helper = load_helper()
        read_text, started = Path.read_text, []

        def look(path, *args, **kwargs):
            # The helper's first look at any child list sees the tree as it
            # was; agy starts a tool before the helper's next look.
            text = read_text(path, *args, **kwargs)
            if str(path).endswith("/children") and not started:
                started.append(self.start_tool(agy))
            return text

        with mock.patch.object(Path, "read_text", look):
            helper.stop_tree(root.pid, agy, time.monotonic() + 0.3)
        root.wait(timeout=5)

    @linux_only
    def test_a_tool_started_during_discovery_is_never_signalled(self):
        for launch in ("wrapper", "direct"):
            with self.subTest(launch=launch):
                self.kill_tools()
                self.tools = []
                for stale in self.root.iterdir():
                    stale.unlink()
                command = [sys.executable, "-c", STOP_TREE_AGY, str(self.root)]
                if launch == "wrapper":
                    root = subprocess.Popen(["/bin/sh", "-c", '"$@"; exit $?', "sh", *command])
                else:
                    root = subprocess.Popen(command)
                self.addCleanup(lambda root=root: root.poll() is None and root.kill())
                agy = root.pid if launch == "direct" else None
                deadline = time.monotonic() + 5
                while agy is None:
                    agy = next(iter(load_helper().descendants(root.pid)), None)
                    self.assertLess(time.monotonic(), deadline, "agy did not start")
                    time.sleep(0.01)
                # A tool already running when shutdown starts is left alone too.
                self.start_tool(agy)
                self.run_stop_tree(root, agy)
                # A wrapped agy may stay a zombie until init reaps it.
                deadline = time.monotonic() + 5
                while (PROC / str(agy)).exists():
                    self.assertLess(time.monotonic(), deadline, "agy is still running")
                    time.sleep(0.01)
                self.assert_tools_untouched()


    @linux_only
    def test_a_parent_that_exits_just_after_its_children_are_read_keeps_them_owned(self):
        # The middle process dies right after the helper reads its child
        # list, so its child leaves the launched tree before the next look.
        # That child was in the list the helper read, so it must be owned and
        # stopped all the same.
        root = subprocess.Popen(["/bin/sh", "-c", '/bin/sh -c "sleep 30; exit"; sleep 30; exit'])
        self.addCleanup(lambda: root.poll() is None and root.kill())
        helper = load_helper()
        deadline = time.monotonic() + 5
        while len(found := helper.descendants(root.pid)) < 2:
            self.assertLess(time.monotonic(), deadline, "the tree did not start")
            time.sleep(0.01)
        middle, leaf = found[0], found[1]
        self.tools.append(leaf)  # killed on cleanup if it survives
        self.assertIn(leaf, helper.descendants(middle))
        read_text, killed = Path.read_text, []

        def look(path, *args, **kwargs):
            text = read_text(path, *args, **kwargs)
            if str(path).endswith("/children") and str(leaf) in text.split() and not killed:
                os.kill(middle, signal.SIGKILL)
                killed.append(middle)
                deadline = time.monotonic() + 5
                while (PROC / str(leaf) / "stat").read_text().rsplit(")", 1)[1].split()[1] == str(middle):
                    self.assertLess(time.monotonic(), deadline, "the leaf was not orphaned")
                    time.sleep(0.005)
            return text

        with mock.patch.object(Path, "read_text", look):
            helper.stop_tree(root.pid, None, time.monotonic() + 1)
        root.wait(timeout=5)
        self.assertEqual(killed, [middle])
        deadline = time.monotonic() + 5
        while (PROC / str(leaf)).exists():
            self.assertLess(time.monotonic(), deadline, "the orphaned leaf is still running")
            time.sleep(0.01)


    @linux_only
    def test_fallback_keeps_children_when_a_new_parent_exits_after_the_scan(self):
        root = subprocess.Popen(["/bin/sh", "-c", '/bin/sh -c "sleep 30; exit"; sleep 30; exit'])
        self.addCleanup(self.stop, root)
        helper = load_helper()
        deadline = time.monotonic() + 5
        while len(found := helper.descendants(root.pid)) < 2:
            self.assertLess(time.monotonic(), deadline, "the tree did not start")
            time.sleep(0.01)
        middle, leaf = found[:2]
        self.tools.extend([middle, leaf])
        self.assertIn(leaf, helper.descendants(middle))
        lister, exists = helper.child_lister, Path.exists
        killed = []

        def scan_then_exit():
            result = lister()
            if not killed:
                # The scan has both identities and their relationship, but
                # stop_tree has not opened a pidfd for the middle process.
                os.kill(middle, signal.SIGKILL)
                killed.append(middle)
                deadline = time.monotonic() + 5
                while (PROC / str(middle)).exists():
                    self.assertLess(time.monotonic(), deadline, "parent was not reaped")
                    time.sleep(0.005)
            return result

        def no_child_lists(path, *args, **kwargs):
            return False if str(path).endswith("/children") else exists(path, *args, **kwargs)

        with (mock.patch.object(helper, "child_lister", scan_then_exit),
              mock.patch.object(Path, "exists", no_child_lists)):
            helper.stop_tree(root.pid, None, time.monotonic() + 0.5)
        root.wait(timeout=5)
        self.assertEqual(killed, [middle])
        deadline = time.monotonic() + 5
        while True:
            try:
                state = (PROC / str(leaf) / "stat").read_text().rsplit(")", 1)[1].split()[0]
            except FileNotFoundError:
                break
            if state in ("Z", "X"):
                break
            self.assertLess(time.monotonic(), deadline, "the scanned orphan is still running")
            time.sleep(0.01)


    @linux_only
    def test_a_newly_held_process_must_match_the_scanned_identity(self):
        helper = load_helper()
        bystander = subprocess.Popen(["sleep", "30"])
        self.addCleanup(self.stop, bystander)
        # Simulate the scan naming an earlier process at this PID. Opening
        # a pidfd now must not turn the replacement into an owned process.
        snapshot = {bystander.pid: helper.start_time(bystander.pid) - 1}
        for pidfds in (True, False):
            with self.subTest(pidfds=pidfds):
                with pidfd_open_patch(self, pidfds):
                    held = helper.Owned(bystander.pid, snapshot=snapshot)
                    try:
                        self.assertEqual(held.start, snapshot[bystander.pid])
                        self.assertTrue(held.finished())
                        held.send(signal.SIGTERM)
                        held.send(signal.SIGKILL)
                    finally:
                        held.close()
                self.assertIsNone(bystander.poll(), "the replacement process was signalled")


    @linux_only
    def test_identity_change_during_construction_stays_finished_without_pidfds(self):
        helper = load_helper()
        bystander = subprocess.Popen(["sleep", "30"])
        self.addCleanup(self.stop, bystander)
        earlier = helper.start_time(bystander.pid) - 1
        # The initial read saw an earlier process, but finished() sees the
        # replacement. Subsequent checks must never revive this ownership.
        with (pidfd_open_patch(self, False),
              mock.patch.object(helper, "start_time", return_value=earlier)):
            held = helper.Owned(bystander.pid)
        try:
            self.assertTrue(held.finished())
            self.assertTrue(held.finished())
            held.send(signal.SIGTERM)
            held.send(signal.SIGKILL)
            self.assertIsNone(bystander.poll(), "the replacement process was signalled")
        finally:
            held.close()


    @linux_only
    def test_the_fallback_scan_never_adopts_children_of_a_reused_pid(self):
        # Without the kernel's per-thread child lists, the helper scans /proc
        # for parent PIDs. A held process that dies, is reaped, and whose PID
        # another process takes before the scan must not hand that process's
        # children to the helper. Real PID reuse cannot be timed, so the scan
        # is shown it: the held process is killed and reaped as the scan
        # starts, and its PID then names an impostor that started later and
        # claims an unrelated process as its child.
        ready = self.root / "held-ready"
        held = subprocess.Popen([sys.executable, "-c", "import signal, sys, time\n"
                                 "signal.signal(signal.SIGTERM, signal.SIG_IGN)\n"
                                 "open(sys.argv[1], 'w').close()\ntime.sleep(30)\n", str(ready)])
        self.addCleanup(self.stop, held)
        deadline = time.monotonic() + 5
        while not ready.exists():
            self.assertLess(time.monotonic(), deadline, "held process did not install its handler")
            time.sleep(0.005)
        (self.root / "hold").touch()
        root = subprocess.Popen(["/bin/sh", "-c", 'while [ -e "$1" ]; do sleep 0.05; done', "sh",
                                 str(self.root / "hold")])
        self.addCleanup(self.stop, root)
        # The held process is the root's child for the helper: the scan reads
        # its parent PID as the root's until the impostor takes over.
        bystander = subprocess.Popen(["sleep", "30"])
        self.addCleanup(self.stop, bystander)
        held_stat, bystander_stat = (f"/proc/{held.pid}/stat", f"/proc/{bystander.pid}/stat")
        read_text, listdir, exists = Path.read_text, os.listdir, Path.exists
        scans, reused = [], []

        def stat_with(text, parent=None, start=None):
            head, tail = text.rsplit(")", 1)
            fields = tail.split()
            if parent is not None:
                fields[1] = str(parent)
            if start is not None:
                fields[19] = str(start)
            return f"{head}) {' '.join(fields)}\n"

        def look(path, *args, **kwargs):
            if str(path) == held_stat:
                if reused:
                    return reused[0]
                return stat_with(read_text(path, *args, **kwargs), parent=root.pid)
            if str(path) == bystander_stat and reused:
                return stat_with(read_text(path, *args, **kwargs), parent=held.pid)
            return read_text(path, *args, **kwargs)

        def scan(path=".", *args, **kwargs):
            entries = listdir(path, *args, **kwargs)
            if path != "/proc":
                return entries
            scans.append(True)
            if len(scans) == 2 and not reused:
                # The held process is owned and running; now it dies and is
                # reaped, and an impostor takes its PID.
                text = read_text(Path(held_stat))
                start = int(text.rsplit(")", 1)[1].split()[19])
                held.kill()
                held.wait()
                reused.append(stat_with(text, parent=1, start=start + 1000))
            if reused and str(held.pid) not in entries:
                entries.append(str(held.pid))
            return entries

        def no_child_lists(path, *args, **kwargs):
            return False if str(path).endswith("/children") else exists(path, *args, **kwargs)

        helper = load_helper()
        with (mock.patch.object(Path, "read_text", look), mock.patch.object(os, "listdir", scan),
              mock.patch.object(Path, "exists", no_child_lists)):
            helper.stop_tree(root.pid, None, time.monotonic() + 0.5)
        root.wait(timeout=5)
        self.assertGreaterEqual(len(scans), 2, "the helper did not use the fallback scan")
        self.assertTrue(reused)
        self.assertIsNone(bystander.poll(), "the helper signalled the reused PID's child")


# Stands in for sudo and for what sudo runs, in RefusedSignals: run as
# `node.py <directory> <name>`, it writes its PID to pid-<name> once ready,
# records any SIGTERM it gets in term-<name> and keeps running, and starts
# another such process named <child> each time req-<name>-<child> appears.
SPAWNING_NODE = r"""
import os, signal, subprocess, sys, time
directory, name = sys.argv[1], sys.argv[2]
signal.signal(signal.SIGTERM, lambda *_: open(os.path.join(directory, f"term-{name}"), "w").close())
with open(os.path.join(directory, f"pid-{name}.tmp"), "w") as out:
    out.write(str(os.getpid()))
os.rename(os.path.join(directory, f"pid-{name}.tmp"), os.path.join(directory, f"pid-{name}"))
started = set()
while True:
    for entry in os.listdir(directory):
        if entry.startswith(f"req-{name}-") and entry not in started:
            started.add(entry)
            subprocess.Popen([sys.executable, __file__, directory, entry[len(f"req-{name}-"):]])
    time.sleep(0.003)
"""


class RefusedSignals(unittest.TestCase):
    # stop_tree against real processes, with one of them made to refuse
    # signals as a process running as another user does.

    def start_tree(self, script):
        # A root shell whose two children are the process that will refuse,
        # `sleep 300`, created first, then agy, `sleep 301`.
        root = subprocess.Popen(["/bin/sh", "-c", script])
        self.addCleanup(lambda: root.poll() is None and root.kill())
        helper = load_helper()
        expected = (b"sleep\x00300\x00", b"sleep\x00301\x00")
        held, deadline = set(), time.monotonic() + 5
        while True:
            # A child shows the shell's command line until it has run sleep,
            # so wait for both command lines, not just for two children.
            found = helper.descendants(root.pid)
            for pid in set(found) - held:
                held.add(pid)
                self.addCleanup(self.kill, pid)
            by_command = {}
            for pid in found:
                try:
                    by_command[(PROC / str(pid) / "cmdline").read_bytes()] = pid
                except OSError:
                    pass
            if all(command in by_command for command in expected):
                break
            self.assertLess(time.monotonic(), deadline, "the tree did not start")
            time.sleep(0.01)
        refuser, agy = (by_command[command] for command in expected)
        # The refusing process is signalled before agy.
        self.assertEqual(found, [refuser, agy])
        return helper, root, refuser, agy

    def kill(self, pid):
        try:
            os.kill(pid, signal.SIGKILL)
        except ProcessLookupError:
            pass

    def state(self, pid):
        try:
            return (PROC / str(pid) / "stat").read_text().rsplit(")", 1)[1].split()[0]
        except FileNotFoundError:
            return "gone"

    def assert_running(self, pid):
        # Left alone, or resumed: still alive and not stopped. Just after
        # SIGCONT it may be runnable rather than back asleep.
        self.assertIn(self.state(pid), ("R", "S"), f"process {pid} is not running")

    def wait_gone(self, pid, what):
        # A process under the root shell may stay a zombie until init reaps it.
        deadline = time.monotonic() + 5
        while self.state(pid) not in ("Z", "X", "gone"):
            self.assertLess(time.monotonic(), deadline, f"{what} is still running")
            time.sleep(0.01)

    def stop_refusing(self, helper, root, agy, refuser, signals, pidfds):
        kill, pidfd_send_signal = refusing_signals(
            lambda pid, signum: pid == refuser and signum in signals)
        stderr = io.StringIO()
        with contextlib.ExitStack() as patches:
            patches.enter_context(pidfd_open_patch(self, pidfds))
            patches.enter_context(mock.patch.object(os, "kill", kill))
            if pidfd_send_signal:
                patches.enter_context(mock.patch.object(signal, "pidfd_send_signal", pidfd_send_signal))
            patches.enter_context(contextlib.redirect_stderr(stderr))
            helper.stop_tree(root.pid, agy, time.monotonic() + 0.3)
        return stderr.getvalue()

    @linux_only
    def test_a_process_refusing_sigterm_before_agy_does_not_stop_the_shutdown(self):
        # As in `sh -c 'sudo -n sleep 30 & agy'`: the process that refuses
        # comes before agy, and agy must still get its SIGTERM. Both ways of
        # signalling are covered: through a pidfd, and by PID without one.
        for pidfds in (True, False):
            with self.subTest(pidfds=pidfds):
                helper, root, refuser, agy = self.start_tree("sleep 300 & sleep 301 & wait")
                stderr = self.stop_refusing(helper, root, agy, refuser,
                                            {signal.SIGTERM, signal.SIGSTOP, signal.SIGKILL}, pidfds)
                root.wait(timeout=5)
                self.wait_gone(agy, "agy")
                # The helper left the refusing process alone, running, and said so once.
                self.assert_running(refuser)
                self.assertEqual(stderr.count(f"not allowed to signal process {refuser},"), 1, stderr)
                self.kill(refuser)

    @linux_only
    def test_a_process_refusing_sigstop_leaves_nothing_stopped(self):
        # Everything ignores SIGTERM, so the deadline passes and the helper
        # stops (SIGSTOP) the tree before killing it. The process that
        # refuses accepted SIGTERM but refuses SIGSTOP, as sudo does once it
        # has made itself root. The root shell, stopped before it, and agy,
        # after it, must still be killed, not left stopped. A process that
        # takes SIGSTOP but refuses SIGKILL, as a security policy may allow,
        # must not be left stopped either.
        for signals in ({signal.SIGSTOP, signal.SIGKILL}, {signal.SIGKILL}):
            with self.subTest(refused=sorted(signals)):
                helper, root, refuser, agy = self.start_tree(
                    "trap '' TERM; sleep 300 & sleep 301 & wait")
                stderr = self.stop_refusing(helper, root, agy, refuser, signals, True)
                self.assertEqual(root.wait(timeout=5), -signal.SIGKILL)
                self.wait_gone(agy, "agy")
                self.assert_running(refuser)
                self.assertEqual(stderr.count(f"not allowed to signal process {refuser},"), 1, stderr)
                # Nothing resumed a stopped process before the kill.
                self.assertNotIn("running again", stderr)
                self.kill(refuser)

    def start_starter(self):
        # A root shell that ignores SIGTERM, so the helper keeps polling until
        # its deadline, and under it the process that will refuse signals:
        # STOP_TREE_AGY, which starts one child each time it is asked, as
        # `sudo -u "$USER" cmd` starts cmd once its password is typed. Each
        # child records any SIGTERM it gets and keeps running.
        self.dir = Path(tempfile.mkdtemp(prefix="agy-pair-refused-"))
        self.addCleanup(shutil.rmtree, self.dir, ignore_errors=True)
        started = self.started = []
        self.addCleanup(lambda: [self.kill(pid) for pid in started])
        root = subprocess.Popen(["/bin/sh", "-c", 'trap "" TERM; "$@" & wait', "sh",
                                 sys.executable, "-c", STOP_TREE_AGY, str(self.dir)])
        self.addCleanup(lambda: root.poll() is None and root.kill())
        helper = load_helper()
        deadline = time.monotonic() + 5
        while True:
            # The child shows the shell's command line until it has run Python.
            starter = next(iter(helper.descendants(root.pid)), None)
            if starter is not None:
                self.addCleanup(self.kill, starter)
                try:
                    argv0 = (PROC / str(starter) / "cmdline").read_bytes().split(b"\0")[0]
                except OSError:
                    argv0 = None
                if argv0 == os.fsencode(sys.executable):
                    return helper, root, starter
            self.assertLess(time.monotonic(), deadline, "the tree did not start")
            time.sleep(0.01)

    def start_child(self):
        # Ask the refusing process for one more child and wait until it runs
        # with its SIGTERM handler in place.
        index = len(self.started)
        (self.dir / f"request-{index}").touch()
        ready, deadline = self.dir / f"ready-{index}", time.monotonic() + 5
        while not ready.exists():
            self.assertLess(time.monotonic(), deadline, "the refusing process did not start a child")
            time.sleep(0.005)
        self.started.append(int(ready.read_text()))
        return index, self.started[-1]

    def stop_starting(self, helper, root, starter, refuse, pidfds=True, looking=None):
        # stop_tree with each signal refused for which refuse(pid, signum),
        # which sees every signal the helper sends, is true. When given,
        # looking(children) wraps each child-list function the helper makes.
        kill, pidfd_send_signal = refusing_signals(refuse)
        real_lister = helper.child_lister

        def lister():
            children, starts = real_lister()
            return looking(children), starts

        stderr = io.StringIO()
        with contextlib.ExitStack() as patches:
            patches.enter_context(pidfd_open_patch(self, pidfds))
            patches.enter_context(mock.patch.object(os, "kill", kill))
            if pidfd_send_signal:
                patches.enter_context(mock.patch.object(signal, "pidfd_send_signal", pidfd_send_signal))
            if looking:
                patches.enter_context(mock.patch.object(helper, "child_lister", lister))
            patches.enter_context(contextlib.redirect_stderr(stderr))
            helper.stop_tree(root.pid, None, time.monotonic() + 0.3)
        stderr = stderr.getvalue()
        self.assertEqual(root.wait(timeout=5), -signal.SIGKILL)
        self.assertEqual(stderr.count(f"not allowed to signal process {starter},"), 1, stderr)
        self.assert_running(starter)
        return stderr

    @linux_only
    def test_a_process_started_under_a_refusing_process_after_its_refusal_gets_sigterm(self):
        # The process refuses SIGTERM and only then starts a child, while the
        # helper is still waiting for the root to exit. The helper must find
        # that child on its next poll and send it SIGTERM, then kill it at the
        # deadline, since it does not exit on SIGTERM.
        for pidfds in (True, False):
            with self.subTest(pidfds=pidfds):
                helper, root, starter = self.start_starter()
                children = []

                def refuse(pid, signum):
                    if pid != starter:
                        return False
                    if signum == signal.SIGTERM and not children:
                        children.append(self.start_child())
                    return True

                self.stop_starting(helper, root, starter, refuse, pidfds)
                [(index, child)] = children
                self.assertTrue((self.dir / f"term-{index}").exists(), "the child never got SIGTERM")
                self.wait_gone(child, "the child started after the refusal")
                self.kill(starter)

    @linux_only
    def test_a_process_started_under_a_process_refusing_sigstop_is_killed(self):
        # The process takes SIGTERM (and ignores it) but refuses SIGSTOP at
        # the deadline, as sudo does once it has made itself root, and starts
        # a child just then. The stop pass must still find and kill it.
        helper, root, starter = self.start_starter()
        children = []

        def refuse(pid, signum):
            if pid != starter:
                return False
            if signum == signal.SIGSTOP and not children:
                children.append(self.start_child())
            return signum in (signal.SIGSTOP, signal.SIGKILL)

        stderr = self.stop_starting(helper, root, starter, refuse)
        self.assertNotIn("running again", stderr)
        [(index, child)] = children
        # It appeared after the SIGTERM polls, so it was stopped and killed
        # without a SIGTERM.
        self.assertFalse((self.dir / f"term-{index}").exists())
        self.wait_gone(child, "the child started during the stop pass")
        self.kill(starter)

    @linux_only
    def test_a_refusing_process_that_never_stops_starting_children_cannot_hold_up_the_stop(self):
        # Every time the helper looks under the refusing process, it has
        # started one more child. The helper must still finish, looking there
        # exactly once after it has begun stopping (SIGSTOP) the tree, and
        # every child it saw must be gone.
        if not Path(f"/proc/self/task/{os.getpid()}/children").exists():
            self.skipTest("needs the kernel's per-thread child lists")
        helper, root, starter = self.start_starter()
        looks, stopping = [], []

        def refuse(pid, signum):
            # The starter refuses everything, so it is never sent SIGSTOP;
            # the root shell's SIGSTOP marks the start of the stop pass.
            if signum == signal.SIGSTOP:
                stopping.append(pid)
            return pid == starter

        def looking(children):
            def look(pid, held=None):
                if pid == starter:
                    looks.append(bool(stopping))
                    # Without a bound this would go on for ever; fail instead.
                    self.assertLess(len(looks), 100, "the helper kept looking under the refusing process")
                    self.start_child()
                return children(pid, held)
            return look

        self.stop_starting(helper, root, starter, refuse, looking=looking)
        self.assertTrue(stopping)
        self.assertEqual(looks.count(True), 1, looks)
        for pid in self.started:
            self.wait_gone(pid, f"child {pid}")
        self.kill(starter)

    def start_nodes(self):
        # A root shell that ignores SIGTERM, and under it SPAWNING_NODE "R".
        self.dir = Path(tempfile.mkdtemp(prefix="agy-pair-refused-"))
        self.addCleanup(shutil.rmtree, self.dir, ignore_errors=True)
        node = self.dir / "node.py"
        node.write_text(SPAWNING_NODE)
        directory = self.dir
        self.addCleanup(lambda: [self.kill(int(pid.read_text())) for pid in directory.glob("pid-*")
                                 if not pid.name.endswith(".tmp")])
        root = subprocess.Popen(["/bin/sh", "-c", 'trap "" TERM; "$@" & wait', "sh",
                                 sys.executable, str(node), str(self.dir), "R"])
        self.addCleanup(lambda: root.poll() is None and root.kill())
        return load_helper(), root, self.node("R")

    def node(self, name, parent=None):
        # With a parent, ask it to start the node first. Either way, wait
        # until the node runs with its SIGTERM handler in place.
        if parent:
            (self.dir / f"req-{parent}-{name}").touch()
        pid, deadline = self.dir / f"pid-{name}", time.monotonic() + 5
        while not pid.exists():
            self.assertLess(time.monotonic(), deadline, f"{name} did not start")
            time.sleep(0.003)
        return int(pid.read_text())

    @linux_only
    def test_a_process_that_first_refuses_in_the_stop_pass_still_gets_one_look(self):
        # R takes SIGTERM but refuses SIGSTOP, and just then starts C, so the
        # helper first sees C in the stop pass. C refuses SIGSTOP in turn and
        # just then starts G. The helper must look under C once after its
        # refusal and kill G.
        for pidfds in (True, False):
            with self.subTest(pidfds=pidfds):
                helper, root, refuser = self.start_nodes()
                ids = {}

                def refuse(pid, signum):
                    if pid == refuser:
                        if signum == signal.SIGSTOP and "C" not in ids:
                            ids["C"] = self.node("C", "R")
                        return signum != signal.SIGTERM
                    if pid == ids.get("C"):
                        if signum == signal.SIGSTOP and "G" not in ids:
                            ids["G"] = self.node("G", "C")
                        return True
                    return False

                self.stop_starting(helper, root, refuser, refuse, pidfds)
                self.assertIn("G", ids, "the helper never sent C SIGSTOP")
                self.wait_gone(ids["G"], "G, started under C after C refused SIGSTOP")
                self.assertFalse((self.dir / "term-G").exists())
                self.assert_running(ids["C"])
                self.kill(ids["C"])
                self.kill(refuser)

    @linux_only
    def test_a_chain_of_processes_first_refusing_in_the_stop_pass_cannot_hold_up_the_stop(self):
        # Each refusing process starts another just as it refuses SIGSTOP, so
        # every look under the newest one finds a new one to refuse. The
        # helper must stop looking after its bounded number of looks, leaving
        # the last one it started running, unseen.
        helper, root, refuser = self.start_nodes()
        chain = [refuser]

        def refuse(pid, signum):
            if pid not in chain:
                return False
            if signum == signal.SIGSTOP and pid == chain[-1]:
                # Without a bound this would go on for ever; fail instead.
                self.assertLess(len(chain), 20, "the helper kept looking under new refusing processes")
                parent = "R" if pid == refuser else f"N{len(chain) - 1}"
                chain.append(self.node(f"N{len(chain)}", parent))
            return signum != signal.SIGTERM

        self.stop_starting(helper, root, refuser, refuse)
        # R, then one more for each look that searched under the newest.
        self.assertEqual(len(chain), 1 + helper.REFUSED_STOP_LOOKS + 1, chain)
        for pid in chain:
            self.assert_running(pid)
            self.kill(pid)

    @linux_only
    def test_a_refusing_process_that_keeps_resuming_a_stopped_child_cannot_hold_up_the_stop(self):
        # R refuses every signal; its child K takes them. Each time the helper
        # looks under K in the stop pass, R has resumed K (SIGCONT), as a
        # process running as root may, and K has started one more child, so
        # every look finds a new process. The helper must stop after
        # STOP_LOOKS looks, kill K and every child it found, and say that
        # anything started since may be left running.
        #
        # Only this test starts processes here: R, then K, then one child of
        # K per look, and never more than `cap` children, so it cannot flood
        # the machine even against a helper that never stops looking. Each
        # process it starts is killed at cleanup.
        #
        # The child must appear in the look that started it, so the helper
        # must read the kernel's child lists: the fallback scan of /proc is
        # taken before the look and would miss it.
        if not Path(f"/proc/self/task/{os.getpid()}/children").exists():
            self.skipTest("needs the kernel's per-thread child lists")
        cap, real_kill = 40, os.kill
        for pidfds in (True, False):
            with self.subTest(pidfds=pidfds):
                helper, root, refuser = self.start_nodes()
                resumed = self.node("K", "R")
                looks, stopping = [], []

                def refuse(pid, signum):
                    # The root shell's SIGSTOP marks the start of the stop pass.
                    if signum == signal.SIGSTOP:
                        stopping.append(pid)
                    return pid == refuser

                def looking(children):
                    def look(pid, held=None):
                        if pid == resumed and stopping:
                            looks.append(pid)
                            # Fail rather than start more than `cap` children.
                            self.assertLessEqual(len(looks), cap,
                                                 "the helper kept looking under the resumed process")
                            real_kill(resumed, signal.SIGCONT)
                            self.node(f"C{len(looks)}", "K")
                        return children(pid, held)
                    return look

                stderr = self.stop_starting(helper, root, refuser, refuse, pidfds, looking)
                self.assertTrue(stopping)
                self.assertLessEqual(len(looks), helper.STOP_LOOKS, looks)
                self.assertEqual(stderr.count("new processes were still appearing"), 1, stderr)
                self.wait_gone(resumed, "K, the stopped process R kept resuming")
                for index in range(1, len(looks) + 1):
                    self.wait_gone(self.node(f"C{index}"), f"C{index}, found under K")
                self.kill(refuser)

    @linux_only
    def test_a_process_that_stops_between_two_reads_is_not_counted_as_resumed(self):
        # On a busy CPU, /proc/<pid>/status can show a process running with no
        # SIGSTOP pending: State is printed before the process acts on its
        # SIGSTOP, and the pending signals after. And State is only the main
        # thread's, which may still be busy in the kernel while another thread
        # has taken the stop. The helper must call the process resumed only if
        # no thread is stopped when it looks again after "nothing pending".
        # The reads are fed to it; the process held is this test's own, and
        # it is never signalled.
        helper = load_helper()
        sigstop = f"{1 << (signal.SIGSTOP - 1):016x}"
        nothing = "0" * 16
        pid = os.getpid()

        def status(state, pending=nothing):
            return f"Name:\tsleep\nState:\t{state}\nSigPnd:\t{nothing}\nShdPnd:\t{pending}\n"

        def stat(state):
            return f"{pid} (sleep) {state} 1 2 3\n"

        cases = {
            "running, nothing pending, then stopped": (status("R (running)"), [stat("T")], False),
            "running, nothing pending, still running": (status("R (running)"), [stat("S")], True),
            "main thread busy in the kernel, another stopped": (status("D (disk sleep)"), [stat("D"), stat("T")], False),
            "every thread running again": (status("D (disk sleep)"), [stat("D"), stat("S")], True),
            "running with SIGSTOP pending": (status("R (running)", sigstop), None, False),
            "stopped": (status("T (stopped)"), None, False),
        }
        for (name, (first, threads, expected)), pidfds in itertools.product(cases.items(), (True, False)):
            with self.subTest(name, pidfds=pidfds):
                with pidfd_open_patch(self, pidfds):
                    process = helper.Owned(pid)
                self.addCleanup(process.close)
                process.stopped = True
                read_text, listdir, looked = Path.read_text, os.listdir, []

                def read(path, *args, **kwargs):
                    name = str(path)
                    if name == f"/proc/{pid}/status":
                        return first
                    if name.startswith(f"/proc/{pid}/task/"):
                        looked.append(name)
                        return threads[int(name.split("/")[4])]
                    return read_text(path, *args, **kwargs)

                def list_threads(path="."):
                    if str(path) == f"/proc/{pid}/task":
                        return [str(index) for index in range(len(threads))]
                    return listdir(path)

                with mock.patch.object(Path, "read_text", read), mock.patch.object(os, "listdir", list_threads):
                    self.assertIs(process.resumed(), expected)
                self.assertEqual(len(looked), len(threads or []), looked)

    @linux_only
    def test_a_stopped_process_resumed_before_the_kill_is_reported(self):
        # R refuses every signal and resumes (SIGCONT) its child K once, just
        # after the helper has stopped it, so the next look finds nothing new
        # and the pass ends early. K could now start a process the helper
        # never sees, so the helper must say that a process it stopped is
        # running again, and still kill K. This test starts only R and K.
        for pidfds in (True, False):
            with self.subTest(pidfds=pidfds):
                helper, root, refuser = self.start_nodes()
                resumed = self.node("K", "R")
                stopping, resumes = [], []

                def refuse(pid, signum):
                    # The root shell's SIGSTOP marks the start of the stop pass.
                    if signum == signal.SIGSTOP:
                        stopping.append(pid)
                    return pid == refuser

                def looking(children):
                    def look(pid, held=None):
                        if pid == resumed and stopping and not resumes:
                            resumes.append(pid)
                            os.kill(resumed, signal.SIGCONT)
                        return children(pid, held)
                    return look

                stderr = self.stop_starting(helper, root, refuser, refuse, pidfds, looking)
                self.assertEqual(resumes, [resumed])
                self.assertEqual(stderr.count("a process the helper stopped is running again"), 1, stderr)
                self.assertNotIn("still appearing", stderr)
                self.wait_gone(resumed, "K, resumed after the helper stopped it")
                self.kill(refuser)


class ListenerCheck(unittest.TestCase):
    @linux_only
    def test_a_listener_in_another_network_namespace_does_not_count(self):
        # agy listens in a network namespace of its own, which has loopback
        # ports of its own. Another account listens on the same port number
        # in the helper's namespace, where the helper's connection goes.
        unshare = shutil.which("unshare")
        if not unshare or subprocess.run([unshare, "-rn", "true"], capture_output=True).returncode:
            self.skipTest("needs unprivileged user and network namespaces")
        helper = load_helper()
        decoy = Decoy(self)
        # A new network namespace starts with loopback down and, on many
        # kernels, without 127.0.0.1, so bring it up first, as a container
        # runtime would. The child then connects to its own listener to show
        # that the listener works in its namespace.
        inner = subprocess.Popen(
            [unshare, "-rn", sys.executable, "-c",
             "import fcntl, socket, struct, time\n"
             "s = socket.socket()\n"
             "request = struct.pack('16sH22x', b'lo', 0)\n"
             "flags = struct.unpack_from('16sH', fcntl.ioctl(s, 0x8913, request))[1]\n"  # SIOCGIFFLAGS
             "fcntl.ioctl(s, 0x8914, struct.pack('16sH22x', b'lo', flags | 1))\n"  # SIOCSIFFLAGS, IFF_UP
             f"s.bind(('127.0.0.1', {decoy.port})); s.listen()\n"
             f"socket.create_connection(('127.0.0.1', {decoy.port}), timeout=5).close()\n"
             "print('ready', flush=True); time.sleep(30)\n"],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
        )
        self.addCleanup(inner.stderr.close)
        self.addCleanup(inner.stdout.close)
        self.addCleanup(inner.wait)
        self.addCleanup(inner.kill)
        if inner.stdout.readline().strip() != "ready":
            inner.kill()
            self.fail(f"the listener in the other namespace did not start: {inner.stderr.read()}")
        self.assertNotEqual(os.readlink(PROC / str(inner.pid) / "ns/net"), os.readlink(PROC / "self/ns/net"))
        self.assertIsNone(helper.listener(inner.pid, decoy.port))
        self.assertFalse(helper.authenticated(inner.pid, decoy.port, "test-token", 0.5))
        self.assertEqual(decoy.connections, 0)
        # The same check accepts a listener held in the helper's namespace.
        self.assertIsNotNone(helper.listener(os.getpid(), decoy.port))


class DirectoryCheck(unittest.TestCase):
    def test_a_directory_another_account_owns_is_not_private(self):
        # Without root, a test cannot create a directory that belongs to
        # another account. Look at one of ours as another account would.
        if os.geteuid() == 0:
            self.skipTest("root may use any directory")
        helper = load_helper()
        with tempfile.TemporaryDirectory() as temp:
            log = Path(temp).resolve() / "log"
            log.mkdir(mode=0o700)
            self.assertFalse(helper.others_can_change(log))
            with mock.patch.object(os, "geteuid", return_value=os.geteuid() + 1):
                self.assertTrue(helper.others_can_change(log))


if __name__ == "fake_agy":
    sys.exit(fake_agy())
elif __name__ == "fake_tool":
    sys.exit(fake_tool())
elif __name__ == "fake_reader":
    sys.exit(fake_reader())
elif __name__ == "refusing_helper":
    refusing_helper()
elif __name__ == "__main__":
    unittest.main()
