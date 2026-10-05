package completeness

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Historical sources carry CascadePlannerConfig field 50 (wire type 2) inside
// generator_metadata and executor_metadatas records. The current descriptor
// reserves that number and the daemon's RPC never sends it, so full-raw
// equality cannot cover it. The profile declares it source-only by number and
// wire type alone: no name or type is assigned, because no authentic
// definition for the stored bytes is known. Its payload is limited to the
// shapes observed in those sources: empty, or varint fields 2 and/or 4, each
// once, in that order, and set to zero.
const (
	trajectoryName        protoreflect.FullName = "gemini_coder.Trajectory"
	generatorMetadataName protoreflect.FullName = "exa.cortex_pb.CortexStepGeneratorMetadata"
	executorMetadataName  protoreflect.FullName = "exa.cortex_pb.ExecutorMetadata"
	plannerConfigName     protoreflect.FullName = "exa.cortex_pb.CascadePlannerConfig"
	plannerSourceOnly     protowire.Number      = 50

	// SourceOnlyPlannerPath names the declared locations in an attestation:
	// the field-name paths of sourceOnlyPlannerPaths below, from the trajectory.
	SourceOnlyPlannerPath = "generator_metadata.planner_config|executor_metadatas.cascade_config.planner_config/exa.cortex_pb.CascadePlannerConfig#50/wire2"
	// SourceOnlyReason marks a verified result that relies on declared
	// source-only content. Each output that reports a verification result
	// shows it apart from plain verified; it is as fresh and as qualified as a
	// plain verified one.
	SourceOnlyReason = "verified except declared source-only fields absent from RPC"
)

// sourceOnlyPlannerPaths lists, per record type, the only CascadePlannerConfig
// locations that may carry the declared field: exactly the ones observed in the
// retained historical sources. Every generator occurrence was at planner_config
// and every executor occurrence at cascade_config.planner_config; none was at
// ExecutorMetadata.request_override_config.planner_config, which stays unknown.
// The descriptor nests planners recursively (agent definitions in a planner's
// customization settings carry their own cascade and planner configuration, and
// ExecutorMetadata.agent_config_resolution holds more), so "any planner inside
// the record" would be an unbounded set of locations. Those nested planners,
// steps, trajectories and embedded Any payloads never carry it.
var sourceOnlyPlannerPaths = map[protoreflect.FullName][]string{
	generatorMetadataName: {"planner_config"},
	executorMetadataName:  {"cascade_config.planner_config"},
}

// sourceOnlyScope is a position inside a generator or executor record: the
// record type and the dot-separated field names from it. The zero value is
// outside any record, or off every declared path.
type sourceOnlyScope struct {
	record protoreflect.FullName
	path   string
}

// child descends into field name, staying in scope only while the path is a
// declared location or leads to one.
func (s sourceOnlyScope) child(name protoreflect.Name) sourceOnlyScope {
	if s.record == "" {
		return sourceOnlyScope{}
	}
	next := string(name)
	if s.path != "" {
		next = s.path + "." + next
	}
	for _, p := range sourceOnlyPlannerPaths[s.record] {
		if p == next || strings.HasPrefix(p, next+".") {
			return sourceOnlyScope{s.record, next}
		}
	}
	return sourceOnlyScope{}
}

// declared reports whether the scope is exactly one of the declared locations.
func (s sourceOnlyScope) declared() bool {
	for _, p := range sourceOnlyPlannerPaths[s.record] {
		if p == s.path {
			return true
		}
	}
	return false
}

// SourceOnlyField records the declared source-only content an attestation
// covers. SHA256 is the digest of the stored field encodings (tag, length and
// payload) concatenated in Occurrences order. The sidecar keeps the payloads
// themselves under agyReader.completeness, so nothing stored is lost.
type SourceOnlyField struct {
	Path        string                 `json:"path"`
	Count       int                    `json:"count"`
	SHA256      string                 `json:"sha256"`
	Occurrences []SourceOnlyOccurrence `json:"occurrences"`
}

// SourceOnlyOccurrence is one stored field: the message that holds it, as a
// field-name path from the trajectory, and its payload bytes.
type SourceOnlyOccurrence struct {
	Location string `json:"location"`
	Payload  []byte `json:"payload"`
}

func sourceOnlyRecordField(name protoreflect.Name) bool {
	return name == "generator_metadata" || name == "executor_metadatas"
}

// declaredPlannerField reports whether a CascadePlannerConfig's unknown bytes
// are exactly one field 50, wire type 2, with an observed payload.
func declaredPlannerField(unknown []byte) bool {
	num, typ, n := protowire.ConsumeTag(unknown)
	if n < 0 || num != plannerSourceOnly || typ != protowire.BytesType {
		return false
	}
	payload, m := protowire.ConsumeBytes(unknown[n:])
	if m < 0 || n+m != len(unknown) {
		return false
	}
	last := protowire.Number(0)
	for len(payload) > 0 {
		num, typ, n := protowire.ConsumeTag(payload)
		if n != 1 || typ != protowire.VarintType || num != 2 && num != 4 || num <= last {
			return false
		}
		payload = payload[n:]
		if len(payload) == 0 || payload[0] != 0 {
			return false
		}
		payload = payload[1:]
		last = num
	}
	return true
}

// stripSourceOnly removes the declared field from a normalized trajectory and
// records each occurrence in a fixed traversal order. normalize has already
// rejected the field anywhere else, so only these records are visited.
func stripSourceOnly(root protoreflect.Message) []SourceOnlyField {
	var occurrences []SourceOnlyOccurrence
	var encoded []byte
	fields := root.Descriptor().Fields()
	for _, name := range []protoreflect.Name{"generator_metadata", "executor_metadatas"} {
		list := root.Get(fields.ByName(name)).List()
		for i := 0; i < list.Len(); i++ {
			collectSourceOnly(list.Get(i).Message(), fmt.Sprintf("%s[%d]", name, i), &occurrences, &encoded)
		}
	}
	if len(occurrences) == 0 {
		return nil
	}
	sum := sha256.Sum256(encoded)
	return []SourceOnlyField{{Path: SourceOnlyPlannerPath, Count: len(occurrences), SHA256: hex.EncodeToString(sum[:]), Occurrences: occurrences}}
}

func collectSourceOnly(m protoreflect.Message, location string, out *[]SourceOnlyOccurrence, encoded *[]byte) {
	if raw := m.GetUnknown(); len(raw) > 0 && m.Descriptor().FullName() == plannerConfigName {
		_, _, n := protowire.ConsumeTag(raw)
		payload, _ := protowire.ConsumeBytes(raw[n:])
		*out = append(*out, SourceOnlyOccurrence{Location: location, Payload: append([]byte{}, payload...)})
		*encoded = append(*encoded, raw...)
		m.SetUnknown(nil)
	}
	fields := m.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		if !m.Has(f) {
			continue
		}
		at := location + "." + string(f.Name())
		switch {
		case f.IsMap():
			if f.MapValue().Message() == nil {
				continue
			}
			entries := m.Get(f).Map()
			var keys []protoreflect.MapKey
			entries.Range(func(k protoreflect.MapKey, _ protoreflect.Value) bool {
				keys = append(keys, k)
				return true
			})
			sort.Slice(keys, func(a, b int) bool { return keys[a].String() < keys[b].String() })
			for _, k := range keys {
				collectSourceOnly(entries.Get(k).Message(), fmt.Sprintf("%s[%q]", at, k.String()), out, encoded)
			}
		case f.IsList():
			if f.Message() == nil {
				continue
			}
			list := m.Get(f).List()
			for j := 0; j < list.Len(); j++ {
				collectSourceOnly(list.Get(j).Message(), fmt.Sprintf("%s[%d]", at, j), out, encoded)
			}
		case f.Message() != nil:
			collectSourceOnly(m.Get(f).Message(), at, out, encoded)
		}
	}
}
