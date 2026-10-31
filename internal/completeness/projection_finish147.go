package completeness

import (
	"errors"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// projectFinish147 reproduces the structured-result Finish in the stable
// agy 1.4.7 case-02 capture. The daemon copies arguments_json verbatim to both
// output["arguments_json"] and output_string, preserving the outer metadata.
// The dropped wrapper arguments must remain in that metadata's tool call;
// full-source comparison still checks result text and rendering information.
// Finishes without a result schema have no qualified modern mapping yet.
func projectFinish147(step, generic, result, payload protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported Finish projection")
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic")) ||
		genericToolName(step) != "finish" || statusName(step) != "CORTEX_STEP_STATUS_DONE" ||
		!onlyFields(result, fieldSet("result", "payload", "full_output_uri", "step_render_info")) ||
		!argumentsPresented(generic, step) {
		return nil, unsupported
	}
	decoded, err := decode("exa.cortex.core.tools.Finish", tool143Get(payload, "value").Bytes())
	if err != nil {
		return nil, err
	}
	p := decoded.ProtoReflect()
	if !tool143Get(p, "has_result_schema").Bool() {
		return nil, unsupported
	}
	out := proto.Clone(step.Interface())
	finish := tool143Native(out.ProtoReflect(), "FINISH", "finish")
	text := tool143Get(p, "arguments_json")
	tool143Set(finish, "output_string", text)
	output := finish.Mutable(finish.Descriptor().Fields().ByName("output")).Map()
	output.Set(protoreflect.ValueOfString("arguments_json").MapKey(), text)
	return out, nil
}

// Missing-file reads in 1.4.7 and corrupt-image reads in 1.4.9 leave only
// render information in the result. The daemon retains the error and metadata
// and derives a VIEW_FILE URI from Path. Error text is preserved verbatim;
// accepting a different message does not omit it from either comparison.
func projectFailedRead149(step, generic, result protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported failed read_file projection")
	if !onlyFields(step, fieldSet("type", "status", "metadata", "error", "generic")) ||
		genericToolName(step) != "read_file" || statusName(step) != "CORTEX_STEP_STATUS_ERROR" ||
		!onlyFields(result, fieldSet("step_render_info")) ||
		!result.Has(result.Descriptor().Fields().ByName("step_render_info")) ||
		!genericArgsAllowed(generic, "Path") || !argumentsPresented(generic, step) {
		return nil, unsupported
	}
	failure := tool143Get(step, "error").Message()
	shortError := tool143Get(failure, "short_error").String()
	if !onlyFields(failure, fieldSet("short_error", "full_error")) ||
		shortError == "" || tool143Get(failure, "full_error").String() != shortError {
		return nil, unsupported
	}
	path, ok := genericArg(generic, "Path")
	if !ok || !verbatimURIPath(path) || skillFileRead146(path) {
		return nil, unsupported
	}
	out := proto.Clone(step.Interface())
	view := tool143Native(out.ProtoReflect(), "VIEW_FILE", "view_file")
	tool143Set(view, "absolute_path_uri", protoreflect.ValueOfString("file://"+path))
	return out, nil
}
