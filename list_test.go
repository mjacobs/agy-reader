package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// markIDERoot makes a temp dir detect as the IDE surface via the IDE-only
// state marker file, without touching the real home directory.
func markIDERoot(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "antigravity_state.pbtxt"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
}

// markCLIRoot gives a temp dir the cli.log that authoritatively marks a CLI
// root.
func markCLIRoot(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "cli.log"), []byte("log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Single-root --list output is a live contract with existing CLI users'
// scripts: no surface column, exact line shape unchanged.
func TestRunListSingleRootFormatUnchanged(t *testing.T) {
	root := t.TempDir()
	markCLIRoot(t, root)
	mtime := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	seedPB(t, root, "conversations", "aaa", mtime)

	var out, msg bytes.Buffer
	if err := runListTo(&out, &msg, []string{root}, false); err != nil {
		t.Fatalf("runListTo: %v", err)
	}
	// Stat round-trips the mtime through the filesystem in the local zone.
	want := "  aaa  " + mtime.Local().Format(time.RFC3339) + "  (conversations)\n"
	if out.String() != want {
		t.Fatalf("single-root line format changed:\ngot:  %q\nwant: %q", out.String(), want)
	}
}

// With more than one root, --list aggregates sessions across all roots,
// newest first, and gains a surface column so IDE and CLI sessions are
// distinguishable.
func TestRunListMultiRootAggregatesWithSurfaceColumn(t *testing.T) {
	cliRoot := t.TempDir()
	markCLIRoot(t, cliRoot)
	ideRoot := t.TempDir()
	markIDERoot(t, ideRoot)

	older := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 7, 8, 11, 0, 0, 0, time.UTC)
	seedPB(t, cliRoot, "conversations", "cli-session", older)
	seedPB(t, ideRoot, "conversations", "ide-session", newer)

	var out, msg bytes.Buffer
	if err := runListTo(&out, &msg, []string{cliRoot, ideRoot}, false); err != nil {
		t.Fatalf("runListTo: %v", err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 aggregated lines, got %d:\n%s", len(lines), out.String())
	}
	// Newest first across roots: the IDE session (newer) leads.
	if !strings.Contains(lines[0], "ide-session") || !strings.HasSuffix(lines[0], "ide") {
		t.Errorf("first line should be the newer ide session with an ide surface column, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "cli-session") || !strings.HasSuffix(lines[1], "cli") {
		t.Errorf("second line should be the cli session with a cli surface column, got %q", lines[1])
	}
}

// An empty multi-root listing must name every root it looked under, so the
// "nothing found" message stays actionable when two stores are in play.
func TestRunListMultiRootEmptyNamesAllRoots(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()

	var out, msg bytes.Buffer
	if err := runListTo(&out, &msg, []string{rootA, rootB}, false); err != nil {
		t.Fatalf("runListTo: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no session lines, got %q", out.String())
	}
	for _, root := range []string{rootA, rootB} {
		if !strings.Contains(msg.String(), root) {
			t.Errorf("empty-list message should name %s, got %q", root, msg.String())
		}
	}
}
