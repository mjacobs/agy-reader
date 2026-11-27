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
	unsupported := errors.New("unsupported running run_command")
	if statusName(step) != "CORTEX_STEP_STATUS_RUNNING" ||
		!onlyFields(result, fieldSet("step_render_info")) ||
		!result.Has(result.Descriptor().Fields().ByName("step_render_info")) {
		return unsupported
	}
	if _, timed := genericArg(generic, "NotificationTimeoutSeconds"); timed {
		// During the 1.5.17 command's wakeup, full and ordinary RPC retained
		// the complete running wrapper, including its background task ID.
		if !uncompletedNotificationTask(step, generic) {
			return unsupported
		}
		return nil
	}
	bypass := bypassCommand1512(step, generic, "10000")
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic", "task_details")) ||
		(!bypass && !genericArgsAllowed(generic, "CommandLine", "IsDaemon", "WaitMsBeforeAsync")) ||
		!argumentsPresented(generic, step) || !qualifiedCommandDaemonFlag(generic, step) || (!bypass && !onlyAllowHookResult(step) && !hookFreeBackground157(step, generic)) {
		return unsupported
	}
	return nil
}

// A live timed task has no completion time. The 1.5.17 daemon also reused
// the task's two-transition history when it reopened the database as CANCELED.
// ACP's running task has an additional wait and resume for its approval.
func uncompletedNotificationTask(step, generic protoreflect.Message) bool {
	args, ok := readNotificationCommandArgs(step, generic)
	meta := tool143Get(step, "metadata").Message()
	if !ok || args.bypass || !notificationBackgroundTask(step) || meta.Has(meta.Descriptor().Fields().ByName("completed_at")) {
		return false
	}
	if onlyFields(step, fieldSet("type", "status", "metadata", "generic", "task_details")) {
		return historicalTransitions(step, "CORTEX_STEP_STATUS_PENDING", "CORTEX_STEP_STATUS_RUNNING")
	}
	// T3's ACP daemon keeps the running wrapper unchanged, including task
	// progress and the approval it waited for. With no command result yet,
	// the request is the source of the shell name.
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic", "task_details", "permissions", "completed_interactions")) ||
		!historicalTransitions(step, "CORTEX_STEP_STATUS_PENDING", "CORTEX_STEP_STATUS_RUNNING", "CORTEX_STEP_STATUS_WAITING", "CORTEX_STEP_STATUS_RUNNING") ||
		!notificationResponseMatchesStep(step, meta) {
		return false
	}
	interaction := tool143Get(step, "completed_interactions").List().Get(0).Message()
	spec := tool143Get(tool143Get(interaction, "request").Message(), "permission").Message()
	return acpApproval1516(step, generic, acpCommandKind1516(args.command, tool143Get(spec, "shell_name").String()))
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
