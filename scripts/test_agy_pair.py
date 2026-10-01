"""Process-level tests: real child processes and an authenticated loopback RPC."""

import hashlib
import http.client
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import os
from pathlib import Path
import pty
import select
import shutil
import signal
import subprocess
import sys
import tempfile
import termios
import threading
import time
import tty
import unittest
import uuid


HELPER = Path(__file__).with_name("agy-pair.py")


def fake_agy():
    options = dict(arg.split("=", 1) for arg in sys.argv[1:] if "=" in arg)
    root = Path(options["--fixture"])
    config = json.loads((root / "config.json").read_text())
    token = options["--csrf_token"]
    if config.get("ignore_term"):
        signal.signal(signal.SIGTERM, signal.SIG_IGN)
    if config.get("raw_terminal"):
        tty.setraw(sys.stdin.fileno())
    (root / "agy.json").write_text(json.dumps({
        "pid": os.getpid(), "token_hash": hashlib.sha256(token.encode()).hexdigest(),
        "first_arg": sys.argv[1].split("=")[0], "args": sys.argv[3:],
        "same_env": token == os.environ.get("ANTIGRAVITY_CSRF_TOKEN"),
        "url": os.environ.get("ANTIGRAVITY_DAEMON_URL"),
    }))
    if config.get("early_exit") is not None:
        return config["early_exit"]

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            good = self.headers.get("x-codeium-csrf-token") == token
            self.send_response(200 if good and not config.get("reject") else 401)
            self.end_headers()
            self.wfile.write(b"{}")

        def log_message(self, *_args):
            pass

    server = HTTPServer(("127.0.0.1", 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    if not config.get("no_listener_log"):
        Path(options["--log-file"]).write_text(
            f"listening on random port at {server.server_port} for HTTP\n"
            "listening on random port at 1 for HTTPS (gRPC)\n"
        )
    while True:
        ready, _, _ = select.select([sys.stdin], [], [], 0.05)
        if ready:
            line = sys.stdin.readline().strip()
            if line.startswith("exit "):
                return int(line.split()[1])
        if (root / "exit").exists():
            return int((root / "exit").read_text())
        time.sleep(0.02)


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
        "status": status, "url": url, "args": sys.argv[1:],
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

    def launch(self, *extra):
        env = os.environ.copy()
        env.update(ANTIGRAVITY_CSRF_TOKEN="stale-token",
                   ANTIGRAVITY_DAEMON_URL="http://127.0.0.1:1",
                   ANTIGRAVITY_CLI_ROOT="/wrong-root")
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

    def test_authenticated_pair_and_agy_exit_code(self):
        process = self.launch()
        agy, reader = self.record("agy"), self.record("reader")
        self.assertEqual(agy["first_arg"], "--csrf_token")
        self.assertTrue(agy["same_env"])
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

    def test_managed_arguments_cannot_be_overridden(self):
        for arg in ("--csrf_token=bad", "--log-file=/tmp/wrong"):
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
elif __name__ == "fake_reader":
    sys.exit(fake_reader())
elif __name__ == "__main__":
    unittest.main()
