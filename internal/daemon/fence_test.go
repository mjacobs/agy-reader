package daemon

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDiffFenceAllocationDoesNotGrowPerBacktick(t *testing.T) {
	raw, err := json.Marshal(map[string]any{"edit": map[string]any{"diff": map[string]any{"unifiedDiff": map[string]any{"lines": []map[string]string{{"type": "UNIFIED_DIFF_LINE_TYPE_INSERT", "text": strings.Repeat("`", 8192)}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	action := CodeAction{ActionResult: raw}
	allocations := testing.AllocsPerRun(1, func() { _ = action.FormattedDiff() })
	t.Logf("diff fence allocations for 8192 backticks: %.0f", allocations)
	if allocations > 100 {
		t.Fatalf("fence selection allocates per backtick: %.0f allocations", allocations)
	}
}

func TestBacktickDelimiterLargeRunsAndMinimum(t *testing.T) {
	for _, test := range []struct {
		body           string
		minimum, width int
	}{{"", 3, 3}, {"abc", 1, 1}, {"` a `` b", 3, 3}, {"``` a `", 3, 4}, {"~~", 3, 3}, {strings.Repeat("`", 8<<20), 3, (8 << 20) + 1}} {
		delimiter := BacktickDelimiter(test.body, test.minimum)
		if len(delimiter) != test.width {
			t.Fatalf("delimiter width %d, want %d", len(delimiter), test.width)
		}
		if strings.Trim(delimiter, "`") != "" {
			t.Fatal("invalid backtick delimiter")
		}
	}
}
