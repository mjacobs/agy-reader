package completeness

import (
	"errors"
	"strconv"
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
	// IDE 2.25.0 preserves both permission fields on completed modern reads.
	// Captured 1.5.7 command/edit approvals also retain both records, qualified
	// separately by their granting resource and completed interaction.
	// Unlike the embedded-native-Step mapping, no duplicate record is dropped:
	// the outer fields remain on the cloned step, compared in full below.
	f := step.Descriptor().Fields()
	hasPermissions, hasInteractions := step.Has(f.ByName("permissions")), step.Has(f.ByName("completed_interactions"))
	if name == "exa.cortex.core.tools.ReadFile" && result.Has(result.Descriptor().Fields().ByName("media")) {
		return projectImageRead147(step, generic, result, p)
	}
	if step.Has(step.Descriptor().Fields().ByName("error")) ||
		((hasPermissions || hasInteractions) && (!hasPermissions || !hasInteractions || (name != "exa.cortex.core.tools.ReadFile" && !modernPermissions157(step, generic, p, name)))) ||
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
		if tool != "run_command" || !genericArgsAllowed(generic, "CommandLine", "WaitMsBeforeAsync", "BypassSandbox", "IsDaemon") ||
			!onlyFields(p, fieldSet("exit_code", "sandbox_details", "shell_name", "command_line", "cwd")) ||
			!qualifiedCommandDaemonFlag(generic, step) {
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
		if tool != "read_file" || !genericArgsAllowed(generic, "Path", "StartLine", "EndLine", "ByteOffset") ||
			!onlyFields(p, fieldSet("file_num_lines", "file_num_bytes", "path", "start_line", "end_line", "line_range_bytes")) {
			return nil, unsupported
		}
		// agy retains a byte offset only in the tool call. The payload still
		// supplies the displayed line range, byte counts and content.
		if offset, present := genericArg(generic, "ByteOffset"); present {
			value, err := strconv.ParseUint(offset, 10, 63)
			if err != nil || strconv.FormatUint(value, 10) != offset || !argumentsPresented(generic, step) {
				return nil, unsupported
			}
		}
		path := tool143Get(p, "path").String()
		start, end := tool143Get(p, "start_line").Int(), tool143Get(p, "end_line").Int()
		// Empty reads in the 1.5.4 capture have zero counts and no line range.
		empty := start == 0 && end == 0 && tool143Get(p, "file_num_lines").Uint() == 0 &&
			tool143Get(p, "file_num_bytes").Uint() == 0 && tool143Get(p, "line_range_bytes").Uint() == 0 &&
			emptyRead156(tool143Get(result, "result").String())
		if !verbatimURIPath(path) || (!empty && (start < 1 || end < start)) {
			return nil, unsupported
		}
		v := tool143Native(m, "VIEW_FILE", "view_file")
		tool143Set(v, "absolute_path_uri", protoreflect.ValueOfString("file://"+path))
		if !empty {
			tool143Set(v, "start_line", protoreflect.ValueOfUint32(uint32(start-1)))
			tool143Set(v, "end_line", protoreflect.ValueOfUint32(uint32(end-1)))
		}
		tool143Set(v, "num_lines", tool143Get(p, "file_num_lines"))
		tool143Set(v, "num_bytes", tool143Get(p, "file_num_bytes"))
		tool143Copy(v, p, "line_range_bytes")
		tool143Set(v, "content", tool143Get(result, "result"))
		if skillFileRead146(path) {
			tool143Set(v, "is_skill_file", protoreflect.ValueOfBool(true))
		}
	case "exa.cortex.core.tools.FileEdit":
		if !onlyFields(p, fieldSet("kind", "path", "diff", "diff_stats", "is_artifact_file", "artifact_metadata")) {
			return nil, unsupported
		}
		kindField := p.Descriptor().Fields().ByName("kind")
		kind := kindField.Enum().Values().ByNumber(tool143Get(p, "kind").Enum())
		path := tool143Get(p, "path").String()
		if kind == nil || !verbatimURIPath(path) {
			return nil, unsupported
		}
		create := kind.Name() == "CREATE"
		overwrite := kind.Name() == "OVERWRITE"
		if overwrite {
			if !overwrite1411(generic, step, p, tool, path) {
				return nil, unsupported
			}
		} else if (create && (tool != "write_file" || !genericArgsAllowed(generic, "Path", "Content", "Overwrite", "ArtifactMetadata", "Append") || !nonAppendingWrite156(generic, step, path))) ||
			(!create && (kind.Name() != "EDIT" || tool != "edit_file" || !genericArgsAllowed(generic, "Path", "Target", "Replacement", "ArtifactMetadata"))) {
			return nil, unsupported
		}
		if !artifactMetadata225(generic, step, p, create, overwrite) {
			return nil, unsupported
		}
		v := tool143Native(m, "CODE_ACTION", "code_action")
		tool143Copy(v, p, "is_artifact_file")
		tool143CopyPresent(v, p, "artifact_metadata")
		spec := tool143Mutable(v, "action_spec")
		var target protoreflect.Message
		if create || overwrite {
			createFile := tool143Mutable(spec, "create_file")
			target = tool143Mutable(createFile, "path")
			if overwrite {
				tool143Set(createFile, "overwrite", protoreflect.ValueOfBool(true))
			}
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
		if tool != "invoke_subagent" || !genericArgsAllowed(generic, "Model", "Prompt", "Role", "TypeName", "Workspace") ||
			!onlyFields(p, fieldSet("subagents", "results")) {
			return nil, unsupported
		}
		// Captures qualify Workspace="inherit", "share" and "branch". It is a selector, not
		// a path: preserve the returned workspace URIs without deriving them.
		// The retained tool call must repeat every dropped wrapper argument.
		if workspace, present := genericArg(generic, "Workspace"); present &&
			((workspace != "inherit" && workspace != "share" && workspace != "branch") || !argumentsPresented(generic, step)) {
			return nil, unsupported
		}
		v := tool143Native(m, "INVOKE_SUBAGENT", "invoke_subagent")
		tool143Copy(v, p, "subagents", "results")
	case "exa.cortex.core.tools.ManageTask":
		if tool != "manage_task" || !genericArgsAllowed(generic, "Action", "TaskId", "Input") ||
			!onlyFields(p, fieldSet("action", "task_ids", "input")) {
			return nil, unsupported
		}
		action := tool143Get(p, "action").String()
		ids := tool143Get(p, "task_ids").List()
		if _, hasInput := genericArg(generic, "Input"); action != "send_input" && (hasInput || tool143Get(p, "input").String() != "") {
			return nil, unsupported
		}
		// The 1.5.5 empty list adds only action metadata, no fabricated ID.
		if ids.Len() == 0 && action == "list" {
			arg, present := genericArg(generic, "Action")
			if !present || arg != action || !genericArgsAllowed(generic, "Action") || !argumentsPresented(generic, step) || step.Has(f.ByName("task_details")) {
				return nil, unsupported
			}
			metadata := tool143Mutable(tool143Mutable(m, "generic"), "result").Mutable(result.Descriptor().Fields().ByName("metadata")).Map()
			metadata.Set(protoreflect.ValueOfString("action").MapKey(), protoreflect.ValueOfString(action))
			break
		}
		if action == "send_input" {
			input, hasInput := genericArg(generic, "Input")
			task, hasTask := genericArg(generic, "TaskId")
			argAction, _ := genericArg(generic, "Action")
			if !hasInput || input == "" || !hasTask || task == "" || argAction != action || ids.Len() != 1 ||
				ids.Get(0).String() != task || tool143Get(p, "input").String() != input || !argumentsPresented(generic, step) {
				return nil, unsupported
			}
		}
		if (action != "status" && action != "list" && action != "kill" && action != "send_input") || ids.Len() != 1 {
			return nil, unsupported
		}
		metadata := tool143Mutable(tool143Mutable(m, "generic"), "result").Mutable(result.Descriptor().Fields().ByName("metadata")).Map()
		metadata.Set(protoreflect.ValueOfString("action").MapKey(), protoreflect.ValueOfString(action))
		if action == "send_input" {
			metadata.Set(protoreflect.ValueOfString("input").MapKey(), tool143Get(p, "input"))
		}
		metadata.Set(protoreflect.ValueOfString("task_id").MapKey(), ids.Get(0))
		metadata.Set(protoreflect.ValueOfString("task_ids").MapKey(), ids.Get(0))
	case "exa.cortex.core.tools.Schedule":
		if tool143Get(p, "mode").String() == "cron" {
			if !qualifiedCron156(step, generic, p) {
				return nil, unsupported
			}
			metadata := tool143Mutable(tool143Mutable(m, "generic"), "result").Mutable(result.Descriptor().Fields().ByName("metadata")).Map()
			for _, name := range []protoreflect.Name{"mode", "outcome"} {
				metadata.Set(protoreflect.ValueOfString(string(name)).MapKey(), tool143Get(p, name))
			}
			if iterations := tool143Get(p, "iterations").Int(); iterations != 0 {
				metadata.Set(protoreflect.ValueOfString("iterations").MapKey(), protoreflect.ValueOfString(strconv.FormatInt(iterations, 10)))
			}
			return out, nil
		}
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

// qualifiedCommandDaemonFlag retains the historical explicit false and the
// completed IsDaemon=true command captured with a 1000 ms wait and no task.
// The displayed command omits is_daemon; the retained call keeps the flag.
func qualifiedCommandDaemonFlag(generic, step protoreflect.Message) bool {
	arg, ok := genericArg(generic, "IsDaemon")
	if !ok {
		return true
	}
	called, ok := calledArguments(step)
	if ok && arg == "false" && string(called["IsDaemon"]) == "false" {
		return true
	}
	wait, _ := genericArg(generic, "WaitMsBeforeAsync")
	return ok && arg == "true" && string(called["IsDaemon"]) == "true" && wait == "1000" &&
		statusName(step) == "CORTEX_STEP_STATUS_DONE" && !step.Has(step.Descriptor().Fields().ByName("task_details")) && argumentsPresented(generic, step)
}

// overwrite1411 qualifies the write_file over an existing file captured from
// agy 1.4.11, whose payload kind is OVERWRITE. The daemon presents it as a file
// creation with overwrite set, and its edit result does not mark the file as
// created. Every observed call set Overwrite and repeated all arguments; none
// was an artifact or brain scratch file. The 1.5.4 capture also qualifies
// artifact overwrites; artifactMetadata225 checks their retained metadata.
func overwrite1411(generic, step, payload protoreflect.Message, tool, path string) bool {
	arg, _ := genericArg(generic, "Path")
	if appendValue, ok := genericArg(generic, "Append"); ok {
		return tool == "write_file" && arg == path && appendValue == "true" &&
			exactGenericArgs(step, generic, "Path", "Content", "Append")
	}
	value, _ := genericArg(generic, "Overwrite")
	return tool == "write_file" && arg == path && value == "true" &&
		genericArgsAllowed(generic, "Path", "Content", "Overwrite", "ArtifactMetadata") && argumentsPresented(generic, step)
}

// artifactMetadata225 qualifies the artifact creation observed in IDE 2.25.0.
// The 1.5.4 capture adds overwrite metadata with RequestFeedback=true. Retained
// 1.5.6 captures add metadata on edits and artifact overwrites without metadata.
// Populated metadata must repeat the argument, including in the retained call.
func artifactMetadata225(generic, step, payload protoreflect.Message, create, overwrite bool) bool {
	arg, hasArg := genericArg(generic, "ArtifactMetadata")
	hasMetadata := payload.Has(payload.Descriptor().Fields().ByName("artifact_metadata"))
	if !hasArg && !hasMetadata {
		if !create || !tool143Get(payload, "is_artifact_file").Bool() {
			return true
		}
		// agy 1.4.8 also marks scratch files in the brain directory as artifacts
		// without artifact metadata. Keep the flag and require the retained
		// call to account for all wrapper arguments.
		path, hasPath := genericArg(generic, "Path")
		overwrite, hasOverwrite := genericArg(generic, "Overwrite")
		return hasPath && path == tool143Get(payload, "path").String() && (!hasOverwrite || overwrite == "true") && argumentsPresented(generic, step)
	}
	if !hasArg || !hasMetadata || !tool143Get(payload, "is_artifact_file").Bool() || !argumentsPresented(generic, step) {
		return false
	}
	meta, ok := decodeArtifactMetadata(arg)
	p := tool143Get(payload, "artifact_metadata").Message()
	return ok && (!meta.RequestFeedback || overwrite) &&
		onlyFields(p, fieldSet("summary", "updated_at", "user_facing", "request_feedback")) &&
		tool143Get(p, "request_feedback").Bool() == meta.RequestFeedback &&
		tool143Get(p, "summary").String() == meta.Summary &&
		tool143Get(p, "user_facing").Bool() == meta.UserFacing
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
