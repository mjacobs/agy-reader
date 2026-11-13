package completeness

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// DefineSubagent stays generic. The daemon adds the lowercase definition and
// name metadata, both derived from the retained arguments. Config and all
// other payload fields remain unchanged and participate in both comparisons.
func projectDefineSubagent158(step, generic, result, payload protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported define_subagent projection")
	if genericToolName(step) != "define_subagent" || statusName(step) != "CORTEX_STEP_STATUS_DONE" ||
		!onlyFields(step, fieldSet("type", "status", "metadata", "generic")) ||
		!onlyFields(result, fieldSet("result", "payload", "full_output_uri", "step_render_info")) ||
		!genericArgsAllowed(generic, "Name", "Description", "SystemPrompt", "EnableWriteTools", "EnableMCPTools", "EnableSubagentTools") || !argumentsPresented(generic, step) {
		return nil, unsupported
	}
	p, err := decode("exa.cortex.core.tools.DefineSubagent", tool143Get(payload, "value").Bytes())
	if err != nil {
		return nil, err
	}
	m := p.ProtoReflect()
	if !onlyFields(m, fieldSet("name", "description", "agent_config")) || !m.Has(m.Descriptor().Fields().ByName("agent_config")) {
		return nil, unsupported
	}
	definition := map[string]any{}
	for arg, key := range map[string]string{"Name": "name", "Description": "description", "SystemPrompt": "system_prompt"} {
		v, ok := genericArg(generic, arg)
		if !ok || v == "" {
			return nil, unsupported
		}
		definition[key] = v
		if arg != "SystemPrompt" && v != tool143Get(m, protoreflect.Name(key)).String() {
			return nil, unsupported
		}
	}
	for arg, key := range map[string]string{"EnableWriteTools": "enable_write_tools", "EnableMCPTools": "enable_mcp_tools", "EnableSubagentTools": "enable_subagent_tools"} {
		v, ok := genericArg(generic, arg)
		if !ok || (v != "true" && v != "false") {
			return nil, unsupported
		}
		definition[key] = v == "true"
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		return nil, err
	}
	out := proto.Clone(step.Interface())
	g := tool143Get(out.ProtoReflect(), "generic").Message()
	tool143Get(g, "args").Map().Set(protoreflect.ValueOfString("definition").MapKey(), protoreflect.ValueOfString(string(encoded)))
	r := tool143Get(g, "result").Message()
	r.Mutable(r.Descriptor().Fields().ByName("metadata")).Map().Set(protoreflect.ValueOfString("name").MapKey(), tool143Get(m, "name"))
	return out, nil
}

// Modern MCP payloads are presented as native MCP_TOOL steps. Only the observed
// completed string result and effective-grant approval are qualified. Permission
// records stay on the same cloned step, so neither copy is silently discarded.
func projectMCP158(step, generic, result, payload protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported MCP tool projection")
	if genericToolName(step) != "call_mcp_tool" || statusName(step) != "CORTEX_STEP_STATUS_DONE" ||
		!onlyFields(step, fieldSet("type", "status", "metadata", "generic", "permissions", "completed_interactions")) ||
		!onlyFields(result, fieldSet("result", "payload", "full_output_uri")) ||
		!genericArgsAllowed(generic, "Arguments", "ServerName", "ToolName") || !argumentsPresented(generic, step) {
		return nil, unsupported
	}
	p, err := decode("exa.cortex.core.tools.McpTool", tool143Get(payload, "value").Bytes())
	if err != nil {
		return nil, err
	}
	m := p.ProtoReflect()
	if !onlyFields(m, fieldSet("server_name", "tool_call", "result_string")) || !m.Has(m.Descriptor().Fields().ByName("result_string")) {
		return nil, unsupported
	}
	call := tool143Get(m, "tool_call").Message()
	server, sok := genericArg(generic, "ServerName")
	name, nok := genericArg(generic, "ToolName")
	args, aok := genericArg(generic, "Arguments")
	sourceCall := tool143Get(tool143Get(step, "metadata").Message(), "tool_call").Message()
	if !sok || !nok || !aok || server == "" || name == "" || server != tool143Get(m, "server_name").String() || name != tool143Get(call, "name").String() ||
		!onlyFields(call, fieldSet("id", "name", "arguments_json")) || tool143Get(call, "id").String() != tool143Get(sourceCall, "id").String() ||
		!jsonArgumentEqual(json.RawMessage(args), tool143Get(call, "arguments_json").String()) {
		return nil, unsupported
	}
	f := step.Descriptor().Fields()
	hasP, hasI := step.Has(f.ByName("permissions")), step.Has(f.ByName("completed_interactions"))
	if hasP || hasI {
		kind := permissionRecordKind{tool: "call_mcp_tool", action: "mcp", target: server + "/" + name, trigger: "TYPE_EFFECTIVE_GRANT"}
		if !hasP || !hasI || !approvedGrant(tool143Get(step, "permissions").Message(), kind) || !completedRequest(tool143Get(step, "completed_interactions").List(), kind, true) {
			return nil, unsupported
		}
	}
	out := proto.Clone(step.Interface())
	native := tool143Native(out.ProtoReflect(), "MCP_TOOL", "mcp_tool")
	tool143Copy(native, m, "server_name", "tool_call", "result_string")
	return out, nil
}

func qualifyDeniedMCP158(step, generic protoreflect.Message) error {
	unsupported := errors.New("unsupported denied MCP call")
	server, sok := genericArg(generic, "ServerName")
	name, nok := genericArg(generic, "ToolName")
	target := server + "/" + name
	want := fmt.Sprintf("permission check failed for mcp %q: Permission denied for mcp(%s). Matches user-configured deny rule.", target, target)
	if statusName(step) != "CORTEX_STEP_STATUS_ERROR" || !sok || !nok || server == "" || name == "" ||
		!onlyFields(step, fieldSet("type", "status", "metadata", "generic", "error")) || !plainFailure(step) || tool143Get(tool143Get(step, "error").Message(), "short_error").String() != want ||
		!genericArgsAllowed(generic, "Arguments", "ServerName", "ToolName") || !argumentsPresented(generic, step) {
		return unsupported
	}
	return nil
}

// Fresh 1.5.8 traces include resultless missing files, an inverted line range,
// and a PreToolUse JSON decode failure. The daemon builds the file view from
// arguments and preserves the original error, including any hook stack trace.
func failedRead158Candidate(step protoreflect.Message) bool {
	text := tool143Get(tool143Get(step, "error").Message(), "short_error").String()
	return (text == missingFileError && tool143Get(tool143Get(step, "metadata").Message(), "pre_tool_hook_results").List().Len() == 0) || strings.HasPrefix(text, "invalid line range: StartLine ") || strings.HasPrefix(text, "failed to unmarshal result from hook ")
}
func projectFailedRead158(step, generic protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported failed read_file projection")
	if genericToolName(step) != "read_file" || statusName(step) != "CORTEX_STEP_STATUS_ERROR" ||
		!onlyFields(step, fieldSet("type", "status", "metadata", "generic", "error")) || !genericArgsAllowed(generic, "Path", "StartLine", "EndLine") || !argumentsPresented(generic, step) {
		return nil, unsupported
	}
	gf := generic.Descriptor().Fields()
	if generic.Has(gf.ByName("result")) && !onlyFields(tool143Get(generic, "result").Message(), fieldSet("step_render_info")) {
		return nil, unsupported
	}
	failure := tool143Get(step, "error").Message()
	if !onlyFields(failure, fieldSet("short_error", "full_error")) {
		return nil, unsupported
	}
	short, full := tool143Get(failure, "short_error").String(), tool143Get(failure, "full_error").String()
	path, ok := genericArg(generic, "Path")
	if !ok || !verbatimURIPath(path) || skillFileRead146(path) {
		return nil, unsupported
	}
	hooks := tool143Get(tool143Get(step, "metadata").Message(), "pre_tool_hook_results").List()
	if hooks.Len() != 0 {
		return nil, unsupported
	}
	start, end, valid := zeroBasedLineArgs(generic)
	if strings.HasPrefix(short, "invalid line range: StartLine ") {
		a, aok := genericArg(generic, "StartLine")
		b, bok := genericArg(generic, "EndLine")
		av, ae := strconv.ParseUint(a, 10, 32)
		bv, be := strconv.ParseUint(b, 10, 32)
		if !aok || !bok || ae != nil || be != nil || av <= bv || bv < 1 || strconv.FormatUint(av, 10) != a || strconv.FormatUint(bv, 10) != b || short != fmt.Sprintf("invalid line range: StartLine %s is greater than EndLine %s", a, b) || full != short {
			return nil, unsupported
		}
		start, end = av-1, bv-1
	} else if short == missingFileError {
		if !valid || full != short {
			return nil, unsupported
		}
		// Missing-file ranges retain the qualified readlines150 boundary.
		if _, lines := genericArg(generic, "StartLine"); lines && (start != 0 || end == 0) {
			return nil, unsupported
		}
	} else if strings.HasPrefix(short, "failed to unmarshal result from hook ") {
		if !valid || start != 0 || end != 0 || !strings.Contains(short, " via protojson: ") || !strings.Contains(short, `unknown field "systemMessage"`) || !strings.HasPrefix(full, short+"\n(1) attached stack trace") {
			return nil, unsupported
		}
	} else {
		return nil, unsupported
	}
	out := proto.Clone(step.Interface())
	view := tool143Native(out.ProtoReflect(), "VIEW_FILE", "view_file")
	tool143Set(view, "absolute_path_uri", protoreflect.ValueOfString("file://"+path))
	if start != 0 {
		tool143Set(view, "start_line", protoreflect.ValueOfUint32(uint32(start)))
	}
	if end != 0 {
		tool143Set(view, "end_line", protoreflect.ValueOfUint32(uint32(end)))
	}
	return out, nil
}
