package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

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

// renderMcpTool renders a CORTEX_STEP_TYPE_MCP_TOOL step with its server name,
// tool name, formatted arguments, and output, while preserving the full stored
// payload (with inline bytes redacted) in a collapsible details block.
func renderMcpTool(buf *bytes.Buffer, s daemon.Step) {
	if len(s.McpTool) == 0 {
		buf.WriteString(noDetails)
		return
	}
	var rawObj map[string]json.RawMessage
	var mcp struct {
		ServerName string `json:"serverName"`
		ToolCall   *struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			ArgumentsJSON string `json:"argumentsJson"`
		} `json:"toolCall"`
		ResultString string `json:"resultString"`
	}
	if json.Unmarshal(s.McpTool, &rawObj) != nil || rawObj == nil || json.Unmarshal(s.McpTool, &mcp) != nil {
		renderRawPayload(buf, s.McpTool)
		return
	}

	buf.WriteString("### MCP Tool Call\n\n")
	wroteMeta := false
	if mcp.ServerName != "" {
		if isSingleLineCodeSafe(mcp.ServerName) {
			fmt.Fprintf(buf, "- **Server:** %s\n", verbatimInlineCode(mcp.ServerName))
		} else {
			fmt.Fprintf(buf, "- **Server:** %s\n", inlineCode(mcp.ServerName))
		}
		wroteMeta = true
	}
	if mcp.ToolCall != nil && mcp.ToolCall.Name != "" {
		if isSingleLineCodeSafe(mcp.ToolCall.Name) {
			fmt.Fprintf(buf, "- **Tool:** %s\n", verbatimInlineCode(mcp.ToolCall.Name))
		} else {
			fmt.Fprintf(buf, "- **Tool:** %s\n", inlineCode(mcp.ToolCall.Name))
		}
		wroteMeta = true
	}
	if wroteMeta {
		buf.WriteString("\n")
	}
	if mcp.ToolCall != nil && mcp.ToolCall.ArgumentsJSON != "" {
		buf.WriteString("**Arguments:**\n")
		writeJSONOrTextFence(buf, mcp.ToolCall.ArgumentsJSON)
	}
	if mcp.ResultString != "" {
		buf.WriteString("**Output:**\n")
		writeFence(buf, "", mcp.ResultString)
	}
	buf.WriteString("<details><summary>Stored MCP tool details</summary>\n\n")
	renderRawPayload(buf, redactMcpToolPayload(s.McpTool))
	buf.WriteString("</details>\n\n")
}

// redactMcpToolPayload redacts inline media bytes inside toolCall.argumentsJson
// (a JSON-encoded string) before renderRawPayload redacts the outer MCP object,
// so embedded media is not retained in Stored MCP tool details.
func redactMcpToolPayload(raw json.RawMessage) json.RawMessage {
	var outer map[string]json.RawMessage
	if json.Unmarshal(raw, &outer) != nil || outer == nil {
		return raw
	}
	rawCall, ok := outer["toolCall"]
	if !ok {
		return raw
	}
	var call map[string]json.RawMessage
	if json.Unmarshal(rawCall, &call) != nil || call == nil {
		return raw
	}
	rawArgsJSON, ok := call["argumentsJson"]
	if !ok {
		return raw
	}
	var argsStr string
	if json.Unmarshal(rawArgsJSON, &argsStr) != nil || argsStr == "" {
		return raw
	}
	val, err := decodeJSONValue([]byte(argsStr))
	if err != nil {
		return raw
	}
	before, err := json.Marshal(val)
	if err != nil {
		return raw
	}
	after, err := json.Marshal(redactInlineBytes(val))
	if err != nil || bytes.Equal(before, after) {
		return raw
	}
	encodedStr, err := json.Marshal(string(after))
	if err != nil {
		return raw
	}
	call["argumentsJson"] = encodedStr
	encodedCall, err := json.Marshal(call)
	if err != nil {
		return raw
	}
	outer["toolCall"] = encodedCall
	encodedOuter, err := json.Marshal(outer)
	if err != nil {
		return raw
	}
	return encodedOuter
}

// Generic tools (notably manage_task, plan mode, and run_workflow) store their
// arguments and display under generic rather than a native step payload. Show
// structured arguments for known tools and any saved output/content blocks,
// then retain every remaining wrapper field in a collapsible details section.
func renderGeneric(buf *bytes.Buffer, s daemon.Step) {
	var generic map[string]json.RawMessage
	if json.Unmarshal(s.Generic, &generic) != nil || generic == nil {
		renderRawPayload(buf, s.Generic)
		return
	}

	toolName := ""
	if s.Metadata.ToolCall != nil {
		toolName = s.Metadata.ToolCall.Name
	}

	var argsBuf bytes.Buffer
	var args map[string]json.RawMessage
	if json.Unmarshal(generic["args"], &args) == nil && args != nil {
		renderGenericArgs(&argsBuf, toolName, args)
	}

	var result map[string]json.RawMessage
	hasResultObj := json.Unmarshal(generic["result"], &result) == nil && result != nil

	if argsBuf.Len() == 0 && !hasResultObj {
		renderRawPayload(buf, s.Generic)
		return
	}

	name := "Tool result"
	if toolName != "" {
		name = oneLine(toolName)
	}
	fmt.Fprintf(buf, "### %s\n\n", name)

	if argsBuf.Len() > 0 {
		buf.Write(argsBuf.Bytes())
		if len(args) == 0 {
			delete(generic, "args")
		} else {
			generic["args"], _ = json.Marshal(args)
		}
	}

	if hasResultObj {
		var output string
		if json.Unmarshal(result["result"], &output) == nil && output != "" {
			if argsBuf.Len() > 0 {
				buf.WriteString("**Result:**\n")
			}
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
	}

	raw, _ := json.Marshal(generic)
	buf.WriteString("<details><summary>Stored tool details</summary>\n\n")
	renderRawPayload(buf, raw)
	buf.WriteString("</details>\n\n")
}

func renderGenericArgs(buf *bytes.Buffer, toolName string, args map[string]json.RawMessage) {
	switch toolName {
	case "enter_plan_mode":
		if reason, ok := stringArg(args, "reason"); ok {
			buf.WriteString("**Reason:**\n")
			writeFence(buf, "", reason)
			delete(args, "reason")
		}
	case "exit_plan_mode":
		if plan, ok := stringArg(args, "plan"); ok {
			buf.WriteString("**Plan:**\n")
			writeFence(buf, "markdown", plan)
			delete(args, "plan")
		}
	case "run_workflow":
		wroteMeta := false
		var deferredBlocks bytes.Buffer
		if desc, ok := stringArg(args, "Description"); ok {
			if isSingleLineCodeSafe(desc) {
				fmt.Fprintf(buf, "- **Description:** %s\n", verbatimInlineCode(desc))
				wroteMeta = true
			} else {
				deferredBlocks.WriteString("**Description:**\n")
				writeFence(&deferredBlocks, "", desc)
			}
			delete(args, "Description")
		}
		if cwd, ok := stringArg(args, "Cwd"); ok {
			if isSingleLineCodeSafe(cwd) {
				fmt.Fprintf(buf, "- **Cwd:** %s\n", verbatimInlineCode(cwd))
				wroteMeta = true
			} else {
				deferredBlocks.WriteString("**Cwd:**\n")
				writeFence(&deferredBlocks, "", cwd)
			}
			delete(args, "Cwd")
		}
		if scriptPath, ok := stringArg(args, "ScriptPath"); ok {
			switch {
			case !isSingleLineCodeSafe(scriptPath):
				deferredBlocks.WriteString("**Script Path:**\n")
				writeFence(&deferredBlocks, "", scriptPath)
			case isClickableFileLink(scriptPath):
				fmt.Fprintf(buf, "- **Script Path:** %s\n", clickableLink(scriptPath))
				wroteMeta = true
			default:
				fmt.Fprintf(buf, "- **Script Path:** %s\n", verbatimInlineCode(scriptPath))
				wroteMeta = true
			}
			delete(args, "ScriptPath")
		}
		if wroteMeta {
			buf.WriteString("\n")
		}
		if deferredBlocks.Len() > 0 {
			buf.Write(deferredBlocks.Bytes())
		}
		if script, ok := stringArg(args, "Script"); ok {
			buf.WriteString("**Script:**\n")
			writeFence(buf, "python", script)
			delete(args, "Script")
		}
		if rawArgs, ok := args["Args"]; ok && len(rawArgs) > 0 && !bytes.Equal(bytes.TrimSpace(rawArgs), []byte("null")) {
			var argsStr string
			if json.Unmarshal(rawArgs, &argsStr) == nil {
				if argsStr != "" {
					buf.WriteString("**Args:**\n")
					writeJSONOrTextFence(buf, argsStr)
					delete(args, "Args")
				}
			} else {
				buf.WriteString("**Args:**\n")
				renderRawPayload(buf, rawArgs)
				delete(args, "Args")
			}
		}
	}
}

func stringArg(args map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := args[key]
	if !ok {
		return "", false
	}
	var s string
	if json.Unmarshal(raw, &s) != nil || s == "" {
		return "", false
	}
	return s, true
}

// isClickableFileLink reports whether uri has a non-empty filename segment after
// its final slash so clickableLink will escape the label and encode the target
// rather than falling back to a raw link.
func isClickableFileLink(uri string) bool {
	if !strings.HasPrefix(uri, "/") && !strings.HasPrefix(uri, "file://") {
		return false
	}
	clean := strings.TrimPrefix(uri, "file://")
	idx := strings.LastIndex(clean, "/")
	return idx != -1 && idx < len(clean)-1
}

// isSingleLineCodeSafe reports whether s can be represented verbatim in a
// CommonMark inline code span (no control characters and not all spaces).
func isSingleLineCodeSafe(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) < 0 && strings.Trim(s, " ") != ""
}

// verbatimInlineCode wraps s in a backtick span without collapsing internal
// whitespace, using CommonMark padding when s starts or ends with a backtick
// or space so the decoded span stays identical to s.
func verbatimInlineCode(s string) string {
	ticks := daemon.BacktickDelimiter(s, 1)
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		return ticks + " " + s + " " + ticks
	}
	return ticks + s + ticks
}

// writeJSONOrTextFence renders s as redacted, pretty-printed JSON when s is a
// single valid JSON value, or as a plain fenced block otherwise.
func writeJSONOrTextFence(buf *bytes.Buffer, s string) {
	if _, err := decodeJSONValue([]byte(s)); err == nil {
		renderRawPayload(buf, []byte(s))
		return
	}
	writeFence(buf, "", s)
}
