package completeness

import "google.golang.org/protobuf/reflect/protoreflect"

// createAppendPresented qualifies the explicit Append flag on captured CREATE
// writes. The call retains the flag, while the file payload supplies the path.
func createAppendPresented(generic, step protoreflect.Message, path string) bool {
	appendValue, present := genericArg(generic, "Append")
	if !present {
		return true
	}
	called, ok := calledArguments(step)
	if appendValue == "true" && !exactGenericArgs(step, generic, "Path", "Content", "Append") {
		return false
	}
	argumentPath, _ := genericArg(generic, "Path")
	return ok && (appendValue == "false" || appendValue == "true") && string(called["Append"]) == appendValue &&
		argumentPath == path && argumentsPresented(generic, step)
}

// The captured cron schedules finish DONE after cancellation. Their task and
// text stay intact; the daemon adds mode, outcome and iterations metadata.
func qualifiedCron156(step, generic, payload protoreflect.Message) bool {
	if genericToolName(step) != "schedule" || statusName(step) != "CORTEX_STEP_STATUS_DONE" ||
		!onlyFields(payload, fieldSet("mode", "outcome", "iterations")) ||
		tool143Get(payload, "outcome").String() != "cancelled" || tool143Get(payload, "iterations").Int() < 0 ||
		!genericArgsAllowed(generic, "CronExpression", "IsDaemon", "Prompt") || !argumentsPresented(generic, step) ||
		!step.Has(step.Descriptor().Fields().ByName("task_details")) {
		return false
	}
	daemon, _ := genericArg(generic, "IsDaemon")
	cron, _ := genericArg(generic, "CronExpression")
	prompt, _ := genericArg(generic, "Prompt")
	called, ok := calledArguments(step)
	task := tool143Get(step, "task_details").Message()
	taskDaemon := tool143Get(task, "is_daemon").Bool()
	flagMatches := daemon == "true" && string(called["IsDaemon"]) == "true" && taskDaemon
	if !taskDaemon {
		flagMatches = exactGenericArgs(step, generic, "CronExpression", "Prompt")
	}
	return ok && flagMatches && cron != "" && prompt != "" && tool143Get(task, "id").String() != "" &&
		task.Has(task.Descriptor().Fields().ByName("moved_to_background_at"))
}
