package daemon

import (
	"encoding/json"
	"testing"
)

func TestWorkflowSourceRequiresMatchingInvocation(t *testing.T) {
	const parentID = "11111111-1111-4111-8111-111111111111"
	const childID = "22222222-2222-4222-8222-222222222222"
	call := ToolCall{Name: "run_workflow", ID: "call-workflow", ArgumentsJSON: `{"Script":"print('ok')"}`}
	for _, tc := range []struct {
		name   string
		change func(map[string]any, *Trajectory)
	}{
		{"valid", nil},
		{"missing index", func(o map[string]any, _ *Trajectory) { delete(o, "stepIndex") }},
		{"negative index", func(o map[string]any, _ *Trajectory) { o["stepIndex"] = -1 }},
		{"out of bounds", func(o map[string]any, _ *Trajectory) { o["stepIndex"] = 1 }},
		{"wrong parent", func(o map[string]any, _ *Trajectory) { o["conversationId"] = "33333333-3333-4333-8333-333333333333" }},
		{"self pointer", func(o map[string]any, _ *Trajectory) { o["conversationId"] = childID }},
		{"wrong call", func(o map[string]any, _ *Trajectory) { c := call; c.ID = "other"; o["toolCall"] = c }},
		{"wrong tool", func(o map[string]any, _ *Trajectory) { c := call; c.Name = "invoke_subagent"; o["toolCall"] = c }},
		{"wrong arguments", func(o map[string]any, _ *Trajectory) { c := call; c.ArgumentsJSON = `{}`; o["toolCall"] = c }},
		{"malformed arguments", func(o map[string]any, _ *Trajectory) { c := call; c.ArgumentsJSON = `broken`; o["toolCall"] = c }},
		{"unfinished parent", func(_ map[string]any, p *Trajectory) { p.Steps[0].Status = "CORTEX_STEP_STATUS_RUNNING" }},
		{"wrong step type", func(_ map[string]any, p *Trajectory) { p.Steps[0].Type = "CORTEX_STEP_TYPE_RUN_COMMAND" }},
		{"missing call", func(_ map[string]any, p *Trajectory) { p.Steps[0].Metadata.ToolCall = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := &Trajectory{CascadeID: parentID, Steps: []Step{{Type: "CORTEX_STEP_TYPE_GENERIC", Status: "CORTEX_STEP_STATUS_DONE", Metadata: StepMetadata{ToolCall: &call}}}}
			origin := map[string]any{"conversationId": parentID, "stepIndex": 0, "toolCall": call, "title": "Parallel worker"}
			if tc.change != nil {
				tc.change(origin, parent)
			}
			raw, err := json.Marshal(map[string]any{"tool": origin, "hideFromConversationList": true})
			if err != nil {
				t.Fatal(err)
			}
			child := &Trajectory{CascadeID: childID, Metadata: TrajectoryMetadata{SourceMetadata: raw}}
			index, ok := child.WorkflowStep(parent)
			if ok != (tc.change == nil) || (ok && index != 0) {
				t.Fatalf("WorkflowStep = %d, %v", index, ok)
			}
			if tc.change == nil && child.WorkflowParentCascadeID() != parentID {
				t.Fatal("missing explicit parent")
			}
			if _, ok := child.WorkflowStep(nil); ok {
				t.Fatal("nil parent matched")
			}
			child.CascadeID = ""
			if _, ok := child.WorkflowStep(parent); ok {
				t.Fatal("unidentified child matched")
			}
		})
	}
	var child *Trajectory
	if child.WorkflowParentCascadeID() != "" {
		t.Fatal("nil child has a parent")
	}
}
