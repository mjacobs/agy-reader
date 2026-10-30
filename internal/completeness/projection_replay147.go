package completeness

import (
	"encoding/json"
	"errors"
	"strconv"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// The same frozen source produced no modern-command waits on 1.4.6 and
// source-derived waits on 1.4.7. Compare two entire source-built projections;
// never remove incoming fields or choose behavior independently per step.
func compareProjections147(source, historical proto.Message, raw []byte) Result {
	r := compareProjection(historical, raw)
	if r.State == Unknown {
		return r
	}
	hasWait, err := hasCommandWait147(source, raw)
	if err != nil {
		return Result{Unknown, "invalid command presentation JSON"}
	}
	if !hasWait {
		return r
	}
	current, err := commandWaits147(source, historical)
	if err != nil {
		return Result{Incomplete, "daemon presentation differs from supported source projection"}
	}
	return compareProjection(current, raw)
}

// Scalar protobuf equality treats an explicit zero as absent. Presence in the
// JSON selects the current whole-trajectory rule, so a corrupted zero cannot
// masquerade as the historical omission of a nonzero stored wait.
func hasCommandWait147(source proto.Message, raw []byte) (bool, error) {
	// Match protobuf JSON keys exactly. A Go struct would also accept "Steps",
	// letting an unrelated ignored field change this presence check.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return false, err
	}
	var incoming []map[string]json.RawMessage
	if data, ok := top["steps"]; ok {
		if err := json.Unmarshal(data, &incoming); err != nil {
			return false, err
		}
	}
	steps := tool143Get(source.ProtoReflect(), "steps").List()
	for i := 0; i < steps.Len() && i < len(incoming); i++ {
		step := steps.Get(i).Message()
		generic := tool143Get(step, "generic").Message()
		payload := tool143Get(tool143Get(generic, "result").Message(), "payload").Message()
		if tool143Get(payload, "type_url").String() != "type.googleapis.com/exa.cortex.core.tools.RunCommand" {
			continue
		}
		for _, name := range []string{"runCommand", "run_command"} {
			var command map[string]json.RawMessage
			if json.Unmarshal(incoming[i][name], &command) != nil {
				continue
			}
			for _, field := range []string{"waitMsBeforeAsync", "wait_ms_before_async"} {
				if _, ok := command[field]; ok {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func commandWaits147(source, historical proto.Message) (proto.Message, error) {
	out := proto.Clone(historical)
	src := tool143Get(source.ProtoReflect(), "steps").List()
	dst := tool143Get(out.ProtoReflect(), "steps").List()
	for i := 0; i < src.Len(); i++ {
		step := src.Get(i).Message()
		if !step.Has(step.Descriptor().Fields().ByName("generic")) {
			continue
		}
		generic := tool143Get(step, "generic").Message()
		result := tool143Get(generic, "result").Message()
		payload := tool143Get(result, "payload").Message()
		if tool143Get(payload, "type_url").String() != "type.googleapis.com/exa.cortex.core.tools.RunCommand" {
			continue
		}
		wait, present := genericArg(generic, "WaitMsBeforeAsync")
		if !present {
			continue
		}
		value, err := strconv.ParseUint(wait, 10, 64)
		if err != nil || strconv.FormatUint(value, 10) != wait {
			return nil, errors.New("unsupported command wait argument")
		}
		command := tool143Mutable(dst.Get(i).Message(), "run_command")
		tool143Set(command, "wait_ms_before_async", protoreflect.ValueOfUint64(value))
	}
	return out, nil
}

// Frozen 1.4.7 captures contain single inline JPEG, PNG and WebP reads.
// All image bytes and MIME metadata are copied from the persisted result;
// no filesystem lookup or payload omission can satisfy this rule.
// The result text names the type of the stored media, except for the one pair
// imageReadNamed1410 qualifies.
func projectImageRead147(step, generic, result, payload protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported image read projection")
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic")) ||
		statusName(step) != "CORTEX_STEP_STATUS_DONE" || genericToolName(step) != "read_file" ||
		!genericArgsAllowed(generic, "Path") ||
		!onlyFields(result, fieldSet("result", "media", "payload", "step_render_info")) ||
		!onlyFields(payload, fieldSet("path", "file_num_bytes")) {
		return nil, unsupported
	}
	path := tool143Get(payload, "path").String()
	arg, present := genericArg(generic, "Path")
	if !plainAbsolutePath(path) || !present || arg != path || skillFileRead146(path) {
		return nil, unsupported
	}
	media := tool143Get(result, "media").List()
	if media.Len() != 1 {
		return nil, unsupported
	}
	item := media.Get(0).Message()
	if !onlyFields(item, fieldSet("mime_type", "inline_data")) || len(tool143Get(item, "inline_data").Bytes()) == 0 {
		return nil, unsupported
	}
	mime := tool143Get(item, "mime_type").String()
	if (mime != "image/jpeg" && mime != "image/png" && mime != "image/webp") ||
		!imageReadNamed1410(tool143Get(result, "result").String(), mime) {
		return nil, unsupported
	}
	out := proto.Clone(step.Interface())
	v := tool143Native(out.ProtoReflect(), "VIEW_FILE", "view_file")
	tool143Set(v, "absolute_path_uri", protoreflect.ValueOfString("file://"+path))
	tool143Set(v, "num_bytes", tool143Get(payload, "file_num_bytes"))
	tool143Set(v, "content", tool143Get(result, "result"))
	tool143Set(v, "media_data", media.Get(0))
	return out, nil
}

// imageReadNamed1410 matches the result text against the stored media type.
// agy 1.4.10 caps images at 2000 pixels per side and sends a large PNG as a
// JPEG. The captured read of a 3200x2400 PNG kept the file's own type in the
// text ("image/png"), reported the file's full size, and stored a 2000x1500
// JPEG. No other conversion was observed, so no other pair is qualified.
func imageReadNamed1410(text, mime string) bool {
	const prefix = "Showing binary file of type "
	return text == prefix+mime || (mime == "image/jpeg" && text == prefix+"image/png")
}
