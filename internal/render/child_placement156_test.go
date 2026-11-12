package render_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

func swarmMulti156(tasks bool) daemon.Step {
	args := map[string]string{"action": "add", "name": "worker", "n": "2"}
	if tasks {
		delete(args, "n")
		args["tasks"] = `["first task","second task"]`
	}
	result := `{"status":"added","names":["worker-1","worker-2"],"conversation_ids":["first","second"]}`
	b, _ := json.Marshal(map[string]any{"args": args, "result": map[string]string{"result": "Created At: 2026-11-11T21:00:32-08:00\nCompleted At: 2026-11-11T21:00:34-08:00\n" + result}})
	return daemon.Step{Type: "CORTEX_STEP_TYPE_GENERIC", Status: "CORTEX_STEP_STATUS_DONE", Metadata: daemon.StepMetadata{ToolCall: &daemon.ToolCall{Name: "swarm"}}, Generic: b}
}

func TestMultiSwarm156Placement(t *testing.T) {
	for _, tasks := range []bool{false, true} {
		text := tree155(t, []daemon.Step{swarmMulti156(tasks), invokeIDs155("last")}, "last", "second", "first", "extra")
		end := strings.Index(text, "## Step 2: INVOKE_SUBAGENT")
		for _, id := range []string{"first", "second"} {
			if at := strings.Index(text, "<code>"+id+"</code>"); at < 0 || at >= end || strings.Count(text, "<code>"+id+"</code>") != 1 {
				t.Fatal("multi worker missing or misplaced", id)
			}
		}
		if strings.Index(text, "<code>first</code>") > strings.Index(text, "<code>second</code>") {
			t.Fatal("result order not retained")
		}
		if strings.Index(text, "<code>extra</code>") < strings.Index(text, "## Subagent Transcripts") {
			t.Fatal("unmatched child missing")
		}
		// Missing resolved children and repeated add results neither steal nor repeat.
		text = tree155(t, []daemon.Step{swarmMulti156(tasks), swarmMulti156(tasks)}, "second", "extra")
		if strings.Count(text, "<code>second</code>") != 1 || !strings.Contains(text, "<code>extra</code>") {
			t.Fatal("missing/repeated multi child mishandled")
		}
	}
}

func TestMultiSwarm156Refusals(t *testing.T) {
	for _, change := range []func(map[string]any){
		func(g map[string]any) { g["args"].(map[string]any)["n"] = "3" },
		func(g map[string]any) { g["args"].(map[string]any)["n"] = "02" },
		func(g map[string]any) { g["args"].(map[string]any)["tasks"] = `["one","two"]` },
		func(g map[string]any) { delete(g["args"].(map[string]any), "n") },
		func(g map[string]any) {
			g["result"].(map[string]any)["result"] = `{"status":"added","name":"worker","conversation_id":"first"}`
		},
		func(g map[string]any) {
			g["result"].(map[string]any)["result"] = `{"status":"added","names":["worker-1","worker-2"],"conversation_ids":["first","first"]}`
		},
		func(g map[string]any) {
			g["result"].(map[string]any)["result"] = `{"status":"added","names":["worker-2","worker-1"],"conversation_ids":["first","second"]}`
		},
		func(g map[string]any) {
			g["result"].(map[string]any)["result"] = `{"status":"added","names":["worker-1"],"conversation_ids":["first","second"]}`
		},
		func(g map[string]any) {
			g["result"].(map[string]any)["result"] = `{"status":"failed","names":["worker-1","worker-2"],"conversation_ids":["first","second"]}`
		},
		func(g map[string]any) {
			g["result"].(map[string]any)["result"] = `{"status":"added","name":"worker","conversation_id":"first","names":["worker-1","worker-2"],"conversation_ids":["first","second"]}`
		},
		func(g map[string]any) {
			g["result"].(map[string]any)["result"] = "Created At: invalid\nCompleted At: 2026-11-11T21:00:34-08:00\n{}"
		},
	} {
		s := swarmMulti156(false)
		var g map[string]any
		if err := json.Unmarshal(s.Generic, &g); err != nil {
			t.Fatal(err)
		}
		change(g)
		s.Generic, _ = json.Marshal(g)
		text := tree155(t, []daemon.Step{s, invokeStep("")}, "first", "second")
		start := strings.Index(text, "## Subagent Transcripts")
		if start < 0 || strings.Index(text, "<code>first</code>") < start || strings.Index(text, "<code>second</code>") < start {
			t.Fatal("malformed result stole a child")
		}
	}
	s := swarmMulti156(false)
	s.Status = "CORTEX_STEP_STATUS_ERROR"
	if text := tree155(t, []daemon.Step{s}, "first", "second"); !strings.Contains(text, "## Subagent Transcripts") {
		t.Fatal("failed add placed workers")
	}
}

func TestOneWorkerCount156(t *testing.T) {
	for _, tasks := range []bool{false, true} {
		for _, array := range []bool{false, true} {
			step := swarmMulti156(tasks)
			var g map[string]any
			if err := json.Unmarshal(step.Generic, &g); err != nil {
				t.Fatal(err)
			}
			args := g["args"].(map[string]any)
			if tasks {
				args["tasks"] = `["one task"]`
			} else {
				args["n"] = "1"
			}
			result := `{"status":"added","name":"worker","conversation_id":"first"}`
			if array {
				result = `{"status":"added","names":["worker-1"],"conversation_ids":["first"]}`
			}
			g["result"].(map[string]any)["result"] = result
			step.Generic, _ = json.Marshal(g)
			if text := tree155(t, []daemon.Step{step}, "first"); strings.Contains(text, "## Subagent Transcripts") || !strings.Contains(text, "<code>first</code>") {
				t.Fatal("single worker count misplaced")
			}
		}
	}
}
