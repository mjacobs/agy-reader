package completeness

import (
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// These saved wrapper shapes also occur in frozen in-flight and ACP sessions.
// Reconstruction may preserve a pending status or a permission exchange without
// certifying it as a verified daemon response. The sync verifier uses projectStep.
func projectOfflineTool(step protoreflect.Message, refusal error) (proto.Message, error) {
	f := step.Descriptor().Fields()
	if enumName(step, f.ByName("type")) != "CORTEX_STEP_TYPE_GENERIC" || !step.Has(f.ByName("generic")) || !onlyFields(step, permissionOuterFields) {
		return nil, refusal
	}
	generic := tool143Get(step, "generic").Message()
	if !onlyFields(generic, fieldSet("args", "result")) {
		return nil, refusal
	}
	result := tool143Get(generic, "result").Message()
	payload := tool143Get(result, "payload").Message()
	tool, status := genericToolName(step), statusName(step)
	switch tool143Get(payload, "type_url").String() {
	case "type.googleapis.com/exa.cortex.core.tools.RunCommand", "type.googleapis.com/exa.cortex.core.tools.FileEdit", "type.googleapis.com/exa.cortex.core.tools.InvokeSubagent":
		return projectTool143(step, generic, result, payload, true)
	case "":
	default:
		return nil, refusal
	}

	// The daemon displays these tools as generic steps. All arguments, results,
	// metadata and pending state are retained without interpreting them.
	if strings.HasPrefix(tool, "mcp_") && (status == "CORTEX_STEP_STATUS_DONE" || status == "CORTEX_STEP_STATUS_ERROR") && step.Has(f.ByName("permissions")) && step.Has(f.ByName("completed_interactions")) {
		return nil, nil
	}
	if tool == "schedule" && status == "CORTEX_STEP_STATUS_RUNNING" && genericArgsAllowed(generic, "CronExpression", "IsDaemon", "Prompt") {
		return nil, nil
	}
	if !argumentsPresented(generic, step) {
		return nil, refusal
	}
	if tool == "run_command" && status == "CORTEX_STEP_STATUS_INVALID" && exactHistoricalError(step, notificationRejection) {
		return nil, nil
	}
	if !onlyFields(result, fieldSet("step_render_info")) {
		return nil, refusal
	}
	switch tool {
	case "run_command":
		command, ok := genericArg(generic, "CommandLine")
		if ok && offlineUnfinishedCommand(step, generic, status) {
			return commandWithoutOutput(step, command), nil
		}
	case "read_file", "edit_file", "write_file":
		path, ok := genericArg(generic, "Path")
		// Failed file operations report the attempted URI with Unix separators.
		if status == "CORTEX_STEP_STATUS_ERROR" {
			editApproval := tool == "edit_file" && step.Has(f.ByName("permissions")) && step.Has(f.ByName("completed_interactions"))
			if !strings.Contains(path, `\`) && !editApproval {
				return nil, refusal
			}
			failure := tool143Get(step, "error").Message()
			short := tool143Get(failure, "short_error").String()
			if !onlyFields(step, permissionOuterFields) || !onlyFields(failure, fieldSet("short_error", "full_error")) || short == "" || tool143Get(failure, "full_error").String() != short || !result.Has(result.Descriptor().Fields().ByName("step_render_info")) {
				return nil, refusal
			}
			path = strings.ReplaceAll(path, `\`, "/")
		}
		if !ok || !verbatimURIPath(path) {
			return nil, refusal
		}
		if tool == "read_file" && status == "CORTEX_STEP_STATUS_ERROR" && genericArgsAllowed(generic, "Path") {
			out := proto.Clone(step.Interface())
			view := tool143Native(out.ProtoReflect(), "VIEW_FILE", "view_file")
			tool143Set(view, "absolute_path_uri", protoreflect.ValueOfString("file://"+path))
			return out, nil
		}
		if tool == "edit_file" && status == "CORTEX_STEP_STATUS_ERROR" && genericArgsAllowed(generic, "Path", "Target", "Replacement") {
			return editWithoutResult(step, path), nil
		}
		if tool == "write_file" && status == "CORTEX_STEP_STATUS_PENDING" && !generic.Has(generic.Descriptor().Fields().ByName("result")) {
			return nil, nil
		}
	}
	return nil, refusal
}

func offlineUnfinishedCommand(step, generic protoreflect.Message, status protoreflect.Name) bool {
	const pending = "CORTEX_STEP_STATUS_PENDING"
	const running = "CORTEX_STEP_STATUS_RUNNING"
	const canceled = "CORTEX_STEP_STATUS_CANCELED"
	f := step.Descriptor().Fields()
	meta := tool143Get(step, "metadata").Message()
	if tool143Get(meta, "pre_tool_hook_results").List().Len() != 0 || !onlyFields(step, permissionOuterFields) || step.Has(f.ByName("error")) {
		return false
	}
	if historicalTransitions(step, pending, running) && !step.Has(f.ByName("task_details")) && !meta.Has(meta.Descriptor().Fields().ByName("completed_at")) {
		return (status == running && bypassCommand1512(step, generic, "5000")) || (status == canceled && hookFreeUnrun157(step, generic) && !generic.Has(generic.Descriptor().Fields().ByName("result")))
	}
	if status != canceled || !step.Has(f.ByName("task_details")) || !meta.Has(meta.Descriptor().Fields().ByName("completed_at")) {
		return false
	}
	wait, _ := genericArg(generic, "WaitMsBeforeAsync")
	if wait != "10000" {
		return false
	}
	_, notification := genericArg(generic, "NotificationTimeoutSeconds")
	return (notification && exactGenericArgs(step, generic, "CommandLine", "NotificationTimeoutSeconds", "WaitMsBeforeAsync", "toolSummary") && historicalTransitions(step, pending, running, canceled)) ||
		(step.Has(f.ByName("permissions")) && step.Has(f.ByName("completed_interactions")) && exactGenericArgs(step, generic, "CommandLine", "WaitMsBeforeAsync", "toolSummary") && historicalTransitions(step, pending, running, "CORTEX_STEP_STATUS_WAITING", running, canceled))
}
