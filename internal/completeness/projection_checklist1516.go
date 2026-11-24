package completeness

import (
	"encoding/json"
	"errors"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// agy 1.5.16 returns a completed checklist update as the original generic
// step. Leave the whole record intact for comparison with both RPC responses.
func qualifyChecklist1516(step, generic, result protoreflect.Message) error {
	unsupported := errors.New("unsupported checklist update")
	body, ok := timestampedBody159(tool143Get(result, "result").String())
	if statusName(step) != "CORTEX_STEP_STATUS_DONE" ||
		!onlyFields(step, fieldSet("type", "status", "metadata", "generic")) ||
		!exactGenericArgs(step, generic, "steps") ||
		!onlyFields(result, fieldSet("result", "full_output_uri", "step_render_info")) ||
		tool143Get(result, "full_output_uri").String() == "" || !ok || body != "OK" {
		return unsupported
	}
	render := tool143Get(result, "step_render_info").Message()
	if !onlyFields(render, fieldSet("title", "markdown", "hidden")) ||
		!tool143Get(render, "hidden").Bool() || tool143Get(render, "title").String() == "" ||
		tool143Get(render, "markdown").String() == "" {
		return unsupported
	}
	called, _ := calledArguments(step)
	var checklist []map[string]string
	if json.Unmarshal(called["steps"], &checklist) != nil || len(checklist) == 0 {
		return unsupported
	}
	for _, entry := range checklist {
		if len(entry) != 2 || entry["title"] == "" {
			return unsupported
		}
		switch entry["status"] {
		case "pending", "in_progress", "done":
		default:
			return unsupported
		}
	}
	return nil
}
