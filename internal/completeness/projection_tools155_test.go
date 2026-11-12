package completeness

import (
	"encoding/json"
	"strings"
	"testing"
)

func swarm155Pair(tool string, args map[string]any) string {
	call, _ := json.Marshal(args)
	saved := map[string]string{}
	for k, v := range args {
		if s, ok := v.(string); ok {
			saved[k] = s
		} else {
			b, _ := json.Marshal(v)
			saved[k] = string(b)
		}
	}
	b, _ := json.Marshal(map[string]any{"cascadeId": "fixture", "steps": []any{map[string]any{
		"type": "CORTEX_STEP_TYPE_GENERIC", "status": "CORTEX_STEP_STATUS_DONE",
		"metadata": map[string]any{"toolCall": map[string]any{"name": tool, "argumentsJson": string(call)}},
		"generic":  map[string]any{"args": saved, "result": map[string]any{"result": "fixture result", "fullOutputUri": "file:///fixture/result.txt", "stepRenderInfo": map[string]any{}}},
	}}})
	return string(b)
}

func TestSwarm155(t *testing.T) {
	cases := []struct {
		tool string
		args map[string]any
	}{
		{"swarm", map[string]any{"action": "status"}},
		{"swarm", map[string]any{"action": "models"}},
		{"swarm", map[string]any{"action": "title", "title": "fixture"}},
		{"swarm", map[string]any{"action": "kill", "name": "worker"}},
		{"swarm", map[string]any{"action": "add", "brief": "fixture", "model": "fixture-model", "name": "worker", "workspace": "inherit"}},
		{"read_board", map[string]any{"limit": 10}},
		{"read_board", map[string]any{"since": 1}},
		{"post_message", map[string]any{"text": "fixture post"}},
		{"swarm_wait", map[string]any{"since": 1}},
	}
	for _, c := range cases {
		t.Run(c.tool, func(t *testing.T) {
			raw := swarm155Pair(c.tool, c.args)
			s := snapshot1216(t, raw)
			require1216(t, s, raw, raw, Verified)
			offline, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, raw, string(offline), Verified)
			changed := strings.Replace(raw, "fixture result", "changed result", 1)
			require1216(t, s, raw, changed, Incomplete)
			require1216(t, s, changed, raw, Incomplete)
			var mismatch map[string]any
			if err := json.Unmarshal([]byte(raw), &mismatch); err != nil {
				t.Fatal(err)
			}
			mismatch["steps"].([]any)[0].(map[string]any)["metadata"].(map[string]any)["toolCall"].(map[string]any)["argumentsJson"] = "{}"
			badCall, _ := json.Marshal(mismatch)
			require1216(t, snapshot1216(t, string(badCall)), string(badCall), raw, Unknown)
			for _, change := range [][2]string{
				{`"CORTEX_STEP_STATUS_DONE"`, `"CORTEX_STEP_STATUS_ERROR"`},
				{`"generic":{`, `"taskDetails":{"id":"unexpected"},"generic":{`},
				{`"result":"fixture result"`, `"result":"fixture result","metadata":{"unexpected":"value"}`},
				{`"args":{`, `"args":{"unexpected":"value",`},
			} {
				bad := replace1216(t, raw, change[0], change[1])
				require1216(t, snapshot1216(t, bad), bad, raw, Unknown)
			}
		})
	}
}

func TestSwarm155UnobservedCalls(t *testing.T) {
	for _, c := range []struct {
		tool string
		args map[string]any
	}{
		{"swarm", map[string]any{"action": "title", "title": ""}},
		{"swarm", map[string]any{"action": "kill"}},
		{"read_board", map[string]any{"limit": 0}},
		{"swarm_wait", map[string]any{"since": -1}},
		{"swarm_wait", map[string]any{"since": "01"}},
		{"post_message", map[string]any{"text": ""}},
	} {
		raw := swarm155Pair(c.tool, c.args)
		if _, err := snapshot1216(t, raw).OfflineJSON(); err == nil {
			t.Fatalf("accepted unobserved %s %v", c.tool, c.args)
		}
	}
}

func TestWorkspace155(t *testing.T) {
	for _, workspace := range []string{"share", "branch"} {
		raw, shown := invokeWorkspace144Pair()
		raw, shown = strings.ReplaceAll(raw, "inherit", workspace), strings.ReplaceAll(shown, "inherit", workspace)
		s := snapshot1216(t, raw)
		require1216(t, s, raw, shown, Verified)
		offline, err := s.OfflineJSON()
		if err != nil {
			t.Fatal(err)
		}
		require1216(t, s, raw, string(offline), Verified)
		bad := strings.ReplaceAll(raw, workspace, "unobserved")
		require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		bad = replace1216(t, raw, `"Workspace":"`+workspace+`"`, `"Workspace":"inherit"`)
		require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
	}
}

func pairTwoChildren155(t *testing.T, action string) (string, string) {
	raw, shown := subagents148Pair(t, action)
	convert := func(s string) string {
		var doc map[string]any
		if err := json.Unmarshal([]byte(s), &doc); err != nil {
			t.Fatal(err)
		}
		step := doc["steps"].([]any)[0].(map[string]any)
		g := step["generic"].(map[string]any)
		r := g["result"].(map[string]any)
		p := r["payload"].(map[string]any)
		p["conversationIds"] = []string{"fixture-child", "second"}
		state := map[string]any{"conversationId": "second"}
		if action == "list" {
			state["stepIndex"] = -1
		}
		p["subagents"] = append(p["subagents"].([]any), state)
		if action == "kill" {
			g["args"].(map[string]any)["ConversationIds"] = `["fixture-child","second"]`
			step["metadata"].(map[string]any)["toolCall"].(map[string]any)["argumentsJson"] = `{"Action":"kill","ConversationIds":["fixture-child","second"]}`
		}
		if meta, ok := r["metadata"].(map[string]any); ok {
			meta["conversation_ids"] = "fixture-child,second"
			if action == "list" {
				meta["subagent_step_indices"] = `{"fixture-child":-1,"second":-1}`
			} else {
				meta["killed_conversation_ids"] = `["fixture-child","second"]`
			}
		}
		b, _ := json.Marshal(doc)
		return string(b)
	}
	return convert(raw), convert(shown)
}

func TestTwoChildren155(t *testing.T) {
	for _, action := range []string{"list", "kill"} {
		raw, shown := pairTwoChildren155(t, action)
		s := snapshot1216(t, raw)
		require1216(t, s, raw, shown, Verified)
		offline, err := s.OfflineJSON()
		if err != nil {
			t.Fatal(err)
		}
		require1216(t, s, raw, string(offline), Verified)
		require1216(t, s, raw, strings.Replace(shown, `"conversation_ids":"fixture-child,second"`, `"conversation_ids":"second,fixture-child"`, 1), Incomplete)
		for _, change := range [][2]string{
			{`"conversationId":"second"`, `"conversationId":"wrong"`},
			{`"conversationIds":["fixture-child","second"]`, `"conversationIds":["second","fixture-child"]`},
		} {
			bad := replace1216(t, raw, change[0], change[1])
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		}
		duplicate := strings.ReplaceAll(raw, "second", "fixture-child")
		require1216(t, snapshot1216(t, duplicate), duplicate, shown, Unknown)
	}
}

func TestEmptyTaskList155(t *testing.T) {
	f := tool143Fixtures[6]
	f.payload = `"@type":"type.googleapis.com/exa.cortex.core.tools.ManageTask","action":"list"`
	f.presented = `"action":"list"`
	raw, shown := f.pair()
	call := `"name":"manage_task","argumentsJson":"{\"Action\":\"list\"}"`
	raw = strings.Replace(raw, `"name":"manage_task"`, call, 1)
	shown = strings.Replace(shown, `"name":"manage_task"`, call, 1)
	s := snapshot1216(t, raw)
	require1216(t, s, raw, shown, Verified)
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, raw, string(offline), Verified)
	for _, change := range [][2]string{
		{`"Action":"list"`, `"Action":"status"`},
		{`"args":{`, `"args":{"TaskId":"unexpected",`},
		{`"generic":{`, `"taskDetails":{"id":"unexpected"},"generic":{`},
	} {
		bad := replace1216(t, raw, change[0], change[1])
		require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
	}
	require1216(t, s, raw, replace1216(t, shown, `"metadata":{"action":"list"}`, `"metadata":{"action":"list","task_id":"invented"}`), Incomplete)
}

func TestTimestampedSkill155(t *testing.T) {
	prefix := "Created At: 2026-11-10T09:52:15-08:00\nCompleted At: 2026-11-10T09:52:15-08:00\n"
	content := "Showing lines 1-4 of 8\n1: ---\n2: name: sample\n3: description: A sample skill.\n4: ---"
	raw, old := skill150Pair(prefix + content)
	shown := strings.Replace(old, `"isSkillFile":true,`, `"isSkillFile":true,"skillMetadata":{"name":"sample","description":"A sample skill."},`, 1)
	s := snapshot1216(t, raw)
	require1216(t, s, raw, shown, Verified)
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, raw, string(offline), Verified)
	require1216(t, s, strings.Replace(raw, "A sample skill.", "changed", 1), shown, Incomplete)
	require1216(t, s, raw, strings.Replace(shown, `"description":"A sample skill."`, `"description":"changed"`, 1), Incomplete)
	for _, bad := range []string{
		strings.Replace(prefix, "Completed At:", "Other At:", 1) + content,
		strings.Replace(prefix, "2026-11-10T09:52:15-08:00", "invalid", 1) + content,
		prefix + strings.Replace(content, "1: ---", "1: other", 1),
		prefix + strings.Replace(content, "name: sample", "extra: sample", 1),
		prefix + strings.Replace(content, "Showing lines 1-", "Partial read (lines 1-", 1),
	} {
		if _, ok := skillFrontmatter150(bad); ok {
			t.Fatal("accepted unsupported timestamped skill read")
		}
	}
}

func TestTwoChildren155UnobservedStates(t *testing.T) {
	for _, action := range []string{"list", "kill"} {
		raw, shown := pairTwoChildren155(t, action)
		for _, change := range []func(map[string]any){
			func(p map[string]any) {
				p["conversationIds"] = append(p["conversationIds"].([]any), "third")
				p["subagents"] = append(p["subagents"].([]any), map[string]any{"conversationId": "third"})
			},
			func(p map[string]any) { p["subagents"] = p["subagents"].([]any)[:1] },
			func(p map[string]any) { p["subagents"].([]any)[1].(map[string]any)["stepIndex"] = 0 },
			func(p map[string]any) { p["action"] = "other" },
		} {
			var doc map[string]any
			if err := json.Unmarshal([]byte(raw), &doc); err != nil {
				t.Fatal(err)
			}
			p := doc["steps"].([]any)[0].(map[string]any)["generic"].(map[string]any)["result"].(map[string]any)["payload"].(map[string]any)
			change(p)
			bad, _ := json.Marshal(doc)
			require1216(t, snapshot1216(t, string(bad)), string(bad), shown, Unknown)
		}
	}
}

func TestTimestampedImageRead155(t *testing.T) {
	prefix := "Created At: 2026-11-10T14:56:09-08:00\\nCompleted At: 2026-11-10T14:56:11-08:00\\n"
	for _, mime := range []string{"image/jpeg", "image/png", "image/webp"} {
		raw, shown := imageReadPair147(mime)
		raw = replace1216(t, raw, `"result":"Showing binary file of type `+mime+`"`, `"result":"`+prefix+`Showing binary file of type `+mime+`"`)
		shown = replace1216(t, shown, `"content":"Showing binary file of type `+mime+`"`, `"content":"`+prefix+`Showing binary file of type `+mime+`"`)
		s := snapshot1216(t, raw)
		require1216(t, s, raw, shown, Verified)
		offline, err := s.OfflineJSON()
		if err != nil {
			t.Fatal(err)
		}
		require1216(t, s, raw, string(offline), Verified)
		require1216(t, s, strings.Replace(raw, "2026-11-10T14:56:09-08:00", "2026-11-10T14:56:10-08:00", 1), shown, Incomplete)
		require1216(t, s, raw, strings.Replace(shown, "2026-11-10T14:56:09-08:00", "2026-11-10T14:56:10-08:00", 1), Incomplete)
	}

	// Downscaled PNG stored as JPEG with timestamps.
	convRaw, convShown := convertedImage1410(t)
	convRaw = replace1216(t, convRaw, `"result":"Showing binary file of type image/png"`, `"result":"`+prefix+`Showing binary file of type image/png"`)
	convShown = replace1216(t, convShown, `"content":"Showing binary file of type image/png"`, `"content":"`+prefix+`Showing binary file of type image/png"`)
	cs := snapshot1216(t, convRaw)
	require1216(t, cs, convRaw, convShown, Verified)
	convOffline, err := cs.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, cs, convRaw, string(convOffline), Verified)

	// Malformed timestamp lines fail-closed as Unknown.
	raw, shown := imageReadPair147("image/png")
	for _, badPrefix := range []string{
		"Created At: not-a-date\\nCompleted At: 2026-11-10T14:56:11-08:00\\n",
		"Created At: 2026-11-10T14:56:09-08:00\\nCompleted At: not-a-date\\n",
		"Created At: 2026-11-10T14:56:09-08:00\\nOther Line: 2026-11-10T14:56:11-08:00\\n",
		"Created At: 2026-11-10T14:56:09-08:00\\n",
	} {
		badRaw := replace1216(t, raw, `"result":"Showing binary file of type image/png"`, `"result":"`+badPrefix+`Showing binary file of type image/png"`)
		badShown := replace1216(t, shown, `"content":"Showing binary file of type image/png"`, `"content":"`+badPrefix+`Showing binary file of type image/png"`)
		require1216(t, snapshot1216(t, badRaw), badRaw, badShown, Unknown)
	}
}
