package completeness

import (
	"strings"
	"testing"
)

// These fixtures cover a run_command call whose permission request was
// rejected (agy-reader#xyfe), seen in the agy 1.3.1 audit of 2026-10-07 in
// three situations: a user answering "No, cancel" to a command in the parent
// conversation, "No, deny" to a command a subagent asked for, and a subagent
// of a headless -p run, whose request agy soft-denied because nobody could
// answer. Frozen source, full RPC and default RPC copies of all three showed
// the same shape: a generic wrapper with status ERROR, the call's arguments,
// no result and no grant, metadata with one pre-tool hook result
// {decision: allow} (no hook was configured), an outer error (short and full
// text equal) that says the user denied the command, and the completed
// request, raised by the default fallback trigger and answered without allow.
// The default response replaces the wrapper with a RUN_COMMAND step built
// from the arguments, as for a command a hook denied (agy-reader#azya), and
// keeps the status, metadata, error and completed request. The headless child
// also passed BypassSandbox "false" and waited 10000 ms.
//
// Values are synthetic; every response is spelled out here, never derived from
// the projection.
const (
	rejectedCommand = "echo FIXTURE_REJECT"
	rejectedArgs    = `"CommandLine":"echo FIXTURE_REJECT","Cwd":"/fixture/ws","WaitMsBeforeAsync":"5000","toolAction":"Running fixture","toolSummary":"Echo fixture"`
	rejectedHooks   = `"preToolHookResults":[{"decision":"allow"}]`
	rejectedMeta    = `"metadata":{"toolCall":{"id":"call-reject","name":"run_command","argumentsJson":"{\"CommandLine\":\"echo FIXTURE_REJECT\",\"Cwd\":\"/fixture/ws\",\"WaitMsBeforeAsync\":5000,\"toolAction\":\"Running fixture\",\"toolSummary\":\"Echo fixture\"}"},"completedAt":"2026-01-01T00:00:01Z",` + rejectedHooks + `,"toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN"}`
	rejectedText    = `permission check failed for command \"echo FIXTURE_REJECT\": user denied permission to run command:\necho FIXTURE_REJECT\nDo not attempt to circumvent this denial by rephrasing the command, using alternative tools/scripts (e.g. python, sh, curl), or accessing the same target resource. Proceed without performing this action.`
	rejectedError   = `"error":{"shortError":"` + rejectedText + `","fullError":"` + rejectedText + `"}`
	rejectedRequest = `{"permission":{"resource":{"action":"command","target":"echo FIXTURE_REJECT"},"persistSuggestionType":"PERSIST_SUGGESTION_TYPE_SUGGESTED","suggestedPersistPattern":"echo","actionDescription":"Running fixture","triggerSource":{"type":"TYPE_DEFAULT_FALLBACK"},"shellName":"bash"}}`
	rejectedAnswer  = `{"trajectoryId":"fixture-trajectory","stepIndex":2,"permission":{"persistGrants":{}}}`
	rejectedRecord  = `"completedInteractions":[{"request":` + rejectedRequest + `,"response":` + rejectedAnswer + `}]`
	rejectedStatus  = `"status":"CORTEX_STEP_STATUS_ERROR"`
	rejectedRaw     = `{"cascadeId":"rejected-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC",` + rejectedStatus + `,` + rejectedMeta + `,` + rejectedError + `,` + rejectedRecord + `,"generic":{"args":{` + rejectedArgs + `}}}]}`
	rejectedPresent = `{"cascadeId":"rejected-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND",` + rejectedStatus + `,` + rejectedMeta + `,` + rejectedError + `,` + rejectedRecord + `,"runCommand":{"commandLine":"echo FIXTURE_REJECT","proposedCommandLine":"echo FIXTURE_REJECT","cwd":"/fixture/ws","blocking":true,"combinedOutput":{}}}]}`
)

// rejectedHeadless returns the source and presentation of the headless
// child's form: BypassSandbox "false" in the arguments and the tool call (the
// presented step leaves sandbox_override unset) and a 10000 ms wait, still
// presented as blocking.
func rejectedHeadless(raw, presentation string) (string, string) {
	r := strings.NewReplacer(
		`"CommandLine":"echo FIXTURE_REJECT","Cwd"`, `"BypassSandbox":"false","CommandLine":"echo FIXTURE_REJECT","Cwd"`,
		`{\"CommandLine\":`, `{\"BypassSandbox\":false,\"CommandLine\":`,
		`"WaitMsBeforeAsync":"5000"`, `"WaitMsBeforeAsync":"10000"`,
		`\"WaitMsBeforeAsync\":5000`, `\"WaitMsBeforeAsync\":10000`,
	)
	return r.Replace(raw), r.Replace(presentation)
}

func TestProjectionRejectedCommandQualified(t *testing.T) {
	headlessRaw, headlessPresent := rejectedHeadless(rejectedRaw, rejectedPresent)
	if headlessRaw == rejectedRaw || !strings.Contains(headlessRaw, `\"BypassSandbox\":false`) || !strings.Contains(headlessRaw, `"WaitMsBeforeAsync":"10000"`) {
		t.Fatal("headless fixture unchanged")
	}
	other := strings.NewReplacer(`FIXTURE_REJECT`, `'two words' && echo \"quoted\"`, `\"echo FIXTURE_REJECT\"`, `\"echo 'two words' && echo \\\"quoted\\\"\"`)
	for name, pair := range map[string][2]string{
		"rejected by the user":                  {rejectedRaw, rejectedPresent},
		"soft-denied for a headless subagent":   {headlessRaw, headlessPresent},
		"rejected with a different answer step": {strings.ReplaceAll(rejectedRaw, `"stepIndex":2`, `"stepIndex":6`), strings.ReplaceAll(rejectedPresent, `"stepIndex":2`, `"stepIndex":6`)},
		"command that needs quoting":            {other.Replace(rejectedRaw), other.Replace(rejectedPresent)},
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, snapshot1216(t, pair[0]), pair[0], pair[1], Verified)
		})
	}
}

// A source mutation applies to the persisted trajectory and both RPC copies. A
// presentation or raw mutation changes only that response.
func TestProjectionRejectedCommandChanges(t *testing.T) {
	c := func(name, before, after, where string, want State) blockedChange {
		return blockedChange{name, rejectedRaw, rejectedPresent, before, after, where, want}
	}
	for _, tc := range []blockedChange{
		// Unknown extra fields, wherever they appear, fail.
		c("unknown argument", `"args":{`, `"args":{"Future":"new",`, "source", Unknown),
		c("argument of another tool", `"args":{`, `"args":{"AbsolutePath":"/fixture/x",`, "source", Unknown),
		c("sandbox bypassed", `"args":{`, `"args":{"BypassSandbox":"true",`, "source", Unknown),
		c("argument missing from the tool call", `"args":{`, `"args":{"BypassSandbox":"false",`, "source", Unknown),
		c("argument that differs from the tool call", `"Cwd":"/fixture/ws"`, `"Cwd":"/fixture/other"`, "source", Unknown),
		c("missing wait argument", `"WaitMsBeforeAsync":"5000",`, ``, "source", Unknown),
		c("grant on the wrapper", `"generic":{`, `"permissions":{"permissionGrants":[{"action":"command","target":"echo FIXTURE_REJECT"}]},"generic":{`, "source", Unknown),
		c("empty permissions on the wrapper", `"generic":{`, `"permissions":{},"generic":{`, "source", Unknown),
		c("task details", `"generic":{`, `"taskDetails":{"id":"rejected-fixture/task-1"},"generic":{`, "source", Unknown),
		c("extra outer error field", `"error":{"shortError"`, `"error":{"errorId":"fixture-error","shortError"`, "source", Unknown),
		c("request reason", `"shellName":"bash"`, `"shellName":"bash","reason":"fixture reason"`, "source", Unknown),
		c("trigger detail", `{"type":"TYPE_DEFAULT_FALLBACK"}`, `{"type":"TYPE_DEFAULT_FALLBACK","detail":"fixture"}`, "source", Unknown),
		c("pending request on the wrapper", `"generic":{`, `"requestedInteraction":{"permission":{"shellName":"bash"}},"generic":{`, "source", Unknown),
		c("request of another kind", `"request":`+rejectedRequest, `"request":{"elicitation":{"message":"fixture"}}`, "source", Unknown),
		c("answer timed out", `"stepIndex":2,`, `"stepIndex":2,"timedOut":true,`, "source", Unknown),
		c("answer edited target", `"permission":{"persistGrants":{}}`, `"permission":{"editedTarget":"echo OTHER","persistGrants":{}}`, "source", Unknown),
		c("persisted rule", `"persistGrants":{}`, `"persistGrants":{"allow":["echo"]}`, "source", Unknown),
		c("hook result with a reason", rejectedHooks, `"preToolHookResults":[{"decision":"allow","reason":"fixture"}]`, "source", Unknown),
		c("second interaction", `"response":`+rejectedAnswer+`}]`, `"response":`+rejectedAnswer+`},{"request":`+rejectedRequest+`,"response":`+rejectedAnswer+`}]`, "source", Unknown),
		// An answer that allows is the approved shape, which needs a result.
		c("answer that allows", `"permission":{"persistGrants":{}}`, `"permission":{"allow":true,"persistGrants":{}}`, "source", Unknown),
		// Another trigger, a changed error or a missing record fails.
		c("raised by a force_ask hook", `{"type":"TYPE_DEFAULT_FALLBACK"}`, `{"type":"TYPE_FORCE_ASK_HOOK"}`, "source", Unknown),
		c("request without a trigger", `,"triggerSource":{"type":"TYPE_DEFAULT_FALLBACK"}`, ``, "source", Unknown),
		c("request for another command", `"target":"echo FIXTURE_REJECT"`, `"target":"echo OTHER"`, "source", Unknown),
		c("request for another action", `"action":"command"`, `"action":"read_file"`, "source", Unknown),
		c("error changed", `user denied permission to run command:`, `permission request timed out:`, "source", Unknown),
		c("error for another command", `failed for command \"echo FIXTURE_REJECT\"`, `failed for command \"echo OTHER\"`, "source", Unknown),
		c("error without the advice", ` Proceed without performing this action.`, ``, "source", Unknown),
		c("full error differs", `","fullError":"permission check failed`, `","fullError":"fixture: permission check failed`, "source", Unknown),
		c("no outer error", rejectedError+`,`, ``, "source", Unknown),
		c("missing interaction record", `,`+rejectedRecord, ``, "source", Unknown),
		c("interaction without an answer", `,"response":`+rejectedAnswer, ``, "source", Unknown),
		c("hook denied", `{"decision":"allow"}`, `{"decision":"deny"}`, "source", Unknown),
		c("no hook result", `,`+rejectedHooks, ``, "source", Unknown),
		c("two hook results", rejectedHooks, `"preToolHookResults":[{"decision":"allow"},{"decision":"allow"}]`, "source", Unknown),
		c("status canceled", rejectedStatus, `"status":"CORTEX_STEP_STATUS_CANCELED"`, "source", Unknown),
		c("status done", rejectedStatus, `"status":"CORTEX_STEP_STATUS_DONE"`, "source", Unknown),
		// A response that differs from the source is incomplete.
		c("presented record dropped", `,`+rejectedRecord, ``, "presentation", Incomplete),
		c("presented answer allows", `"permission":{"persistGrants":{}}`, `"permission":{"allow":true,"persistGrants":{}}`, "presentation", Incomplete),
		c("presented trigger changed", `"TYPE_DEFAULT_FALLBACK"`, `"TYPE_FORCE_ASK_HOOK"`, "presentation", Incomplete),
		c("presented error changed", `user denied permission`, `user granted permission`, "presentation", Incomplete),
		c("presented error dropped", rejectedError+`,`, ``, "presentation", Incomplete),
		c("presented status changed", rejectedStatus, `"status":"CORTEX_STEP_STATUS_DONE"`, "presentation", Incomplete),
		c("presented with output", `"combinedOutput":{}`, `"combinedOutput":{"full":"FIXTURE_REJECT\n"}`, "presentation", Incomplete),
		c("presented with an exit code", `"blocking":true,`, `"blocking":true,"exitCode":0,`, "presentation", Incomplete),
		c("presented not blocking", `"blocking":true,`, ``, "presentation", Incomplete),
		c("presented sandbox override", `"blocking":true,`, `"blocking":true,"sandboxOverride":true,`, "presentation", Incomplete),
		c("presented command changed", `"commandLine":"echo FIXTURE_REJECT"`, `"commandLine":"echo OTHER"`, "presentation", Incomplete),
		c("raw answer changed", `"stepIndex":2`, `"stepIndex":3`, "raw", Incomplete),
		{"presented as the unchanged wrapper", rejectedRaw, rejectedRaw, "", "", "presentation", Incomplete},
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

// The rejected request is bounded to a resultless run_command wrapper: the same
// record on another tool's resultless wrapper, or on a run_command that ran,
// fails, and the hook-denied shape still may not carry a record.
func TestProjectionRejectedCommandOtherSteps(t *testing.T) {
	for name, raw := range map[string]string{
		"rejected record over list_dir":      strings.Replace(strings.ReplaceAll(blockedImageRaw, `"name":"generate_image"`, `"name":"list_dir"`), `"generic":{`, rejectedRecord+`,"generic":{`, 1),
		"rejected record over view_file":     strings.Replace(strings.Replace(strings.ReplaceAll(blockedImageRaw, `"name":"generate_image"`, `"name":"view_file"`), `"CORTEX_STEP_STATUS_INVALID"`, `"CORTEX_STEP_STATUS_ERROR"`, 1), `"generic":{`, rejectedRecord+`,"generic":{`, 1),
		"rejected record over a hook denial": strings.Replace(blockedHookRaw, `"generic":{`, rejectedRecord+`,"generic":{`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, snapshot1216(t, raw), raw, raw, Unknown)
		})
	}
	// The approved record (agy-reader#pzg7) with a rejecting answer still fails
	// on a command that ran.
	ran := strings.ReplaceAll(permissionRaw, `"permission":{"allow":true,"persistGrants":{}}`, `"permission":{"persistGrants":{}}`)
	t.Run("rejected answer on a command that ran", func(t *testing.T) {
		require1216(t, snapshot1216(t, ran), ran, strings.ReplaceAll(permissionPresent, `"permission":{"allow":true,"persistGrants":{}}`, `"permission":{"persistGrants":{}}`), Unknown)
	})
}
