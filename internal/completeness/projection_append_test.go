package completeness

import (
	"strings"
	"testing"
)

// This fixture covers a write_to_file call that appends to a file
// (agy-reader#s42p). One retained agy 1.2.16 session stores it as a generic
// wrapper with the arguments Append "true", CodeContent, Description, Overwrite
// "false" and TargetFile, over an embedded CODE_ACTION Step. Its full RPC
// equals the source, and the default RPC presents the native Step, which
// drops the generic arguments but records Append twice: as the literal true in
// the tool-call arguments and as append on the createFile action spec. The
// result is an edit diff with the appended lines inserted after the existing
// ones.
//
// Values are synthetic and the expected presentation is written out
// explicitly. The source is the same trajectory as the full RPC copy, as in the
// other projection fixtures.

const appendArguments = `{\"Append\":true,\"CodeContent\":\"\\n## Fixture notes\\n\",\"Description\":\"Append fixture notes\",\"Overwrite\":false,\"TargetFile\":\"/fixture/NOTES.md\",\"toolAction\":\"Appending to NOTES.md\",\"toolSummary\":\"Append to NOTES\"}`
const appendNative = `"type":"CORTEX_STEP_TYPE_CODE_ACTION","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-append","name":"write_to_file","argumentsJson":"` + appendArguments +
	`"},"completedAt":"2026-01-01T00:00:01Z","toolSummary":"Append to NOTES","toolAction":"Appending to NOTES.md","preToolHookResults":[{"decision":"allow"}],"toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN"},` +
	`"codeAction":{"actionSpec":{"createFile":{"instruction":"\n## Fixture notes\n","path":{"absoluteUri":"file:///fixture/NOTES.md","workspaceUrisToRelativePaths":{"file:///fixture":"NOTES.md"}},"append":true}},` +
	`"actionResult":{"edit":{"diff":{"endLine":2,"unifiedDiff":{"lines":[{"text":"# Notes","type":"UNIFIED_DIFF_LINE_TYPE_UNCHANGED"},{"type":"UNIFIED_DIFF_LINE_TYPE_INSERT"},{"text":"## Fixture notes","type":"UNIFIED_DIFF_LINE_TYPE_INSERT"},{"type":"UNIFIED_DIFF_LINE_TYPE_UNCHANGED"}]}},"absoluteUri":"file:///fixture/NOTES.md"}},` +
	`"useFastApply":true,"description":"Append fixture notes","diffStats":{"additions":2}}`
const appendRaw = `{"cascadeId":"append-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-append","name":"write_to_file","argumentsJson":"` + appendArguments +
	`"},"completedAt":"2026-01-01T00:00:02Z","preToolHookResults":[{"decision":"allow"}],"toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN"},"generic":{"args":{"Append":"true","CodeContent":"\n## Fixture notes\n","Description":"Append fixture notes","Overwrite":"false","TargetFile":"/fixture/NOTES.md","toolAction":"Appending to NOTES.md","toolSummary":"Append to NOTES"},` +
	`"result":{"result":"Created file file:///fixture/NOTES.md with requested content.","fullOutputUri":"file:///fixture/steps/1/output.txt","payload":{"@type":"type.googleapis.com/gemini_coder.Step",` + appendNative + `}}}}]}`
const appendPresentation = `{"cascadeId":"append-fixture","metadata":{},"steps":[{` + appendNative + `}]}`

// appendFalse rewrites the fixture to the unobserved Append false: the
// literal false in the tool call and append left unset, as the daemon leaves
// a false boolean.
func appendFalse(text string) string {
	text = strings.ReplaceAll(text, `"Append":"true"`, `"Append":"false"`)
	text = strings.ReplaceAll(text, `\"Append\":true`, `\"Append\":false`)
	return strings.ReplaceAll(text, `,"append":true}`, `}`)
}

// requireAppendRefused checks the reason as well as the state, so a change
// that the Append rule should refuse is not passed by an earlier check.
func requireAppendRefused(t *testing.T, raw, presentation string) {
	t.Helper()
	require1216(t, snapshot1216(t, raw), raw, presentation, Unknown)
	if r := snapshot1216(t, raw).CompareResponses([]byte(raw), []byte(presentation)); r.Reason != "unsupported write_to_file append" {
		t.Fatalf("got reason %q, want the Append refusal", r.Reason)
	}
}

func TestProjectionAppendQualifiedShapes(t *testing.T) {
	t.Run("append true", func(t *testing.T) {
		require1216(t, snapshot1216(t, appendRaw), appendRaw, appendPresentation, Verified)
	})
	t.Run("append false", func(t *testing.T) {
		raw, presentation := appendFalse(appendRaw), appendFalse(appendPresentation)
		require1216(t, snapshot1216(t, raw), raw, presentation, Verified)
	})
	t.Run("without append", func(t *testing.T) {
		raw := replace1216(t, appendRaw, `"Append":"true",`, ``)
		require1216(t, snapshot1216(t, raw), raw, appendPresentation, Verified)
	})
}

func TestProjectionAppendRefusedShapes(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		// A changed Append value: the dropped argument no longer matches what
		// the presented step records.
		{"argument changed to false", `"Append":"true"`, `"Append":"false"`},
		{"presented call changed to false", `\"Append\":true`, `\"Append\":false`},
		{"native append unset", `,"append":true}`, `}`},
		// A non-literal value.
		{"capitalised argument", `"Append":"true"`, `"Append":"True"`},
		{"numeric argument", `"Append":"true"`, `"Append":"1"`},
		{"empty argument", `"Append":"true"`, `"Append":""`},
		{"string in the presented call", `\"Append\":true`, `\"Append\":\"true\"`},
		{"null in the presented call", `\"Append\":true`, `\"Append\":null`},
		{"numeric in the presented call", `\"Append\":true`, `\"Append\":1`},
		{"missing from the presented call", `\"Append\":true,`, ``},
		// The native step must be the write_to_file createFile spec: a
		// CODE_ACTION step under replace_file_content cannot account for
		// Append even with a matching spec.
		{"another tool name", `"name":"write_to_file"`, `"name":"replace_file_content"`},
		{"no createFile spec", `"actionSpec":{"createFile":{"instruction":"\n## Fixture notes\n","path":{"absoluteUri":"file:///fixture/NOTES.md","workspaceUrisToRelativePaths":{"file:///fixture":"NOTES.md"}},"append":true}}`, `"actionSpec":{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := replace1216(t, appendRaw, tc.before, tc.after)
			requireAppendRefused(t, raw, strings.ReplaceAll(appendPresentation, tc.before, tc.after))
		})
	}
	t.Run("false argument over native append set", func(t *testing.T) {
		raw := replace1216(t, appendFalse(appendRaw), `"createFile":{"instruction":"\n## Fixture notes\n","path":{"absoluteUri":"file:///fixture/NOTES.md","workspaceUrisToRelativePaths":{"file:///fixture":"NOTES.md"}}}`,
			`"createFile":{"instruction":"\n## Fixture notes\n","path":{"absoluteUri":"file:///fixture/NOTES.md","workspaceUrisToRelativePaths":{"file:///fixture":"NOTES.md"}},"append":true}`)
		requireAppendRefused(t, raw, appendPresentation)
	})
}

func TestProjectionAppendChanges(t *testing.T) {
	runArgumentsChanges(t, appendRaw, appendPresentation, []argumentsChange{
		{"unknown extra argument", `"args":{`, `"args":{"Prepend":"true",`, "source", Unknown},
		{"argument in another case", `"Append":"true"`, `"append":"true"`, "source", Unknown},
		{"argument changed", `"Append":"true"`, `"Append":"false"`, "source", Unknown},
		{"unequal status pair", `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE"`, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR"`, "source", Unknown},
		{"raw argument changed", `"Append":"true"`, `"Append":"false"`, "raw", Incomplete},
		{"raw native append unset", `,"append":true}`, `}`, "raw", Incomplete},
		{"presented call changed", `\"Append\":true`, `\"Append\":false`, "presentation", Incomplete},
		{"presented native append unset", `,"append":true}`, `}`, "presentation", Incomplete},
		{"presented diff changed", `"text":"## Fixture notes"`, `"text":"## Other notes"`, "presentation", Incomplete},
		{"presented status changed", `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_ERROR"`, "presentation", Incomplete},
	})
	t.Run("unknown argument reason", func(t *testing.T) {
		raw := replace1216(t, appendRaw, `"args":{`, `"args":{"Prepend":"true",`)
		if r := snapshot1216(t, raw).CompareResponses([]byte(raw), []byte(appendPresentation)); r.State != Unknown || r.Reason != "unsupported generic arguments" {
			t.Fatalf("got %+v, want unsupported generic arguments", r)
		}
	})
}
