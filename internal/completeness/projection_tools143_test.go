package completeness

import (
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Synthetic source/presentation pairs reproduce the mappings in the frozen
// 1.4.3 native capture. Expected presentations are independent of project().
type tool143Fixture struct {
	name, tool, payload, args, presented string
	task, canceled                       bool
}

var tool143Fixtures = []tool143Fixture{
	{name: "command sandbox", tool: "run_command", payload: `"@type":"type.googleapis.com/exa.cortex.core.tools.RunCommand","commandLine":"echo fixture","cwd":"/fixture","shellName":"bash","sandboxDetails":{"ranInSandbox":true,"sandboxErrors":"fixture denial"}`,
		args:      `"CommandLine":"echo fixture","WaitMsBeforeAsync":"5000","toolSummary":"Fixture"`,
		presented: `"type":"CORTEX_STEP_TYPE_RUN_COMMAND","runCommand":{"commandLine":"echo fixture","proposedCommandLine":"echo fixture","cwd":"/fixture","shellName":"bash","exitCode":0,"blocking":true,"ranInSandbox":true,"sandboxErrors":"fixture denial","combinedOutput":{"full":"fixture result"}}`},
	{name: "command task", tool: "run_command", payload: `"@type":"type.googleapis.com/exa.cortex.core.tools.RunCommand","commandLine":"exit 2","cwd":"/fixture","shellName":"bash","exitCode":2`,
		args: `"CommandLine":"exit 2","BypassSandbox":"true"`, task: true,
		presented: `"type":"CORTEX_STEP_TYPE_RUN_COMMAND","runCommand":{"commandLine":"exit 2","proposedCommandLine":"exit 2","cwd":"/fixture","shellName":"bash","exitCode":2,"combinedOutput":{"full":"fixture result"}}`},
	{name: "read range", tool: "read_file", payload: `"@type":"type.googleapis.com/exa.cortex.core.tools.ReadFile","path":"/fixture/notes.txt","startLine":2,"endLine":4,"fileNumLines":8,"fileNumBytes":80,"lineRangeBytes":30`,
		args:      `"Path":"/fixture/notes.txt","StartLine":"2","EndLine":"4"`,
		presented: `"type":"CORTEX_STEP_TYPE_VIEW_FILE","viewFile":{"absolutePathUri":"file:///fixture/notes.txt","startLine":1,"endLine":3,"numLines":8,"numBytes":80,"lineRangeBytes":30,"content":"fixture result"}`},
	{name: "empty file", tool: "write_file", payload: `"@type":"type.googleapis.com/exa.cortex.core.tools.FileEdit","kind":"CREATE","path":"/fixture/empty.txt","diff":{"unifiedDiff":{}},"diffStats":{}`,
		args:      `"Path":"/fixture/empty.txt","Content":"","Overwrite":"true"`,
		presented: `"type":"CORTEX_STEP_TYPE_CODE_ACTION","codeAction":{"actionSpec":{"createFile":{"path":{"absoluteUri":"file:///fixture/empty.txt"}}},"actionResult":{"edit":{"absoluteUri":"file:///fixture/empty.txt","createFile":true,"diff":{"unifiedDiff":{}}}},"diffStats":{}}`},
	{name: "edit file", tool: "edit_file", payload: `"@type":"type.googleapis.com/exa.cortex.core.tools.FileEdit","kind":"EDIT","path":"/fixture/notes.txt","diff":{"unifiedDiff":{"lines":[{"text":"fixture","type":"UNIFIED_DIFF_LINE_TYPE_INSERT"}]}},"diffStats":{"additions":1}`,
		args:      `"Path":"/fixture/notes.txt","Target":"old","Replacement":"fixture"`,
		presented: `"type":"CORTEX_STEP_TYPE_CODE_ACTION","codeAction":{"actionSpec":{"command":{"isEdit":true,"file":{"absoluteUri":"file:///fixture/notes.txt"}}},"actionResult":{"edit":{"absoluteUri":"file:///fixture/notes.txt","diff":{"unifiedDiff":{"lines":[{"text":"fixture","type":"UNIFIED_DIFF_LINE_TYPE_INSERT"}]}}}},"diffStats":{"additions":1}}`},
	{name: "manage status", tool: "manage_task", payload: `"@type":"type.googleapis.com/exa.cortex.core.tools.ManageTask","action":"status","taskIds":["fixture/task-1"]`, args: `"Action":"status","TaskId":"fixture/task-1"`, presented: `"action":"status","task_id":"fixture/task-1","task_ids":"fixture/task-1"`},
	{name: "manage list", tool: "manage_task", payload: `"@type":"type.googleapis.com/exa.cortex.core.tools.ManageTask","action":"list","taskIds":["fixture/task-1"]`, args: `"Action":"list"`, presented: `"action":"list","task_id":"fixture/task-1","task_ids":"fixture/task-1"`},
	{name: "manage kill", tool: "manage_task", payload: `"@type":"type.googleapis.com/exa.cortex.core.tools.ManageTask","action":"kill","taskIds":["fixture/task-1"]`, args: `"Action":"kill","TaskId":"fixture/task-1"`, presented: `"action":"kill","task_id":"fixture/task-1","task_ids":"fixture/task-1"`},
	{name: "schedule canceled", tool: "schedule", payload: `"@type":"type.googleapis.com/exa.cortex.core.tools.Schedule","mode":"timer","outcome":"cancelled"`, args: `"DurationSeconds":"300","Prompt":"fixture timer","TimerCondition":"once"`, task: true, canceled: true, presented: `"mode":"timer","outcome":"cancelled"`},
	{name: "invoke child", tool: "invoke_subagent", payload: `"@type":"type.googleapis.com/exa.cortex.core.tools.InvokeSubagent","subagents":[{"typeName":"researcher","role":"fixture role"}],"results":[{"conversationId":"fixture-child","logAbsoluteUri":"file:///fixture/child.log","workspaceUris":["file:///fixture"]}]`, args: `"TypeName":"researcher","Role":"fixture role","Prompt":"fixture prompt","Model":"fixture-model"`,
		presented: `"type":"CORTEX_STEP_TYPE_INVOKE_SUBAGENT","invokeSubagent":{"subagents":[{"typeName":"researcher","role":"fixture role"}],"results":[{"conversationId":"fixture-child","logAbsoluteUri":"file:///fixture/child.log","workspaceUris":["file:///fixture"]}]}`},
}

func (f tool143Fixture) pair() (string, string) {
	status := "DONE"
	if f.canceled {
		status = "CANCELED"
	}
	common := `"status":"CORTEX_STEP_STATUS_` + status + `","metadata":{"toolCall":{"name":"` + f.tool + `"},"completedAt":"2026-01-01T00:00:02Z"}`
	if f.task {
		common += `,"taskDetails":{"id":"fixture/task-1","logUri":"file:///fixture/task.log"}`
	}
	result := `"result":"fixture result","fullOutputUri":"file:///fixture/output.txt","stepRenderInfo":{"title":"fixture title"},"payload":{` + f.payload + `}`
	generic := `"generic":{"args":{` + f.args + `},"result":{` + result + `}}`
	envelope := func(step string) string {
		return `{"cascadeId":"fixture-parent","metadata":{},"steps":[{` + step + `}]}`
	}
	raw := envelope(`"type":"CORTEX_STEP_TYPE_GENERIC",` + common + `,` + generic)
	if f.tool == "manage_task" || f.tool == "schedule" {
		return raw, envelope(`"type":"CORTEX_STEP_TYPE_GENERIC",` + common + `,"generic":{"args":{` + f.args + `},"result":{` + result + `,"metadata":{` + f.presented + `}}}`)
	}
	return raw, envelope(f.presented + `,` + common)
}

func TestProjectionTools143(t *testing.T) {
	for _, f := range tool143Fixtures {
		t.Run(f.name, func(t *testing.T) {
			raw, shown := f.pair()
			s := snapshot1216(t, raw)
			require1216(t, s, raw, shown, Verified)
			// Altering even wrapper-only text must still fail full-source equality.
			require1216(t, s, strings.Replace(raw, "fixture result", "changed result", 1), shown, Incomplete)
			// Dropping any presented step cannot pass on counts or schema alone.
			require1216(t, s, raw, `{"cascadeId":"fixture-parent","metadata":{},"steps":[]}`, Incomplete)
			for _, mutation := range []struct{ before, after string }{
				{`"name":"` + f.tool + `"`, `"name":"unsupported_tool"`},
				{`"args":{`, `"args":{"Unexpected":"value",`},
			} {
				bad := replace1216(t, raw, mutation.before, mutation.after)
				require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
			}
			bad := strings.ReplaceAll(strings.ReplaceAll(raw, "CORTEX_STEP_STATUS_DONE", "CORTEX_STEP_STATUS_RUNNING"), "CORTEX_STEP_STATUS_CANCELED", "CORTEX_STEP_STATUS_RUNNING")
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
			// Unknown wire fields inside a known Any payload are never discarded.
			m := trajectory1216(t, raw).ProtoReflect()
			step := tool143Get(m, "steps").List().Get(0).Message()
			payload := tool143Get(tool143Get(tool143Get(step, "generic").Message(), "result").Message(), "payload").Message()
			data := append([]byte(nil), tool143Get(payload, "value").Bytes()...)
			data = protowire.AppendTag(data, 9999, protowire.VarintType)
			data = protowire.AppendVarint(data, 1)
			tool143Set(payload, "value", protoreflect.ValueOfBytes(data))
			if _, err := finishSnapshot(m.Interface()); err == nil {
				t.Fatal("unknown nested payload field accepted")
			}
		})
	}
}

func TestProjectionTools143PresentedMutations(t *testing.T) {
	for _, f := range tool143Fixtures {
		raw, shown := f.pair()
		var doc map[string]any
		if err := json.Unmarshal([]byte(shown), &doc); err != nil {
			t.Fatal(err)
		}
		step := doc["steps"].([]any)[0].(map[string]any)
		for field := range step {
			if field == "type" || field == "status" {
				continue
			}
			t.Run(f.name+" drop "+field, func(t *testing.T) {
				value := step[field]
				delete(step, field)
				changed, err := json.Marshal(doc)
				if err != nil {
					t.Fatal(err)
				}
				step[field] = value
				require1216(t, snapshot1216(t, raw), raw, string(changed), Incomplete)
			})
		}
	}
}

func TestProjectionTools143UnqualifiedFields(t *testing.T) {
	for _, tc := range []struct {
		index         int
		before, after string
	}{
		{0, `"shellName":"bash"`, `"timedOut":true,"shellName":"bash"`},
		{2, `"startLine":2`, `"startLine":0`},
		{2, `/fixture/notes.txt`, `/fixture/../notes.txt`},
		{3, `"kind":"CREATE"`, `"kind":"EDIT"`},
		{4, `"path":"/fixture/notes.txt"`, `"artifactMetadata":{"summary":"unqualified edit"},"path":"/fixture/notes.txt"`},
		{5, `"action":"status"`, `"action":"unqualified"`},
		{5, `"taskIds":["fixture/task-1"]`, `"taskIds":["fixture/task-1","fixture/task-2"]`},
		{8, `"outcome":"cancelled"`, `"outcome":"completed"`},
		{9, `"subagents":[`, `"taskMode":true,"subagents":[`},
	} {
		f := tool143Fixtures[tc.index]
		t.Run(f.name+tc.before, func(t *testing.T) {
			raw, shown := f.pair()
			bad := replace1216(t, raw, tc.before, tc.after)
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
}

func TestProjectionTools143ChangedValues(t *testing.T) {
	for _, tc := range []struct {
		index         int
		before, after string
	}{
		{0, `"commandLine":"echo fixture"`, `"commandLine":"echo changed"`},
		{0, `"ranInSandbox":true`, `"ranInSandbox":false`},
		{1, `"exitCode":2`, `"exitCode":0`},
		{2, `"content":"fixture result"`, `"content":"changed content"`},
		{2, `"endLine":3`, `"endLine":4`},
		{3, `"createFile":true`, `"createFile":false`},
		{4, `"text":"fixture"`, `"text":"different diff"`},
		{5, `"task_id":"fixture/task-1"`, `"task_id":"fixture/task-2"`},
		{8, `"outcome":"cancelled"`, `"outcome":"completed"`},
		{9, `"conversationId":"fixture-child"`, `"conversationId":"wrong-child"`},
	} {
		f := tool143Fixtures[tc.index]
		t.Run(f.name+tc.before, func(t *testing.T) {
			raw, shown := f.pair()
			changed := replace1216(t, shown, tc.before, tc.after)
			require1216(t, snapshot1216(t, raw), raw, changed, Incomplete)
		})
	}
}

func FuzzProjectionTools143(f *testing.F) {
	for _, fixture := range tool143Fixtures {
		raw, _ := fixture.pair()
		f.Add([]byte(raw))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64<<10 {
			return
		}
		m, err := rpcMessage(data)
		if err != nil {
			return
		}
		before := proto.Clone(m)
		_, _ = project(m)
		if !proto.Equal(before, m) {
			t.Fatal("projection mutated source")
		}
	})
}

// Check projection leaves its source untouched, including repeated child data.
func TestProjectionTools143DoesNotMutateSource(t *testing.T) {
	for _, f := range tool143Fixtures {
		raw, _ := f.pair()
		m := trajectory1216(t, raw)
		before := proto.Clone(m)
		if _, err := project(m); err != nil {
			t.Fatal(err)
		}
		if !proto.Equal(before, m) {
			t.Fatal("projection mutated its source")
		}
	}
}

func TestProjectionTools143PermissionRecordsUnsupported(t *testing.T) {
	raw, shown := tool143Fixtures[0].pair()
	for _, field := range []string{
		`"permissions":{"permissionGrants":[{"allow":true,"action":"command","target":"something else"}]},`,
		`"completedInteractions":[{}],`,
	} {
		rawWithRecord := strings.Replace(raw, `"status":`, field+`"status":`, 1)
		shownWithRecord := strings.Replace(shown, `"status":`, field+`"status":`, 1)
		s := snapshot1216(t, rawWithRecord)
		require1216(t, s, rawWithRecord, shownWithRecord, Unknown)
		r, sourceProven := s.compareResponses([]byte(rawWithRecord), []byte(shownWithRecord))
		if r.State != Unknown || !sourceProven {
			t.Fatalf("unqualified permission record must be a source-proven refusal: %+v, %v", r, sourceProven)
		}
	}
}
