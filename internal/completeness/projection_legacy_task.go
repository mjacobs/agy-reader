package completeness

import (
	"errors"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// qualifyManageTaskInput bounds the completed payload-free send_input calls
// captured in historical sessions on agy 1.4.4. Default RPC preserves the whole
// generic record. The repeated input and task identity must agree, and the
// source records one allow hook with no permission or interaction fields.
func qualifyManageTaskInput(step, generic, result protoreflect.Message) error {
	unsupported := errors.New("unsupported manage_task input")
	if genericToolName(step) != "manage_task" || statusName(step) != "CORTEX_STEP_STATUS_DONE" ||
		!onlyFields(step, fieldSet("type", "status", "metadata", "generic")) ||
		!onlyFields(result, fieldSet("result", "full_output_uri", "step_render_info", "metadata")) ||
		!genericArgsAllowed(generic, "Action", "TaskId", "Input") || !argumentsPresented(generic, step) ||
		!onlyAllowHookResult(step) {
		return unsupported
	}
	metadata := tool143Get(step, "metadata").Message()
	if enumName(metadata, metadata.Descriptor().Fields().ByName("tool_source")) != "CORTEX_COMPONENT_SOURCE_BUILTIN" {
		return unsupported
	}
	action, _ := genericArg(generic, "Action")
	input, hasInput := genericArg(generic, "Input")
	task, hasTask := genericArg(generic, "TaskId")
	if action != "send_input" || !hasInput || input == "" || !hasTask || task == "" {
		return unsupported
	}
	values := tool143Get(result, "metadata").Map()
	if values.Len() != 4 || !stringMapKeysAllowed(values, "input", "task_description", "task_id", "tool_name") {
		return unsupported
	}
	get := func(key string) string { return values.Get(protoreflect.ValueOfString(key).MapKey()).String() }
	if get("input") != input || get("task_id") != task || get("tool_name") != "run_command" {
		return unsupported
	}
	return nil
}
