package completeness

import "testing"

// This fixture reproduces the structure of the payload-free list_permissions
// step found in nine retained sessions (agy-reader#jgfc): a generic wrapper
// with status DONE, no arguments, and a result of only text and render
// information. Frozen source, full RPC and default RPC copies showed the
// default response presenting the step unchanged, so the expected presentation
// is the fixture's own raw copy. Values are synthetic.
const listPermissionsRaw = `{"cascadeId":"permissions-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-permissions","name":"list_permissions","argumentsJson":"{\"toolAction\":\"Listing permissions\",\"toolSummary\":\"List permissions\"}"},"completedAt":"2026-01-01T00:00:01Z"},"generic":{"result":{"result":"Your current permission grants (listed in the order they are checked) are:\n- command(*): allowed\n- read_file(/fixture/scratch): allowed\n- write_file(/fixture/scratch): denied","stepRenderInfo":{"title":"permissions","titlePrefix":"Listed","icon":"shield","aggregationKey":"permissions"}}}}]}`

func TestProjectionListPermissionsQualified(t *testing.T) {
	require1216(t, snapshot1216(t, listPermissionsRaw), listPermissionsRaw, listPermissionsRaw, Verified)
}

// A changed value or status in either response is incomplete: the daemon
// presents the step unchanged, and full raw equality runs first.
func TestProjectionListPermissionsResponseChanges(t *testing.T) {
	s := snapshot1216(t, listPermissionsRaw)
	for name, change := range map[string][2]string{
		"status":         {`"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_CANCELED"`},
		"tool identity":  {`"name":"list_permissions"`, `"name":"unknown_permissions"`},
		"result text":    {`write_file(/fixture/scratch): denied`, `write_file(/fixture/scratch): allowed`},
		"render info":    {`"titlePrefix":"Listed"`, `"titlePrefix":"Showed"`},
		"call time":      {`2026-01-01T00:00:01Z`, `2026-01-01T00:00:09Z`},
		"added argument": {`"generic":{`, `"generic":{"args":{"Path":"/fixture"},`},
	} {
		t.Run(name, func(t *testing.T) {
			changed := replace1216(t, listPermissionsRaw, change[0], change[1])
			require1216(t, s, listPermissionsRaw, changed, Incomplete)
			require1216(t, s, changed, listPermissionsRaw, Incomplete)
		})
	}
}

// A source mutation applies to the persisted trajectory and both RPC copies, so
// only the projection rules can refuse it.
func TestProjectionListPermissionsUnsupportedShapes(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"argument", `"generic":{`, `"generic":{"args":{"Path":"/fixture"},`},
		{"presented tool argument", `"generic":{`, `"generic":{"args":{"toolAction":"Listing permissions"},`},
		{"payload", `"result":{"result":`, `"result":{"payload":{"@type":"type.googleapis.com/gemini_coder.Step"},"result":`},
		{"unlisted result field metadata", `"result":{"result":`, `"result":{"metadata":{"key":"value"},"result":`},
		{"output uri", `"result":{"result":`, `"result":{"fullOutputUri":"file:///fixture/steps/1/output.txt","result":`},
		{"media", `"result":{"result":`, `"result":{"media":[{"mimeType":"image/png","uri":"file:///fixture/extra.png"}],"result":`},
		{"task details", `"generic":{`, `"taskDetails":{"id":"task-1"},"generic":{`},
		{"changed status canceled", `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_CANCELED"`},
		{"changed status running", `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_RUNNING"`},
		{"error", `"status":"CORTEX_STEP_STATUS_DONE","metadata"`, `"status":"CORTEX_STEP_STATUS_ERROR","error":{"shortError":"failed"},"metadata"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := replace1216(t, listPermissionsRaw, tc.before, tc.after)
			require1216(t, snapshot1216(t, raw), raw, raw, Unknown)
		})
	}
}
