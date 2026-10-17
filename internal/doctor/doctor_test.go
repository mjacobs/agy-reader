package doctor

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

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

	// A newer mtime cannot certify an unsupported SQLite source.
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

	total, fresh, stale, err := SidecarCoverage(root)
	if err != nil {
		t.Fatalf("SidecarCoverage: %v", err)
	}
	// None of the three is a readable SQLite source, so none is stale:
	// watch would refuse each one, and doctor reports them as unsupported.
	if total != 3 || fresh != 0 || stale != 0 {
		t.Fatalf("got total=%d fresh=%d stale=%d, want 3/0/0", total, fresh, stale)
	}
	r := Report{}
	PopulateSidecarCoverage(&r, root)
	if r.Unsupported != 3 || r.Unknown != 3 {
		t.Fatalf("got unsupported=%d unknown=%d, want 3/3: %+v", r.Unsupported, r.Unknown, r)
	}
}

func TestCompatibilityMDSyncWithRoot(t *testing.T) {
	rootCompat, err := os.ReadFile("../../COMPATIBILITY.md")
	if os.IsNotExist(err) {
		t.Skip("not running in repo checkout")
	} else if err != nil {
		t.Fatal(err)
	}
	if string(rootCompat) != compatibilityMD {
		t.Fatal("internal/doctor/COMPATIBILITY.md is out of sync with root COMPATIBILITY.md")
	}
}

func TestRecordedAgyVersionParsesEmbed(t *testing.T) {
	v := RecordedAgyVersion()
	if v == "" {
		t.Fatal("expected a recorded agy version from embedded COMPATIBILITY.md")
	}
	// COMPATIBILITY.md is machine-generated as "1.0.13"-style; assert shape.
	if !regexp.MustCompile(`^\d+\.\d+\.\d+`).MatchString(v) {
		t.Fatalf("recorded version %q is not semver-shaped", v)
	}
}

func TestParseRecordedVersionLine(t *testing.T) {
	got := ParseRecordedAgyVersion(
		"- **agy version:** 1.0.13\n- **Verified on:** 2026-06-27\n")
	if got != "1.0.13" {
		t.Fatalf("got %q want 1.0.13", got)
	}
}

func TestWriteDoctorReportHealthy(t *testing.T) {
	var buf bytes.Buffer
	code := WriteReport(&buf, Report{
		DaemonURL:    "http://127.0.0.1:51847",
		AgyVer:       "1.0.13",
		RecordedVer:  "1.0.13",
		Total:        23,
		Fresh:        23,
		Stale:        0,
		WatchKnown:   true,
		WatchRunning: true,
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
	code := WriteReport(&buf, Report{
		DaemonURL:   "http://127.0.0.1:51847",
		AgyVer:      "1.0.13",
		RecordedVer: "1.0.13",
		Total:       23,
		Fresh:       18,
		Stale:       5,
	})
	if code == 0 {
		t.Fatal("stale sidecars should be actionable (non-zero exit)")
	}
	if !strings.Contains(buf.String(), "agy-reader --watch") {
		t.Fatalf("should suggest --watch:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "agy-reader --sync\n") {
		t.Fatalf("should not recommend bare --sync:\n%s", buf.String())
	}
}

func TestWriteDoctorReportCoverageError(t *testing.T) {
	var buf bytes.Buffer
	code := WriteReport(&buf, Report{
		DaemonURL:   "http://127.0.0.1:51847",
		AgyVer:      "1.0.14",
		RecordedVer: "1.0.14",
		CoverageErr: errStubCoverage,
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

func TestWriteDoctorReportPinnedDaemonUnreachable(t *testing.T) {
	var buf bytes.Buffer
	code := WriteReport(&buf, Report{
		PinnedURL:   "http://127.0.0.1:51847",
		AgyVer:      "1.0.14",
		RecordedVer: "1.0.14",
		Total:       5,
		Fresh:       2,
		Stale:       3,
	})
	out := buf.String()
	if !strings.Contains(out, "ANTIGRAVITY_DAEMON_URL (http://127.0.0.1:51847) unreachable") {
		t.Fatalf("daemon line should name the unreachable pinned override:\n%s", out)
	}
	if strings.Contains(out, "not running") {
		t.Fatalf("should not report generic 'not running' for a pinned override:\n%s", out)
	}
	if !strings.Contains(out, "fix or unset ANTIGRAVITY_DAEMON_URL") {
		t.Fatalf("stale remediation should advise fixing the env var:\n%s", out)
	}
	if code == 0 {
		t.Fatal("stale sidecars should be actionable")
	}
}

func TestWriteDoctorReportPinnedUnreachableActionableWhenFresh(t *testing.T) {
	var buf bytes.Buffer
	code := WriteReport(&buf, Report{
		PinnedURL:    "http://127.0.0.1:51847",
		AgyVer:       "1.0.14",
		RecordedVer:  "1.0.14",
		Total:        9,
		Fresh:        9,
		Stale:        0,
		WatchKnown:   true,
		WatchRunning: true,
	})
	if code == 0 {
		t.Fatalf("unreachable pinned daemon URL must be actionable even with fresh sidecars:\n%s", buf.String())
	}
}

func TestWriteDoctorReportVersionSkew(t *testing.T) {
	var buf bytes.Buffer
	code := WriteReport(&buf, Report{
		DaemonURL:   "http://127.0.0.1:51847",
		AgyVer:      "1.0.20",
		RecordedVer: "1.0.13",
		Total:       1,
		Fresh:       1,
		Stale:       0,
	})
	if code == 0 {
		t.Fatal("version skew should be actionable")
	}
	if !strings.Contains(buf.String(), "1.0.20") {
		t.Fatalf("should show running version:\n%s", buf.String())
	}
}

func TestWriteDoctorReportCLIHasNoCSRFLine(t *testing.T) {
	var buf bytes.Buffer
	WriteReport(&buf, Report{
		DaemonURL:    "http://127.0.0.1:51847",
		AgyVer:       "1.0.16",
		RecordedVer:  "1.0.16",
		Total:        1,
		Fresh:        1,
		WatchKnown:   true,
		WatchRunning: true,
	})
	out := buf.String()
	if !strings.Contains(out, "surface:     cli") {
		t.Fatalf("zero-value surface should render as cli:\n%s", out)
	}
	if strings.Contains(out, "csrf:") {
		t.Fatalf("CLI reports must not carry a csrf line:\n%s", out)
	}
}

func TestWriteDoctorReportIDEHealthy(t *testing.T) {
	var buf bytes.Buffer
	code := WriteReport(&buf, Report{
		Surface:      "ide",
		DaemonURL:    "http://127.0.0.1:38775",
		CSRFFound:    true,
		AgyVer:       "1.0.16",
		RecordedVer:  "1.0.16",
		Total:        4,
		Fresh:        4,
		WatchKnown:   true,
		WatchRunning: true,
	})
	if code != 0 {
		t.Fatalf("healthy IDE report should exit 0, got %d:\n%s", code, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "surface:     ide") {
		t.Fatalf("should report the ide surface:\n%s", out)
	}
	if !strings.Contains(out, "csrf:        token found") {
		t.Fatalf("should report the discovered token:\n%s", out)
	}
}

func TestWriteDoctorReportIDEMissingTokenActionable(t *testing.T) {
	var buf bytes.Buffer
	code := WriteReport(&buf, Report{
		Surface:     "ide",
		DaemonURL:   "http://127.0.0.1:38775",
		CSRFFound:   false,
		AgyVer:      "1.0.16",
		RecordedVer: "1.0.16",
		Total:       4,
		Fresh:       4,
	})
	if code == 0 {
		t.Fatal("a reachable IDE daemon with no CSRF token must be actionable")
	}
	if !strings.Contains(buf.String(), "ANTIGRAVITY_CSRF_TOKEN") {
		t.Fatalf("remediation should name the override env var:\n%s", buf.String())
	}
}

func TestWriteDoctorReportIDEMissingTokenInformationalWhenClosed(t *testing.T) {
	var buf bytes.Buffer
	code := WriteReport(&buf, Report{
		Surface:      "ide",
		CSRFFound:    false,
		AgyVer:       "1.0.16",
		RecordedVer:  "1.0.16",
		Total:        4,
		Fresh:        4,
		WatchKnown:   true,
		WatchRunning: true,
	})
	if code != 0 {
		t.Fatalf("a closed IDE with no token must stay informational:\n%s", buf.String())
	}
}

func TestWriteDoctorReportIDEStaleSuggestsRootedWatch(t *testing.T) {
	var buf bytes.Buffer
	code := WriteReport(&buf, Report{
		Surface:     "ide",
		Root:        "/home/x/.gemini/antigravity",
		DaemonURL:   "http://127.0.0.1:38775",
		CSRFFound:   true,
		AgyVer:      "1.0.16",
		RecordedVer: "1.0.16",
		Total:       4,
		Fresh:       1,
		Stale:       3,
	})
	if code == 0 {
		t.Fatal("stale sidecars should be actionable")
	}
	if !strings.Contains(buf.String(), "agy-reader --root /home/x/.gemini/antigravity --watch") {
		t.Fatalf("IDE stale remediation must carry --root:\n%s", buf.String())
	}
}

func TestCmdlineIsWatch(t *testing.T) {
	nul := func(args ...string) []byte {
		return []byte(strings.Join(args, "\x00") + "\x00")
	}
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"installed watcher", nul("/home/user-a/.local/bin/agy-reader", "--watch", "--watch-idle-timeout=5m"), true},
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
		{"unrelated proc with agy-reader in a path", nul("node", "serve", "--path", "/home/user-a/dev/projects/agy-reader"), false},
		{"empty", []byte(""), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, isWatch := CmdlineWatchRoots(tc.data)
			if isWatch != tc.want {
				t.Fatalf("CmdlineWatchRoots(%q) = %v, want %v", tc.data, isWatch, tc.want)
			}
		})
	}
}

func TestDaemonReachableDirect(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	if err := daemon.DaemonReachable("http://" + ln.Addr().String()); err != nil {
		t.Fatalf("live listener should be reachable: %v", err)
	}
	if err := daemon.DaemonReachable("http://"); err == nil {
		t.Fatal("url with no host should error")
	}
	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	dead := "http://" + ln2.Addr().String()
	ln2.Close()
	if err := daemon.DaemonReachable(dead); err == nil {
		t.Fatal("closed listener should be unreachable")
	}
}

func TestReachableDaemonConnectionHonorsEnvOverride(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	want := "http://" + ln.Addr().String()
	t.Setenv("ANTIGRAVITY_DAEMON_URL", want)

	got, err := ReachableDaemonConnection(t.TempDir())
	if err != nil {
		t.Fatalf("ReachableDaemonConnection: %v", err)
	}
	if got.BaseURL != want {
		t.Fatalf("got %q, want the env override %q", got.BaseURL, want)
	}
}

func TestReachableDaemonConnectionPinnedUnreachableDoesNotFallBack(t *testing.T) {
	live, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer live.Close()
	_, port, err := net.SplitHostPort(live.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	root := t.TempDir()
	writeFileT(t, filepath.Join(root, "cli.log"),
		[]byte("listening on random port at "+port+" for HTTP\n"))

	dead, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	deadURL := "http://" + dead.Addr().String()
	dead.Close()
	t.Setenv("ANTIGRAVITY_DAEMON_URL", deadURL)

	got, err := ReachableDaemonConnection(root)
	if err == nil {
		t.Fatalf("pinned-unreachable override must error, not fall back to %q", got.BaseURL)
	}
}

func TestRunDoctorEndToEnd(t *testing.T) {
	root := t.TempDir()
	conv := filepath.Join(root, "conversations")
	writeFileT(t, filepath.Join(conv, "c.db"), []byte("x"))

	var buf bytes.Buffer
	code := RunTo(&buf, []string{root}, true)
	if code == 0 {
		t.Fatal("a missing sidecar should make doctor non-zero")
	}
	if !strings.Contains(buf.String(), "sidecars:") {
		t.Fatalf("expected a sidecars line:\n%s", buf.String())
	}
}

func TestReachableDaemonConnectionCarriesTheScannedToken(t *testing.T) {
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	root := t.TempDir()
	writeFileT(t, filepath.Join(root, "cli.log"),
		[]byte("listening on random port at "+port+" for HTTP\n"))
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "scanned-token")

	got, err := ReachableDaemonConnection(root)
	if err != nil {
		t.Fatalf("ReachableDaemonConnection: %v", err)
	}
	if got.BaseURL != "http://127.0.0.1:"+port {
		t.Errorf("BaseURL = %q, want the discovered endpoint", got.BaseURL)
	}
	if got.Token != "scanned-token" {
		t.Errorf("Token = %q, want the token from the same scan", got.Token)
	}
}

func TestWriteDoctorReportUnsupportedNeedsCompatibilityUpdate(t *testing.T) {
	var buf bytes.Buffer
	code := WriteReport(&buf, Report{
		DaemonURL:    "http://127.0.0.1:51847",
		AgyVer:       "1.0.16",
		RecordedVer:  "1.0.16",
		SQLite:       1,
		Unknown:      1,
		Unsupported:  1,
		Total:        1,
		WatchKnown:   true,
		WatchRunning: true,
	})
	if code == 0 {
		t.Fatal("unsupported SQLite session should fail doctor (non-zero exit)")
	}
	out := buf.String()
	want := "1 unsupported -> needs an agy-reader compatibility update (see README)"
	if !strings.Contains(out, want) {
		t.Fatalf("expected unsupported remediation line %q in:\n%s", want, out)
	}
	if strings.Contains(out, "agy-reader --watch") {
		t.Fatalf("unsupported session must not suggest watch hint:\n%s", out)
	}
	if strings.Contains(out, "missing/stale") {
		t.Fatalf("unsupported session must not be labeled missing/stale:\n%s", out)
	}
}

func TestWriteDoctorReportMissingSidecarSuggestsWatch(t *testing.T) {
	var buf bytes.Buffer
	code := WriteReport(&buf, Report{
		DaemonURL:   "http://127.0.0.1:51847",
		AgyVer:      "1.0.16",
		RecordedVer: "1.0.16",
		Total:       1,
		Stale:       1,
	})
	if code == 0 {
		t.Fatal("missing sidecar should fail doctor (non-zero exit)")
	}
	out := buf.String()
	if !strings.Contains(out, "1 missing/stale -> run: agy-reader --watch") {
		t.Fatalf("expected missing/stale watch hint in:\n%s", out)
	}
	if strings.Contains(out, "unsupported") {
		t.Fatalf("ordinary missing sidecar must not report unsupported:\n%s", out)
	}
}

func TestPopulateSidecarCoverageUnsupportedSourceIsNotStale(t *testing.T) {
	root := t.TempDir()
	conv := filepath.Join(root, "conversations")
	writeFileT(t, filepath.Join(conv, "sess.db"), []byte("not-a-db"))

	r := Report{}
	PopulateSidecarCoverage(&r, root)
	if r.Total != 1 || r.SQLite != 1 || r.Stale != 0 || r.Unsupported != 1 || r.Unknown != 1 || r.Fresh != 0 {
		t.Fatalf("expected 1 total, 1 sqlite, 0 stale, 1 unsupported, 1 unknown, 0 fresh; got %+v", r)
	}
	var out bytes.Buffer
	WriteReport(&out, r)
	if !strings.Contains(out.String(), "1 unsupported -> needs an agy-reader compatibility update (see README)") || strings.Contains(out.String(), "missing/stale") {
		t.Fatalf("unsupported source must get the compatibility hint, not the watch hint:\n%s", out.String())
	}
}
