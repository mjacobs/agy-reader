package completeness

import (
	"strings"
	"testing"
)

// These fixtures reproduce the structure of payload-free schedule,
// manage_subagents and define_subagent steps found in retained sessions
// (agy-reader#hty6). Frozen source, full RPC and default RPC copies of those
// sessions showed that the default response presents each of these steps
// unchanged, so every fixture's expected presentation is its own raw copy.
// Values are synthetic.
const payloadFreeScheduleRaw = `{"cascadeId":"schedule-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-timer","name":"schedule","argumentsJson":"{\"DurationSeconds\":30,\"Prompt\":\"Check the build\",\"TimerCondition\":\"always\",\"toolAction\":\"Setting timer\",\"toolSummary\":\"Build check\"}"},"completedAt":"2026-01-01T00:00:01Z"},"taskDetails":{"id":"schedule-fixture/timer-1","logUri":"file:///fixture/tasks/timer-1.log","description":"Check the build"},"generic":{"args":{"DurationSeconds":"30","Prompt":"Check the build","TimerCondition":"always","toolAction":"Setting timer","toolSummary":"Build check"},"result":{"result":"Timer set for 30 seconds","fullOutputUri":"file:///fixture/steps/1/output.txt","stepRenderInfo":{"title":"30s","titlePrefix":"Timed","markdown":"Check the build","icon":"timer","aggregationKey":"timers"}}}}]}`

// A schedule call that fails keeps an outer error on its payload-free wrapper,
// and the default response presents it unchanged, error included.
const payloadFreeScheduleErrorRaw = `{"cascadeId":"schedule-error-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR","metadata":{"toolCall":{"id":"call-timer","name":"schedule","argumentsJson":"{\"DurationSeconds\":30,\"Prompt\":\"Check the build\",\"TimerCondition\":\"always\"}"},"completedAt":"2026-01-01T00:00:01Z"},"error":{"shortError":"timer limit reached","fullError":"timer limit reached: 4 timers active"},"generic":{"args":{"DurationSeconds":"30","Prompt":"Check the build","TimerCondition":"always","toolAction":"Setting timer","toolSummary":"Build check"},"result":{"result":"Encountered error in step execution: timer limit reached","fullOutputUri":"file:///fixture/steps/1/output.txt"}}}]}`

const payloadFreeListSubagentsRaw = `{"cascadeId":"subagents-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-list","name":"manage_subagents","argumentsJson":"{\"Action\":\"list\",\"toolAction\":\"Listing subagents\",\"toolSummary\":\"Subagents\"}"},"completedAt":"2026-01-01T00:00:01Z","toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN"},"generic":{"args":{"Action":"list","toolAction":"Listing subagents","toolSummary":"Subagents"},"result":{"result":"No subagents are running","fullOutputUri":"file:///fixture/steps/1/output.txt","stepRenderInfo":{"title":"0 subagents","titlePrefix":"Found","icon":"robot_2","aggregationKey":"subagents"}}}}]}`

const payloadFreeKillSubagentsRaw = `{"cascadeId":"subagents-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-kill","name":"manage_subagents","argumentsJson":"{\"Action\":\"kill_all\",\"toolAction\":\"Stopping subagents\",\"toolSummary\":\"Stop all\"}"},"completedAt":"2026-01-01T00:00:01Z"},"generic":{"args":{"Action":"kill_all","toolAction":"Stopping subagents","toolSummary":"Stop all"},"result":{"result":"Killed 2 subagents","fullOutputUri":"file:///fixture/steps/1/output.txt","stepRenderInfo":{"title":"2 subagents","titlePrefix":"Killed","markdown":"child-fixture-1, child-fixture-2","icon":"robot_2","aggregationKey":"subagents"},"metadata":{"killed_conversation_ids":"child-fixture-1,child-fixture-2"}}}}]}`

const payloadFreeDefineFieldsRaw = `{"cascadeId":"define-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-define","name":"define_subagent","argumentsJson":"{\"description\":\"Reviews fixtures\",\"enable_mcp_tools\":false,\"enable_subagent_tools\":false,\"enable_write_tools\":true,\"name\":\"fixture-reviewer\",\"system_prompt\":\"Review the fixture\",\"toolAction\":\"Defining reviewer\",\"toolSummary\":\"Reviewer\"}"},"completedAt":"2026-01-01T00:00:01Z"},"generic":{"args":{"description":"Reviews fixtures","enable_mcp_tools":"false","enable_subagent_tools":"false","enable_write_tools":"true","name":"fixture-reviewer","system_prompt":"Review the fixture","toolAction":"Defining reviewer","toolSummary":"Reviewer"},"result":{"result":"Defined subagent fixture-reviewer","fullOutputUri":"file:///fixture/steps/1/output.txt","stepRenderInfo":{"title":"fixture-reviewer","titlePrefix":"Defined","markdown":"Reviews fixtures","icon":"robot_2","aggregationKey":"subagents"}}}}]}`

// An older define_subagent shape stores one definition argument and no output
// URI, while its tool call still lists the separate fields.
const payloadFreeDefinitionRaw = `{"cascadeId":"define-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-define","name":"define_subagent","argumentsJson":"{\"description\":\"Reviews fixtures\",\"enable_mcp_tools\":false,\"enable_subagent_tools\":false,\"enable_write_tools\":true,\"name\":\"fixture-reviewer\",\"system_prompt\":\"Review the fixture\",\"toolAction\":\"Defining reviewer\",\"toolSummary\":\"Reviewer\"}"},"completedAt":"2026-01-01T00:00:01Z"},"generic":{"args":{"definition":"name: fixture-reviewer"},"result":{"result":"Defined subagent fixture-reviewer","stepRenderInfo":{"title":"fixture-reviewer","titlePrefix":"Defined","markdown":"Reviews fixtures","icon":"robot_2","aggregationKey":"subagents"}}}}]}`

var payloadFreeFixtures = map[string]string{
	"schedule":                 payloadFreeScheduleRaw,
	"failed schedule":          payloadFreeScheduleErrorRaw,
	"list subagents":           payloadFreeListSubagentsRaw,
	"kill subagents":           payloadFreeKillSubagentsRaw,
	"define subagent fields":   payloadFreeDefineFieldsRaw,
	"define subagent combined": payloadFreeDefinitionRaw,
}

func TestProjectionPayloadFreeQualifiedShapes(t *testing.T) {
	for name, raw := range payloadFreeFixtures {
		t.Run(name, func(t *testing.T) {
			require1216(t, snapshot1216(t, raw), raw, raw, Verified)
		})
	}
}

// A changed value or status in either response is incomplete: the daemon
// presents these steps unchanged, and full raw equality runs first.
func TestProjectionPayloadFreeResponseChanges(t *testing.T) {
	for name, raw := range payloadFreeFixtures {
		t.Run(name, func(t *testing.T) {
			s := snapshot1216(t, raw)
			status := `"status":"CORTEX_STEP_STATUS_DONE"`
			if !strings.Contains(raw, status) {
				status = `"status":"CORTEX_STEP_STATUS_ERROR"`
			}
			for field, change := range map[string][2]string{
				"status":        {status, `"status":"CORTEX_STEP_STATUS_CANCELED"`},
				"tool identity": {`"name":"`, `"name":"unknown_`},
				"argument name": {`"args":{"`, `"args":{"stale`},
				"result text":   {`"result":{"result":"`, `"result":{"result":"stale `},
				"added media":   {`"result":{"result":`, `"result":{"media":[{"mimeType":"image/png","uri":"file:///fixture/extra.png"}],"result":`},
				"call time":     {`2026-01-01T00:00:01Z`, `2026-01-01T00:00:09Z`},
			} {
				t.Run(field, func(t *testing.T) {
					changed := replace1216(t, raw, change[0], change[1])
					require1216(t, s, raw, changed, Incomplete)
					require1216(t, s, changed, raw, Incomplete)
				})
			}
		})
	}
	for _, tc := range []struct{ name, raw, before, after string }{
		{"schedule duration", payloadFreeScheduleRaw, `"DurationSeconds":"30"`, `"DurationSeconds":"60"`},
		{"schedule task details", payloadFreeScheduleRaw, `"id":"schedule-fixture/timer-1"`, `"id":"schedule-fixture/timer-2"`},
		{"schedule error text", payloadFreeScheduleErrorRaw, `"shortError":"timer limit reached"`, `"shortError":"timer rejected"`},
		{"schedule error dropped", payloadFreeScheduleErrorRaw, `"error":{"shortError":"timer limit reached","fullError":"timer limit reached: 4 timers active"},`, ``},
		{"schedule status", payloadFreeScheduleErrorRaw, `"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_DONE"`},
		{"subagents action", payloadFreeListSubagentsRaw, `"Action":"list"`, `"Action":"kill_all"`},
		{"subagents status", payloadFreeListSubagentsRaw, `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_ERROR"`},
		{"killed conversations", payloadFreeKillSubagentsRaw, `"killed_conversation_ids":"child-fixture-1,child-fixture-2"`, `"killed_conversation_ids":"child-fixture-1"`},
		{"define write tools", payloadFreeDefineFieldsRaw, `"enable_write_tools":"true"`, `"enable_write_tools":"false"`},
		{"define definition", payloadFreeDefinitionRaw, `"definition":"name: fixture-reviewer"`, `"definition":"name: other-reviewer"`},
		{"define render", payloadFreeDefinitionRaw, `"titlePrefix":"Defined"`, `"titlePrefix":"Updated"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := snapshot1216(t, tc.raw)
			changed := replace1216(t, tc.raw, tc.before, tc.after)
			require1216(t, s, tc.raw, changed, Incomplete)
			require1216(t, s, changed, tc.raw, Incomplete)
		})
	}
}

// A source mutation applies to the persisted trajectory and both RPC copies, so
// only the projection rules can refuse it.
func TestProjectionPayloadFreeUnsupportedShapes(t *testing.T) {
	for _, tc := range []struct{ name, source, before, after string }{
		// Unlisted payload-free tools still fail.
		{"unknown payload-free tool", payloadFreeScheduleRaw, `"name":"schedule"`, `"name":"future_tool"`},
		{"known tool without a payload-free rule", payloadFreeListSubagentsRaw, `"name":"manage_subagents"`, `"name":"list_dir"`},
		{"missing tool identity", payloadFreeDefineFieldsRaw, `"name":"define_subagent",`, ``},
		// Arguments.
		{"unknown schedule argument", payloadFreeScheduleRaw, `"args":{`, `"args":{"Future":"new",`},
		{"unrelated known schedule argument", payloadFreeScheduleRaw, `"args":{`, `"args":{"Action":"list",`},
		{"unknown subagents argument", payloadFreeListSubagentsRaw, `"args":{`, `"args":{"Future":"new",`},
		{"unrelated known subagents argument", payloadFreeListSubagentsRaw, `"args":{`, `"args":{"TaskId":"task-1",`},
		{"unknown define argument", payloadFreeDefineFieldsRaw, `"args":{`, `"args":{"Future":"new",`},
		{"unknown definition argument", payloadFreeDefinitionRaw, `"args":{`, `"args":{"Future":"new",`},
		{"mixed define argument shapes", payloadFreeDefinitionRaw, `"args":{`, `"args":{"name":"fixture-reviewer",`},
		// Result fields and metadata.
		{"schedule result metadata", payloadFreeScheduleRaw, `"result":{"result":`, `"result":{"metadata":{"timer_id":"timer-1"},"result":`},
		{"define result metadata", payloadFreeDefineFieldsRaw, `"result":{"result":`, `"result":{"metadata":{"name":"fixture-reviewer"},"result":`},
		{"unknown subagents metadata", payloadFreeKillSubagentsRaw, `"killed_conversation_ids":`, `"future_metadata":`},
		{"extra subagents metadata", payloadFreeKillSubagentsRaw, `"metadata":{"killed_conversation_ids"`, `"metadata":{"future_metadata":"x","killed_conversation_ids"`},
		{"schedule result media", payloadFreeScheduleRaw, `"result":{"result":`, `"result":{"media":[{"mimeType":"image/png","uri":"file:///fixture/extra.png"}],"result":`},
		// Outer fields.
		{"subagents task details", payloadFreeListSubagentsRaw, `"generic":{`, `"taskDetails":{"id":"task-1"},"generic":{`},
		{"define task details", payloadFreeDefineFieldsRaw, `"generic":{`, `"taskDetails":{"id":"task-1"},"generic":{`},
		// Statuses and outer errors.
		{"schedule canceled", payloadFreeScheduleRaw, `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_CANCELED"`},
		{"schedule error status without error", payloadFreeScheduleRaw, `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_ERROR"`},
		{"schedule error on a step that did not fail", payloadFreeScheduleErrorRaw, `"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_DONE"`},
		{"schedule error with extra field", payloadFreeScheduleErrorRaw, `"error":{`, `"error":{"errorId":"error-1",`},
		{"subagents running", payloadFreeListSubagentsRaw, `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_RUNNING"`},
		{"subagents error", payloadFreeListSubagentsRaw, `"status":"CORTEX_STEP_STATUS_DONE","metadata"`, `"status":"CORTEX_STEP_STATUS_ERROR","error":{"shortError":"failed"},"metadata"`},
		{"define error", payloadFreeDefineFieldsRaw, `"status":"CORTEX_STEP_STATUS_DONE","metadata"`, `"status":"CORTEX_STEP_STATUS_ERROR","error":{"shortError":"failed"},"metadata"`},
		{"define canceled", payloadFreeDefinitionRaw, `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_CANCELED"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := replace1216(t, tc.source, tc.before, tc.after)
			require1216(t, snapshot1216(t, raw), raw, raw, Unknown)
		})
	}
}
