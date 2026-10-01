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

func intPtr(v int) *int { return &v }

func TestMarkdownAllStepTypes(t *testing.T) {
	traj := &daemon.Trajectory{
		CascadeID: "fixture-1",
		Steps: []daemon.Step{
			{
				Type:      "CORTEX_STEP_TYPE_USER_INPUT",
				Status:    "COMPLETED",
				Metadata:  daemon.StepMetadata{CreatedAt: "2026-05-25T12:00:00Z"},
				UserInput: &daemon.UserInput{UserResponse: "say hi"},
			},
			{
				Type:   "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
				Status: "COMPLETED",
				PlannerResponse: &daemon.PlannerResponse{
					Thinking: "thought-A",
					Response: "answer-A",
					ToolCalls: []daemon.ToolCall{
						{Name: "Bash", ArgumentsJSON: `{"cmd":"ls"}`},
					},
				},
			},
			{
				Type:   "CORTEX_STEP_TYPE_RUN_COMMAND",
				Status: "COMPLETED",
				RunCommand: &daemon.RunCommand{
					CommandLine:    "ls /tmp",
					Cwd:            "/home/u",
					ExitCode:       intPtr(0),
					CombinedOutput: []byte(`"a\nb"`),
				},
			},
			{
				Type:     "CORTEX_STEP_TYPE_VIEW_FILE",
				ViewFile: &daemon.ViewFile{AbsolutePathURI: "file:///x", StartLine: 1, EndLine: 3, Content: "1\n2\n3"},
			},
			{
				Type:       "CORTEX_STEP_TYPE_CODE_ACTION",
				CodeAction: &daemon.CodeAction{Description: "edit X", ActionResult: []byte(`"ok"`)},
			},
			{
				Type: "CORTEX_STEP_TYPE_CODE_ACTION",
				CodeAction: &daemon.CodeAction{
					Description: "detailed edit",
					ActionSpec: []byte(`{
						"command": {
							"instruction": "add alarm permission",
							"file": {
								"absoluteUri": "file:///wxt.config.ts",
								"workspaceUrisToRelativePaths": {
									"file:///": "wxt.config.ts"
								}
							}
						}
					}`),
					ActionResult: []byte(`{
						"edit": {
							"diff": {
								"unifiedDiff": {
									"lines": [
										{"text": "import config", "type": "UNIFIED_DIFF_LINE_TYPE_UNCHANGED"},
										{"text": "permissions: [\"alarms\"]", "type": "UNIFIED_DIFF_LINE_TYPE_INSERT"},
										{"text": "permissions: []", "type": "UNIFIED_DIFF_LINE_TYPE_DELETE"}
									]
								}
							}
						}
					}`),
				},
			},
			{
				Type:       "CORTEX_STEP_TYPE_GREP_SEARCH",
				GrepSearch: &daemon.GrepSearch{Query: "TODO", SearchPathURI: "file:///repo"},
			},
			{
				Type: "CORTEX_STEP_TYPE_ERROR_MESSAGE",
				ErrorMessage: &daemon.ErrorMessage{
					Error: daemon.ErrorMessageError{
						UserErrorMessage:  "The model produced an invalid tool call.",
						ModelErrorMessage: "Parser failed at token 42: unexpected '}' in arguments.",
					},
					ShouldShowModel: true,
				},
			},
			{
				Type:          "CORTEX_STEP_TYPE_SYSTEM_MESSAGE",
				SystemMessage: &daemon.SystemMessage{Message: "noted"},
			},
			{
				Type:       "CORTEX_STEP_TYPE_CHECKPOINT",
				Checkpoint: &daemon.Checkpoint{UserRequests: []string{"r1", "r2"}, SessionSummary: "summary"},
			},
			{
				Type:    "CORTEX_STEP_TYPE_FUTURE_THING",
				Generic: []byte(`{"foo":"bar"}`),
			},
		},
	}

	var buf bytes.Buffer
	if _, err := render.Markdown(&buf, traj, time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	out := buf.String()

	mustContain := []string{
		"# Conversation Decrypted Transcript",
		"`fixture-1`",
		"- **Total Steps:** 11",
		"## Step 1: USER_INPUT",
		"say hi",
		"### Agent Internal Thought",
		"thought-A",
		"### Agent Chat Response",
		"answer-A",
		"### Tool Calls Proposed",
		"`Bash`",
		"### Command Execution",
		"**Command:** `ls /tmp`",
		"**Exit Code:** `0`",
		"### File Viewed",
		"(Lines 1-3)",
		"**File:** [x](file:///x)",
		"### Code Action",
		"**Description:** edit X",
		"**Description:** detailed edit",
		"**File:** [wxt.config.ts](file:///wxt.config.ts)",
		"**Instruction:** add alarm permission",
		"+permissions: [\"alarms\"]",
		"-permissions: []",
		"### Ripgrep Search",
		"**Path:** [repo](file:///repo)",
		"### Tool Execution Error",
		"> [!WARNING]",
		"The model produced an invalid tool call.",
		"<details><summary>Model error detail</summary>",
		"Parser failed at token 42",
		"### System Message",
		"### Checkpoint / Compaction State",
		"r1, r2",
		"FUTURE_THING",
		"\"foo\"",
	}
	for _, want := range mustContain {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestMarkdownEmptyTrajectory(t *testing.T) {
	var buf bytes.Buffer
	if _, err := render.Markdown(&buf, &daemon.Trajectory{}, time.Now()); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	if !strings.Contains(buf.String(), "**Total Steps:** 0") {
		t.Errorf("expected zero-step transcript: %s", buf.String())
	}
}

func TestMarkdownNilTrajectory(t *testing.T) {
	var buf bytes.Buffer
	if _, err := render.Markdown(&buf, nil, time.Now()); err == nil {
		t.Fatal("expected error")
	}
}

func TestMarkdownErrorMessageEmpty(t *testing.T) {
	var buf bytes.Buffer
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type:         "CORTEX_STEP_TYPE_ERROR_MESSAGE",
		ErrorMessage: &daemon.ErrorMessage{},
	}}}
	if _, err := render.Markdown(&buf, traj, time.Now()); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "(error details unavailable)") {
		t.Errorf("expected placeholder for empty error, got: %s", out)
	}
}

func TestMarkdownErrorMessageUserOnly(t *testing.T) {
	var buf bytes.Buffer
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type: "CORTEX_STEP_TYPE_ERROR_MESSAGE",
		ErrorMessage: &daemon.ErrorMessage{
			Error: daemon.ErrorMessageError{UserErrorMessage: "Just a friendly message"},
		},
	}}}
	if _, err := render.Markdown(&buf, traj, time.Now()); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Just a friendly message") {
		t.Errorf("expected user error rendered, got: %s", out)
	}
	if strings.Contains(out, "<details>") {
		t.Errorf("did not expect <details> block when no model error: %s", out)
	}
}

// Regression: a CodeAction whose spec has a command but no `file` block
// previously panicked because the renderer dereferenced spec.File unconditionally.
func TestMarkdownCodeActionSpecWithoutFile(t *testing.T) {
	var buf bytes.Buffer
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type: "CORTEX_STEP_TYPE_CODE_ACTION",
		CodeAction: &daemon.CodeAction{
			ActionSpec: []byte(`{"command":{"instruction":"do a thing"}}`),
		},
	}}}
	if _, err := render.Markdown(&buf, traj, time.Now()); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "**Instruction:** do a thing") {
		t.Errorf("expected instruction rendered: %s", out)
	}
	if strings.Contains(out, "**File:**") {
		t.Errorf("did not expect a File line when spec.File is nil: %s", out)
	}
}

// Code action with a workspace path but no absoluteUri should render the
// relative path as plain `code` rather than a clickable link.
func TestMarkdownCodeActionFileWithoutAbsoluteURI(t *testing.T) {
	var buf bytes.Buffer
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type: "CORTEX_STEP_TYPE_CODE_ACTION",
		CodeAction: &daemon.CodeAction{
			ActionSpec: []byte(`{"command":{"file":{"workspaceUrisToRelativePaths":{"file:///":"a/b.ts"}}}}`),
		},
	}}}
	if _, err := render.Markdown(&buf, traj, time.Now()); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "**File:** `a/b.ts`") {
		t.Errorf("expected plain backtick file path: %s", out)
	}
}

func TestMarkdownUnknownExitCode(t *testing.T) {
	var buf bytes.Buffer
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type:       "CORTEX_STEP_TYPE_RUN_COMMAND",
		RunCommand: &daemon.RunCommand{CommandLine: "x"},
	}}}
	if _, err := render.Markdown(&buf, traj, time.Now()); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	if !strings.Contains(buf.String(), "**Exit Code:** `unknown`") {
		t.Errorf("expected unknown exit code: %s", buf.String())
	}
}

func TestMarkdownStepErrorWithoutPayload(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type:   "CORTEX_STEP_TYPE_VIEW_FILE",
		Status: "CORTEX_STEP_STATUS_ERROR",
		Error:  &daemon.StepError{ShortError: "no such file"},
	}}}
	if _, err := render.Markdown(&buf, traj, time.Now()); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"### Step Error", "> no such file"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "Full error") {
		t.Errorf("unexpected full-error block\n%s", out)
	}
}

func TestMarkdownFinishStructuredOutput(t *testing.T) {
	var buf bytes.Buffer
	traj := &daemon.Trajectory{Steps: []daemon.Step{
		{
			Type:            "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
			PlannerResponse: &daemon.PlannerResponse{Response: "Working on it."},
		},
		{
			Type:   "CORTEX_STEP_TYPE_FINISH",
			Status: "COMPLETED",
			Finish: &daemon.Finish{
				Output:       &daemon.FinishOutput{ArgumentsJSON: `{"verdict":"ship-it","score":42}`},
				OutputString: `{"verdict":"ship-it","score":42}`,
			},
		},
	}}
	if _, err := render.Markdown(&buf, traj, time.Now()); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"### Final Structured Output", `"verdict": "ship-it"`, `"score": 42`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "No additional details stored") {
		t.Errorf("FINISH payload still dropped:\n%s", out)
	}
	if strings.Count(out, "ship-it") != 1 {
		t.Errorf("identical outputString should not be repeated:\n%s", out)
	}
}

func TestMarkdownFinishOutputStringOnly(t *testing.T) {
	var buf bytes.Buffer
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type:   "CORTEX_STEP_TYPE_FINISH",
		Finish: &daemon.Finish{OutputString: "plain final answer"},
	}}}
	if _, err := render.Markdown(&buf, traj, time.Now()); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	if !strings.Contains(buf.String(), "plain final answer") {
		t.Errorf("outputString not rendered:\n%s", buf.String())
	}
}

func TestMarkdownFinishFromRPCJSON(t *testing.T) {
	var traj daemon.Trajectory
	raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_FINISH","status":"CORTEX_STEP_STATUS_DONE",` +
		`"finish":{"output":{"arguments_json":"{\"city\":\"Oslo\"}"},"outputString":"{\"city\":\"Oslo\"}"}}]}`
	if err := json.Unmarshal([]byte(raw), &traj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var buf bytes.Buffer
	if _, err := render.Markdown(&buf, &traj, time.Now()); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	if !strings.Contains(buf.String(), `"city": "Oslo"`) {
		t.Errorf("decoded FINISH payload not rendered:\n%s", buf.String())
	}
}

func TestMarkdownFinishKeepsLargeIntegersExact(t *testing.T) {
	var buf bytes.Buffer
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type: "CORTEX_STEP_TYPE_FINISH",
		Finish: &daemon.Finish{
			Output:       &daemon.FinishOutput{ArgumentsJSON: `{"id":9007199254740993}`},
			OutputString: `{"id":9007199254740993}`,
		},
	}}}
	if _, err := render.Markdown(&buf, traj, time.Now()); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	if !strings.Contains(buf.String(), "9007199254740993") {
		t.Errorf("large integer was altered:\n%s", buf.String())
	}
}

func TestMarkdownFinishKeepsOutputStringWithTrailingText(t *testing.T) {
	var buf bytes.Buffer
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type: "CORTEX_STEP_TYPE_FINISH",
		Finish: &daemon.Finish{
			Output:       &daemon.FinishOutput{ArgumentsJSON: `{"ok":true}`},
			OutputString: "{\"ok\":true}\nWARNING: result incomplete",
		},
	}}}
	if _, err := render.Markdown(&buf, traj, time.Now()); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	if !strings.Contains(buf.String(), "WARNING: result incomplete") {
		t.Errorf("output string with trailing text was dropped:\n%s", buf.String())
	}
}

func TestMarkdownFinishEmbeddedFenceDoesNotLeak(t *testing.T) {
	var buf bytes.Buffer
	traj := &daemon.Trajectory{Steps: []daemon.Step{
		{
			Type:   "CORTEX_STEP_TYPE_FINISH",
			Finish: &daemon.Finish{OutputString: "Example:\n```go\nfmt.Println(1)\n```\nDone."},
		},
		{
			Type:            "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
			PlannerResponse: &daemon.PlannerResponse{Response: "Next turn"},
		},
	}}
	if _, err := render.Markdown(&buf, traj, time.Now()); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "````\nExample:") || !strings.Contains(out, "Done.\n````") {
		t.Errorf("outer fence must be longer than embedded fence:\n%s", out)
	}
}
