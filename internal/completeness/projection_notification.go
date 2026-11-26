package completeness

import (
	"strconv"

	"google.golang.org/protobuf/reflect/protoreflect"
)

const notificationRejection = "invalid arguments:\n- additional properties 'NotificationTimeoutSeconds' not allowed"

type notificationCommandArgs struct {
	command string
	wait    uint64
	timeout uint64
	bypass  bool
}

// The generic map stores text. Only accept the matching typed arguments of a
// builtin run_command with no pre-tool hooks.
func readNotificationCommandArgs(step, generic protoreflect.Message) (notificationCommandArgs, bool) {
	var args notificationCommandArgs
	timeout, present := genericArg(generic, "NotificationTimeoutSeconds")
	meta := tool143Get(step, "metadata").Message()
	if !present || genericToolName(step) != "run_command" || !historicalToolSource(step, "CORTEX_COMPONENT_SOURCE_BUILTIN") || tool143Get(meta, "pre_tool_hook_results").List().Len() != 0 {
		return args, false
	}
	names := []string{"CommandLine", "NotificationTimeoutSeconds", "WaitMsBeforeAsync", "toolSummary"}
	_, args.bypass = genericArg(generic, "BypassSandbox")
	if args.bypass {
		names = append(names, "BypassSandbox")
	}
	if !exactGenericArgs(step, generic, names...) {
		return args, false
	}
	called, _ := calledArguments(step)
	var validCommand, validWait, validTimeout bool
	args.command, validCommand = literalString(called["CommandLine"])
	_, validSummary := literalString(called["toolSummary"])
	wait, _ := genericArg(generic, "WaitMsBeforeAsync")
	args.wait, validWait = positiveCommandDuration(wait)
	args.timeout, validTimeout = positiveCommandDuration(timeout)
	return args, validCommand && args.command != "" && validSummary && validWait && validTimeout &&
		wait == string(called["WaitMsBeforeAsync"]) && timeout == string(called["NotificationTimeoutSeconds"]) &&
		(!args.bypass || string(called["BypassSandbox"]) == "true")
}

// notificationCommandWait qualifies a stored builtin call and returns the wait
// its ordinary RPC must show. agy 1.5.17 keeps the notification timeout in the
// retained arguments and presents the wait on every completed command. Canceled
// and rejected commands have no presented wait.
func notificationCommandWait(step, generic protoreflect.Message, approved bool) (uint64, bool) {
	args, ok := readNotificationCommandArgs(step, generic)
	meta := tool143Get(step, "metadata").Message()
	if !ok || !meta.Has(meta.Descriptor().Fields().ByName("completed_at")) {
		return 0, false
	}
	status := statusName(step)
	const pending = "CORTEX_STEP_STATUS_PENDING"
	if status == "CORTEX_STEP_STATUS_INVALID" {
		return 0, args.bypass && args.timeout == 1800 && args.wait == 5000 && onlyFields(step, fieldSet("type", "status", "metadata", "error", "generic")) && onlyFields(generic, fieldSet("args")) && exactHistoricalError(step, notificationRejection) && historicalTransitions(step, pending, status)
	}
	if status == "CORTEX_STEP_STATUS_ERROR" {
		return 0, !args.bypass && timedCommandRejected(step, generic, args.command)
	}
	if approved {
		// Both strict-mode captures kept their one permission exchange.
		// projectTool143 matched the grant and answer to the payload command.
		if args.bypass || status != "CORTEX_STEP_STATUS_DONE" || !onlyFields(step, fieldSet("type", "status", "metadata", "generic", "permissions", "completed_interactions", "task_details")) ||
			!historicalTransitions(step, pending, "CORTEX_STEP_STATUS_RUNNING", "CORTEX_STEP_STATUS_WAITING", "CORTEX_STEP_STATUS_RUNNING", status) ||
			!notificationResponseMatchesStep(step, meta) {
			return 0, false
		}
	} else if !onlyFields(step, fieldSet("type", "status", "metadata", "generic", "task_details")) || !historicalTransitions(step, pending, "CORTEX_STEP_STATUS_RUNNING", status) {
		return 0, false
	}
	background := step.Has(step.Descriptor().Fields().ByName("task_details"))
	if background && !notificationBackgroundTask(step) {
		return 0, false
	}
	if status == "CORTEX_STEP_STATUS_CANCELED" {
		return 0, background && !args.bypass
	}
	if status != "CORTEX_STEP_STATUS_DONE" {
		return 0, false
	}
	// The historical bypass capture is the only qualified bypass shape.
	if args.bypass && (!background || args.wait != 10000 || args.timeout != 120) {
		return 0, false
	}
	return args.wait, true
}

func notificationBackgroundTask(step protoreflect.Message) bool {
	task := tool143Get(step, "task_details").Message()
	return onlyFields(task, fieldSet("id", "log_uri", "progress", "description", "title", "moved_to_background_at")) &&
		task.Has(task.Descriptor().Fields().ByName("moved_to_background_at")) && tool143Get(task, "id").String() != ""
}

// A tool call contains JSON numbers; the generic argument map stores their
// text. Require a positive unsigned integer in its canonical decimal form.
func positiveCommandDuration(text string) (uint64, bool) {
	n, err := strconv.ParseUint(text, 10, 64)
	return n, err == nil && n > 0 && strconv.FormatUint(n, 10) == text
}

func notificationResponseMatchesStep(step, metadata protoreflect.Message) bool {
	interactions := tool143Get(step, "completed_interactions").List()
	if interactions.Len() != 1 {
		return false
	}
	source := tool143Get(metadata, "source_trajectory_step_info").Message()
	response := tool143Get(interactions.Get(0).Message(), "response").Message()
	id := tool143Get(source, "trajectory_id").String()
	return id != "" && id == tool143Get(response, "trajectory_id").String() &&
		tool143Get(source, "step_index").Uint() == tool143Get(response, "step_index").Uint()
}

// A denied timed command never started. Keep the original error and the
// matching denial; the ordinary response has no output, wait or exit code.
func timedCommandRejected(step, generic protoreflect.Message, command string) bool {
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic", "error", "completed_interactions")) ||
		!onlyFields(generic, fieldSet("args")) ||
		!historicalTransitions(step, "CORTEX_STEP_STATUS_PENDING", "CORTEX_STEP_STATUS_RUNNING", "CORTEX_STEP_STATUS_WAITING", "CORTEX_STEP_STATUS_RUNNING", "CORTEX_STEP_STATUS_ERROR") {
		return false
	}
	want := "permission check failed for command " + strconv.Quote(command) + ": user denied permission to run command:\n" + command + "\n" + rejectedCommandAdvice
	kind := permissionRecordKind{tool: "run_command", action: "command", target: command, trigger: "TYPE_EFFECTIVE_GRANT"}
	return exactHistoricalError(step, want) && notificationResponseMatchesStep(step, tool143Get(step, "metadata").Message()) &&
		completedRequest(tool143Get(step, "completed_interactions").List(), kind, false)
}
