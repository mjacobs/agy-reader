package completeness

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// The captured calls share a builtin tool call, explicit numeric arguments and
// status history. Keep the test commands and their output synthetic.
func TestNotificationCommand(t *testing.T) {
	for _, tc := range []struct {
		status     string
		timeout    int
		wait       int
		background bool
		bypass     bool
		exitCode   int
	}{
		// Historical transcripts captured before 1.5.17.
		{"DONE", 180, 10000, true, false, 0},
		{"CANCELED", 120, 10000, true, false, 0},
		{"DONE", 120, 10000, true, true, 0},
		{"INVALID", 1800, 5000, false, true, 0},
		// The 1.5.17 exercise completed one foreground and two background commands.
		{"DONE", 4, 1000, false, false, 0},
		{"DONE", 3, 500, true, false, 0},
		{"DONE", 2, 500, true, false, 3},
	} {
		t.Run(tc.status+strconv.Itoa(tc.timeout), func(t *testing.T) {
			const cmd = "echo fixture"
			status := "CORTEX_STEP_STATUS_" + tc.status
			history := []string{"PENDING", "RUNNING", tc.status}
			if tc.status == "INVALID" {
				history = []string{"PENDING", "INVALID"}
			}
			called := map[string]any{"CommandLine": cmd, "WaitMsBeforeAsync": tc.wait, "NotificationTimeoutSeconds": tc.timeout, "toolSummary": "Fixture"}
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
				if tc.background {
					task := map[string]string{"id": "fixture", "movedToBackgroundAt": "2026-01-01T00:00:01Z"}
					stored["taskDetails"] = task
					shown["taskDetails"] = task
				}
				result := map[string]any{"stepRenderInfo": map[string]string{"title": "Fixture"}}
				generic["result"] = result
				run := map[string]any{"commandLine": cmd, "proposedCommandLine": cmd, "combinedOutput": map[string]any{}}
				shown["runCommand"] = run
				if tc.status == "DONE" {
					payload := map[string]any{"@type": "type.googleapis.com/exa.cortex.core.tools.RunCommand", "commandLine": cmd}
					if tc.exitCode != 0 {
						payload["exitCode"] = tc.exitCode
					}
					if !tc.background {
						run["blocking"] = true
					}
					result["payload"] = payload
					result["result"] = "fixture output"
					run["combinedOutput"] = map[string]string{"full": "fixture output"}
					run["exitCode"] = tc.exitCode
					run["waitMsBeforeAsync"] = strconv.Itoa(tc.wait)
				}
			}
			trajectory := func(step map[string]any) string {
				return encode(map[string]any{"cascadeId": "fixture", "steps": []any{step}})
			}
			raw, presentation := trajectory(stored), trajectory(shown)
			s := snapshot1216(t, raw)
			require1216(t, s, raw, presentation, Verified)
			if tc.status == "DONE" {
				offline, err := s.OfflineJSON()
				if err != nil {
					t.Fatal(err)
				}
				var reconstructed map[string]any
				if err := json.Unmarshal(offline, &reconstructed); err != nil {
					t.Fatal(err)
				}
				if got := encode(reconstructed["steps"]); got != encode([]any{shown}) {
					t.Fatalf("offline command differs from RPC: %s", got)
				}
				// Fractions, negative values, numeric strings and invalid integer
				// ranges cannot qualify a recorded notification duration.
				for _, duration := range []string{`0`, `-1`, `2.5`, `"3"`, `18446744073709551616`} {
					called["NotificationTimeoutSeconds"] = json.RawMessage(duration)
					args["NotificationTimeoutSeconds"] = strings.Trim(duration, `"`)
					call["argumentsJson"] = encode(called)
					bad := trajectory(stored)
					require1216(t, snapshot1216(t, bad), bad, presentation, Unknown)
				}
				called["NotificationTimeoutSeconds"] = tc.timeout
				args["NotificationTimeoutSeconds"] = strconv.Itoa(tc.timeout)
				call["argumentsJson"] = encode(called)
				run := shown["runCommand"].(map[string]any)
				delete(run, "waitMsBeforeAsync")
				require1216(t, s, raw, trajectory(shown), Incomplete)
				for _, wrongWait := range []string{"0", strconv.Itoa(tc.wait + 1)} {
					run["waitMsBeforeAsync"] = wrongWait
					require1216(t, s, raw, trajectory(shown), Incomplete)
				}
				run["waitMsBeforeAsync"] = strconv.Itoa(tc.wait)
				run["notificationTimeoutSeconds"] = strconv.Itoa(tc.timeout)
				require1216(t, s, raw, trajectory(shown), Incomplete)
				delete(run, "notificationTimeoutSeconds")
				changedCommand := strings.Replace(raw, `"commandLine":"echo fixture"`, `"commandLine":"echo changed"`, 1)
				require1216(t, snapshot1216(t, changedCommand), changedCommand, presentation, Unknown)
				// Dropping a lifecycle event or the background marker leaves
				// a native source outside the qualified completed shapes.
				history := meta["internalMetadata"].(map[string]any)
				history["statusTransitions"] = transitions[1:]
				partial := trajectory(stored)
				require1216(t, snapshot1216(t, partial), partial, presentation, Unknown)
				history["statusTransitions"] = transitions
				if tc.background {
					task := stored["taskDetails"].(map[string]string)
					stamp := task["movedToBackgroundAt"]
					delete(task, "movedToBackgroundAt")
					partial = trajectory(stored)
					require1216(t, snapshot1216(t, partial), partial, presentation, Unknown)
					task["movedToBackgroundAt"] = stamp
				}
			}
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

// The 1.5.17 strict-mode exercise repeats the 1.5.7 permission exchange with
// a typed timeout, a 1000 ms wait, and a response referring to the command
// step. Reuse the sanitized grant and result in both captured views.
func TestApprovedNotificationCommand1517(t *testing.T) {
	raw, ordinary := runtime157Pair(t, "strict-command")
	raw, ordinary = timedPermissionPair1517(t, raw, ordinary)
	ordinary = replace1216(t, ordinary, `"waitMsBeforeAsync": "5000"`, `"waitMsBeforeAsync":"1000"`)
	snapshot := snapshot1216(t, raw)
	require1216(t, snapshot, raw, ordinary, Verified)
	t.Run("approved background command", func(t *testing.T) {
		// The strict background capture also retains the task and presents a
		// non-blocking result after the same permission exchange.
		background := func(s string) string {
			s = strings.ReplaceAll(s, "1000", "500")
			s = strings.ReplaceAll(s, `Seconds\":4`, `Seconds\":2`)
			s = strings.ReplaceAll(s, `Seconds":"4"`, `Seconds":"2"`)
			return replace1216(t, s, `"permissions": {`, `"taskDetails":{"id":"fixture-task","movedToBackgroundAt":"2026-01-01T00:00:01Z"},"permissions":{`)
		}
		source := background(raw)
		shown := replace1216(t, background(ordinary), `"blocking": true,`, "")
		s := snapshot1216(t, source)
		require1216(t, s, source, shown, Verified)
		changed := replace1216(t, source, `,"movedToBackgroundAt":"2026-01-01T00:00:01Z"`, "")
		require1216(t, snapshot1216(t, changed), changed, shown, Unknown)
		require1216(t, s, source, replace1216(t, shown, `"id":"fixture-task"`, `"id":"different"`), Incomplete)
	})
	for _, bad := range []struct{ name, before, after string }{
		{"grant target", `"target": "echo fixture"`, `"target": "other fixture"`},
		{"permission denied", `"allow": true`, `"allow": false`},
		{"wrong response trajectory", `"trajectoryId": "fixture-trajectory"`, `"trajectoryId":"different"`},
		{"wrong response step", `"stepIndex": 2`, `"stepIndex":1`},
		{"missing wait transition", `"CORTEX_STEP_STATUS_WAITING"`, `"CORTEX_STEP_STATUS_RUNNING"`},
		{"unmatched timeout", `"NotificationTimeoutSeconds":"4"`, `"NotificationTimeoutSeconds":"5"`},
		{"unmatched payload command", `"commandLine": "echo fixture"`, `"commandLine": "different"`},
	} {
		t.Run(bad.name, func(t *testing.T) {
			changed := replace1216(t, raw, bad.before, bad.after)
			require1216(t, snapshot1216(t, changed), changed, ordinary, Unknown)
		})
	}
	for _, field := range []string{"permissions", "completedInteractions"} {
		t.Run("missing "+field, func(t *testing.T) {
			var doc struct {
				Steps []map[string]any `json:"steps"`
			}
			if err := json.Unmarshal([]byte(raw), &doc); err != nil {
				t.Fatal(err)
			}
			delete(doc.Steps[0], field)
			data, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			changed := string(data)
			require1216(t, snapshot1216(t, changed), changed, ordinary, Unknown)
		})
	}
	for _, bad := range []struct{ before, after string }{
		{`"waitMsBeforeAsync":"1000",`, ``},
		{`"allow": true`, `"allow": false`},
		{`"full": "fixture result"`, `"full": "different"`},
	} {
		require1216(t, snapshot, raw, replace1216(t, ordinary, bad.before, bad.after), Incomplete)
	}
}

func timedPermissionPair1517(t *testing.T, raw, ordinary string) (string, string) {
	t.Helper()
	addTimedCall := func(s string) string {
		s = replace1216(t, s, `"toolCall": {`, `"sourceTrajectoryStepInfo":{"trajectoryId":"fixture-trajectory","stepIndex":2},"toolCall":{`)
		return replace1216(t, s, `\"WaitMsBeforeAsync\":5000`, `\"NotificationTimeoutSeconds\":4,\"WaitMsBeforeAsync\":1000`)
	}
	raw = replace1216(t, raw, `"WaitMsBeforeAsync": "5000"`, `"NotificationTimeoutSeconds":"4","WaitMsBeforeAsync":"1000"`)
	return addTimedCall(raw), addTimedCall(ordinary)
}

func TestDeniedNotificationCommand1517(t *testing.T) {
	raw, shown := runtime157Pair(t, "rejected-command")
	raw, shown = timedPermissionPair1517(t, raw, shown)
	// 1.5.17 stored the same effective-grant denial with new error wording.
	raw = strings.ReplaceAll(raw, "failed for unsandboxed", "failed for command")
	shown = strings.ReplaceAll(shown, "failed for unsandboxed", "failed for command")
	s := snapshot1216(t, raw)
	require1216(t, s, raw, shown, Verified)
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, raw, string(offline), Verified)
	for _, tc := range []struct{ name, before, after string }{
		{"wrong target", `"target": "echo fixture"`, `"target":"another"`},
		{"allow instead of deny", `"persistGrants": {}`, `"allow":true,"persistGrants":{}`},
		{"wrong response step", `"stepIndex": 2`, `"stepIndex":12`},
		{"wrong response trajectory", `"trajectoryId": "fixture-trajectory"`, `"trajectoryId":"other"`},
		{"altered error", "user denied permission", "agent denied permission"},
		{"wrong transition", "CORTEX_STEP_STATUS_WAITING", "CORTEX_STEP_STATUS_PENDING"},
		{"already started a task", `"generic": {`, `"taskDetails":{"id":"fixture-task"},"generic":{`},
		{"unexpected result", `"args": {`, `"result":{"result":"fixture"},"args":{`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := replace1216(t, raw, tc.before, tc.after)
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
	for _, added := range []string{`"exitCode":0,`, `"waitMsBeforeAsync":"1000",`} {
		require1216(t, s, raw, replace1216(t, shown, `"runCommand": {`, `"runCommand":{`+added), Incomplete)
	}
}

// Direct 1.5.17 full and ordinary RPC agree on the complete running generic
// step. Killing its background task records completion, adds a canceled
// transition, and presents a native command with empty output and no exit code.
// A fresh daemon reopening the same frozen running DB presents a canceled
// command while the original two transitions and absent completion time remain.
func TestNotificationCommandLifecycle1517(t *testing.T) {
	for _, state := range []string{"running", "killed", "reloaded"} {
		t.Run(state, func(t *testing.T) {
			fixture := "running"
			if state != "running" {
				fixture = "background-canceled"
			}
			raw, shown := runtime157Pair(t, fixture)
			adapt := func(s string) string {
				s = replace1216(t, s, `\"WaitMsBeforeAsync\":500`, `\"WaitMsBeforeAsync\":500,\"NotificationTimeoutSeconds\":30`)
				if strings.Contains(s, `"generic": {`) {
					s = replace1216(t, s, `"WaitMsBeforeAsync": "500"`, `"WaitMsBeforeAsync":"500","NotificationTimeoutSeconds":"30"`)
				}
				if state == "killed" {
					s = replace1216(t, s, `"toolCall": {`, `"completedAt":"2026-01-01T00:00:10Z","toolCall":{`)
					s = replace1216(t, s, `"updatedStatus": "CORTEX_STEP_STATUS_RUNNING"`, `"updatedStatus":"CORTEX_STEP_STATUS_RUNNING"},{"updatedStatus":"CORTEX_STEP_STATUS_CANCELED"`)
				}
				return s
			}
			raw, shown = adapt(raw), adapt(shown)
			snapshot := snapshot1216(t, raw)
			require1216(t, snapshot, raw, shown, Verified)
			offline, err := snapshot.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			if r := compareProjection(trajectory1216(t, shown), offline); r.State != Verified {
				t.Fatalf("offline presentation differs from native RPC: %v", r)
			}
			for _, mutation := range []struct{ before, after string }{
				{`"movedToBackgroundAt": "2026-01-01T00:00:01Z"`, `"isDaemon":true`},
				{`"id": "fixture/task-1"`, `"id":""`},
				{`"NotificationTimeoutSeconds":"30"`, `"NotificationTimeoutSeconds":"31"`},
				{`\"NotificationTimeoutSeconds\":30`, `\"NotificationTimeoutSeconds\":\"30\"`},
				{`CORTEX_STEP_STATUS_PENDING`, `CORTEX_STEP_STATUS_WAITING`},
				{`"stepRenderInfo": {`, `"result":"unexpected command output","stepRenderInfo":{`},
			} {
				bad := replace1216(t, raw, mutation.before, mutation.after)
				require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
			}
			if state != "killed" {
				bad := replace1216(t, raw, `"toolCall": {`, `"completedAt":"2026-01-01T00:00:10Z","toolCall":{`)
				require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
			} else {
				bad := replace1216(t, raw, `"completedAt":"2026-01-01T00:00:10Z",`, "")
				require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
			}
			if state != "running" {
				for _, extra := range []string{`"exitCode":0,`, `"waitMsBeforeAsync":"500",`} {
					require1216(t, snapshot, raw, replace1216(t, shown, `"runCommand": {`, `"runCommand":{`+extra), Incomplete)
				}
			}
			require1216(t, snapshot, raw, replace1216(t, shown, `fixture/task-1`, "different-task"), Incomplete)
		})
	}
}
