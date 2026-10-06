package completeness

import (
	"strings"
	"testing"
)

// These fixtures reproduce the structure of a run_command step that agy 1.2.17
// left behind when it was stopped while a foreground command ran
// (agy-reader#vx0f): the wrapper is CANCELED, but the embedded native Step was
// never updated and is still RUNNING, with no error. Frozen source, full RPC and
// default RPC copies of two such sessions showed the default response presenting
// the native Step with the outer CANCELED status; in one session the wrapper
// also carried task details, which the presentation keeps, and in the other it
// did not. Values are synthetic; the expected presentations are written out
// explicitly.
const runningCommandMetadata = `"metadata":{"toolCall":{"id":"call-run","name":"run_command","argumentsJson":"{\"CommandLine\":\"sleep 300\",\"Cwd\":\"/fixture\",\"WaitMsBeforeAsync\":10000}"},"startedAt":"2026-01-01T00:00:01Z"}`
const runningCommandTask = `"taskDetails":{"id":"running-fixture/task-1","logUri":"file:///fixture/tasks/task-1.log","description":"sleep 300","title":"Long command"}`
const runningCommandBody = `"runCommand":{"commandLine":"sleep 300","proposedCommandLine":"sleep 300","cwd":"/fixture","waitMsBeforeAsync":"10000","combinedOutput":{"full":"started\r\n"},"combinedOutputSnapshot":{"full":"started\r\n"},"ranInSandbox":true}`

func runningCommandRaw(task string) string {
	if task != "" {
		task += `,`
	}
	return `{"cascadeId":"running-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_CANCELED","metadata":{"toolCall":{"id":"call-run","name":"run_command"},"completedAt":"2026-01-01T00:00:02Z"},` + task + `"generic":{"args":{"CommandLine":"sleep 300","Cwd":"/fixture","WaitMsBeforeAsync":"10000","toolAction":"Running","toolSummary":"Long command"},"result":{"result":"started\r\n","payload":{"@type":"type.googleapis.com/gemini_coder.Step","type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_RUNNING",` + runningCommandMetadata + `,` + runningCommandBody + `}}}}]}`
}

func runningCommandPresentation(task string) string {
	if task != "" {
		task += `,`
	}
	return `{"cascadeId":"running-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_CANCELED",` + runningCommandMetadata + `,` + task + runningCommandBody + `}]}`
}

var runningCommandShapes = map[string][2]string{
	"with task details":    {runningCommandRaw(runningCommandTask), runningCommandPresentation(runningCommandTask)},
	"without task details": {runningCommandRaw(""), runningCommandPresentation("")},
}

func TestProjectionRunningCommandQualified(t *testing.T) {
	for name, pair := range runningCommandShapes {
		t.Run(name, func(t *testing.T) {
			require1216(t, snapshot1216(t, pair[0]), pair[0], pair[1], Verified)
			if !strings.Contains(pair[0], `"status":"CORTEX_STEP_STATUS_RUNNING"`) || strings.Contains(pair[1], "RUNNING") {
				t.Fatal("fixture does not model a RUNNING native step under a CANCELED wrapper")
			}
		})
	}
}

// A source mutation applies to the persisted trajectory and both RPC copies; a
// presentation or raw mutation changes only that response.
func TestProjectionRunningCommandChanges(t *testing.T) {
	const inner = `"status":"CORTEX_STEP_STATUS_RUNNING"`
	const outer = `"status":"CORTEX_STEP_STATUS_CANCELED"`
	for _, tc := range []struct {
		name, before, after, where string
		want                       State
	}{
		// Unequal status pairs other than the observed one.
		{"canceled outer with inner done", inner, `"status":"CORTEX_STEP_STATUS_DONE"`, "source", Unknown},
		{"canceled outer with inner pending", inner, `"status":"CORTEX_STEP_STATUS_PENDING"`, "source", Unknown},
		{"canceled outer with inner interrupted", inner, `"status":"CORTEX_STEP_STATUS_INTERRUPTED"`, "source", Unknown},
		{"canceled outer with inner error but no details", inner, `"status":"CORTEX_STEP_STATUS_ERROR"`, "source", Unknown},
		{"done outer with inner running", outer, `"status":"CORTEX_STEP_STATUS_DONE"`, "source", Unknown},
		{"error outer with inner running", outer, `"status":"CORTEX_STEP_STATUS_ERROR"`, "source", Unknown},
		// Unexpected content under the qualified shape.
		{"unknown extra argument", `"args":{`, `"args":{"Future":"new",`, "source", Unknown},
		{"unrelated known argument", `"args":{`, `"args":{"AbsolutePath":"/fixture/other",`, "source", Unknown},
		{"new native type", `"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_RUNNING"`, `"type":"CORTEX_STEP_TYPE_LIST_DIRECTORY","status":"CORTEX_STEP_STATUS_RUNNING"`, "source", Unknown},
		// Changed values in the responses.
		{"raw inner status changed", inner, `"status":"CORTEX_STEP_STATUS_DONE"`, "raw", Incomplete},
		{"raw output changed", `"full":"started\r\n"`, `"full":"changed\r\n"`, "raw", Incomplete},
		{"presented status running", outer, inner, "presentation", Incomplete},
		{"presented status error", outer, `"status":"CORTEX_STEP_STATUS_ERROR"`, "presentation", Incomplete},
		{"presented output changed", `"full":"started\r\n"`, `"full":"changed\r\n"`, "presentation", Incomplete},
		{"presented command changed", `"commandLine":"sleep 300"`, `"commandLine":"sleep 301"`, "presentation", Incomplete},
	} {
		for name, pair := range runningCommandShapes {
			t.Run(tc.name+" "+name, func(t *testing.T) {
				raw, presentation := pair[0], pair[1]
				s := snapshot1216(t, raw)
				switch tc.where {
				case "source":
					raw = replace1216(t, raw, tc.before, tc.after)
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
}

// A RUNNING native step that carries an error is not the observed shape, even
// when the presentation repeats it.
func TestProjectionRunningCommandWithInnerError(t *testing.T) {
	const added = `,"error":{"shortError":"context canceled"}`
	for name, pair := range runningCommandShapes {
		t.Run(name, func(t *testing.T) {
			raw := replace1216(t, pair[0], `"status":"CORTEX_STEP_STATUS_RUNNING",`+runningCommandMetadata, `"status":"CORTEX_STEP_STATUS_RUNNING",`+runningCommandMetadata+added)
			presentation := replace1216(t, pair[1], `"status":"CORTEX_STEP_STATUS_CANCELED",`+runningCommandMetadata, `"status":"CORTEX_STEP_STATUS_CANCELED",`+runningCommandMetadata+added)
			require1216(t, snapshot1216(t, raw), raw, presentation, Unknown)
		})
	}
}

// An outer error over a running native step is refused: only the outer
// CANCELED status without an error is qualified.
func TestProjectionRunningCommandWithOuterError(t *testing.T) {
	raw := replace1216(t, runningCommandRaw(""), `"generic":{`, `"error":{"shortError":"context canceled"},"generic":{`)
	require1216(t, snapshot1216(t, raw), raw, runningCommandPresentation(""), Unknown)
}
