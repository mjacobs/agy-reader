package completeness

import (
	"strings"
	"testing"
)

// These fixtures reproduce the structure of two steps that agy 1.2.16 wrote in
// ordinary sessions (agy-reader#t2x5): a view_file call with a ContentOffset
// argument, and a run_command canceled while running. Frozen source, full RPC
// and default RPC copies of those sessions showed that the default response
// presents each embedded native Step: verbatim for view_file, and with the outer
// CANCELED status instead of the inner ERROR for run_command. Values are
// synthetic; the expected presentations are written out explicitly.
const currentViewArguments = `{\"AbsolutePath\":\"/fixture/notes.md\",\"ContentOffset\":0,\"EndLine\":120,\"StartLine\":1,\"toolAction\":\"Viewing notes\",\"toolSummary\":\"View notes\"}`
const currentViewNative = `"type":"CORTEX_STEP_TYPE_VIEW_FILE","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-view","name":"view_file","argumentsJson":"` + currentViewArguments + `"},"completedAt":"2026-01-01T00:00:01Z"},"viewFile":{"absolutePathUri":"file:///fixture/notes.md","endLine":2,"content":"one\ntwo\n","lineRangeBytes":8,"numLines":3,"numBytes":8}`
const currentViewRaw = `{"cascadeId":"view-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-view","name":"view_file","argumentsJson":"` + currentViewArguments + `"},"completedAt":"2026-01-01T00:00:02Z"},"generic":{"args":{"AbsolutePath":"/fixture/notes.md","ContentOffset":"0","EndLine":"120","StartLine":"1","toolAction":"Viewing notes","toolSummary":"View notes"},"result":{"result":"File Path: file:///fixture/notes.md\none\ntwo\n","payload":{"@type":"type.googleapis.com/gemini_coder.Step",` + currentViewNative + `}}}}]}`
const currentViewPresentation = `{"cascadeId":"view-fixture","metadata":{},"steps":[{` + currentViewNative + `}]}`

const currentCanceledMetadata = `"metadata":{"toolCall":{"id":"call-run","name":"run_command","argumentsJson":"{\"CommandLine\":\"grep -rn fixture /fixture\",\"Cwd\":\"/fixture\",\"WaitMsBeforeAsync\":5000}"},"completedAt":"2026-01-01T00:00:01Z"}`
const currentCanceledTask = `"taskDetails":{"id":"canceled-fixture/task-1","logUri":"file:///fixture/tasks/task-1.log","description":"grep -rn fixture /fixture","title":"Search fixture"}`
const currentCanceledCommand = `"error":{"shortError":"context canceled"},"runCommand":{"commandLine":"grep -rn fixture /fixture","proposedCommandLine":"grep -rn fixture /fixture","cwd":"/fixture","waitMsBeforeAsync":"5000","shellName":"bash","ranInSandbox":true}`
const currentCanceledRaw = `{"cascadeId":"canceled-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_CANCELED","metadata":{"toolCall":{"id":"call-run","name":"run_command"},"completedAt":"2026-01-01T00:00:02Z"},` + currentCanceledTask + `,"generic":{"args":{"CommandLine":"grep -rn fixture /fixture","Cwd":"/fixture","WaitMsBeforeAsync":"5000","toolAction":"Searching","toolSummary":"Search fixture"},"result":{"result":"\nEncountered error in step execution: context canceled\n","fullOutputUri":"file:///fixture/steps/1/output.txt","payload":{"@type":"type.googleapis.com/gemini_coder.Step","type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_ERROR",` + currentCanceledMetadata + `,` + currentCanceledCommand + `}}}}]}`
const currentCanceledPresentation = `{"cascadeId":"canceled-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_CANCELED",` + currentCanceledMetadata + `,` + currentCanceledTask + `,` + currentCanceledCommand + `}]}`

func TestProjectionCurrentQualifiedShapes(t *testing.T) {
	for name, pair := range map[string][2]string{
		"view file content offset": {currentViewRaw, currentViewPresentation},
		"canceled run command":     {currentCanceledRaw, currentCanceledPresentation},
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, snapshot1216(t, pair[0]), pair[0], pair[1], Verified)
		})
	}
	t.Run("view file without content offset", func(t *testing.T) {
		raw := replace1216(t, currentViewRaw, `"ContentOffset":"0",`, ``)
		require1216(t, snapshot1216(t, raw), raw, currentViewPresentation, Verified)
	})
}

// A source mutation applies to the persisted trajectory and both RPC copies; a
// presentation or raw mutation changes only that response.
func TestProjectionCurrentChanges(t *testing.T) {
	for _, tc := range []struct {
		name, source, presentation, before, after string
		where                                     string
		want                                      State
	}{
		// view_file
		{"unknown extra argument", currentViewRaw, currentViewPresentation, `"args":{`, `"args":{"Future":"new",`, "source", Unknown},
		{"unrelated known argument", currentViewRaw, currentViewPresentation, `"args":{`, `"args":{"CommandLine":"echo unsafe",`, "source", Unknown},
		{"changed offset argument", currentViewRaw, currentViewPresentation, `"ContentOffset":"0"`, `"ContentOffset":"7"`, "source", Unknown},
		{"non-canonical offset argument", currentViewRaw, currentViewPresentation, `"ContentOffset":"0"`, `"ContentOffset":"00"`, "source", Unknown},
		{"negative offset argument", currentViewRaw, currentViewPresentation, `"ContentOffset":"0"`, `"ContentOffset":"-1"`, "source", Unknown},
		{"offset missing from presented call", currentViewRaw, currentViewPresentation, `\"ContentOffset\":0,`, ``, "source", Unknown},
		{"changed offset in presented call", currentViewRaw, currentViewPresentation, `\"ContentOffset\":0,`, `\"ContentOffset\":7,`, "source", Unknown},
		{"changed native offset", currentViewRaw, currentViewPresentation, `"viewFile":{`, `"viewFile":{"contentOffset":7,`, "source", Unknown},
		{"presented offset changed", currentViewRaw, currentViewPresentation, `\"ContentOffset\":0,`, `\"ContentOffset\":7,`, "presentation", Incomplete},
		{"presented native offset", currentViewRaw, currentViewPresentation, `"viewFile":{`, `"viewFile":{"contentOffset":7,`, "presentation", Incomplete},
		{"raw offset argument changed", currentViewRaw, currentViewPresentation, `"ContentOffset":"0"`, `"ContentOffset":"7"`, "raw", Incomplete},
		// run_command
		{"canceled outer with inner done", currentCanceledRaw, currentCanceledPresentation, `"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_DONE"`, "source", Unknown},
		{"canceled outer with inner running", currentCanceledRaw, currentCanceledPresentation, `"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_RUNNING"`, "source", Unknown},
		{"canceled outer with inner interrupted", currentCanceledRaw, currentCanceledPresentation, `"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_INTERRUPTED"`, "source", Unknown},
		{"inner error without details", currentCanceledRaw, currentCanceledPresentation, `"error":{"shortError":"context canceled"},`, ``, "source", Unknown},
		{"other outer status over inner error", currentCanceledRaw, currentCanceledPresentation, `"status":"CORTEX_STEP_STATUS_CANCELED"`, `"status":"CORTEX_STEP_STATUS_DONE"`, "source", Unknown},
		{"unknown command argument", currentCanceledRaw, currentCanceledPresentation, `"args":{`, `"args":{"Future":"new",`, "source", Unknown},
		{"raw inner status changed", currentCanceledRaw, currentCanceledPresentation, `"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_DONE"`, "raw", Incomplete},
		{"raw inner error changed", currentCanceledRaw, currentCanceledPresentation, `context canceled"}`, `deadline exceeded"}`, "raw", Incomplete},
		{"presented inner status", currentCanceledRaw, currentCanceledPresentation, `"status":"CORTEX_STEP_STATUS_CANCELED"`, `"status":"CORTEX_STEP_STATUS_ERROR"`, "presentation", Incomplete},
		{"presented error changed", currentCanceledRaw, currentCanceledPresentation, `context canceled"}`, `deadline exceeded"}`, "presentation", Incomplete},
		{"presented task dropped", currentCanceledRaw, currentCanceledPresentation, currentCanceledTask + `,`, ``, "presentation", Incomplete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, presentation := tc.source, tc.presentation
			s := snapshot1216(t, raw)
			switch tc.where {
			case "source":
				raw = replace1216(t, raw, tc.before, tc.after)
				// The presentation follows only where it carries the same content.
				presentation = strings.ReplaceAll(presentation, tc.before, tc.after)
				s = snapshot1216(t, raw)
			case "raw":
				raw = replace1216(t, raw, tc.before, tc.after)
			case "presentation":
				presentation = replace1216(t, presentation, tc.before, tc.after)
			}
			require1216(t, s, raw, presentation, tc.want)
		})
	}
}

// A manage_task call that fails (here, killing a task that already finished)
// keeps an outer error on its payload-free wrapper. The default response
// presents the step unchanged, including the error.
const currentTaskErrorRaw = `{"cascadeId":"task-error-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR","metadata":{"toolCall":{"id":"call-kill","name":"manage_task","argumentsJson":"{\"Action\":\"kill\",\"TaskId\":\"task-error-fixture/task-1\"}"},"completedAt":"2026-01-01T00:00:01Z"},"error":{"shortError":"cannot kill task: task is not running (status: DONE)","fullError":"cannot kill task: task is not running (status: DONE)"},"generic":{"args":{"Action":"kill","TaskId":"task-error-fixture/task-1","toolAction":"Killing task","toolSummary":"Cancel task-1"},"result":{"result":"Encountered error in step execution: cannot kill task: task is not running (status: DONE)","fullOutputUri":"file:///fixture/steps/1/output.txt"}}}]}`

func TestProjectionCurrentTaskError(t *testing.T) {
	require1216(t, snapshot1216(t, currentTaskErrorRaw), currentTaskErrorRaw, currentTaskErrorRaw, Verified)
	for _, tc := range []struct {
		name, source, before, after, where string
		want                               State
	}{
		{"presented error changed", currentTaskErrorRaw, `"shortError":"cannot kill`, `"shortError":"could not kill`, "presentation", Incomplete},
		{"presented error dropped", currentTaskErrorRaw, `"error":{"shortError":"cannot kill task: task is not running (status: DONE)","fullError":"cannot kill task: task is not running (status: DONE)"},`, ``, "presentation", Incomplete},
		{"raw error changed", currentTaskErrorRaw, `"shortError":"cannot kill`, `"shortError":"could not kill`, "raw", Incomplete},
		{"error on a step that did not fail", currentTaskErrorRaw, `"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_DONE"`, "source", Unknown},
		{"error with unknown task argument", currentTaskErrorRaw, `"args":{`, `"args":{"Future":"new",`, "source", Unknown},
		{"error on another payload-free tool", syntheticSendRaw, `"status":"CORTEX_STEP_STATUS_DONE","metadata"`, `"status":"CORTEX_STEP_STATUS_ERROR","error":{"shortError":"failed"},"metadata"`, "source", Unknown},
		{"error on a native wrapper", currentCanceledRaw, `"generic":{`, `"error":{"shortError":"failed"},"generic":{`, "source", Unknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, presentation := tc.source, tc.source
			s := snapshot1216(t, raw)
			switch tc.where {
			case "source":
				raw = replace1216(t, raw, tc.before, tc.after)
				presentation, s = raw, snapshot1216(t, raw)
			case "raw":
				raw = replace1216(t, raw, tc.before, tc.after)
			case "presentation":
				presentation = replace1216(t, presentation, tc.before, tc.after)
			}
			require1216(t, s, raw, presentation, tc.want)
		})
	}
}
