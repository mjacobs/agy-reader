//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || windows

package watcher

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/discovery"
)

// The helper is a separate process so the test exercises kernel ownership,
// including automatic release after a crash, rather than an in-process mutex.
func TestWatchLockProcess(t *testing.T) {
	root := os.Getenv("AGY_READER_TEST_LOCK_ROOT")
	if root == "" {
		return
	}
	f, err := acquireWatchLock(root)
	if err != nil || f == nil {
		t.Fatalf("acquire lock: %v", err)
	}
	defer f.Close()
	fmt.Println("locked")
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func TestRunLoopWaitsForOtherProcessAndTakesOverAfterCrash(t *testing.T) {
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root, other := t.TempDir(), t.TempDir()
	seedPB(t, root, "conversations", "aaa", time.Now())
	seedPB(t, other, "conversations", "bbb", time.Now())
	var sharedFetches atomic.Int32
	srv := fakeDaemon(t, func(id string) {
		if id == "aaa" {
			sharedFetches.Add(1)
		}
	})
	defer srv.Close()
	advertiseCLIDaemon(t, root, srv.URL)
	advertiseCLIDaemon(t, other, srv.URL)

	owner := exec.Command(os.Args[0], "-test.run=^TestWatchLockProcess$")
	owner.Env = append(os.Environ(), "AGY_READER_TEST_LOCK_ROOT="+root, "XDG_CACHE_HOME="+t.TempDir())
	stdin, err := owner.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := owner.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	owner.Stderr = os.Stderr
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		if owner.ProcessState == nil {
			_ = owner.Process.Kill()
			_ = owner.Wait()
		}
	})
	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		ready <- scanner.Scan() && scanner.Text() == "locked"
	}()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("owner did not acquire the lock")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out starting the owner")
	}

	// Simulate a sidecar written under an incompatible verification profile.
	// A waiting watcher must leave it alone even though it would fail its check.
	sidecar := filepath.Join(root, "conversations", "aaa.trajectory.json")
	foreign := `{"cascadeId":"aaa","agyReader":{"completeness":{"profile":"other-version"}}}`
	if err := os.WriteFile(sidecar, []byte(foreign), 0o600); err != nil {
		t.Fatal(err)
	}
	watchRoot := root
	alias := filepath.Join(t.TempDir(), "store-alias")
	if err := os.Symlink(root, alias); err == nil {
		watchRoot = alias
	} else if os.PathSeparator != '\\' {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- RunLoop(ctx, []string{watchRoot, other}, 10*time.Millisecond, 30*time.Millisecond) }()
	t.Cleanup(cancel)
	waitForWatchFile(t, filepath.Join(other, "conversations", "bbb.trajectory.json"), "\"steps\"")
	time.Sleep(100 * time.Millisecond) // several ticks and idle timeouts
	data, err := os.ReadFile(sidecar)
	if err != nil || string(data) != foreign || sharedFetches.Load() != 0 {
		t.Fatalf("waiting watcher touched the shared store: data=%s fetches=%d err=%v", data, sharedFetches.Load(), err)
	}
	select {
	case err := <-done:
		t.Fatalf("waiting watcher exited: %v", err)
	default:
	}

	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = owner.Wait()
	waitForWatchFile(t, sidecar, "\"sourceDigest\"")
	time.Sleep(60 * time.Millisecond)
	// One refresh fetches both the full and the presentation RPC response.
	if got := sharedFetches.Load(); got != 2 {
		t.Fatalf("takeover made %d trajectory RPCs, want 2 for one refresh", got)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("waiting watcher did not cancel")
	}
	f, err := acquireWatchLock(root)
	if err != nil || f == nil {
		t.Fatalf("lock was not released after cancellation: %v", err)
	}
	_ = f.Close()
}

func waitForWatchFile(t *testing.T, path, contains string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), contains) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s never contained %s", path, contains)
}

func TestRunLoopDeduplicatesRootAliases(t *testing.T) {
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "")
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	if err := RunLoop(ctx, []string{root, alias}, 10*time.Millisecond, 30*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() != nil {
		t.Fatal("duplicate root waited on its own lock instead of idle-exiting")
	}
}

func TestRunLoopFailsWhenStoreCannotBeLocked(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, watchLockName), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := RunLoop(t.Context(), []string{root}, time.Second, 0); err == nil {
		t.Fatal("watcher ran without acquiring its store lock")
	}
}

func TestRunLoopPreservesSymlinkedDefaultIDERoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "")
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("APPDATA", filepath.Join(home, "config"))
	store := t.TempDir()
	if err := os.Symlink(store, filepath.Join(home, ".gemini")); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	root := filepath.Join(home, discovery.DefaultIDERootSubpath)
	seedPB(t, root, "conversations", "ide-session", time.Now())
	srv := fakeDaemon(t, nil)
	defer srv.Close()
	logsDir, err := discovery.IDELogsDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(logsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")
	if err := os.WriteFile(filepath.Join(logsDir, "language_server.log"), []byte("Language server listening on random port at "+port+" for HTTP\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- RunLoop(ctx, []string{root}, 10*time.Millisecond, 0) }()
	waitForWatchFile(t, filepath.Join(root, "conversations", "ide-session.trajectory.json"), "\"sourceDigest\"")
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
