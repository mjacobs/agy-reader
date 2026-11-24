package completeness

import "google.golang.org/protobuf/reflect/protoreflect"

const notificationRejection = "invalid arguments:\n- additional properties 'NotificationTimeoutSeconds' not allowed"

// Historical builtin run_command calls retained the notification timeout in
// the tool call. Only these four argument and status combinations have been
// compared with the daemon's full and ordinary responses.
func historicalNotificationCommand(step, generic protoreflect.Message) bool {
	timeout, present := genericArg(generic, "NotificationTimeoutSeconds")
	if !present || genericToolName(step) != "run_command" || !historicalToolSource(step, "CORTEX_COMPONENT_SOURCE_BUILTIN") {
		return false
	}
	meta := tool143Get(step, "metadata").Message()
	if !meta.Has(meta.Descriptor().Fields().ByName("completed_at")) || tool143Get(meta, "pre_tool_hook_results").List().Len() != 0 {
		return false
	}
	args := []string{"CommandLine", "NotificationTimeoutSeconds", "WaitMsBeforeAsync", "toolSummary"}
	_, bypass := genericArg(generic, "BypassSandbox")
	if bypass {
		args = append(args, "BypassSandbox")
	}
	if !exactGenericArgs(step, generic, args...) {
		return false
	}
	called, _ := calledArguments(step)
	command, commandOK := literalString(called["CommandLine"])
	_, summaryOK := literalString(called["toolSummary"])
	if !commandOK || command == "" || !summaryOK {
		return false
	}
	if string(called["NotificationTimeoutSeconds"]) != timeout || (bypass && string(called["BypassSandbox"]) != "true") {
		return false
	}
	status := statusName(step)
	const pending = "CORTEX_STEP_STATUS_PENDING"
	if status == "CORTEX_STEP_STATUS_INVALID" {
		return bypass && timeout == "1800" && string(called["WaitMsBeforeAsync"]) == "5000" && onlyFields(step, fieldSet("type", "status", "metadata", "error", "generic")) && onlyFields(generic, fieldSet("args")) && exactHistoricalError(step, notificationRejection) && historicalTransitions(step, pending, status)
	}
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic", "task_details")) || string(called["WaitMsBeforeAsync"]) != "10000" || !historicalTransitions(step, pending, "CORTEX_STEP_STATUS_RUNNING", status) {
		return false
	}
	task := tool143Get(step, "task_details").Message()
	if !onlyFields(task, fieldSet("id", "log_uri", "progress", "description", "title", "moved_to_background_at")) || !task.Has(task.Descriptor().Fields().ByName("moved_to_background_at")) || tool143Get(task, "id").String() == "" {
		return false
	}
	switch status {
	case "CORTEX_STEP_STATUS_CANCELED":
		return !bypass && timeout == "120"
	case "CORTEX_STEP_STATUS_DONE":
		return (bypass && timeout == "120") || (!bypass && timeout == "180")
	}
	return false
}
