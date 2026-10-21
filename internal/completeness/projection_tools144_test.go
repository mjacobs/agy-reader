package completeness

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func invokeWorkspace144Pair() (string, string) {
	f := tool143Fixtures[9]
	f.args += `,"Workspace":"inherit"`
	raw, shown := f.pair()
	call := `"toolCall":{"name":"invoke_subagent","argumentsJson":"{\"TypeName\":\"researcher\",\"Role\":\"fixture role\",\"Prompt\":\"fixture prompt\",\"Model\":\"fixture-model\",\"Workspace\":\"inherit\"}"}`
	return strings.Replace(raw, `"toolCall":{"name":"invoke_subagent"}`, call, 1), strings.Replace(shown, `"toolCall":{"name":"invoke_subagent"}`, call, 1)
}

func TestProjectionTools144InheritedWorkspace(t *testing.T) {
	raw, shown := invokeWorkspace144Pair()
	require1216(t, snapshot1216(t, raw), raw, shown, Verified)
	for name, change := range map[string][2]string{
		"other selector":        {`"Workspace":"inherit"`, `"Workspace":"worktree"`},
		"empty selector":        {`"Workspace":"inherit"`, `"Workspace":""`},
		"path selector":         {`"Workspace":"inherit"`, `"Workspace":"/fixture"`},
		"mismatched call":       {`\"Workspace\":\"inherit\"`, `\"Workspace\":\"other\"`},
		"missing call argument": {`,\"Workspace\":\"inherit\"`, ``},
	} {
		t.Run(name, func(t *testing.T) {
			bad := replace1216(t, raw, change[0], change[1])
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
	for name, change := range map[string][2]string{
		"workspace URI":     {`file:///fixture`, `file:///other`},
		"child identity":    {`fixture-child`, `other-child`},
		"retained selector": {`\"Workspace\":\"inherit\"`, `\"Workspace\":\"other\"`},
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, snapshot1216(t, raw), raw, replace1216(t, shown, change[0], change[1]), Incomplete)
		})
	}
}

const skill144Path = "/fixture/skills/example/SKILL.md"
const skill144Entry = `{"type":"REFRESH_CUSTOMIZATION_TYPE_SKILL","path":"` + skill144Path + `","status":"STATUS_ENABLED"}`

func skill144Executor(id, entries string) string {
	return `{"executionId":"` + id + `","loadedCustomizations":{"customizations":[` + entries + `]}}`
}

func skill144Pair(executors, executionID, path string, marked bool) (string, string) {
	raw, shown := tool143Fixtures[2].pair()
	apply := func(s string) string {
		s = strings.ReplaceAll(s, "/fixture/notes.txt", path)
		if executionID != "" {
			s = strings.Replace(s, `"metadata":{"toolCall":`, `"metadata":{"executionId":"`+executionID+`","toolCall":`, 1)
		}
		if executors != "" {
			s = strings.Replace(s, `{"cascadeId":`, `{"executorMetadatas":[`+executors+`],"cascadeId":`, 1)
		}
		return s
	}
	raw, shown = apply(raw), apply(shown)
	if marked {
		shown = strings.Replace(shown, `"viewFile":{`, `"viewFile":{"isSkillFile":true,`, 1)
	}
	return raw, shown
}

func TestProjectionTools144SkillReads(t *testing.T) {
	executor := skill144Executor("fixture-exec", skill144Entry)
	for _, tc := range []struct {
		name, executors, executionID, path string
		marked                             bool
	}{
		{"registered skill", executor, "fixture-exec", skill144Path, true},
		{"unregistered same SKILL filename", skill144Executor("fixture-exec", ""), "fixture-exec", skill144Path, false},
		{"no executor metadata", "", "fixture-exec", skill144Path, false},
		{"missing execution identity", executor, "", skill144Path, false},
		{"another execution", executor, "other-exec", skill144Path, false},
		{"another executor has the skill", executor + "," + skill144Executor("other-exec", ""), "other-exec", skill144Path, false},
		{"disabled skill", strings.Replace(executor, "STATUS_ENABLED", "STATUS_DISABLED", 1), "fixture-exec", skill144Path, false},
		{"not a skill", strings.Replace(executor, "REFRESH_CUSTOMIZATION_TYPE_SKILL", "REFRESH_CUSTOMIZATION_TYPE_RULE", 1), "fixture-exec", skill144Path, false},
		{"case differs from registry", executor, "fixture-exec", "/fixture/skills/example/skill.md", false},
		{"reference not registered", executor, "fixture-exec", "/fixture/skills/example/references/SKILL.md", false},
		{"plain filename not registered", executor, "fixture-exec", "/fixture/SKILL.md", false},
		{"registered path has no special suffix", strings.ReplaceAll(executor, skill144Path, "/fixture/entry.md"), "fixture-exec", "/fixture/entry.md", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, shown := skill144Pair(tc.executors, tc.executionID, tc.path, tc.marked)
			s := snapshot1216(t, raw)
			require1216(t, s, raw, shown, Verified)
			var wrong string
			if tc.marked {
				wrong = strings.Replace(shown, `"isSkillFile":true,`, "", 1)
			} else {
				wrong = strings.Replace(shown, `"viewFile":{`, `"viewFile":{"isSkillFile":true,`, 1)
			}
			require1216(t, s, raw, wrong, Incomplete)
		})
	}
}

func TestProjectionTools144AmbiguousSkillRegistry(t *testing.T) {
	executor := skill144Executor("fixture-exec", skill144Entry)
	for name, executors := range map[string]string{
		"duplicate executor":       executor + "," + executor,
		"conflicting executors":    executor + "," + skill144Executor("fixture-exec", ""),
		"duplicate registration":   skill144Executor("fixture-exec", skill144Entry+","+skill144Entry),
		"conflicting registration": skill144Executor("fixture-exec", skill144Entry+","+strings.Replace(skill144Entry, "STATUS_ENABLED", "STATUS_DISABLED", 1)),
	} {
		t.Run(name, func(t *testing.T) {
			raw, shown := skill144Pair(executors, "fixture-exec", skill144Path, true)
			require1216(t, snapshot1216(t, raw), raw, shown, Unknown)
		})
	}
}

func TestProjectionTools144SkillRegistryResponseChanges(t *testing.T) {
	raw, shown := skill144Pair(skill144Executor("fixture-exec", skill144Entry), "fixture-exec", skill144Path, true)
	s := snapshot1216(t, raw)
	for name, change := range map[string][2]string{
		"registry path":     {`"path":"` + skill144Path + `"`, `"path":"/fixture/other.md"`},
		"registry status":   {`"STATUS_ENABLED"`, `"STATUS_DISABLED"`},
		"executor identity": {`"executorMetadatas":[{"executionId":"fixture-exec"`, `"executorMetadatas":[{"executionId":"other-exec"`},
		"step identity":     {`"metadata":{"executionId":"fixture-exec"`, `"metadata":{"executionId":"other-exec"`},
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, s, replace1216(t, raw, change[0], change[1]), shown, Incomplete)
			require1216(t, s, raw, replace1216(t, shown, change[0], change[1]), Incomplete)
		})
	}
}

// Synthetic source/default pairs follow the agy 1.4.4 native capture. Expected
// output is written independently of the projection; no private content is used.
const (
	background144Args   = `"CommandLine":"sleep 30","WaitMsBeforeAsync":"5000","toolSummary":"Wait for fixture"`
	background144Meta   = `"metadata":{"toolCall":{"name":"run_command","argumentsJson":"{\"CommandLine\":\"sleep 30\",\"WaitMsBeforeAsync\":5000,\"toolSummary\":\"Wait for fixture\"}"},"preToolHookResults":[{"decision":"allow"}],"internalMetadata":{"statusTransitions":[{"updatedStatus":"CORTEX_STEP_STATUS_PENDING"},{"updatedStatus":"CORTEX_STEP_STATUS_RUNNING"},{"updatedStatus":"CORTEX_STEP_STATUS_CANCELED"}]}}`
	background144Task   = `"taskDetails":{"id":"fixture/task-1","logUri":"file:///fixture/task.log","progress":"Waiting","description":"Fixture command","title":"Wait","movedToBackgroundAt":"2026-01-01T00:00:01Z"}`
	background144Common = `"status":"CORTEX_STEP_STATUS_CANCELED",` + background144Meta + `,` + background144Task
	background144Raw    = `{"cascadeId":"fixture-parent","steps":[{"type":"CORTEX_STEP_TYPE_GENERIC",` + background144Common + `,"generic":{"args":{` + background144Args + `},"result":{"stepRenderInfo":{"titlePrefix":"Running","icon":"terminal","aggregationKey":"commands"}}}}]}`
	background144Shown  = `{"cascadeId":"fixture-parent","steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND",` + background144Common + `,"runCommand":{"commandLine":"sleep 30","proposedCommandLine":"sleep 30","combinedOutput":{}}}]}`
	manage144Common     = `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"manage_subagents","argumentsJson":"{\"Action\":\"list\"}"},"completedAt":"2026-01-01T00:00:02Z"}`
	manage144Result     = `"result":"Two fixture children","fullOutputUri":"file:///fixture/children.txt","stepRenderInfo":{"titlePrefix":"Listed children"},"payload":{"@type":"type.googleapis.com/exa.cortex.core.tools.ManageSubagents","action":"list","conversationIds":["fixture-child-a","fixture-child-b"]}`
	manage144Raw        = `{"cascadeId":"fixture-parent","steps":[{` + manage144Common + `,"generic":{"args":{"Action":"list"},"result":{` + manage144Result + `}}}]}`
	manage144Shown      = `{"cascadeId":"fixture-parent","steps":[{` + manage144Common + `,"generic":{"args":{"Action":"list"},"result":{` + manage144Result + `,"metadata":{"action":"list","conversation_ids":"fixture-child-a,fixture-child-b"}}}}]}`
)

func TestProjectionTools144(t *testing.T) {
	for name, pair := range map[string][2]string{
		"canceled background command": {background144Raw, background144Shown},
		"background without progress": {strings.Replace(background144Raw, `"progress":"Waiting",`, "", 1), strings.Replace(background144Shown, `"progress":"Waiting",`, "", 1)},
		"list subagents":              {manage144Raw, manage144Shown},
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, snapshot1216(t, pair[0]), pair[0], pair[1], Verified)
			m := trajectory1216(t, pair[0])
			before := proto.Clone(m)
			if _, err := project(m); err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(before, m) {
				t.Fatal("projection mutated source")
			}
		})
	}
}

func TestProjectionTools144Refusals(t *testing.T) {
	for _, tc := range []struct{ name, raw, shown, before, after string }{
		{"background done", background144Raw, background144Shown, `"status":"CORTEX_STEP_STATUS_CANCELED"`, `"status":"CORTEX_STEP_STATUS_DONE"`},
		{"background error", background144Raw, background144Shown, `"generic":{`, `"error":{"shortError":"canceled"},"generic":{`},
		{"background result text", background144Raw, background144Shown, `"result":{"stepRenderInfo"`, `"result":{"result":"output","stepRenderInfo"`},
		{"background result metadata", background144Raw, background144Shown, `"result":{"stepRenderInfo"`, `"result":{"metadata":{"action":"future"},"stepRenderInfo"`},
		{"background no render", background144Raw, background144Shown, `"stepRenderInfo":{"titlePrefix":"Running","icon":"terminal","aggregationKey":"commands"}`, ``},
		{"background no moved time", background144Raw, background144Shown, `,"movedToBackgroundAt":"2026-01-01T00:00:01Z"`, ``},
		{"background no task id", background144Raw, background144Shown, `"id":"fixture/task-1",`, ``},
		{"background unknown argument", background144Raw, background144Shown, `"args":{`, `"args":{"Future":"x",`},
		{"background cwd", background144Raw, background144Shown, `"args":{`, `"args":{"Cwd":"/fixture",`},
		{"background wait", background144Raw, background144Shown, `"WaitMsBeforeAsync":"5000"`, `"WaitMsBeforeAsync":"10000"`},
		{"background mismatched argument", background144Raw, background144Shown, `"CommandLine":"sleep 30"`, `"CommandLine":"sleep 60"`},
		{"background hook denial", background144Raw, background144Shown, `"decision":"allow"`, `"decision":"deny"`},
		{"background waiting history", background144Raw, background144Shown, `"updatedStatus":"CORTEX_STEP_STATUS_RUNNING"`, `"updatedStatus":"CORTEX_STEP_STATUS_WAITING"`},
		{"background wrong tool", background144Raw, background144Shown, `"name":"run_command"`, `"name":"view_file"`},
		{"manage running", manage144Raw, manage144Shown, `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_RUNNING"`},
		{"manage wrong tool", manage144Raw, manage144Shown, `"name":"manage_subagents"`, `"name":"manage_task"`},
		{"manage extra argument", manage144Raw, manage144Shown, `"args":{`, `"args":{"ConversationId":"fixture-child-a",`},
		{"manage task details", manage144Raw, manage144Shown, `"generic":{`, `"taskDetails":{"id":"fixture/task-1"},"generic":{`},
		{"manage metadata in source", manage144Raw, manage144Shown, `"result":{"result"`, `"result":{"metadata":{"action":"list"},"result"`},
		{"manage error", manage144Raw, manage144Shown, `"generic":{`, `"error":{"shortError":"failed"},"generic":{`},
		{"manage other action", manage144Raw, manage144Shown, `"action":"list"`, `"action":"kill"`},
		{"manage mismatched action", manage144Raw, manage144Shown, `"args":{"Action":"list"}`, `"args":{"Action":"kill"}`},
		{"manage one child", manage144Raw, manage144Shown, `"fixture-child-a","fixture-child-b"`, `"fixture-child-a"`},
		{"manage no children", manage144Raw, manage144Shown, `"fixture-child-a","fixture-child-b"`, ``},
		{"manage empty child", manage144Raw, manage144Shown, `"fixture-child-a","fixture-child-b"`, `"","fixture-child-b"`},
		{"manage ambiguous separator", manage144Raw, manage144Shown, `"fixture-child-a","fixture-child-b"`, `"fixture,child-a","fixture-child-b"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := replace1216(t, tc.raw, tc.before, tc.after)
			s := snapshot1216(t, bad)
			require1216(t, s, bad, tc.shown, Unknown)
			if _, sourceProven := s.compareResponses([]byte(bad), []byte(tc.shown)); !sourceProven {
				t.Fatal("unsupported source must produce a source-proven refusal")
			}
		})
	}
	for name, pair := range map[string][2]string{"background": {background144Raw, background144Shown}, "manage": {manage144Raw, manage144Shown}} {
		for _, field := range []string{`"permissions":{},`, `"completedInteractions":[{}],`, `"requestedInteraction":{"permission":{}},`} {
			t.Run(name+field, func(t *testing.T) {
				raw := strings.Replace(pair[0], `"generic":`, field+`"generic":`, 1)
				require1216(t, snapshot1216(t, raw), raw, pair[1], Unknown)
			})
		}
	}
}

func TestProjectionTools144ResponseChanges(t *testing.T) {
	for _, tc := range []struct{ name, raw, shown, before, after, where string }{
		{"dropped render data still checked", background144Raw, background144Shown, `"titlePrefix":"Running"`, `"titlePrefix":"Finished"`, "raw"},
		{"background command", background144Raw, background144Shown, `"commandLine":"sleep 30"`, `"commandLine":"sleep 60"`, "shown"},
		{"background task", background144Raw, background144Shown, `"id":"fixture/task-1"`, `"id":"fixture/task-2"`, "shown"},
		{"background moved time", background144Raw, background144Shown, `2026-01-01T00:00:01Z`, `2026-01-01T00:00:03Z`, "shown"},
		{"background output", background144Raw, background144Shown, `"combinedOutput":{}`, `"combinedOutput":{"full":"new output"}`, "shown"},
		{"background exit code", background144Raw, background144Shown, `"combinedOutput":{}`, `"combinedOutput":{},"exitCode":0`, "shown"},
		{"background blocking", background144Raw, background144Shown, `"combinedOutput":{}`, `"combinedOutput":{},"blocking":true`, "shown"},
		{"background cwd added", background144Raw, background144Shown, `"combinedOutput":{}`, `"combinedOutput":{},"cwd":"/fixture"`, "shown"},
		{"manage metadata ids", manage144Raw, manage144Shown, `"conversation_ids":"fixture-child-a,fixture-child-b"`, `"conversation_ids":"fixture-child-b,fixture-child-a"`, "shown"},
		{"manage metadata action", manage144Raw, manage144Shown, `"metadata":{"action":"list"`, `"metadata":{"action":"kill"`, "shown"},
		{"manage dropped metadata", manage144Raw, manage144Shown, `,"metadata":{"action":"list","conversation_ids":"fixture-child-a,fixture-child-b"}`, ``, "shown"},
		{"manage result changed", manage144Raw, manage144Shown, `Two fixture children`, `One fixture child`, "raw"},
		{"manage payload changed", manage144Raw, manage144Shown, `"fixture-child-a","fixture-child-b"`, `"fixture-child-c","fixture-child-b"`, "raw"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := snapshot1216(t, tc.raw)
			raw, shown := tc.raw, tc.shown
			if tc.where == "raw" {
				raw = replace1216(t, raw, tc.before, tc.after)
			} else {
				shown = replace1216(t, shown, tc.before, tc.after)
			}
			require1216(t, s, raw, shown, Incomplete)
		})
	}
}

func TestProjectionTools144UnknownWireFields(t *testing.T) {
	for name, raw := range map[string]string{"background": background144Raw, "manage": manage144Raw} {
		t.Run(name, func(t *testing.T) {
			m := trajectory1216(t, raw).ProtoReflect()
			step := tool143Get(m, "steps").List().Get(0).Message()
			result := tool143Get(tool143Get(step, "generic").Message(), "result").Message()
			extra := protowire.AppendVarint(protowire.AppendTag(nil, 9999, protowire.VarintType), 1)
			if name == "manage" {
				payload := tool143Get(result, "payload").Message()
				data := append(append([]byte(nil), tool143Get(payload, "value").Bytes()...), extra...)
				tool143Set(payload, "value", protoreflect.ValueOfBytes(data))
			} else {
				result.SetUnknown(extra)
			}
			if _, err := finishSnapshot(m.Interface()); err == nil {
				t.Fatal("unknown wire field accepted")
			}
		})
	}
}
