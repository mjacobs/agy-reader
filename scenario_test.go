package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The release-notes scenario for the phase-2 default flip (kata 83gs): an
// existing CLI user whose machine also has the Antigravity 2.0 store, with
// that IDE daemon down (IDE closed). Bare invocations now see both stores,
// but everything must stay quiet: watch keeps syncing the CLI root and
// treats the IDE root as waiting, and doctor exits 0 because a discovered
// store-exists-but-daemon-down root is WAITING, not failing.
func TestCLIOnlyUserWithIDEStorePresentStaysQuiet(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config")) // no IDE logs -> IDE daemon undiscoverable
	t.Setenv("ANTIGRAVITY_CLI_ROOT", "")
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "")
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "")

	cliRoot := filepath.Join(home, ".gemini", "antigravity-cli")
	ideRoot := filepath.Join(home, ".gemini", "antigravity")
	// CLI store: one session, live daemon advertised in cli.log.
	seedPB(t, cliRoot, "conversations", "cli-aaa", time.Now().Add(-time.Hour))
	srv := fakeDaemon(t, nil)
	defer srv.Close()
	advertiseCLIDaemon(t, cliRoot, srv.URL)
	// IDE store: present with an unsynced session, daemon down (IDE closed).
	seedDB(t, ideRoot, "conversations", "ide-bbb", time.Now().Add(-time.Hour), time.Time{})

	roots, err := resolveRoots(nil)
	if err != nil {
		t.Fatalf("resolveRoots: %v", err)
	}
	if len(roots) != 2 || roots[0] != cliRoot || roots[1] != ideRoot {
		t.Fatalf("bare invocation should discover both stores, got %v", roots)
	}

	// Watch: the CLI root syncs; the IDE root waits without failing the run.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runWatchLoop(ctx, roots, 20*time.Millisecond, 0) }()
	cliSidecar := filepath.Join(cliRoot, "conversations", "cli-aaa.trajectory.json")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(cliSidecar); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("CLI session never synced with the IDE store present but its daemon down")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("watch must exit nil on cancel, got %v", err)
	}

	// Doctor: the discovered IDE root (stale sidecar, daemon down) is
	// waiting, not failing — the run passes on the healthy CLI root.
	t.Setenv("PATH", "") // keep the agy-version line hermetic ("not on PATH")
	var buf bytes.Buffer
	if code := runDoctorTo(&buf, roots, false); code != 0 {
		t.Fatalf("doctor must exit 0 for a CLI-healthy machine with the IDE closed, got %d:\n%s", code, buf.String())
	}
	out := buf.String()
	// It still reports per-surface so the waiting root is visible.
	if !strings.Contains(out, "surface:     cli") || !strings.Contains(out, "surface:     ide") {
		t.Fatalf("doctor should report both surfaces:\n%s", out)
	}
}
