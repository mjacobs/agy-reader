package discovery

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
)

// failProvidersRead replaces the providers-directory read. The listing for
// dir returns what os.ReadDir found, cut to the first keep entries, then EIO,
// the way os.ReadDir reports a directory whose read fails partway. Every
// other path reads normally.
func failProvidersRead(t *testing.T, dir string, keep int) {
	t.Helper()
	orig := readProvidersDir
	readProvidersDir = func(name string) ([]os.DirEntry, error) {
		entries, err := orig(name)
		if name != dir || err != nil {
			return entries, err
		}
		return entries[:min(keep, len(entries))], &fs.PathError{Op: "readdirent", Path: name, Err: syscall.EIO}
	}
	t.Cleanup(func() { readProvidersDir = orig })
}

// A providers-directory read that yields an instance and then fails must keep
// that instance's store when the store itself is still accessible.
func TestDefaultStoreRootsUnderKeepsStoresListedBeforeReadError(t *testing.T) {
	home := t.TempDir()
	providers := filepath.Join(home, acpProvidersSubpath)
	store := filepath.Join(providers, "abcd", "antigravity-acp")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	failProvidersRead(t, providers, 1)

	got := DefaultStoreRootsUnder(home)
	if want := []string{store}; !slices.Equal(got, want) {
		t.Fatalf("DefaultStoreRootsUnder = %v, want %v", got, want)
	}
}

// A providers-directory read that fails before yielding anything still lists
// nothing, so a home with no other store falls back to the CLI default.
func TestDefaultStoreRootsUnderProvidersReadErrorBeforeAnyEntry(t *testing.T) {
	home := t.TempDir()
	providers := filepath.Join(home, acpProvidersSubpath)
	if err := os.MkdirAll(filepath.Join(providers, "abcd", "antigravity-acp"), 0o755); err != nil {
		t.Fatal(err)
	}
	failProvidersRead(t, providers, 0)

	got := DefaultStoreRootsUnder(home)
	if want := []string{filepath.Join(home, DefaultRootSubpath)}; !slices.Equal(got, want) {
		t.Fatalf("DefaultStoreRootsUnder = %v, want %v", got, want)
	}
}
