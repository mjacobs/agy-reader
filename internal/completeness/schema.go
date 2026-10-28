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
// It names the agy release and binary hash the descriptors were extracted from.
// The planner50 suffix adds the declared source-only CascadePlannerConfig field
// 50 (sourceonly.go). The permission2 suffix marks the stricter
// permission-record rule: a native RUN_COMMAND or VIEW_FILE step that carries a
// permission record under a wrapper without one is refused, where earlier
// profiles accepted it (permission1 did so for RUN_COMMAND alone). The prompt23
// suffix adds the declared source-only ChatMessagePrompt field 23 (prompt23.go)
// and a second kind of entry in an attestation's source-only record.
// The tools143 suffix adds the six qualified modern tool payload mappings.
// The combined tools143-permission1 suffix additionally refuses permission
// grants and completed interactions on those modern wrappers. This is separate
// from the historical native-Step permission1/permission2 rules above.
// The tools144 suffix adds qualified canceled background commands, modern
// subagent lists, inherited-workspace invocations and skill-file read markers,
// with the same permission restrictions. The skill1 suffix bound skill markers
// to the step executor's persisted enabled-skill registry; skill146 replaces it.
// The history2 suffix adds qualified historical generic wrappers, task input
// metadata and legacy failed-read skill registrations, preserving unmarked reads
// without enabled skill evidence that history1 unnecessarily refused;
// failedread146 replaces its failed-read skill rule.
// The ide225 suffix adds completed modern read permission records, zero byte
// offsets, and artifact file metadata qualified against IDE 2.25.0.
// The options145 suffix adds three agy 1.4.5 configuration fields to this
// historical descriptor base: browser auto-invoke, per-tool output overrides,
// and skip-plugin-logo. It does not enable unrelated newer definitions or BASH.
// Per-tool map entries are checked before protobuf decoding can discard bytes.
// The compaction146 suffix adds captured checkpoint requests (text and media),
// plus optional context-window and captured-media-token checkpoint settings.
// It also retains the kind-enablement map persisted by native 1.4.6 startup
// configuration, with strict map-wire checks and no source-only exemption.
// The skill146 suffix derives a modern read's skill marker from its stored path,
// as agy 1.4.6 does when it presents the read: the path ends in /SKILL.md.
// Executor registrations no longer decide it.
// The finish147 suffix adds the structured-result Finish payload and its
// qualified conversion to a native FINISH step, plus the missing-file read
// wrappers observed in the same capture.
// The replay147 suffix qualifies image reads and both complete historical and
// 1.4.7 command-wait presentations. No source-only declarations change.
// The failedread146 suffix replaces history2's registry rule for the skill
// marker of a failed legacy view_file with the skill146 path rule, as agy
// 1.4.6 viewFileFromArgs sets it. Executor registrations no longer decide it.
// The subagents148 suffix adds persisted subagent state and the captured
// single-child list/kill, rejected agent-definition, and artifact-overwrite
// presentations.
// The media149 suffix adds planner response media and generic structured result
// declarations. Unknown nested fields and unqualified structured-result
// projections remain unsupported. The tools149 suffix qualifies empty subagent
// lists and failed read presentations captured from agy 1.4.9.
// Attestations and saved audit manifests minted under a previous profile read
// as unknown until re-verified.
const Profile = "sqlite-v1-format0-agy-1.3.4-5b93565c5f04-planner50-permission2-prompt23-tools143-permission1-tools144-skill1-history2-ide225-options145-compaction146-skill146-finish147-replay147-failedread146-subagents148-media149-tools149"
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
	if err = unmarshalRecord(data, m); err != nil {
		return nil, errors.New("invalid persisted protobuf")
	}
	if err = normalize(m.ProtoReflect(), 0); err != nil {
		return nil, err
	}
	return m, nil
}

// unmarshalRecord decodes untrusted bytes into m, a message built from the
// embedded schema. protobuf-go (v1.36.11) can panic instead of failing while
// decoding a map entry into such a dynamic message: when an entry carries its
// key (or a scalar value) a second time with the wrong wire type, the decoder
// discards the first value but keeps the "seen" flag, then stores an empty
// value as the map key. The panic is reported as an ordinary decoding error, so
// the record reads as invalid rather than crashing the process. Generated
// messages such as structpb.Struct take a different decoder and are not
// affected. Regression seed:
// testdata/fuzz/FuzzDescriptorParser/df15c77f1523feba.
func unmarshalRecord(data []byte, m proto.Message) (err error) {
	if err = checkQualifiedMapsWire(data, m.ProtoReflect().Descriptor(), 0); err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("protobuf decoder failure: %v", r)
		}
	}()
	return proto.UnmarshalOptions{RecursionLimit: maxDepth, Resolver: schemaTypes}.Unmarshal(data, m)
}

// normalize rejects unsupported content before canonicalizing embedded Any
// messages. Any.value bytes can differ only in protobuf field/map ordering;
// comparing those bytes directly would reject otherwise identical messages.
//
// A generator_metadata or executor_metadatas record, decoded on its own or
// reached from the top-level trajectory, may carry a declared source-only
// field at the locations in sourceOnlyLocations (see declaredUnknown).
// Nothing else may carry unknown fields.
func normalize(m protoreflect.Message, depth int) error {
	var scope sourceOnlyScope
	if name := m.Descriptor().FullName(); depth == 0 && sourceOnlyLocations[name] != nil {
		scope.record = name
	}
	return normalizeIn(m, depth, scope)
}

func normalizeIn(m protoreflect.Message, depth int, scope sourceOnlyScope) error {
	if depth > maxDepth {
		return errors.New("protobuf nesting limit")
	}
	if unknown := m.GetUnknown(); len(unknown) > 0 && !declaredUnknown(scope, m, unknown) {
		return errUnknownFields
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
		if err = unmarshalRecord(data, nested); err != nil {
			return errors.New("invalid Any payload")
		}
		// The source-only allowance never extends into an embedded Any.
		if err = normalizeIn(nested.ProtoReflect(), depth+1, sourceOnlyScope{}); err != nil {
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
	root := depth == 0 && m.Descriptor().FullName() == trajectoryName
	check := func(f protoreflect.FieldDescriptor, v protoreflect.Value) error {
		switch f.Kind() {
		case protoreflect.MessageKind, protoreflect.GroupKind:
			next := scope.child(f.Name())
			if root && sourceOnlyRecordField(f.Name()) {
				next = sourceOnlyScope{record: f.Message().FullName()}
			}
			return normalizeIn(v.Message(), depth+1, next)
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
// the complete reconstructed protobuf, including normalized embedded messages
// and declared source-only bytes. trajectory is the copy used for comparisons,
// with the source-only bytes removed and recorded in sourceOnly.
type Snapshot struct {
	Profile    string
	Digest     string
	StepCount  int
	trajectory proto.Message
	sourceOnly []SourceOnlyField
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
// The declared source-only fields are the one exception: the comparison copy
// omits them, and the attestation records them instead.
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
	return finishSnapshotProfile(m, Profile)
}

func finishSnapshotProfile(m proto.Message, profile string) (*Snapshot, error) {
	if err := normalize(m.ProtoReflect(), 0); err != nil {
		return nil, err
	}
	data, err := (proto.MarshalOptions{Deterministic: true}).Marshal(m)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(append([]byte(profile+"\x00"), data...))
	// The RPC never carries the source-only fields, so every comparison uses a
	// copy without them. The digest above still covers the stored bytes.
	comparable := proto.Clone(m)
	sourceOnly, err := stripSourceOnly(comparable.ProtoReflect())
	if err != nil {
		return nil, err
	}
	return &Snapshot{Profile: profile, Digest: hex.EncodeToString(sum[:]), StepCount: m.ProtoReflect().Get(m.ProtoReflect().Descriptor().Fields().ByName("steps")).List().Len(), trajectory: comparable, sourceOnly: sourceOnly}, nil
}

// CascadeID is the authoritative persisted identity, independent of filename aliases.
func (s *Snapshot) CascadeID() string {
	m := s.trajectory.ProtoReflect()
	return m.Get(m.Descriptor().Fields().ByName("cascade_id")).String()
}
