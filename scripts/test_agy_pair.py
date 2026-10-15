"""Process-level tests: real child processes and an authenticated loopback RPC."""

import hashlib
import http.client
import importlib.util
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import pty
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

    def launch(self, *extra, env_overrides=None):
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
            [sys.executable, str(HELPER), "run", *self.base, *extra, *self.agy_args],
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
elif __name__ == "__main__":
    unittest.main()
