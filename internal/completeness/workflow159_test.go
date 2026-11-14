package completeness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
)

func workflow159Pair(t *testing.T, name string) (string, string) {
	t.Helper()
	read := func(kind string) string {
		b, err := os.ReadFile(filepath.Join("testdata", "workflow159", name+"."+kind+".json"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	return read("full"), read("default")
}

func TestWorkflow159CapturedPresentations(t *testing.T) {
	for _, name := range []string{"enter-plan", "exit-plan", "inline", "agents", "failed"} {
		t.Run(name, func(t *testing.T) {
			raw, shown := workflow159Pair(t, name)
			path := filepath.Join(t.TempDir(), "source.db")
			db := testfixture.Source(t, path, []byte(raw))
			testfixture.Exec(t, db, "UPDATE steps SET render_info=NULL,permissions=NULL")
			s, err := ReadSnapshot(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, raw, shown, Verified)
			offline, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, raw, string(offline), Verified)
			changed := replace1216(t, raw, "call-fixture", "call-changed")
			require1216(t, s, changed, shown, Incomplete)
			changed = replace1216(t, shown, "call-fixture", "call-changed")
			require1216(t, s, raw, changed, Incomplete)
			bad := mutateJSON158(t, raw, func(step map[string]any) {
				step["generic"].(map[string]any)["args"].(map[string]any)["Future"] = "unsupported"
			})
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
}

func TestWorkflow159RejectsUnqualifiedShapes(t *testing.T) {
	for _, tc := range []struct {
		name, fixture string
		change        func(map[string]any)
	}{
		{"running", "inline", func(s map[string]any) { s["status"] = "CORTEX_STEP_STATUS_RUNNING" }},
		{"canceled", "inline", func(s map[string]any) { s["status"] = "CORTEX_STEP_STATUS_CANCELED" }},
		{"outer task", "inline", func(s map[string]any) { s["taskDetails"] = map[string]any{"id": "task"} }},
		{"outer permission", "inline", func(s map[string]any) { s["permissions"] = map[string]any{} }},
		{"result metadata", "inline", func(s map[string]any) { result159(s)["metadata"] = map[string]any{"future": "value"} }},
		{"changed script", "inline", func(s map[string]any) { args159(s)["Script"] = "print('changed')" }},
		{"two scripts", "inline", func(s map[string]any) {
			setArgument(s, "ScriptPath", "/fixture/work/script.py", json.RawMessage(`"/fixture/work/script.py"`))
		}},
		{"relative path", "agents", func(s map[string]any) { setArgument(s, "ScriptPath", "script.py", json.RawMessage(`"script.py"`)) }},
		{"unobserved wait", "inline", func(s map[string]any) { setArgument(s, "WaitMsBeforeAsync", "20000", json.RawMessage(`20000`)) }},
		{"array args", "inline", func(s map[string]any) { setArgument(s, "Args", "[]", json.RawMessage(`[]`)) }},
		{"wrong result", "inline", func(s map[string]any) { result159(s)["result"] = "Workflow completed." }},
		{"no output URI", "inline", func(s map[string]any) { delete(result159(s), "fullOutputUri") }},
		{"wrong plan result", "exit-plan", func(s map[string]any) { result159(s)["result"] = "Plan mode is off." }},
		{"empty reason", "enter-plan", func(s map[string]any) { setArgument(s, "reason", "", json.RawMessage(`""`)) }},
		{"different errors", "failed", func(s map[string]any) { s["error"].(map[string]any)["fullError"] = "different" }},
		{"no failure display", "failed", func(s map[string]any) { delete(result159(s), "stepRenderInfo") }},
		{"failure with result text", "failed", func(s map[string]any) { result159(s)["result"] = "unobserved" }},
		{"other failure", "failed", func(s map[string]any) { s["error"] = map[string]any{"shortError": "failed", "fullError": "failed"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, shown := workflow159Pair(t, tc.fixture)
			bad := mutateJSON158(t, raw, tc.change)
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
}

func args159(s map[string]any) map[string]any {
	return s["generic"].(map[string]any)["args"].(map[string]any)
}

func result159(s map[string]any) map[string]any {
	return s["generic"].(map[string]any)["result"].(map[string]any)
}

func TestWorkflow159SourceTitle(t *testing.T) {
	data := bytesField(4, []byte("Parallel worker")...)
	if _, err := decode("exa.cortex_pb.ToolSourceMetadata", data); err != nil {
		t.Fatal(err)
	}
	if _, err := decode("exa.cortex_pb.ToolSourceMetadata", append(data, options145Varint(99, 1)...)); err == nil {
		t.Fatal("unknown source metadata field accepted")
	}
}
