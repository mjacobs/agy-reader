package daemon_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

func TestCombinedOutputString(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain string", `"hello"`, "hello"},
		{"stdout key", `{"stdout":"out"}`, "out"},
		{"stderr key", `{"stderr":"err"}`, "err"},
		{"text key", `{"text":"t"}`, "t"},
		{"full key", `{"full":"complete log"}`, "complete log"},
		{"stdout+stderr joined", `{"stdout":"a","stderr":"b"}`, "a\nb"},
		{"unknown shape falls back to raw", `{"weird":"x"}`, `{"weird":"x"}`},
		{"empty object means no output", `{}`, ""},
		{"empty known key falls back to raw", `{"stdout":""}`, `{"stdout":""}`},
		{"empty bytes returns empty", ``, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rc := &daemon.RunCommand{CombinedOutput: []byte(tc.in)}
			if got := rc.CombinedOutputString(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCodeActionFormattedDiff(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var ca *daemon.CodeAction
		if got := ca.FormattedDiff(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("empty action result", func(t *testing.T) {
		ca := &daemon.CodeAction{}
		if got := ca.FormattedDiff(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		ca := &daemon.CodeAction{ActionResult: []byte(`not-json`)}
		if got := ca.FormattedDiff(); got != "" {
			t.Errorf("expected empty on parse failure, got %q", got)
		}
	})

	t.Run("missing edit", func(t *testing.T) {
		ca := &daemon.CodeAction{ActionResult: []byte(`{}`)}
		if got := ca.FormattedDiff(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("empty lines", func(t *testing.T) {
		ca := &daemon.CodeAction{ActionResult: []byte(`{"edit":{"diff":{"unifiedDiff":{"lines":[]}}}}`)}
		if got := ca.FormattedDiff(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("all line types", func(t *testing.T) {
		ca := &daemon.CodeAction{ActionResult: []byte(`{
			"edit":{"diff":{"unifiedDiff":{"lines":[
				{"text":"ctx","type":"UNIFIED_DIFF_LINE_TYPE_UNCHANGED"},
				{"text":"new","type":"UNIFIED_DIFF_LINE_TYPE_INSERT"},
				{"text":"old","type":"UNIFIED_DIFF_LINE_TYPE_DELETE"},
				{"text":"weird","type":"SOMETHING_ELSE"}
			]}}}
		}`)}
		got := ca.FormattedDiff()
		want := "```diff\n ctx\n+new\n-old\n weird\n```"
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})
}

func TestCodeActionGetSpec(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var ca *daemon.CodeAction
		if _, err := ca.GetSpec(); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("empty spec bytes", func(t *testing.T) {
		ca := &daemon.CodeAction{}
		if _, err := ca.GetSpec(); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		ca := &daemon.CodeAction{ActionSpec: []byte(`{`)}
		if _, err := ca.GetSpec(); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("missing command", func(t *testing.T) {
		ca := &daemon.CodeAction{ActionSpec: []byte(`{}`)}
		_, err := ca.GetSpec()
		if err == nil || !strings.Contains(err.Error(), "no command") {
			t.Errorf("expected 'no command' error, got %v", err)
		}
	})

	t.Run("valid", func(t *testing.T) {
		ca := &daemon.CodeAction{ActionSpec: []byte(`{"command":{"instruction":"do it"}}`)}
		cmd, err := ca.GetSpec()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cmd.Instruction != "do it" {
			t.Errorf("got instruction %q", cmd.Instruction)
		}
	})
}

func TestCodeActionCommandFilePath(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var cmd *daemon.CodeActionCommand
		if got := cmd.FilePath(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("nil file", func(t *testing.T) {
		cmd := &daemon.CodeActionCommand{}
		if got := cmd.FilePath(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("workspace mapping wins", func(t *testing.T) {
		cmd := &daemon.CodeActionCommand{File: &daemon.CodeActionFile{
			AbsoluteURI:                  "file:///abs/path.ts",
			WorkspaceURIsToRelativePaths: map[string]string{"file:///": "rel/path.ts"},
		}}
		if got := cmd.FilePath(); got != "rel/path.ts" {
			t.Errorf("got %q, want rel/path.ts", got)
		}
	})

	t.Run("falls back to absolute uri", func(t *testing.T) {
		cmd := &daemon.CodeActionCommand{File: &daemon.CodeActionFile{
			AbsoluteURI: "file:///abs/path.ts",
		}}
		if got := cmd.FilePath(); got != "file:///abs/path.ts" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("empty workspace value falls back", func(t *testing.T) {
		cmd := &daemon.CodeActionCommand{File: &daemon.CodeActionFile{
			AbsoluteURI:                  "file:///abs/path.ts",
			WorkspaceURIsToRelativePaths: map[string]string{"file:///": ""},
		}}
		if got := cmd.FilePath(); got != "file:///abs/path.ts" {
			t.Errorf("got %q", got)
		}
	})
}

func TestStepMetadataModelAPIContentID(t *testing.T) {
	t.Run("unmarshal and round-trip", func(t *testing.T) {
		rawJSON := `{
			"createdAt": "2026-10-14T05:12:36.892815324Z",
			"source": "CORTEX_STEP_SOURCE_USER_EXPLICIT",
			"executionId": "bf46d57b-5bb6-4894-98b7-bf6b407550b6",
			"modelApiContentId": "e8fd8c186714472db7e2ce33886b4e53"
		}`

		var meta daemon.StepMetadata
		if err := json.Unmarshal([]byte(rawJSON), &meta); err != nil {
			t.Fatalf("failed to unmarshal StepMetadata: %v", err)
		}

		if meta.ModelAPIContentID != "e8fd8c186714472db7e2ce33886b4e53" {
			t.Errorf("got ModelAPIContentID %q, want %q", meta.ModelAPIContentID, "e8fd8c186714472db7e2ce33886b4e53")
		}
		if meta.CreatedAt != "2026-10-14T05:12:36.892815324Z" {
			t.Errorf("got CreatedAt %q", meta.CreatedAt)
		}
		if meta.Source != "CORTEX_STEP_SOURCE_USER_EXPLICIT" {
			t.Errorf("got Source %q", meta.Source)
		}
		if meta.ExecutionID != "bf46d57b-5bb6-4894-98b7-bf6b407550b6" {
			t.Errorf("got ExecutionID %q", meta.ExecutionID)
		}

		out, err := json.Marshal(meta)
		if err != nil {
			t.Fatalf("failed to marshal StepMetadata: %v", err)
		}
		if !strings.Contains(string(out), `"modelApiContentId":"e8fd8c186714472db7e2ce33886b4e53"`) {
			t.Errorf("marshaled json missing modelApiContentId: %s", string(out))
		}

		var roundTrip daemon.StepMetadata
		if err := json.Unmarshal(out, &roundTrip); err != nil {
			t.Fatalf("failed to unmarshal round-tripped json: %v", err)
		}
		if roundTrip.ModelAPIContentID != meta.ModelAPIContentID {
			t.Errorf("roundTrip ModelAPIContentID got %q, want %q", roundTrip.ModelAPIContentID, meta.ModelAPIContentID)
		}
	})

	t.Run("omitted when empty", func(t *testing.T) {
		meta := daemon.StepMetadata{
			CreatedAt:   "2026-10-14T05:12:36.892815324Z",
			Source:      "CORTEX_STEP_SOURCE_USER_EXPLICIT",
			ExecutionID: "bf46d57b-5bb6-4894-98b7-bf6b407550b6",
		}
		out, err := json.Marshal(meta)
		if err != nil {
			t.Fatalf("failed to marshal StepMetadata: %v", err)
		}
		if strings.Contains(string(out), "modelApiContentId") {
			t.Errorf("expected modelApiContentId to be omitted, got %s", string(out))
		}
	})

	t.Run("step unmarshal and round-trip", func(t *testing.T) {
		stepJSON := `{
			"type": "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
			"status": "CORTEX_STEP_STATUS_COMPLETED",
			"metadata": {
				"createdAt": "2026-10-14T05:12:36.892815324Z",
				"modelApiContentId": "planner-content-id-12345"
			}
		}`

		var step daemon.Step
		if err := json.Unmarshal([]byte(stepJSON), &step); err != nil {
			t.Fatalf("failed to unmarshal Step: %v", err)
		}
		if step.Metadata.ModelAPIContentID != "planner-content-id-12345" {
			t.Errorf("got step.Metadata.ModelAPIContentID %q, want planner-content-id-12345", step.Metadata.ModelAPIContentID)
		}

		out, err := json.Marshal(step)
		if err != nil {
			t.Fatalf("failed to marshal Step: %v", err)
		}
		if !strings.Contains(string(out), `"modelApiContentId":"planner-content-id-12345"`) {
			t.Errorf("marshaled step json missing modelApiContentId: %s", string(out))
		}
	})
}

