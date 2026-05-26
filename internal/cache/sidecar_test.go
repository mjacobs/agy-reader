package cache_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/daemon"
)

func TestSidecarRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abc.trajectory.json")

	in := &daemon.Trajectory{
		CascadeID: "abc",
		Source:    "CORTEX_TRAJECTORY_SOURCE_CLI",
		Steps: []daemon.Step{
			{
				Type:      "CORTEX_STEP_TYPE_USER_INPUT",
				Status:    "COMPLETED",
				UserInput: &daemon.UserInput{UserResponse: "hello world"},
			},
			{
				Type:   "CORTEX_STEP_TYPE_RUN_COMMAND",
				Status: "COMPLETED",
				RunCommand: &daemon.RunCommand{
					CommandLine:    "ls",
					Cwd:            "/tmp",
					CombinedOutput: []byte(`"a\nb\n"`),
				},
			},
		},
	}
	if cache.Exists(path) {
		t.Fatal("sidecar should not exist yet")
	}
	if err := cache.Write(path, in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !cache.Exists(path) {
		t.Fatal("sidecar should exist after write")
	}
	out, err := cache.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if out.CascadeID != "abc" || len(out.Steps) != 2 {
		t.Fatalf("unexpected trajectory: %+v", out)
	}
	if out.Steps[0].UserInput == nil || out.Steps[0].UserInput.UserResponse != "hello world" {
		t.Errorf("user input lost: %+v", out.Steps[0])
	}
	if out.Steps[1].RunCommand == nil || out.Steps[1].RunCommand.CommandLine != "ls" {
		t.Errorf("run command lost: %+v", out.Steps[1])
	}
}

func TestSidecarReadMissing(t *testing.T) {
	_, err := cache.Read(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got %v", err)
	}
}

func TestWriteNil(t *testing.T) {
	err := cache.Write(filepath.Join(t.TempDir(), "x.json"), nil)
	if err == nil {
		t.Fatal("expected error for nil")
	}
}
