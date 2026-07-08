package main

import (
	"strings"
	"testing"
)

func nulArgv(args ...string) []byte {
	return []byte(strings.Join(args, "\x00") + "\x00")
}

// The watch detector must surface which roots a watcher process was launched
// for, so doctor can attribute "watch: running" to the right root instead of
// reporting any watcher as covering every root.
func TestCmdlineWatchRoots(t *testing.T) {
	roots, ok := cmdlineWatchRoots(nulArgv("agy-reader", "--root", "/a", "--root", "/b", "--watch"))
	if !ok {
		t.Fatal("expected a watcher")
	}
	if len(roots) != 2 || roots[0] != "/a" || roots[1] != "/b" {
		t.Fatalf("expected explicit roots [/a /b], got %v", roots)
	}

	roots, ok = cmdlineWatchRoots(nulArgv("agy-reader", "--watch"))
	if !ok {
		t.Fatal("expected a watcher")
	}
	if len(roots) != 0 {
		t.Fatalf("a bare watcher has no explicit roots, got %v", roots)
	}

	if _, ok := cmdlineWatchRoots(nulArgv("agy-reader", "doctor")); ok {
		t.Fatal("doctor invocation is not a watcher")
	}
}

// A watcher covers a root when launched with that --root explicitly; a bare
// watcher covers exactly the discovered default roots.
func TestWatchCoversRoot(t *testing.T) {
	defaults := []string{"/home/x/.gemini/antigravity-cli", "/home/x/.gemini/antigravity"}

	if !watchCoversRoot([]string{"/a", "/b/"}, "/b", defaults) {
		t.Error("explicit --root /b/ should cover /b")
	}
	if watchCoversRoot([]string{"/a"}, "/b", defaults) {
		t.Error("a watcher pinned to /a must not cover /b")
	}
	if !watchCoversRoot(nil, "/home/x/.gemini/antigravity", defaults) {
		t.Error("a bare watcher covers the discovered default roots")
	}
	if watchCoversRoot(nil, "/elsewhere", defaults) {
		t.Error("a bare watcher does not cover a non-default root")
	}
}
