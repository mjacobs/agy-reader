package completeness

import (
	"strings"
	"testing"
)

// These fixtures cover a run_command step whose command asked for permission
// and was approved (agy-reader#pzg7). In the agy 1.2.17 audit a workspace
// PreToolUse hook answered force_ask for one command, agy's always-proceed
// setting approved the request, and the command ran. Frozen source, full RPC
// and default RPC copies of the parent conversation (step 6) and of a
// subagent's (step 2) showed the same shape: the wrapper and the embedded
// RUN_COMMAND Step both store the grant (permissions) and the request with its
// answer (completedInteractions), and the native metadata lists the hook
// results. The default response presents the native Step verbatim and drops
// the wrapper's copy. Values are synthetic; every response is spelled out
// here, never derived from the projection.
//
// A request the user rejects, and one raised by a subagent that cannot ask,
// were never produced, so no fixture claims to show them; the mutations below
// only check that a rejected or otherwise different record fails closed.

const (
	permissionCommand  = "echo FIXTURE_ASK"
	permissionHooks    = `"preToolHookResults":[{"decision":"force_ask","reason":"fixture hook: needs approval"},{"decision":"allow"}]`
	permissionGrant    = `{"allow":true,"action":"command","target":"echo FIXTURE_ASK","resource":{"action":"command","target":"echo FIXTURE_ASK"}}`
	permissionRequest  = `{"permission":{"resource":{"action":"command","target":"echo FIXTURE_ASK"},"persistSuggestionType":"PERSIST_SUGGESTION_TYPE_SUGGESTED","suggestedPersistPattern":"echo","reason":"fixture hook: needs approval","actionDescription":"Running fixture","triggerSource":{"type":"TYPE_FORCE_ASK_HOOK","detail":"fixture hook: needs approval"},"shellName":"bash"}}`
	permissionResponse = `{"trajectoryId":"fixture-trajectory","stepIndex":2,"permission":{"allow":true,"persistGrants":{}}}`
	permissionRecord   = `"permissions":{"permissionGrants":[` + permissionGrant + `]},"completedInteractions":[{"request":` + permissionRequest + `,"response":` + permissionResponse + `}]`
	permissionNative   = `"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-perm","name":"run_command","argumentsJson":"{\"CommandLine\":\"echo FIXTURE_ASK\",\"Cwd\":\"/fixture/ws\",\"WaitMsBeforeAsync\":5000,\"toolAction\":\"Running fixture\",\"toolSummary\":\"Echo fixture\"}"},"completedAt":"2026-01-01T00:00:01Z",` + permissionHooks + `},` + permissionRecord + `,"runCommand":{"commandLine":"echo FIXTURE_ASK","proposedCommandLine":"echo FIXTURE_ASK","cwd":"/fixture/ws","waitMsBeforeAsync":"5000","blocking":true,"exitCode":0,"combinedOutput":{"full":"FIXTURE_ASK\r\n"},"shellName":"bash","ranInSandbox":true}`
	permissionRaw      = `{"cascadeId":"permission-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-perm","name":"run_command"},"completedAt":"2026-01-01T00:00:02Z",` + permissionHooks + `},` + permissionRecord + `,"generic":{"args":{"CommandLine":"echo FIXTURE_ASK","Cwd":"/fixture/ws","WaitMsBeforeAsync":"5000","toolAction":"Running fixture","toolSummary":"Echo fixture"},"result":{"result":"\nThe command exited with code 0.\nOutput:\nFIXTURE_ASK\r\n\n","fullOutputUri":"file:///fixture/steps/2/output.txt","payload":{"@type":"type.googleapis.com/gemini_coder.Step",` + permissionNative + `}}}}]}`
	permissionPresent  = `{"cascadeId":"permission-fixture","metadata":{},"steps":[{` + permissionNative + `}]}`
)

// The wrapper's record is the first copy in the source and full RPC; the
// native Step's is the second.
const (
	wrapperRecordStart = `"completedAt":"2026-01-01T00:00:02Z",`
	nativeRecordStart  = `"completedAt":"2026-01-01T00:00:01Z",`
)

type permissionMutation struct {
	name string
	// source edits apply to every copy in the persisted trajectory, the full
	// RPC and the presentation.
	source [][2]string
	// wrapper edits apply only to the wrapper's copy in the persisted
	// trajectory and the full RPC; native edits only to the embedded Step's
	// copy there and to the presentation, which shows that copy.
	wrapper, native [][2]string
	// raw and presentation edits change only that response.
	raw, presentation [][2]string
	want              State
}

// editCopy applies one edit to the copy of the record that follows start.
func editCopy(t *testing.T, s, start string, e [2]string) string {
	t.Helper()
	i := strings.Index(s, start)
	if i < 0 {
		t.Fatalf("no %q in fixture", start)
	}
	head, tail := s[:i], s[i:]
	// Stop at the other copy, so an edit to one copy never reaches the next.
	end := len(tail)
	for _, other := range []string{wrapperRecordStart, nativeRecordStart} {
		if other == start {
			continue
		}
		if j := strings.Index(tail, other); j > 0 && j < end {
			end = j
		}
	}
	part := tail[:end]
	if !strings.Contains(part, e[0]) || e[0] == e[1] {
		t.Fatalf("invalid fixture mutation: %q", e[0])
	}
	return head + strings.Replace(part, e[0], e[1], 1) + tail[end:]
}

func applyPermissionMutation(t *testing.T, m permissionMutation) {
	t.Helper()
	raw, presentation := permissionRaw, permissionPresent
	for _, e := range m.source {
		raw = replace1216(t, raw, e[0], e[1])
		presentation = strings.ReplaceAll(presentation, e[0], e[1])
	}
	for _, e := range m.wrapper {
		raw = editCopy(t, raw, wrapperRecordStart, e)
	}
	for _, e := range m.native {
		raw = editCopy(t, raw, nativeRecordStart, e)
		presentation = replace1216(t, presentation, e[0], e[1])
	}
	s := snapshot1216(t, raw)
	for _, e := range m.raw {
		raw = editCopy(t, raw, wrapperRecordStart, e)
	}
	for _, e := range m.presentation {
		presentation = replace1216(t, presentation, e[0], e[1])
	}
	require1216(t, s, raw, presentation, m.want)
}

func TestProjectionPermissionQualifiedShape(t *testing.T) {
	require1216(t, snapshot1216(t, permissionRaw), permissionRaw, permissionPresent, Verified)
	// Free-text values flow through: a different reason, pattern or answer
	// step still verifies when every copy and the presentation agree.
	for _, m := range []permissionMutation{
		{name: "hook reason changed everywhere", source: [][2]string{{`fixture hook: needs approval`, `fixture hook: other reason`}}, want: Verified},
		{name: "suggested pattern changed everywhere", source: [][2]string{{`"suggestedPersistPattern":"echo"`, `"suggestedPersistPattern":"echo FIXTURE"`}}, want: Verified},
		{name: "answer step index changed everywhere", source: [][2]string{{`"stepIndex":2`, `"stepIndex":6`}}, want: Verified},
	} {
		t.Run(m.name, func(t *testing.T) { applyPermissionMutation(t, m) })
	}
}

// An unknown extra field anywhere in the record, on the wrapper, or a record
// on an unexpected step fails, even when every copy and the presentation agree.
func TestProjectionPermissionExtraFields(t *testing.T) {
	for _, m := range []permissionMutation{
		{name: "pending request on the wrapper", wrapper: [][2]string{{`"permissions":`, `"requestedInteraction":{"permission":{"shellName":"bash"}},"permissions":`}}, want: Unknown},
		{name: "file access permission", source: [][2]string{{`"permissions":{`, `"permissions":{"fileAccessPermissions":[{"path":"/fixture/ws","allow":true}],`}}, want: Unknown},
		{name: "grant scope", source: [][2]string{{`{"allow":true,"action":"command"`, `{"allow":true,"scope":"PERMISSION_SCOPE_ONCE","action":"command"`}}, want: Unknown},
		{name: "grant deny instruction", source: [][2]string{{`{"allow":true,"action":"command"`, `{"allow":true,"userDenyInstruction":"no","action":"command"`}}, want: Unknown},
		{name: "request of another kind", source: [][2]string{{`"request":` + permissionRequest, `"request":{"elicitation":{"message":"fixture"}}`}}, want: Unknown},
		{name: "answer timed out", source: [][2]string{{`"stepIndex":2,`, `"stepIndex":2,"timedOut":true,`}}, want: Unknown},
		{name: "answer from remote control", source: [][2]string{{`"stepIndex":2,`, `"stepIndex":2,"remoteControlDetails":{"userAgent":"fixture"},`}}, want: Unknown},
		{name: "answer edited target", source: [][2]string{{`"permission":{"allow":true,"persistGrants":{}}`, `"permission":{"allow":true,"editedTarget":"echo OTHER","persistGrants":{}}`}}, want: Unknown},
		{name: "answer scope", source: [][2]string{{`"permission":{"allow":true,"persistGrants":{}}`, `"permission":{"allow":true,"scope":"PERMISSION_SCOPE_WORKSPACE","persistGrants":{}}`}}, want: Unknown},
		{name: "answer sandbox override", source: [][2]string{{`"permission":{"allow":true,"persistGrants":{}}`, `"permission":{"allow":true,"persistGrants":{},"sandboxOverride":true}`}}, want: Unknown},
		{name: "persisted grant", source: [][2]string{{`"persistGrants":{}`, `"persistGrants":{"allow":["echo"]}`}}, want: Unknown},
		{name: "second grant", source: [][2]string{{`"permissionGrants":[` + permissionGrant, `"permissionGrants":[` + permissionGrant + `,` + permissionGrant}}, want: Unknown},
		{name: "second interaction", source: [][2]string{{`"response":` + permissionResponse + `}]`, `"response":` + permissionResponse + `},{"request":` + permissionRequest + `,"response":` + permissionResponse + `}]`}}, want: Unknown},
		{name: "interaction without a request", source: [][2]string{{`"request":` + permissionRequest + `,`, ``}}, want: Unknown},
		{name: "interaction without an answer", source: [][2]string{{`,"response":` + permissionResponse, ``}}, want: Unknown},
		{name: "grant without a resource", source: [][2]string{{`,"resource":{"action":"command","target":"echo FIXTURE_ASK"}}]`, `}]`}}, want: Unknown},
	} {
		t.Run(m.name, func(t *testing.T) { applyPermissionMutation(t, m) })
	}
}

// Changed values fail: the record must be the approved one, for the command
// that ran, and the wrapper's copy must equal the presented native Step's.
func TestProjectionPermissionChangedValues(t *testing.T) {
	for _, m := range []permissionMutation{
		// Never-observed records fail closed wherever they appear.
		{name: "rejected request", source: [][2]string{{`"permission":{"allow":true,"persistGrants":{}}`, `"permission":{"persistGrants":{}}`}}, want: Unknown},
		{name: "denying grant", source: [][2]string{{`{"allow":true,"action":"command"`, `{"action":"command"`}}, want: Unknown},
		{name: "rejected request and denying grant", source: [][2]string{{`"permission":{"allow":true,"persistGrants":{}}`, `"permission":{"persistGrants":{}}`}, {`{"allow":true,"action":"command"`, `{"action":"command"`}}, want: Unknown},
		{name: "request raised by the default policy", source: [][2]string{{`"type":"TYPE_FORCE_ASK_HOOK"`, `"type":"TYPE_DEFAULT_FALLBACK"`}}, want: Unknown},
		{name: "request without a trigger", source: [][2]string{{`,"triggerSource":{"type":"TYPE_FORCE_ASK_HOOK","detail":"fixture hook: needs approval"}`, ``}}, want: Unknown},
		{name: "grant for another action", source: [][2]string{{`"allow":true,"action":"command"`, `"allow":true,"action":"file"`}}, want: Unknown},
		{name: "grant for another command", source: [][2]string{{`"target":"echo FIXTURE_ASK","resource"`, `"target":"echo OTHER","resource"`}}, want: Unknown},
		{name: "grant resource for another command", source: [][2]string{{`"resource":{"action":"command","target":"echo FIXTURE_ASK"}}]`, `"resource":{"action":"command","target":"echo OTHER"}}]`}}, want: Unknown},
		{name: "request for another command", source: [][2]string{{`"permission":{"resource":{"action":"command","target":"echo FIXTURE_ASK"}`, `"permission":{"resource":{"action":"command","target":"echo OTHER"}`}}, want: Unknown},
		{name: "command that ran differs from the grant", source: [][2]string{{`"runCommand":{"commandLine":"echo FIXTURE_ASK"`, `"runCommand":{"commandLine":"echo OTHER"`}}, want: Unknown},
		// The wrapper's copy, which the presentation drops, must equal the native Step's.
		{name: "wrapper grant differs", wrapper: [][2]string{{`"target":"echo FIXTURE_ASK","resource"`, `"target":"echo OTHER","resource"`}}, want: Unknown},
		{name: "wrapper answer differs", wrapper: [][2]string{{`"stepIndex":2`, `"stepIndex":3`}}, want: Unknown},
		{name: "wrapper request reason differs", wrapper: [][2]string{{`"reason":"fixture hook: needs approval","actionDescription"`, `"reason":"fixture hook: other","actionDescription"`}}, want: Unknown},
		{name: "native answer differs", native: [][2]string{{`"stepIndex":2`, `"stepIndex":3`}}, want: Unknown},
		{name: "wrapper without permissions", wrapper: [][2]string{{`"permissions":{"permissionGrants":[` + permissionGrant + `]},`, ``}}, want: Unknown},
		{name: "wrapper without interactions", wrapper: [][2]string{{`,"completedInteractions":[{"request":` + permissionRequest + `,"response":` + permissionResponse + `}]`, ``}}, want: Unknown},
		{name: "wrapper without a record", wrapper: [][2]string{{permissionRecord + `,`, ``}}, want: Unknown},
		{name: "native step without a record", native: [][2]string{{`,` + permissionRecord, ``}}, want: Unknown},
		// Only a completed command was observed with a record.
		{name: "outer canceled", source: [][2]string{{`"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE"`, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_CANCELED"`}}, want: Unknown},
		{name: "failed with a repeated error", source: [][2]string{
			{`"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE"`, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR"`},
			{`"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_DONE"`, `"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_ERROR"`},
			{permissionRecord + `,"generic"`, `"error":{"shortError":"fixture failure","fullError":"fixture: fixture failure"},` + permissionRecord + `,"generic"`},
			{permissionRecord + `,"runCommand"`, `"error":{"shortError":"fixture failure"},` + permissionRecord + `,"runCommand"`},
		}, want: Unknown},
		// A response that differs from the source is incomplete.
		{name: "presented answer changed", presentation: [][2]string{{`"permission":{"allow":true,"persistGrants":{}}`, `"permission":{"persistGrants":{}}`}}, want: Incomplete},
		{name: "presented grant changed", presentation: [][2]string{{`"target":"echo FIXTURE_ASK","resource"`, `"target":"echo OTHER","resource"`}}, want: Incomplete},
		{name: "presented trigger changed", presentation: [][2]string{{`"type":"TYPE_FORCE_ASK_HOOK"`, `"type":"TYPE_DEFAULT_FALLBACK"`}}, want: Incomplete},
		{name: "presented record dropped", presentation: [][2]string{{`,` + permissionRecord, ``}}, want: Incomplete},
		{name: "presented hook results changed", presentation: [][2]string{{`{"decision":"force_ask"`, `{"decision":"deny"`}}, want: Incomplete},
		{name: "presented hook results dropped", presentation: [][2]string{{`,` + permissionHooks, ``}}, want: Incomplete},
		{name: "raw wrapper grant changed", raw: [][2]string{{`"target":"echo FIXTURE_ASK","resource"`, `"target":"echo OTHER","resource"`}}, want: Incomplete},
		{name: "raw wrapper answer changed", raw: [][2]string{{`"stepIndex":2`, `"stepIndex":3`}}, want: Incomplete},
		{name: "raw wrapper hook results changed", raw: [][2]string{{`{"decision":"force_ask"`, `{"decision":"deny"`}}, want: Incomplete},
	} {
		t.Run(m.name, func(t *testing.T) { applyPermissionMutation(t, m) })
	}
}

// Only a run_command wrapper over a RUN_COMMAND Step may carry a permission
// record.
func TestProjectionPermissionOtherSteps(t *testing.T) {
	// Another tool's wrapper, with the same record on it and its native Step.
	list, listPresentation := nativeToolFixture(nativeToolByName(t, "list_dir"))
	withRecord := strings.Replace(list, `"generic":{`, permissionRecord+`,"generic":{`, 1)
	withRecord = strings.Replace(withRecord, `"listDirectory":`, permissionRecord+`,"listDirectory":`, 1)
	t.Run("record on a list_dir wrapper", func(t *testing.T) {
		require1216(t, snapshot1216(t, withRecord), withRecord, strings.Replace(listPresentation, `"listDirectory":`, permissionRecord+`,"listDirectory":`, 1), Unknown)
	})
	// A wrapper named run_command over a VIEW_FILE Step.
	raw := strings.Replace(currentViewRaw, `"name":"view_file"`, `"name":"run_command"`, 1)
	raw = strings.Replace(raw, `"generic":{`, permissionRecord+`,"generic":{`, 1)
	if raw == currentViewRaw {
		t.Fatal("fixture unchanged")
	}
	t.Run("run_command wrapper over a view_file step", func(t *testing.T) {
		require1216(t, snapshot1216(t, raw), raw, currentViewPresentation, Unknown)
	})
	// The ask_question fixture, whose interactions are the question's, still verifies,
	// and its wrapper may not carry permissions.
	ask := nativeToolByName(t, "ask_question")
	askRaw, askPresentation := nativeToolFixture(ask)
	t.Run("ask_question unchanged", func(t *testing.T) {
		require1216(t, snapshot1216(t, askRaw), askRaw, askPresentation, Verified)
	})
	grants := `"permissions":{"permissionGrants":[` + permissionGrant + `]},`
	askWithGrant := strings.Replace(askRaw, `"generic":{`, grants+`"generic":{`, 1)
	t.Run("permissions on an ask_question wrapper", func(t *testing.T) {
		require1216(t, snapshot1216(t, askWithGrant), askWithGrant, askPresentation, Unknown)
	})
}
