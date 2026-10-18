package completeness

import (
	"errors"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// These mappings were qualified against stable source/full/default RPC captures
// from agy 1.4.3. Unlike embedded native Steps, these payloads retain the outer
// metadata and task details. Full wrapper equality is still checked first.
// Restrict this to observed fields and outcomes; a newly populated field needs
// its own evidence, even when the descriptor already defines it.
func projectTool143(step, generic, result, anyPayload protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported 1.4.3 tool projection")
	name := strings.TrimPrefix(tool143Get(anyPayload, "type_url").String(), "type.googleapis.com/")
	payload, err := decode(protoreflect.FullName(name), tool143Get(anyPayload, "value").Bytes())
	if err != nil {
		return nil, err
	}
	p := payload.ProtoReflect()
	// No native 1.4.3 permission-bearing capture has qualified these mappings.
	if step.Has(step.Descriptor().Fields().ByName("error")) ||
		step.Has(step.Descriptor().Fields().ByName("permissions")) ||
		step.Has(step.Descriptor().Fields().ByName("completed_interactions")) ||
		!onlyFields(result, fieldSet("result", "payload", "full_output_uri", "step_render_info")) {
		return nil, unsupported
	}
	out := proto.Clone(step.Interface())
	m := out.ProtoReflect()
	tool := genericToolName(step)
	status := statusName(step)
	if status != "CORTEX_STEP_STATUS_DONE" && (name != "exa.cortex.core.tools.Schedule" || status != "CORTEX_STEP_STATUS_CANCELED") {
		return nil, unsupported
	}
	switch name {
	case "exa.cortex.core.tools.RunCommand":
		if tool != "run_command" || !genericArgsAllowed(generic, "CommandLine", "WaitMsBeforeAsync", "BypassSandbox") ||
			!onlyFields(p, fieldSet("exit_code", "sandbox_details", "shell_name", "command_line", "cwd")) {
			return nil, unsupported
		}
		sandbox := tool143Get(p, "sandbox_details").Message()
		if !onlyFields(sandbox, fieldSet("ran_in_sandbox", "sandbox_errors")) {
			return nil, unsupported
		}
		v := tool143Native(m, "RUN_COMMAND", "run_command")
		tool143Copy(v, p, "command_line", "cwd", "shell_name")
		tool143Set(v, "proposed_command_line", tool143Get(p, "command_line"))
		// The native exit_code is optional; the payload's zero is a real exit 0.
		tool143Set(v, "exit_code", tool143Get(p, "exit_code"))
		tool143Set(v, "blocking", protoreflect.ValueOfBool(!step.Has(step.Descriptor().Fields().ByName("task_details"))))
		tool143Copy(v, sandbox, "ran_in_sandbox", "sandbox_errors")
		tool143Set(tool143Mutable(v, "combined_output"), "full", tool143Get(result, "result"))
	case "exa.cortex.core.tools.ReadFile":
		if tool != "read_file" || !genericArgsAllowed(generic, "Path", "StartLine", "EndLine") ||
			!onlyFields(p, fieldSet("file_num_lines", "file_num_bytes", "path", "start_line", "end_line", "line_range_bytes")) {
			return nil, unsupported
		}
		path := tool143Get(p, "path").String()
		start, end := tool143Get(p, "start_line").Int(), tool143Get(p, "end_line").Int()
		if !plainAbsolutePath(path) || start < 1 || end < start {
			return nil, unsupported
		}
		v := tool143Native(m, "VIEW_FILE", "view_file")
		tool143Set(v, "absolute_path_uri", protoreflect.ValueOfString("file://"+path))
		tool143Set(v, "start_line", protoreflect.ValueOfUint32(uint32(start-1)))
		tool143Set(v, "end_line", protoreflect.ValueOfUint32(uint32(end-1)))
		tool143Set(v, "num_lines", tool143Get(p, "file_num_lines"))
		tool143Set(v, "num_bytes", tool143Get(p, "file_num_bytes"))
		tool143Copy(v, p, "line_range_bytes")
		tool143Set(v, "content", tool143Get(result, "result"))
	case "exa.cortex.core.tools.FileEdit":
		if !onlyFields(p, fieldSet("kind", "path", "diff", "diff_stats")) {
			return nil, unsupported
		}
		kindField := p.Descriptor().Fields().ByName("kind")
		kind := kindField.Enum().Values().ByNumber(tool143Get(p, "kind").Enum())
		path := tool143Get(p, "path").String()
		if kind == nil || !plainAbsolutePath(path) {
			return nil, unsupported
		}
		create := kind.Name() == "CREATE"
		if (create && (tool != "write_file" || !genericArgsAllowed(generic, "Path", "Content", "Overwrite"))) ||
			(!create && (kind.Name() != "EDIT" || tool != "edit_file" || !genericArgsAllowed(generic, "Path", "Target", "Replacement"))) {
			return nil, unsupported
		}
		v := tool143Native(m, "CODE_ACTION", "code_action")
		spec := tool143Mutable(v, "action_spec")
		var target protoreflect.Message
		if create {
			target = tool143Mutable(tool143Mutable(spec, "create_file"), "path")
		} else {
			command := tool143Mutable(spec, "command")
			tool143Set(command, "is_edit", protoreflect.ValueOfBool(true))
			target = tool143Mutable(command, "file")
		}
		uri := protoreflect.ValueOfString("file://" + path)
		tool143Set(target, "absolute_uri", uri)
		edit := tool143Mutable(tool143Mutable(v, "action_result"), "edit")
		tool143Set(edit, "absolute_uri", uri)
		tool143Set(edit, "create_file", protoreflect.ValueOfBool(create))
		// Preserve message presence, including empty-file diffs and stats.
		tool143CopyPresent(edit, p, "diff")
		tool143CopyPresent(v, p, "diff_stats")
	case "exa.cortex.core.tools.InvokeSubagent":
		if tool != "invoke_subagent" || !genericArgsAllowed(generic, "Model", "Prompt", "Role", "TypeName") ||
			!onlyFields(p, fieldSet("subagents", "results")) {
			return nil, unsupported
		}
		v := tool143Native(m, "INVOKE_SUBAGENT", "invoke_subagent")
		tool143Copy(v, p, "subagents", "results")
	case "exa.cortex.core.tools.ManageTask":
		if tool != "manage_task" || !genericArgsAllowed(generic, "Action", "TaskId") ||
			!onlyFields(p, fieldSet("action", "task_ids")) {
			return nil, unsupported
		}
		action := tool143Get(p, "action").String()
		ids := tool143Get(p, "task_ids").List()
		if (action != "status" && action != "list" && action != "kill") || ids.Len() != 1 {
			return nil, unsupported
		}
		metadata := tool143Mutable(tool143Mutable(m, "generic"), "result").Mutable(result.Descriptor().Fields().ByName("metadata")).Map()
		metadata.Set(protoreflect.ValueOfString("action").MapKey(), protoreflect.ValueOfString(action))
		metadata.Set(protoreflect.ValueOfString("task_id").MapKey(), ids.Get(0))
		metadata.Set(protoreflect.ValueOfString("task_ids").MapKey(), ids.Get(0))
	case "exa.cortex.core.tools.Schedule":
		if tool != "schedule" || !genericArgsAllowed(generic, "DurationSeconds", "Prompt", "TimerCondition") ||
			!onlyFields(p, fieldSet("mode", "outcome")) || tool143Get(p, "mode").String() != "timer" ||
			tool143Get(p, "outcome").String() != "cancelled" || status != "CORTEX_STEP_STATUS_CANCELED" {
			return nil, unsupported
		}
		metadata := tool143Mutable(tool143Mutable(m, "generic"), "result").Mutable(result.Descriptor().Fields().ByName("metadata")).Map()
		for _, name := range []protoreflect.Name{"mode", "outcome"} {
			metadata.Set(protoreflect.ValueOfString(string(name)).MapKey(), tool143Get(p, name))
		}
	}
	return out, nil
}

func tool143Get(m protoreflect.Message, name protoreflect.Name) protoreflect.Value {
	return m.Get(m.Descriptor().Fields().ByName(name))
}

func tool143Set(m protoreflect.Message, name protoreflect.Name, value protoreflect.Value) {
	m.Set(m.Descriptor().Fields().ByName(name), value)
}

func tool143Mutable(m protoreflect.Message, name protoreflect.Name) protoreflect.Message {
	return m.Mutable(m.Descriptor().Fields().ByName(name)).Message()
}

func tool143Copy(dst, src protoreflect.Message, names ...protoreflect.Name) {
	for _, name := range names {
		field := dst.Descriptor().Fields().ByName(name)
		if field.IsList() {
			from, to := tool143Get(src, name).List(), dst.Mutable(field).List()
			for i := 0; i < from.Len(); i++ {
				to.Append(from.Get(i))
			}
		} else {
			tool143Set(dst, name, tool143Get(src, name))
		}
	}
}

func tool143CopyPresent(dst, src protoreflect.Message, names ...protoreflect.Name) {
	for _, name := range names {
		if src.Has(src.Descriptor().Fields().ByName(name)) {
			tool143Set(dst, name, tool143Get(src, name))
		}
	}
}

func tool143Native(m protoreflect.Message, kind protoreflect.Name, field protoreflect.Name) protoreflect.Message {
	t := m.Descriptor().Fields().ByName("type")
	m.Set(t, protoreflect.ValueOfEnum(t.Enum().Values().ByName("CORTEX_STEP_TYPE_"+kind).Number()))
	m.Clear(m.Descriptor().Fields().ByName("generic"))
	return tool143Mutable(m, field)
}
