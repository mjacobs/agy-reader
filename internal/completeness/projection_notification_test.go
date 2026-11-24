package completeness

import (
	"encoding/json"
	"strconv"
	"testing"
)

// The captured calls share a builtin tool call, explicit numeric arguments and
// status history. Keep the test commands and their output synthetic.
func TestHistoricalNotificationCommand(t *testing.T) {
	for _, tc := range []struct {
		status  string
		timeout int
		bypass  bool
	}{
		{"DONE", 180, false}, {"CANCELED", 120, false}, {"DONE", 120, true}, {"INVALID", 1800, true},
	} {
		t.Run(tc.status+strconv.Itoa(tc.timeout), func(t *testing.T) {
			const cmd = "echo fixture"
			status := "CORTEX_STEP_STATUS_" + tc.status
			wait := 10000
			history := []string{"PENDING", "RUNNING", tc.status}
			if tc.status == "INVALID" {
				wait = 5000
				history = []string{"PENDING", "INVALID"}
			}
			called := map[string]any{"CommandLine": cmd, "WaitMsBeforeAsync": wait, "NotificationTimeoutSeconds": tc.timeout, "toolSummary": "Fixture"}
			if tc.bypass {
				called["BypassSandbox"] = true
			}
			args := map[string]string{}
			for k, v := range called {
				if str, ok := v.(string); ok {
					args[k] = str
				} else {
					b, err := json.Marshal(v)
					if err != nil {
						t.Fatal(err)
					}
					args[k] = string(b)
				}
			}
			encode := func(v any) string {
				b, err := json.Marshal(v)
				if err != nil {
					t.Fatal(err)
				}
				return string(b)
			}
			transitions := []map[string]string{}
			for _, v := range history {
				transitions = append(transitions, map[string]string{"updatedStatus": "CORTEX_STEP_STATUS_" + v})
			}
			call := map[string]any{"name": "run_command", "argumentsJson": encode(called)}
			meta := map[string]any{"completedAt": "2026-01-01T00:00:02Z", "toolSource": "CORTEX_COMPONENT_SOURCE_BUILTIN", "toolCall": call, "internalMetadata": map[string]any{"statusTransitions": transitions}}
			generic := map[string]any{"args": args}
			stored := map[string]any{"type": "CORTEX_STEP_TYPE_GENERIC", "status": status, "metadata": meta, "generic": generic}
			shown := map[string]any{"type": "CORTEX_STEP_TYPE_RUN_COMMAND", "status": status, "metadata": meta}
			if tc.status == "INVALID" {
				stored["error"] = map[string]string{"shortError": notificationRejection, "fullError": notificationRejection}
				shown = stored
			} else {
				task := map[string]string{"id": "fixture", "movedToBackgroundAt": "2026-01-01T00:00:01Z"}
				stored["taskDetails"] = task
				shown["taskDetails"] = task
				result := map[string]any{"stepRenderInfo": map[string]string{"title": "Fixture"}}
				generic["result"] = result
				run := map[string]any{"commandLine": cmd, "proposedCommandLine": cmd, "combinedOutput": map[string]any{}}
				shown["runCommand"] = run
				if tc.status == "DONE" {
					result["payload"] = map[string]string{"@type": "type.googleapis.com/exa.cortex.core.tools.RunCommand", "commandLine": cmd}
					result["result"] = "fixture output"
					run["combinedOutput"] = map[string]string{"full": "fixture output"}
					run["exitCode"] = 0
					run["waitMsBeforeAsync"] = "10000"
				}
			}
			trajectory := func(step map[string]any) string {
				return encode(map[string]any{"cascadeId": "fixture", "steps": []any{step}})
			}
			raw, presentation := trajectory(stored), trajectory(shown)
			s := snapshot1216(t, raw)
			require1216(t, s, raw, presentation, Verified)
			shown["status"] = "CORTEX_STEP_STATUS_PENDING"
			require1216(t, s, raw, trajectory(shown), Incomplete)
			shown["status"] = status
			// A numeric string would compare equal to the generic map's saved
			// text. The retained tool call must contain an actual JSON number.
			called["NotificationTimeoutSeconds"] = args["NotificationTimeoutSeconds"]
			call["argumentsJson"] = encode(called)
			bad := trajectory(stored)
			require1216(t, snapshot1216(t, bad), bad, trajectory(shown), Unknown)
		})
	}
}
