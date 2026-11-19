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
	bypass := bypassCommand1512(step, generic, "10000")
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic", "task_details")) ||
		statusName(step) != "CORTEX_STEP_STATUS_RUNNING" ||
		!onlyFields(result, fieldSet("step_render_info")) ||
		!result.Has(result.Descriptor().Fields().ByName("step_render_info")) ||
		(!bypass && !genericArgsAllowed(generic, "CommandLine", "IsDaemon", "WaitMsBeforeAsync")) ||
		!argumentsPresented(generic, step) || !qualifiedCommandDaemonFlag(generic, step) || (!bypass && !onlyAllowHookResult(step) && !hookFreeBackground157(step, generic)) {
		return errors.New("unsupported running run_command")
	}
	return nil
}

// Both 1.5.12 captures retain the boolean bypass request in the builtin
// tool call without a working directory or a pre-tool hook decision.
func bypassCommand1512(step, generic protoreflect.Message, wait string) bool {
	actualWait, _ := genericArg(generic, "WaitMsBeforeAsync")
	flag, _ := genericArg(generic, "BypassSandbox")
	if flag != "true" || actualWait != wait {
		return false
	}
	called, ok := calledArguments(step)
	meta := tool143Get(step, "metadata").Message()
	return ok && string(called["BypassSandbox"]) == "true" &&
		exactGenericArgs(step, generic, "CommandLine", "WaitMsBeforeAsync", "toolSummary", "BypassSandbox") &&
		tool143Get(meta, "pre_tool_hook_results").List().Len() == 0 &&
		enumName(meta, meta.Descriptor().Fields().ByName("tool_source")) == "CORTEX_COMPONENT_SOURCE_BUILTIN"
}
