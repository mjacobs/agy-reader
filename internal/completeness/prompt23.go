package completeness

import (
	"math"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"
)

// ChatMessagePrompt field 23 (wire type 2) was written by agy CLI builds in use
// between mid-June and late July 2026, on the tool-result prompt that answers
// a grep_search call. Later builds put the same content in field 25
// (prompt_struct). Every observed payload is a google.protobuf.Struct that
// restates the grep_search step's results:
//
//	{"results": [{"path": ..., "line_content": ..., "line_number": ...}, ...],
//	 "num_elided_results": ...}
//
// with path-only entries when the search listed files rather than lines, and
// num_elided_results only when the step found more results than it kept.
// The profile accepts the field only in that shape and only when each value is
// derived from the step: the checks below on the prompt alone, and
// derivedFromStep once the trajectory's steps are known.
const (
	promptResultsKey       = "results"
	promptElidedKey        = "num_elided_results"
	promptPathKey          = "path"
	promptLineContentKey   = "line_content"
	promptLineNumberKey    = "line_number"
	promptMaxResults       = 50
	chatMessageSourceTool  = 4 // CHAT_MESSAGE_SOURCE_TOOL
	cortexStepTypeGrep     = 7 // CORTEX_STEP_TYPE_GREP_SEARCH
	maxPromptLineNumber    = math.MaxUint32 + 1
	maxPromptElidedResults = math.MaxUint32
)

// declaredPromptField reports whether a ChatMessagePrompt's unknown bytes are
// exactly one field 23, wire type 2, on a tool-result prompt that names its
// tool call and step, with a payload that parses strictly as a Struct of the
// observed shape. It returns the parsed payload.
func declaredPromptField(m protoreflect.Message, unknown []byte) (*structpb.Struct, bool) {
	num, typ, n := protowire.ConsumeTag(unknown)
	if n < 0 || num != promptSourceOnly || typ != protowire.BytesType {
		return nil, false
	}
	payload, l := protowire.ConsumeBytes(unknown[n:])
	if l < 0 || n+l != len(unknown) {
		return nil, false
	}
	fields := m.Descriptor().Fields()
	if m.Get(fields.ByName("source")).Enum() != chatMessageSourceTool ||
		m.Get(fields.ByName("tool_call_id")).String() == "" ||
		m.Get(fields.ByName("step_idx")).Int() <= 0 ||
		m.Has(fields.ByName("prompt_struct")) {
		return nil, false
	}
	s, ok := parsePromptStruct(payload)
	if !ok || !promptShape(s) {
		return nil, false
	}
	return s, true
}

// parsePromptStruct decodes payload as google.protobuf.Struct and refuses any
// unknown field. Go drops unknown fields inside map entries, and keeps only the
// last of a repeated key or oneof member, so the payload must also re-encode to
// its own length: anything dropped or merged makes it shorter.
func parsePromptStruct(payload []byte) (*structpb.Struct, bool) {
	var s structpb.Struct
	if err := (proto.UnmarshalOptions{RecursionLimit: maxDepth}).Unmarshal(payload, &s); err != nil {
		return nil, false
	}
	if proto.Size(&s) != len(payload) || hasUnknown(s.ProtoReflect()) {
		return nil, false
	}
	return &s, true
}

func hasUnknown(m protoreflect.Message) bool {
	if len(m.GetUnknown()) > 0 {
		return true
	}
	found := false
	m.Range(func(f protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case f.IsMap():
			if f.MapValue().Message() != nil {
				v.Map().Range(func(_ protoreflect.MapKey, item protoreflect.Value) bool {
					found = hasUnknown(item.Message())
					return !found
				})
			}
		case f.IsList():
			if f.Message() != nil {
				for i := 0; i < v.List().Len() && !found; i++ {
					found = hasUnknown(v.List().Get(i).Message())
				}
			}
		case f.Message() != nil:
			found = hasUnknown(v.Message())
		}
		return !found
	})
	return found
}

// promptShape checks the observed shape: a results list of 1 to 50 entries,
// all path-only or all with path, line content and a whole line number of at
// least 1, and at most a positive whole remaining count beside it.
func promptShape(s *structpb.Struct) bool {
	for key := range s.GetFields() {
		if key != promptResultsKey && key != promptElidedKey {
			return false
		}
	}
	results := s.GetFields()[promptResultsKey].GetListValue()
	if results == nil || len(results.GetValues()) == 0 || len(results.GetValues()) > promptMaxResults {
		return false
	}
	if v, ok := s.GetFields()[promptElidedKey]; ok && !wholeNumber(v, 1, maxPromptElidedResults) {
		return false
	}
	lines := len(results.GetValues()[0].GetStructValue().GetFields()) != 1
	for _, v := range results.GetValues() {
		entry := v.GetStructValue()
		if entry == nil || !isString(entry.GetFields()[promptPathKey]) {
			return false
		}
		if !lines {
			if len(entry.GetFields()) != 1 {
				return false
			}
			continue
		}
		if len(entry.GetFields()) != 3 || !isString(entry.GetFields()[promptLineContentKey]) || !wholeNumber(entry.GetFields()[promptLineNumberKey], 1, maxPromptLineNumber) {
			return false
		}
	}
	return true
}

func isString(v *structpb.Value) bool {
	_, ok := v.GetKind().(*structpb.Value_StringValue)
	return ok
}

func wholeNumber(v *structpb.Value, lowest, highest float64) bool {
	n, ok := v.GetKind().(*structpb.Value_NumberValue)
	return ok && n.NumberValue == math.Trunc(n.NumberValue) && n.NumberValue >= lowest && n.NumberValue <= highest
}

// derivedFromStep reports whether a field-23 prompt's payload is derived from
// the step it names: a GREP_SEARCH step at step_idx whose tool call is the
// prompt's, with the same number of results, where entry i's path is result
// i's relative or absolute path, its line content is result i's content and
// its line number is result i's line_number + 1. Path-only entries belong to a
// search that listed files (match_per_line false) and line entries to one that
// listed lines. A remaining count is present exactly when total_results
// exceeds the results kept, and equals the difference.
func derivedFromStep(m protoreflect.Message, unknown []byte, steps protoreflect.List) bool {
	s, ok := declaredPromptField(m, unknown)
	if !ok {
		return false
	}
	fields := m.Descriptor().Fields()
	idx := m.Get(fields.ByName("step_idx")).Int()
	if idx >= int64(steps.Len()) {
		return false
	}
	step := steps.Get(int(idx)).Message()
	sf := step.Descriptor().Fields()
	grepField := sf.ByName("grep_search")
	if step.Get(sf.ByName("type")).Enum() != cortexStepTypeGrep || !step.Has(grepField) {
		return false
	}
	metadata := step.Get(sf.ByName("metadata")).Message()
	call := metadata.Get(metadata.Descriptor().Fields().ByName("tool_call")).Message()
	if call.Get(call.Descriptor().Fields().ByName("id")).String() != m.Get(fields.ByName("tool_call_id")).String() {
		return false
	}
	grep := step.Get(grepField).Message()
	gf := grep.Descriptor().Fields()
	results := grep.Get(gf.ByName("results")).List()
	entries := s.GetFields()[promptResultsKey].GetListValue().GetValues()
	if len(entries) != results.Len() {
		return false
	}
	lines := grep.Get(gf.ByName("match_per_line")).Bool()
	for i, v := range entries {
		entry := v.GetStructValue().GetFields()
		result := results.Get(i).Message()
		rf := result.Descriptor().Fields()
		path := entry[promptPathKey].GetStringValue()
		if path != result.Get(rf.ByName("relative_path")).String() && path != result.Get(rf.ByName("absolute_path")).String() {
			return false
		}
		if (len(entry) == 3) != lines {
			return false
		}
		if !lines {
			continue
		}
		if entry[promptLineContentKey].GetStringValue() != result.Get(rf.ByName("content")).String() ||
			entry[promptLineNumberKey].GetNumberValue() != float64(result.Get(rf.ByName("line_number")).Uint())+1 {
			return false
		}
	}
	total := grep.Get(gf.ByName("total_results")).Uint()
	if total < uint64(len(entries)) {
		return false
	}
	elided, present := s.GetFields()[promptElidedKey]
	remaining := total - uint64(len(entries))
	if remaining == 0 {
		return !present
	}
	return present && elided.GetNumberValue() == float64(remaining)
}
