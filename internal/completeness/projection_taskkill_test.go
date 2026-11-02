package completeness

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// A command that agy moved to the background and the agent then ended with
// manage_task kill. The synthetic fixtures follow a native agy 1.4.12 capture
// with WaitMsBeforeAsync 5000 (source, full RPC and default RPC from a fresh
// daemon): a canceled run_command with task details, a completed kill whose
// payload is ManageTask, and the task_notification system message that agy
// appends. Expected presentations are written out independently of the
// projection. No private content is used.
const (
	taskKill1412Run       = `{"type":"CORTEX_STEP_TYPE_GENERIC",` + background144Common + `,"generic":{"args":{` + background144Args + `},"result":{"stepRenderInfo":{"titlePrefix":"Canceled","icon":"terminal","aggregationKey":"terminal","titleParts":[{"text":"wait for fixture"},{"badge":{"icon":"shield","tooltip":"Ran in the sandbox"}}],"contentBlocks":[{"terminal":{"command":"sleep 30","cwd":"/fixture","output":"Waiting"}}]}}}}`
	taskKill1412RunShown  = `{"type":"CORTEX_STEP_TYPE_RUN_COMMAND",` + background144Common + `,"runCommand":{"commandLine":"sleep 30","proposedCommandLine":"sleep 30","combinedOutput":{}}}`
	taskKill1412Call      = `"toolCall":{"id":"call-kill","name":"manage_task","argumentsJson":"{\"Action\":\"kill\",\"TaskId\":\"fixture/task-1\"}"}`
	taskKill1412Meta      = `"metadata":{` + taskKill1412Call + `,"preToolHookResults":[{"decision":"allow"}],"toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN","completedAt":"2026-01-01T00:00:02Z"}`
	taskKill1412Payload   = `"payload":{"@type":"type.googleapis.com/exa.cortex.core.tools.ManageTask","action":"kill","taskIds":["fixture/task-1"]}`
	taskKill1412Result    = `"result":"Task \"fixture/task-1\" cancelled.","fullOutputUri":"file:///fixture/steps/1/output.txt",` + taskKill1412Payload + `,"stepRenderInfo":{"titlePrefix":"Killed task","icon":"task_alt","aggregationKey":"tasks","titleParts":[{"text":"Wait"}]}`
	taskKill1412Kill      = `{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE",` + taskKill1412Meta + `,"generic":{"args":{"Action":"kill","TaskId":"fixture/task-1"},"result":{` + taskKill1412Result + `}}}`
	taskKill1412KillShown = `{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE",` + taskKill1412Meta + `,"generic":{"args":{"Action":"kill","TaskId":"fixture/task-1"},"result":{` + taskKill1412Result + `,"metadata":{"action":"kill","task_id":"fixture/task-1","task_ids":"fixture/task-1"}}}}`
	taskKill1412Notice    = `{"type":"CORTEX_STEP_TYPE_SYSTEM_MESSAGE","status":"CORTEX_STEP_STATUS_DONE","metadata":{"source":"CORTEX_STEP_SOURCE_SYSTEM","isInterruptingStep":true},"systemMessage":{"message":"[Message] timestamp=2026-01-01T00:00:03Z sender=fixture/task-1 priority=MESSAGE_PRIORITY_LOW content=Task id \"fixture/task-1\" was canceled","renderInfo":{"title":"Wait was canceled","hidden":true,"contentBlocks":[{"terminal":{"command":"sleep 30","cwd":"/fixture","output":"Waiting"}}]},"eventType":"task_notification","agentMessage":{"id":"fixture-message","recipient":"fixture-parent","sender":"fixture/task-1","priority":"MESSAGE_PRIORITY_LOW","timestamp":"2026-01-01T00:00:03Z","renderDetails":{"messageTitle":"Wait was canceled"},"hideFromUser":true,"content":"Task id \"fixture/task-1\" was canceled with result:\nTool execution was canceled","sourceMetadata":{"tool":{"conversationId":"fixture-parent","stepIndex":0,"toolCall":{"id":"call-run","name":"run_command","argumentsJson":"{\"CommandLine\":\"sleep 30\",\"WaitMsBeforeAsync\":5000}"}}}}}}`
	taskKill1412Raw       = `{"cascadeId":"fixture-parent","steps":[` + taskKill1412Run + `,` + taskKill1412Kill + `,` + taskKill1412Notice + `]}`
	taskKill1412Shown     = `{"cascadeId":"fixture-parent","steps":[` + taskKill1412RunShown + `,` + taskKill1412KillShown + `,` + taskKill1412Notice + `]}`
)

func TestProjectionTaskKill1412Verifies(t *testing.T) {
	require1216(t, snapshot1216(t, taskKill1412Raw), taskKill1412Raw, taskKill1412Shown, Verified)
}

// Each mutation of the source makes the sequence unsupported (the reader
// cannot vouch for it); the same mutation of the default response is a
// mismatch with a supported source. The task_notification system message is
// presented unchanged, so any change on either side is a mismatch.
func TestProjectionTaskKill1412Refusals(t *testing.T) {
	for _, tc := range []struct {
		name, before, after string
		source              State
	}{
		{"changed status: command done", `"status":"CORTEX_STEP_STATUS_CANCELED"`, `"status":"CORTEX_STEP_STATUS_DONE"`, Unknown},
		{"changed status: kill errored", `"status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-kill"`, `"status":"CORTEX_STEP_STATUS_ERROR","metadata":{"toolCall":{"id":"call-kill"`, Unknown},
		{"changed status: kill running", `"status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-kill"`, `"status":"CORTEX_STEP_STATUS_RUNNING","metadata":{"toolCall":{"id":"call-kill"`, Unknown},
		{"changed status: notice errored", `"type":"CORTEX_STEP_TYPE_SYSTEM_MESSAGE","status":"CORTEX_STEP_STATUS_DONE"`, `"type":"CORTEX_STEP_TYPE_SYSTEM_MESSAGE","status":"CORTEX_STEP_STATUS_ERROR"`, Incomplete},
		{"changed error: command failed", `"generic":{"args":{"CommandLine"`, `"error":{"shortError":"failed"},"generic":{"args":{"CommandLine"`, Unknown},
		{"changed error: kill failed", `"generic":{"args":{"Action":"kill"`, `"error":{"shortError":"failed"},"generic":{"args":{"Action":"kill"`, Unknown},
		{"changed error: notice failed", `"systemMessage":{`, `"error":{"shortError":"failed"},"systemMessage":{`, Incomplete},
		{"extra argument: command", `"args":{"CommandLine"`, `"args":{"Future":"x","CommandLine"`, Unknown},
		{"extra argument: kill", `"args":{"Action":"kill",`, `"args":{"Future":"x","Action":"kill",`, Unknown},
		{"wait other than the captured 5000", `"WaitMsBeforeAsync":"5000"`, `"WaitMsBeforeAsync":"3000"`, Unknown},
		{"kill with another action", `"action":"kill"`, `"action":"future"`, Unknown},
		{"kill naming two tasks", `"taskIds":["fixture/task-1"]`, `"taskIds":["fixture/task-1","fixture/task-2"]`, Unknown},
		{"notice text", `was canceled with result`, `was finished with result`, Incomplete},
		{"notice event type", `"eventType":"task_notification"`, `"eventType":"other_notification"`, Incomplete},
		{"notice render title", `"title":"Wait was canceled"`, `"title":"Wait finished"`, Incomplete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := replace1216(t, taskKill1412Raw, tc.before, tc.after)
			require1216(t, snapshot1216(t, bad), bad, taskKill1412Shown, tc.source)
			if strings.Contains(taskKill1412Shown, tc.before) {
				shown := replace1216(t, taskKill1412Shown, tc.before, tc.after)
				require1216(t, snapshot1216(t, taskKill1412Raw), taskKill1412Raw, shown, Incomplete)
			}
		})
	}
}

// A persisted field the descriptors do not know is rejected on each step.
func TestProjectionTaskKill1412UnknownWireFields(t *testing.T) {
	extra := protowire.AppendVarint(protowire.AppendTag(nil, 9999, protowire.VarintType), 1)
	for name, mutate := range map[string]func(step protoreflect.Message){
		"command result": func(step protoreflect.Message) {
			tool143Get(tool143Get(step, "generic").Message(), "result").Message().SetUnknown(extra)
		},
		"command task details": func(step protoreflect.Message) { tool143Get(step, "task_details").Message().SetUnknown(extra) },
		"kill payload": func(step protoreflect.Message) {
			payload := tool143Get(tool143Get(tool143Get(step, "generic").Message(), "result").Message(), "payload").Message()
			data := append(append([]byte(nil), tool143Get(payload, "value").Bytes()...), extra...)
			tool143Set(payload, "value", protoreflect.ValueOfBytes(data))
		},
		"notice message": func(step protoreflect.Message) { tool143Get(step, "system_message").Message().SetUnknown(extra) },
		"notice agent message": func(step protoreflect.Message) {
			tool143Get(tool143Get(step, "system_message").Message(), "agent_message").Message().SetUnknown(extra)
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := trajectory1216(t, taskKill1412Raw).ProtoReflect()
			steps := tool143Get(m, "steps").List()
			index := 0
			switch {
			case strings.HasPrefix(name, "kill"):
				index = 1
			case strings.HasPrefix(name, "notice"):
				index = 2
			}
			mutate(steps.Get(index).Message())
			if _, err := finishSnapshot(m.Interface()); err == nil {
				t.Fatal("unknown wire field accepted")
			}
		})
	}
}
