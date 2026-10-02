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

// project implements the observed profile: a generic wrapper's recognized
// native Step replaces the entire wrapper, with no outer metadata/status merge.
// Its args, result text, full-output URI and outer metadata are checked through
// full raw equality, but intentionally absent from the default daemon contract.
func project(source proto.Message) (proto.Message, error) {
	out := proto.Clone(source)
	m := out.ProtoReflect()
	steps := m.Mutable(m.Descriptor().Fields().ByName("steps")).List()
	outerFields := map[protoreflect.Name]bool{"type": true, "status": true, "metadata": true, "generic": true}
	genericFields := map[protoreflect.Name]bool{"args": true, "result": true}
	resultFields := map[protoreflect.Name]bool{"result": true, "payload": true, "full_output_uri": true}
	allowedArgs := map[string]bool{"CommandLine": true, "Cwd": true, "toolAction": true, "toolSummary": true, "WaitMsBeforeAsync": true, "CodeContent": true, "Overwrite": true, "TargetFile": true, "Description": true, "EndLine": true, "AbsolutePath": true, "StartLine": true, "TargetContent": true, "AllowMultiple": true, "Instruction": true, "ReplacementContent": true}
	allowedTypes := map[protoreflect.Name]bool{"CORTEX_STEP_TYPE_RUN_COMMAND": true, "CORTEX_STEP_TYPE_CODE_ACTION": true, "CORTEX_STEP_TYPE_VIEW_FILE": true}
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
		if !onlyFields(generic, genericFields) || !generic.Has(gf.ByName("result")) {
			return nil, errors.New("unsupported generic fields")
		}
		argsOK := true
		generic.Get(gf.ByName("args")).Map().Range(func(k protoreflect.MapKey, _ protoreflect.Value) bool {
			argsOK = allowedArgs[k.String()]
			return argsOK
		})
		if !argsOK {
			return nil, errors.New("unsupported generic arguments")
		}
		result := generic.Get(gf.ByName("result")).Message()
		rf := result.Descriptor().Fields()
		if !onlyFields(result, resultFields) || !result.Has(rf.ByName("payload")) {
			return nil, errors.New("unsupported generic result")
		}
		payload := result.Get(rf.ByName("payload")).Message()
		pf := payload.Descriptor().Fields()
		if payload.Get(pf.ByName("type_url")).String() != "type.googleapis.com/gemini_coder.Step" {
			return nil, errors.New("unsupported generic payload type")
		}
		native, err := decode("gemini_coder.Step", payload.Get(pf.ByName("value")).Bytes())
		if err != nil {
			return nil, err
		}
		nm := native.ProtoReflect()
		nf := nm.Descriptor().Fields()
		nt := nf.ByName("type").Enum().Values().ByNumber(nm.Get(nf.ByName("type")).Enum())
		if nt == nil || !allowedTypes[nt.Name()] {
			return nil, errors.New("unsupported native tool projection")
		}
		if step.Get(f.ByName("status")).Enum() != nm.Get(nf.ByName("status")).Enum() {
			return nil, errors.New("unsupported wrapper/native status difference")
		}
		steps.Set(i, protoreflect.ValueOfMessage(nm))
	}
	return out, nil
}
