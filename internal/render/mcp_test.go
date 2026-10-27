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
