package completeness

import (
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const finish147Meta = `"metadata":{"toolCall":{"name":"finish","argumentsJson":"{\"summary\":\"fixture\",\"count\":2}"},"completedAt":"2026-01-01T00:00:02Z"}`
const finish147Text = `"{\"summary\":\"fixture\",\"count\":2}"`
const finish147Raw = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE",` + finish147Meta + `,"generic":{"args":{"summary":"fixture","count":"2"},"result":{"result":"fixture result","fullOutputUri":"file:///fixture/output.txt","stepRenderInfo":{"hidden":true,"titlePrefix":"Finished"},"payload":{"@type":"type.googleapis.com/exa.cortex.core.tools.Finish","argumentsJson":` + finish147Text + `,"hasResultSchema":true}}}}]}`
const finish147Shown = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_FINISH","status":"CORTEX_STEP_STATUS_DONE",` + finish147Meta + `,"finish":{"output":{"arguments_json":` + finish147Text + `},"outputString":` + finish147Text + `}}]}`
const missingRead147Meta = `"metadata":{"toolCall":{"name":"read_file","argumentsJson":"{\"Path\":\"/fixture/missing.md\"}"}}`
const missingRead147Error = `"error":{"shortError":"file does not exist","fullError":"file does not exist"}`
const missingRead147Raw = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR",` + missingRead147Meta + `,` + missingRead147Error + `,"generic":{"args":{"Path":"/fixture/missing.md"},"result":{"stepRenderInfo":{"titlePrefix":"Analyzing","icon":"description"}}}}]}`
const missingRead147Shown = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_VIEW_FILE","status":"CORTEX_STEP_STATUS_ERROR",` + missingRead147Meta + `,` + missingRead147Error + `,"viewFile":{"absolutePathUri":"file:///fixture/missing.md"}}]}`

func TestFinish147AndMissingRead(t *testing.T) {
	for _, pair := range [][2]string{{finish147Raw, finish147Shown}, {missingRead147Raw, missingRead147Shown}} {
		s := snapshot1216(t, pair[0])
		require1216(t, s, pair[0], pair[1], Verified)
		offline, err := s.OfflineJSON()
		if err != nil {
			t.Fatal(err)
		}
		require1216(t, s, pair[0], string(offline), Verified)
		// Changes to either a preserved field or dropped render data remain visible.
		changed := strings.Replace(pair[1], "fixture-parent", "other-parent", 1)
		require1216(t, s, pair[0], changed, Incomplete)
		changed = strings.Replace(pair[0], `"titlePrefix":"`, `"titlePrefix":"changed `, 1)
		require1216(t, s, changed, pair[1], Incomplete)
	}
	s := snapshot1216(t, finish147Raw)
	for _, key := range []string{"output", "outputString"} {
		var shown map[string]any
		if err := json.Unmarshal([]byte(finish147Shown), &shown); err != nil {
			t.Fatal(err)
		}
		delete(shown["steps"].([]any)[0].(map[string]any)["finish"].(map[string]any), key)
		data, _ := json.Marshal(shown)
		require1216(t, s, finish147Raw, string(data), Incomplete)
	}
}

func TestFinish147RefusesUnqualifiedShapes(t *testing.T) {
	for _, tc := range []struct{ raw, before, after string }{
		{finish147Raw, `"hasResultSchema":true`, `"hasResultSchema":false`},
		{finish147Raw, `"name":"finish"`, `"name":"other"`},
		{finish147Raw, `"count":"2"`, `"count":"3"`},
		{finish147Raw, `CORTEX_STEP_STATUS_DONE`, `CORTEX_STEP_STATUS_ERROR`},
		{finish147Raw, `"generic":`, `"error":{"shortError":"failed"},"generic":`},
		{finish147Raw, `"generic":`, `"taskDetails":{"id":"task"},"generic":`},
		{finish147Raw, `"result":"fixture result"`, `"metadata":{"future":"value"},"result":"fixture result"`},
		{missingRead147Raw, `"shortError":"file does not exist"`, `"shortError":"permission denied"`},
		{missingRead147Raw, `file does not exist`, ``},
		{missingRead147Raw, `"Path":"/fixture/missing.md"`, `"Path":"/fixture/other.md"`},
		{missingRead147Raw, `missing.md`, `SKILL.md`},
		{missingRead147Raw, `CORTEX_STEP_STATUS_ERROR`, `CORTEX_STEP_STATUS_DONE`},
		{missingRead147Raw, `"result":{`, `"result":{"result":"new output",`},
		{missingRead147Raw, `"args":{`, `"args":{"StartLine":"1",`},
		{missingRead147Raw, `"generic":`, `"permissions":{},"generic":`},
	} {
		bad := strings.ReplaceAll(tc.raw, tc.before, tc.after)
		if bad == tc.raw {
			t.Fatal("mutation missed")
		}
		s := snapshot1216(t, bad)
		if data, err := s.OfflineJSON(); err == nil || data != nil {
			t.Fatalf("accepted %s -> %s", tc.before, tc.after)
		}
	}
}

func TestFinish147RejectsUnknownWire(t *testing.T) {
	m := trajectory1216(t, finish147Raw).ProtoReflect()
	step := tool143Get(m, "steps").List().Get(0).Message()
	payload := tool143Get(tool143Get(tool143Get(step, "generic").Message(), "result").Message(), "payload").Message()
	data := append([]byte(nil), tool143Get(payload, "value").Bytes()...)
	data = protowire.AppendTag(data, 9999, protowire.VarintType)
	data = protowire.AppendVarint(data, 1)
	tool143Set(payload, "value", protoreflect.ValueOfBytes(data))
	if _, err := finishSnapshot(m.Interface()); err == nil {
		t.Fatal("unknown Finish field accepted")
	}
	tool143Set(payload, "type_url", protoreflect.ValueOfString("type.googleapis.com/example.Future"))
	if _, err := finishSnapshot(m.Interface()); err == nil {
		t.Fatal("unknown Any accepted")
	}
}

func TestFinish147InvalidatesPreviousProfile(t *testing.T) {
	const previous = "sqlite-v1-format0-agy-1.3.4-5b93565c5f04-planner50-permission2-prompt23-tools143-permission1-tools144-skill1-history2-ide225-options145-compaction146-skill146"
	if Profile == previous {
		t.Fatal("Finish support did not change profile")
	}
	s := snapshot1216(t, finish147Raw)
	a, r := s.VerifyResponses([]byte(finish147Raw), []byte(finish147Shown))
	if r.State != Verified {
		t.Fatal(r)
	}
	a.Profile = previous
	if r := s.CheckAttestation([]byte(finish147Shown), a); r.State != Unknown {
		t.Fatalf("previous profile accepted: %+v", r)
	}
}
