package completeness

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// The authenticated 1.5.0 descriptor declares these two fields. The 1.5.6
// descriptor reserves their numbers and names, and its RPC drops them from
// retained configurations. Descriptions and newer choice kinds remain compared.
const legacyChoiceLocations = "generator_metadata.planner_config.tool_config.invoke_subagent.model_options.options|executor_metadatas.cascade_config.planner_config.tool_config.invoke_subagent.model_options.options"
const SourceOnlyLegacyChoiceNamePath = legacyChoiceLocations + "/exa.cortex_pb.SubagentModelOption#1/wire2"
const SourceOnlyLegacyChoiceModelPath = legacyChoiceLocations + "/exa.cortex_pb.SubagentModelOption#2/wire2"

type legacyChoiceField struct {
	path        string
	number      protowire.Number
	occurrences []SourceOnlyOccurrence
	encoded     []byte
}

func (f *legacyChoiceField) add(location string, payload []byte) {
	f.occurrences = append(f.occurrences, SourceOnlyOccurrence{Location: location, Payload: payload})
	f.encoded = protowire.AppendBytes(protowire.AppendTag(f.encoded, f.number, protowire.BytesType), payload)
}

func (f legacyChoiceField) record() SourceOnlyField {
	sum := sha256.Sum256(f.encoded)
	return SourceOnlyField{Path: f.path, Count: len(f.occurrences), SHA256: hex.EncodeToString(sum[:]), Occurrences: f.occurrences}
}

// withoutLegacyModelChoices constructs the current RPC representation from
// saved fields. The original snapshot and its digest stay intact. This is an
// alternative to exact historical equality, selected only when the full RPC
// matches it, or when an attestation explicitly records these source-only bytes.
func (s *Snapshot) withoutLegacyModelChoices() (*Snapshot, bool) {
	trajectory := proto.Clone(s.trajectory)
	root := trajectory.ProtoReflect()
	names := legacyChoiceField{path: SourceOnlyLegacyChoiceNamePath, number: 1}
	models := legacyChoiceField{path: SourceOnlyLegacyChoiceModelPath, number: 2}
	for _, location := range []struct {
		record protoreflect.Name
		path   []protoreflect.Name
	}{
		{"generator_metadata", []protoreflect.Name{"planner_config", "tool_config", "invoke_subagent", "model_options"}},
		{"executor_metadatas", []protoreflect.Name{"cascade_config", "planner_config", "tool_config", "invoke_subagent", "model_options"}},
	} {
		records := tool143Get(root, location.record).List()
		for i := 0; i < records.Len(); i++ {
			message := records.Get(i).Message()
			at := fmt.Sprintf("%s[%d]", location.record, i)
			present := true
			for _, part := range location.path {
				field := message.Descriptor().Fields().ByName(part)
				if !message.Has(field) {
					present = false
					break
				}
				message = message.Get(field).Message()
				at += "." + string(part)
			}
			if !present {
				continue
			}
			options := tool143Get(message, "options").List()
			for j := 0; j < options.Len(); j++ {
				option := options.Get(j).Message()
				fields := option.Descriptor().Fields()
				nameField, modelField := fields.ByName("name"), fields.ByName("model")
				if !option.Has(nameField) && !option.Has(modelField) {
					continue
				}
				if !option.Has(nameField) || !option.Has(modelField) ||
					!onlyFields(option, fieldSet("name", "model", "description")) || option.Get(nameField).String() == "" {
					return nil, false
				}
				model := option.Get(modelField).Message()
				if !onlyFields(model, fieldSet("model_name", "thinking_level")) || tool143Get(model, "model_name").String() == "" {
					return nil, false
				}
				encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(model.Interface())
				if err != nil {
					return nil, false
				}
				where := fmt.Sprintf("%s.options[%d]", at, j)
				names.add(where, []byte(option.Get(nameField).String()))
				models.add(where, encoded)
				option.Clear(nameField)
				option.Clear(modelField)
			}
		}
	}
	if len(names.occurrences) == 0 {
		return nil, false
	}
	current := *s
	current.trajectory = trajectory
	current.sourceOnly = append(append([]SourceOnlyField{}, s.sourceOnly...), names.record(), models.record())
	return &current, true
}

// rpcSnapshot chooses a complete source-built representation. It never strips
// fields from incoming RPC data or accepts a partial match between alternatives.
func (s *Snapshot) rpcSnapshot(raw []byte) (*Snapshot, Result) {
	if s == nil || s.trajectory == nil {
		return nil, Result{Unknown, "source snapshot unavailable"}
	}
	incoming, err := rpcMessage(raw)
	if err != nil {
		return nil, Result{Unknown, err.Error()}
	}
	if proto.Equal(s.trajectory, incoming) {
		return s, Result{Verified, ""}
	}
	if current, ok := s.withoutLegacyModelChoices(); ok && proto.Equal(current.trajectory, incoming) {
		return current, Result{Verified, ""}
	}
	return nil, Result{Incomplete, "persisted and RPC trajectory content differ"}
}
