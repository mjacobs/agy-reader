package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// One watch process drives one loop per root: a root whose daemon is up must
// keep syncing even while another root's daemon has never appeared — the
// pending root polls quietly and never takes the process down.
func TestRunWatchLoopMultiRootSyncsLiveRootWhileOtherPending(t *testing.T) {
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "")
	liveRoot := t.TempDir()
	seedPB(t, liveRoot, "conversations", "aaa", time.Now().Add(-time.Hour))
	srv := fakeDaemon(t, nil)
	defer srv.Close()
	advertiseCLIDaemon(t, liveRoot, srv.URL)

	pendingRoot := t.TempDir() // no cli.log: its daemon is never found

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunLoop(ctx, []string{liveRoot, pendingRoot}, 20*time.Millisecond, 0)
	}()

	sidecar := filepath.Join(liveRoot, "conversations", "aaa.trajectory.json")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(sidecar); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("live root's session never synced while the other root was pending")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil after cancel, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("loop did not return after context cancellation")
	}
}

// An idle-expired root is not retired: while another root keeps the process
// alive, a root whose daemon was down past the idle timeout must resume
// polling — and syncing — when its daemon comes back later.
func TestRunWatchLoopIdleExpiredRootRevives(t *testing.T) {
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "")
	liveRoot := t.TempDir()
	liveSrv := fakeDaemon(t, nil)
	defer liveSrv.Close()
	advertiseCLIDaemon(t, liveRoot, liveSrv.URL)

	lateRoot := t.TempDir() // daemon absent long enough to trip the idle timeout
	seedPB(t, lateRoot, "conversations", "late-aaa", time.Now().Add(-time.Hour))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunLoop(ctx, []string{liveRoot, lateRoot}, 20*time.Millisecond, 60*time.Millisecond)
	}()

	// Let the late root's idle timeout expire several times over.
	time.Sleep(200 * time.Millisecond)

	// Its daemon finally comes up. The watcher must still be polling this
	// root and sync the backlog.
	lateSrv := fakeDaemon(t, nil)
	defer lateSrv.Close()
	advertiseCLIDaemon(t, lateRoot, lateSrv.URL)

	sidecar := filepath.Join(lateRoot, "conversations", "late-aaa.trajectory.json")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(sidecar); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("idle-expired root was retired: its session never synced after the daemon came back")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil after cancel, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("loop did not return after context cancellation")
	}
}

// The idle-timeout is per-process only when every root is idle: a root whose
// daemon never appears must not shut the process down while another root's
// daemon is alive and being watched.
func TestRunWatchLoopIdleTimeoutRequiresAllRootsIdle(t *testing.T) {
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "")
	liveRoot := t.TempDir()
	srv := fakeDaemon(t, nil)
	defer srv.Close()
	advertiseCLIDaemon(t, liveRoot, srv.URL)

	deadRoot := t.TempDir() // daemon never appears: idle every tick

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunLoop(ctx, []string{liveRoot, deadRoot}, 20*time.Millisecond, 60*time.Millisecond)
	}()

	// Many multiples of the idle timeout elapse; with one live root the loop
	// must still be running.
	select {
	case err := <-done:
		t.Fatalf("loop exited while one root's daemon was still alive (err=%v)", err)
	case <-time.After(300 * time.Millisecond):
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil after cancel, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("loop did not return after context cancellation")
	}
}
