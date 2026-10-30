package completeness

import (
	"errors"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// qualifyRunningCommand1411 covers a run_command still executing when agy
// 1.4.11 persisted the conversation. Its result holds render information only.
// Captures before and after the command moved to the background (which adds
// task details) showed the default RPC presenting the generic wrapper
// unchanged, so the step needs no projection and is compared in full. The
// completed step replaces it once the command finishes.
func qualifyRunningCommand1411(step, generic, result protoreflect.Message) error {
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic", "task_details")) ||
		statusName(step) != "CORTEX_STEP_STATUS_RUNNING" ||
		!onlyFields(result, fieldSet("step_render_info")) ||
		!result.Has(result.Descriptor().Fields().ByName("step_render_info")) ||
		!genericArgsAllowed(generic, "CommandLine", "IsDaemon", "WaitMsBeforeAsync") ||
		!argumentsPresented(generic, step) || !notDaemon1411(generic, step) || !onlyAllowHookResult(step) {
		return errors.New("unsupported running run_command")
	}
	return nil
}
