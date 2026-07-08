package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// Explicit configuration always wins: with ANTIGRAVITY_CLI_ROOT set, a bare
// invocation must operate on exactly that root — never grow extra stores —
// so existing scripts see zero behavior change.
func TestDefaultRootsEnvOverrideIsSingleRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mkdirAll(t, filepath.Join(home, ".gemini", "antigravity-cli"))
	mkdirAll(t, filepath.Join(home, ".gemini", "antigravity"))
	t.Setenv("ANTIGRAVITY_CLI_ROOT", "/custom/root")

	roots, err := DefaultRoots()
	if err != nil {
		t.Fatalf("DefaultRoots: %v", err)
	}
	if len(roots) != 1 || roots[0] != "/custom/root" {
		t.Fatalf("env override must yield exactly that root, got %v", roots)
	}
}

// With no explicit config, each known store that exists is included, CLI
// first.
func TestDefaultRootsIncludesEachExistingStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ANTIGRAVITY_CLI_ROOT", "")
	cli := filepath.Join(home, ".gemini", "antigravity-cli")
	ide := filepath.Join(home, ".gemini", "antigravity")
	mkdirAll(t, cli)
	mkdirAll(t, ide)

	roots, err := DefaultRoots()
	if err != nil {
		t.Fatalf("DefaultRoots: %v", err)
	}
	if len(roots) != 2 || roots[0] != cli || roots[1] != ide {
		t.Fatalf("want [%s %s], got %v", cli, ide, roots)
	}
}

// A machine with only one store gets only that store.
func TestDefaultRootsSingleExistingStore(t *testing.T) {
	for _, sub := range []string{"antigravity-cli", "antigravity"} {
		t.Run(sub, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("ANTIGRAVITY_CLI_ROOT", "")
			store := filepath.Join(home, ".gemini", sub)
			mkdirAll(t, store)

			roots, err := DefaultRoots()
			if err != nil {
				t.Fatalf("DefaultRoots: %v", err)
			}
			if len(roots) != 1 || roots[0] != store {
				t.Fatalf("want [%s], got %v", store, roots)
			}
		})
	}
}

// With neither store on disk, the CLI default is still the answer so a bare
// invocation keeps its "no sessions found under ~/.gemini/antigravity-cli"
// behavior on a fresh machine.
func TestDefaultRootsFallsBackToCLIDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ANTIGRAVITY_CLI_ROOT", "")

	roots, err := DefaultRoots()
	if err != nil {
		t.Fatalf("DefaultRoots: %v", err)
	}
	want := filepath.Join(home, ".gemini", "antigravity-cli")
	if len(roots) != 1 || roots[0] != want {
		t.Fatalf("want [%s], got %v", want, roots)
	}
}

// DefaultStoreRoots is the env-independent variant used to attribute a bare
// `--watch` process: what a bare invocation would cover with no env pin. The
// caller's own ANTIGRAVITY_CLI_ROOT must not leak into that answer.
func TestDefaultStoreRootsIgnoresEnvPin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ANTIGRAVITY_CLI_ROOT", "/pinned/elsewhere")
	cli := filepath.Join(home, ".gemini", "antigravity-cli")
	ide := filepath.Join(home, ".gemini", "antigravity")
	mkdirAll(t, cli)
	mkdirAll(t, ide)

	roots, err := DefaultStoreRoots()
	if err != nil {
		t.Fatalf("DefaultStoreRoots: %v", err)
	}
	if len(roots) != 2 || roots[0] != cli || roots[1] != ide {
		t.Fatalf("env pin must be ignored, want [%s %s], got %v", cli, ide, roots)
	}
}

// ~/.gemini is Gemini CLI's config home, NOT an Antigravity store directory:
// only the two known store locations may ever be picked up. The empty
// antigravity-ide/ dir some installs leave behind is a known red herring.
func TestDefaultRootsNeverScansGeminiHomeGenerically(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ANTIGRAVITY_CLI_ROOT", "")
	cli := filepath.Join(home, ".gemini", "antigravity-cli")
	mkdirAll(t, cli)
	mkdirAll(t, filepath.Join(home, ".gemini", "antigravity-ide")) // red herring
	mkdirAll(t, filepath.Join(home, ".gemini", "tmp"))             // unrelated Gemini CLI dir

	roots, err := DefaultRoots()
	if err != nil {
		t.Fatalf("DefaultRoots: %v", err)
	}
	if len(roots) != 1 || roots[0] != cli {
		t.Fatalf("only the known stores may be discovered, got %v", roots)
	}
}
