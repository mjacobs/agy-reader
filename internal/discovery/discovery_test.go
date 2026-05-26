package discovery_test

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/discovery"
)

func writeFile(t *testing.T, path, content string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if !mtime.IsZero() {
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
}

func TestListSessionsBothBuckets(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeFile(t, filepath.Join(root, "conversations", "aaaa.pb"), "x", now.Add(-2*time.Hour))
	writeFile(t, filepath.Join(root, "conversations", "bbbb.pb"), "x", now.Add(-1*time.Hour))
	writeFile(t, filepath.Join(root, "implicit", "cccc.pb"), "x", now)
	// Should be ignored
	writeFile(t, filepath.Join(root, "conversations", "notes.txt"), "x", now)
	writeFile(t, filepath.Join(root, "conversations", "aaaa.trajectory.json"), "{}", now)

	got, err := discovery.ListSessions(root)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 sessions, got %d: %+v", len(got), got)
	}
	if got[0].CascadeID != "cccc" {
		t.Errorf("expected newest first, got %q", got[0].CascadeID)
	}
	if got[0].Bucket != "implicit" {
		t.Errorf("expected bucket implicit, got %q", got[0].Bucket)
	}
	wantSidecar := filepath.Join(root, "implicit", "cccc.trajectory.json")
	if got[0].SidecarPath != wantSidecar {
		t.Errorf("sidecar path: got %q want %q", got[0].SidecarPath, wantSidecar)
	}
}

func TestListSessionsMissingRoot(t *testing.T) {
	got, err := discovery.ListSessions(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(got))
	}
}

func TestFindByID(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "conversations", "abc.pb"), "x", time.Now())
	got, ok, err := discovery.FindByID(root, "abc.pb")
	if err != nil || !ok {
		t.Fatalf("FindByID: ok=%v err=%v", ok, err)
	}
	if got.CascadeID != "abc" {
		t.Errorf("got %q want abc", got.CascadeID)
	}
	_, ok, err = discovery.FindByID(root, "nope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Errorf("expected not found")
	}
}

func TestRootEnvOverride(t *testing.T) {
	t.Setenv("ANTIGRAVITY_CLI_ROOT", "/tmp/override")
	r, err := discovery.Root()
	if err != nil {
		t.Fatal(err)
	}
	if r != "/tmp/override" {
		t.Errorf("got %q want /tmp/override", r)
	}
}

func TestDiscoverDaemonURL(t *testing.T) {
	root := t.TempDir()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}

	logContent := "I0526 13:14:00.866371 95191 server.go:494] Language server listening on random port at " + portStr + " for HTTP\n"
	if err := os.WriteFile(filepath.Join(root, "cli.log"), []byte(logContent), 0o644); err != nil {
		t.Fatalf("write cli.log: %v", err)
	}

	discovered, err := discovery.DiscoverDaemonURL(root)
	if err != nil {
		t.Fatalf("DiscoverDaemonURL failed: %v", err)
	}

	expected := "http://127.0.0.1:" + portStr
	if discovered != expected {
		t.Errorf("got %q want %q", discovered, expected)
	}
}

func TestDiscoverDaemonURLMissingLog(t *testing.T) {
	_, err := discovery.DiscoverDaemonURL(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing cli.log")
	}
	if !strings.Contains(err.Error(), "cli.log") {
		t.Errorf("expected cli.log in error, got %v", err)
	}
}

func TestDiscoverDaemonURLNoMatchingLine(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cli.log"), []byte("some unrelated log line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := discovery.DiscoverDaemonURL(root)
	if err == nil {
		t.Fatal("expected error when no matching line present")
	}
	if !strings.Contains(err.Error(), "no active HTTP daemon port") {
		t.Errorf("got error %v", err)
	}
}

func TestDiscoverDaemonURLPortUnreachable(t *testing.T) {
	root := t.TempDir()
	// Grab an ephemeral port then close it so nothing is listening.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	ln.Close()

	logContent := "Language server listening on random port at " + portStr + " for HTTP\n"
	if err := os.WriteFile(filepath.Join(root, "cli.log"), []byte(logContent), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = discovery.DiscoverDaemonURL(root)
	if err == nil {
		t.Fatal("expected unreachable error")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Errorf("expected 'unreachable' in error, got %v", err)
	}
}

// Multiple matching lines: the most recent (last) one wins. Simulates a daemon
// restart that rebound to a new port within the same log file.
func TestDiscoverDaemonURLLatestLineWins(t *testing.T) {
	root := t.TempDir()

	// Stale port: grab and immediately release so it isn't listening.
	stale, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen stale: %v", err)
	}
	_, stalePort, _ := net.SplitHostPort(stale.Addr().String())
	stale.Close()

	// Live port: keep the listener open.
	live, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen live: %v", err)
	}
	defer live.Close()
	_, livePort, _ := net.SplitHostPort(live.Addr().String())

	logContent := "Language server listening on random port at " + stalePort + " for HTTP\n" +
		"some other line\n" +
		"Language server listening on random port at " + livePort + " for HTTP\n"
	if err := os.WriteFile(filepath.Join(root, "cli.log"), []byte(logContent), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := discovery.DiscoverDaemonURL(root)
	if err != nil {
		t.Fatalf("DiscoverDaemonURL: %v", err)
	}
	want := "http://127.0.0.1:" + livePort
	if got != want {
		t.Errorf("got %q want %q (should pick latest line)", got, want)
	}
}
