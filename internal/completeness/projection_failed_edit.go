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
	path, ok := failedFileTool(step, generic, result, "edit_file", "Path", "Target", "Replacement")
	if !ok {
		return nil, errors.New("unsupported failed edit_file projection")
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

// A write_file that agy 1.4.11 refused before writing (the captured call named
// an existing file without Overwrite) has the same result shape. The daemon
// presents the requested file creation from Path, without an overwrite flag or
// action result. A failed call with Overwrite was not observed.
func projectFailedWrite(step, generic, result protoreflect.Message) (proto.Message, error) {
	path, ok := failedFileTool(step, generic, result, "write_file", "Path", "Content")
	if !ok {
		return nil, errors.New("unsupported failed write_file projection")
	}
	out := proto.Clone(step.Interface())
	v := tool143Native(out.ProtoReflect(), "CODE_ACTION", "code_action")
	target := tool143Mutable(tool143Mutable(tool143Mutable(v, "action_spec"), "create_file"), "path")
	tool143Set(target, "absolute_uri", protoreflect.ValueOfString("file://"+path))
	return out, nil
}

// failedFileTool qualifies a failed file-tool wrapper whose result keeps only
// render information and whose error repeats one nonempty message. The
// retained tool call must account for every dropped argument.
func failedFileTool(step, generic, result protoreflect.Message, tool string, args ...string) (string, bool) {
	if !onlyFields(step, fieldSet("type", "status", "metadata", "error", "generic")) ||
		genericToolName(step) != tool || statusName(step) != "CORTEX_STEP_STATUS_ERROR" ||
		!onlyFields(result, fieldSet("step_render_info")) ||
		!result.Has(result.Descriptor().Fields().ByName("step_render_info")) ||
		!genericArgsAllowed(generic, args...) || !argumentsPresented(generic, step) {
		return "", false
	}
	failure := tool143Get(step, "error").Message()
	shortError := tool143Get(failure, "short_error").String()
	if !onlyFields(failure, fieldSet("short_error", "full_error")) ||
		shortError == "" || tool143Get(failure, "full_error").String() != shortError {
		return "", false
	}
	path, ok := genericArg(generic, "Path")
	return path, ok && plainAbsolutePath(path)
}
