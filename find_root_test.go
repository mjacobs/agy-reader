package main

import (
	"context"
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

// An id on no root's disk is probed against each root's daemon in order —
// daemon-only sessions living behind a later root's daemon must still be
// fetched, not bound to the first root up front.
func TestFetchByIDProbesLaterRootsForDaemonOnlySessions(t *testing.T) {
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "")
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "")
	rootA := t.TempDir() // no cli.log: daemon undiscoverable
	rootB := t.TempDir()
	srv := fakeDaemon(t, nil)
	defer srv.Close()
	advertiseCLIDaemon(t, rootB, srv.URL)

	traj, sidecarPath, err := fetchByID(t.Context(), []string{rootA, rootB}, "daemon-only")
	if err != nil {
		t.Fatalf("fetchByID should have probed rootB's daemon: %v", err)
	}
	if traj == nil || traj.CascadeID != "daemon-only" {
		t.Fatalf("unexpected trajectory: %+v", traj)
	}
	if sidecarPath != "" {
		t.Errorf("a daemon-only session has no sidecar location, got %q", sidecarPath)
	}
}

// With no root able to serve the id, fetchByID errors instead of silently
// succeeding.
func TestFetchByIDErrorsWhenNoRootServes(t *testing.T) {
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "")
	rootA := t.TempDir()
	rootB := t.TempDir()

	if _, _, err := fetchByID(context.Background(), []string{rootA, rootB}, "nowhere"); err == nil {
		t.Fatal("expected an error when no root's daemon serves the id")
	}
}

// An id found on disk binds the fetch to its root: daemon, CSRF config, and
// sidecar location all follow that root.
func TestFetchByIDUsesOwningRootForOnDiskSessions(t *testing.T) {
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "")
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "")
	rootA := t.TempDir()
	rootB := t.TempDir()
	seedPB(t, rootB, "conversations", "on-disk", time.Now())
	srv := fakeDaemon(t, nil)
	defer srv.Close()
	advertiseCLIDaemon(t, rootB, srv.URL)

	traj, sidecarPath, err := fetchByID(t.Context(), []string{rootA, rootB}, "on-disk")
	if err != nil {
		t.Fatalf("fetchByID: %v", err)
	}
	if traj == nil || traj.CascadeID != "on-disk" {
		t.Fatalf("unexpected trajectory: %+v", traj)
	}
	want := filepath.Join(rootB, "conversations", "on-disk.trajectory.json")
	if sidecarPath != want {
		t.Errorf("sidecar path %q, want %q", sidecarPath, want)
	}
}
