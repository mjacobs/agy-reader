package daemon

import (
	"encoding/json"
	"testing"
)

// execMetaJSON builds an executorMetadatas array carrying one agentPath.
func execMetaJSON(agentPath string) json.RawMessage {
	return json.RawMessage(`[{"cascadeConfig":{"plannerConfig":{"customizationConfig":{"agentPath":"` + agentPath + `"}}}}]`)
}

func TestParentCascadeIDFromExecutorMetadata(t *testing.T) {
	const parent = "152700d3-fe94-40d3-b6f2-a3365a9037cc"
	traj := &Trajectory{
		ExecutorMetadatas: execMetaJSON(
			"file:///home/u/.gemini/antigravity-cli/brain/" + parent + "/.agents/agents/child_agent"),
	}
	if got := traj.ParentCascadeID(); got != parent {
		t.Errorf("ParentCascadeID() = %q, want %q", got, parent)
	}
}

func TestParentCascadeIDFromGeneratorMetadata(t *testing.T) {
	const parent = "05b227b2-e660-49f3-8d17-ea1f3059441a"
	traj := &Trajectory{
		GeneratorMetadata: json.RawMessage(
			`[{"plannerConfig":{"customizationConfig":{"agentPath":"file:///x/brain/` + parent + `/.agents/agents/grandchild"}}}]`),
	}
	if got := traj.ParentCascadeID(); got != parent {
		t.Errorf("ParentCascadeID() = %q, want %q", got, parent)
	}
}

func TestParentCascadeIDRootHasNone(t *testing.T) {
	// No executor/generator metadata at all -> root.
	if got := (&Trajectory{}).ParentCascadeID(); got != "" {
		t.Errorf("root ParentCascadeID() = %q, want empty", got)
	}
	// Metadata present but no agentPath -> still a root.
	traj := &Trajectory{ExecutorMetadatas: json.RawMessage(`[{"cascadeConfig":{}}]`)}
	if got := traj.ParentCascadeID(); got != "" {
		t.Errorf("no-agentPath ParentCascadeID() = %q, want empty", got)
	}
}

func TestParentCascadeIDNonUUIDBrainSegment(t *testing.T) {
	// A built-in agent path whose brain/ segment isn't a UUID must not crash
	// and must be treated as a root (unlinkable).
	traj := &Trajectory{
		ExecutorMetadatas: execMetaJSON("file:///opt/agy/brain/builtin/.agents/agents/reviewer"),
	}
	if got := traj.ParentCascadeID(); got != "" {
		t.Errorf("non-UUID brain ParentCascadeID() = %q, want empty", got)
	}
}

func TestParentCascadeIDMalformedMetadataNoCrash(t *testing.T) {
	traj := &Trajectory{
		ExecutorMetadatas: json.RawMessage(`{"not":"an array"}`),
		GeneratorMetadata: json.RawMessage(`garbage`),
	}
	if got := traj.ParentCascadeID(); got != "" {
		t.Errorf("malformed metadata ParentCascadeID() = %q, want empty", got)
	}
}

func TestFirstStepTime(t *testing.T) {
	traj := &Trajectory{Steps: []Step{
		{Metadata: StepMetadata{CreatedAt: "2026-01-01T00:00:00.000000000Z"}},
	}}
	if got := traj.FirstStepTime(); got != "2026-01-01T00:00:00.000000000Z" {
		t.Errorf("FirstStepTime() = %q", got)
	}
	if got := (&Trajectory{}).FirstStepTime(); got != "" {
		t.Errorf("empty FirstStepTime() = %q, want empty", got)
	}
}
