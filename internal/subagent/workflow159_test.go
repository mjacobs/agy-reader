package subagent_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/subagent"
)

func TestWorkflow159LinksExplicitChildrenAndRejectsConflicts(t *testing.T) {
	call := map[string]any{"name": "run_workflow", "id": "call-workflow", "argumentsJson": `{"ScriptPath":"/fixture/work/workflow.py"}`}
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "four workflow children", true: "conflicting native parent"}[conflict], func(t *testing.T) {
			dir := t.TempDir()
			writeJSON(t, filepath.Join(dir, rootID+".trajectory.json"), map[string]any{"cascadeId": rootID, "steps": []any{
				map[string]any{"type": "CORTEX_STEP_TYPE_GENERIC", "status": "CORTEX_STEP_STATUS_DONE", "metadata": map[string]any{"toolCall": call}},
			}})
			children := []string{childID, grandchildID, unrelatedID, "55555555-5555-4555-8555-555555555555"}
			for _, id := range children {
				metadata := map[string]any{"sourceMetadata": map[string]any{"tool": map[string]any{"conversationId": rootID, "stepIndex": 0, "toolCall": call, "title": "Workflow worker"}, "hideFromConversationList": true}}
				if conflict {
					metadata["parentConversationId"] = "66666666-6666-4666-8666-666666666666"
				}
				writeJSON(t, filepath.Join(dir, id+".trajectory.json"), map[string]any{"cascadeId": id, "metadata": metadata, "steps": []any{}})
			}
			report, err := subagent.Backfill(dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := 4
			if conflict {
				want = 0
				if len(report.Diagnostics) != 4 {
					t.Fatalf("conflicts: %+v", report.Diagnostics)
				}
			}
			if report.Stamped != want {
				t.Fatalf("stamped %d, want %d", report.Stamped, want)
			}
			r, err := subagent.Build(dir, nil)
			if err != nil || len(r.Children(rootID)) != want {
				t.Fatalf("children: %v, %v", r.Children(rootID), err)
			}
			for _, id := range children {
				traj, err := cache.Read(filepath.Join(dir, id+".trajectory.json"))
				if err != nil || !json.Valid(traj.Metadata.SourceMetadata) {
					t.Fatalf("source metadata lost: %v", err)
				}
				if conflict && traj.StampedParentCascadeID() != "" {
					t.Fatal("conflicting source gained a parent")
				}
			}
		})
	}
}
