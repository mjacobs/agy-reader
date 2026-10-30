package completeness

import (
	"strings"
	"testing"
)

const failedEditMeta = `"metadata":{"toolCall":{"name":"edit_file","argumentsJson":"{\"Path\":\"/fixture/code.go\",\"Replacement\":\"new\",\"Target\":\"old\"}"},"completedAt":"2026-01-01T00:00:02Z"}`
const failedEditError = `"error":{"shortError":"target content not found in the file; the Target must exactly match existing file text, including whitespace","fullError":"target content not found in the file; the Target must exactly match existing file text, including whitespace"}`
const failedEditShown = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_CODE_ACTION","status":"CORTEX_STEP_STATUS_ERROR",` + failedEditMeta + `,` + failedEditError + `,"codeAction":{"actionSpec":{"command":{"file":{"absoluteUri":"file:///fixture/code.go"},"isEdit":true}}}}]}`
const failedEditRawClean = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR",` + failedEditMeta + `,` + failedEditError + `,"generic":{"args":{"Path":"/fixture/code.go","Replacement":"new","Target":"old"},"result":{"stepRenderInfo":{"titlePrefix":"Editing","icon":"edit"}}}}]}`

func TestFailedEditFile(t *testing.T) {
	s := snapshot1216(t, failedEditRawClean)
	require1216(t, s, failedEditRawClean, failedEditShown, Verified)
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, failedEditRawClean, string(offline), Verified)

	// Altering preserved error or action spec must fail verification.
	changed := strings.Replace(failedEditShown, `"isEdit":true`, `"isEdit":false`, 1)
	require1216(t, s, failedEditRawClean, changed, Incomplete)
}

func TestFailedEditFileRefusesUnqualifiedShapes(t *testing.T) {
	for _, tc := range []struct{ before, after string }{
		{`CORTEX_STEP_STATUS_ERROR`, `CORTEX_STEP_STATUS_DONE`},
		{`"name":"edit_file"`, `"name":"write_file"`},
		{`"Path":"/fixture/code.go"`, `"Path":"relative/code.go"`},
		{`"Target":"old"`, `"Target":"other"`},
		{`"args":{`, `"args":{"Unexpected":"arg",`},
		{`"stepRenderInfo":{"titlePrefix":"Editing"`, `"result":"unexpected text","stepRenderInfo":{"titlePrefix":"Editing"`},
		{`"fullError":"target content`, `"fullError":"different error`},
	} {
		bad := replace1216(t, failedEditRawClean, tc.before, tc.after)
		if _, err := snapshot1216(t, bad).OfflineJSON(); err == nil {
			t.Fatalf("accepted unqualified failure with %s -> %s", tc.before, tc.after)
		}
	}
}
