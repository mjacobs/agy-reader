package completeness

import (
	"strings"
	"testing"
)

// Synthetic pairs follow the native 1.5.4 source/full/default captures.
func emptyRead154Pair() (string, string) {
	f := tool143Fixtures[2]
	f.payload = `"@type":"type.googleapis.com/exa.cortex.core.tools.ReadFile","path":"/fixture/empty.txt"`
	f.args = `"Path":"/fixture/empty.txt"`
	f.presented = `"type":"CORTEX_STEP_TYPE_VIEW_FILE","viewFile":{"absolutePathUri":"file:///fixture/empty.txt","content":"File is empty"}`
	raw, shown := f.pair()
	return strings.ReplaceAll(raw, "fixture result", "File is empty"), shown
}

func feedback154Pair(t *testing.T) (string, string) {
	t.Helper()
	raw, shown := ide225ArtifactPair(true)
	raw = strings.ReplaceAll(raw, `\"RequestFeedback\":false`, `\"RequestFeedback\":true`)
	shown = strings.ReplaceAll(shown, `\"RequestFeedback\":false`, `\"RequestFeedback\":true`)
	raw = replace1216(t, raw, `"kind":"CREATE"`, `"kind":"OVERWRITE"`)
	raw = strings.ReplaceAll(raw, `"artifactMetadata":{`, `"artifactMetadata":{"requestFeedback":true,`)
	shown = strings.ReplaceAll(shown, `"artifactMetadata":{`, `"artifactMetadata":{"requestFeedback":true,`)
	shown = replace1216(t, shown, `"path":{"absoluteUri":"file:///fixture/empty.txt"}}`, `"path":{"absoluteUri":"file:///fixture/empty.txt"},"overwrite":true}`)
	shown = replace1216(t, shown, `,"createFile":true`, ``)
	return raw, shown
}

func TestTools154Qualified(t *testing.T) {
	a, b := emptyRead154Pair()
	c, d := feedback154Pair(t)
	for name, pair := range map[string][2]string{"empty read": {a, b}, "feedback overwrite": {c, d}} {
		t.Run(name, func(t *testing.T) {
			raw, shown := pair[0], pair[1]
			s := snapshot1216(t, raw)
			require1216(t, s, raw, shown, Verified)
			offline, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, raw, string(offline), Verified)
			require1216(t, s, strings.Replace(raw, `"result":"`, `"result":"changed `, 1), shown, Incomplete)
		})
	}
	require1216(t, snapshot1216(t, a), a, replace1216(t, b, `"content":"File is empty"`, `"content":"changed"`), Incomplete)
	require1216(t, snapshot1216(t, c), c, strings.ReplaceAll(d, `"requestFeedback":true`, `"requestFeedback":false`), Incomplete)
}

func TestTools154Bounds(t *testing.T) {
	raw, shown := emptyRead154Pair()
	for _, field := range []string{"startLine", "endLine", "fileNumLines", "fileNumBytes", "lineRangeBytes"} {
		t.Run(field, func(t *testing.T) {
			bad := replace1216(t, raw, `"path":"/fixture/empty.txt"`, `"path":"/fixture/empty.txt","`+field+`":1`)
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
	bad := replace1216(t, raw, `"result":"File is empty"`, `"result":"unexpected content"`)
	require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
	raw, shown = feedback154Pair(t)
	for _, change := range [][2]string{
		{`"artifactMetadata":{"requestFeedback":true,`, `"artifactMetadata":{`},
		{`"kind":"OVERWRITE"`, `"kind":"CREATE"`},
		{`"isArtifactFile":true`, `"isArtifactFile":false`},
		{`"Overwrite":"true"`, `"Overwrite":"false"`},
		{`\"RequestFeedback\":true`, `\"RequestFeedback\":false`},
	} {
		bad := replace1216(t, raw, change[0], change[1])
		require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
	}
}

func TestTools154CanceledBypass(t *testing.T) {
	raw := strings.ReplaceAll(background144Raw, `,`+background144Task, ``)
	shown := strings.ReplaceAll(background144Shown, `,`+background144Task, ``)
	for _, p := range []*string{&raw, &shown} {
		*p = strings.ReplaceAll(*p, `"metadata":{`, `"metadata":{"completedAt":"2026-01-01T00:00:02Z",`)
		*p = strings.ReplaceAll(*p, `\"WaitMsBeforeAsync\":5000,`, `\"WaitMsBeforeAsync\":5000,\"BypassSandbox\":true,`)
	}
	raw = replace1216(t, raw, `"args":{`, `"args":{"BypassSandbox":"true",`)
	shown = replace1216(t, shown, `"combinedOutput":{}`, `"blocking":true,"combinedOutput":{}`)
	require1216(t, snapshot1216(t, raw), raw, shown, Verified)
	for _, change := range [][2]string{
		{`"completedAt":"2026-01-01T00:00:02Z",`, ``},
		{`"BypassSandbox":"true"`, `"BypassSandbox":"false"`},
		{`"WaitMsBeforeAsync":"5000"`, `"WaitMsBeforeAsync":"1000"`},
	} {
		bad := replace1216(t, raw, change[0], change[1])
		require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
	}
	require1216(t, snapshot1216(t, raw), raw, replace1216(t, shown, `"blocking":true,`, ``), Incomplete)
}
