package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCollectSidecarsTreatsDirectoryMetacharactersLiterally(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "[audit]?corpus")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(dir, "a.trajectory.json")
	b := filepath.Join(dir, "b.trajectory.json")
	for _, path := range []string{b, a, filepath.Join(dir, "ignore.json")} {
		if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "nested.trajectory.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := collectSidecars([]string{dir, b})
	if err != nil {
		t.Fatalf("collectSidecars: %v", err)
	}
	want := []string{a, b}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectSidecars() = %v, want %v", got, want)
	}
}
