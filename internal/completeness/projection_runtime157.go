package completeness

import (
	"errors"
	"strconv"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Auto/Default mode no longer records the implicit allow hook on a headless
// command canceled at its permission prompt. Keep the qualified no-Cwd call
// shape; projectCanceledRunCommand also checks the complete prompt history.
func hookFreeUnrun157(step, generic protoreflect.Message) bool {
	metadata := tool143Get(step, "metadata").Message()
	wait, _ := genericArg(generic, "WaitMsBeforeAsync")
	return tool143Get(metadata, "pre_tool_hook_results").List().Len() == 0 && unrunArgs1412(generic) && wait == "5000" &&
		enumName(metadata, metadata.Descriptor().Fields().ByName("tool_source")) == "CORTEX_COMPONENT_SOURCE_BUILTIN"
}

// A user deny rule fails before creating an interaction or hook result. The
// modern daemon still synthesizes an unrun command from the retained call.
func deniedRule157(step, generic protoreflect.Message) bool {
	if genericToolName(step) != "run_command" || statusName(step) != "CORTEX_STEP_STATUS_ERROR" ||
		!onlyFields(step, fieldSet("type", "status", "metadata", "generic", "error")) ||
		!hookFreeUnrun157(step, generic) || !plainFailure(step) || !argumentsPresented(generic, step) {
		return false
	}
	command, _ := genericArg(generic, "CommandLine")
	want := "permission check failed for command " + strconv.Quote(command) + ": Permission denied for command(" + command + "). Matches user-configured deny rule."
	return tool143Get(tool143Get(step, "error").Message(), "short_error").String() == want
}

// A persisted in-flight daemon cron is canceled on reload without producing a
// payload. Full and default RPC preserve the complete wrapper unchanged.
func canceledCron157(step, generic, result protoreflect.Message) bool {
	daemon, _ := genericArg(generic, "IsDaemon")
	cron, _ := genericArg(generic, "CronExpression")
	prompt, _ := genericArg(generic, "Prompt")
	task := tool143Get(step, "task_details").Message()
	return genericToolName(step) == "schedule" && statusName(step) == "CORTEX_STEP_STATUS_CANCELED" &&
		onlyFields(step, fieldSet("type", "status", "metadata", "generic", "task_details")) &&
		onlyFields(result, fieldSet("step_render_info")) && result.Has(result.Descriptor().Fields().ByName("step_render_info")) &&
		exactGenericArgs(step, generic, "CronExpression", "IsDaemon", "Prompt") && daemon == "true" && cron != "" && prompt != "" &&
		tool143Get(task, "id").String() != "" && tool143Get(task, "is_daemon").Bool() &&
		task.Has(task.Descriptor().Fields().ByName("moved_to_background_at"))
}

// Modern running commands retain their whole wrapper. On daemon reload the
// captured uncompleted background command becomes CANCELED and is rehydrated
// from the same arguments. No hook or permission decision is invented.
func hookFreeBackground157(step, generic protoreflect.Message) bool {
	metadata := tool143Get(step, "metadata").Message()
	wait, hasWait := genericArg(generic, "WaitMsBeforeAsync")
	value, err := strconv.ParseUint(wait, 10, 64)
	return hasWait && err == nil && strconv.FormatUint(value, 10) == wait &&
		exactGenericArgs(step, generic, "CommandLine", "WaitMsBeforeAsync", "toolSummary") &&
		tool143Get(metadata, "pre_tool_hook_results").List().Len() == 0 &&
		enumName(metadata, metadata.Descriptor().Fields().ByName("tool_source")) == "CORTEX_COMPONENT_SOURCE_BUILTIN"
}

// Esc at a strict read prompt leaves a resultless canceled wrapper. The
// default response shows only the requested path, retaining status/metadata.
func projectCanceledRead157(step, generic protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported canceled read_file")
	metadata := tool143Get(step, "metadata").Message()
	path, hasPath := genericArg(generic, "Path")
	if statusName(step) != "CORTEX_STEP_STATUS_CANCELED" ||
		!onlyFields(step, fieldSet("type", "status", "metadata", "generic")) ||
		!hasPath || !verbatimURIPath(path) || skillFileRead146(path) || !exactGenericArgs(step, generic, "Path") ||
		tool143Get(metadata, "pre_tool_hook_results").List().Len() != 0 ||
		enumName(metadata, metadata.Descriptor().Fields().ByName("tool_source")) != "CORTEX_COMPONENT_SOURCE_BUILTIN" {
		return nil, unsupported
	}
	out := proto.Clone(step.Interface())
	view := tool143Native(out.ProtoReflect(), "VIEW_FILE", "view_file")
	tool143Set(view, "absolute_path_uri", protoreflect.ValueOfString("file://"+path))
	return out, nil
}

// Strict-mode command and edit approvals retain both records on the modern
// projection. They must describe one allowing interaction for this exact call.
func modernPermissions157(step, generic, payload protoreflect.Message, name string) bool {
	kind := permissionRecordKind{trigger: "TYPE_EFFECTIVE_GRANT"}
	switch name {
	case "exa.cortex.core.tools.RunCommand":
		kind.tool = "run_command"
		kind.action = "command"
		kind.target = tool143Get(payload, "command_line").String()
		command, _ := genericArg(generic, "CommandLine")
		if command != kind.target {
			return false
		}
	case "exa.cortex.core.tools.FileEdit":
		kind.tool = "edit_file"
		kind.action = "write_file"
		kind.target = tool143Get(payload, "path").String()
		path, _ := genericArg(generic, "Path")
		if path != kind.target {
			return false
		}
	default:
		return false
	}
	return genericToolName(step) == kind.tool && kind.target != "" && argumentsPresented(generic, step) &&
		approvedGrant(tool143Get(step, "permissions").Message(), kind) && completedRequest(tool143Get(step, "completed_interactions").List(), kind, true)
}

// Modern prompted rejections use the unsandboxed wording and an effective
// grant trigger. The rejected answer and the request both remain compared.
func rejectedCommand157(step, generic protoreflect.Message) bool {
	if genericToolName(step) != "run_command" || statusName(step) != "CORTEX_STEP_STATUS_ERROR" ||
		!onlyFields(step, fieldSet("type", "status", "metadata", "generic", "error", "completed_interactions")) ||
		!hookFreeUnrun157(step, generic) || !plainFailure(step) || !argumentsPresented(generic, step) {
		return false
	}
	command, _ := genericArg(generic, "CommandLine")
	want := "permission check failed for unsandboxed " + strconv.Quote(command) + ": user denied permission to run command:\n" + command + "\n" + rejectedCommandAdvice
	kind := permissionRecordKind{tool: "run_command", action: "command", target: command, trigger: "TYPE_EFFECTIVE_GRANT"}
	return tool143Get(tool143Get(step, "error").Message(), "short_error").String() == want &&
		completedRequest(tool143Get(step, "completed_interactions").List(), kind, false)
}
