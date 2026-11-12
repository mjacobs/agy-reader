package render

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

// Keep execution records visible even when the primary tool payload has a
// specialized renderer. Raw JSON preserves fields the renderer does not model.
func renderExecutionMetadata(buf *bytes.Buffer, s daemon.Step) {
	for _, item := range []struct {
		title string
		raw   json.RawMessage
	}{
		{"Pre-tool hook results", s.Metadata.PreToolHookResults},
		{"Permissions", s.Permissions},
		{"Completed interactions", s.CompletedInteractions},
		{"Pending interaction", s.RequestedInteraction},
		{"Background task", s.TaskDetails},
	} {
		if len(item.raw) == 0 {
			continue
		}
		fmt.Fprintf(buf, "### %s\n\n", item.title)
		renderRawPayload(buf, item.raw)
	}
}

// Generic tools (notably manage_task) store their display under result rather
// than a native step payload. Show the saved output and content blocks, then
// retain every remaining wrapper field in a collapsible details section.
func renderGeneric(buf *bytes.Buffer, s daemon.Step) {
	var generic map[string]json.RawMessage
	if json.Unmarshal(s.Generic, &generic) != nil || generic == nil {
		renderRawPayload(buf, s.Generic)
		return
	}
	var result map[string]json.RawMessage
	if json.Unmarshal(generic["result"], &result) != nil || result == nil {
		renderRawPayload(buf, s.Generic)
		return
	}
	name := "Tool result"
	if s.Metadata.ToolCall != nil && s.Metadata.ToolCall.Name != "" {
		name = oneLine(s.Metadata.ToolCall.Name)
	}
	fmt.Fprintf(buf, "### %s\n\n", name)
	var output string
	if json.Unmarshal(result["result"], &output) == nil && output != "" {
		writeFence(buf, "", output)
		delete(result, "result")
	}
	var info map[string]json.RawMessage
	if json.Unmarshal(result["stepRenderInfo"], &info) == nil {
		var blocks []json.RawMessage
		if json.Unmarshal(info["contentBlocks"], &blocks) == nil {
			for _, raw := range blocks {
				var block map[string]json.RawMessage
				var markdown string
				if json.Unmarshal(raw, &block) == nil && json.Unmarshal(block["markdown"], &markdown) == nil {
					// A saved display may have unfinished fences or HTML. Show it
					// literally; the result above remains the readable task output.
					buf.WriteString("**Saved tool display (Markdown):**\n")
					writeFence(buf, "markdown", markdown)
					delete(block, "markdown")
					if len(block) > 0 {
						rest, _ := json.Marshal(block)
						renderRawPayload(buf, rest)
					}
				} else {
					renderRawPayload(buf, raw)
				}
			}
			delete(info, "contentBlocks")
			if len(info) == 0 {
				delete(result, "stepRenderInfo")
			} else {
				result["stepRenderInfo"], _ = json.Marshal(info)
			}
		}
	}
	generic["result"], _ = json.Marshal(result)
	raw, _ := json.Marshal(generic)
	buf.WriteString("<details><summary>Stored tool details</summary>\n\n")
	renderRawPayload(buf, raw)
	buf.WriteString("</details>\n\n")
}
