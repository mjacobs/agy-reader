package main

import (
	"bytes"
	"errors"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// errStubCoverage stands in for a sidecarCoverage failure (e.g. an unreadable
// conversations dir) in renderer tests.
var errStubCoverage = errors.New("stub: permission denied")

func writeFileT(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func mustChtimes(t *testing.T, p string, ts time.Time) {
	t.Helper()
	if err := os.Chtimes(p, ts, ts); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

func TestSidecarCoverage(t *testing.T) {
	root := t.TempDir()
	conv := filepath.Join(root, "conversations")

	// fresh: sidecar newer than .db
	writeFileT(t, filepath.Join(conv, "a.db"), []byte("x"))
	writeFileT(t, filepath.Join(conv, "a.trajectory.json"), []byte("{}"))
	old := time.Unix(1779000000, 0)
	newer := time.Unix(1779000300, 0)
	mustChtimes(t, filepath.Join(conv, "a.db"), old)
	mustChtimes(t, filepath.Join(conv, "a.trajectory.json"), newer)

	// stale: .db newer than sidecar
	writeFileT(t, filepath.Join(conv, "b.db"), []byte("x"))
	writeFileT(t, filepath.Join(conv, "b.trajectory.json"), []byte("{}"))
	mustChtimes(t, filepath.Join(conv, "b.trajectory.json"), old)
	mustChtimes(t, filepath.Join(conv, "b.db"), newer)

	// missing: no sidecar
	writeFileT(t, filepath.Join(conv, "c.db"), []byte("x"))

	total, fresh, stale, err := sidecarCoverage(root)
	if err != nil {
		t.Fatalf("sidecarCoverage: %v", err)
	}
	if total != 3 || fresh != 1 || stale != 2 {
		t.Fatalf("got total=%d fresh=%d stale=%d, want 3/1/2", total, fresh, stale)
	}
}

func TestRecordedAgyVersionParsesEmbed(t *testing.T) {
	v := recordedAgyVersion()
	if v == "" {
		t.Fatal("expected a recorded agy version from embedded COMPATIBILITY.md")
	}
	// COMPATIBILITY.md is machine-generated as "1.0.13"-style; assert shape.
	if !regexp.MustCompile(`^\d+\.\d+\.\d+`).MatchString(v) {
		t.Fatalf("recorded version %q is not semver-shaped", v)
	}
}

func TestParseRecordedVersionLine(t *testing.T) {
	got := parseRecordedAgyVersion(
		"- **agy version:** 1.0.13\n- **Verified on:** 2026-06-27\n")
	if got != "1.0.13" {
		t.Fatalf("got %q want 1.0.13", got)
	}
}

func TestWriteDoctorReportHealthy(t *testing.T) {
	var buf bytes.Buffer
	code := writeDoctorReport(&buf, doctorReport{
		daemonURL: "http://127.0.0.1:51847",
		agyVer:    "1.0.13", recordedVer: "1.0.13",
		total: 23, fresh: 23, stale: 0,
		watchKnown: true, watchRunning: true,
	})
	if code != 0 {
		t.Fatalf("healthy report should exit 0, got %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "reachable") || !strings.Contains(out, "23/23") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestWriteDoctorReportStaleNeedsAction(t *testing.T) {
	var buf bytes.Buffer
	code := writeDoctorReport(&buf, doctorReport{
		daemonURL: "http://127.0.0.1:51847",
		agyVer:    "1.0.13", recordedVer: "1.0.13",
		total: 23, fresh: 18, stale: 5,
	})
	if code == 0 {
		t.Fatal("stale sidecars should be actionable (non-zero exit)")
	}
	// Must recommend a command that can actually batch-fix sidecars. Bare
	// --sync errors with "missing conversation id", so doctor recommends --watch.
	if !strings.Contains(buf.String(), "agy-reader --watch") {
		t.Fatalf("should suggest --watch:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "agy-reader --sync\n") {
		t.Fatalf("should not recommend bare --sync:\n%s", buf.String())
	}
}

func TestWriteDoctorReportCoverageError(t *testing.T) {
	var buf bytes.Buffer
	code := writeDoctorReport(&buf, doctorReport{
		daemonURL: "http://127.0.0.1:51847",
		agyVer:    "1.0.14", recordedVer: "1.0.14",
		coverageErr: errStubCoverage,
	})
	if code == 0 {
		t.Fatal("an unreadable sidecar root should be actionable, not healthy")
	}
	out := buf.String()
	if !strings.Contains(out, "unknown") {
		t.Fatalf("should report sidecar coverage as unknown:\n%s", out)
	}
	if strings.Contains(out, "0/0 fresh") {
		t.Fatalf("must not imply a healthy 0/0 when coverage failed:\n%s", out)
	}
}

func TestWriteDoctorReportVersionSkew(t *testing.T) {
	var buf bytes.Buffer
	code := writeDoctorReport(&buf, doctorReport{
		daemonURL: "http://127.0.0.1:51847",
		agyVer:    "1.0.20", recordedVer: "1.0.13",
		total: 1, fresh: 1, stale: 0,
	})
	if code == 0 {
		t.Fatal("version skew should be actionable")
	}
	if !strings.Contains(buf.String(), "1.0.20") {
		t.Fatalf("should show running version:\n%s", buf.String())
	}
}

func TestCmdlineIsWatch(t *testing.T) {
	// cmdline is NUL-separated argv; build them that way.
	nul := func(args ...string) []byte {
		return []byte(strings.Join(args, "\x00") + "\x00")
	}
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"installed watcher", nul("/home/mj/.local/bin/agy-reader", "--watch", "--watch-idle-timeout=5m"), true},
		{"bare name", nul("agy-reader", "--watch"), true},
		{"go run temp binary", nul("/tmp/go-build123/b001/exe/agy-reader", "--watch"), true},
		{"single dash", nul("agy-reader", "-watch"), true},
		{"explicit true", nul("agy-reader", "--watch=true"), true},
		{"value flag then watch still parses", nul("agy-reader", "--root", "/foo", "--watch"), true},
		{"explicit false is not running", nul("agy-reader", "--watch=false"), false},
		{"explicit zero is not running", nul("agy-reader", "-watch=0"), false},
		{"watch after a positional does not parse as a flag", nul("agy-reader", "some-cascade-id", "--watch"), false},
		{"watch after -- terminator does not parse", nul("agy-reader", "--", "--watch"), false},
		{"interval flag only is not the watch bool", nul("agy-reader", "--watch-interval=30s"), false},
		{"idle-timeout flag only is not the watch bool", nul("agy-reader", "--watch-idle-timeout=5m"), false},
		{"doctor invocation", nul("agy-reader", "doctor"), false},
		{"shell echoing the strings", nul("/usr/bin/zsh", "-c", "echo agy-reader --watch"), false},
		{"unrelated proc with agy-reader in a path", nul("node", "serve", "--path", "/home/mj/dev/projects/agy-reader"), false},
		{"empty", []byte(""), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cmdlineIsWatch(tc.data); got != tc.want {
				t.Fatalf("cmdlineIsWatch(%q) = %v, want %v", tc.data, got, tc.want)
			}
		})
	}
}

func TestDaemonReachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	if err := daemonReachable("http://" + ln.Addr().String()); err != nil {
		t.Fatalf("live listener should be reachable: %v", err)
	}

	// A URL with no host is an error, not a silent success.
	if err := daemonReachable("http://"); err == nil {
		t.Fatal("url with no host should error")
	}

	// Nothing listening on the address -> error.
	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	dead := "http://" + ln2.Addr().String()
	ln2.Close()
	if err := daemonReachable(dead); err == nil {
		t.Fatal("closed listener should be unreachable")
	}
}

func TestReachableDaemonURLHonorsEnvOverride(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	want := "http://" + ln.Addr().String()
	t.Setenv("ANTIGRAVITY_DAEMON_URL", want)

	got, err := reachableDaemonURL(t.TempDir())
	if err != nil {
		t.Fatalf("reachableDaemonURL: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want the env override %q", got, want)
	}
}

func TestReachableDaemonURLFallsBackWhenEnvUnreachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	dead := "http://" + ln.Addr().String()
	ln.Close() // nothing is listening now
	t.Setenv("ANTIGRAVITY_DAEMON_URL", dead)

	// Empty root has no cli.log, so discovery also fails: an unreachable env
	// override must NOT be returned as if it were a live daemon.
	got, err := reachableDaemonURL(t.TempDir())
	if err == nil {
		t.Fatalf("unreachable env override + no discovery should error, got %q", got)
	}
	if got == dead {
		t.Fatalf("must not report the unreachable pinned URL %q as reachable", dead)
	}
}

func TestRunDoctorEndToEnd(t *testing.T) {
	root := t.TempDir()
	conv := filepath.Join(root, "conversations")
	writeFileT(t, filepath.Join(conv, "c.db"), []byte("x")) // missing sidecar -> stale

	var buf bytes.Buffer
	code := runDoctorTo(&buf, root) // testable variant taking an io.Writer
	if code == 0 {
		t.Fatal("a missing sidecar should make doctor non-zero")
	}
	if !strings.Contains(buf.String(), "sidecars:") {
		t.Fatalf("expected a sidecars line:\n%s", buf.String())
	}
}
