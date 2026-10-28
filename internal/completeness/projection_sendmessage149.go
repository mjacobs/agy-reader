package completeness

import (
	"errors"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// projectSendMessage149 covers completed native SendMessage payloads captured
// through agy 1.4.9. The daemon retains the generic step and adds recipient
// metadata from the payload. Arguments, message text and render information
// remain in the comparison; no part of the result is replaced or discarded.
// Payload-free historical calls keep their separate pass-through rule.
func projectSendMessage149(step, generic, result, payload protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported send_message projection")
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic")) ||
		genericToolName(step) != "send_message" || statusName(step) != "CORTEX_STEP_STATUS_DONE" ||
		!genericArgsAllowed(generic, "Recipient", "Message") || !argumentsPresented(generic, step) ||
		!onlyFields(result, fieldSet("result", "payload", "full_output_uri", "step_render_info")) {
		return nil, unsupported
	}
	decoded, err := decode("exa.cortex.core.tools.SendMessage", tool143Get(payload, "value").Bytes())
	if err != nil {
		return nil, err
	}
	p := decoded.ProtoReflect()
	if !onlyFields(p, fieldSet("recipient", "recipient_name", "message")) {
		return nil, unsupported
	}
	recipient, hasRecipient := genericArg(generic, "Recipient")
	message, hasMessage := genericArg(generic, "Message")
	name := tool143Get(p, "recipient_name").String()
	if !hasRecipient || recipient == "" || recipient != tool143Get(p, "recipient").String() ||
		!hasMessage || message == "" || message != tool143Get(p, "message").String() || name == "" {
		return nil, unsupported
	}
	out := proto.Clone(step.Interface())
	target := tool143Mutable(tool143Mutable(out.ProtoReflect(), "generic"), "result")
	metadata := target.Mutable(target.Descriptor().Fields().ByName("metadata")).Map()
	metadata.Set(protoreflect.ValueOfString("recipient").MapKey(), protoreflect.ValueOfString(recipient))
	metadata.Set(protoreflect.ValueOfString("recipient_name").MapKey(), protoreflect.ValueOfString(name))
	return out, nil
}

// qualifyInvalidSendMessage149 covers the captured rejection of Priority,
// which agy 1.4.9 rejects for this tool. There is no result or native
// payload, and the daemon presents the failed call and error unchanged. This
// does not qualify Priority on a completed SendMessage payload.
func qualifyInvalidSendMessage149(step, generic protoreflect.Message) error {
	unsupported := errors.New("unsupported invalid send_message projection")
	const rejection = "invalid arguments:\n- additional properties 'Priority' not allowed"
	if statusName(step) != "CORTEX_STEP_STATUS_INVALID" ||
		!onlyFields(generic, fieldSet("args")) || !argumentsPresented(generic, step) {
		return unsupported
	}
	args := tool143Get(generic, "args").Map()
	if args.Len() != 3 || !stringMapKeysAllowed(args, "Message", "Priority", "Recipient") {
		return unsupported
	}
	priority, _ := genericArg(generic, "Priority")
	recipient, _ := genericArg(generic, "Recipient")
	message, _ := genericArg(generic, "Message")
	failure := tool143Get(step, "error").Message()
	if priority != "high" || recipient == "" || message == "" ||
		!onlyFields(failure, fieldSet("short_error", "full_error")) ||
		tool143Get(failure, "short_error").String() != rejection ||
		tool143Get(failure, "full_error").String() != rejection {
		return unsupported
	}
	return nil
}
