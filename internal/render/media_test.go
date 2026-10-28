package render_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

// Both message fields contain codeium_common_pb.Media. User media predates
// 1.4.9; planner media is new in 1.4.9. The bytes here are synthetic.
func TestMarkdownMessageAttachments(t *testing.T) {
	for _, tc := range []struct{ stepType, field, heading string }{
		{"USER_INPUT", "userInput", "User Attachments"},
		{"PLANNER_RESPONSE", "plannerResponse", "Agent Attachments"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			// An attachment-only message must be visible even without text.
			raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_` + tc.stepType + `","` + tc.field + `":{"media":[
				{"mimeType":"image/png","displayName":"pasted image.png","description":"A small test image","mediaId":"image-1","uri":"/fixture/pasted image.png","inlineData":"UEFZTE9BRA==","thumbnail":"VEhVTUI="},
				{"mimeType":"application/pdf","displayName":"plan.pdf","blobref":{"blobId":"blob-2"},"startPage":0,"endPage":2},
				{}
			]}}]}`
			out := renderString(t, decodeTrajectory(t, raw))
			for _, want := range []string{
				"### " + tc.heading, "**Attachment 1:**", "**Attachment 2:**", "**Attachment 3:**",
				"**Media Type:** `image/png`", "**Display Name:** `pasted image.png`", "**Description:** `A small test image`",
				"**Media ID:** `image-1`", "file:///fixture/pasted%20image.png",
				"**Inline Data:** embedded, not shown (12 base64 characters)", "**Thumbnail:** embedded, not shown (8 base64 characters)",
				"**Blob ID:** `blob-2` (reference only)", "**Start Page:** 0", "**End Page:** 2", "*No stored attachment reference.*",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("missing %q in:\n%s", want, out)
				}
			}
			for _, payload := range []string{"UEFZTE9BRA==", "VEhVTUI="} {
				if strings.Contains(out, payload) {
					t.Errorf("embedded payload leaked: %s", payload)
				}
			}
		})
	}
}

func TestMarkdownMessageAttachmentsEscapeMetadata(t *testing.T) {
	media := []daemon.Media{
		{DisplayName: "`\n# injected", Description: "<script>bad</script>", URI: "javascript:alert(1)"},
		{URI: "file:///fixture/a](javascript:alert(1)).png"},
		{URI: " data:image/png;base64,SElEREVO "},
	}
	trajectory := &daemon.Trajectory{Steps: []daemon.Step{
		{Type: "CORTEX_STEP_TYPE_USER_INPUT", UserInput: &daemon.UserInput{Media: media}},
		{Type: "CORTEX_STEP_TYPE_PLANNER_RESPONSE", PlannerResponse: &daemon.PlannerResponse{Media: media}},
	}}
	out := renderString(t, trajectory)
	for _, forbidden := range []string{"\n# injected", "](javascript:", "SElEREVO", "**Description:** <script>"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("unsafe metadata %q in:\n%s", forbidden, out)
		}
	}
	for _, want := range []string{"`javascript:alert(1)`", "file:///fixture/a%5D%28javascript:alert%281%29%29.png", "data URI"} {
		if strings.Count(out, want) != 2 {
			t.Errorf("missing safe rendering for both message types: %q in:\n%s", want, out)
		}
	}
}

func TestMarkdownMessagesWithoutAttachments(t *testing.T) {
	var trajectory daemon.Trajectory
	if err := json.Unmarshal([]byte(`{"steps":[
		{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":"hello","media":[]}},
		{"type":"CORTEX_STEP_TYPE_PLANNER_RESPONSE","plannerResponse":{"response":"world","media":null}}
	]}`), &trajectory); err != nil {
		t.Fatal(err)
	}
	out := renderString(t, &trajectory)
	if strings.Contains(out, "Attachment") || !strings.Contains(out, "hello") || !strings.Contains(out, "world") {
		t.Fatalf("text-only messages changed:\n%s", out)
	}
}
