package completeness

import (
	"errors"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Failed edit_file calls (e.g. target content not found) leave only render
// information in the result. The daemon retains the error and metadata and
// derives a CODE_ACTION command from Path with isEdit=true.
func projectFailedEdit(step, generic, result protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported failed edit_file projection")
	if !onlyFields(step, fieldSet("type", "status", "metadata", "error", "generic")) ||
		genericToolName(step) != "edit_file" || statusName(step) != "CORTEX_STEP_STATUS_ERROR" ||
		!onlyFields(result, fieldSet("step_render_info")) ||
		!result.Has(result.Descriptor().Fields().ByName("step_render_info")) ||
		!genericArgsAllowed(generic, "Path", "Target", "Replacement") || !argumentsPresented(generic, step) {
		return nil, unsupported
	}
	failure := tool143Get(step, "error").Message()
	shortError := tool143Get(failure, "short_error").String()
	if !onlyFields(failure, fieldSet("short_error", "full_error")) ||
		shortError == "" || tool143Get(failure, "full_error").String() != shortError {
		return nil, unsupported
	}
	path, ok := genericArg(generic, "Path")
	if !ok || !plainAbsolutePath(path) {
		return nil, unsupported
	}
	out := proto.Clone(step.Interface())
	v := tool143Native(out.ProtoReflect(), "CODE_ACTION", "code_action")
	spec := tool143Mutable(v, "action_spec")
	command := tool143Mutable(spec, "command")
	tool143Set(command, "is_edit", protoreflect.ValueOfBool(true))
	target := tool143Mutable(command, "file")
	tool143Set(target, "absolute_uri", protoreflect.ValueOfString("file://"+path))
	return out, nil
}
