package render_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/render"
)

// mapResolver is an in-code SubagentResolver: a parent-cascade-id -> children
// map, so tree tests build a delegation tree without touching the filesystem.
type mapResolver map[string][]*daemon.Trajectory

func (m mapResolver) Children(cascadeID string) []*daemon.Trajectory {
	return m[cascadeID]
}

func invokeStep(ts string) daemon.Step {
	return daemon.Step{
		Type:     "CORTEX_STEP_TYPE_INVOKE_SUBAGENT",
		Status:   "CORTEX_STEP_STATUS_DONE",
		Metadata: daemon.StepMetadata{CreatedAt: ts},
	}
}

func responseStep(ts, text string) daemon.Step {
	return daemon.Step{
		Type:            "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
		Status:          "CORTEX_STEP_STATUS_DONE",
		Metadata:        daemon.StepMetadata{CreatedAt: ts},
		PlannerResponse: &daemon.PlannerResponse{Response: text},
	}
}

var testNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// TestTreeDepth2NestingUnderInvokeStep: a child nests under the parent's
// INVOKE_SUBAGENT step, and a grandchild nests under the child's.
func TestTreeDepth2NestingUnderInvokeStep(t *testing.T) {
	grandchild := &daemon.Trajectory{
		CascadeID: "gc-3333",
		Steps:     []daemon.Step{responseStep("2026-01-01T00:00:03Z", "grandchild worked")},
	}
	child := &daemon.Trajectory{
		CascadeID: "ch-2222",
		Steps: []daemon.Step{
			responseStep("2026-01-01T00:00:01Z", "child starting"),
			invokeStep("2026-01-01T00:00:02Z"),
			responseStep("2026-01-01T00:00:04Z", "child done"),
		},
	}
	parent := &daemon.Trajectory{
		CascadeID: "pa-1111",
		Steps: []daemon.Step{
			responseStep("2026-01-01T00:00:00Z", "parent starting"),
			invokeStep("2026-01-01T00:00:01Z"),
		},
	}
	r := mapResolver{"pa-1111": {child}, "ch-2222": {grandchild}}

	var buf bytes.Buffer
	if _, err := render.MarkdownTree(&buf, parent, testNow, r); err != nil {
		t.Fatalf("MarkdownTree: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"### Subagent Invoked",
		"The subagent's transcript is nested below.",
		"<details><summary>Subagent transcript: <code>ch-2222</code> (depth 1, 3 steps)</summary>",
		"child starting",
		"<details><summary>Subagent transcript: <code>gc-3333</code> (depth 2, 1 steps)</summary>",
		"grandchild worked",
		"child done",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
	// Two nested subagents -> two closing </details>.
	if n := strings.Count(out, "</details>"); n != 2 {
		t.Errorf("got %d </details>, want 2\n%s", n, out)
	}
	// Grandchild's details must open before the child's closes (nesting order).
	gcOpen := strings.Index(out, "gc-3333")
	firstClose := strings.Index(out, "</details>")
	if gcOpen > firstClose {
		t.Errorf("grandchild not nested inside child (open at %d, first close at %d)", gcOpen, firstClose)
	}
	// No trailing section when every child matched an invoke step.
	if strings.Contains(out, "## Subagent Transcripts") {
		t.Errorf("unexpected trailing section:\n%s", out)
	}
}

// TestTreeNoInvokeStepTrailingFallback: a parent with linked children but zero
// INVOKE_SUBAGENT steps (stale/truncated sidecar) renders them in the trailing
// "Subagent Transcripts" section.
func TestTreeNoInvokeStepTrailingFallback(t *testing.T) {
	child := &daemon.Trajectory{
		CascadeID: "ch-2222",
		Steps:     []daemon.Step{responseStep("2026-01-01T00:00:01Z", "child ran anyway")},
	}
	parent := &daemon.Trajectory{
		CascadeID: "pa-1111",
		Steps:     []daemon.Step{responseStep("2026-01-01T00:00:00Z", "parent, no invoke step")},
	}
	r := mapResolver{"pa-1111": {child}}

	var buf bytes.Buffer
	if _, err := render.MarkdownTree(&buf, parent, testNow, r); err != nil {
		t.Fatalf("MarkdownTree: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "## Subagent Transcripts") {
		t.Errorf("expected trailing section, got:\n%s", out)
	}
	if !strings.Contains(out, "<details><summary>Subagent transcript: <code>ch-2222</code>") {
		t.Errorf("expected child in trailing section, got:\n%s", out)
	}
	if !strings.Contains(out, "child ran anyway") {
		t.Errorf("expected child content, got:\n%s", out)
	}
	// Trailing section appears after the parent's own step.
	if strings.Index(out, "parent, no invoke step") > strings.Index(out, "## Subagent Transcripts") {
		t.Errorf("trailing section should follow the parent steps:\n%s", out)
	}
}

// TestTreeExtraChildrenSpillToTrailing: more children than invoke steps — the
// surplus lands in the trailing section.
func TestTreeExtraChildrenSpillToTrailing(t *testing.T) {
	c1 := &daemon.Trajectory{CascadeID: "c1", Steps: []daemon.Step{responseStep("2026-01-01T00:00:01Z", "one")}}
	c2 := &daemon.Trajectory{CascadeID: "c2", Steps: []daemon.Step{responseStep("2026-01-01T00:00:02Z", "two")}}
	parent := &daemon.Trajectory{
		CascadeID: "pa",
		Steps: []daemon.Step{
			invokeStep("2026-01-01T00:00:00Z"),
		},
	}
	r := mapResolver{"pa": {c1, c2}}

	var buf bytes.Buffer
	if _, err := render.MarkdownTree(&buf, parent, testNow, r); err != nil {
		t.Fatalf("MarkdownTree: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "## Subagent Transcripts") {
		t.Errorf("expected trailing section for surplus child:\n%s", out)
	}
	// c1 nests under the invoke step (before the trailing header); c2 spills after.
	if strings.Index(out, "<code>c1</code>") > strings.Index(out, "## Subagent Transcripts") {
		t.Errorf("c1 should nest under invoke step, not trailing:\n%s", out)
	}
	if strings.Index(out, "<code>c2</code>") < strings.Index(out, "## Subagent Transcripts") {
		t.Errorf("c2 should spill to trailing section:\n%s", out)
	}
}

// TestTreeNoChildrenByteIdenticalToFlat: a resolver reporting no children must
// produce exactly the plain Markdown output (additive-only inlining).
func TestTreeNoChildrenByteIdenticalToFlat(t *testing.T) {
	parent := &daemon.Trajectory{
		CascadeID: "solo",
		Steps: []daemon.Step{
			responseStep("2026-01-01T00:00:00Z", "hello"),
			invokeStep("2026-01-01T00:00:01Z"),
		},
	}
	var flat, tree bytes.Buffer
	if _, err := render.Markdown(&flat, parent, testNow); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	if _, err := render.MarkdownTree(&tree, parent, testNow, mapResolver{}); err != nil {
		t.Fatalf("MarkdownTree: %v", err)
	}
	if !bytes.Equal(flat.Bytes(), tree.Bytes()) {
		t.Errorf("resolver with no children changed output\n--- flat ---\n%s\n--- tree ---\n%s", flat.String(), tree.String())
	}
	// The lone-invoke wording (no nested transcript) is preserved.
	if !strings.Contains(flat.String(), "recorded in its own separate trajectory") {
		t.Errorf("expected the un-nested invoke marker:\n%s", flat.String())
	}
}

// TestTreeCycleGuard: a child that links back to an ancestor is shown once and
// then guarded, not expanded infinitely.
func TestTreeCycleGuard(t *testing.T) {
	parent := &daemon.Trajectory{
		CascadeID: "A",
		Steps:     []daemon.Step{invokeStep("2026-01-01T00:00:00Z")},
	}
	child := &daemon.Trajectory{
		CascadeID: "B",
		Steps:     []daemon.Step{invokeStep("2026-01-01T00:00:01Z")},
	}
	// B's child links back to A -> cycle A -> B -> A.
	r := mapResolver{"A": {child}, "B": {parent}}

	var buf bytes.Buffer
	if _, err := render.MarkdownTree(&buf, parent, testNow, r); err != nil {
		t.Fatalf("MarkdownTree: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "already shown above (cycle guard)") {
		t.Errorf("expected cycle-guard note, got:\n%s", out)
	}
	// B is expanded once (one <details>); the back-link to A hits the guard and
	// emits a note instead of re-expanding, so there is no second details block.
	if n := strings.Count(out, "<details>"); n != 1 {
		t.Errorf("expected exactly 1 <details> (B; guarded A is a note), got %d:\n%s", n, out)
	}
}
