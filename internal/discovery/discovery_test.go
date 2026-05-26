package discovery_test

import (
	"os"
	"path/filepath"
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
