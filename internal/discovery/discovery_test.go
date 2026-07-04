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
	writeFile(t, filepath.Join(root, "conversations", "dddd.db"), "x", now.Add(1*time.Hour))
	// Should be ignored
	writeFile(t, filepath.Join(root, "conversations", "dddd.db-shm"), "x", now.Add(1*time.Hour))
	writeFile(t, filepath.Join(root, "conversations", "dddd.db-wal"), "x", now.Add(1*time.Hour))
	writeFile(t, filepath.Join(root, "conversations", "notes.txt"), "x", now)
	writeFile(t, filepath.Join(root, "conversations", "aaaa.trajectory.json"), "{}", now)

	got, err := discovery.ListSessions(root)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 sessions, got %d: %+v", len(got), got)
	}
	if got[0].CascadeID != "dddd" {
		t.Errorf("expected newest first (dddd.db), got %q", got[0].CascadeID)
	}
	if got[1].CascadeID != "cccc" {
		t.Errorf("expected second newest (cccc.pb), got %q", got[1].CascadeID)
	}
	if got[1].Bucket != "implicit" {
		t.Errorf("expected bucket implicit, got %q", got[1].Bucket)
	}
	wantSidecar := filepath.Join(root, "implicit", "cccc.trajectory.json")
	if got[1].SidecarPath != wantSidecar {
		t.Errorf("sidecar path: got %q want %q", got[1].SidecarPath, wantSidecar)
	}
}

func TestListConversationSessionsExcludesImplicit(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeFile(t, filepath.Join(root, "conversations", "aaaa.pb"), "x", now.Add(-1*time.Hour))
	writeFile(t, filepath.Join(root, "implicit", "bbbb.pb"), "x", now)

	got, err := discovery.ListConversationSessions(root)
	if err != nil {
		t.Fatalf("ListConversationSessions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 session, got %d: %+v", len(got), got)
	}
	if got[0].CascadeID != "aaaa" || got[0].Bucket != "conversations" {
		t.Errorf("got %+v", got[0])
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
	writeFile(t, filepath.Join(root, "conversations", "def.db"), "x", time.Now())

	got, ok, err := discovery.FindByID(root, "abc.pb")
	if err != nil || !ok {
		t.Fatalf("FindByID: ok=%v err=%v", ok, err)
	}
	if got.CascadeID != "abc" {
		t.Errorf("got %q want abc", got.CascadeID)
	}

	got, ok, err = discovery.FindByID(root, "def.db")
	if err != nil || !ok {
		t.Fatalf("FindByID: ok=%v err=%v", ok, err)
	}
	if got.CascadeID != "def" {
		t.Errorf("got %q want def", got.CascadeID)
	}

	got, ok, err = discovery.FindByID(root, "def")
	if err != nil || !ok {
		t.Fatalf("FindByID: ok=%v err=%v", ok, err)
	}
	if got.CascadeID != "def" {
		t.Errorf("got %q want def", got.CascadeID)
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

// Both surfaces log an HTTPS (gRPC) port line next to the HTTP one, and
// " for HTTPS (gRPC)" contains " for HTTP" as a substring. Discovery must
// skip the gRPC line even when it is the most recent match — the gRPC port
// does not speak the JSON dialect.
func TestDiscoverDaemonURLSkipsHTTPSLine(t *testing.T) {
	root := t.TempDir()

	live, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer live.Close()
	_, httpPort, err := net.SplitHostPort(live.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}

	logContent := "I0702 22:12:11.358803 32171 server.go:522] Language server listening on random port at " + httpPort + " for HTTP\n" +
		"I0702 22:12:11.358699 32171 server.go:514] Language server listening on random port at 40953 for HTTPS (gRPC)\n"
	if err := os.WriteFile(filepath.Join(root, "cli.log"), []byte(logContent), 0o644); err != nil {
		t.Fatalf("write cli.log: %v", err)
	}

	got, err := discovery.DiscoverDaemonURL(root)
	if err != nil {
		t.Fatalf("DiscoverDaemonURL: %v", err)
	}
	want := "http://127.0.0.1:" + httpPort
	if got != want {
		t.Errorf("got %q want %q (must not match the HTTPS/gRPC line)", got, want)
	}
}

// A log holding only the HTTPS (gRPC) line must not yield a port at all.
func TestDiscoverDaemonURLHTTPSOnlyIsNoMatch(t *testing.T) {
	root := t.TempDir()
	logContent := "Language server listening on random port at 40953 for HTTPS (gRPC)\n"
	if err := os.WriteFile(filepath.Join(root, "cli.log"), []byte(logContent), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := discovery.DiscoverDaemonURL(root)
	if err == nil {
		t.Fatal("expected error when only the HTTPS line is present")
	}
	if !strings.Contains(err.Error(), "no active HTTP daemon port") {
		t.Errorf("got error %v", err)
	}
}

func TestRootDefaultFallback(t *testing.T) {
	t.Setenv("ANTIGRAVITY_CLI_ROOT", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("UserHomeDir not available:", err)
	}
	expected := filepath.Join(home, discovery.DefaultRootSubpath)
	got, err := discovery.Root()
	if err != nil {
		t.Fatalf("Root failed: %v", err)
	}
	if got != expected {
		t.Errorf("got %q want %q", got, expected)
	}
}

func TestListSessionsSQLiteSidecars(t *testing.T) {
	root := t.TempDir()
	now := time.Now().Truncate(time.Second) // Truncate to avoid precision mismatch in some filesystems

	// 1) Test with -wal being newer
	db1 := filepath.Join(root, "conversations", "sess1.db")
	writeFile(t, db1, "x", now.Add(-10*time.Minute))
	writeFile(t, db1+"-wal", "x", now)

	// 2) Test with -journal being newer
	db2 := filepath.Join(root, "conversations", "sess2.db")
	writeFile(t, db2, "x", now.Add(-5*time.Minute))
	writeFile(t, db2+"-journal", "x", now.Add(5*time.Minute))

	// 3) Test with main db being newer (no sidecar / sidecar older)
	db3 := filepath.Join(root, "conversations", "sess3.db")
	writeFile(t, db3, "x", now.Add(10*time.Minute))
	writeFile(t, db3+"-wal", "x", now.Add(2*time.Minute))

	// 4) Empty (fully checkpointed) WAL with a newer mtime must be ignored:
	// the daemon touches -wal just by opening the db.
	db4 := filepath.Join(root, "conversations", "sess4.db")
	writeFile(t, db4, "x", now.Add(-20*time.Minute))
	writeFile(t, db4+"-wal", "", now.Add(20*time.Minute))

	got, err := discovery.ListSessions(root)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 sessions, got %d", len(got))
	}

	// Should be sorted by ModTime descending
	// Expected order:
	// sess3 (mtime = now + 10m)
	// sess2 (mtime from -journal = now + 5m)
	// sess1 (mtime from -wal = now)

	if got[0].CascadeID != "sess3" {
		t.Errorf("expected sess3 first, got %q", got[0].CascadeID)
	}
	if got[1].CascadeID != "sess2" {
		t.Errorf("expected sess2 second, got %q", got[1].CascadeID)
	}
	if got[2].CascadeID != "sess1" {
		t.Errorf("expected sess1 third, got %q", got[2].CascadeID)
	}

	// Verify exact ModTime picked up
	if !got[0].ModTime.Equal(now.Add(10 * time.Minute)) {
		t.Errorf("sess3 mtime got %v want %v", got[0].ModTime, now.Add(10*time.Minute))
	}
	if !got[1].ModTime.Equal(now.Add(5 * time.Minute)) {
		t.Errorf("sess2 mtime got %v want %v", got[1].ModTime, now.Add(5*time.Minute))
	}
	if !got[2].ModTime.Equal(now) {
		t.Errorf("sess1 mtime got %v want %v", got[2].ModTime, now)
	}
	if got[3].CascadeID != "sess4" {
		t.Errorf("expected sess4 last, got %q", got[3].CascadeID)
	}
	if !got[3].ModTime.Equal(now.Add(-20 * time.Minute)) {
		t.Errorf("sess4 mtime got %v want %v (empty -wal mtime must be ignored)", got[3].ModTime, now.Add(-20*time.Minute))
	}
}
