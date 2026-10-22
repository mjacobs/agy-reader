package completeness

import (
	"errors"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// checkOptions145Wire checks the per_tool map before dynamic protobuf decoding.
// The decoder drops unknown map-entry fields and keeps only the last duplicate
// key. Neither can be recovered by normalize or covered by the source digest.
// Other unknown fields remain the responsibility of normalize.
func checkOptions145Wire(data []byte, desc protoreflect.MessageDescriptor, depth int) error {
	var state options145WireState
	return state.check(data, desc, depth)
}

// Protobuf merges repeated occurrences of a singular message. Reuse validation
// state for those children so a duplicated map key cannot hide in separate
// copies of tool_output, planner_config, or another enclosing singular message.
// Process each occurrence immediately: retaining its payload slice would let
// many tiny occurrences allocate far more memory than the input limit permits.
type options145WireState struct {
	keys     map[string]bool
	singular map[protoreflect.FieldNumber]*options145WireState
}

func (state *options145WireState) check(data []byte, desc protoreflect.MessageDescriptor, depth int) error {
	if depth > maxDepth {
		return errors.New("protobuf nesting limit")
	}
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return errors.New("invalid persisted protobuf")
		}
		data = data[n:]
		n = protowire.ConsumeFieldValue(num, typ, data)
		if n < 0 {
			return errors.New("invalid persisted protobuf")
		}
		field := desc.Fields().ByNumber(num)
		if field != nil && field.Message() != nil && typ == protowire.BytesType {
			payload, _ := protowire.ConsumeBytes(data[:n])
			if field.FullName() == "exa.cortex_pb.ToolOutputConfig.per_tool" {
				key, err := options145MapKey(payload)
				if err != nil {
					return err
				}
				if state.keys == nil {
					state.keys = make(map[string]bool)
				}
				if state.keys[key] {
					return errors.New("duplicate persisted tool output map key")
				}
				state.keys[key] = true
			}
			if field.IsList() || field.IsMap() {
				if err := checkOptions145Wire(payload, field.Message(), depth+1); err != nil {
					return err
				}
			} else {
				if state.singular == nil {
					state.singular = make(map[protoreflect.FieldNumber]*options145WireState)
				}
				if state.singular[num] == nil {
					state.singular[num] = &options145WireState{}
				}
				if err := state.singular[num].check(payload, field.Message(), depth+1); err != nil {
					return err
				}
			}
		}
		data = data[n:]
	}
	return nil
}

// Missing key/value members retain protobuf's defaults. Repeated members or
// fields other than the string key and message value are not a qualified entry.
func options145MapKey(data []byte) (string, error) {
	var key string
	var seenKey, seenValue bool
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return "", errors.New("invalid persisted tool output map entry")
		}
		data = data[n:]
		if typ != protowire.BytesType || num != 1 && num != 2 ||
			num == 1 && seenKey || num == 2 && seenValue {
			return "", errUnknownFields
		}
		payload, n := protowire.ConsumeBytes(data)
		if n < 0 {
			return "", errors.New("invalid persisted tool output map entry")
		}
		if num == 1 {
			key, seenKey = string(payload), true
		} else {
			seenValue = true
		}
		data = data[n:]
	}
	return key, nil
}
