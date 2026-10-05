"""Process-level tests: real child processes and an authenticated loopback RPC."""

import hashlib
import http.client
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

# Writes the arguments its process received and which shim variables it
# inherited.
REPORT_C = r"""
#include <stdio.h>
#include <stdlib.h>
int main(int argc, char **argv) {
    FILE *out = fopen(getenv("REPORT"), "w");
    if (!out) return 70;
    for (int i = 0; i < argc; i++) fprintf(out, "%s\n", argv[i]);
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
        Path(options["--log-file"]).write_text(
            f"listening on random port at {server.server_port} for HTTP\n"
            "listening on random port at 1 for HTTPS (gRPC)\n"
        )

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
        (self.root / "store").mkdir()
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
        for path in log_dir.iterdir():
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)

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
        # the shim or the token.
        self.assertEqual(agy["first_arg"], "--csrf_token")
        self.assertFalse(agy["cmdline_has_token"])
        self.assertFalse(agy["environ_has_token"])
        self.assertFalse(agy["env_token"])
        self.assertFalse(agy["preload_env"])
        self.assertFalse(agy["pair_env"])
        self.assertFalse(agy["pair_exe_env"])
        self.assertEqual(agy["child_env"], [False, False, False])
        # agy's /proc/<pid>/environ, read from outside as the same user,
        # shows the block agy started with. README documents exactly this:
        # the token's bytes are zeroed, but LD_PRELOAD with the library's
        # path, ANTIGRAVITY_PAIR_EXE with agy's path, and the emptied token
        # variable name remain. Update both together.
        listed, found = environment_block(
            agy["pid"], (b"LD_PRELOAD", b"ANTIGRAVITY_PAIR_TOKEN", b"ANTIGRAVITY_PAIR_EXE"), [token])
        self.assertEqual(found, [False])
        self.assertEqual(listed, [b"ANTIGRAVITY_PAIR_EXE=" + str(native).encode(),
                                  b"ANTIGRAVITY_PAIR_TOKEN=", b"LD_PRELOAD=" + str(shim).encode()])
        self.assertIn("spaces ; $ are literal", agy["args"])
        self.assertEqual(reader["token_hash"], agy["token_hash"])
        self.assertEqual(reader["status"], 200)
        self.assertFalse(visible_in_any_arguments(token))
        stdout, stderr = process.communicate("exit 3\n", timeout=5)
        self.assertEqual(process.returncode, 3, stderr)
        self.assertNotIn(token, stdout + stderr)
        self.assertNotIn("LD_PRELOAD", stderr)
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
               "ANTIGRAVITY_PAIR_EXE": str(report), "REPORT": str(output)}

        def received(program):
            output.unlink(missing_ok=True)
            subprocess.run([str(program), "--pair_token_slot=1", "x"], env=env, check=True, timeout=10)
            lines = output.read_text().splitlines()
            return {"token": any(token in line for line in lines),
                    "placeholder": "--pair_token_slot=1" in lines,
                    "inherited": lines[-1]}

        # Positive control: run directly, the checked program gets the real
        # flag in its own memory, and inherits none of the shim's variables.
        self.assertEqual(received(report),
                         {"token": True, "placeholder": False, "inherited": "inherited:000"})
        for kind, wrapper in self.wrappers(report).items():
            with self.subTest(wrapper=kind):
                self.assertEqual(received(wrapper),
                                 {"token": False, "placeholder": True, "inherited": "inherited:000"})

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


if __name__ == "fake_agy":
    sys.exit(fake_agy())
elif __name__ == "fake_tool":
    sys.exit(fake_tool())
elif __name__ == "fake_reader":
    sys.exit(fake_reader())
elif __name__ == "__main__":
    unittest.main()
