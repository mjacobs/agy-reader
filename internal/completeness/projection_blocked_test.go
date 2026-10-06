package completeness

import (
	"strings"
	"testing"
)

// These fixtures cover two calls that agy stops before running (agy-reader#azya),
// first seen in the agy 1.2.17 audit. Frozen source, full RPC and default RPC
// copies showed that the default response presents both wrappers unchanged:
//
//   - A PreToolUse hook denies a run_command call. The wrapper has status ERROR,
//     the call's arguments, metadata that records the hook's denial, an outer
//     error (short and full text equal) and no result. The default response
//     replaces it with a RUN_COMMAND step that keeps the status, metadata and
//     error and synthesizes the command from the arguments.
//   - The image generator calls generate_image with an aspect ratio it does not
//     offer. The wrapper has status INVALID, arguments, an outer error (short
//     and full text equal) and no result, and is presented unchanged.
//
// Values are synthetic; the expected responses are written out explicitly.
const blockedHookMetadata = `"metadata":{"toolCall":{"id":"call-hook","name":"run_command","argumentsJson":"{\"CommandLine\":\"echo fixture\",\"Cwd\":\"/fixture/work\",\"WaitMsBeforeAsync\":5000,\"toolAction\":\"Running echo\",\"toolSummary\":\"Echo fixture\"}"},"completedAt":"2026-01-01T00:00:01Z","preToolHookResults":[{"decision":"deny","reason":"fixture-hook: blocked by the workspace hook"}],"toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN"}`
const blockedHookError = `"error":{"shortError":"tool call denied by pre-tool hook: fixture-hook: blocked by the workspace hook","fullError":"tool call denied by pre-tool hook: fixture-hook: blocked by the workspace hook"}`
const blockedHookRaw = `{"cascadeId":"hook-denied-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR",` + blockedHookMetadata + `,` + blockedHookError + `,"generic":{"args":{"CommandLine":"echo fixture","Cwd":"/fixture/work","WaitMsBeforeAsync":"5000","toolAction":"Running echo","toolSummary":"Echo fixture"}}}]}`
const blockedHookPresentation = `{"cascadeId":"hook-denied-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_ERROR",` + blockedHookMetadata + `,` + blockedHookError + `,"runCommand":{"commandLine":"echo fixture","proposedCommandLine":"echo fixture","cwd":"/fixture/work","blocking":true,"combinedOutput":{}}}]}`

const blockedImageRaw = `{"cascadeId":"image-invalid-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_INVALID","metadata":{"toolCall":{"id":"call-image","name":"generate_image","argumentsJson":"{\"AspectRatio\":\"37:1\",\"ImageName\":\"triangle\",\"Prompt\":\"A small green triangle\",\"toolAction\":\"Generating image\",\"toolSummary\":\"Image generation\"}"},"completedAt":"2026-01-01T00:00:01Z"},"error":{"shortError":"invalid arguments:\n- at '/AspectRatio': value must be one of '1:1', '2:3', '3:2', '3:4', '4:3', '9:16', '16:9'","fullError":"invalid arguments:\n- at '/AspectRatio': value must be one of '1:1', '2:3', '3:2', '3:4', '4:3', '9:16', '16:9'"},"generic":{"args":{"AspectRatio":"37:1","ImageName":"triangle","Prompt":"A small green triangle","toolAction":"Generating image","toolSummary":"Image generation"}}}]}`

const (
	blockedStatusError   = `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR"`
	blockedStatusInvalid = `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_INVALID"`
)

func TestProjectionBlockedCallsQualified(t *testing.T) {
	for name, pair := range map[string][2]string{
		"run_command denied by a pre-tool hook":   {blockedHookRaw, blockedHookPresentation},
		"hook denial with a multi-line reason":    {strings.ReplaceAll(blockedHookRaw, `blocked by the workspace hook`, `blocked:\nsecond line`), strings.ReplaceAll(blockedHookPresentation, `blocked by the workspace hook`, `blocked:\nsecond line`)},
		"generate_image with an invalid argument": {blockedImageRaw, blockedImageRaw},
		"generate_image without its image name":   {strings.Replace(strings.Replace(blockedImageRaw, `"ImageName":"triangle",`, ``, 1), `\"ImageName\":\"triangle\",`, ``, 1), ""},
	} {
		t.Run(name, func(t *testing.T) {
			raw, presentation := pair[0], pair[1]
			if presentation == "" {
				presentation = raw
			}
			require1216(t, snapshot1216(t, raw), raw, presentation, Verified)
		})
	}
}

// A source mutation applies to the persisted trajectory and both RPC copies. A
// presentation or raw mutation changes only that response.
func TestProjectionBlockedCallsChanges(t *testing.T) {
	const hookErrorFields = `"error":{"shortError":"tool call denied by pre-tool hook: fixture-hook: blocked by the workspace hook","fullError":"tool call denied by pre-tool hook: fixture-hook: blocked by the workspace hook"},`
	const hookResults = `"preToolHookResults":[{"decision":"deny","reason":"fixture-hook: blocked by the workspace hook"}],`
	const imageErrorFields = `"error":{"shortError":"invalid arguments:\n- at '/AspectRatio': value must be one of '1:1', '2:3', '3:2', '3:4', '4:3', '9:16', '16:9'","fullError":"invalid arguments:\n- at '/AspectRatio': value must be one of '1:1', '2:3', '3:2', '3:4', '4:3', '9:16', '16:9'"},`
	hook := func(name, before, after, where string, want State) blockedChange {
		return blockedChange{name, blockedHookRaw, blockedHookPresentation, before, after, where, want}
	}
	image := func(name, before, after, where string, want State) blockedChange {
		return blockedChange{name, blockedImageRaw, blockedImageRaw, before, after, where, want}
	}
	for _, tc := range []blockedChange{
		// run_command denied by a hook
		hook("hook: unknown argument", `"args":{`, `"args":{"Future":"new",`, "source", Unknown),
		hook("hook: argument of another tool", `"args":{`, `"args":{"AbsolutePath":"/fixture/x",`, "source", Unknown),
		hook("hook: missing wait argument", `"WaitMsBeforeAsync":"5000",`, ``, "source", Unknown),
		hook("hook: unknown extra wrapper field", `"generic":{`, `"permissions":{},"generic":{`, "source", Unknown),
		hook("hook: task details", `"generic":{`, `"taskDetails":{"id":"hook-denied-fixture/task-1"},"generic":{`, "source", Unknown),
		hook("hook: extra outer error field", `"error":{"shortError"`, `"error":{"errorId":"fixture-error","shortError"`, "source", Unknown),
		hook("hook: status done", blockedStatusError, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE"`, "source", Unknown),
		hook("hook: status invalid", blockedStatusError, blockedStatusInvalid, "source", Unknown),
		hook("hook: status canceled", blockedStatusError, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_CANCELED"`, "source", Unknown),
		hook("hook: no outer error", hookErrorFields, ``, "source", Unknown),
		hook("hook: error that is not a denial", `"shortError":"tool call denied by pre-tool hook:`, `"shortError":"tool call failed:`, "source", Unknown),
		hook("hook: no hook result", hookResults, ``, "source", Unknown),
		hook("hook: hook result that is not a denial", `"decision":"deny"`, `"decision":"allow"`, "source", Unknown),
		hook("hook: reason that the error does not repeat", `"reason":"fixture-hook: blocked by the workspace hook"`, `"reason":"fixture-hook: blocked by another hook"`, "source", Unknown),
		hook("hook: two hook results", `"preToolHookResults":[{"decision":"deny","reason":"fixture-hook: blocked by the workspace hook"}]`, `"preToolHookResults":[{"decision":"deny","reason":"fixture-hook: blocked by the workspace hook"},{"decision":"deny","reason":"fixture-hook: blocked by the workspace hook"}]`, "source", Unknown),
		hook("hook: presented error changed", `blocked by the workspace hook","fullError"`, `blocked by another hook","fullError"`, "presentation", Incomplete),
		hook("hook: presented error dropped", hookErrorFields, ``, "presentation", Incomplete),
		hook("hook: presented status changed", `"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_ERROR"`, `"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_DONE"`, "presentation", Incomplete),
		hook("hook: presented command changed", `"commandLine":"echo fixture"`, `"commandLine":"echo other"`, "presentation", Incomplete),
		hook("hook: presented proposed command changed", `"proposedCommandLine":"echo fixture"`, `"proposedCommandLine":"echo other"`, "presentation", Incomplete),
		hook("hook: presented working directory changed", `"cwd":"/fixture/work"`, `"cwd":"/fixture/other"`, "presentation", Incomplete),
		hook("hook: presented not blocking", `"blocking":true,`, ``, "presentation", Incomplete),
		hook("hook: presented with output", `"combinedOutput":{}`, `"combinedOutput":{"full":"output"}`, "presentation", Incomplete),
		hook("hook: presented hook result changed", `"decision":"deny"`, `"decision":"allow"`, "presentation", Incomplete),
		hook("hook: raw full error changed", `blocked by the workspace hook"}`, `blocked by another hook"}`, "raw", Incomplete),
		{"hook: presented as the unchanged wrapper", blockedHookRaw, blockedHookRaw, "", "", "presentation", Incomplete},
		// generate_image with an invalid argument
		image("image: unknown argument", `"args":{`, `"args":{"Future":"new",`, "source", Unknown),
		image("image: unknown extra wrapper field", `"generic":{`, `"permissions":{},"generic":{`, "source", Unknown),
		image("image: extra outer error field", `"error":{"shortError"`, `"error":{"errorId":"fixture-error","shortError"`, "source", Unknown),
		image("image: status error", blockedStatusInvalid, blockedStatusError, "source", Unknown),
		image("image: status done", blockedStatusInvalid, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE"`, "source", Unknown),
		image("image: no outer error", imageErrorFields, ``, "source", Unknown),
		image("image: error that is not an invalid argument", `"shortError":"invalid arguments:`, `"shortError":"failed to generate content:`, "source", Unknown),
		image("image: presented error changed", `'16:9'","fullError"`, `'16:9', '37:1'","fullError"`, "presentation", Incomplete),
		image("image: presented error dropped", imageErrorFields, ``, "presentation", Incomplete),
		image("image: presented status changed", blockedStatusInvalid, blockedStatusError, "presentation", Incomplete),
		image("image: presented argument changed", `"AspectRatio":"37:1"`, `"AspectRatio":"1:1"`, "presentation", Incomplete),
		image("image: raw full error changed", `'16:9'"}`, `'16:9', '1:1'"}`, "raw", Incomplete),
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

type blockedChange struct {
	name, source, presentation, before, after, where string
	want                                             State
}

// Each denied or invalid call is bounded to its own tool: the shapes do not
// carry over to the other tool or to unqualified ones.
func TestProjectionBlockedCallsOtherTools(t *testing.T) {
	for name, raw := range map[string]string{
		"hook denial over grep_search":    strings.ReplaceAll(blockedHookRaw, `"name":"run_command"`, `"name":"grep_search"`),
		"hook denial over generate_image": strings.ReplaceAll(blockedHookRaw, `"name":"run_command"`, `"name":"generate_image"`),
		"invalid image over run_command":  strings.ReplaceAll(blockedImageRaw, `"name":"generate_image"`, `"name":"run_command"`),
		"invalid image over view_file":    strings.ReplaceAll(blockedImageRaw, `"name":"generate_image"`, `"name":"view_file"`),
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, snapshot1216(t, raw), raw, raw, Unknown)
		})
	}
}
