package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/cli"
	"github.com/mjacobs/agy-reader/internal/doctor"
	"github.com/mjacobs/agy-reader/internal/watcher"
)

// A healthy CLI root cannot hide unknown SQLite coverage in a discovered IDE
// store whose daemon is closed. Watch keeps serving the CLI root and waiting
// for the IDE; doctor fails until the IDE sidecars have completeness evidence.
func TestCLIHealthyRootCannotHideUnknownSQLiteCoverage(t *testing.T) {
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

	roots, err := cli.ResolveRoots(nil)
	if err != nil {
		t.Fatalf("resolveRoots: %v", err)
	}
	if len(roots) != 2 || roots[0] != cliRoot || roots[1] != ideRoot {
		t.Fatalf("bare invocation should discover both stores, got %v", roots)
	}

	// Watch: the CLI root syncs; the IDE root waits without failing the run.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- watcher.RunLoop(ctx, roots, 20*time.Millisecond, 0) }()
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

	// Doctor: unknown SQLite completeness is actionable even in a discovered
	// root whose daemon is down; a healthy legacy root cannot hide it.
	t.Setenv("PATH", "") // keep the agy-version line hermetic ("not on PATH")
	var buf bytes.Buffer
	if code := doctor.RunTo(&buf, roots, false); code != 1 {
		t.Fatalf("doctor must expose unknown SQLite coverage even with a healthy CLI root, got %d:\n%s", code, buf.String())
	}
	out := buf.String()
	// It still reports per-surface so the waiting root is visible.
	if !strings.Contains(out, "surface:     cli") || !strings.Contains(out, "surface:     ide") {
		t.Fatalf("doctor should report both surfaces:\n%s", out)
	}
}

// A bare --watch discovers T3 stores alongside the writable CLI store. New
// sessions must keep syncing after the watcher encounters the read-only store.
func TestBareWatchContinuesWithReadOnlyT3Store(t *testing.T) {
	home := t.TempDir()
	cliRoot := filepath.Join(home, ".gemini", "antigravity-cli")
	t3Root := filepath.Join(home, ".t3", "userdata", "providers", "antigravity", "instance", "antigravity-acp")
	if err := os.MkdirAll(filepath.Join(cliRoot, "conversations"), 0700); err != nil {
		t.Fatal(err)
	}
	qualifySource(t, t3Root, completeID, false, time.Now())
	if err := os.Chmod(t3Root, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(t3Root, 0700) })
	probe, err := os.CreateTemp(t3Root, "permission-probe")
	if err == nil {
		probe.Close()
		t.Skip("the current account can write under a mode-0555 directory")
	}
	if !os.IsPermission(err) {
		t.Fatalf("checking read-only directory: %v", err)
	}

	// Publish a complete SQLite file atomically once the lock failure has
	// been reported, so even a fast startup tick cannot have synced it yet.
	staging := t.TempDir()
	qualifySource(t, staging, qualifyVerifiedID, false, time.Now())
	srv := qualifyDaemon(t, completeID, qualifyVerifiedID)
	advertiseCLIDaemon(t, cliRoot, srv.URL)
	env := append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "AGY_READER_TEST_AS_CLI=1",
		"ANTIGRAVITY_CLI_ROOT=", "ANTIGRAVITY_DAEMON_URL=", "AGY_READER_PAIR_VERIFIER=")
	logs, exited := startWatchCLI(t, env, "--watch-interval=25ms")
	waitForLog(t, logs, exited, "the read-only T3 lock diagnostic", func(log string) bool {
		return strings.Contains(log, filepath.Join(t3Root, ".agy-reader-watch.lock"))
	})

	db := filepath.Join("conversations", qualifyVerifiedID+".db")
	if err := os.Rename(filepath.Join(staging, db), filepath.Join(cliRoot, db)); err != nil {
		t.Fatal(err)
	}
	checkSync := func(root, id string) {
		t.Helper()
		waitForLog(t, logs, exited, "sync of "+id, func(log string) bool {
			return strings.Contains(log, "synced "+id)
		})
		sidecar := filepath.Join(root, "conversations", id+".trajectory.json")
		transcript, err := cache.Read(sidecar)
		if err != nil {
			t.Fatal(err)
		}
		if transcript.CascadeID != id || len(transcript.Steps) != 1 {
			t.Fatalf("sidecar has conversation %s and %d steps, want %s and one step", transcript.CascadeID, len(transcript.Steps), id)
		}
	}
	checkSync(cliRoot, qualifyVerifiedID)
	if _, err := os.Stat(filepath.Join(t3Root, "conversations", completeID+".trajectory.json")); !os.IsNotExist(err) {
		t.Fatalf("T3 sidecar should be absent while its store is unlocked: %v", err)
	}

	if err := os.Chmod(t3Root, 0700); err != nil {
		t.Fatal(err)
	}
	// A T3 server announces its HTTP port in its own log when it starts.
	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")
	writeFileT(t, filepath.Join(t3Root, "log", "acp-server-fixture.log"),
		[]byte("Language server listening on random port at "+port+" for HTTP\n"))
	checkSync(t3Root, completeID)
	if count := strings.Count(logs.String(), "cannot write store lock for "+t3Root); count != 1 {
		t.Fatalf("got %d warnings for one unwritable store, want 1", count)
	}
}
