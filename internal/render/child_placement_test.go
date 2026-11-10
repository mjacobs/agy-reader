package render_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/render"
)

func swarmAdd155(id string) daemon.Step {
	result := fmt.Sprintf(`{"status":"added","name":"worker","conversation_id":%q}`, id)
	b, _ := json.Marshal(map[string]any{"args": map[string]string{"action": "add", "name": "worker"}, "result": map[string]string{"result": "Created At: 2026-11-10T10:00:01-08:00\nCompleted At: 2026-11-10T10:00:01-08:00\n" + result}})
	return daemon.Step{Type: "CORTEX_STEP_TYPE_GENERIC", Status: "CORTEX_STEP_STATUS_DONE", Metadata: daemon.StepMetadata{ToolCall: &daemon.ToolCall{Name: "swarm"}}, Generic: b}
}

func invokeIDs155(ids ...string) daemon.Step {
	results := []map[string]string{}
	for _, id := range ids {
		results = append(results, map[string]string{"conversationId": id})
	}
	b, _ := json.Marshal(map[string]any{"results": results})
	return daemon.Step{Type: "CORTEX_STEP_TYPE_INVOKE_SUBAGENT", Status: "CORTEX_STEP_STATUS_DONE", InvokeSubagent: b}
}

func tree155(t *testing.T, steps []daemon.Step, ids ...string) string {
	t.Helper()
	children := []*daemon.Trajectory{}
	for _, id := range ids {
		children = append(children, &daemon.Trajectory{CascadeID: id, Steps: []daemon.Step{responseStep("", "child content "+id)}})
	}
	parent := &daemon.Trajectory{CascadeID: "parent", Steps: steps}
	var b bytes.Buffer
	if _, err := render.MarkdownTree(&b, parent, testNow, mapResolver{"parent": children}); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestSwarm155ChildrenFollowReturnedIDs(t *testing.T) {
	// Resolver order is deliberately unrelated to invocation order.
	text := tree155(t, []daemon.Step{swarmAdd155("worker"), invokeIDs155("share"), invokeIDs155("branch")}, "branch", "worker", "share")
	for _, c := range []struct {
		id   string
		step int
	}{{"worker", 1}, {"share", 2}, {"branch", 3}} {
		start := strings.Index(text, fmt.Sprintf("## Step %d:", c.step))
		end := len(text)
		if c.step < 3 {
			end = strings.Index(text, fmt.Sprintf("## Step %d: INVOKE_SUBAGENT", c.step+1))
		}
		at := strings.Index(text, "<code>"+c.id+"</code>")
		if at < start || at >= end {
			t.Errorf("%s misplaced", c.id)
		}
	}
	if strings.Contains(text, "## Subagent Transcripts") {
		t.Fatal("unexpected unmatched children")
	}
}

func TestMultipleAndRepeatedReturnedChildren155(t *testing.T) {
	text := tree155(t, []daemon.Step{invokeIDs155("second", "first"), invokeIDs155("first")}, "first", "second", "extra")
	end := strings.Index(text, "## Step 2: INVOKE_SUBAGENT")
	for _, id := range []string{"first", "second"} {
		if strings.Count(text, "<code>"+id+"</code>") != 1 || strings.Index(text, "<code>"+id+"</code>") >= end {
			t.Fatal("child missing or repeated", id)
		}
	}
	if strings.Index(text, "<code>extra</code>") < strings.Index(text, "## Subagent Transcripts") {
		t.Fatal("extra child misplaced")
	}
}

func TestSwarm155UnmatchedChildrenStayVisible(t *testing.T) {
	for _, mutate := range []func(*daemon.Step){
		func(s *daemon.Step) { s.Status = "CORTEX_STEP_STATUS_ERROR" },
		func(s *daemon.Step) { s.Generic = bytes.Replace(s.Generic, []byte(`added`), []byte(`failed`), 1) },
		func(s *daemon.Step) {
			s.Generic = bytes.Replace(s.Generic, []byte(`Created At:`), []byte(`arbitrary prose`), 1)
		},
		func(s *daemon.Step) {
			s.Generic = bytes.Replace(s.Generic, []byte(`\"name\":\"worker\"`), []byte(`\"name\":\"other\"`), 1)
		},
		func(s *daemon.Step) { *s = swarmAdd155("missing") },
	} {
		step := swarmAdd155("worker")
		mutate(&step)
		// The unresolved swarm worker must never be taken by a legacy invocation.
		text := tree155(t, []daemon.Step{step, invokeStep("")}, "worker")
		if at := strings.Index(text, "## Subagent Transcripts"); at < 0 || strings.Index(text, "<code>worker</code>") < at {
			t.Fatal("unknown swarm result stole a child")
		}
	}
	text := tree155(t, []daemon.Step{invokeIDs155("absent")}, "other")
	if !strings.Contains(text, "## Subagent Transcripts") {
		t.Fatal("unmatched explicit ID fell back to another child")
	}
}

func TestLegacyInvocationDoesNotStealLaterExplicitChild155(t *testing.T) {
	text := tree155(t, []daemon.Step{invokeStep(""), invokeIDs155("explicit")}, "explicit", "legacy")
	end := strings.Index(text, "## Step 2: INVOKE_SUBAGENT")
	if strings.Index(text, "<code>legacy</code>") >= end || strings.Index(text, "<code>explicit</code>") < end {
		t.Fatal("legacy fallback stole explicit child")
	}
}
