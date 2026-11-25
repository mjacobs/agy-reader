package completeness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Synthetic source/full/default pairs with the step shapes T3 Code's
// Antigravity ACP sessions (agy 1.5.16 language server) persist and present.
// Every free-text value is a placeholder; only the structural values the
// projection checks (actions, triggers, shells, MCP naming) are kept.
func acp1516Pair(t *testing.T, name string) (string, string) {
	t.Helper()
	read := func(kind string) string {
		b, err := os.ReadFile(filepath.Join("testdata", "acp1516", name+"."+kind+".json"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	return read("full"), read("default")
}

var acp1516Names = []string{
	"command", "background-command", "blocked-command", "canceled-command",
	"edit", "suggested-edit", "write", "suggested-write", "failed-edit",
	"mcp-done", "mcp-error",
}

func TestACP1516Captures(t *testing.T) {
	for _, name := range acp1516Names {
		t.Run(name, func(t *testing.T) {
			raw, shown := acp1516Pair(t, name)
			s := snapshot1216(t, raw)
			require1216(t, s, raw, shown, Verified)
			offline, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, raw, string(offline), Verified)
			// The permission records stay on the presented step and are compared.
			require1216(t, s, raw, strings.Replace(shown, `"stepIndex": `, `"stepIndex": 9`, 1), Incomplete)
			require1216(t, s, raw, `{"cascadeId":"fixture-parent","steps":[]}`, Incomplete)
			bad := replace1216(t, raw, `"args": {`, `"args": {"Unexpected":"value",`)
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
}

// Each mutation leaves the source well-formed but outside the observed
// shape, so the projection must refuse it rather than certify it.
func TestACP1516Boundaries(t *testing.T) {
	for _, tc := range []struct {
		name, before, after string
		all                 bool
	}{
		// Only the first occurrence: the request, not the payload.
		{"command", `"shellName": "bash"`, `"shellName": "zsh"`, false},
		{"command", `"allow": true`, `"allow": false`, false},
		{"command", `PERSIST_SUGGESTION_TYPE_SUGGESTED`, `PERSIST_SUGGESTION_TYPE_BLOCKED`, true},
		{"command", `TYPE_DEFAULT_FALLBACK`, `TYPE_DEFAULT_GRANT`, true},
		{"blocked-command", `PERSIST_SUGGESTION_TYPE_BLOCKED`, `PERSIST_SUGGESTION_TYPE_SUGGESTED`, true},
		{"background-command", `"shellName": "bash"`, `"shellName": "zsh"`, false},
		{"canceled-command", `CORTEX_STEP_STATUS_WAITING`, `CORTEX_STEP_STATUS_PENDING`, false},
		{"canceled-command", `"allow": true`, `"allow": false`, false},
		{"edit", `"detail": "write_file(/)"`, `"detail": "write_file(/fixture)"`, true},
		{"edit", `TYPE_DEFAULT_GRANT`, `TYPE_DEFAULT_FALLBACK`, true},
		{"suggested-edit", `"persistSuggestionType": "PERSIST_SUGGESTION_TYPE_SUGGESTED",`, ``, true},
		{"write", `"action": "write_file"`, `"action": "command"`, false},
		{"suggested-write", `"allow": true`, `"allow": false`, false},
		{"failed-edit", `"allow": true`, `"allow": false`, false},
		{"failed-edit", `"detail": "write_file(/)"`, `"detail": "write_file(/fixture)"`, true},
		{"mcp-done", `Run send_note from MCP server fixture-srv`, `Run send_note from fixture-srv`, true},
		{"mcp-done", `"name": "mcp_fixture-srv_send_note"`, `"name": "mcp_fixture-srv_note"`, true},
		{"mcp-done", `CORTEX_STEP_STATUS_DONE`, `CORTEX_STEP_STATUS_CANCELED`, true},
		{"mcp-error", `TYPE_DEFAULT_FALLBACK`, `TYPE_DEFAULT_GRANT`, true},
		{"mcp-error", `"target": "fixture-srv/lookup_item"`, `"target": "fixture-srv/other_item"`, false},
		{"mcp-error", `"fullError": "fixture text 135"`, `"fullError": "fixture text 0"`, true},
	} {
		t.Run(tc.name+tc.before, func(t *testing.T) {
			raw, shown := acp1516Pair(t, tc.name)
			bad := replace1216(t, raw, tc.before, tc.after)
			if !tc.all {
				bad = strings.Replace(raw, tc.before, tc.after, 1)
			}
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
}

// Add the extra arguments to all three copies of the call. Otherwise the
// argument-consistency check would mask the ACP argument rule under test.
func TestACP1516CommandExtraArguments(t *testing.T) {
	for _, tc := range []struct {
		name, args, called, waitMS string
		exportable                 bool
	}{
		{"toolAction", `"toolAction":"run",`, `\"toolAction\":\"run\",`, "5000", false},
		{"IsDaemon false", `"IsDaemon":"false",`, `\"IsDaemon\":false,`, "5000", true},
		{"IsDaemon true", `"IsDaemon":"true",`, `\"IsDaemon\":true,`, "1000", false},
		{"IsDaemon string", `"IsDaemon":"false",`, `\"IsDaemon\":\"false\",`, "5000", false},
		{"IsDaemon false with unknown argument", `"IsDaemon":"false","Future":"value",`, `\"IsDaemon\":false,\"Future\":\"value\",`, "5000", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, shown := acp1516Pair(t, "command")
			// The general command path can accept true with a 1000 ms wait.
			// Exercise that case so the ACP rule itself must reject it.
			raw = strings.ReplaceAll(raw, "5000", tc.waitMS)
			shown = strings.ReplaceAll(shown, "5000", tc.waitMS)
			raw = replace1216(t, raw, `"args": {`, `"args": {`+tc.args)
			raw = replace1216(t, raw, `"argumentsJson": "{`, `"argumentsJson": "{`+tc.called)
			shown = replace1216(t, shown, `"argumentsJson": "{`, `"argumentsJson": "{`+tc.called)
			snapshot := snapshot1216(t, raw)
			require1216(t, snapshot, raw, shown, Unknown)
			offline, err := snapshot.OfflineJSON()
			if !tc.exportable {
				if err == nil || offline != nil {
					t.Fatal("unsupported command produced an offline transcript")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var got, want map[string]any
			if err := json.Unmarshal(offline, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(shown), &want); err != nil {
				t.Fatal(err)
			}
			delete(got, "agyReader")
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("offline transcript differs from expected presentation:\ngot %s\nwant %s", offline, shown)
			}
		})
	}
}
