package completeness

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Historical sources carry two fields that the current descriptor reserves and
// the daemon's RPC never sends, so full-raw equality cannot cover them. The
// profile declares each source-only by number and wire type alone: no name or
// type is assigned, because no authentic definition for the stored bytes is
// known.
//
// CascadePlannerConfig field 50 (wire type 2) sits inside generator_metadata
// and executor_metadatas records. Its payload is limited to the shapes observed
// in those sources: empty, or varint fields 2 and/or 4, each once, in that
// order, and set to zero.
//
// ChatMessagePrompt field 23 (wire type 2) sits on a tool-result prompt in a
// generator_metadata record's chat model. Its payload is a
// google.protobuf.Struct that restates the results of the grep_search step the
// prompt answers (see prompt23.go); it is accepted only when every value in it
// is derived from that step.
const (
	trajectoryName        protoreflect.FullName = "gemini_coder.Trajectory"
	generatorMetadataName protoreflect.FullName = "exa.cortex_pb.CortexStepGeneratorMetadata"
	executorMetadataName  protoreflect.FullName = "exa.cortex_pb.ExecutorMetadata"
	plannerConfigName     protoreflect.FullName = "exa.cortex_pb.CascadePlannerConfig"
	plannerSourceOnly     protowire.Number      = 50
	promptName            protoreflect.FullName = "exa.chat_pb.ChatMessagePrompt"
	promptSourceOnly      protowire.Number      = 23

	// SourceOnlyPlannerPath names the declared field-50 locations in an
	// attestation: the field-name paths of the planner entries in
	// sourceOnlyLocations below, from the trajectory.
	SourceOnlyPlannerPath = "generator_metadata.planner_config|executor_metadatas.cascade_config.planner_config/exa.cortex_pb.CascadePlannerConfig#50/wire2"
	// SourceOnlyPromptPath names the declared field-23 location the same way.
	SourceOnlyPromptPath = "generator_metadata.chat_model.message_prompts/exa.chat_pb.ChatMessagePrompt#23/wire2"
	// SourceOnlyReason marks a verified result that relies on declared
	// source-only content. Each output that reports a verification result
	// shows it apart from plain verified; it is as fresh and as qualified as a
	// plain verified one.
	SourceOnlyReason = "verified except declared source-only fields absent from RPC"
)

// errUnknownFields is the refusal for any unknown persisted field, including a
// declared field whose location, shape or values fall outside its declaration.
var errUnknownFields = errors.New("unknown persisted protobuf fields")

// sourceOnlyLocation is one declared location: a field-name path from a record
// and the message type at its end that may hold the declared field.
type sourceOnlyLocation struct {
	path    string
	message protoreflect.FullName
}

// sourceOnlyLocations lists, per record type, the only locations that may carry
// a declared field: exactly the ones observed in the retained historical
// sources. Every field-50 generator occurrence was at planner_config and every
// executor occurrence at cascade_config.planner_config; none was at
// ExecutorMetadata.request_override_config.planner_config, which stays unknown.
// The descriptor nests planners recursively (agent definitions in a planner's
// customization settings carry their own cascade and planner configuration, and
// ExecutorMetadata.agent_config_resolution holds more), so "any planner inside
// the record" would be an unbounded set of locations. Every field-23 occurrence
// was on an element of the generator record's chat_model.message_prompts.
// Nested planners and prompts, steps, trajectories and embedded Any payloads
// never carry either field.
var sourceOnlyLocations = map[protoreflect.FullName][]sourceOnlyLocation{
	generatorMetadataName: {{"planner_config", plannerConfigName}, {"chat_model.message_prompts", promptName}},
	executorMetadataName:  {{"cascade_config.planner_config", plannerConfigName}},
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
	for _, l := range sourceOnlyLocations[s.record] {
		if l.path == next || strings.HasPrefix(l.path, next+".") {
			return sourceOnlyScope{s.record, next}
		}
	}
	return sourceOnlyScope{}
}

// declared returns the message type the scope's declared location may hold,
// or "" when the scope is not exactly a declared location.
func (s sourceOnlyScope) declared() protoreflect.FullName {
	for _, l := range sourceOnlyLocations[s.record] {
		if l.path == s.path {
			return l.message
		}
	}
	return ""
}

// declaredUnknown reports whether m's unknown bytes are a declared field in
// an accepted shape at this scope. A field-23 prompt is checked here on its
// own; the comparison with the step it names happens in stripSourceOnly, once
// the whole trajectory is available.
func declaredUnknown(scope sourceOnlyScope, m protoreflect.Message, unknown []byte) bool {
	name := m.Descriptor().FullName()
	if scope.declared() != name {
		return false
	}
	switch name {
	case plannerConfigName:
		return declaredPlannerField(unknown)
	case promptName:
		_, ok := declaredPromptField(m, unknown)
		return ok
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

// stripSourceOnly removes the declared fields from a normalized trajectory and
// records each occurrence in a fixed traversal order, one SourceOnlyField per
// declared field that occurs: field 50 first, then field 23. normalize has
// already rejected either field anywhere else, so only these records are
// visited. A field-23 payload must also be derived from the step its prompt
// names; one that is not makes the source unknown.
func stripSourceOnly(root protoreflect.Message) ([]SourceOnlyField, error) {
	c := sourceOnlyCollector{steps: root.Get(root.Descriptor().Fields().ByName("steps")).List()}
	fields := root.Descriptor().Fields()
	for _, name := range []protoreflect.Name{"generator_metadata", "executor_metadatas"} {
		list := root.Get(fields.ByName(name)).List()
		for i := 0; i < list.Len(); i++ {
			c.collect(list.Get(i).Message(), fmt.Sprintf("%s[%d]", name, i))
		}
	}
	if c.err != nil {
		return nil, c.err
	}
	var out []SourceOnlyField
	for _, f := range []struct {
		path string
		occ  []SourceOnlyOccurrence
		enc  []byte
	}{{SourceOnlyPlannerPath, c.planner, c.plannerEncoded}, {SourceOnlyPromptPath, c.prompt, c.promptEncoded}} {
		if len(f.occ) == 0 {
			continue
		}
		sum := sha256.Sum256(f.enc)
		out = append(out, SourceOnlyField{Path: f.path, Count: len(f.occ), SHA256: hex.EncodeToString(sum[:]), Occurrences: f.occ})
	}
	return out, nil
}

type sourceOnlyCollector struct {
	steps                         protoreflect.List
	planner, prompt               []SourceOnlyOccurrence
	plannerEncoded, promptEncoded []byte
	err                           error
}

func (c *sourceOnlyCollector) collect(m protoreflect.Message, location string) {
	if c.err != nil {
		return
	}
	if raw := m.GetUnknown(); len(raw) > 0 {
		_, _, n := protowire.ConsumeTag(raw)
		payload, _ := protowire.ConsumeBytes(raw[n:])
		occurrence := SourceOnlyOccurrence{Location: location, Payload: append([]byte{}, payload...)}
		switch m.Descriptor().FullName() {
		case plannerConfigName:
			c.planner = append(c.planner, occurrence)
			c.plannerEncoded = append(c.plannerEncoded, raw...)
		case promptName:
			if !derivedFromStep(m, raw, c.steps) {
				c.err = errUnknownFields
				return
			}
			c.prompt = append(c.prompt, occurrence)
			c.promptEncoded = append(c.promptEncoded, raw...)
		default:
			c.err = errUnknownFields
			return
		}
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
				c.collect(entries.Get(k).Message(), fmt.Sprintf("%s[%q]", at, k.String()))
			}
		case f.IsList():
			if f.Message() == nil {
				continue
			}
			list := m.Get(f).List()
			for j := 0; j < list.Len(); j++ {
				c.collect(list.Get(j).Message(), fmt.Sprintf("%s[%d]", at, j))
			}
		case f.Message() != nil:
			c.collect(m.Get(f).Message(), at)
		}
	}
}
