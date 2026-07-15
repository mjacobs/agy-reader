package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/cache"
)

func TestBackfillParentLinksCommand(t *testing.T) {
	const (
		parent = "11111111-1111-1111-1111-111111111111"
		child  = "22222222-2222-2222-2222-222222222222"
	)
	root := t.TempDir()
	dir := filepath.Join(root, "conversations")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFileT(t, filepath.Join(dir, parent+".trajectory.json"), []byte(`{"cascadeId":"`+parent+`","steps":[]}`))
	writeFileT(t, filepath.Join(dir, child+".trajectory.json"), []byte(`{
  "cascadeId":"`+child+`",
  "executorMetadatas":[{"cascadeConfig":{"plannerConfig":{"customizationConfig":{"agentPath":"file:///x/brain/`+parent+`/.agents/agents/child"}}}}],
  "steps":[]
}`))

	var out bytes.Buffer
	if err := runBackfillParentLinksTo([]string{"--root", root}, &out); err != nil {
		t.Fatalf("runBackfillParentLinksTo: %v", err)
	}
	if !strings.Contains(out.String(), "stamped=1") {
		t.Fatalf("missing stamped summary:\n%s", out.String())
	}
	traj, err := cache.Read(filepath.Join(dir, child+".trajectory.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := traj.ParentCascadeID(); got != parent {
		t.Errorf("parent = %q, want %q", got, parent)
	}

	out.Reset()
	if err := runBackfillParentLinksTo([]string{"--root", root}, &out); err != nil {
		t.Fatalf("second runBackfillParentLinksTo: %v", err)
	}
	if !strings.Contains(out.String(), "stamped=0") {
		t.Fatalf("second run was not a no-op:\n%s", out.String())
	}
}
