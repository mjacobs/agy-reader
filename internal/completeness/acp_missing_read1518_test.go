package completeness

import (
	"encoding/json"
	"strings"
	"testing"
)

// The failed read has the same error and file view as the existing missing-file
// fixture. ACP adds a permission request for this step and an allow response.
func acpMissingRead1518Pair(t *testing.T) (string, string) {
	t.Helper()
	raw, shown := tools158Pair(t, "missing-read")
	withApproval := func(step map[string]any) {
		const records = `{
			"permissions": {"permissionGrants": [{
				"allow": true, "action": "read_file", "target": "/fixture/input.txt",
				"resource": {"action": "read_file", "target": "/fixture/input.txt"}
			}]},
			"completedInteractions": [{
				"request": {"permission": {
					"resource": {"action": "read_file", "target": "/fixture/input.txt"},
					"actionDescription": "Analyzing input.txt",
					"triggerSource": {"type": "TYPE_DEFAULT_FALLBACK"}
				}},
				"response": {"trajectoryId": "fixture-trajectory", "stepIndex": 1,
					"permission": {"allow": true, "persistGrants": {}}}
			}]
		}`
		if err := json.Unmarshal([]byte(records), &step); err != nil {
			t.Fatal(err)
		}
		metadata := step["metadata"].(map[string]any)
		metadata["toolSource"] = "CORTEX_COMPONENT_SOURCE_BUILTIN"
		metadata["sourceTrajectoryStepInfo"] = map[string]any{"trajectoryId": "fixture-trajectory", "stepIndex": 1}
		var transitions []any
		for _, state := range []string{"PENDING", "RUNNING", "WAITING", "RUNNING", "ERROR"} {
			transitions = append(transitions, map[string]any{"updatedStatus": "CORTEX_STEP_STATUS_" + state})
		}
		metadata["internalMetadata"] = map[string]any{"statusTransitions": transitions}
	}
	return mutateJSON158(t, raw, withApproval), mutateJSON158(t, shown, withApproval)
}

func TestACPMissingRead1518(t *testing.T) {
	raw, shown := acpMissingRead1518Pair(t)
	snapshot := snapshot1216(t, raw)
	require1216(t, snapshot, raw, shown, Verified)
	offline, err := snapshot.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, snapshot, raw, string(offline), Verified)
	for _, key := range []string{"permissions", "completedInteractions", "error", "metadata"} {
		t.Run("retains_"+key, func(t *testing.T) {
			drop := func(step map[string]any) { delete(step, key) }
			require1216(t, snapshot, mutateJSON158(t, raw, drop), shown, Incomplete)
			require1216(t, snapshot, raw, mutateJSON158(t, shown, drop), Incomplete)
		})
	}
}

func TestACPMissingRead1518Refusals(t *testing.T) {
	raw, shown := acpMissingRead1518Pair(t)
	for _, key := range []string{"permissions", "completedInteractions"} {
		t.Run("missing_"+key, func(t *testing.T) {
			bad := mutateJSON158(t, raw, func(step map[string]any) { delete(step, key) })
			s := snapshot1216(t, bad)
			require1216(t, s, bad, shown, Unknown)
			if _, err := s.OfflineJSON(); err == nil {
				t.Fatal("exported incomplete ACP permission record")
			}
		})
	}
	t.Run("matched extra argument", func(t *testing.T) {
		bad := mutateJSON158(t, raw, func(step map[string]any) {
			step["generic"].(map[string]any)["args"].(map[string]any)["StartLine"] = "1"
			call := step["metadata"].(map[string]any)["toolCall"].(map[string]any)
			call["argumentsJson"] = `{"Path":"/fixture/input.txt","StartLine":1}`
		})
		s := snapshot1216(t, bad)
		require1216(t, s, bad, shown, Unknown)
		if _, err := s.OfflineJSON(); err == nil {
			t.Fatal("exported unsupported ACP read range")
		}
	})
	for _, tc := range []struct{ name, old, changed string }{
		{"unrelated request", `"actionDescription":"Analyzing input.txt"`, `"actionDescription":"Analyzing other.txt"`},
		{"different requested resource", `"target":"/fixture/input.txt"`, `"target":"/fixture/other.txt"`},
		{"approval denied", `"allow":true,"persistGrants"`, `"allow":false,"persistGrants"`},
		{"grant denied", `"permissionGrants":[{"action":"read_file","allow":true`, `"permissionGrants":[{"action":"read_file","allow":false`},
		{"changed trigger", "TYPE_DEFAULT_FALLBACK", "TYPE_DEFAULT_GRANT"},
		{"unexpected shell", `"triggerSource":`, `"shellName":"sh","triggerSource":`},
		{"changed tool source", "CORTEX_COMPONENT_SOURCE_BUILTIN", "CORTEX_COMPONENT_SOURCE_UNSPECIFIED"},
		{"missing persist record", `,"persistGrants":{}`, ``},
		{"different step", `"stepIndex":1`, `"stepIndex":8`},
		{"different trajectory", `"trajectoryId":"fixture-trajectory"`, `"trajectoryId":"other"`},
		{"no matching transition", "CORTEX_STEP_STATUS_WAITING", "CORTEX_STEP_STATUS_RUNNING"},
		{"non-missing error", "file does not exist", "other error"},
		{"unrelated render URI", "file:///fixture/input.txt", "file:///fixture/other.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := strings.Replace(raw, tc.old, tc.changed, 1)
			if bad == raw {
				t.Fatal("mutation did not change the source")
			}
			s := snapshot1216(t, bad)
			require1216(t, s, bad, shown, Unknown)
			if _, err := s.OfflineJSON(); err == nil {
				t.Fatal("exported unsupported ACP permission record")
			}
		})
	}
}
