package completeness

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Historical sources store CascadePlannerConfig field 50 (wire type 2) in
// generator and executor metadata. The daemon's RPC omits it. These fixtures
// are synthetic: the planner settings are placeholders, and the field-50
// payloads are the observed shapes (empty, or nested varint fields 2 and/or 4
// set to zero) plus near misses that must fail.
const sourceOnlyGen = `{"plannerConfig":{"maxOutputTokens":1024}}`
const sourceOnlyCascade = `{"cascadeConfig":{"plannerConfig":{"maxOutputTokens":1024}}}`
const sourceOnlyOverride = `{"requestOverrideConfig":{"plannerConfig":{"maxOutputTokens":1024}}}`
const sourceOnlyRaw = `{"cascadeId":"planner-fixture","metadata":{},"generatorMetadata":[` + sourceOnlyGen + `],"executorMetadatas":[` + sourceOnlyCascade + `,` + sourceOnlyOverride + `],"steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","runCommand":{"commandLine":"echo fixture"}}]}`
const sourceOnlyUserInputRaw = `{"cascadeId":"planner-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userConfig":{"plannerConfig":{"maxOutputTokens":1024}}}}]}`

const genQuery = "UPDATE gen_metadata SET data=?1,size=length(?1) WHERE idx=0"

func plannerField(payload ...byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(nil, 50, protowire.BytesType), payload)
}

func bytesField(num protowire.Number, payload ...byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(nil, num, protowire.BytesType), payload)
}

// sourceOnlyEdit rewrites one stored record: the record is built from js and
// unknown is appended to the message at path (dot-separated field names, a
// trailing [0] selecting a list's first element; empty is the record itself).
type sourceOnlyEdit struct {
	raw     string
	query   string
	name    protoreflect.FullName
	js      string
	path    string
	unknown []byte
}

func genEdit(unknown []byte) sourceOnlyEdit {
	return sourceOnlyEdit{"", genQuery, generatorMetadataName, sourceOnlyGen, "planner_config", unknown}
}

func cascadeEdit(unknown []byte) sourceOnlyEdit {
	return sourceOnlyEdit{"", "UPDATE executor_metadata SET data=?1 WHERE idx=0", executorMetadataName, sourceOnlyCascade, "cascade_config.planner_config", unknown}
}

func overrideEdit(unknown []byte) sourceOnlyEdit {
	return sourceOnlyEdit{"", "UPDATE executor_metadata SET data=?1 WHERE idx=1", executorMetadataName, sourceOnlyOverride, "request_override_config.planner_config", unknown}
}

func (e sourceOnlyEdit) record(t *testing.T, unknown []byte) []byte {
	t.Helper()
	m, err := newMessage(e.name)
	if err != nil {
		t.Fatal(err)
	}
	if err = (protojson.UnmarshalOptions{Resolver: schemaTypes}).Unmarshal([]byte(e.js), m); err != nil {
		t.Fatal(err)
	}
	current := m.ProtoReflect()
	if e.path != "" {
		for _, name := range strings.Split(e.path, ".") {
			first := strings.HasSuffix(name, "[0]")
			fd := current.Descriptor().Fields().ByName(protoreflect.Name(strings.TrimSuffix(name, "[0]")))
			if fd == nil {
				t.Fatalf("invalid fixture path %q", e.path)
			}
			if first {
				current = current.Mutable(fd).List().Get(0).Message()
			} else {
				current = current.Mutable(fd).Message()
			}
		}
	}
	current.SetUnknown(append(append([]byte(nil), current.GetUnknown()...), unknown...))
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte{}, b...)
}

func sourceOnlyRead(t *testing.T, raw string, edits ...sourceOnlyEdit) (*Snapshot, error) {
	t.Helper()
	if raw == "" {
		raw = sourceOnlyRaw
	}
	p := filepath.Join(t.TempDir(), "source.db")
	db := testfixture.Source(t, p, []byte(raw))
	for _, e := range edits {
		testfixture.Exec(t, db, e.query, e.record(t, e.unknown))
	}
	return ReadSnapshot(t.Context(), p)
}

func sourceOnlySnapshot(t *testing.T, edits ...sourceOnlyEdit) *Snapshot {
	t.Helper()
	s, err := sourceOnlyRead(t, "", edits...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func withReaderAttestation(t *testing.T, raw string, a *Attestation) []byte {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &top); err != nil {
		t.Fatal(err)
	}
	reader, err := json.Marshal(map[string]*Attestation{"completeness": a})
	if err != nil {
		t.Fatal(err)
	}
	top["agyReader"] = reader
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func copyAttestation(t *testing.T, a *Attestation) *Attestation {
	t.Helper()
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var out Attestation
	if err = json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return &out
}

func TestSourceOnlyPlannerFieldAccepted(t *testing.T) {
	plain := sourceOnlySnapshot(t)
	pa, r := plain.VerifyResponses([]byte(sourceOnlyRaw), []byte(sourceOnlyRaw))
	if r.State != Verified || r.Reason != "" || pa.Reason != "" || pa.SourceOnly != nil {
		t.Fatalf("source without the field: %+v %+v", r, pa)
	}
	if r = plain.CheckAttestation([]byte(sourceOnlyRaw), pa); r.State != Verified || r.Reason != "" {
		t.Fatalf("plain recheck: %+v", r)
	}
	locations := []string{"generator_metadata[0].planner_config", "executor_metadatas[0].cascade_config.planner_config"}
	for name, payload := range map[string][]byte{
		"empty":          {},
		"field 2":        {0x10, 0x00},
		"fields 2 and 4": {0x10, 0x00, 0x20, 0x00},
		"field 4":        {0x20, 0x00},
	} {
		t.Run(name, func(t *testing.T) {
			field := plannerField(payload...)
			s := sourceOnlySnapshot(t, genEdit(field), cascadeEdit(field))
			if s.Digest == plain.Digest {
				t.Fatal("source digest ignores the source-only bytes")
			}
			a, r := s.VerifyResponses([]byte(sourceOnlyRaw), []byte(sourceOnlyRaw))
			if r.State != Verified || r.Reason != SourceOnlyReason {
				t.Fatalf("got %+v", r)
			}
			sum := sha256.Sum256(bytes.Repeat(field, 2))
			if a.Profile != Profile || a.Reason != SourceOnlyReason || len(a.SourceOnly) != 1 {
				t.Fatalf("attestation %+v", a)
			}
			got := a.SourceOnly[0]
			if got.Path != SourceOnlyPlannerPath || got.Count != 2 || got.SHA256 != hex.EncodeToString(sum[:]) || len(got.Occurrences) != 2 {
				t.Fatalf("source-only record %+v", got)
			}
			for i, o := range got.Occurrences {
				if o.Location != locations[i] || !bytes.Equal(o.Payload, payload) {
					t.Fatalf("occurrence %d: %+v", i, o)
				}
			}
			// The sidecar keeps the attestation, and so the payloads, under
			// agyReader; reading it back still verifies, never plainly.
			sidecar := withReaderAttestation(t, sourceOnlyRaw, a)
			read := ReadAttestation(sidecar)
			if read == nil || len(read.SourceOnly) != 1 || !bytes.Equal(read.SourceOnly[0].Occurrences[1].Payload, payload) {
				t.Fatalf("sidecar attestation %+v", read)
			}
			if r := s.CheckAttestation(sidecar, read); r.State != Verified || r.Reason != SourceOnlyReason {
				t.Fatalf("recheck %+v", r)
			}
		})
	}
	t.Run("generator only", func(t *testing.T) {
		s := sourceOnlySnapshot(t, genEdit(plannerField(0x10, 0x00)))
		a, r := s.VerifyResponses([]byte(sourceOnlyRaw), []byte(sourceOnlyRaw))
		if r.State != Verified || r.Reason != SourceOnlyReason || a.SourceOnly[0].Count != 1 || a.SourceOnly[0].Occurrences[0].Location != locations[0] {
			t.Fatalf("got %+v %+v", r, a)
		}
	})
	t.Run("executor only", func(t *testing.T) {
		s := sourceOnlySnapshot(t, cascadeEdit(plannerField()))
		a, r := s.VerifyResponses([]byte(sourceOnlyRaw), []byte(sourceOnlyRaw))
		if r.State != Verified || r.Reason != SourceOnlyReason || a.SourceOnly[0].Count != 1 || a.SourceOnly[0].Occurrences[0].Location != locations[1] {
			t.Fatalf("got %+v %+v", r, a)
		}
	})
}

// Each case must read normally without its unknown bytes and fail with them,
// so the failure comes from the field alone.
func TestSourceOnlyPlannerFieldRejected(t *testing.T) {
	root := func(e sourceOnlyEdit) sourceOnlyEdit { e.path = ""; return e }
	at := func(e sourceOnlyEdit, path string) sourceOnlyEdit { e.path = path; return e }
	for name, e := range map[string]sourceOnlyEdit{
		// field number and wire type
		"varint wire type":  genEdit(protowire.AppendVarint(protowire.AppendTag(nil, 50, protowire.VarintType), 0)),
		"fixed32 wire type": genEdit(protowire.AppendFixed32(protowire.AppendTag(nil, 50, protowire.Fixed32Type), 0)),
		"fixed64 wire type": genEdit(protowire.AppendFixed64(protowire.AppendTag(nil, 50, protowire.Fixed64Type), 0)),
		"field 49":          genEdit(bytesField(49)),
		"field 51":          genEdit(bytesField(51)),
		"field 52":          genEdit(bytesField(52)),
		"field 23":          genEdit(bytesField(23)),
		"field 50 twice":    genEdit(append(plannerField(), plannerField()...)),
		"another unknown":   genEdit(append(plannerField(), protowire.AppendVarint(protowire.AppendTag(nil, 99999, protowire.VarintType), 1)...)),
		"executor field 51": cascadeEdit(bytesField(51, 0x10, 0x00)),
		// nested content
		"nested field 1":          genEdit(plannerField(0x08, 0x00)),
		"nested field 3":          genEdit(plannerField(0x18, 0x00)),
		"nested field 5":          genEdit(plannerField(0x28, 0x00)),
		"nested field 2 set":      genEdit(plannerField(0x10, 0x01)),
		"nested field 4 set":      genEdit(plannerField(0x10, 0x00, 0x20, 0x01)),
		"nested field 2 twice":    genEdit(plannerField(0x10, 0x00, 0x10, 0x00)),
		"nested fields reversed":  genEdit(plannerField(0x20, 0x00, 0x10, 0x00)),
		"nested field 2 bytes":    genEdit(plannerField(0x12, 0x00)),
		"nested field 4 fixed32":  genEdit(plannerField(0x25, 0, 0, 0, 0)),
		"nested long zero":        genEdit(plannerField(0x10, 0x80, 0x00)),
		"nested long tag":         genEdit(plannerField(0x90, 0x00, 0x00)),
		"nested truncated":        genEdit(plannerField(0x10)),
		"nested extra field":      genEdit(plannerField(0x10, 0x00, 0x20, 0x00, 0x28, 0x00)),
		"executor nested field 3": cascadeEdit(plannerField(0x18, 0x00)),
		"override nested set":     overrideEdit(plannerField(0x20, 0x07)),
		// location: request_override_config.planner_config was never observed
		// carrying the field, so even an observed payload shape fails there.
		"override planner config":         overrideEdit(plannerField()),
		"override planner config field 2": overrideEdit(plannerField(0x10, 0x00)),
		"generator record":                root(genEdit(plannerField())),
		"executor record":                 root(cascadeEdit(plannerField())),
		"executor cascade config":         at(cascadeEdit(plannerField()), "cascade_config"),
		"trajectory metadata planner": {"", "UPDATE trajectory_metadata_blob SET data=?1 WHERE id='main'", "exa.cortex_pb.CortexTrajectoryMetadata",
			`{"staticConfig":{"cascadeConfig":{"plannerConfig":{"maxOutputTokens":1024}}}}`, "static_config.cascade_config.planner_config", plannerField()},
		"trajectory metadata": {"", "UPDATE trajectory_metadata_blob SET data=?1 WHERE id='main'", "exa.cortex_pb.CortexTrajectoryMetadata", `{}`, "", plannerField()},
		"parent reference":    {"", "INSERT INTO parent_references VALUES(0,?1)", "exa.cortex_pb.CortexTrajectoryReference", `{}`, "", plannerField()},
		"battle mode info":    {"", "INSERT INTO battle_mode_infos VALUES(0,?1)", "exa.cortex_pb.BattleModeInfo", `{}`, "", plannerField()},
		"step planner config": {sourceOnlyUserInputRaw, "UPDATE steps SET step_payload=?1 WHERE idx=0", "gemini_coder.Step",
			`{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userConfig":{"plannerConfig":{"maxOutputTokens":1024}}}}`, "user_input.user_config.planner_config", plannerField()},
		// ChatMessagePrompt field 23 on a prompt that is not a tool result
		// stays unknown, with or without field 50 (prompt23_test.go covers
		// the declared case).
		"chat message prompt field 23": {"", genQuery, generatorMetadataName, `{"chatModel":{"messagePrompts":[{"prompt":"fixture"}]}}`, "chat_model.message_prompts[0]", bytesField(23, 0x0a, 0x00)},
	} {
		t.Run(name, func(t *testing.T) {
			clean := e
			clean.unknown = nil
			if _, err := sourceOnlyRead(t, e.raw, clean); err != nil {
				t.Fatalf("fixture without the field: %v", err)
			}
			if _, err := sourceOnlyRead(t, e.raw, e); err == nil || err.Error() != "unknown persisted protobuf fields" {
				t.Fatalf("got %v, want unknown persisted protobuf fields", err)
			}
		})
	}
	t.Run("field 23 beside an accepted field 50", func(t *testing.T) {
		prompt := sourceOnlyEdit{"", genQuery, generatorMetadataName, `{"plannerConfig":{"maxOutputTokens":1024},"chatModel":{"messagePrompts":[{"prompt":"fixture"}]}}`, "chat_model.message_prompts[0]", bytesField(23)}
		if _, err := sourceOnlyRead(t, "", prompt, cascadeEdit(plannerField())); err == nil {
			t.Fatal("field 23 accepted")
		}
	})
	t.Run("field 50 inside an embedded Any", func(t *testing.T) {
		s := sourceOnlySnapshot(t, genEdit(plannerField()))
		inner, err := newMessage(executorMetadataName)
		if err != nil {
			t.Fatal(err)
		}
		if err = (protojson.UnmarshalOptions{Resolver: schemaTypes}).Unmarshal([]byte(sourceOnlyCascade), inner); err != nil {
			t.Fatal(err)
		}
		pc := inner.ProtoReflect().Mutable(inner.ProtoReflect().Descriptor().Fields().ByName("cascade_config")).Message()
		pc = pc.Mutable(pc.Descriptor().Fields().ByName("planner_config")).Message()
		pc.SetUnknown(plannerField())
		value, err := proto.Marshal(inner)
		if err != nil {
			t.Fatal(err)
		}
		m := proto.Clone(s.trajectory).ProtoReflect()
		step := m.Mutable(m.Descriptor().Fields().ByName("steps")).List().Get(0).Message()
		g := step.Mutable(step.Descriptor().Fields().ByName("generic")).Message()
		r := g.Mutable(g.Descriptor().Fields().ByName("result")).Message()
		a := r.Mutable(r.Descriptor().Fields().ByName("payload")).Message()
		a.Set(a.Descriptor().Fields().ByName("type_url"), protoreflect.ValueOfString("type.googleapis.com/"+string(executorMetadataName)))
		a.Set(a.Descriptor().Fields().ByName("value"), protoreflect.ValueOfBytes(value))
		if _, err := finishSnapshot(m.Interface()); err == nil || err.Error() != "unknown persisted protobuf fields" {
			t.Fatalf("got %v, want unknown persisted protobuf fields", err)
		}
		pc.SetUnknown(nil)
		if value, err = proto.Marshal(inner); err != nil {
			t.Fatal(err)
		}
		a.Set(a.Descriptor().Fields().ByName("value"), protoreflect.ValueOfBytes(value))
		if _, err := finishSnapshot(m.Interface()); err != nil {
			t.Fatalf("fixture without the field: %v", err)
		}
	})
}

func TestSourceOnlyPlannerFieldRecheck(t *testing.T) {
	raw := []byte(sourceOnlyRaw)
	s := sourceOnlySnapshot(t, genEdit(plannerField(0x10, 0x00, 0x20, 0x00)), cascadeEdit(plannerField(0x10, 0x00)))
	a, r := s.VerifyResponses(raw, raw)
	if r.State != Verified {
		t.Fatal(r)
	}
	// The source-only bytes changed or removed after attestation.
	for name, edits := range map[string][]sourceOnlyEdit{
		"payload changed": {genEdit(plannerField(0x10, 0x00)), cascadeEdit(plannerField(0x10, 0x00))},
		"one removed":     {cascadeEdit(plannerField(0x10, 0x00))},
		"all removed":     nil,
	} {
		t.Run(name, func(t *testing.T) {
			if r := sourceOnlySnapshot(t, edits...).CheckAttestation(raw, a); r.State != Incomplete || r.Reason != "persisted source content changed" {
				t.Fatalf("got %+v", r)
			}
		})
	}
	// The sidecar's record no longer matches the source.
	for name, change := range map[string]func(*Attestation){
		"reason dropped":   func(a *Attestation) { a.Reason = "" },
		"record dropped":   func(a *Attestation) { a.SourceOnly = nil },
		"count changed":    func(a *Attestation) { a.SourceOnly[0].Count = 1 },
		"digest changed":   func(a *Attestation) { a.SourceOnly[0].SHA256 = strings.Repeat("0", 64) },
		"path changed":     func(a *Attestation) { a.SourceOnly[0].Path = "elsewhere" },
		"payload changed":  func(a *Attestation) { a.SourceOnly[0].Occurrences[0].Payload = []byte{0x10, 0x00} },
		"location changed": func(a *Attestation) { a.SourceOnly[0].Occurrences[1].Location = "executor_metadatas[1]" },
		"occurrence lost":  func(a *Attestation) { a.SourceOnly[0].Occurrences = a.SourceOnly[0].Occurrences[:1] },
	} {
		t.Run(name, func(t *testing.T) {
			changed := copyAttestation(t, a)
			change(changed)
			if r := s.CheckAttestation(raw, changed); r.State != Incomplete || r.Reason != "sidecar source-only record changed since verification" {
				t.Fatalf("got %+v", r)
			}
		})
	}
	t.Run("plain attestation claims source-only content", func(t *testing.T) {
		plain := sourceOnlySnapshot(t)
		pa, _ := plain.VerifyResponses(raw, raw)
		pa.Reason, pa.SourceOnly = a.Reason, a.SourceOnly
		if r := plain.CheckAttestation(raw, pa); r.State != Incomplete {
			t.Fatalf("got %+v", r)
		}
	})
	for _, profile := range []string{"sqlite-v1-format0-agy-a759ce7c7a23", "sqlite-v1-format0-agy-a759ce7c7a23-planner50", "sqlite-v1-format0-agy-1.3.0-19be6af38f7b-planner50", "sqlite-v1-format0-agy-1.3.0-19be6af38f7b-planner50-permission1", "sqlite-v1-format0-agy-1.3.0-19be6af38f7b-planner50-permission2", "sqlite-v1-format0-agy-1.3.0-19be6af38f7b-planner50-permission2-prompt23"} {
		t.Run("previous profile "+profile, func(t *testing.T) {
			old := copyAttestation(t, a)
			old.Profile = profile
			if r := s.CheckAttestation(raw, old); r.State != Unknown {
				t.Fatalf("got %+v", r)
			}
		})
	}
	// Removing the field from the comparison copy hides nothing else.
	for name, changed := range map[string][]byte{
		"generator planner setting": bytes.Replace(raw, []byte(`"maxOutputTokens":1024`), []byte(`"maxOutputTokens":2048`), 1),
		"executor planner setting":  bytes.Replace(raw, []byte(`{"cascadeConfig":{"plannerConfig":{"maxOutputTokens":1024}`), []byte(`{"cascadeConfig":{"plannerConfig":{"maxOutputTokens":2048}`), 1),
		"planner setting added":     bytes.Replace(raw, []byte(`"maxOutputTokens":1024`), []byte(`"maxOutputTokens":1024,"noToolSummary":true`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(changed, raw) {
				t.Fatal("invalid fixture mutation")
			}
			if _, r := s.VerifyResponses(changed, raw); r.State != Incomplete {
				t.Fatalf("raw: got %+v", r)
			}
			if _, r := s.VerifyResponses(raw, changed); r.State != Incomplete {
				t.Fatalf("presentation: got %+v", r)
			}
		})
	}
}

// plannerPosition is one place a record can hold a CascadePlannerConfig: the
// field path from the record and the message type of each field on it.
type plannerPosition struct {
	fields []protoreflect.FieldDescriptor
	types  []protoreflect.FullName
}

func (p plannerPosition) String(record protoreflect.FullName) string {
	names := []string{string(record)}
	for _, f := range p.fields {
		names = append(names, string(f.Name()))
	}
	return strings.Join(names, ".")
}

// plannerPositions walks the descriptor from md to every CascadePlannerConfig
// it can hold. The descriptor is recursive: a planner's customization settings
// define agents that carry their own cascade and planner configuration. The
// walk therefore enters a message type at most limit times on one path. It
// never descends into google.protobuf.Any, which the allowance never reaches.
func plannerPositions(md protoreflect.MessageDescriptor, limit int) []plannerPosition {
	var out []plannerPosition
	onPath := map[protoreflect.FullName]int{md.FullName(): 1}
	var walk func(md protoreflect.MessageDescriptor, at plannerPosition)
	walk = func(md protoreflect.MessageDescriptor, at plannerPosition) {
		if md.FullName() == plannerConfigName {
			out = append(out, plannerPosition{append([]protoreflect.FieldDescriptor{}, at.fields...), append([]protoreflect.FullName{}, at.types...)})
		}
		if md.FullName() == "google.protobuf.Any" {
			return
		}
		fields := md.Fields()
		for i := 0; i < fields.Len(); i++ {
			f := fields.Get(i)
			next := f.Message()
			if f.IsMap() {
				next = f.MapValue().Message()
			}
			if next == nil || onPath[next.FullName()] >= limit {
				continue
			}
			onPath[next.FullName()]++
			walk(next, plannerPosition{append(at.fields, f), append(at.types, next.FullName())})
			onPath[next.FullName()]--
		}
	}
	walk(md, plannerPosition{})
	return out
}

// plannerRecord builds a record with the message path at populated and
// unknown stored on the CascadePlannerConfig at its end.
func plannerRecord(t *testing.T, record protoreflect.FullName, at plannerPosition, unknown []byte) proto.Message {
	t.Helper()
	m, err := newMessage(record)
	if err != nil {
		t.Fatal(err)
	}
	current := m.ProtoReflect()
	for _, f := range at.fields {
		switch {
		case f.IsMap():
			current = current.Mutable(f).Map().Mutable(f.MapKey().Default().MapKey()).Message()
		case f.IsList():
			current = current.Mutable(f).List().AppendMutable().Message()
		default:
			current = current.Mutable(f).Message()
		}
	}
	current.SetUnknown(unknown)
	return m
}

// acceptedAt decodes data as name and reports whether it was accepted. The
// only refusal expected here is the unknown field itself.
func acceptedAt(t *testing.T, name protoreflect.FullName, data []byte, where string) bool {
	t.Helper()
	_, err := decode(name, data)
	if err != nil && err.Error() != "unknown persisted protobuf fields" {
		t.Fatalf("%s: unexpected refusal %v", where, err)
	}
	return err == nil
}

// The allowance is pinned to an explicit list of locations. The walk places
// field 50 on every CascadePlannerConfig a generator or executor record can
// hold (up to two visits of each message type per path) and decodes the record
// both on its own, as its SQLite table row, and inside a trajectory.
func TestSourceOnlyPlannerFieldLocations(t *testing.T) {
	if err := loadSchema(); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"exa.cortex_pb.CortexStepGeneratorMetadata.planner_config",
		"exa.cortex_pb.ExecutorMetadata.cascade_config.planner_config",
	}
	// Neither a step nor a trajectory, nor any other message named as one, may
	// lie on an accepted path. Configuration messages that only mention steps or
	// trajectories in their names (StepStringConverterConfig, for example) are
	// not steps.
	stepOrTrajectory := func(name protoreflect.FullName) bool {
		short := string(name.Name())
		return name == "gemini_coder.Step" || name == "gemini_coder.Trajectory" || strings.HasSuffix(short, "Step") || strings.HasSuffix(short, "Trajectory")
	}
	trajectoryField := map[protoreflect.FullName]protoreflect.Name{generatorMetadataName: "generator_metadata", executorMetadataName: "executor_metadatas"}
	var accepted []string
	positions, deeper := 0, 0
	for _, record := range []protoreflect.FullName{generatorMetadataName, executorMetadataName} {
		d, err := schemaFiles.FindDescriptorByName(record)
		if err != nil {
			t.Fatal(err)
		}
		for _, at := range plannerPositions(d.(protoreflect.MessageDescriptor), 2) {
			positions++
			if len(at.fields) > 2 {
				deeper++
			}
			where := at.String(record)
			m := plannerRecord(t, record, at, plannerField())
			data, err := proto.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			alone := acceptedAt(t, record, data, where)
			traj, err := newMessage(trajectoryName)
			if err != nil {
				t.Fatal(err)
			}
			tm := traj.ProtoReflect()
			tm.Mutable(tm.Descriptor().Fields().ByName(trajectoryField[record])).List().Append(protoreflect.ValueOfMessage(m.ProtoReflect()))
			if data, err = proto.Marshal(traj); err != nil {
				t.Fatal(err)
			}
			if inside := acceptedAt(t, trajectoryName, data, where); inside != alone {
				t.Errorf("%s: accepted as a record %v, inside a trajectory %v", where, alone, inside)
			}
			if !alone {
				continue
			}
			accepted = append(accepted, where)
			for _, name := range at.types {
				if stepOrTrajectory(name) {
					t.Errorf("%s: accepted path passes through %s", where, name)
				}
			}
		}
	}
	sort.Strings(accepted)
	if strings.Join(accepted, "\n") != strings.Join(want, "\n") {
		t.Errorf("field 50 accepted at %d of %d positions:\n%s\nwant:\n%s", len(accepted), positions, strings.Join(accepted, "\n"), strings.Join(want, "\n"))
	}
	// The walk must reach well past the accepted paths: nested agent planners
	// and planners inside another planner's customization settings.
	if deeper < 100 {
		t.Fatalf("walk reached only %d positions deeper than two fields", deeper)
	}

	// No step or trajectory message is reachable from either record at all,
	// at any depth, except through an embedded Any (rejected separately).
	for _, record := range []protoreflect.FullName{generatorMetadataName, executorMetadataName} {
		d, _ := schemaFiles.FindDescriptorByName(record)
		seen := map[protoreflect.FullName]bool{}
		var visit func(md protoreflect.MessageDescriptor)
		visit = func(md protoreflect.MessageDescriptor) {
			if seen[md.FullName()] {
				return
			}
			seen[md.FullName()] = true
			fields := md.Fields()
			for i := 0; i < fields.Len(); i++ {
				f := fields.Get(i)
				next := f.Message()
				if f.IsMap() {
					next = f.MapValue().Message()
				}
				if next != nil {
					visit(next)
				}
			}
		}
		visit(d.(protoreflect.MessageDescriptor))
		for name := range seen {
			if name != generatorMetadataName && stepOrTrajectory(name) {
				t.Errorf("%s reaches %s", record, name)
			}
		}
	}
}
