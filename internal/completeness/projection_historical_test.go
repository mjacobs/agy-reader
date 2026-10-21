package completeness

import (
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func invalidHistoricalFixture(tool string) string {
	args := map[string]any{"CommandLine": "echo fixture", "Cwd": "/fixture", "WaitMsBeforeAsync": 5000, "IsDaemon": false, "RunPersistent": false, "toolAction": "Fixture action"}
	if tool == "view_file" {
		args = map[string]any{"AbsolutePath": "/fixture/notes.txt", "StartLine": 1, "EndLine": 60, "toolAction": "Fixture action"}
	}
	called, _ := json.Marshal(args)
	wrapper := map[string]string{}
	for key, value := range args {
		if text, ok := value.(string); ok {
			wrapper[key] = text
		} else {
			b, _ := json.Marshal(value)
			wrapper[key] = string(b)
		}
	}
	encoded, _ := json.Marshal(wrapper)
	quotedCall, _ := json.Marshal(string(called))
	return `{"cascadeId":"historical-invalid","steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_INVALID","metadata":{"toolCall":{"name":"` + tool + `","argumentsJson":` + string(quotedCall) + `},"toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN","internalMetadata":{"statusTransitions":[{"updatedStatus":"CORTEX_STEP_STATUS_PENDING"},{"updatedStatus":"CORTEX_STEP_STATUS_INVALID"}]}},"error":{"shortError":"invalid arguments:\n- missing property 'toolSummary'","fullError":"invalid arguments:\n- missing property 'toolSummary'"},"generic":{"args":` + string(encoded) + `}}]}`
}

func TestProjectionHistoricalInvalidCalls(t *testing.T) {
	for _, tool := range []string{"run_command", "view_file"} {
		t.Run(tool, func(t *testing.T) {
			raw := invalidHistoricalFixture(tool)
			s := snapshot1216(t, raw)
			require1216(t, s, raw, raw, Verified)
			for name, change := range map[string][2]string{
				"other status":          {`"status":"CORTEX_STEP_STATUS_INVALID"`, `"status":"CORTEX_STEP_STATUS_RUNNING"`},
				"other error":           {`missing property 'toolSummary'`, `missing property 'Future'`},
				"extra argument":        {`"args":{`, `"args":{"Future":"value",`},
				"summary present":       {`"args":{`, `"args":{"toolSummary":"present",`},
				"result present":        {`"generic":{"args":`, `"generic":{"result":{"result":"unexpected"},"args":`},
				"permission record":     {`"generic":{`, `"permissions":{"permissionGrants":[{"allow":true}]},"generic":{`},
				"interaction record":    {`"generic":{`, `"completedInteractions":[{}],"generic":{`},
				"requested interaction": {`"generic":{`, `"requestedInteraction":{"permission":{}},"generic":{`},
				"task details":          {`"generic":{`, `"taskDetails":{"id":"unexpected"},"generic":{`},
				"other tool source":     {`CORTEX_COMPONENT_SOURCE_BUILTIN`, `CORTEX_COMPONENT_SOURCE_CUSTOMIZATION`},
				"other history":         {`CORTEX_STEP_STATUS_PENDING`, `CORTEX_STEP_STATUS_RUNNING`},
				"tool call disagrees":   {`\"toolAction\":\"Fixture action\"`, `\"toolAction\":\"Other action\"`},
			} {
				t.Run(name, func(t *testing.T) {
					bad := replace1216(t, raw, change[0], change[1])
					require1216(t, snapshot1216(t, bad), bad, raw, Unknown)
				})
			}
			for name, change := range map[string][2]string{
				"retained argument": {`"toolAction":"Fixture action"`, `"toolAction":"Changed action"`},
				"retained error":    {`missing property 'toolSummary'`, `changed error`},
				"retained metadata": {`CORTEX_COMPONENT_SOURCE_BUILTIN`, `CORTEX_COMPONENT_SOURCE_CUSTOMIZATION`},
			} {
				t.Run(name, func(t *testing.T) {
					changed := replace1216(t, raw, change[0], change[1])
					require1216(t, s, changed, raw, Incomplete)
					require1216(t, s, raw, changed, Incomplete)
				})
			}
		})
	}
}

func TestProjectionHistoricalFailedListResources(t *testing.T) {
	raw, shown := nativeToolFixture(nativeToolCase{tool: "list_resources", nativeType: "CORTEX_STEP_TYPE_LIST_RESOURCES", field: "listResources", arguments: `{"ServerName":"fixture-server"}`, body: `{"serverName":"fixture-server"}`, failure: "fixture server unavailable", resultFile: true})
	s := snapshot1216(t, raw)
	require1216(t, s, raw, shown, Verified)
	for name, change := range map[string][2]string{
		"changed outer error":   {`"shortError":"fixture server unavailable","fullError"`, `"shortError":"different error","fullError"`},
		"successful wrapper":    {`"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_DONE"`},
		"extra argument":        {`"args":{`, `"args":{"Future":"value",`},
		"changed server":        {`"listResources":{"serverName":"fixture-server"}`, `"listResources":{"serverName":"other-server"}`},
		"permission record":     {`"generic":{`, `"permissions":{"permissionGrants":[{"allow":true}]},"generic":{`},
		"interaction record":    {`"generic":{`, `"completedInteractions":[{}],"generic":{`},
		"requested interaction": {`"generic":{`, `"requestedInteraction":{"permission":{}},"generic":{`},
		"task details":          {`"generic":{`, `"taskDetails":{"id":"unexpected"},"generic":{`},
		"other wrapper tool":    {`"name":"list_resources"`, `"name":"call_mcp_tool"`},
		"unobserved rendering":  {`"result":{"result":`, `"result":{"stepRenderInfo":{"title":"unexpected"},"result":`},
	} {
		t.Run(name, func(t *testing.T) {
			bad := replace1216(t, raw, change[0], change[1])
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
	require1216(t, s, replace1216(t, raw, "Fixture result text", "changed dropped result"), shown, Incomplete)
	require1216(t, s, raw, replace1216(t, shown, "fixture server unavailable", "changed error"), Incomplete)
}

func renderFailedHistoricalFixture() (string, string) {
	call := `"toolCall":{"name":"run_command","argumentsJson":"{\"CommandLine\":\"echo fixture\",\"Cwd\":\"/fixture\",\"WaitMsBeforeAsync\":5000,\"toolAction\":\"Run fixture\",\"toolSummary\":\"Fixture command\"}"},"toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN","preToolHookResults":[{"decision":"allow"}]`
	history := `{"updatedStatus":"CORTEX_STEP_STATUS_PENDING"},{"updatedStatus":"CORTEX_STEP_STATUS_RUNNING"}`
	nativeMeta := `"metadata":{` + call + `,"internalMetadata":{"statusTransitions":[` + history + `]}}`
	outerMeta := `"metadata":{` + call + `,"internalMetadata":{"statusTransitions":[` + history + `,{"updatedStatus":"CORTEX_STEP_STATUS_ERROR"}]}}`
	run := `"runCommand":{"commandLine":"echo fixture","proposedCommandLine":"echo fixture","cwd":"/fixture","exitCode":0,"blocking":true,"waitMsBeforeAsync":"5000","combinedOutput":{"full":"fixture output"},"shellName":"bash","ranInSandbox":true}`
	native := `"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_DONE",` + nativeMeta + `,` + run
	raw := `{"cascadeId":"render-failed-fixture","steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR",` + outerMeta + `,"error":{"shortError":"` + commandRenderErrorPrefix + `fixture template","fullError":"` + commandRenderErrorPrefix + `fixture template"},"generic":{"args":{"CommandLine":"echo fixture","Cwd":"/fixture","WaitMsBeforeAsync":"5000","toolAction":"Run fixture","toolSummary":"Fixture command"},"result":{"payload":{"@type":"type.googleapis.com/gemini_coder.Step",` + native + `}}}}]}`
	shown := `{"cascadeId":"render-failed-fixture","steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_ERROR",` + nativeMeta + `,` + run + `}]}`
	return raw, shown
}

func TestProjectionHistoricalCommandRenderFailure(t *testing.T) {
	raw, shown := renderFailedHistoricalFixture()
	s := snapshot1216(t, raw)
	require1216(t, s, raw, shown, Verified)
	for name, change := range map[string][2]string{
		"wrong error class":            {commandRenderErrorPrefix, "unqualified adapter failure: "},
		"unequal full error":           {`"fullError":"` + commandRenderErrorPrefix, `"fullError":"different: `},
		"outer status":                 {`"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_CANCELED"`},
		"inner status":                 {`"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_RUNNING"`},
		"native error":                 {`"runCommand":{`, `"error":{"shortError":"unexpected"},"runCommand":{`},
		"result text":                  {`"result":{"payload":`, `"result":{"result":"unexpected","payload":`},
		"result render info":           {`"result":{"payload":`, `"result":{"stepRenderInfo":{"title":"unexpected"},"payload":`},
		"result output URI":            {`"result":{"payload":`, `"result":{"fullOutputUri":"file:///fixture/output","payload":`},
		"result metadata":              {`"result":{"payload":`, `"result":{"metadata":{"new":"unexpected"},"payload":`},
		"result media":                 {`"result":{"payload":`, `"result":{"media":[{"uri":"file:///fixture/image"}],"payload":`},
		"extra argument":               {`"args":{`, `"args":{"Future":"value",`},
		"command disagreement":         {`"commandLine":"echo fixture"`, `"commandLine":"echo changed"`},
		"proposed disagreement":        {`"proposedCommandLine":"echo fixture"`, `"proposedCommandLine":"echo changed"`},
		"cwd disagreement":             {`"cwd":"/fixture"`, `"cwd":"/other"`},
		"failed command":               {`"exitCode":0`, `"exitCode":1`},
		"no exit code":                 {`"exitCode":0,`, ``},
		"another wait":                 {`"waitMsBeforeAsync":"5000"`, `"waitMsBeforeAsync":"10000"`},
		"background command":           {`"blocking":true`, `"blocking":false`},
		"other shell":                  {`"shellName":"bash"`, `"shellName":"zsh"`},
		"without sandbox":              {`"ranInSandbox":true`, `"ranInSandbox":false`},
		"hook denial":                  {`"decision":"allow"`, `"decision":"deny"`},
		"other history":                {`CORTEX_STEP_STATUS_PENDING`, `CORTEX_STEP_STATUS_WAITING`},
		"permission record":            {`"generic":{`, `"permissions":{"permissionGrants":[{"allow":true}]},"generic":{`},
		"native permission":            {`"runCommand":{`, `"permissions":{"permissionGrants":[{"allow":true}]},"runCommand":{`},
		"interaction record":           {`"generic":{`, `"completedInteractions":[{}],"generic":{`},
		"native interaction":           {`"runCommand":{`, `"completedInteractions":[{}],"runCommand":{`},
		"requested interaction":        {`"generic":{`, `"requestedInteraction":{"permission":{}},"generic":{`},
		"native requested interaction": {`"runCommand":{`, `"requestedInteraction":{"permission":{}},"runCommand":{`},
		"task details":                 {`"generic":{`, `"taskDetails":{"id":"unexpected"},"generic":{`},
		"native task details":          {`"runCommand":{`, `"taskDetails":{"id":"unexpected"},"runCommand":{`},
		"new native output field":      {`"combinedOutput":{"full":`, `"combinedOutput":{"truncated":"unexpected","full":`},
	} {
		t.Run(name, func(t *testing.T) {
			bad := replace1216(t, raw, change[0], change[1])
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
	for name, change := range map[string][2]string{
		"output":   {"fixture output", "changed output"},
		"metadata": {`"toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN"`, `"toolSource":"CORTEX_COMPONENT_SOURCE_CUSTOMIZATION"`},
		"status":   {`"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_DONE"`},
	} {
		t.Run("presentation "+name, func(t *testing.T) {
			require1216(t, s, raw, replace1216(t, shown, change[0], change[1]), Incomplete)
		})
	}
	require1216(t, s, strings.ReplaceAll(raw, "fixture template", "other dropped error"), shown, Incomplete)
	changedOuterMetadata := strings.Replace(raw, `"metadata":{`, `"metadata":{"completedAt":"2026-01-01T00:00:01Z",`, 1)
	require1216(t, s, changedOuterMetadata, shown, Incomplete)

	// Unknown bytes in the nested Any remain a source refusal, even though the
	// qualified wrapper drops its own formatting error during presentation.
	m := trajectory1216(t, raw)
	step := m.ProtoReflect().Get(m.ProtoReflect().Descriptor().Fields().ByName("steps")).List().Get(0).Message()
	generic := step.Get(step.Descriptor().Fields().ByName("generic")).Message()
	result := generic.Get(generic.Descriptor().Fields().ByName("result")).Message()
	payload := result.Get(result.Descriptor().Fields().ByName("payload")).Message()
	value := payload.Descriptor().Fields().ByName("value")
	nested, err := newMessage("gemini_coder.Step")
	if err != nil {
		t.Fatal(err)
	}
	if err = proto.Unmarshal(payload.Get(value).Bytes(), nested); err != nil {
		t.Fatal(err)
	}
	nested.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 99999, protowire.VarintType), 1))
	b, err := proto.Marshal(nested)
	if err != nil {
		t.Fatal(err)
	}
	payload.Set(value, protoreflect.ValueOfBytes(b))
	if _, err = finishSnapshot(m); err == nil {
		t.Fatal("unknown nested payload accepted")
	}
}

func TestProjectionHistoricalCommandRenderDispatch(t *testing.T) {
	raw, shown := renderFailedHistoricalFixture()
	for name, change := range map[string][2]string{
		"unrelated outer error": {commandRenderErrorPrefix, "unrelated failure: "},
		"successful wrapper":    {`"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_DONE"`},
		"other wrapper tool":    {`"name":"run_command"`, `"name":"other_tool"`},
	} {
		t.Run(name, func(t *testing.T) {
			bad := replace1216(t, raw, change[0], change[1])
			s := snapshot1216(t, bad)
			require1216(t, s, bad, shown, Unknown)
			if _, err := project(s.trajectory); err == nil || err.Error() != "unsupported generic wrapper" {
				t.Fatalf("expected ordinary wrapper refusal, got %v", err)
			}
		})
	}
}
