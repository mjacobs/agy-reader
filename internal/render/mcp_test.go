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

func TestMCPDetailsSurviveRendering(t *testing.T) {
	for _, status := range []string{"DONE", "ERROR"} {
		raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_MCP_TOOL","status":"CORTEX_STEP_STATUS_` + status + `","mcpTool":{"serverName":"fixture-server","toolCall":{"name":"fixture-tool","argumentsJson":"{}"},"resultString":"fixture output"}}]}`
		var trajectory daemon.Trajectory
		if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		_, err := render.Markdown(&buf, &trajectory, time.Time{})
		data := buf.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range []string{"fixture-server", "fixture-tool", "argumentsJson", "fixture output"} {
			if !strings.Contains(string(data), text) {
				t.Fatalf("missing %s", text)
			}
		}
		if strings.Contains(string(data), "No step details available") {
			t.Fatal("lost MCP details")
		}
	}
}

func TestMCPToolRendersStructuredSectionsAndRedactsInlineBytes(t *testing.T) {
	raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_MCP_TOOL","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-1","name":"call_mcp_tool"}},"mcpTool":{"serverName":"echo","toolCall":{"id":"call-1","name":"ping","argumentsJson":"{\"count\":9007199254740993,\"text\":\"asked158\"}"},"resultString":"PONG:asked158","attachment":{"inlineData":"QUJDREVG"},"extraField":"retained"}}]}`
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
		"### MCP Tool Call",
		"- **Server:** `echo`",
		"- **Tool:** `ping`",
		"**Arguments:**",
		"```json\n{\n  \"count\": 9007199254740993,\n  \"text\": \"asked158\"\n}\n```",
		"**Output:**",
		"PONG:asked158",
		"<details><summary>Stored MCP tool details</summary>",
		"\"extraField\": \"retained\"",
		"embedded, not shown (8 base64 characters)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "QUJDREVG") {
		t.Fatalf("inline bytes leaked into output:\n%s", out)
	}
}

func TestMCPToolRedactsInlineBytesInsideArgumentsJSON(t *testing.T) {
	raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_MCP_TOOL","mcpTool":{"serverName":"media","toolCall":{"name":"inspect","argumentsJson":"{\"inlineData\":\"QUJDREVG\"}"},"resultString":"ok"}}]}`
	var trajectory daemon.Trajectory
	if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "QUJDREVG") {
		t.Fatalf("inline bytes from argumentsJson leaked into output:\n%s", out)
	}
	if count := strings.Count(out, "embedded, not shown (8 base64 characters)"); count != 2 {
		t.Fatalf("expected 2 redacted summaries (Arguments and Stored MCP tool details), got %d:\n%s", count, out)
	}
}

func TestMCPToolHandlesMissingOutputAndEdgePayloads(t *testing.T) {
	t.Run("missing output", func(t *testing.T) {
		raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_MCP_TOOL","status":"CORTEX_STEP_STATUS_ERROR","mcpTool":{"serverName":"fixture-server","toolCall":{"name":"summarize","argumentsJson":"{}"}}}]}`
		var trajectory daemon.Trajectory
		if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		for _, want := range []string{"### MCP Tool Call", "- **Server:** `fixture-server`", "- **Tool:** `summarize`", "**Arguments:**", "<details><summary>Stored MCP tool details</summary>"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q", want)
			}
		}
		if strings.Contains(out, "**Output:**") {
			t.Fatalf("unexpected empty Output section:\n%s", out)
		}
	})

	t.Run("non-json arguments", func(t *testing.T) {
		raw := `{"steps":[{"type":"CORTEX_STEP_TYPE_MCP_TOOL","status":"CORTEX_STEP_STATUS_DONE","mcpTool":{"serverName":"s","toolCall":{"name":"t","argumentsJson":"{partial"}}}]}`
		var trajectory daemon.Trajectory
		if err := json.Unmarshal([]byte(raw), &trajectory); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if !strings.Contains(out, "```\n{partial\n```") {
			t.Fatalf("expected raw fence for non-JSON arguments, got:\n%s", out)
		}
	})

	t.Run("empty payload", func(t *testing.T) {
		trajectory := daemon.Trajectory{Steps: []daemon.Step{{Type: "CORTEX_STEP_TYPE_MCP_TOOL", Status: "CORTEX_STEP_STATUS_DONE"}}}
		var buf bytes.Buffer
		if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "No additional details stored for this step type") {
			t.Fatalf("expected no-details marker, got:\n%s", buf.String())
		}
	})

	t.Run("non-object json payload", func(t *testing.T) {
		trajectory := daemon.Trajectory{Steps: []daemon.Step{{Type: "CORTEX_STEP_TYPE_MCP_TOOL", Status: "CORTEX_STEP_STATUS_DONE", McpTool: json.RawMessage(`["unexpected"]`)}}}
		var buf bytes.Buffer
		if _, err := render.Markdown(&buf, &trajectory, time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), `"unexpected"`) {
			t.Fatalf("expected fallback raw JSON payload, got:\n%s", buf.String())
		}
	})
}
