package completeness

import (
	"errors"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// qualifyPayloadFree bounds the payload-free schedule, manage_subagents,
// define_subagent, list_permissions and ask_permission steps that the daemon
// presents unchanged, so the pass-through comparison covers everything shown.
// Statuses, outer fields, arguments and result fields are limited to the
// observed shapes: a schedule step may carry task details, or fail with status
// ERROR and an outer short/full error; a manage_subagents result may name
// killed conversations; define_subagent arguments are either the single
// definition or the separate fields; a list_permissions step has no arguments
// and a result of only text and render information; an ask_permission step has
// exactly the Action, Reason and Target arguments and a result of only text
// and render information.
func qualifyPayloadFree(step, generic, result protoreflect.Message, tool string, failed bool) error {
	f := step.Descriptor().Fields()
	fields := fieldSet("result", "full_output_uri", "step_render_info")
	var argsOK bool
	switch tool {
	case "schedule":
		argsOK = genericArgsAllowed(generic, "DurationSeconds", "Prompt", "TimerCondition")
	case "manage_subagents":
		argsOK = genericArgsAllowed(generic, "Action")
		fields["metadata"] = true
	case "define_subagent":
		argsOK = genericArgsAllowed(generic, "definition") ||
			genericArgsAllowed(generic, "description", "enable_mcp_tools", "enable_subagent_tools", "enable_write_tools", "name", "system_prompt")
	case "list_permissions":
		argsOK = generic.Get(generic.Descriptor().Fields().ByName("args")).Map().Len() == 0
		delete(fields, "full_output_uri")
	case "ask_permission":
		args := generic.Get(generic.Descriptor().Fields().ByName("args")).Map()
		argsOK = args.Len() == 3 && stringMapKeysAllowed(args, "Action", "Reason", "Target")
		delete(fields, "full_output_uri")
	default:
		return errors.New("unsupported generic result")
	}
	if !onlyFields(result, fields) {
		return errors.New("unsupported generic result")
	}
	if tool == "manage_subagents" && !stringMapKeysAllowed(result.Get(result.Descriptor().Fields().ByName("metadata")).Map(), "killed_conversation_ids") {
		return errors.New("unsupported generic result metadata")
	}
	if !argsOK {
		return errors.New("unsupported generic arguments")
	}
	if tool != "schedule" && step.Has(f.ByName("task_details")) {
		return errors.New("unsupported generic wrapper")
	}
	want := protoreflect.Name("CORTEX_STEP_STATUS_DONE")
	if failed {
		if tool != "schedule" || !onlyFields(step.Get(f.ByName("error")).Message(), fieldSet("short_error", "full_error")) {
			return errors.New("unsupported generic wrapper")
		}
		want = "CORTEX_STEP_STATUS_ERROR"
	}
	if statusName(step) != want {
		return errors.New("unsupported generic wrapper")
	}
	return nil
}
