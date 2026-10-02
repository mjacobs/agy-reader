package completeness

import (
	"bytes"
	"encoding/json"
	"errors"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// protojson's DiscardUnknown also discards unknown enum names. Validate known
// fields first, so RPC-only extension keys can survive in RawJSON without
// silently converting an unsupported known-field value to its default.
func validateRPCEnums(data []byte, descriptor protoreflect.MessageDescriptor) error {
	var value any
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := d.Decode(&value); err != nil {
		return err
	}
	return validateJSONMessage(value, descriptor, 0)
}

func validateJSONMessage(value any, md protoreflect.MessageDescriptor, depth int) error {
	if depth > maxDepth {
		return errors.New("RPC nesting limit")
	}
	if value == nil {
		return nil
	}
	name := md.FullName()
	if name == "google.protobuf.Any" {
		obj, ok := value.(map[string]any)
		if !ok {
			return errors.New("invalid Any JSON")
		}
		typ, _ := obj["@type"].(string)
		if typ == "" && len(obj) == 0 {
			return nil
		}
		mt, err := schemaTypes.FindMessageByURL(typ)
		if err != nil {
			return errors.New("unsupported Any JSON type")
		}
		nested := mt.Descriptor()
		if specialJSON(nested.FullName()) || nested.FullName() == "google.protobuf.Any" {
			return validateJSONMessage(obj["value"], nested, depth+1)
		}
		return validateJSONMessage(obj, nested, depth+1)
	}
	// These messages have special JSON encodings with no user enum fields.
	if specialJSON(name) {
		return nil
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return errors.New("invalid RPC message JSON")
	}
	for key, item := range obj {
		fd := md.Fields().ByJSONName(key)
		if fd == nil {
			fd = md.Fields().ByName(protoreflect.Name(key))
		}
		if fd == nil {
			continue
		}
		if item == nil {
			continue
		}
		if fd.IsMap() {
			entries, ok := item.(map[string]any)
			if !ok {
				return errors.New("invalid RPC map")
			}
			for _, v := range entries {
				if err := validateJSONValue(v, fd.MapValue(), depth+1); err != nil {
					return err
				}
			}
		} else if fd.IsList() {
			entries, ok := item.([]any)
			if !ok {
				return errors.New("invalid RPC list")
			}
			for _, v := range entries {
				if err := validateJSONValue(v, fd, depth+1); err != nil {
					return err
				}
			}
		} else if err := validateJSONValue(item, fd, depth+1); err != nil {
			return err
		}
	}
	return nil
}
func validateJSONValue(value any, fd protoreflect.FieldDescriptor, depth int) error {
	if value == nil {
		return nil
	}
	if fd.Kind() == protoreflect.EnumKind {
		if name, ok := value.(string); ok && fd.Enum().Values().ByName(protoreflect.Name(name)) == nil {
			return errors.New("unsupported RPC enum name")
		}
	}
	if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
		return validateJSONMessage(value, fd.Message(), depth)
	}
	return nil
}

func specialJSON(name protoreflect.FullName) bool {
	switch name {
	case "google.protobuf.Struct", "google.protobuf.Value", "google.protobuf.ListValue", "google.protobuf.Timestamp", "google.protobuf.Duration", "google.protobuf.FieldMask", "google.protobuf.DoubleValue", "google.protobuf.FloatValue", "google.protobuf.Int64Value", "google.protobuf.UInt64Value", "google.protobuf.Int32Value", "google.protobuf.UInt32Value", "google.protobuf.BoolValue", "google.protobuf.StringValue", "google.protobuf.BytesValue":
		return true
	}
	return false
}
