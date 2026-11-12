package completeness

import (
	"encoding/json"
	"strings"
	"testing"
)

func retainedCall156(t *testing.T, raw, shown, tool string, arguments map[string]any) (string, string) {
	t.Helper()
	args, err := json.Marshal(arguments)
	if err != nil {
		t.Fatal(err)
	}
	call, err := json.Marshal(map[string]string{"name": tool, "argumentsJson": string(args)})
	if err != nil {
		t.Fatal(err)
	}
	old := `"toolCall":{"name":"` + tool + `"}`
	return replace1216(t, raw, old, `"toolCall":`+string(call)), replace1216(t, shown, old, `"toolCall":`+string(call))
}

func cron156Pair(t *testing.T) (string, string) {
	t.Helper()
	fixture := tool143Fixtures[8]
	fixture.canceled = false
	fixture.payload = `"@type":"type.googleapis.com/exa.cortex.core.tools.Schedule","mode":"cron","outcome":"cancelled","iterations":"2"`
	fixture.args = `"CronExpression":"*/3 * * * *","IsDaemon":"true","Prompt":"fixture"`
	fixture.presented = `"mode":"cron","outcome":"cancelled","iterations":"2"`
	raw, shown := fixture.pair()
	raw = strings.Replace(raw, `"taskDetails":{`, `"taskDetails":{"isDaemon":true,"movedToBackgroundAt":"2026-01-01T00:00:01Z",`, 1)
	shown = strings.Replace(shown, `"taskDetails":{`, `"taskDetails":{"isDaemon":true,"movedToBackgroundAt":"2026-01-01T00:00:01Z",`, 1)
	return retainedCall156(t, raw, shown, "schedule", map[string]any{"CronExpression": "*/3 * * * *", "IsDaemon": true, "Prompt": "fixture"})
}

func appendFalse156Pair(t *testing.T) (string, string) {
	fixture := tool143Fixtures[3]
	fixture.args += `,"Append":"false"`
	raw, shown := fixture.pair()
	return retainedCall156(t, raw, shown, "write_file", map[string]any{"Path": "/fixture/empty.txt", "Content": "", "Overwrite": true, "Append": false})
}

func artifactEdit156Pair(t *testing.T) (string, string) {
	fixture := tool143Fixtures[4]
	fixture.args += `,` + ide225ArtifactArg
	fixture.payload += `,"isArtifactFile":true,` + ide225ArtifactMeta
	fixture.presented = strings.Replace(fixture.presented, `"codeAction":{`, `"codeAction":{"isArtifactFile":true,`+ide225ArtifactMeta+`,`, 1)
	raw, shown := fixture.pair()
	return retainedCall156(t, raw, shown, "edit_file", map[string]any{"Path": "/fixture/notes.txt", "Target": "old", "Replacement": "fixture", "ArtifactMetadata": map[string]any{"RequestFeedback": false, "Summary": "Fixture plan", "UserFacing": true}})
}

func completedDaemon156Pair(t *testing.T) (string, string) {
	fixture := tool143Fixtures[0]
	fixture.args = strings.Replace(fixture.args, "5000", "1000", 1) + `,"IsDaemon":"true"`
	fixture.presented = strings.Replace(fixture.presented, `"runCommand":{`, `"runCommand":{"waitMsBeforeAsync":"1000",`, 1)
	raw, shown := fixture.pair()
	return retainedCall156(t, raw, shown, "run_command", map[string]any{"CommandLine": "echo fixture", "WaitMsBeforeAsync": 1000, "toolSummary": "Fixture", "IsDaemon": true})
}

func canceledDaemon156Pair() (string, string) {
	rewrite := strings.NewReplacer(
		`5000`, `1000`,
		`"metadata":{`, `"metadata":{"completedAt":"2026-01-01T00:00:02Z",`,
		`"taskDetails":{`, `"taskDetails":{"isDaemon":true,`,
		`\"WaitMsBeforeAsync\":5000,`, `\"WaitMsBeforeAsync\":1000,\"IsDaemon\":true,`,
	)
	raw, shown := rewrite.Replace(background144Raw), rewrite.Replace(background144Shown)
	raw = strings.Replace(raw, `"args":{`, `"args":{"IsDaemon":"true",`, 1)
	return raw, shown
}

func TestRetained156Qualified(t *testing.T) {
	appendRaw, appendShown := appendFalse156Pair(t)
	editRaw, editShown := artifactEdit156Pair(t)
	cronRaw, cronShown := cron156Pair(t)
	commandRaw, commandShown := completedDaemon156Pair(t)
	canceledRaw, canceledShown := canceledDaemon156Pair()
	overwriteRaw := strings.Replace(overwrite1411Raw, `"kind":"OVERWRITE",`, `"kind":"OVERWRITE","isArtifactFile":true,`, 1)
	overwriteShown := strings.Replace(overwrite1411Shown, `"codeAction":{`, `"codeAction":{"isArtifactFile":true,`, 1)
	for name, pair := range map[string][2]string{
		"non-appending create":                {appendRaw, appendShown},
		"artifact edit metadata":              {editRaw, editShown},
		"artifact overwrite without metadata": {overwriteRaw, overwriteShown},
		"cancelled cron":                      {cronRaw, cronShown},
		"completed daemon command":            {commandRaw, commandShown},
		"canceled daemon command":             {canceledRaw, canceledShown},
	} {
		t.Run(name, func(t *testing.T) {
			snapshot := snapshot1216(t, pair[0])
			require1216(t, snapshot, pair[0], pair[1], Verified)
			offline, err := snapshot.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, snapshot, pair[0], string(offline), Verified)
			// Removing a displayed field or changing wrapper-only source data cannot verify.
			require1216(t, snapshot, pair[0], `{"cascadeId":"fixture-parent","steps":[]}`, Incomplete)
		})
	}
}

func TestRetained156Boundaries(t *testing.T) {
	cron, cronShown := cron156Pair(t)
	for _, change := range [][2]string{
		{`"iterations":"2"`, `"iterations":"0"`},
		{`"iterations":"2"`, `"iterations":"-1"`},
		{`"iterations":"2"`, `"iterations":"2","cancelledBy":"other"`},
		{`"outcome":"cancelled"`, `"outcome":"fired"`},
		{`"CORTEX_STEP_STATUS_DONE"`, `"CORTEX_STEP_STATUS_CANCELED"`},
		{`"isDaemon":true,`, ``},
		{`"movedToBackgroundAt":"2026-01-01T00:00:01Z",`, ``},
		{`"args":{`, `"args":{"Future":"value",`},
	} {
		bad := replace1216(t, cron, change[0], change[1])
		require1216(t, snapshot1216(t, bad), bad, cronShown, Unknown)
	}
	snapshot := snapshot1216(t, cron)
	require1216(t, snapshot, cron, strings.Replace(cronShown, `"metadata":{"mode":"cron","outcome":"cancelled","iterations":"2"}`, `"metadata":{"mode":"cron","outcome":"cancelled","iterations":"3"}`, 1), Incomplete)
	require1216(t, snapshot, strings.Replace(cron, "fixture result", "changed result", 1), cronShown, Incomplete)

	raw, shown := appendFalse156Pair(t)
	bad := strings.NewReplacer(`"Append":"false"`, `"Append":"true"`, `\"Append\":false`, `\"Append\":true`).Replace(raw)
	require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
	bad = replace1216(t, raw, `"Path":"/fixture/empty.txt"`, `"Path":"/fixture/other.txt"`)
	require1216(t, snapshot1216(t, bad), bad, shown, Unknown)

	raw, shown = artifactEdit156Pair(t)
	for _, change := range [][2]string{
		{`"isArtifactFile":true`, `"isArtifactFile":false`},
		{`\"Summary\":\"Fixture plan\"`, `\"Summary\":\"other\"`},
		{`\"RequestFeedback\":false`, `\"RequestFeedback\":true`},
	} {
		bad = replace1216(t, raw, change[0], change[1])
		require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
	}

	raw, shown = completedDaemon156Pair(t)
	for _, change := range [][2]string{
		{`1000`, `3000`},
		{`"CORTEX_STEP_STATUS_DONE"`, `"CORTEX_STEP_STATUS_RUNNING"`},
		{`"generic":{`, `"taskDetails":{"id":"fixture/task"},"generic":{`},
	} {
		bad = replace1216(t, raw, change[0], change[1])
		require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
	}

	raw, shown = canceledDaemon156Pair()
	for _, change := range [][2]string{
		{`1000`, `5000`},
		{`"isDaemon":true,`, ``},
		{`"completedAt":"2026-01-01T00:00:02Z",`, ``},
		{`"preToolHookResults":[{"decision":"allow"}],`, ``},
		{`,{"updatedStatus":"CORTEX_STEP_STATUS_CANCELED"}`, ``},
		{`"args":{`, `"args":{"BypassSandbox":"true",`},
	} {
		bad = replace1216(t, raw, change[0], change[1])
		require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
	}
}

func TestRetained156SkillAndBoard(t *testing.T) {
	content := "Created At: 2026-11-12T00:00:00Z\nCompleted At: 2026-11-12T00:00:01Z\nShowing lines 1-8 of 10\n1: ---\n2: name: sample\n3: description: >-\n4:   First line.\n5:   Second line.\n6: disable-slash-command: true\n7: ---\n8: body"
	values, ok := skillFrontmatter150(content)
	if !ok || values["name"] != "sample" || values["description"] != "First line. Second line." {
		t.Fatal("skill metadata not derived", values)
	}
	for _, bad := range []string{
		strings.Replace(content, "disable-slash-command: true", "disable-slash-command: false", 1),
		strings.Replace(content, "disable-slash-command: true", "unknown: true", 1),
		strings.Replace(content, "description: >-", "description: |", 1),
	} {
		if _, ok := skillFrontmatter150(bad); ok {
			t.Fatal("unqualified frontmatter accepted")
		}
	}
	for _, args := range []map[string]any{{"status": true}, {"seqs": []int{12, 13, 14, 15, 16}}} {
		raw := swarm155Pair("read_board", args)
		require1216(t, snapshot1216(t, raw), raw, raw, Verified)
	}
}
