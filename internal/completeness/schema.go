// Package completeness compares a supported persisted trajectory with RPC data.
// An undecodable or unsupported source is unknown, never implicitly complete.
package completeness

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Profile identifies the independently checked SQLite/descriptor representation.
const Profile = "sqlite-v1-format0-agy-0d0d3eba22da"
const maxBytes = 64 << 20
const maxDepth = 100

//go:embed schema.pb.gz
var schemaBytes []byte
var schemaOnce sync.Once
var schemaFiles *protoregistry.Files
var schemaTypes *dynamicpb.Types
var schemaErr error

func loadSchema() error {
	schemaOnce.Do(func() {
		r, err := gzip.NewReader(bytes.NewReader(schemaBytes))
		if err != nil {
			schemaErr = err
			return
		}
		defer r.Close()
		data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
		if err != nil {
			schemaErr = err
			return
		}
		if len(data) > maxBytes {
			schemaErr = errors.New("descriptor size limit")
			return
		}
		var set descriptorpb.FileDescriptorSet
		if err = proto.Unmarshal(data, &set); err != nil {
			schemaErr = err
			return
		}
		schemaFiles, schemaErr = protodesc.NewFiles(&set)
		if schemaErr == nil {
			schemaTypes = dynamicpb.NewTypes(schemaFiles)
		}
	})
	return schemaErr
}

func newMessage(name protoreflect.FullName) (proto.Message, error) {
	if err := loadSchema(); err != nil {
		return nil, fmt.Errorf("supported schema unavailable: %w", err)
	}
	mt, err := schemaTypes.FindMessageByName(name)
	if err != nil {
		return nil, errors.New("unsupported message type")
	}
	return mt.New().Interface(), nil
}

func decode(name protoreflect.FullName, data []byte) (proto.Message, error) {
	if len(data) > maxBytes {
		return nil, errors.New("protobuf size limit")
	}
	m, err := newMessage(name)
	if err != nil {
		return nil, err
	}
	if err = (proto.UnmarshalOptions{RecursionLimit: maxDepth, Resolver: schemaTypes}).Unmarshal(data, m); err != nil {
		return nil, errors.New("invalid persisted protobuf")
	}
	if err = normalize(m.ProtoReflect(), 0); err != nil {
		return nil, err
	}
	return m, nil
}

// normalize rejects unsupported content before canonicalizing embedded Any
// messages. Any.value bytes can differ only in protobuf field/map ordering;
// comparing those bytes directly would reject otherwise identical messages.
func normalize(m protoreflect.Message, depth int) error {
	if depth > maxDepth {
		return errors.New("protobuf nesting limit")
	}
	if len(m.GetUnknown()) > 0 {
		return errors.New("unknown persisted protobuf fields")
	}
	if m.Descriptor().FullName() == "google.protobuf.Any" {
		fields := m.Descriptor().Fields()
		typ := m.Get(fields.ByName("type_url")).String()
		data := m.Get(fields.ByName("value")).Bytes()
		if typ == "" && len(data) == 0 {
			return nil
		}
		name := typ[strings.LastIndex(typ, "/")+1:]
		mt, err := schemaTypes.FindMessageByName(protoreflect.FullName(name))
		if err != nil {
			return errors.New("unsupported Any payload type")
		}
		nested := mt.New().Interface()
		if err = (proto.UnmarshalOptions{RecursionLimit: maxDepth, Resolver: schemaTypes}).Unmarshal(data, nested); err != nil {
			return errors.New("invalid Any payload")
		}
		if err = normalize(nested.ProtoReflect(), depth+1); err != nil {
			return err
		}
		data, err = (proto.MarshalOptions{Deterministic: true}).Marshal(nested)
		if err != nil {
			return err
		}
		m.Set(fields.ByName("value"), protoreflect.ValueOfBytes(data))
		return nil
	}
	var failure error
	check := func(f protoreflect.FieldDescriptor, v protoreflect.Value) error {
		switch f.Kind() {
		case protoreflect.MessageKind, protoreflect.GroupKind:
			return normalize(v.Message(), depth+1)
		case protoreflect.EnumKind:
			if f.Enum().Values().ByNumber(v.Enum()) == nil {
				return fmt.Errorf("unsupported enum in %s", f.FullName())
			}
		}
		return nil
	}
	m.Range(func(f protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case f.IsMap():
			v.Map().Range(func(_ protoreflect.MapKey, item protoreflect.Value) bool {
				failure = check(f.MapValue(), item)
				return failure == nil
			})
		case f.IsList():
			for i := 0; i < v.List().Len(); i++ {
				if failure = check(f, v.List().Get(i)); failure != nil {
					break
				}
			}
		default:
			failure = check(f, v)
		}
		return failure == nil
	})
	return failure
}

// Snapshot is one supported read-only database transaction. Its digest covers
// the complete reconstructed protobuf, including normalized embedded messages.
type Snapshot struct {
	Digest     string
	StepCount  int
	trajectory proto.Message
}

type State string

const (
	Verified   State = "verified"
	Incomplete State = "incomplete"
	Unknown    State = "unknown"
)

type Result struct {
	State  State
	Reason string
}

// CompareFullRaw compares the disableRehydration/full RPC representation only.
// It does not certify a separate hydrated response. Unsupported daemon fields
// or enum values are unknown; every persisted field must be known and equal.
// No user text is included in diagnostics.
func (s *Snapshot) CompareFullRaw(raw []byte) Result {
	if s == nil || s.trajectory == nil {
		return Result{Unknown, "source snapshot unavailable"}
	}
	m, err := rpcMessage(raw)
	if err != nil {
		return Result{Unknown, err.Error()}
	}
	if !proto.Equal(s.trajectory, m) {
		return Result{Incomplete, "persisted and RPC trajectory content differ"}
	}
	return Result{Verified, ""}
}

func finishSnapshot(m proto.Message) (*Snapshot, error) {
	if err := normalize(m.ProtoReflect(), 0); err != nil {
		return nil, err
	}
	data, err := (proto.MarshalOptions{Deterministic: true}).Marshal(m)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(append([]byte(Profile+"\x00"), data...))
	return &Snapshot{Digest: hex.EncodeToString(sum[:]), StepCount: m.ProtoReflect().Get(m.ProtoReflect().Descriptor().Fields().ByName("steps")).List().Len(), trajectory: m}, nil
}

// CascadeID is the authoritative persisted identity, independent of filename aliases.
func (s *Snapshot) CascadeID() string {
	m := s.trajectory.ProtoReflect()
	return m.Get(m.Descriptor().Fields().ByName("cascade_id")).String()
}
