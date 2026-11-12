package completeness

import (
	"encoding/json"
	"strings"
	"testing"
)

// Synthetic pairs model the two hook-free cancellations captured with agy 1.5.6.
// Expected native output is independent of the projection under test.
func background156Pair() (string, string) {
	r := strings.NewReplacer(`"preToolHookResults":[{"decision":"allow"}],`, ``, `"metadata":{`, `"metadata":{"completedAt":"2026-01-01T00:00:02Z",`)
	return r.Replace(background144Raw), r.Replace(background144Shown)
}

func TestBackground156(t *testing.T) {
	raw, shown := background156Pair()
	s := snapshot1216(t, raw)
	require1216(t, s, raw, shown, Verified)
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, raw, string(offline), Verified)
	for _, c := range []struct{ name, before, after string }{
		{"hook denial", `"metadata":{`, `"metadata":{"preToolHookResults":[{"decision":"deny"}],`},
		{"hook reason", `"metadata":{`, `"metadata":{"preToolHookResults":[{"decision":"allow","reason":"fixture"}],`},
		{"two hooks", `"metadata":{`, `"metadata":{"preToolHookResults":[{"decision":"allow"},{"decision":"allow"}],`},
		{"not completed", `"completedAt":"2026-01-01T00:00:02Z",`, ``},
		{"no task", `,` + background144Task, ``},
		{"missing background time", `,"movedToBackgroundAt":"2026-01-01T00:00:01Z"`, ``},
		{"different wait", `5000`, `10000`},
		{"bypass sandbox", `"args":{`, `"args":{"BypassSandbox":"true",`},
		{"short history", `,{"updatedStatus":"CORTEX_STEP_STATUS_CANCELED"}`, ``},
		{"extra result", `"result":{"stepRenderInfo"`, `"result":{"result":"new output","stepRenderInfo"`},
		{"unknown argument", `"args":{`, `"args":{"Future":"value",`},
	} {
		t.Run(c.name, func(t *testing.T) {
			bad := replace1216(t, raw, c.before, c.after)
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
	for _, c := range []struct{ name, before, after string }{
		{"command", `"commandLine":"sleep 30"`, `"commandLine":"sleep 60"`},
		{"invented exit", `"combinedOutput":{}`, `"combinedOutput":{},"exitCode":0`},
		{"dropped task", `,` + background144Task, ``},
		{"invented hook", `"metadata":{`, `"metadata":{"preToolHookResults":[{"decision":"allow"}],`},
	} {
		t.Run(c.name, func(t *testing.T) { require1216(t, s, raw, replace1216(t, shown, c.before, c.after), Incomplete) })
	}
	// The render-only result is dropped only after full-source equality.
	changed := replace1216(t, raw, `"titlePrefix":"Running"`, `"titlePrefix":"Changed"`)
	require1216(t, s, changed, shown, Incomplete)
}

func TestSwarm156Defaults(t *testing.T) {
	for _, args := range []map[string]any{
		{"action": "add", "name": "worker", "brief": "fixture"},
		{"action": "add", "name": "worker", "brief": "fixture", "model": "fixture-model"},
	} {
		raw := swarm155Pair("swarm", args)
		s := snapshot1216(t, raw)
		require1216(t, s, raw, raw, Verified)
		offline, err := s.OfflineJSON()
		if err != nil {
			t.Fatal(err)
		}
		require1216(t, s, raw, string(offline), Verified)
	}
	for _, args := range []map[string]any{
		{"action": "add", "name": "worker", "brief": "fixture", "model": ""},
		{"action": "add", "name": "worker", "brief": "fixture", "workspace": ""},
		{"action": "add", "name": "worker", "brief": "fixture", "unexpected": "value"},
		{"action": "add", "brief": "fixture"},
		{"action": "add", "name": "worker"},
	} {
		raw := swarm155Pair("swarm", args)
		require1216(t, snapshot1216(t, raw), raw, raw, Unknown)
	}
}

func TestTools156InvalidatesPreviousProfile(t *testing.T) {
	raw, shown := background156Pair()
	s := snapshot1216(t, raw)
	a, r := s.VerifyResponses([]byte(raw), []byte(shown))
	if r.State != Verified {
		t.Fatal(r)
	}
	a.Profile = strings.Split(Profile, "-tools156")[0]
	if a.Profile == Profile {
		t.Fatal("missing tools156 profile suffix")
	}
	if r = s.CheckAttestation([]byte(shown), a); r.State != Unknown {
		t.Fatalf("old profile accepted: %+v", r)
	}
}

func TestTimestampedEmptyRead156(t *testing.T) {
	raw, shown := emptyRead154Pair()
	prefix := "Created At: 2026-11-11T18:35:01-08:00\nCompleted At: 2026-11-11T18:35:01-08:00\n"
	escaped := strings.ReplaceAll(prefix, "\n", `\n`)
	raw = strings.ReplaceAll(raw, "File is empty", escaped+"File is empty")
	shown = strings.ReplaceAll(shown, "File is empty", escaped+"File is empty")
	s := snapshot1216(t, raw)
	require1216(t, s, raw, shown, Verified)
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, raw, string(offline), Verified)
	for _, content := range []string{
		"Created At: invalid\nCompleted At: 2026-11-11T18:35:01-08:00\nFile is empty",
		prefix + "File is empty\n", prefix + "new content", prefix + "File is empty\nextra",
		strings.Replace(prefix, "Completed At:", "Other At:", 1) + "File is empty",
	} {
		if emptyRead156(content) {
			t.Fatalf("accepted %q", content)
		}
	}
	require1216(t, s, raw, replace1216(t, shown, escaped, ""), Incomplete)
}

func TestTimestampedFoldedSkill156(t *testing.T) {
	prefix := "Created At: 2026-11-11T18:40:50-08:00\nCompleted At: 2026-11-11T18:40:54-08:00\n"
	content := "Showing lines 1-7 of 10\n1: ---\n2: name: sample\n3: description: >-\n4:   A sample\n5:   folded description.\n6: ---\n7: body"
	raw, old := skill150Pair(prefix + content)
	shown := strings.Replace(old, `"isSkillFile":true,`, `"isSkillFile":true,"skillMetadata":{"name":"sample","description":"A sample folded description."},`, 1)
	s := snapshot1216(t, raw)
	require1216(t, s, raw, shown, Verified)
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, raw, string(offline), Verified)
	for _, bad := range []string{
		strings.Replace(content, "description: >-", "description: |", 1),
		strings.Replace(content, "description: >-", "description: >", 1),
		strings.Replace(content, "Showing lines 1-", "Partial read (lines 1-", 1),
		strings.Replace(content, "4:   A sample", "4:    A sample", 1),
	} {
		if _, ok := skillFrontmatter150(prefix + bad); ok {
			t.Fatal("accepted unobserved timestamped form")
		}
	}
	require1216(t, s, raw, replace1216(t, shown, `"description":"A sample folded description."`, `"description":"changed"`), Incomplete)
}

func TestSwarmBoard156(t *testing.T) {
	for _, c := range []struct {
		tool string
		args map[string]any
	}{
		{"read_board", map[string]any{}},
		{"read_board", map[string]any{"since": 1, "limit": 10}},
		{"read_board", map[string]any{"limit": 1, "status": true}},
		{"read_board", map[string]any{"seqs": []int{5}}},
		{"swarm_wait", map[string]any{"since": 1, "mentions_only": true}},
		{"swarm_wait", map[string]any{"since": 1, "mentions_only": false}},
	} {
		raw := swarm155Pair(c.tool, c.args)
		s := snapshot1216(t, raw)
		require1216(t, s, raw, raw, Verified)
		offline, err := s.OfflineJSON()
		if err != nil {
			t.Fatal(err)
		}
		require1216(t, s, raw, string(offline), Verified)
		require1216(t, s, raw, strings.Replace(raw, "fixture result", "changed result", 1), Incomplete)
	}
	for _, c := range []struct {
		tool string
		args map[string]any
	}{
		{"read_board", map[string]any{"seqs": []int{}}},
		{"read_board", map[string]any{"seqs": []int{1, 1}}},
		{"read_board", map[string]any{"seqs": []int{0}}},
		{"read_board", map[string]any{"seqs": []any{nil}}},
		{"read_board", map[string]any{"seqs": []string{"5"}}},
		{"read_board", map[string]any{"seqs": []int{5}, "since": 1}},
		{"read_board", map[string]any{"limit": 1, "status": false}},
		{"read_board", map[string]any{"limit": 0, "since": 1}},
		{"read_board", map[string]any{"status": false}},
		{"swarm_wait", map[string]any{"mentions_only": true}},
		{"swarm_wait", map[string]any{"since": 1, "mentions_only": "yes"}},
	} {
		raw := swarm155Pair(c.tool, c.args)
		require1216(t, snapshot1216(t, raw), raw, raw, Unknown)
	}
}

func TestExtendedSwarm156(t *testing.T) {
	for _, args := range []map[string]any{
		{"action": "model", "name": "worker-1", "model": "fixture-model"},
		{"action": "model", "name": "coordinator", "model": "fixture-model"},
		{"action": "add", "name": "worker", "brief": "fixture", "workspace": "share", "n": 2},
		{"action": "add", "name": "worker", "brief": "fixture", "workspace": "branch", "fork": "worker-2", "model": "fixture-model"},
		{"action": "add", "name": "worker", "brief": "fixture", "workspace": "inherit", "model": "fixture-model", "tasks": []string{"first", "second"}},
	} {
		raw := swarm155Pair("swarm", args)
		s := snapshot1216(t, raw)
		require1216(t, s, raw, raw, Verified)
		offline, err := s.OfflineJSON()
		if err != nil {
			t.Fatal(err)
		}
		require1216(t, s, raw, string(offline), Verified)
		require1216(t, s, raw, strings.Replace(raw, "fixture result", "changed", 1), Incomplete)
		for _, change := range [][2]string{
			{`"args":{`, `"args":{"unknown":"value",`},
			{`"CORTEX_STEP_STATUS_DONE"`, `"CORTEX_STEP_STATUS_CANCELED"`},
		} {
			bad := replace1216(t, raw, change[0], change[1])
			require1216(t, snapshot1216(t, bad), bad, raw, Unknown)
		}
	}
	for _, args := range []map[string]any{
		{"action": "model", "name": "worker"},
		{"action": "model", "name": "worker", "model": ""},
		{"action": "add", "name": "worker", "brief": "fixture", "n": 0},
		{"action": "add", "name": "worker", "brief": "fixture", "n": -1},
		{"action": "add", "name": "worker", "brief": "fixture", "n": "02"},
		{"action": "add", "name": "worker", "brief": "fixture", "n": 2, "tasks": []string{"first", "second"}},
		{"action": "add", "name": "worker", "brief": "fixture", "tasks": []string{}},
		{"action": "add", "name": "worker", "brief": "fixture", "tasks": []any{nil, "second"}},
		{"action": "add", "name": "worker", "brief": "fixture", "tasks": []int{1, 2}},
		{"action": "add", "name": "worker", "brief": "fixture", "workspace": "unknown"},
		{"action": "add", "name": "worker", "brief": "fixture", "fork": ""},
	} {
		raw := swarm155Pair("swarm", args)
		require1216(t, snapshot1216(t, raw), raw, raw, Unknown)
	}
}

func TestFailedSwarm156(t *testing.T) {
	for _, c := range []struct {
		args    map[string]any
		message string
	}{
		{map[string]any{"action": "model", "name": "worker", "model": "fixture-model"}, `worker "worker" is mid-turn; switch its model once status shows it waiting`},
		{map[string]any{"action": "add", "name": "worker", "brief": "fixture", "model": "fixture-model", "workspace": "branch", "fork": "coordinator"}, `failed to start worker "worker": executor has not processed the previous input yet`},
		{map[string]any{"action": "add", "name": "worker", "brief": "fixture", "model": "fixture-model", "workspace": "branch", "fork": "coordinator"}, `the swarm already has a member named "worker"`},
	} {
		var doc map[string]any
		if err := json.Unmarshal([]byte(swarm155Pair("swarm", c.args)), &doc); err != nil {
			t.Fatal(err)
		}
		step := doc["steps"].([]any)[0].(map[string]any)
		step["status"] = "CORTEX_STEP_STATUS_ERROR"
		step["error"] = map[string]any{"shortError": c.message, "fullError": c.message}
		delete(step["generic"].(map[string]any), "result")
		b, _ := json.Marshal(doc)
		raw := string(b)
		s := snapshot1216(t, raw)
		require1216(t, s, raw, raw, Verified)
		offline, err := s.OfflineJSON()
		if err != nil {
			t.Fatal(err)
		}
		require1216(t, s, raw, string(offline), Verified)
		for _, change := range [][2]string{
			{`"shortError":`, `"errorId":"unexpected","shortError":`},
			{`"generic":{`, `"taskDetails":{"id":"unexpected"},"generic":{`},
			{`"args":{`, `"args":{"unexpected":"value",`},
		} {
			bad := replace1216(t, raw, change[0], change[1])
			require1216(t, snapshot1216(t, bad), bad, raw, Unknown)
		}
		step["error"].(map[string]any)["shortError"] = "different error"
		step["error"].(map[string]any)["fullError"] = "different error"
		bad, _ := json.Marshal(doc)
		require1216(t, snapshot1216(t, string(bad)), string(bad), raw, Unknown)
		require1216(t, s, raw, string(bad), Incomplete)
	}
}

func TestCanceledFork156(t *testing.T) {
	args := map[string]any{"action": "add", "brief": "fixture", "name": "worker", "model": "fixture-model", "workspace": "branch", "fork": "coordinator"}
	var doc map[string]any
	if err := json.Unmarshal([]byte(swarm155Pair("swarm", args)), &doc); err != nil {
		t.Fatal(err)
	}
	step := doc["steps"].([]any)[0].(map[string]any)
	step["status"] = "CORTEX_STEP_STATUS_CANCELED"
	delete(step["generic"].(map[string]any), "result")
	b, _ := json.Marshal(doc)
	raw := string(b)
	s := snapshot1216(t, raw)
	require1216(t, s, raw, raw, Verified)
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, raw, string(offline), Verified)
	for _, change := range [][2]string{
		{`"fork":"coordinator"`, `"fork":"other"`},
		{`"CORTEX_STEP_STATUS_CANCELED"`, `"CORTEX_STEP_STATUS_RUNNING"`},
		{`"generic":{`, `"error":{"shortError":"unexpected","fullError":"unexpected"},"generic":{`},
		{`"args":{`, `"args":{"extra":"value",`},
	} {
		bad := replace1216(t, raw, change[0], change[1])
		require1216(t, snapshot1216(t, bad), bad, raw, Unknown)
	}
	require1216(t, s, raw, replace1216(t, raw, `"CORTEX_STEP_STATUS_CANCELED"`, `"CORTEX_STEP_STATUS_ERROR"`), Incomplete)
}

func TestChainedTermination156(t *testing.T) {
	raw := `{"cascadeId":"fixture","executorMetadatas":[{"terminationReason":"EXECUTOR_TERMINATION_REASON_CHAINED"}]}`
	s := snapshot1216(t, raw)
	require1216(t, s, raw, raw, Verified)
	// Values beyond the one authenticated addition still fail closed.
	bad := strings.Replace(raw, "EXECUTOR_TERMINATION_REASON_CHAINED", "EXECUTOR_TERMINATION_REASON_FUTURE", 1)
	require1216(t, s, raw, bad, Unknown)
}
