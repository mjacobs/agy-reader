package completeness

import (
	"strings"
	"testing"
)

const legacyTaskInputRaw = `{"cascadeId":"fixture-parent","steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN","preToolHookResults":[{"decision":"allow"}],"toolCall":{"name":"manage_task","argumentsJson":"{\"Action\":\"send_input\",\"TaskId\":\"fixture-task\",\"Input\":\"fixture input\\n\"}"}},"generic":{"args":{"Action":"send_input","TaskId":"fixture-task","Input":"fixture input\n"},"result":{"result":"Input sent","fullOutputUri":"file:///fixture/input.txt","stepRenderInfo":{"title":"Task input"},"metadata":{"input":"fixture input\n","task_description":"fixture command","task_id":"fixture-task","tool_name":"run_command"}}}}]}`

func TestProjectionLegacyTaskInput(t *testing.T) {
	require1216(t, snapshot1216(t, legacyTaskInputRaw), legacyTaskInputRaw, legacyTaskInputRaw, Verified)
	for name, change := range map[string][2]string{
		"other action":           {`send_input`, `kill`},
		"other status":           {`CORTEX_STEP_STATUS_DONE`, `CORTEX_STEP_STATUS_ERROR`},
		"missing Input":          {`,"Input":"fixture input\n"`, ``},
		"missing metadata input": {`"input":"fixture input\n",`, ``},
		"mismatched input":       {`"input":"fixture input\n"`, `"input":"other"`},
		"mismatched task":        {`"task_id":"fixture-task"`, `"task_id":"other-task"`},
		"mismatched call":        {`\"Input\":\"fixture input\\n\"`, `\"Input\":\"other\"`},
		"other target tool":      {`"tool_name":"run_command"`, `"tool_name":"view_file"`},
		"other source":           {`CORTEX_COMPONENT_SOURCE_BUILTIN`, `CORTEX_COMPONENT_SOURCE_PROVIDER`},
		"denied hook":            {`"decision":"allow"`, `"decision":"deny"`},
		"missing hook":           {`"preToolHookResults":[{"decision":"allow"}],`, ``},
		"duplicate hook":         {`[{"decision":"allow"}]`, `[{"decision":"allow"},{"decision":"allow"}]`},
		"extra argument":         {`"args":{`, `"args":{"Future":"new",`},
		"extra result metadata":  {`"input":"fixture input\n"`, `"future":"new","input":"fixture input\n"`},
		"outer task":             {`"generic":{`, `"taskDetails":{"id":"other-task"},"generic":{`},
		"outer permission":       {`"generic":{`, `"permissions":{"fileAccessPermissions":[{"path":"/fixture/private","allow":true}]},"generic":{`},
		"outer error":            {`"generic":{`, `"error":{"shortError":"failed"},"generic":{`},
	} {
		t.Run(name, func(t *testing.T) {
			bad := replace1216(t, legacyTaskInputRaw, change[0], change[1])
			require1216(t, snapshot1216(t, bad), bad, bad, Unknown)
		})
	}
	for name, change := range map[string][2]string{
		"input":  {`"input":"fixture input\n"`, `"input":"other"`},
		"result": {`Input sent`, `Input lost`},
		"URI":    {`file:///fixture/input.txt`, `file:///fixture/other.txt`},
		"render": {`Task input`, `Other render`},
		"hook":   {`"decision":"allow"`, `"decision":"deny"`},
	} {
		t.Run("response "+name, func(t *testing.T) {
			s := snapshot1216(t, legacyTaskInputRaw)
			bad := replace1216(t, legacyTaskInputRaw, change[0], change[1])
			require1216(t, s, legacyTaskInputRaw, bad, Incomplete)
			require1216(t, s, bad, legacyTaskInputRaw, Incomplete)
		})
	}
	// Empty input is a distinct, unobserved operation even when repeated exactly.
	empty := strings.NewReplacer(`fixture input\n`, ``, `fixture input\\n`, ``).Replace(legacyTaskInputRaw)
	require1216(t, snapshot1216(t, empty), empty, empty, Unknown)
}
