package completeness

import "testing"

// This fixture reproduces the structure of the payload-free ask_permission
// step found in one retained session (agy-reader#mbpm, steps 24 and 36): a
// generic wrapper with status DONE, exactly the Action, Reason and Target
// arguments, and a result of only text and render information. Frozen source,
// full RPC and default RPC copies showed the default response presenting the
// step unchanged, so the expected presentation is the fixture's own raw copy.
// Values are synthetic.
const askPermissionRaw = `{"cascadeId":"ask-permission-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-ask-permission","name":"ask_permission","argumentsJson":"{\"Action\":\"read_file\",\"Reason\":\"Read the fixture configuration\",\"Target\":\"/fixture/config.json\",\"toolAction\":\"Requesting permission to read config.json\",\"toolSummary\":\"Asking permission for read_file\"}"},"completedAt":"2026-01-01T00:00:01Z"},"generic":{"args":{"Action":"read_file","Reason":"Read the fixture configuration","Target":"/fixture/config.json"},"result":{"result":"Permission for read_file(/fixture/config.json) was granted. Reason provided by agent: Read the fixture configuration","stepRenderInfo":{"title":"read /fixture/config.json","titlePrefix":"Granted permission to","icon":"shield","aggregationKey":"permissions"}}}}]}`

func TestProjectionAskPermissionQualified(t *testing.T) {
	require1216(t, snapshot1216(t, askPermissionRaw), askPermissionRaw, askPermissionRaw, Verified)
}

// A changed value or status in either response is incomplete: the daemon
// presents the step unchanged, and full raw equality runs first.
func TestProjectionAskPermissionResponseChanges(t *testing.T) {
	s := snapshot1216(t, askPermissionRaw)
	for name, change := range map[string][2]string{
		"status":           {`"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_CANCELED"`},
		"tool identity":    {`"name":"ask_permission"`, `"name":"unknown_permission"`},
		"result text":      {`was granted.`, `was denied.`},
		"render info":      {`"titlePrefix":"Granted permission to"`, `"titlePrefix":"Denied permission to"`},
		"call time":        {`2026-01-01T00:00:01Z`, `2026-01-01T00:00:09Z`},
		"argument value":   {`"Action":"read_file","Reason"`, `"Action":"write_file","Reason"`},
		"added argument":   {`"generic":{"args":{`, `"generic":{"args":{"Path":"/fixture",`},
		"removed argument": {`"Reason":"Read the fixture configuration",`, ``},
	} {
		t.Run(name, func(t *testing.T) {
			changed := replace1216(t, askPermissionRaw, change[0], change[1])
			require1216(t, s, askPermissionRaw, changed, Incomplete)
			require1216(t, s, changed, askPermissionRaw, Incomplete)
		})
	}
}

// A source mutation applies to the persisted trajectory and both RPC copies, so
// only the projection rules can refuse it.
func TestProjectionAskPermissionUnsupportedShapes(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"extra argument", `"generic":{"args":{`, `"generic":{"args":{"Path":"/fixture",`},
		{"presented tool argument", `"generic":{"args":{`, `"generic":{"args":{"toolAction":"Requesting permission",`},
		{"missing argument", `"Reason":"Read the fixture configuration",`, ``},
		{"no arguments", `"args":{"Action":"read_file","Reason":"Read the fixture configuration","Target":"/fixture/config.json"},`, ``},
		{"renamed argument", `"Target":"/fixture/config.json"}`, `"Path":"/fixture/config.json"}`},
		{"payload", `"result":{"result":`, `"result":{"payload":{"@type":"type.googleapis.com/gemini_coder.Step"},"result":`},
		{"unlisted result field metadata", `"result":{"result":`, `"result":{"metadata":{"key":"value"},"result":`},
		{"output uri", `"result":{"result":`, `"result":{"fullOutputUri":"file:///fixture/steps/1/output.txt","result":`},
		{"media", `"result":{"result":`, `"result":{"media":[{"mimeType":"image/png","uri":"file:///fixture/extra.png"}],"result":`},
		{"task details", `"generic":{`, `"taskDetails":{"id":"task-1"},"generic":{`},
		{"outer permissions", `"generic":{`, `"permissions":{"permissionGrants":[{"allow":true,"action":"read_file"}]},"generic":{`},
		{"completed interactions", `"generic":{`, `"completedInteractions":[{"response":{}}],"generic":{`},
		{"changed status canceled", `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_CANCELED"`},
		{"changed status running", `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_RUNNING"`},
		{"changed status error", `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_ERROR"`},
		{"error", `"status":"CORTEX_STEP_STATUS_DONE","metadata"`, `"status":"CORTEX_STEP_STATUS_ERROR","error":{"shortError":"failed"},"metadata"`},
		{"error with done status", `"status":"CORTEX_STEP_STATUS_DONE","metadata"`, `"status":"CORTEX_STEP_STATUS_DONE","error":{"shortError":"failed"},"metadata"`},
		{"other tool", `"name":"ask_permission"`, `"name":"grant_permission"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := replace1216(t, askPermissionRaw, tc.before, tc.after)
			require1216(t, snapshot1216(t, raw), raw, raw, Unknown)
		})
	}
}
