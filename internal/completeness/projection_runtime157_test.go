package completeness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Sanitized source/full/default triplets from isolated 1.5.7 runtime captures
// and retained cron/swarm/append conversations. Presentations come from the
// daemon independently of project(). Private source copies stay outside Git.
func runtime157Pair(t *testing.T, name string) (string, string) {
	t.Helper()
	read := func(kind string) string {
		b, e := os.ReadFile(filepath.Join("testdata", "runtime157", name+"."+kind+".json"))
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	return read("full"), read("default")
}

func TestRuntime157Captures(t *testing.T) {
	for _, name := range []string{"rejected-command", "headless-canceled", "rule-denied", "task-input", "running", "background-canceled", "strict-edit", "strict-command", "canceled-read", "cron-zero", "cron-canceled", "swarm-models", "append"} {
		t.Run(name, func(t *testing.T) {
			raw, shown := runtime157Pair(t, name)
			s := snapshot1216(t, raw)
			require1216(t, s, raw, shown, Verified)
			offline, e := s.OfflineJSON()
			if e != nil {
				t.Fatal(e)
			}
			require1216(t, s, raw, string(offline), Verified)
			// Retained metadata and every source byte still participate in comparison.
			require1216(t, s, strings.Replace(raw, "fixture", "changed", 1), shown, Incomplete)
			require1216(t, s, raw, `{"cascadeId":"fixture-parent","steps":[]}`, Incomplete)
			bad := replace1216(t, raw, `"args": {`, `"args": {"Unexpected":"value",`)
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
}

func TestRuntime157Boundaries(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"rejected-command", `user denied permission to run command:`, `another failure:`},
		{"task-input", `"input": "`, `"input": "changed`},
		{"background-canceled", `"taskDetails": {`, `"taskDetails": {"isDaemon": true,`},
		{"task-input", `"action": "send_input"`, `"action": "future"`},
		{"rule-denied", `Matches user-configured deny rule.`, `Another failure.`},
		{"canceled-read", `CORTEX_STEP_STATUS_CANCELED`, `CORTEX_STEP_STATUS_DONE`},
		{"background-canceled", `CORTEX_STEP_STATUS_CANCELED`, `CORTEX_STEP_STATUS_ERROR`},
		{"swarm-models", `"all": "true"`, `"all": "false"`},
		{"append", `"Append": "true"`, `"Append": "false"`},
		{"strict-edit", `"allow": true`, `"allow": false`},
		{"strict-command", `"allow": true`, `"allow": false`},
		{"strict-edit", `TYPE_EFFECTIVE_GRANT`, `TYPE_DEFAULT_FALLBACK`},
	} {
		t.Run(tc.name+tc.before, func(t *testing.T) {
			raw, shown := runtime157Pair(t, tc.name)
			bad := replace1216(t, raw, tc.before, tc.after)
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
}
