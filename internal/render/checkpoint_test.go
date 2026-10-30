package render_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/render"
)

func renderCheckpointJSON(t *testing.T, checkpoint string) string {
	t.Helper()
	var traj daemon.Trajectory
	if err := json.Unmarshal([]byte(`{"steps":[{"type":"CORTEX_STEP_TYPE_CHECKPOINT","checkpoint":`+checkpoint+`}]}`), &traj); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := render.Markdown(&buf, &traj, time.Time{}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// Synthetic protojson follows CapturedUserRequest and Media in the agy 1.4.6
// snapshot. No private trajectory or attachment bytes are fixture material.
func TestCheckpointCapturedRequestsAndAttachments(t *testing.T) {
	out := renderCheckpointJSON(t, `{
		"capturedUserRequests":[
			{"text":"Compare these files.\nKeep the annotations.","media":[
				{"mimeType":"application/pdf","description":"Annotated plan","displayName":"plan.pdf","mediaId":"media-1","uri":"file:///fixture/plan.pdf","startPage":0,"endPage":2,"thumbnail":"VEhVTUI="},
				{"mimeType":"image/png","blobref":{"blobId":"blob-2"},"inlineData":"SU1BR0U="}
			]},
			{"media":[{"displayName":"missing.png"},{}]},
			{"text":"Compare these files.\nKeep the annotations.","media":[{"blobref":{"blobId":"blob-3"}}]}
		],
		"sessionSummary":"Summary survives."
	}`)
	for _, want := range []string{
		"**Captured User Requests:**", "#### Captured Request 1", "#### Captured Request 2", "#### Captured Request 3",
		"Compare these files.\nKeep the annotations.", "**Attachment 1:**", "**Attachment 2:**",
		"**Display Name:** `plan.pdf`", "**Media Type:** `application/pdf`", "**Description:** `Annotated plan`",
		"**Media ID:** `media-1`", "file:///fixture/plan.pdf", "**Start Page:** 0", "**End Page:** 2",
		"**Blob ID:** `blob-2` (reference only)", "**Blob ID:** `blob-3` (reference only)",
		"**Thumbnail:** embedded, not shown (8 base64 characters)", "**Inline Data:** embedded, not shown (8 base64 characters)",
		"*No request text stored.*", "*No stored attachment reference.*", "Summary survives.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	for _, secret := range []string{"SU1BR0U=", "VEhVTUI="} {
		if strings.Contains(out, secret) {
			t.Errorf("embedded payload leaked: %s", secret)
		}
	}
	if count := strings.Count(out, "Compare these files."); count != 2 {
		t.Errorf("distinct captured requests were merged: count=%d\n%s", count, out)
	}
}

func TestCheckpointLegacyAndMixedRequests(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		captured    bool
	}{
		{"legacy", `{"userRequests":["first","second"],"sessionSummary":"old summary"}`, false},
		{"mixed", `{"userRequests":["first","second"],"capturedUserRequests":[{"text":"first"}],"sessionSummary":"old summary"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := renderCheckpointJSON(t, tc.input)
			if !strings.Contains(out, "**User Requests:** first, second\n") || !strings.Contains(out, "old summary") {
				t.Fatalf("legacy text or summary lost:\n%s", out)
			}
			if strings.Contains(out, "**Captured User Requests:**") != tc.captured {
				t.Fatalf("unexpected captured section:\n%s", out)
			}
			if tc.captured && strings.Count(out, "first") != 2 {
				t.Fatalf("mixed fields silently deduplicated:\n%s", out)
			}
		})
	}
}

func TestCheckpointCapturedParentMessagesAndAttachments(t *testing.T) {
	out := renderCheckpointJSON(t, `{
		"capturedParentMessages":[
			{"text":"Instructions from parent agent.","media":[
				{"mimeType":"text/plain","displayName":"context.txt","uri":"file:///fixture/context.txt"}
			]},
			{}
		],
		"sessionSummary":"Summary survives."
	}`)
	for _, want := range []string{
		"**Captured Parent Messages:**", "#### Captured Parent Message 1", "#### Captured Parent Message 2",
		"Instructions from parent agent.", "**Attachment 1:**",
		"**Display Name:** `context.txt`", "**Media Type:** `text/plain`",
		"file:///fixture/context.txt", "*No message text stored.*", "Summary survives.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestCheckpointAttachmentMetadataCannotEscapeMarkdown(t *testing.T) {
	payload := map[string]any{"capturedUserRequests": []any{map[string]any{
		"text": "Request with ``` fences\n<script>request</script>",
		"media": []any{
			map[string]any{"displayName": "`\n# injected", "description": "<script>alert(1)</script>", "uri": "javascript:alert(1)"},
			map[string]any{"uri": "file:///fixture/a](javascript:alert(1)).png"},
			map[string]any{"uri": "https://example.test/<script>.png"},
			map[string]any{"uri": "data:image/png;base64,SElEREVO"},
			map[string]any{"uri": " data:image/png;base64,SElEREVO "},
			map[string]any{"uri": "https://example.test/\n# injected"},
			map[string]any{"uri": "http://[invalid"},
		},
	}}}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	out := renderCheckpointJSON(t, string(raw))
	for _, forbidden := range []string{"\n# injected", "](javascript:", "SElEREVO", "<script>.png", "**Description:** <script>"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("unsafe metadata %q in:\n%s", forbidden, out)
		}
	}
	for _, want := range []string{"`javascript:alert(1)`", "file:///fixture/a%5D%28javascript:alert%281%29%29.png", "https://example.test/%3Cscript%3E.png", "data URI", "````\nRequest with ``` fences\n<script>request</script>\n````", "`http://[invalid`"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing safe rendering %q in:\n%s", want, out)
		}
	}
}

func TestCheckpointNativeAbsoluteAttachmentPaths(t *testing.T) {
	for _, tc := range []struct {
		name, path, reference string
	}{
		{"absolute", "/fixture/photo.png", "file:///fixture/photo.png"},
		{"space", "/fixture/report draft.pdf", "file:///fixture/report%20draft.pdf"},
		{"fragment", "/fixture/report#2.pdf", "file:///fixture/report%232.pdf"},
		{"brackets", "/fixture/photo[1](final).png", "file:///fixture/photo%5B1%5D%28final%29.png"},
		{"percent", "/fixture/100%25.txt", "file:///fixture/100%2525.txt"},
		{"relative", "fixture/photo.png", "`fixture/photo.png`"},
		{"relative parent", "../fixture/photo.png", "`../fixture/photo.png`"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cp := daemon.Checkpoint{CapturedUserRequests: []daemon.CapturedUserRequest{{Media: []daemon.CheckpointMedia{{URI: tc.path}}}}}
			raw, err := json.Marshal(cp)
			if err != nil {
				t.Fatal(err)
			}
			out := renderCheckpointJSON(t, string(raw))
			if !strings.Contains(out, tc.reference) {
				t.Fatalf("missing path reference %q:\n%s", tc.reference, out)
			}
			if strings.HasPrefix(tc.path, "/") {
				if !strings.Contains(out, "]("+tc.reference+")") {
					t.Fatalf("absolute path is not clickable:\n%s", out)
				}
			} else if strings.Contains(out, "](") {
				t.Fatalf("relative path became a link:\n%s", out)
			}
		})
	}
}
