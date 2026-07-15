package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

func TestWriteJSONPreservesRawUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trajectory.json")
	traj := &daemon.Trajectory{
		CascadeID: "11111111-1111-1111-1111-111111111111",
		RawJSON:   []byte(`{"cascadeId":"11111111-1111-1111-1111-111111111111","future":{"large":900719925474099312345678901234567890},"steps":[]}`),
	}
	if err := writeJSON(path, traj); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"future"`, `900719925474099312345678901234567890`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("JSON output lost %s:\n%s", want, b)
		}
	}
}
