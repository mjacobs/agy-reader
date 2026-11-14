package completeness

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// These captured tools stay generic in the default RPC. Qualification bounds
// their arguments and wrapper shapes; both comparisons still cover all saved
// script, output, render information and error content without transformation.
func qualifyWorkflow159(step, generic, result protoreflect.Message) error {
	unsupported := errors.New("unsupported workflow tool projection")
	if !argumentsPresented(generic, step) {
		return unsupported
	}
	text := tool143Get(result, "result").String()
	tool := genericToolName(step)
	if tool == "exit_plan_mode" || tool == "enter_plan_mode" {
		argument, expected := "plan", "Plan mode is off."
		if tool == "enter_plan_mode" {
			argument, expected = "reason", "Plan mode is on."
		}
		plan, _ := genericArg(generic, argument)
		body, ok := timestampedBody159(text)
		if statusName(step) != "CORTEX_STEP_STATUS_DONE" ||
			!onlyFields(step, fieldSet("type", "status", "metadata", "generic")) ||
			!onlyFields(result, fieldSet("result", "full_output_uri")) ||
			!exactGenericArgs(step, generic, argument) || plan == "" || !ok || body != expected ||
			tool143Get(result, "full_output_uri").String() == "" {
			return unsupported
		}
		return nil
	}
	if genericToolName(step) != "run_workflow" ||
		!genericArgsAllowed(generic, "Description", "Script", "ScriptPath", "Args", "Cwd", "WaitMsBeforeAsync") {
		return unsupported
	}
	description, _ := genericArg(generic, "Description")
	script, inline := genericArg(generic, "Script")
	path, file := genericArg(generic, "ScriptPath")
	if description == "" || inline == file || (inline && script == "") || (file && !verbatimURIPath(path)) {
		return unsupported
	}
	if cwd, ok := genericArg(generic, "Cwd"); ok && !verbatimURIPath(cwd) {
		return unsupported
	}
	if wait, ok := genericArg(generic, "WaitMsBeforeAsync"); ok && wait != "10000" {
		return unsupported
	}
	if args, ok := genericArg(generic, "Args"); ok {
		var object map[string]json.RawMessage
		if json.Unmarshal([]byte(args), &object) != nil || object == nil {
			return unsupported
		}
	}
	switch statusName(step) {
	case "CORTEX_STEP_STATUS_DONE":
		body, ok := timestampedBody159(text)
		if !onlyFields(step, fieldSet("type", "status", "metadata", "generic")) ||
			!onlyFields(result, fieldSet("result", "full_output_uri", "step_render_info")) ||
			tool143Get(result, "full_output_uri").String() == "" || !ok ||
			!strings.HasPrefix(body, "Workflow \""+description+"\" completed.\n") {
			return unsupported
		}
	case "CORTEX_STEP_STATUS_ERROR":
		if !onlyFields(step, fieldSet("type", "status", "metadata", "generic", "error")) ||
			!plainFailure(step) || !onlyFields(result, fieldSet("step_render_info")) ||
			!result.Has(result.Descriptor().Fields().ByName("step_render_info")) {
			return unsupported
		}
		failure := tool143Get(tool143Get(step, "error").Message(), "short_error").String()
		line, rest, ok := strings.Cut(strings.TrimPrefix(failure, "workflow execution failed (line "), "):")
		n, err := strconv.ParseUint(line, 10, 63)
		if !strings.HasPrefix(failure, "workflow execution failed (line ") || !ok || err != nil || n == 0 ||
			strconv.FormatUint(n, 10) != line || strings.TrimSpace(rest) == "" {
			return unsupported
		}
	default:
		return unsupported
	}
	return nil
}

func timestampedBody159(text string) (string, bool) {
	for _, prefix := range []string{"Created At: ", "Completed At: "} {
		line, rest, ok := strings.Cut(text, "\n")
		stamp, named := strings.CutPrefix(line, prefix)
		if !ok || !named {
			return "", false
		}
		if _, err := time.Parse(time.RFC3339, stamp); err != nil {
			return "", false
		}
		text = rest
	}
	return text, true
}
