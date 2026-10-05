package completeness

import (
	"encoding/json"
	"errors"
	"strconv"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// CompareResponses requires both the full unhydrated source representation and
// the default presentation to match this snapshot. The two responses must come
// from the same endpoint/trajectory; callers bracket them with source snapshots.
// Checking only the projection would miss changes to omitted wrapper content.
func (s *Snapshot) CompareResponses(fullRaw, presentation []byte) Result {
	r, _ := s.compareResponses(fullRaw, presentation)
	return r
}

// compareResponses is CompareResponses that also reports whether a refusal is
// source-proven: fixed by the snapshot and Profile alone, so that every daemon
// answer for this snapshot is refused.
//
// Only an unsupported projection of the snapshot is source-proven. It is
// reached only after the full representation equals the snapshot, and project
// reads nothing but the snapshot, so no presentation the daemon sends can ever
// verify it. Refusals found while validating either response (malformed JSON,
// an RPC enum name or Any type outside the profile, size or nesting limits)
// and differing content depend on what the daemon sent.
func (s *Snapshot) compareResponses(fullRaw, presentation []byte) (Result, bool) {
	if r := s.CompareFullRaw(fullRaw); r.State != Verified {
		return r, false
	}
	projected, err := project(s.trajectory)
	if err != nil {
		return Result{Unknown, err.Error()}, true
	}
	return compareProjection(projected, presentation), false
}

// ComparePresentation checks the supported daemon projection, not a lossless
// archive. Callers may use it alone only to recheck an existing attestation whose
// source digest also matches; it cannot mint a completeness attestation.
func (s *Snapshot) ComparePresentation(raw []byte) Result {
	if s == nil || s.trajectory == nil {
		return Result{Unknown, "source snapshot unavailable"}
	}
	projected, err := project(s.trajectory)
	if err != nil {
		return Result{Unknown, err.Error()}
	}
	return compareProjection(projected, raw)
}

func compareProjection(projected proto.Message, raw []byte) Result {
	incoming, err := rpcMessage(raw)
	if err != nil {
		return Result{Unknown, err.Error()}
	}
	if !proto.Equal(projected, incoming) {
		return Result{Incomplete, "daemon presentation differs from supported source projection"}
	}
	return Result{Verified, ""}
}

func rpcMessage(raw []byte) (proto.Message, error) {
	if len(raw) > maxBytes {
		return nil, errors.New("RPC size limit")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil || top == nil {
		return nil, errors.New("invalid trajectory JSON")
	}
	delete(top, "agyReader")
	data, err := json.Marshal(top)
	if err != nil {
		return nil, errors.New("invalid trajectory JSON")
	}
	m, err := newMessage("gemini_coder.Trajectory")
	if err != nil {
		return nil, err
	}
	if err = validateRPCEnums(data, m.ProtoReflect().Descriptor()); err != nil {
		return nil, err
	}
	if err = (protojson.UnmarshalOptions{DiscardUnknown: true, Resolver: schemaTypes, RecursionLimit: maxDepth}).Unmarshal(data, m); err != nil {
		return nil, errors.New("unsupported RPC protobuf JSON")
	}
	if err = normalize(m.ProtoReflect(), 0); err != nil {
		return nil, err
	}
	return m, nil
}

func onlyFields(m protoreflect.Message, allowed map[protoreflect.Name]bool) bool {
	ok := true
	m.Range(func(f protoreflect.FieldDescriptor, _ protoreflect.Value) bool { ok = allowed[f.Name()]; return ok })
	return ok
}

// project implements only qualified daemon presentation behaviors. Dropped
// wrapper content is still required to match in CompareFullRaw before a new
// attestation can be issued.
func project(source proto.Message) (proto.Message, error) {
	out := proto.Clone(source)
	m := out.ProtoReflect()
	steps := m.Mutable(m.Descriptor().Fields().ByName("steps")).List()
	// An outer error is accepted only on a failed payload-free manage_task or
	// schedule step or a failed resultless wrapper, all presented unchanged,
	// and on a failed native Step of an observed type whose own error it
	// repeats (see outerErrorTypes).
	outerFields := fieldSet("type", "status", "metadata", "generic", "task_details", "error")
	for i := 0; i < steps.Len(); i++ {
		step := steps.Get(i).Message()
		f := step.Descriptor().Fields()
		typ := f.ByName("type").Enum().Values().ByNumber(step.Get(f.ByName("type")).Enum())
		if typ == nil {
			return nil, errors.New("unsupported step type")
		}
		if typ.Name() != "CORTEX_STEP_TYPE_GENERIC" {
			continue
		}
		if !onlyFields(step, outerFields) || !step.Has(f.ByName("generic")) {
			return nil, errors.New("unsupported generic wrapper")
		}
		failed := step.Has(f.ByName("error"))
		generic := step.Get(f.ByName("generic")).Message()
		gf := generic.Descriptor().Fields()
		if !onlyFields(generic, fieldSet("args", "result")) || (!generic.Has(gf.ByName("result")) && !failed) {
			return nil, errors.New("unsupported generic fields")
		}
		if !generic.Has(gf.ByName("result")) {
			if err := qualifyResultless(step, generic); err != nil {
				return nil, err
			}
			continue
		}
		result := generic.Get(gf.ByName("result")).Message()
		rf := result.Descriptor().Fields()
		tool := genericToolName(step)
		if !result.Has(rf.ByName("payload")) {
			// These tools remain generic, including all their text, render
			// information and metadata. Missing payloads on other tools fail closed.
			var args []string
			fields := fieldSet("result", "full_output_uri", "step_render_info")
			switch tool {
			case "manage_task":
				args = []string{"Action", "TaskId"}
				fields["metadata"] = true
			case "send_message":
				args = []string{"Message", "Recipient"}
			case "schedule", "manage_subagents", "define_subagent":
				if err := qualifyPayloadFree(step, generic, result, tool, failed); err != nil {
					return nil, err
				}
				continue
			default:
				return nil, errors.New("unsupported generic result")
			}
			if !onlyFields(result, fields) {
				return nil, errors.New("unsupported generic result")
			}
			if tool == "manage_task" && !stringMapKeysAllowed(result.Get(rf.ByName("metadata")).Map(), "task_description", "task_id", "tool_name") {
				return nil, errors.New("unsupported generic result metadata")
			}
			if !genericArgsAllowed(generic, args...) {
				return nil, errors.New("unsupported generic arguments")
			}
			if failed && (tool != "manage_task" || statusName(step) != "CORTEX_STEP_STATUS_ERROR") {
				return nil, errors.New("unsupported generic wrapper")
			}
			continue
		}
		payload := result.Get(rf.ByName("payload")).Message()
		pf := payload.Descriptor().Fields()
		var native proto.Message
		var err error
		switch payload.Get(pf.ByName("type_url")).String() {
		case "type.googleapis.com/gemini_coder.Step":
			if !onlyFields(result, fieldSet("result", "payload", "full_output_uri", "media")) {
				return nil, errors.New("unsupported generic result")
			}
			native, err = decode("gemini_coder.Step", payload.Get(pf.ByName("value")).Bytes())
			if err == nil {
				err = qualifyNativeStep(step, generic, native.ProtoReflect(), tool)
			}
		case "type.googleapis.com/exa.cortex.core.tools.GenerateImage":
			if failed {
				return nil, errors.New("unsupported generic wrapper")
			}
			if tool != "generate_image" || !onlyFields(result, fieldSet("result", "payload", "full_output_uri", "media", "step_render_info")) {
				return nil, errors.New("unsupported image tool projection")
			}
			if !genericArgsAllowed(generic, "AspectRatio", "ImageName", "Prompt") {
				return nil, errors.New("unsupported generic arguments")
			}
			native, err = projectImage(step, result, payload.Get(pf.ByName("value")).Bytes())
		default:
			return nil, errors.New("unsupported generic payload type")
		}
		if err != nil {
			return nil, err
		}
		steps.Set(i, protoreflect.ValueOfMessage(native.ProtoReflect()))
	}
	return out, nil
}

func fieldSet(names ...protoreflect.Name) map[protoreflect.Name]bool {
	fields := make(map[protoreflect.Name]bool, len(names))
	for _, name := range names {
		fields[name] = true
	}
	return fields
}

func genericToolName(step protoreflect.Message) string {
	metadata := step.Get(step.Descriptor().Fields().ByName("metadata")).Message()
	call := metadata.Get(metadata.Descriptor().Fields().ByName("tool_call")).Message()
	return call.Get(call.Descriptor().Fields().ByName("name")).String()
}

func genericArgsAllowed(generic protoreflect.Message, names ...string) bool {
	allowed := map[string]bool{"toolAction": true, "toolSummary": true}
	for _, name := range names {
		allowed[name] = true
	}
	return mapKeysAllowed(generic.Get(generic.Descriptor().Fields().ByName("args")).Map(), allowed)
}

func stringMapKeysAllowed(m protoreflect.Map, names ...string) bool {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	return mapKeysAllowed(m, allowed)
}

func mapKeysAllowed(m protoreflect.Map, allowed map[string]bool) bool {
	ok := true
	m.Range(func(k protoreflect.MapKey, _ protoreflect.Value) bool {
		ok = allowed[k.String()]
		return ok
	})
	return ok
}

// qualifyPayloadFree bounds the payload-free schedule, manage_subagents and
// define_subagent steps that the daemon presents unchanged, so the pass-through
// comparison covers everything shown. Statuses, outer fields, arguments and
// result fields are limited to the observed shapes: a schedule step may carry
// task details, or fail with status ERROR and an outer short/full error; a
// manage_subagents result may name killed conversations; define_subagent
// arguments are either the single definition or the separate fields.
func qualifyPayloadFree(step, generic, result protoreflect.Message, tool string, failed bool) error {
	f := step.Descriptor().Fields()
	fields := fieldSet("result", "full_output_uri", "step_render_info")
	var argsOK bool
	switch tool {
	case "schedule":
		argsOK = genericArgsAllowed(generic, "DurationSeconds", "Prompt", "TimerCondition")
	case "manage_subagents":
		argsOK = genericArgsAllowed(generic, "Action")
		fields["metadata"] = true
	case "define_subagent":
		argsOK = genericArgsAllowed(generic, "definition") ||
			genericArgsAllowed(generic, "description", "enable_mcp_tools", "enable_subagent_tools", "enable_write_tools", "name", "system_prompt")
	default:
		return errors.New("unsupported generic result")
	}
	if !onlyFields(result, fields) {
		return errors.New("unsupported generic result")
	}
	if tool == "manage_subagents" && !stringMapKeysAllowed(result.Get(result.Descriptor().Fields().ByName("metadata")).Map(), "killed_conversation_ids") {
		return errors.New("unsupported generic result metadata")
	}
	if !argsOK {
		return errors.New("unsupported generic arguments")
	}
	if tool != "schedule" && step.Has(f.ByName("task_details")) {
		return errors.New("unsupported generic wrapper")
	}
	want := protoreflect.Name("CORTEX_STEP_STATUS_DONE")
	if failed {
		if tool != "schedule" || !onlyFields(step.Get(f.ByName("error")).Message(), fieldSet("short_error", "full_error")) {
			return errors.New("unsupported generic wrapper")
		}
		want = "CORTEX_STEP_STATUS_ERROR"
	}
	if statusName(step) != want {
		return errors.New("unsupported generic wrapper")
	}
	return nil
}

func qualifyNativeStep(step, generic, native protoreflect.Message, tool string) error {
	f, nf := step.Descriptor().Fields(), native.Descriptor().Fields()
	typ := nf.ByName("type").Enum().Values().ByNumber(native.Get(nf.ByName("type")).Enum())
	if typ == nil {
		return errors.New("unsupported native tool projection")
	}
	if step.Has(f.ByName("error")) && (!outerErrorTypes[typ.Name()] || !outerErrorRepeated(step, native)) {
		return errors.New("unsupported generic wrapper")
	}
	var args []string
	switch typ.Name() {
	case "CORTEX_STEP_TYPE_RUN_COMMAND":
		args = []string{"CommandLine", "Cwd", "WaitMsBeforeAsync"}
	case "CORTEX_STEP_TYPE_CODE_ACTION":
		args = []string{"CodeContent", "Overwrite", "TargetFile", "Description", "EndLine", "StartLine", "TargetContent", "AllowMultiple", "Instruction", "ReplacementContent"}
	case "CORTEX_STEP_TYPE_VIEW_FILE":
		args = []string{"AbsolutePath", "StartLine", "EndLine", "ContentOffset"}
		if !contentOffsetPresented(generic, native) {
			return errors.New("unsupported view_file content offset")
		}
	case "CORTEX_STEP_TYPE_INVOKE_SUBAGENT":
		if tool != "invoke_subagent" {
			return errors.New("unsupported native tool projection")
		}
		args = []string{"Subagents"}
	case "CORTEX_STEP_TYPE_GENERATE_IMAGE":
		// A successful call stores a GenerateImage payload (projectImage); an
		// embedded image Step was observed only for a failed call.
		if tool != "generate_image" || !step.Has(f.ByName("error")) {
			return errors.New("unsupported native tool projection")
		}
		args = []string{"AspectRatio", "ImageName", "Prompt"}
	default:
		return errors.New("unsupported native tool projection")
	}
	if !genericArgsAllowed(generic, args...) {
		return errors.New("unsupported generic arguments")
	}
	if step.Get(f.ByName("status")).Enum() != native.Get(nf.ByName("status")).Enum() {
		// A command canceled while running keeps its inner ERROR, with error
		// details, in the persisted payload; the daemon presents the outer
		// CANCELED status instead. Full raw equality still covers the dropped
		// inner status. Every other difference fails closed.
		if typ.Name() != "CORTEX_STEP_TYPE_RUN_COMMAND" || statusName(step) != "CORTEX_STEP_STATUS_CANCELED" ||
			statusName(native) != "CORTEX_STEP_STATUS_ERROR" || !native.Has(nf.ByName("error")) {
			return errors.New("unsupported wrapper/native status difference")
		}
		native.Set(nf.ByName("status"), step.Get(f.ByName("status")))
	}
	if step.Has(f.ByName("task_details")) {
		if native.Has(nf.ByName("task_details")) && !proto.Equal(step.Get(f.ByName("task_details")).Message().Interface(), native.Get(nf.ByName("task_details")).Message().Interface()) {
			return errors.New("unsupported wrapper/native task_details difference")
		}
		native.Set(nf.ByName("task_details"), step.Get(f.ByName("task_details")))
	} else if native.Has(nf.ByName("task_details")) {
		return errors.New("unsupported wrapper/native task_details difference")
	}
	return nil
}

// statusName returns the step status name, or "" for an undeclared value.
func statusName(step protoreflect.Message) protoreflect.Name {
	f := step.Descriptor().Fields().ByName("status")
	if v := f.Enum().Values().ByNumber(step.Get(f).Enum()); v != nil {
		return v.Name()
	}
	return ""
}

// outerErrorTypes lists the native Step types observed in retained sessions
// under a failed wrapper with an outer error. Add a type only with an observed
// source, full RPC and default RPC triplet; an outer error over any other
// native Step, including a qualified invoke_subagent, fails as it did before.
var outerErrorTypes = map[protoreflect.Name]bool{
	"CORTEX_STEP_TYPE_CODE_ACTION":    true,
	"CORTEX_STEP_TYPE_GENERATE_IMAGE": true,
	"CORTEX_STEP_TYPE_RUN_COMMAND":    true,
	"CORTEX_STEP_TYPE_VIEW_FILE":      true,
}

// outerErrorRepeated reports whether a failed wrapper's outer error only
// repeats the embedded native Step's own error. The daemon presents the native
// Step, with its inner error, and drops the outer one, so both steps must be
// ERROR and the outer short error must equal the inner one. The outer full
// error, which the presentation drops, is still covered by full raw equality.
func outerErrorRepeated(step, native protoreflect.Message) bool {
	f, nf := step.Descriptor().Fields(), native.Descriptor().Fields()
	if statusName(step) != "CORTEX_STEP_STATUS_ERROR" || statusName(native) != "CORTEX_STEP_STATUS_ERROR" || !native.Has(nf.ByName("error")) {
		return false
	}
	outer, inner := step.Get(f.ByName("error")).Message(), native.Get(nf.ByName("error")).Message()
	if !onlyFields(outer, fieldSet("short_error", "full_error")) {
		return false
	}
	short := outer.Descriptor().Fields().ByName("short_error")
	return outer.Get(short).String() != "" && outer.Get(short).String() == inner.Get(short).String()
}

// qualifyResultless accepts a generic wrapper that failed before its tool ran,
// so it stores arguments and an outer error but no result. The daemon presents
// these steps unchanged, so the pass-through comparison covers everything
// shown; tools and arguments are bounded to the observed shapes. A resultless
// view_file that names its tool source is presented instead as a VIEW_FILE
// step synthesized from its arguments, which is not qualified.
func qualifyResultless(step, generic protoreflect.Message) error {
	f := step.Descriptor().Fields()
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic", "error")) {
		return errors.New("unsupported generic wrapper")
	}
	// INVALID was observed only on write_to_file; the other tools were ERROR.
	if s := statusName(step); s != "CORTEX_STEP_STATUS_ERROR" && (s != "CORTEX_STEP_STATUS_INVALID" || genericToolName(step) != "write_to_file") {
		return errors.New("unsupported generic wrapper")
	}
	var args []string
	switch genericToolName(step) {
	case "find_by_name":
		args = []string{"Excludes", "Extensions", "Pattern", "SearchDirectory"}
	case "list_dir":
		args = []string{"DirectoryPath"}
	case "schedule":
		args = []string{"DurationSeconds", "Prompt", "TimerCondition"}
	case "view_file":
		metadata := step.Get(f.ByName("metadata")).Message()
		if metadata.Has(metadata.Descriptor().Fields().ByName("tool_source")) {
			return errors.New("unsupported resultless view_file")
		}
		args = []string{"AbsolutePath", "StartLine", "EndLine"}
	case "write_to_file":
		args = []string{"ArtifactMetadata", "CodeContent", "Description", "Overwrite", "TargetFile"}
	default:
		return errors.New("unsupported generic result")
	}
	if !genericArgsAllowed(generic, args...) {
		return errors.New("unsupported generic arguments")
	}
	return nil
}

// contentOffsetPresented lets view_file drop its ContentOffset argument only
// when the presented native step carries the same canonical value, both in its
// tool-call arguments and in view_file.content_offset. Without this, a changed
// argument would vanish from the comparison. An absent argument is the shape
// qualified before agy 1.2.16 wrote ContentOffset.
func contentOffsetPresented(generic, native protoreflect.Message) bool {
	args := generic.Get(generic.Descriptor().Fields().ByName("args")).Map()
	key := protoreflect.ValueOfString("ContentOffset").MapKey()
	if !args.Has(key) {
		return true
	}
	arg := args.Get(key).String()
	offset, err := strconv.ParseUint(arg, 10, 32)
	if err != nil || strconv.FormatUint(offset, 10) != arg {
		return false
	}
	nf := native.Descriptor().Fields()
	view := native.Get(nf.ByName("view_file")).Message()
	if view.Get(view.Descriptor().Fields().ByName("content_offset")).Uint() != offset {
		return false
	}
	metadata := native.Get(nf.ByName("metadata")).Message()
	call := metadata.Get(metadata.Descriptor().Fields().ByName("tool_call")).Message()
	var called map[string]json.RawMessage
	if json.Unmarshal([]byte(call.Get(call.Descriptor().Fields().ByName("arguments_json")).String()), &called) != nil {
		return false
	}
	return string(called["ContentOffset"]) == arg
}

func projectImage(step, result protoreflect.Message, data []byte) (proto.Message, error) {
	payload, err := decode("exa.cortex.core.tools.GenerateImage", data)
	if err != nil {
		return nil, err
	}
	image := payload.ProtoReflect()
	if !onlyFields(image, fieldSet("prompt", "aspect_ratio", "image_uri", "image_num_bytes", "model_name", "image_name")) {
		return nil, errors.New("unsupported image payload fields")
	}
	media := result.Get(result.Descriptor().Fields().ByName("media")).List()
	imageURI := image.Get(image.Descriptor().Fields().ByName("image_uri")).String()
	if media.Len() != 1 || imageURI == "" || media.Get(0).Message().Get(media.Get(0).Message().Descriptor().Fields().ByName("uri")).String() != imageURI {
		return nil, errors.New("unsupported image payload/media difference")
	}
	// Unlike embedded native Steps, the image projection retains outer metadata.
	native := proto.Clone(step.Interface())
	m := native.ProtoReflect()
	f := m.Descriptor().Fields()
	m.Set(f.ByName("type"), protoreflect.ValueOfEnum(f.ByName("type").Enum().Values().ByName("CORTEX_STEP_TYPE_GENERATE_IMAGE").Number()))
	m.Clear(f.ByName("generic"))
	generated := m.Mutable(f.ByName("generate_image")).Message()
	for _, name := range []protoreflect.Name{"prompt", "image_name", "aspect_ratio", "model_name"} {
		generated.Set(generated.Descriptor().Fields().ByName(name), image.Get(image.Descriptor().Fields().ByName(name)))
	}
	generated.Set(generated.Descriptor().Fields().ByName("generated_media"), media.Get(0))
	return native, nil
}
