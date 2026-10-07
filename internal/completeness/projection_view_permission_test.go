package completeness

import (
	"strings"
	"testing"
)

// These fixtures cover a view_file step whose read asked for permission and was
// approved (agy-reader#axnr). In a retained agy 1.2.16 conversation the wrapper
// and the embedded VIEW_FILE Step both store the grant (permissions) and the
// request with its answer (completedInteractions); the request names only the
// resource and an action description. The default response presents the native
// Step verbatim and drops the wrapper's copy. Values are synthetic; every
// response is spelled out here, never derived from the projection.

const (
	viewPermPath     = "/fixture/notes.md"
	viewPermGrant    = `{"resource":{"action":"read_file","target":"/fixture/notes.md"},"allow":true,"action":"read_file","target":"/fixture/notes.md"}`
	viewPermRequest  = `{"permission":{"resource":{"action":"read_file","target":"/fixture/notes.md"},"actionDescription":"Viewing notes"}}`
	viewPermResponse = `{"trajectoryId":"fixture-trajectory","stepIndex":2,"permission":{"allow":true,"persistGrants":{}}}`
	viewPermRecord   = `"permissions":{"permissionGrants":[` + viewPermGrant + `]},"completedInteractions":[{"request":` + viewPermRequest + `,"response":` + viewPermResponse + `}]`
	viewPermNative   = `"type":"CORTEX_STEP_TYPE_VIEW_FILE","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-view","name":"view_file","argumentsJson":"` + currentViewArguments + `"},"completedAt":"2026-01-01T00:00:01Z","preToolHookResults":[{"decision":"allow"}]},` + viewPermRecord + `,"viewFile":{"absolutePathUri":"file:///fixture/notes.md","endLine":2,"content":"one\ntwo\n","lineRangeBytes":8,"numLines":3,"numBytes":8}`
	viewPermRaw      = `{"cascadeId":"view-permission-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-view","name":"view_file"},"completedAt":"2026-01-01T00:00:02Z","preToolHookResults":[{"decision":"allow"}]},` + viewPermRecord + `,"generic":{"args":{"AbsolutePath":"/fixture/notes.md","ContentOffset":"0","EndLine":"120","StartLine":"1","toolAction":"Viewing notes","toolSummary":"View notes"},"result":{"result":"File Path: file:///fixture/notes.md\none\ntwo\n","payload":{"@type":"type.googleapis.com/gemini_coder.Step",` + viewPermNative + `}}}}]}`
	viewPermPresent  = `{"cascadeId":"view-permission-fixture","metadata":{},"steps":[{` + viewPermNative + `}]}`
)

// viewPermMutation edits copies of the fixture. source edits apply to every
// copy (the persisted trajectory, the full RPC and the presentation); wrapper
// edits only to the wrapper's record in the persisted trajectory and the full
// RPC; native edits only to the embedded Step's, which the presentation shows;
// raw and presentation edits change only that response.
type viewPermMutation struct {
	name                                string
	source, wrapper, native, raw, shown [][2]string
	want                                State
}

func (m viewPermMutation) run(t *testing.T) {
	t.Helper()
	raw, shown := viewPermRaw, viewPermPresent
	for _, e := range m.source {
		raw = replace1216(t, raw, e[0], e[1])
		shown = strings.ReplaceAll(shown, e[0], e[1])
	}
	for _, e := range m.wrapper {
		raw = editCopy(t, raw, wrapperRecordStart, e)
	}
	for _, e := range m.native {
		raw = editCopy(t, raw, nativeRecordStart, e)
		shown = replace1216(t, shown, e[0], e[1])
	}
	s := snapshot1216(t, raw)
	for _, e := range m.raw {
		raw = editCopy(t, raw, wrapperRecordStart, e)
	}
	for _, e := range m.shown {
		shown = replace1216(t, shown, e[0], e[1])
	}
	require1216(t, s, raw, shown, m.want)
}

func TestProjectionViewPermissionQualifiedShape(t *testing.T) {
	require1216(t, snapshot1216(t, viewPermRaw), viewPermRaw, viewPermPresent, Verified)
	// Free text flows through when every copy agrees.
	for _, m := range []viewPermMutation{
		{name: "action description changed everywhere", source: [][2]string{{`"actionDescription":"Viewing notes"`, `"actionDescription":"Reading"`}}, want: Verified},
		{name: "answer step index changed everywhere", source: [][2]string{{`"stepIndex":2`, `"stepIndex":6`}}, want: Verified},
	} {
		t.Run(m.name, m.run)
	}
}

// An unknown field anywhere in the record fails, even when every copy agrees.
func TestProjectionViewPermissionExtraFields(t *testing.T) {
	for _, m := range []viewPermMutation{
		{name: "pending request on the wrapper", wrapper: [][2]string{{`"permissions":`, `"requestedInteraction":{"permission":{"shellName":"bash"}},"permissions":`}}, want: Unknown},
		{name: "file access permission", source: [][2]string{{`"permissions":{`, `"permissions":{"fileAccessPermissions":[{"path":"/fixture","allow":true}],`}}, want: Unknown},
		{name: "grant scope", source: [][2]string{{`"allow":true,"action":"read_file"`, `"allow":true,"scope":"PERMISSION_SCOPE_ONCE","action":"read_file"`}}, want: Unknown},
		{name: "request trigger", source: [][2]string{{`"actionDescription":"Viewing notes"}`, `"actionDescription":"Viewing notes","triggerSource":{"type":"TYPE_FORCE_ASK_HOOK"}}`}}, want: Unknown},
		{name: "request reason", source: [][2]string{{`"actionDescription":"Viewing notes"}`, `"actionDescription":"Viewing notes","reason":"fixture"}`}}, want: Unknown},
		{name: "request of another kind", source: [][2]string{{`"request":` + viewPermRequest, `"request":{"elicitation":{"message":"fixture"}}`}}, want: Unknown},
		{name: "answer timed out", source: [][2]string{{`"stepIndex":2,`, `"stepIndex":2,"timedOut":true,`}}, want: Unknown},
		{name: "answer edited target", source: [][2]string{{`"permission":{"allow":true,"persistGrants":{}}`, `"permission":{"allow":true,"editedTarget":"/other","persistGrants":{}}`}}, want: Unknown},
		{name: "persisted grant", source: [][2]string{{`"persistGrants":{}`, `"persistGrants":{"allow":["/fixture"]}`}}, want: Unknown},
		{name: "second grant", source: [][2]string{{`"permissionGrants":[` + viewPermGrant, `"permissionGrants":[` + viewPermGrant + `,` + viewPermGrant}}, want: Unknown},
		{name: "second interaction", source: [][2]string{{`"response":` + viewPermResponse + `}]`, `"response":` + viewPermResponse + `},{"request":` + viewPermRequest + `,"response":` + viewPermResponse + `}]`}}, want: Unknown},
		{name: "interaction without a request", source: [][2]string{{`"request":` + viewPermRequest + `,`, ``}}, want: Unknown},
		{name: "interaction without an answer", source: [][2]string{{`,"response":` + viewPermResponse, ``}}, want: Unknown},
		{name: "grant without a resource", source: [][2]string{{`{"resource":{"action":"read_file","target":"/fixture/notes.md"},"allow":true`, `{"allow":true`}}, want: Unknown},
	} {
		t.Run(m.name, m.run)
	}
}

// Changed values fail: the record must be the approved read of the file the
// call named, and the wrapper's copy must equal the presented native Step's.
func TestProjectionViewPermissionChangedValues(t *testing.T) {
	for _, m := range []viewPermMutation{
		{name: "rejected request", source: [][2]string{{`"permission":{"allow":true,"persistGrants":{}}`, `"permission":{"persistGrants":{}}`}}, want: Unknown},
		{name: "denying grant", source: [][2]string{{`"allow":true,"action":"read_file"`, `"action":"read_file"`}}, want: Unknown},
		{name: "grant for another action", source: [][2]string{{`"allow":true,"action":"read_file"`, `"allow":true,"action":"command"`}}, want: Unknown},
		{name: "grant for another file", source: [][2]string{{`"allow":true,"action":"read_file","target":"/fixture/notes.md"`, `"allow":true,"action":"read_file","target":"/fixture/other.md"`}}, want: Unknown},
		{name: "grant resource for another file", source: [][2]string{{`{"resource":{"action":"read_file","target":"/fixture/notes.md"},"allow"`, `{"resource":{"action":"read_file","target":"/fixture/other.md"},"allow"`}}, want: Unknown},
		{name: "request for another file", source: [][2]string{{`"permission":{"resource":{"action":"read_file","target":"/fixture/notes.md"}`, `"permission":{"resource":{"action":"read_file","target":"/fixture/other.md"}`}}, want: Unknown},
		// The wrapper's copy, which the presentation drops, must equal the native Step's.
		{name: "wrapper grant differs", wrapper: [][2]string{{`"allow":true,"action":"read_file","target":"/fixture/notes.md"`, `"allow":true,"action":"read_file","target":"/fixture/other.md"`}}, want: Unknown},
		{name: "wrapper denies", wrapper: [][2]string{{`"allow":true,"action":"read_file"`, `"action":"read_file"`}}, want: Unknown},
		{name: "wrapper interaction answer differs", wrapper: [][2]string{{`"stepIndex":2`, `"stepIndex":3`}}, want: Unknown},
		{name: "wrapper interaction request differs", wrapper: [][2]string{{`"actionDescription":"Viewing notes"`, `"actionDescription":"Other"`}}, want: Unknown},
		{name: "native grant differs", native: [][2]string{{`"allow":true,"action":"read_file","target":"/fixture/notes.md"`, `"allow":true,"action":"read_file","target":"/fixture/other.md"`}}, want: Unknown},
		{name: "native interaction differs", native: [][2]string{{`"stepIndex":2`, `"stepIndex":3`}}, want: Unknown},
		{name: "wrapper without permissions", wrapper: [][2]string{{`"permissions":{"permissionGrants":[` + viewPermGrant + `]},`, ``}}, want: Unknown},
		{name: "wrapper without interactions", wrapper: [][2]string{{`,"completedInteractions":[{"request":` + viewPermRequest + `,"response":` + viewPermResponse + `}]`, ``}}, want: Unknown},
		{name: "wrapper without a record", wrapper: [][2]string{{viewPermRecord + `,`, ``}}, want: Unknown},
		{name: "native step without a record", native: [][2]string{{`,` + viewPermRecord, ``}}, want: Unknown},
		{name: "outer canceled", source: [][2]string{{`"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE"`, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_CANCELED"`}}, want: Unknown},
		// A response that differs from the source is incomplete.
		{name: "presented answer changed", shown: [][2]string{{`"permission":{"allow":true,"persistGrants":{}}`, `"permission":{"persistGrants":{}}`}}, want: Incomplete},
		{name: "presented grant changed", shown: [][2]string{{`"allow":true,"action":"read_file","target":"/fixture/notes.md"`, `"allow":true,"action":"read_file","target":"/fixture/other.md"`}}, want: Incomplete},
		{name: "presented record dropped", shown: [][2]string{{`,` + viewPermRecord, ``}}, want: Incomplete},
		{name: "raw wrapper grant changed", raw: [][2]string{{`"allow":true,"action":"read_file","target":"/fixture/notes.md"`, `"allow":true,"action":"read_file","target":"/fixture/other.md"`}}, want: Incomplete},
		{name: "raw wrapper answer changed", raw: [][2]string{{`"stepIndex":2`, `"stepIndex":3`}}, want: Incomplete},
	} {
		t.Run(m.name, m.run)
	}
}
