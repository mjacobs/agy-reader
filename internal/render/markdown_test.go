package render_test

import (
	"bytes"
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
		"- **Total Steps:** 10",
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
		"### Code Action",
		"**Description:** edit X",
		"### Ripgrep Search",
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
