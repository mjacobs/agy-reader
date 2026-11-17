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
		"user":                  `{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":` + string(encoded) + `}}`,
		"command":               `{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","runCommand":{"combinedOutput":{"full":` + string(encoded) + `}}}`,
		"file":                  `{"type":"CORTEX_STEP_TYPE_VIEW_FILE","viewFile":{"content":` + string(encoded) + `}}`,
		"media caption":         `{"type":"CORTEX_STEP_TYPE_VIEW_FILE","viewFile":{"mediaData":{"mimeType":"image/png","uri":"file:///fixture/image.png"},"content":` + string(encoded) + `}}`,
		"model error":           `{"type":"CORTEX_STEP_TYPE_ERROR_MESSAGE","errorMessage":{"error":{"modelErrorMessage":` + string(encoded) + `}}}`,
		"diff":                  `{"type":"CORTEX_STEP_TYPE_CODE_ACTION","codeAction":{"actionResult":{"edit":{"diff":{"unifiedDiff":{"lines":[{"type":"UNIFIED_DIFF_LINE_TYPE_INSERT","text":` + string(encoded) + `}]}}}}}}`,
		"generic result":        `{"type":"CORTEX_STEP_TYPE_GENERIC","generic":{"result":{"result":` + string(encoded) + `}}}`,
		"mcp result":            `{"type":"CORTEX_STEP_TYPE_MCP_TOOL","mcpTool":{"serverName":"s","toolCall":{"name":"t","argumentsJson":"{}"},"resultString":` + string(encoded) + `}}`,
		"exit plan fence":       `{"type":"CORTEX_STEP_TYPE_GENERIC","metadata":{"toolCall":{"name":"exit_plan_mode"}},"generic":{"args":{"plan":` + string(encoded) + `}}}`,
		"workflow script fence": `{"type":"CORTEX_STEP_TYPE_GENERIC","metadata":{"toolCall":{"name":"run_workflow"}},"generic":{"args":{"Description":"d","Script":` + string(encoded) + `}}}`,
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

func TestRenderPlanModeGenericSteps(t *testing.T) {
	t.Run("enter_plan_mode with result", func(t *testing.T) {
		raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-1","name":"enter_plan_mode"}},"generic":{"args":{"reason":"Revise checks in ` + "`workflow.go`" + ` after failure."},"result":{"result":"Plan mode is on.","fullOutputUri":"file:///fixture/output.txt"}}}]}`
		var trajectory daemon.Trajectory
		if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		for _, want := range []string{
			"### enter_plan_mode",
			"**Reason:**\n```\nRevise checks in `workflow.go` after failure.\n```",
			"**Result:**",
			"Plan mode is on.",
			"<details><summary>Stored tool details</summary>",
			"file:///fixture/output.txt",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
		if strings.Index(out, "**Reason:**") > strings.Index(out, "<details>") {
			t.Fatalf("reason rendered after details:\n%s", out)
		}
	})

	t.Run("enter_plan_mode HTML tag cannot enclose following step", func(t *testing.T) {
		raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","metadata":{"toolCall":{"name":"enter_plan_mode"}},"generic":{"args":{"reason":"Inspect <details> rendering."}}},{"type":"CORTEX_STEP_TYPE_SYSTEM_MESSAGE","systemMessage":{"message":"following step"}}]}`
		var trajectory daemon.Trajectory
		if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if !strings.Contains(out, "**Reason:**\n```\nInspect <details> rendering.\n```") {
			t.Fatalf("expected reason to be fenced, got:\n%s", out)
		}
		assertFollowingStepOutsideFence(t, out)
		beforeStep2, _, ok := strings.Cut(out, "## Step 2:")
		if !ok {
			t.Fatalf("missing Step 2 in:\n%s", out)
		}
		// Only the balanced Stored tool details element should appear outside fences.
		if strings.Count(beforeStep2, "<details>") != 2 || strings.Count(beforeStep2, "</details>") != 1 {
			t.Fatalf("unexpected details balance before Step 2:\n%s", beforeStep2)
		}
	})

	t.Run("enter_plan_mode multiline without result", func(t *testing.T) {
		raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR","metadata":{"toolCall":{"name":"enter_plan_mode"}},"generic":{"args":{"reason":"Line one\nLine two"}}}]}`
		var trajectory daemon.Trajectory
		if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		for _, want := range []string{"### enter_plan_mode", "**Reason:**\n```\nLine one\nLine two\n```", "<details><summary>Stored tool details</summary>"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
	})

	t.Run("exit_plan_mode with and without result", func(t *testing.T) {
		for _, resultFragment := range []string{
			`,"result":{"result":"Plan mode is off.","fullOutputUri":"file:///fixture/out.txt"}`,
			`,"result":null`,
			``,
		} {
			raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"exit_plan_mode"}},"generic":{"args":{"plan":"# Plan\n\n- Step A\n- Step B"}` + resultFragment + `}}]}`
			var trajectory daemon.Trajectory
			if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
				t.Fatal(err)
			}
			out := buf.String()
			for _, want := range []string{
				"### exit_plan_mode",
				"**Plan:**\n```markdown\n# Plan\n\n- Step A\n- Step B\n```",
				"<details><summary>Stored tool details</summary>",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("missing %q (result=%q) in:\n%s", want, resultFragment, out)
				}
			}
			if strings.Index(out, "**Plan:**") > strings.Index(out, "<details>") {
				t.Fatalf("plan rendered after details:\n%s", out)
			}
		}
	})
}

func TestRenderRunWorkflowGenericStep(t *testing.T) {
	t.Run("inline workflow with JSON string Args and result", func(t *testing.T) {
		raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-wf","name":"run_workflow"}},"generic":{"args":{"Args":"{\"tag\":\"v1\",\"count\":9007199254740993}","Cwd":"/fixture/work","Description":"Run inline check","Script":"phase(\"Check\")\nprint(\"OK\")","WaitMsBeforeAsync":"10000"},"result":{"result":"Output:\nOK","fullOutputUri":"file:///fixture/wf.txt"}}}]}`
		var trajectory daemon.Trajectory
		if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		for _, want := range []string{
			"### run_workflow",
			"- **Description:** `Run inline check`",
			"- **Cwd:** `/fixture/work`",
			"**Script:**\n```python\nphase(\"Check\")\nprint(\"OK\")\n```",
			"**Args:**\n```json\n{\n  \"count\": 9007199254740993,\n  \"tag\": \"v1\"\n}\n```",
			"**Result:**\n```\nOutput:\nOK\n```",
			"<details><summary>Stored tool details</summary>",
			`"WaitMsBeforeAsync": "10000"`,
			"file:///fixture/wf.txt",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
		if strings.Index(out, "**Script:**") > strings.Index(out, "**Result:**") || strings.Index(out, "**Result:**") > strings.Index(out, "<details>") {
			t.Fatalf("unexpected section order in:\n%s", out)
		}
	})

	t.Run("file workflow without result and object Args", func(t *testing.T) {
		raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR","metadata":{"toolCall":{"name":"run_workflow"}},"error":{"shortError":"workflow execution failed (line 2): boom"},"generic":{"args":{"Args":{"inlineData":"QUJDREVG","mode":"fast"},"Description":"Run file workflow","ScriptPath":"/fixture/brain/flow.py"}}}]}`
		var trajectory daemon.Trajectory
		if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		for _, want := range []string{
			"### run_workflow",
			"- **Description:** `Run file workflow`",
			"- **Script Path:** [flow.py](/fixture/brain/flow.py)",
			"**Args:**\n```json",
			"embedded, not shown (8 base64 characters)",
			"<details><summary>Stored tool details</summary>",
			"### Step Error",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
		if strings.Contains(out, "QUJDREVG") {
			t.Fatalf("inline bytes leaked into output:\n%s", out)
		}
		if strings.Contains(out, "**Result:**") {
			t.Fatalf("unexpected Result section when result is absent:\n%s", out)
		}
	})

	t.Run("preserves repeated spaces in Cwd and protects HTML in Description", func(t *testing.T) {
		raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"run_workflow"}},"generic":{"args":{"Description":"Check <details> in project","Cwd":"/work/Project  A","Script":"print('ok')"},"result":{"result":"OK"}}},{"type":"CORTEX_STEP_TYPE_SYSTEM_MESSAGE","systemMessage":{"message":"following step"}}]}`
		var trajectory daemon.Trajectory
		if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if !strings.Contains(out, "- **Cwd:** `/work/Project  A`") {
			t.Fatalf("repeated spaces in Cwd were not preserved:\n%s", out)
		}
		if !strings.Contains(out, "- **Description:** `Check <details> in project`") {
			t.Fatalf("angle brackets in Description were not inline-code-wrapped:\n%s", out)
		}
		assertFollowingStepOutsideFence(t, out)
	})

	t.Run("trailing-slash ScriptPath with HTML cannot enclose following step", func(t *testing.T) {
		raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR","metadata":{"toolCall":{"name":"run_workflow"}},"generic":{"args":{"Description":"Run workflow","ScriptPath":"/tmp/</li></ul><details>/"}}},{"type":"CORTEX_STEP_TYPE_SYSTEM_MESSAGE","systemMessage":{"message":"following step"}}]}`
		var trajectory daemon.Trajectory
		if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if !strings.Contains(out, "- **Script Path:** `/tmp/</li></ul><details>/`") {
			t.Fatalf("trailing-slash ScriptPath was not inline-code-wrapped:\n%s", out)
		}
		assertFollowingStepOutsideFence(t, out)
		beforeStep2, _, ok := strings.Cut(out, "## Step 2:")
		if !ok {
			t.Fatalf("missing Step 2 in:\n%s", out)
		}
		// Only the inline-code-wrapped <details> in ScriptPath and the balanced
		// Stored tool details block should appear before Step 2.
		if strings.Count(beforeStep2, "<details>") != 2 || strings.Count(beforeStep2, "</details>") != 1 {
			t.Fatalf("unexpected details balance before Step 2:\n%s", beforeStep2)
		}
	})
}

func TestRenderWorkflowDescriptionVerbatim(t *testing.T) {
	for _, tt := range []struct {
		name, description, want string
	}{
		{"link", "[docs](https://example.com/docs)", "- **Description:** `[docs](https://example.com/docs)`\n"},
		{"image", "![badge](https://example.com/badge.png)", "- **Description:** `![badge](https://example.com/badge.png)`\n"},
		{"formatting", "**bold** _italic_ ~~strike~~ &amp; `code`", "- **Description:** `` **bold** _italic_ ~~strike~~ &amp; `code` ``\n"},
		{"spaces", "  repeated  spaces  ", "- **Description:** `   repeated  spaces   `\n"},
		{"space only", "   ", "**Description:**\n```\n   \n```\n"},
		{"multiline", "first\n```\n\t**second**", "**Description:**\n````\nfirst\n```\n\t**second**\n````\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			description, err := json.Marshal(tt.description)
			if err != nil {
				t.Fatal(err)
			}
			raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","metadata":{"toolCall":{"name":"run_workflow"}},"generic":{"args":{"Description":` + string(description) + `}}},{"type":"CORTEX_STEP_TYPE_SYSTEM_MESSAGE","systemMessage":{"message":"following step"}}]}`
			var trajectory daemon.Trajectory
			if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
				t.Fatal(err)
			}
			out := buf.String()
			if !strings.Contains(out, tt.want) {
				t.Fatalf("missing literal description %q in:\n%s", tt.want, out)
			}
			assertFollowingStepOutsideFence(t, out)
		})
	}
}
