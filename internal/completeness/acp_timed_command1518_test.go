package completeness

import (
	"strings"
	"testing"
)

// The stable ACP capture kept the running wrapper intact. Once complete, it
// showed the source wait, exit code, and output on a native RUN_COMMAND and
// retained the approval and task. Build these shapes from the synthetic ACP
// approval fixture; the expected ordinary form is independent of the projection.
func acpTimedCommand1518(t *testing.T, running bool) (string, string) {
	t.Helper()
	raw, shown := acp1516Pair(t, "command")
	timed := func(data string) string {
		return mutateJSON158(t, data, func(step map[string]any) {
			meta := step["metadata"].(map[string]any)
			call := meta["toolCall"].(map[string]any)
			call["argumentsJson"] = strings.Replace(call["argumentsJson"].(string), "{", `{"NotificationTimeoutSeconds":120,`, 1)
			meta["sourceTrajectoryStepInfo"] = map[string]any{"trajectoryId": "fixture text 5", "stepIndex": 29}
			step["taskDetails"] = map[string]any{"id": "fixture-task", "progress": "fixture progress", "movedToBackgroundAt": "2026-01-01T23:08:46Z"}
			if generic, ok := step["generic"].(map[string]any); ok {
				generic["args"].(map[string]any)["NotificationTimeoutSeconds"] = "120"
				if running {
					result := generic["result"].(map[string]any)
					generic["result"] = map[string]any{"stepRenderInfo": result["stepRenderInfo"]}
					step["status"] = "CORTEX_STEP_STATUS_RUNNING"
					delete(meta, "completedAt")
					history := meta["internalMetadata"].(map[string]any)
					transitions := history["statusTransitions"].([]any)
					history["statusTransitions"] = transitions[:len(transitions)-1]
				}
			} else {
				delete(step["runCommand"].(map[string]any), "blocking")
			}
		})
	}
	raw = timed(raw)
	if running {
		return raw, raw
	}
	return raw, timed(shown)
}

func TestACPTimedBackgroundCommand1518(t *testing.T) {
	for _, phase := range []string{"running", "completed"} {
		t.Run(phase, func(t *testing.T) {
			raw, shown := acpTimedCommand1518(t, phase == "running")
			s := snapshot1216(t, raw)
			require1216(t, s, raw, shown, Verified)
			offline, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, raw, string(offline), Verified)

			changedProgress := replace1216(t, shown, `"progress":"fixture progress"`, `"progress":"more output"`)
			require1216(t, s, raw, changedProgress, Incomplete)
			require1216(t, s, replace1216(t, raw, "fixture progress", "more output"), shown, Incomplete)

			for _, tc := range []struct{ name, before, after string }{
				{"missing task id", `"id":"fixture-task"`, `"id":""`},
				{"not backgrounded", `"movedToBackgroundAt":"2026-01-01T23:08:46Z",`, ""},
				{"denied approval", `"allow":true`, `"allow":false`},
				{"missing wait transition", "CORTEX_STEP_STATUS_WAITING", "CORTEX_STEP_STATUS_PENDING"},
				{"duration string in call", `Seconds\":120`, `Seconds\":\"120\"`},
				{"duration differs from call", `"NotificationTimeoutSeconds":"120"`, `"NotificationTimeoutSeconds":"121"`},
			} {
				t.Run(tc.name, func(t *testing.T) {
					bad := replace1216(t, raw, tc.before, tc.after)
					require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
				})
			}
			badStep := strings.Replace(raw, `"stepIndex":29`, `"stepIndex":28`, 1)
			require1216(t, snapshot1216(t, badStep), badStep, shown, Unknown)

			if phase == "completed" {
				missingWait := replace1216(t, shown, `,"waitMsBeforeAsync":"5000"`, "")
				require1216(t, s, raw, missingWait, Incomplete)
			}
		})
	}
}
