package completeness

import (
	"errors"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// skillFileRead146 mirrors how agy presents a modern ReadFile payload. The
// payload has no marker; when agy 1.4.6 rebuilds the VIEW_FILE step for the
// ordinary RPC (rehydrateReadFile), it sets is_skill_file from the stored path
// alone: after turning backslashes into slashes, the path ends in "/SKILL.md"
// or is exactly "SKILL.md". The comparison is case-sensitive. It does not
// consult the executor's skill registry, the filesystem or the file content, so
// neither does this rule. A daemon answer with a different marker still fails
// exact comparison.
func skillFileRead146(path string) bool {
	path = strings.ReplaceAll(path, `\`, "/")
	return strings.HasSuffix(path, "/SKILL.md") || path == "SKILL.md"
}

// projectCanceledBackgroundCommand144 covers the canceled background commands
// captured on agy 1.4.4. Their result contains render information only; default
// RPC presents the command from its arguments with empty combined output. It
// preserves metadata and task details, but does not claim an exit code, cwd or
// blocking execution. Full-source equality still checks the dropped render data.
func projectCanceledBackgroundCommand144(step, generic, result protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported canceled background command projection")
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic", "task_details")) ||
		statusName(step) != "CORTEX_STEP_STATUS_CANCELED" ||
		!onlyFields(result, fieldSet("step_render_info")) ||
		!result.Has(result.Descriptor().Fields().ByName("step_render_info")) ||
		!genericArgsAllowed(generic, "CommandLine", "WaitMsBeforeAsync") ||
		!argumentsPresented(generic, step) || !onlyAllowHookResult(step) {
		return nil, unsupported
	}
	command, hasCommand := genericArg(generic, "CommandLine")
	wait, hasWait := genericArg(generic, "WaitMsBeforeAsync")
	if !hasCommand || !hasWait || wait != "5000" {
		return nil, unsupported
	}
	task := tool143Get(step, "task_details").Message()
	if !onlyFields(task, fieldSet("id", "log_uri", "progress", "description", "title", "moved_to_background_at")) ||
		tool143Get(task, "id").String() == "" || !task.Has(task.Descriptor().Fields().ByName("moved_to_background_at")) {
		return nil, unsupported
	}
	metadata := tool143Get(step, "metadata").Message()
	internal := tool143Get(metadata, "internal_metadata").Message()
	transitions := tool143Get(internal, "status_transitions").List()
	want := []protoreflect.Name{"CORTEX_STEP_STATUS_PENDING", "CORTEX_STEP_STATUS_RUNNING", "CORTEX_STEP_STATUS_CANCELED"}
	if transitions.Len() != len(want) {
		return nil, unsupported
	}
	for i, status := range want {
		transition := transitions.Get(i).Message()
		if enumName(transition, transition.Descriptor().Fields().ByName("updated_status")) != status {
			return nil, unsupported
		}
	}
	out := proto.Clone(step.Interface())
	run := tool143Native(out.ProtoReflect(), "RUN_COMMAND", "run_command")
	tool143Set(run, "command_line", protoreflect.ValueOfString(command))
	tool143Set(run, "proposed_command_line", protoreflect.ValueOfString(command))
	tool143Mutable(run, "combined_output")
	return out, nil
}

// projectManageSubagents144 covers the observed completed list calls naming two
// conversations, and the empty list captured on agy 1.4.9. The daemon preserves
// the wrapper and adds action and, when nonempty, the comma-joined conversation
// IDs to result metadata. Other actions, counts and permission-bearing shapes
// need their own native evidence.
func projectManageSubagents144(step, generic, result, payload protoreflect.Message) (proto.Message, error) {
	decoded, err := decode("exa.cortex.core.tools.ManageSubagents", tool143Get(payload, "value").Bytes())
	if err != nil {
		return nil, err
	}
	if tool143Get(decoded.ProtoReflect(), "subagents").List().Len() > 0 {
		return projectManageSubagents148(step, generic, result, decoded.ProtoReflect())
	}
	unsupported := errors.New("unsupported manage_subagents projection")
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic")) ||
		genericToolName(step) != "manage_subagents" || statusName(step) != "CORTEX_STEP_STATUS_DONE" ||
		!onlyFields(result, fieldSet("result", "payload", "full_output_uri", "step_render_info")) ||
		!genericArgsAllowed(generic, "Action") || !argumentsPresented(generic, step) {
		return nil, unsupported
	}
	action, present := genericArg(generic, "Action")
	if !present || action != "list" {
		return nil, unsupported
	}
	p := decoded.ProtoReflect()
	if !onlyFields(p, fieldSet("action", "conversation_ids")) || tool143Get(p, "action").String() != action {
		return nil, unsupported
	}
	ids := tool143Get(p, "conversation_ids").List()
	if ids.Len() != 0 && ids.Len() != 2 {
		return nil, unsupported
	}
	names := make([]string, ids.Len())
	for i := range names {
		names[i] = ids.Get(i).String()
		if names[i] == "" || strings.Contains(names[i], ",") {
			return nil, unsupported
		}
	}
	out := proto.Clone(step.Interface())
	clonedResult := tool143Mutable(tool143Mutable(out.ProtoReflect(), "generic"), "result")
	metadata := clonedResult.Mutable(clonedResult.Descriptor().Fields().ByName("metadata")).Map()
	metadata.Set(protoreflect.ValueOfString("action").MapKey(), protoreflect.ValueOfString(action))
	if len(names) > 0 {
		metadata.Set(protoreflect.ValueOfString("conversation_ids").MapKey(), protoreflect.ValueOfString(strings.Join(names, ",")))
	}
	return out, nil
}
