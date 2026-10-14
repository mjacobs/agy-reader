package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var errStubCoverage = errors.New("stub: permission denied")

func nulArgv(args ...string) []byte {
	return []byte(strings.Join(args, "\x00") + "\x00")
}

// The watch detector must surface which roots a watcher process was launched
// for, so doctor can attribute "watch: running" to the right root instead of
// reporting any watcher as covering every root.
func TestCmdlineWatchRoots(t *testing.T) {
	roots, ok := CmdlineWatchRoots(nulArgv("agy-reader", "--root", "/a", "--root", "/b", "--watch"))
	if !ok {
		t.Fatal("expected a watcher")
	}
	if len(roots) != 2 || roots[0] != "/a" || roots[1] != "/b" {
		t.Fatalf("expected explicit roots [/a /b], got %v", roots)
	}

	roots, ok = CmdlineWatchRoots(nulArgv("agy-reader", "--watch"))
	if !ok {
		t.Fatal("expected a watcher")
	}
	if len(roots) != 0 {
		t.Fatalf("a bare watcher has no explicit roots, got %v", roots)
	}

	if _, ok := CmdlineWatchRoots(nulArgv("agy-reader", "doctor")); ok {
		t.Fatal("doctor invocation is not a watcher")
	}
}

// A watcher covers a root when launched with that --root explicitly; with an
// ANTIGRAVITY_CLI_ROOT env pin it covers exactly that root; a truly bare
// watcher covers the default store roots.
func TestWatchCoversRoot(t *testing.T) {
	defaults := []string{"/home/x/.gemini/antigravity-cli", "/home/x/.gemini/antigravity"}

	if !WatchCoversRoot([]string{"/a", "/b/"}, "", "/b", defaults) {
		t.Error("explicit --root /b/ should cover /b")
	}
	if WatchCoversRoot([]string{"/a"}, "", "/b", defaults) {
		t.Error("a watcher pinned to /a must not cover /b")
	}
	if !WatchCoversRoot(nil, "", "/home/x/.gemini/antigravity", defaults) {
		t.Error("a bare watcher covers the discovered default roots")
	}
	if WatchCoversRoot(nil, "", "/elsewhere", defaults) {
		t.Error("a bare watcher does not cover a non-default root")
	}
	if !WatchCoversRoot(nil, "/custom", "/custom", defaults) {
		t.Error("an env-pinned bare watcher covers its pinned root")
	}
	if WatchCoversRoot(nil, "/custom", "/home/x/.gemini/antigravity", defaults) {
		t.Error("an env-pinned bare watcher does not cover the default stores")
	}
	if !WatchCoversRoot([]string{"/a"}, "/custom", "/a", defaults) {
		t.Error("explicit --root wins over an env pin (run() ignores env when --root is passed)")
	}
}

// envValueFromProcEnviron extracts one variable from NUL-separated
// /proc/<pid>/environ contents.
func TestEnvValueFromProcEnviron(t *testing.T) {
	environ := []byte("HOME=/home/x\x00ANTIGRAVITY_CLI_ROOT=/custom\x00LANG=C\x00")
	if got := envValueFromProcEnviron(environ, "ANTIGRAVITY_CLI_ROOT"); got != "/custom" {
		t.Errorf("got %q, want /custom", got)
	}
	if got := envValueFromProcEnviron(environ, "HOME"); got != "/home/x" {
		t.Errorf("got %q, want /home/x", got)
	}
	if got := envValueFromProcEnviron([]byte("HOME=/home/x\x00"), "ANTIGRAVITY_CLI_ROOT"); got != "" {
		t.Errorf("unset pin should be empty, got %q", got)
	}
}

// watchProcCoverage decides one watch process's coverage of a root from its
// argv and its OWN environment — never this process's env or HOME. When the
// environment is needed but unavailable, coverage is indeterminate rather
// than assumed.
func TestWatchProcCoverage(t *testing.T) {
	watcherHome := t.TempDir()
	store := filepath.Join(watcherHome, ".gemini", "antigravity")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	environ := func(vars ...string) []byte {
		return []byte(strings.Join(vars, "\x00") + "\x00")
	}

	// Explicit --root: argv is authoritative, environ irrelevant.
	covered, indeterminate := WatchProcCoverage([]string{"/a"}, nil, errStubCoverage, "/a")
	if !covered || indeterminate {
		t.Errorf("explicit --root should cover without environ, got covered=%v indeterminate=%v", covered, indeterminate)
	}

	// Env-pinned bare watcher covers exactly its pin.
	covered, indeterminate = WatchProcCoverage(nil, environ("HOME="+watcherHome, "ANTIGRAVITY_CLI_ROOT=/custom"), nil, "/custom")
	if !covered || indeterminate {
		t.Errorf("env pin should cover its root, got covered=%v indeterminate=%v", covered, indeterminate)
	}

	// Truly bare watcher: defaults derive from the WATCHER's HOME.
	covered, indeterminate = WatchProcCoverage(nil, environ("HOME="+watcherHome), nil, store)
	if !covered || indeterminate {
		t.Errorf("bare watcher should cover its own home's store, got covered=%v indeterminate=%v", covered, indeterminate)
	}
	covered, _ = WatchProcCoverage(nil, environ("HOME="+watcherHome), nil, "/elsewhere")
	if covered {
		t.Error("bare watcher must not cover an unrelated root")
	}

	// Unreadable environ on a bare watcher: indeterminate, never assumed.
	if _, indeterminate = WatchProcCoverage(nil, nil, errStubCoverage, store); !indeterminate {
		t.Error("unreadable environ must be indeterminate")
	}
	// Environ readable but HOME absent: also indeterminate.
	if _, indeterminate = WatchProcCoverage(nil, environ("LANG=C"), nil, store); !indeterminate {
		t.Error("bare watcher with no HOME must be indeterminate")
	}
}
