package completeness

import (
	"testing"
)

// Synthetic values follow a native agy 1.4.11 run_command captured twice while
// it ran (before and after it moved to the background, which adds task
// details) and once after it completed. The source matched the full and
// default RPC of the daemon running it each time.
const running1411Meta = `"metadata":{"toolCall":{"name":"run_command","argumentsJson":"{\"CommandLine\":\"sleep 45\",\"IsDaemon\":false,\"WaitMsBeforeAsync\":10000,\"toolSummary\":\"Fixture command\"}"},"startedAt":"2026-01-01T00:00:01Z","preToolHookResults":[{"decision":"allow"}]}`
const running1411Task = `"taskDetails":{"id":"fixture/task-2","logUri":"file:///fixture/task-2.log","description":"sleep 45","title":"Fixture command","movedToBackgroundAt":"2026-01-01T00:00:11Z"},`
const running1411Generic = `"generic":{"args":{"CommandLine":"sleep 45","IsDaemon":"false","WaitMsBeforeAsync":"10000","toolSummary":"Fixture command"},"result":{"stepRenderInfo":{"titlePrefix":"Running","icon":"terminal","contentBlocks":[{"terminal":{"command":"sleep 45","cwd":"/fixture"}}]}}}`

func running1411Raw(task string) string {
	return `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_RUNNING",` + running1411Meta + `,` + task + running1411Generic + `}]}`
}

var running1411Shapes = map[string]string{
	"foreground": running1411Raw(""),
	"background": running1411Raw(running1411Task),
}

func TestRunning1411Qualified(t *testing.T) {
	for name, raw := range running1411Shapes {
		t.Run(name, func(t *testing.T) {
			s := snapshot1216(t, raw)
			require1216(t, s, raw, raw, Verified)
			offline, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, raw, string(offline), Verified)
		})
	}
}

func TestRunning1411Changes(t *testing.T) {
	for name, raw := range running1411Shapes {
		for _, change := range [][2]string{
			{`"command":"sleep 45"`, `"command":"sleep 46"`},
			{`CORTEX_STEP_STATUS_RUNNING`, `CORTEX_STEP_STATUS_DONE`},
			{`"titlePrefix":"Running"`, `"titlePrefix":"Ran"`},
		} {
			t.Run(name+" "+change[1], func(t *testing.T) {
				require1216(t, snapshot1216(t, raw), raw, replace1216(t, raw, change[0], change[1]), Incomplete)
			})
		}
	}
}

func TestRunning1411RefusesUnqualifiedShapes(t *testing.T) {
	for name, raw := range running1411Shapes {
		for _, tc := range []struct{ name, before, after string }{
			{"result text", `"result":{"stepRenderInfo"`, `"result":{"result":"partial","stepRenderInfo"`},
			{"outer error", `"generic":{`, `"error":{"shortError":"fixture","fullError":"fixture"},"generic":{`},
			{"no hook result", `,"preToolHookResults":[{"decision":"allow"}]`, ``},
			{"daemon command", `"IsDaemon":"false"`, `"IsDaemon":"true"`},
			{"argument not retained", `\"IsDaemon\":false,`, ``},
			{"unobserved argument", `"args":{`, `"args":{"Cwd":"/fixture",`},
			{"other tool", `"name":"run_command"`, `"name":"view_file"`},
		} {
			t.Run(name+" "+tc.name, func(t *testing.T) {
				if _, err := snapshot1216(t, replace1216(t, raw, tc.before, tc.after)).OfflineJSON(); err == nil {
					t.Fatal("accepted unqualified running command")
				}
			})
		}
		// Review rv-8w87.1: a consistent IsDaemon=true must also be refused.
		t.Run(name+" daemon command in both copies", func(t *testing.T) {
			bad := replace1216(t, replace1216(t, raw, `"IsDaemon":"false"`, `"IsDaemon":"true"`), `\"IsDaemon\":false`, `\"IsDaemon\":true`)
			s := snapshot1216(t, bad)
			if _, err := s.OfflineJSON(); err == nil {
				t.Fatal("accepted a running daemon command")
			}
			require1216(t, s, bad, bad, Unknown)
		})
	}
}

// The completed command from the same capture keeps the modern RunCommand
// mapping; agy 1.4.11 added the model's IsDaemon=false argument to it.
const daemon1411Meta = `"metadata":{"toolCall":{"name":"run_command","argumentsJson":"{\"CommandLine\":\"sleep 45\",\"IsDaemon\":false,\"toolSummary\":\"Fixture command\"}"},"completedAt":"2026-01-01T00:00:46Z"},` + running1411Task
const daemon1411Raw = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE",` + daemon1411Meta + `"generic":{"args":{"CommandLine":"sleep 45","IsDaemon":"false","toolSummary":"Fixture command"},"result":{"result":"Command exited with code 0.","fullOutputUri":"file:///fixture/output.txt","payload":{"@type":"type.googleapis.com/exa.cortex.core.tools.RunCommand","sandboxDetails":{"ranInSandbox":true},"shellName":"bash","commandLine":"sleep 45","cwd":"/fixture"},"stepRenderInfo":{"titlePrefix":"Ran","icon":"terminal"}}}}]}`
const daemon1411Shown = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_DONE",` + daemon1411Meta + `"runCommand":{"commandLine":"sleep 45","proposedCommandLine":"sleep 45","cwd":"/fixture","exitCode":0,"combinedOutput":{"full":"Command exited with code 0."},"shellName":"bash","ranInSandbox":true}}]}`

func TestNotDaemon1411(t *testing.T) {
	s := snapshot1216(t, daemon1411Raw)
	require1216(t, s, daemon1411Raw, daemon1411Shown, Verified)
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, daemon1411Raw, string(offline), Verified)
	require1216(t, s, daemon1411Raw, replace1216(t, daemon1411Shown, `"ranInSandbox":true}`, `"ranInSandbox":true,"isDaemon":true}`), Incomplete)
	for _, tc := range []struct{ name, before, after string }{
		{"daemon true", `"IsDaemon":"false"`, `"IsDaemon":"true"`},
		{"retained literal absent", `\"IsDaemon\":false,`, ``},
		{"retained literal differs", `\"IsDaemon\":false`, `\"IsDaemon\":true`},
		{"retained as string", `\"IsDaemon\":false`, `\"IsDaemon\":\"false\"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := snapshot1216(t, replace1216(t, daemon1411Raw, tc.before, tc.after)).OfflineJSON(); err == nil {
				t.Fatal("accepted unqualified IsDaemon argument")
			}
		})
	}
	both := replace1216(t, replace1216(t, daemon1411Raw, `"IsDaemon":"false"`, `"IsDaemon":"true"`), `\"IsDaemon\":false`, `\"IsDaemon\":true`)
	if _, err := snapshot1216(t, both).OfflineJSON(); err == nil {
		t.Fatal("accepted a completed daemon command")
	}
}
