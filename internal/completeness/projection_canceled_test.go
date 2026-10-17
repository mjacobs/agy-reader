package completeness

import (
	"strings"
	"testing"
)

// These fixtures cover a run_command call that was canceled while its
// permission prompt was still open (agy-reader#3cxt). It was seen once, in the
// agy 1.3.1 audit of 2026-10-07: agy was stopped while the prompt for the
// parent's first command waited, hidden behind the first-run screens, in
// report alias conversation-114. The fixture keeps that step's structure field by
// field, from the frozen source and the default RPC response, with synthetic
// values. The wrapper has status CANCELED, the call's arguments, no result, no
// error, no grant and no interaction record; its metadata holds one pre-tool
// hook result {decision: allow} (no hook was configured) and a status history
// PENDING, RUNNING, WAITING, RUNNING, CANCELED. The default response replaces
// the wrapper with a RUN_COMMAND step built from the arguments, as for a
// rejected request (agy-reader#xyfe), and keeps the status and metadata.
//
// Every response is spelled out here, never derived from the projection.
const (
	canceledArgs        = `"CommandLine":"echo FIXTURE_CANCEL","Cwd":"/fixture/scratch","WaitMsBeforeAsync":"5000","toolAction":"Running fixture","toolSummary":"Run fixture"`
	canceledCall        = `"toolCall":{"id":"call-cancel","name":"run_command","argumentsJson":"{\"CommandLine\":\"echo FIXTURE_CANCEL\",\"Cwd\":\"/fixture/scratch\",\"WaitMsBeforeAsync\":5000,\"toolAction\":\"Running fixture\",\"toolSummary\":\"Run fixture\"}"}`
	canceledTransitions = `"internalMetadata":{"statusTransitions":[` +
		`{"updatedStatus":"CORTEX_STEP_STATUS_PENDING","timestamp":"2026-01-01T00:00:01.100Z"},` +
		`{"updatedStatus":"CORTEX_STEP_STATUS_RUNNING","timestamp":"2026-01-01T00:00:01.101Z"},` +
		`{"updatedStatus":"CORTEX_STEP_STATUS_WAITING","timestamp":"2026-01-01T00:00:01.102Z"},` +
		`{"updatedStatus":"CORTEX_STEP_STATUS_RUNNING","timestamp":"2026-01-01T00:00:41.900Z"},` +
		`{"updatedStatus":"CORTEX_STEP_STATUS_CANCELED","timestamp":"2026-01-01T00:00:41.901Z"}]}`
	canceledHooks   = `"preToolHookResults":[{"decision":"allow"}]`
	canceledMeta    = `"metadata":{"createdAt":"2026-01-01T00:00:01Z","viewableAt":"2026-01-01T00:00:01.050Z","finishedGeneratingAt":"2026-01-01T00:00:01.050Z","startedAt":"2026-01-01T00:00:01.101Z","completedAt":"2026-01-01T00:00:41.901Z","source":"CORTEX_STEP_SOURCE_MODEL",` + canceledCall + `,"generatorModel":"MODEL_PLACEHOLDER_M318","executionId":"fixture-execution","sourceTrajectoryStepInfo":{"trajectoryId":"fixture-trajectory","stepIndex":2,"cascadeId":"canceled-fixture"},` + canceledTransitions + `,` + canceledHooks + `,"toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN"}`
	canceledStatus  = `"status":"CORTEX_STEP_STATUS_CANCELED"`
	canceledRaw     = `{"cascadeId":"canceled-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC",` + canceledStatus + `,` + canceledMeta + `,"generic":{"args":{` + canceledArgs + `}}}]}`
	canceledPresent = `{"cascadeId":"canceled-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND",` + canceledStatus + `,` + canceledMeta + `,"runCommand":{"commandLine":"echo FIXTURE_CANCEL","proposedCommandLine":"echo FIXTURE_CANCEL","cwd":"/fixture/scratch","blocking":true,"combinedOutput":{}}}]}`
)

func TestProjectionCanceledPromptQualified(t *testing.T) {
	quoted := strings.NewReplacer(`FIXTURE_CANCEL`, `'two words' && echo \"quoted\"`, `\"echo FIXTURE_CANCEL\"`, `\"echo 'two words' && echo \\\"quoted\\\"\"`)
	for name, pair := range map[string][2]string{
		"canceled at the prompt":     {canceledRaw, canceledPresent},
		"command that needs quoting": {quoted.Replace(canceledRaw), quoted.Replace(canceledPresent)},
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, snapshot1216(t, pair[0]), pair[0], pair[1], Verified)
		})
	}
}

// A source mutation applies to the persisted trajectory and both RPC copies. A
// presentation or raw mutation changes only that response.
func TestProjectionCanceledPromptChanges(t *testing.T) {
	c := func(name, before, after, where string, want State) blockedChange {
		return blockedChange{name, canceledRaw, canceledPresent, before, after, where, want}
	}
	waiting := `{"updatedStatus":"CORTEX_STEP_STATUS_WAITING","timestamp":"2026-01-01T00:00:01.102Z"},`
	for _, tc := range []blockedChange{
		// Unknown extra fields, wherever they appear, fail.
		c("unknown argument", `"args":{`, `"args":{"Future":"new",`, "source", Unknown),
		c("argument of another tool", `"args":{`, `"args":{"AbsolutePath":"/fixture/x",`, "source", Unknown),
		c("argument missing from the tool call", `"args":{`, `"args":{"BypassSandbox":"false",`, "source", Unknown),
		c("argument that differs from the tool call", `"Cwd":"/fixture/scratch","WaitMsBeforeAsync"`, `"Cwd":"/fixture/other","WaitMsBeforeAsync"`, "source", Unknown),
		c("task details", `"generic":{`, `"taskDetails":{"id":"canceled-fixture/task-1"},"generic":{`, "source", Unknown),
		c("grant on the wrapper", `"generic":{`, `"permissions":{"permissionGrants":[{"action":"command","target":"echo FIXTURE_CANCEL"}]},"generic":{`, "source", Unknown),
		c("empty permissions on the wrapper", `"generic":{`, `"permissions":{},"generic":{`, "source", Unknown),
		c("pending request on the wrapper", `"generic":{`, `"requestedInteraction":{"permission":{"shellName":"bash"}},"generic":{`, "source", Unknown),
		c("completed request on the wrapper", `"generic":{`, `"completedInteractions":[{"request":`+rejectedRequest+`,"response":`+rejectedAnswer+`}],"generic":{`, "source", Unknown),
		c("outer error", `"generic":{`, `"error":{"shortError":"canceled","fullError":"canceled"},"generic":{`, "source", Unknown),
		// The hook result and the status history must be the observed ones.
		c("no hook result", `,`+canceledHooks, ``, "source", Unknown),
		c("hook denied", `{"decision":"allow"}`, `{"decision":"deny"}`, "source", Unknown),
		c("hook result with a reason", canceledHooks, `"preToolHookResults":[{"decision":"allow","reason":"fixture"}]`, "source", Unknown),
		c("two hook results", canceledHooks, `"preToolHookResults":[{"decision":"allow"},{"decision":"allow"}]`, "source", Unknown),
		c("no status history", canceledTransitions+`,`, ``, "source", Unknown),
		c("canceled without waiting for an answer", waiting, ``, "source", Unknown),
		c("canceled while waiting", `{"updatedStatus":"CORTEX_STEP_STATUS_RUNNING","timestamp":"2026-01-01T00:00:41.900Z"},`, ``, "source", Unknown),
		c("history that ends otherwise", `"CORTEX_STEP_STATUS_CANCELED","timestamp"`, `"CORTEX_STEP_STATUS_ERROR","timestamp"`, "source", Unknown),
		// Another status, or another tool, keeps failing.
		c("status error without an error", canceledStatus, `"status":"CORTEX_STEP_STATUS_ERROR"`, "source", Unknown),
		c("status done", canceledStatus, `"status":"CORTEX_STEP_STATUS_DONE"`, "source", Unknown),
		c("status running", canceledStatus, `"status":"CORTEX_STEP_STATUS_RUNNING"`, "source", Unknown),
		c("canceled view_file", `"name":"run_command"`, `"name":"view_file"`, "source", Unknown),
		c("canceled list_dir", `"name":"run_command"`, `"name":"list_dir"`, "source", Unknown),
		// A response that differs from the source is incomplete.
		c("presented status changed", canceledStatus, `"status":"CORTEX_STEP_STATUS_ERROR"`, "presentation", Incomplete),
		c("presented with an error", `,"runCommand"`, `,"error":{"shortError":"canceled"},"runCommand"`, "presentation", Incomplete),
		c("presented with output", `"combinedOutput":{}`, `"combinedOutput":{"full":"FIXTURE_CANCEL\n"}`, "presentation", Incomplete),
		c("presented with an exit code", `"blocking":true,`, `"blocking":true,"exitCode":0,`, "presentation", Incomplete),
		c("presented not blocking", `"blocking":true,`, ``, "presentation", Incomplete),
		c("presented sandbox override", `"blocking":true,`, `"blocking":true,"sandboxOverride":true,`, "presentation", Incomplete),
		c("presented command changed", `"commandLine":"echo FIXTURE_CANCEL"`, `"commandLine":"echo OTHER"`, "presentation", Incomplete),
		c("presented hook result dropped", `,`+canceledHooks, ``, "presentation", Incomplete),
		c("presented status history dropped", canceledTransitions+`,`, ``, "presentation", Incomplete),
		c("raw argument changed", `"WaitMsBeforeAsync":"5000"`, `"WaitMsBeforeAsync":"6000"`, "raw", Incomplete),
		{"presented as the unchanged wrapper", canceledRaw, canceledRaw, "", "", "presentation", Incomplete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, presentation := tc.source, tc.presentation
			s := snapshot1216(t, raw)
			switch tc.where {
			case "source":
				raw = replace1216(t, raw, tc.before, tc.after)
				presentation = strings.ReplaceAll(presentation, tc.before, tc.after)
				s = snapshot1216(t, raw)
			case "raw":
				raw = replace1216(t, raw, tc.before, tc.after)
			case "presentation":
				if tc.before != "" {
					presentation = replace1216(t, presentation, tc.before, tc.after)
				}
			}
			require1216(t, s, raw, presentation, tc.want)
		})
	}
}

// The rejected headless child also passed BypassSandbox "false"
// (agy-reader#xyfe). It was not seen on a canceled prompt, so a canceled call
// that names it in both the arguments and the tool call still fails.
func TestProjectionCanceledPromptSandboxArgument(t *testing.T) {
	r := strings.NewReplacer(
		`"CommandLine":"echo FIXTURE_CANCEL","Cwd"`, `"BypassSandbox":"false","CommandLine":"echo FIXTURE_CANCEL","Cwd"`,
		`{\"CommandLine\":`, `{\"BypassSandbox\":false,\"CommandLine\":`,
	)
	raw, presentation := r.Replace(canceledRaw), r.Replace(canceledPresent)
	if raw == canceledRaw || presentation == canceledPresent {
		t.Fatal("sandbox fixture unchanged")
	}
	require1216(t, snapshot1216(t, raw), raw, presentation, Unknown)
}
