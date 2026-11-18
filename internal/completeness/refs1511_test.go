package completeness

import (
	"strings"
	"testing"
)

func TestFileReferenceSizes1511(t *testing.T) {
	raw, shown := schedule1510Pair(t, "pdf")
	const uri = `"uri": "file:///fixture/manual.pdf"`
	for _, sizes := range []string{
		`"totalLines": 0, "totalBytes": "0"`,
		`"totalLines": 18, "totalBytes": "4294967296"`,
	} {
		full := replace1216(t, raw, uri, uri+", "+sizes)
		source := snapshot1216(t, full)
		require1216(t, source, full, shown, Verified)
		require1216(t, source, raw, shown, Incomplete)
	}
}

func TestCompletedBackgroundCancellation1511(t *testing.T) {
	full, ordinary := tools158Pair(t, "canceled-command")
	raw := strings.ReplaceAll(full, "10000", "500")
	shown := strings.ReplaceAll(ordinary, "10000", "500")
	source := snapshot1216(t, raw)
	require1216(t, source, raw, shown, Verified)
	require1216(t, source, raw, ordinary, Incomplete)
	for _, changed := range []string{
		strings.ReplaceAll(raw, "500", "501"),
		strings.ReplaceAll(raw, "CORTEX_COMPONENT_SOURCE_BUILTIN", "CORTEX_COMPONENT_SOURCE_UNSPECIFIED"),
	} {
		require1216(t, snapshot1216(t, changed), changed, shown, Unknown)
	}
}

func TestAgentReferenceLocations1511(t *testing.T) {
	full, ordinary := workflow159Pair(t, "agents")
	const title = `"title": "Parallel alpha"`
	const enriched = title + `, "logUri": "file:///fixture/log", "workspaceUris": ["file:///fixture/workspace"]`
	raw := strings.ReplaceAll(full, title, enriched)
	shown := strings.ReplaceAll(ordinary, title, enriched)
	source := snapshot1216(t, raw)
	require1216(t, source, raw, shown, Verified)
	require1216(t, source, full, shown, Incomplete)
	require1216(t, source, raw, ordinary, Incomplete)
}
