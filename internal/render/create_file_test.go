package render_test

import (
	"strings"
	"testing"
)

// write_file steps present a createFile spec rather than a command. Shapes
// follow agy 1.4.11: a creation, an overwrite of an existing file, and a call
// refused because the file existed.
func TestMarkdownCreateFileActions(t *testing.T) {
	const diff = `"actionResult":{"edit":{"absoluteUri":"file:///fixture/notes.txt","diff":{"unifiedDiff":{"lines":[{"text":"new","type":"UNIFIED_DIFF_LINE_TYPE_INSERT"}]}}}}`
	for _, tc := range []struct {
		name, action string
		replaced     bool
		asked        bool
	}{
		{"create", `{"actionSpec":{"createFile":{"path":{"absoluteUri":"file:///fixture/notes.txt"}}},` + diff + `}`, false, false},
		{"overwrite", `{"actionSpec":{"createFile":{"path":{"absoluteUri":"file:///fixture/notes.txt"},"overwrite":true}},` + diff + `}`, true, false},
		{"failed", `{"actionSpec":{"createFile":{"path":{"absoluteUri":"file:///fixture/notes.txt"}}}}`, false, false},
		// Review rv-sbed.1: a failed overwrite must not claim the file was replaced.
		{"failed overwrite", `{"actionSpec":{"createFile":{"path":{"absoluteUri":"file:///fixture/notes.txt"},"overwrite":true}}}`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := renderString(t, decodeTrajectory(t, `{"steps":[{"type":"CORTEX_STEP_TYPE_CODE_ACTION","codeAction":`+tc.action+`}]}`))
			if !strings.Contains(out, "**File:** [file:///fixture/notes.txt](file:///fixture/notes.txt)") {
				t.Errorf("missing file in:\n%s", out)
			}
			if got := strings.Contains(out, "*Replaced the existing file.*"); got != tc.replaced {
				t.Errorf("replaced note = %v in:\n%s", got, out)
			}
			if got := strings.Contains(out, "*Asked to replace the existing file.*"); got != tc.asked {
				t.Errorf("requested note = %v in:\n%s", got, out)
			}
		})
	}
}

// Review rv-4smn.1: edit and create-file steps must encode a file name the same
// way as read and grep links, so # does not become a fragment.
func TestMarkdownEditAndCreateFileLinksEncodeNames(t *testing.T) {
	const want = "**File:** [file:///fixture/Project Notes #1 \\&amp; \\_x\\_.md](file:///fixture/Project%20Notes%20%231%20%26amp;%20_x_.md)"
	for _, tc := range []struct{ name, action string }{
		{"edit", `{"actionSpec":{"command":{"isEdit":true,"file":{"absoluteUri":"file:///fixture/Project Notes #1 &amp; _x_.md"}}}}`},
		{"create", `{"actionSpec":{"createFile":{"path":{"absoluteUri":"file:///fixture/Project Notes #1 &amp; _x_.md"}}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := renderString(t, decodeTrajectory(t, `{"steps":[{"type":"CORTEX_STEP_TYPE_CODE_ACTION","codeAction":`+tc.action+`}]}`))
			if !strings.Contains(out, want) {
				t.Errorf("want %q in:\n%s", want, out)
			}
		})
	}
}
