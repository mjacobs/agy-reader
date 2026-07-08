package main

import (
	"path/filepath"
	"testing"
	"time"
)

// Fetching by id with multiple roots must resolve against the root that
// actually holds the session, searching in root order.
func TestFindSessionRootSearchesRootsInOrder(t *testing.T) {
	cliRoot := t.TempDir()
	ideRoot := t.TempDir()
	seedPB(t, ideRoot, "conversations", "ide-only", time.Now())

	root, s, found, err := findSessionRoot([]string{cliRoot, ideRoot}, "ide-only")
	if err != nil {
		t.Fatalf("findSessionRoot: %v", err)
	}
	if !found {
		t.Fatal("expected the session to be found in the second root")
	}
	if root != ideRoot {
		t.Errorf("got root %q, want the ide root %q", root, ideRoot)
	}
	want := filepath.Join(ideRoot, "conversations", "ide-only.trajectory.json")
	if s.SidecarPath != want {
		t.Errorf("sidecar path %q, want %q", s.SidecarPath, want)
	}
}

// When the same id exists in more than one root, the earlier root wins —
// root order is the documented tie-break.
func TestFindSessionRootFirstRootWinsOnTie(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	seedPB(t, rootA, "conversations", "both", time.Now())
	seedPB(t, rootB, "conversations", "both", time.Now())

	root, _, found, err := findSessionRoot([]string{rootA, rootB}, "both")
	if err != nil {
		t.Fatalf("findSessionRoot: %v", err)
	}
	if !found || root != rootA {
		t.Fatalf("expected first root %q to win, got found=%v root=%q", rootA, found, root)
	}
}

// An id on no root's disk still resolves to the first root so the daemon
// fetch can be attempted there — preserving the single-root behavior of
// fetching ids that only exist daemon-side.
func TestFindSessionRootFallsBackToFirstRoot(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()

	root, _, found, err := findSessionRoot([]string{rootA, rootB}, "nowhere")
	if err != nil {
		t.Fatalf("findSessionRoot: %v", err)
	}
	if found {
		t.Fatal("expected found=false for an id on no disk")
	}
	if root != rootA {
		t.Errorf("fallback root should be the first root %q, got %q", rootA, root)
	}
}
