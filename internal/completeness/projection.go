package completeness

import (
	"encoding/json"
	"errors"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// CompareResponses requires both the full unhydrated source representation and
// the default presentation to match this snapshot. The two responses must come
// from the same endpoint/trajectory; callers bracket them with source snapshots.
// Checking only the projection would miss changes to omitted wrapper content.
func (s *Snapshot) CompareResponses(fullRaw, presentation []byte) Result {
	if r := s.CompareFullRaw(fullRaw); r.State != Verified {
		return r
	}
	return s.ComparePresentation(presentation)
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
	outerFields := fieldSet("type", "status", "metadata", "generic", "task_details")
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
		generic := step.Get(f.ByName("generic")).Message()
		gf := generic.Descriptor().Fields()
		if !onlyFields(generic, fieldSet("args", "result")) || !generic.Has(gf.ByName("result")) {
			return nil, errors.New("unsupported generic fields")
		}
		result := generic.Get(gf.ByName("result")).Message()
		rf := result.Descriptor().Fields()
		tool := genericToolName(step)
		if !result.Has(rf.ByName("payload")) {
			// These two tools remain generic, including all their text, render
			// information and metadata. Missing payloads on other tools fail closed.
			var args []string
			fields := fieldSet("result", "full_output_uri", "step_render_info")
			switch tool {
			case "manage_task":
				args = []string{"Action", "TaskId"}
				fields["metadata"] = true
			case "send_message":
				args = []string{"Message", "Recipient"}
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

func qualifyNativeStep(step, generic, native protoreflect.Message, tool string) error {
	f, nf := step.Descriptor().Fields(), native.Descriptor().Fields()
	typ := nf.ByName("type").Enum().Values().ByNumber(native.Get(nf.ByName("type")).Enum())
	if typ == nil {
		return errors.New("unsupported native tool projection")
	}
	var args []string
	switch typ.Name() {
	case "CORTEX_STEP_TYPE_RUN_COMMAND":
		args = []string{"CommandLine", "Cwd", "WaitMsBeforeAsync"}
	case "CORTEX_STEP_TYPE_CODE_ACTION":
		args = []string{"CodeContent", "Overwrite", "TargetFile", "Description", "EndLine", "StartLine", "TargetContent", "AllowMultiple", "Instruction", "ReplacementContent"}
	case "CORTEX_STEP_TYPE_VIEW_FILE":
		args = []string{"AbsolutePath", "StartLine", "EndLine"}
	case "CORTEX_STEP_TYPE_INVOKE_SUBAGENT":
		if tool != "invoke_subagent" {
			return errors.New("unsupported native tool projection")
		}
		args = []string{"Subagents"}
	default:
		return errors.New("unsupported native tool projection")
	}
	if !genericArgsAllowed(generic, args...) {
		return errors.New("unsupported generic arguments")
	}
	if step.Get(f.ByName("status")).Enum() != native.Get(nf.ByName("status")).Enum() {
		return errors.New("unsupported wrapper/native status difference")
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
