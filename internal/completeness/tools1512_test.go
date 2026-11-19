package completeness

import (
	"strings"
	"testing"
)

func TestTimestampedLiteralSkill1512(t *testing.T) {
	content := "Created At: 2026-01-01T00:00:01Z\nCompleted At: 2026-01-01T00:00:02Z\nShowing lines 1-7 of 7\n1: ---\n2: name: sample\n3: description: |\n4:   First.\n5: \n6:   Second.\n7: ---"
	raw, old := skill150Pair(content)
	shown := replace1216(t, old, `"isSkillFile":true,`, `"isSkillFile":true,"skillMetadata":{"name":"sample","description":"First.\n\nSecond."},`)
	s := snapshot1216(t, raw)
	require1216(t, s, raw, shown, Verified)
	require1216(t, s, raw, strings.Replace(shown, `First.\n\nSecond.`, "First. Second.", 1), Incomplete)
}

func TestHeadlessBypass1512(t *testing.T) {
	raw, shown := runtime157Pair(t, "headless-canceled")
	addFlag := func(s string) string {
		return replace1216(t, s, `{\"CommandLine\":`, `{\"BypassSandbox\":true,\"CommandLine\":`)
	}
	raw = replace1216(t, addFlag(raw), `"args": {`, `"args": {"BypassSandbox":"true",`)
	shown = addFlag(shown)
	require1216(t, snapshot1216(t, raw), raw, shown, Verified)
	bad := replace1216(t, raw, `\"BypassSandbox\":true`, `\"BypassSandbox\":\"true\"`)
	require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
}

func TestTimerSender1512(t *testing.T) {
	raw, shown := schedule1510Pair(t, "cancelled")
	early := strings.NewReplacer("never", "any", "Wait cancelled.", "Timer cancelled early: its early-termination condition (any) was met by a message from child/task-1.", `"outcome": "cancelled"`, `"outcome": "cancelled_early", "cancelledBy": "child/task-1"`)
	raw, shown = early.Replace(raw), early.Replace(shown)
	// Payload uses protobuf JSON names. The derived metadata uses snake case.
	i := strings.LastIndex(shown, `"cancelledBy"`)
	shown = shown[:i] + strings.Replace(shown[i:], "cancelledBy", "cancelled_by", 1)
	s := snapshot1216(t, raw)
	require1216(t, s, raw, shown, Verified)
	bad := replace1216(t, raw, "message from child/task-1.", "message from other.")
	require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
}
