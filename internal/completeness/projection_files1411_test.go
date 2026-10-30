package completeness

import (
	"strings"
	"testing"
)

// Synthetic values follow the source, full-RPC and default-RPC shapes captured
// from a native agy 1.4.11 session: 24 write_file calls over existing files
// (payload kind OVERWRITE) and one write_file refused because its file existed.
const overwrite1411Meta = `"metadata":{"toolCall":{"name":"write_file","argumentsJson":"{\"Content\":\"new\\n\",\"Overwrite\":true,\"Path\":\"/fixture/notes.txt\"}"},"completedAt":"2026-01-01T00:00:02Z","preToolHookResults":[{"decision":"allow"}]}`
const overwrite1411Diff = `"diff":{"endLine":1,"unifiedDiff":{"lines":[{"text":"old","type":"UNIFIED_DIFF_LINE_TYPE_DELETE"},{"text":"new","type":"UNIFIED_DIFF_LINE_TYPE_INSERT"}]}}`
const overwrite1411Raw = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE",` + overwrite1411Meta + `,"generic":{"args":{"Content":"new\n","Overwrite":"true","Path":"/fixture/notes.txt"},"result":{"result":"Created file.","fullOutputUri":"file:///fixture/output.txt","stepRenderInfo":{"titlePrefix":"Edited","icon":"edit"},"payload":{"@type":"type.googleapis.com/exa.cortex.core.tools.FileEdit","kind":"OVERWRITE","path":"/fixture/notes.txt",` + overwrite1411Diff + `,"diffStats":{"additions":1,"deletions":1}}}}}]}`
const overwrite1411Shown = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_CODE_ACTION","status":"CORTEX_STEP_STATUS_DONE",` + overwrite1411Meta + `,"codeAction":{"actionSpec":{"createFile":{"path":{"absoluteUri":"file:///fixture/notes.txt"},"overwrite":true}},"actionResult":{"edit":{` + overwrite1411Diff + `,"absoluteUri":"file:///fixture/notes.txt"}},"diffStats":{"additions":1,"deletions":1}}}]}`

const failedWrite1411Text = `/fixture/notes.txt already exists. To overwrite it, retry with Overwrite=true.`
const failedWrite1411Meta = `"metadata":{"toolCall":{"name":"write_file","argumentsJson":"{\"Content\":\"new\\n\",\"Path\":\"/fixture/notes.txt\"}"},"completedAt":"2026-01-01T00:00:02Z"},"error":{"shortError":"` + failedWrite1411Text + `","fullError":"` + failedWrite1411Text + `"}`
const failedWrite1411Raw = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR",` + failedWrite1411Meta + `,"generic":{"args":{"Content":"new\n","Path":"/fixture/notes.txt"},"result":{"stepRenderInfo":{"titlePrefix":"Writing","icon":"edit"}}}}]}`
const failedWrite1411Shown = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_CODE_ACTION","status":"CORTEX_STEP_STATUS_ERROR",` + failedWrite1411Meta + `,"codeAction":{"actionSpec":{"createFile":{"path":{"absoluteUri":"file:///fixture/notes.txt"}}}}}]}`

func TestFiles1411Qualified(t *testing.T) {
	for name, pair := range map[string][2]string{
		"overwrite":    {overwrite1411Raw, overwrite1411Shown},
		"failed write": {failedWrite1411Raw, failedWrite1411Shown},
	} {
		t.Run(name, func(t *testing.T) {
			s := snapshot1216(t, pair[0])
			require1216(t, s, pair[0], pair[1], Verified)
			offline, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, pair[0], string(offline), Verified)
		})
	}
}

// A changed presentation or full response must not verify.
func TestFiles1411Changes(t *testing.T) {
	for _, tc := range []struct {
		name, raw, shown, before, after string
	}{
		{"overwrite flag dropped", overwrite1411Raw, overwrite1411Shown, `,"overwrite":true`, ``},
		{"overwrite marked created", overwrite1411Raw, overwrite1411Shown, `"absoluteUri":"file:///fixture/notes.txt"}},`, `"absoluteUri":"file:///fixture/notes.txt","createFile":true}},`},
		{"overwrite diff changed", overwrite1411Raw, overwrite1411Shown, `"text":"new"`, `"text":"other"`},
		{"overwrite path changed", overwrite1411Raw, overwrite1411Shown, `"createFile":{"path":{"absoluteUri":"file:///fixture/notes.txt"}`, `"createFile":{"path":{"absoluteUri":"file:///fixture/other.txt"}`},
		{"overwrite stats changed", overwrite1411Raw, overwrite1411Shown, `"deletions":1}}}]}`, `"deletions":2}}}]}`},
		{"failed write error changed", failedWrite1411Raw, failedWrite1411Shown, `"fullError":"/fixture/notes.txt already exists.`, `"fullError":"/fixture/notes.txt is missing.`},
		{"failed write marked overwrite", failedWrite1411Raw, failedWrite1411Shown, `"absoluteUri":"file:///fixture/notes.txt"}}}`, `"absoluteUri":"file:///fixture/notes.txt"},"overwrite":true}}`},
		{"failed write as edit", failedWrite1411Raw, failedWrite1411Shown, `"actionSpec":{"createFile":{"path":{"absoluteUri":"file:///fixture/notes.txt"}}}`, `"actionSpec":{"command":{"isEdit":true,"file":{"absoluteUri":"file:///fixture/notes.txt"}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := snapshot1216(t, tc.raw)
			require1216(t, s, tc.raw, replace1216(t, tc.shown, tc.before, tc.after), Incomplete)
		})
	}
}

// Only the captured arguments, statuses and payload fields are qualified.
func TestFiles1411RefusesUnqualifiedShapes(t *testing.T) {
	for _, tc := range []struct {
		name, raw, before, after string
	}{
		{"overwrite false", overwrite1411Raw, `"Overwrite":"true"`, `"Overwrite":"false"`},
		{"overwrite missing", overwrite1411Raw, `"Overwrite":"true",`, ``},
		{"overwrite not retained", overwrite1411Raw, `\"Overwrite\":true,`, ``},
		{"overwrite extra argument", overwrite1411Raw, `"args":{`, `"args":{"ArtifactMetadata":"{}",`},
		{"overwrite artifact", overwrite1411Raw, `"kind":"OVERWRITE",`, `"kind":"OVERWRITE","isArtifactFile":true,`},
		{"overwrite path differs", overwrite1411Raw, `"kind":"OVERWRITE","path":"/fixture/notes.txt"`, `"kind":"OVERWRITE","path":"/fixture/other.txt"`},
		{"overwrite by edit_file", overwrite1411Raw, `"name":"write_file"`, `"name":"edit_file"`},
		{"overwrite relative path", overwrite1411Raw, `"path":"/fixture/notes.txt"`, `"path":"fixture/notes.txt"`},
		{"overwrite canceled", overwrite1411Raw, `CORTEX_STEP_STATUS_DONE`, `CORTEX_STEP_STATUS_CANCELED`},
		{"failed write done", failedWrite1411Raw, `CORTEX_STEP_STATUS_ERROR`, `CORTEX_STEP_STATUS_DONE`},
		{"failed write with overwrite", failedWrite1411Raw, `"args":{`, `"args":{"Overwrite":"true",`},
		{"failed write not retained", failedWrite1411Raw, `\"Content\":\"new\\n\",`, ``},
		{"failed write relative path", failedWrite1411Raw, `"Path":"/fixture/notes.txt"`, `"Path":"fixture/notes.txt"`},
		{"failed write unequal errors", failedWrite1411Raw, `"fullError":"/fixture/notes.txt already exists.`, `"fullError":"/fixture/notes.txt is missing.`},
		{"failed write result text", failedWrite1411Raw, `"result":{"stepRenderInfo"`, `"result":{"result":"unexpected","stepRenderInfo"`},
		{"failed write without render info", failedWrite1411Raw, `"result":{"stepRenderInfo":{"titlePrefix":"Writing","icon":"edit"}}`, `"result":{"result":"unexpected"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := replace1216(t, tc.raw, tc.before, tc.after)
			if _, err := snapshot1216(t, bad).OfflineJSON(); err == nil {
				t.Fatalf("accepted unqualified shape")
			}
		})
	}
}

// The historical CREATE presentation with Overwrite keeps its own mapping: it
// shows no overwrite flag and marks the file as created.
func TestFiles1411KeepsHistoricalCreate(t *testing.T) {
	raw := replace1216(t, overwrite1411Raw, `"kind":"OVERWRITE"`, `"kind":"CREATE"`)
	shown := replace1216(t, overwrite1411Shown, `,"overwrite":true`, ``)
	shown = replace1216(t, shown, `"absoluteUri":"file:///fixture/notes.txt"}},`, `"absoluteUri":"file:///fixture/notes.txt","createFile":true}},`)
	require1216(t, snapshot1216(t, raw), raw, shown, Verified)
	require1216(t, snapshot1216(t, raw), raw, overwrite1411Shown, Incomplete)
}

func TestFiles1411InvalidatesPreviousProfile(t *testing.T) {
	previous := strings.SplitN(Profile, "-files1411", 2)[0]
	if previous == Profile {
		t.Fatal("profile does not name files1411")
	}
	s := snapshot1216(t, overwrite1411Raw)
	a, r := s.VerifyResponses([]byte(overwrite1411Raw), []byte(overwrite1411Shown))
	if r.State != Verified {
		t.Fatal(r)
	}
	a.Profile = previous
	if r := s.CheckAttestation([]byte(overwrite1411Shown), a); r.State != Unknown {
		t.Fatalf("previous profile accepted: %+v", r)
	}
}
