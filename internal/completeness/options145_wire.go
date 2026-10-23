package completeness

import (
	"errors"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// checkQualifiedMapsWire checks the per_tool and kind_enablement maps before dynamic protobuf decoding.
// The decoder drops unknown map-entry fields and keeps only the last duplicate
// key. Neither can be recovered by normalize or covered by the source digest.
// Other unknown fields remain the responsibility of normalize.
func checkQualifiedMapsWire(data []byte, desc protoreflect.MessageDescriptor, depth int) error {
	var state qualifiedMapsWireState
	return state.check(data, desc, depth)
}

// Protobuf merges repeated occurrences of a singular message. Reuse validation
// state for those children so a duplicated map key cannot hide in separate
// copies of tool_output, planner_config, or another enclosing singular message.
// Process each occurrence immediately: retaining its payload slice would let
// many tiny occurrences allocate far more memory than the input limit permits.
type qualifiedMapsWireState struct {
	keys     map[protoreflect.FieldNumber]map[string]bool
	singular map[protoreflect.FieldNumber]*qualifiedMapsWireState
}

func (state *qualifiedMapsWireState) check(data []byte, desc protoreflect.MessageDescriptor, depth int) error {
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
			valueWire, checkedMap := qualifiedMapValueWire(field.FullName())
			if checkedMap {
				key, err := qualifiedMapKey(payload, valueWire)
				if err != nil {
					return err
				}
				if state.keys == nil {
					state.keys = make(map[protoreflect.FieldNumber]map[string]bool)
				}
				if state.keys[num] == nil {
					state.keys[num] = make(map[string]bool)
				}
				if state.keys[num][key] {
					return errors.New("duplicate persisted qualified map key")
				}
				state.keys[num][key] = true
			}
			if field.IsList() || field.IsMap() {
				if err := checkQualifiedMapsWire(payload, field.Message(), depth+1); err != nil {
					return err
				}
			} else {
				if state.singular == nil {
					state.singular = make(map[protoreflect.FieldNumber]*qualifiedMapsWireState)
				}
				if state.singular[num] == nil {
					state.singular[num] = &qualifiedMapsWireState{}
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

// These are the only maps qualified for this stricter wire contract. Unknown
// map-entry members cannot be recovered after protobuf decoding drops them.
func qualifiedMapValueWire(name protoreflect.FullName) (protowire.Type, bool) {
	switch name {
	case "exa.cortex_pb.ToolOutputConfig.per_tool":
		return protowire.BytesType, true
	case "exa.cortex_pb.CustomizationDiscoveryConfig.kind_enablement":
		return protowire.VarintType, true
	default:
		return 0, false
	}
}

// Missing key/value members retain protobuf defaults. Repeated members or
// fields other than the string key and qualified value are refused.
func qualifiedMapKey(data []byte, valueWire protowire.Type) (string, error) {
	var key string
	var seenKey, seenValue bool
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return "", errors.New("invalid persisted qualified map entry")
		}
		data = data[n:]
		if num != 1 && num != 2 || num == 1 && (seenKey || typ != protowire.BytesType) ||
			num == 2 && (seenValue || typ != valueWire) {
			return "", errUnknownFields
		}
		if num == 1 {
			var payload []byte
			payload, n = protowire.ConsumeBytes(data)
			key, seenKey = string(payload), true
		} else {
			seenValue = true
			n = protowire.ConsumeFieldValue(num, typ, data)
		}
		if n < 0 {
			return "", errors.New("invalid persisted qualified map entry")
		}
		data = data[n:]
	}
	return key, nil
}
