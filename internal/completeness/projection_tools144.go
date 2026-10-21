package completeness

import (
	"errors"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// registeredSkillRead144 uses the skill registry saved for the step's execution,
// not the filename or current filesystem. In the 1.4.4 capture, modern ReadFile
// results gain is_skill_file only when that executor records the exact path as
// an enabled skill. Without a matching registration the previous unmarked
// projection remains; any unexpected daemon marker still fails exact comparison.
// Duplicate executor identities or path registrations are ambiguous and refused.
func registeredSkillRead144(trajectory, step protoreflect.Message, path string) (bool, error) {
	metadata := tool143Get(step, "metadata").Message()
	executionID := tool143Get(metadata, "execution_id").String()
	if executionID == "" {
		return false, nil
	}
	executors := tool143Get(trajectory, "executor_metadatas").List()
	var executor protoreflect.Message
	for i := 0; i < executors.Len(); i++ {
		candidate := executors.Get(i).Message()
		if tool143Get(candidate, "execution_id").String() != executionID {
			continue
		}
		if executor != nil {
			return false, errors.New("ambiguous skill-read executor")
		}
		executor = candidate
	}
	if executor == nil {
		return false, nil
	}
	registry := tool143Get(tool143Get(executor, "loaded_customizations").Message(), "customizations").List()
	var registration protoreflect.Message
	for i := 0; i < registry.Len(); i++ {
		candidate := registry.Get(i).Message()
		if tool143Get(candidate, "path").String() != path {
			continue
		}
		if registration != nil {
			return false, errors.New("ambiguous skill-read registration")
		}
		registration = candidate
	}
	if registration == nil {
		return false, nil
	}
	f := registration.Descriptor().Fields()
	return enumName(registration, f.ByName("type")) == "REFRESH_CUSTOMIZATION_TYPE_SKILL" &&
		enumName(registration, f.ByName("status")) == "STATUS_ENABLED", nil
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
// conversations. The daemon preserves the wrapper and adds action and the
// comma-joined conversation IDs to result metadata. Other actions, counts and
// permission-bearing shapes need their own native evidence.
func projectManageSubagents144(step, generic, result, payload protoreflect.Message) (proto.Message, error) {
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
	decoded, err := decode("exa.cortex.core.tools.ManageSubagents", tool143Get(payload, "value").Bytes())
	if err != nil {
		return nil, err
	}
	p := decoded.ProtoReflect()
	if !onlyFields(p, fieldSet("action", "conversation_ids")) || tool143Get(p, "action").String() != action {
		return nil, unsupported
	}
	ids := tool143Get(p, "conversation_ids").List()
	if ids.Len() != 2 {
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
	metadata.Set(protoreflect.ValueOfString("conversation_ids").MapKey(), protoreflect.ValueOfString(strings.Join(names, ",")))
	return out, nil
}
