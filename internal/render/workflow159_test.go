package render_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/render"
)

func TestWorkflow159PlacesChildrenAtExplicitCall(t *testing.T) {
	const parentID = "11111111-1111-4111-8111-111111111111"
	const childID = "22222222-2222-4222-8222-222222222222"
	call := daemon.ToolCall{Name: "run_workflow", ID: "call-workflow", ArgumentsJSON: `{"ScriptPath":"/fixture/work/workflow.py"}`}
	parent := &daemon.Trajectory{CascadeID: parentID, Steps: []daemon.Step{
		{Type: "CORTEX_STEP_TYPE_GENERIC", Status: "CORTEX_STEP_STATUS_DONE", Metadata: daemon.StepMetadata{ToolCall: &call}, Generic: json.RawMessage(`{"result":{"result":"WORKFLOW_OUTPUT"}}`)},
		{Type: "CORTEX_STEP_TYPE_INVOKE_SUBAGENT", Status: "CORTEX_STEP_STATUS_DONE"},
	}}
	source, _ := json.Marshal(map[string]any{"tool": map[string]any{"conversationId": parentID, "stepIndex": 0, "toolCall": call, "title": "Workflow worker"}})
	child := &daemon.Trajectory{CascadeID: childID, Metadata: daemon.TrajectoryMetadata{SourceMetadata: source}, Steps: []daemon.Step{responseStep("", "WORKFLOW_CHILD_OUTPUT")}}
	extra := &daemon.Trajectory{CascadeID: "33333333-3333-4333-8333-333333333333", Steps: []daemon.Step{responseStep("", "UNMATCHED_CHILD")}}
	for _, mismatch := range []bool{false, true} {
		if mismatch {
			parent.Steps[0].Metadata.ToolCall = &daemon.ToolCall{Name: "run_workflow", ID: "wrong-call", ArgumentsJSON: call.ArgumentsJSON}
		}
		var b bytes.Buffer
		if _, err := render.MarkdownTree(&b, parent, testNow, mapResolver{parentID: {nil, child, extra, child}}); err != nil {
			t.Fatal(err)
		}
		text := b.String()
		childAt, next := strings.Index(text, "<code>"+childID+"</code>"), strings.Index(text, "## Step 2: INVOKE_SUBAGENT")
		if childAt < 0 || next < 0 || (childAt < next) == mismatch || strings.Count(text, "WORKFLOW_CHILD_OUTPUT") != 1 {
			t.Fatalf("workflow child misplaced or repeated (mismatch=%v)", mismatch)
		}
		if strings.Index(text, "UNMATCHED_CHILD") < strings.Index(text, "## Subagent Transcripts") {
			t.Fatal("legacy invocation stole an unmatched workflow child")
		}
	}
}
