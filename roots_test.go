package main

import (
	"os"
	"path/filepath"
	"testing"
)

// resolveRoots is the single place run() turns --root flags, env, and store
// discovery into the ordered list of session roots to operate on. Explicit
// roots must win outright and suppress discovery entirely.
func TestResolveRootsExplicitWinInOrder(t *testing.T) {
	t.Setenv("ANTIGRAVITY_CLI_ROOT", "/env/should/be/ignored")
	got, err := resolveRoots([]string{"/a/cli-root", "/b/ide-root"})
	if err != nil {
		t.Fatalf("resolveRoots: %v", err)
	}
	if len(got) != 2 || got[0] != "/a/cli-root" || got[1] != "/b/ide-root" {
		t.Fatalf("explicit roots should pass through in order, got %v", got)
	}
}

// Passing the same root twice (e.g. a script appending --root to a fixed
// command line) must not double-list or double-sync it.
func TestResolveRootsDedupesExplicitDuplicates(t *testing.T) {
	got, err := resolveRoots([]string{"/a/root", "/a/root/", "/b/root"})
	if err != nil {
		t.Fatalf("resolveRoots: %v", err)
	}
	if len(got) != 2 || got[0] != "/a/root" || got[1] != "/b/root" {
		t.Fatalf("duplicate roots should collapse, got %v", got)
	}
}

// ANTIGRAVITY_CLI_ROOT is a live contract for existing CLI users: when set
// (and no --root), it must resolve to exactly that single root — no
// default-store discovery alongside it.
func TestResolveRootsEnvOverrideIsSingleRoot(t *testing.T) {
	t.Setenv("ANTIGRAVITY_CLI_ROOT", "/custom/root")
	got, err := resolveRoots(nil)
	if err != nil {
		t.Fatalf("resolveRoots: %v", err)
	}
	if len(got) != 1 || got[0] != "/custom/root" {
		t.Fatalf("env override should yield exactly that root, got %v", got)
	}
}

// The phase-2 default flip: with no --root and no ANTIGRAVITY_CLI_ROOT, a
// bare invocation operates on every known store that exists — CLI first,
// then the Antigravity 2.0 store.
func TestResolveRootsBareDiscoversBothStores(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ANTIGRAVITY_CLI_ROOT", "")
	cli := filepath.Join(home, ".gemini", "antigravity-cli")
	ide := filepath.Join(home, ".gemini", "antigravity")
	for _, d := range []string{cli, ide} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := resolveRoots(nil)
	if err != nil {
		t.Fatalf("resolveRoots: %v", err)
	}
	if len(got) != 2 || got[0] != cli || got[1] != ide {
		t.Fatalf("bare invocation should discover both stores CLI-first, got %v", got)
	}
}

// With no flags, no env, and neither default store on disk, the CLI default
// root is still the answer — a bare `agy-reader --list` on a fresh machine
// keeps printing "no sessions found under ~/.gemini/antigravity-cli".
func TestResolveRootsBareDefaultsToCLIRoot(t *testing.T) {
	home := t.TempDir() // neither store exists under this home
	t.Setenv("HOME", home)
	t.Setenv("ANTIGRAVITY_CLI_ROOT", "")
	got, err := resolveRoots(nil)
	if err != nil {
		t.Fatalf("resolveRoots: %v", err)
	}
	want := filepath.Join(home, ".gemini", "antigravity-cli")
	if len(got) != 1 || got[0] != want {
		t.Fatalf("bare invocation should default to the CLI root %s, got %v", want, got)
	}
}
