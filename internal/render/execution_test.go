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

func TestEmbeddedFencesCannotConsumeFollowingStep(t *testing.T) {
	body := "before\n```go\nfmt.Println(1)\n```\nafter"
	encoded, _ := json.Marshal(body)
	for name, step := range map[string]string{
		"user":           `{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":` + string(encoded) + `}}`,
		"command":        `{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","runCommand":{"combinedOutput":{"full":` + string(encoded) + `}}}`,
		"file":           `{"type":"CORTEX_STEP_TYPE_VIEW_FILE","viewFile":{"content":` + string(encoded) + `}}`,
		"media caption":  `{"type":"CORTEX_STEP_TYPE_VIEW_FILE","viewFile":{"mediaData":{"mimeType":"image/png","uri":"file:///fixture/image.png"},"content":` + string(encoded) + `}}`,
		"model error":    `{"type":"CORTEX_STEP_TYPE_ERROR_MESSAGE","errorMessage":{"error":{"modelErrorMessage":` + string(encoded) + `}}}`,
		"diff":           `{"type":"CORTEX_STEP_TYPE_CODE_ACTION","codeAction":{"actionResult":{"edit":{"diff":{"unifiedDiff":{"lines":[{"type":"UNIFIED_DIFF_LINE_TYPE_INSERT","text":` + string(encoded) + `}]}}}}}}`,
		"generic result": `{"type":"CORTEX_STEP_TYPE_GENERIC","generic":{"result":{"result":` + string(encoded) + `}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var trajectory daemon.Trajectory
			raw := `{"cascadeId":"fixture","steps":[` + step + `,{"type":"CORTEX_STEP_TYPE_SYSTEM_MESSAGE","systemMessage":{"message":"following step"}}]}`
			if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
				t.Fatal(err)
			}
			output := buf.String()
			if !strings.Contains(output, "````") {
				t.Fatal("embedded backticks did not widen the fence")
			}
			// Follow Markdown's backtick closing rule while scanning to the next
			// step. Its heading and body must remain outside a code block.
			assertFollowingStepOutsideFence(t, output)
		})
	}
}

func TestMarkdownIncludesExecutionRecordsAndTaskContentBlocks(t *testing.T) {
	raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","metadata":{"toolCall":{"name":"manage_task"},"preToolHookResults":[{"decision":"auto_approve","reason":"fixture hook"}]},"permissions":{"permissionGrants":[{"action":"write_file","target":"/fixture/note.txt","allow":true}]},"completedInteractions":[{"response":{"permission":{"allow":true}}}],"taskDetails":{"id":"fixture/task-1","progress":"waiting for input"},"generic":{"args":{"Action":"status"},"result":{"result":"task output","metadata":{"action":"status"},"stepRenderInfo":{"contentBlocks":[{"markdown":"MARKDOWN_FIXTURE"},{"unknownFixtureBlock":{"text":"retained block"}}]}}}}]}`
	markdown, _ := json.Marshal("**Status**: RUNNING\n\n````\n```go\n````")
	raw = strings.Replace(raw, `"MARKDOWN_FIXTURE"`, string(markdown), 1)
	var trajectory daemon.Trajectory
	if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"### manage_task", "task output", "**Status**: RUNNING", "````\n```go\n````", "retained block", "### Pre-tool hook results", "auto_approve", "fixture hook", "### Permissions", "write_file", "### Completed interactions", "### Background task", "waiting for input", "Stored tool details"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestSavedMarkdownCannotConsumeFollowingStep(t *testing.T) {
	for _, fragment := range []string{"````text\npartial output", "~~~text\npartial output", "<details>\n<pre>\npartial output", "**Status**: RUNNING\n\n````\n```go\n````"} {
		encoded, _ := json.Marshal(fragment)
		raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","generic":{"result":{"stepRenderInfo":{"contentBlocks":[{"markdown":` + string(encoded) + `}]}}}},{"type":"CORTEX_STEP_TYPE_SYSTEM_MESSAGE","systemMessage":{"message":"following step"}}]}`
		var trajectory daemon.Trajectory
		if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		// A saved fragment must be literal content in its own dynamic fence,
		// including HTML and unfinished fences. It must never enter the document
		// as live Markdown before the next step.
		assertFollowingStepOutsideFence(t, buf.String())
		if !strings.Contains(buf.String(), "markdown\n"+fragment+"\n") {
			t.Fatalf("saved display was emitted as live Markdown: %q", fragment)
		}
	}
}

func assertFollowingStepOutsideFence(t *testing.T, output string) {
	t.Helper()
	fence := 0
	seen := false
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "## Step 2:") {
			seen = true
			if fence != 0 {
				t.Fatal("following step swallowed by code fence")
			}
		}
		trim := strings.TrimLeft(line, " ")
		n := len(trim) - len(strings.TrimLeft(trim, "`"))
		if n >= 3 {
			if fence == 0 {
				fence = n
			} else if n >= fence && strings.TrimSpace(trim[n:]) == "" {
				fence = 0
			}
		}
	}
	if !seen || fence != 0 {
		t.Fatalf("missing step or unclosed fence:\n%s", output)
	}
}
