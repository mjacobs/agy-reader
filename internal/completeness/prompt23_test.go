package completeness

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"
)

// Historical CLI sources store ChatMessagePrompt field 23 (wire type 2) on the
// tool-result prompt that answers a grep_search call. The daemon's RPC omits
// it. These fixtures are synthetic: three grep_search steps (lines, files only,
// and lines with more results than kept) and the payloads the observed build
// would have written for them, plus near misses that must fail.
const promptSteps = `[
 {"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":"find the needle"}},
 {"type":"CORTEX_STEP_TYPE_GREP_SEARCH","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-lines","name":"grep_search"}},
  "grepSearch":{"query":"needle","matchPerLine":true,"results":[{"relativePath":"a.go","absolutePath":"/w/a.go","lineNumber":4,"content":"needle one"},{"relativePath":"b.go","absolutePath":"/w/b.go","content":"needle two"}],"totalResults":2}},
 {"type":"CORTEX_STEP_TYPE_GREP_SEARCH","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-files","name":"grep_search"}},
  "grepSearch":{"query":"needle","results":[{"relativePath":"c.go","absolutePath":"/w/c.go"}],"totalResults":1}},
 {"type":"CORTEX_STEP_TYPE_GREP_SEARCH","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-elided","name":"grep_search"}},
  "grepSearch":{"query":"needle","matchPerLine":true,"results":[{"relativePath":"d.go","absolutePath":"/w/d.go","lineNumber":9,"content":"needle three"}],"totalResults":4}},
 {"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-command","name":"run_command"}},"runCommand":{"commandLine":"echo fixture"}}
]`

func promptJSON(call string, step int) string {
	return `{"source":"CHAT_MESSAGE_SOURCE_TOOL","prompt":"tool output","numTokens":3,"toolCallId":"` + call + `","stepIdx":` + string(rune('0'+step)) + `}`
}

var promptGen = `{"chatModel":{"messagePrompts":[{"source":"CHAT_MESSAGE_SOURCE_USER","prompt":"find the needle"},` +
	promptJSON("call-lines", 1) + `,` + promptJSON("call-files", 2) + `,` + promptJSON("call-elided", 3) + `,` + promptJSON("call-command", 4) + `]}}`

var promptRaw = `{"cascadeId":"prompt-fixture","metadata":{},"generatorMetadata":[` + promptGen + `],"steps":` + promptSteps + `}`

// The prompt index of each fixture prompt in promptGen.
const (
	promptUser = iota
	promptLines
	promptFiles
	promptElided
	promptCommand
)

func entry(path string, line ...any) any {
	if len(line) == 0 {
		return map[string]any{"path": path}
	}
	return map[string]any{"path": path, "line_content": line[0], "line_number": line[1]}
}

func structBytes(t *testing.T, v map[string]any) []byte {
	t.Helper()
	s, err := structpb.NewStruct(v)
	if err != nil {
		t.Fatal(err)
	}
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The payload the observed build would have written for each fixture step.
func linesPayload(t *testing.T) []byte {
	return structBytes(t, map[string]any{"results": []any{entry("/w/a.go", "needle one", 5), entry("b.go", "needle two", 1)}})
}

func filesPayload(t *testing.T) []byte {
	return structBytes(t, map[string]any{"results": []any{entry("c.go")}})
}

func elidedPayload(t *testing.T) []byte {
	return structBytes(t, map[string]any{"results": []any{entry("/w/d.go", "needle three", 10)}, "num_elided_results": 3})
}

func promptField(payload []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(nil, 23, protowire.BytesType), payload)
}

// promptRecord builds the generator record from js, with unknown bytes
// appended to the prompts at the given indices.
func promptRecord(t *testing.T, js string, unknown map[int][]byte) []byte {
	t.Helper()
	m, err := newMessage(generatorMetadataName)
	if err != nil {
		t.Fatal(err)
	}
	if err = (protojson.UnmarshalOptions{Resolver: schemaTypes}).Unmarshal([]byte(js), m); err != nil {
		t.Fatal(err)
	}
	cm := m.ProtoReflect().Mutable(m.ProtoReflect().Descriptor().Fields().ByName("chat_model")).Message()
	prompts := cm.Mutable(cm.Descriptor().Fields().ByName("message_prompts")).List()
	for i, u := range unknown {
		p := prompts.Get(i).Message()
		p.SetUnknown(append(append([]byte(nil), p.GetUnknown()...), u...))
	}
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte{}, b...)
}

// promptSource writes the fixture with the given generator record and
// returns its snapshot. raw replaces promptRaw when it is not empty.
func promptSource(t *testing.T, raw string, record []byte, more ...sourceOnlyEdit) (*Snapshot, error) {
	t.Helper()
	if raw == "" {
		raw = promptRaw
	}
	p := filepath.Join(t.TempDir(), "source.db")
	db := testfixture.Source(t, p, []byte(raw))
	if record != nil {
		testfixture.Exec(t, db, genQuery, record)
	}
	for _, e := range more {
		testfixture.Exec(t, db, e.query, e.record(t, e.unknown))
	}
	return ReadSnapshot(t.Context(), p)
}

func promptSnapshot(t *testing.T, unknown map[int][]byte) *Snapshot {
	t.Helper()
	s, err := promptSource(t, "", promptRecord(t, promptGen, unknown))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSourceOnlyPromptFieldAccepted(t *testing.T) {
	raw := []byte(promptRaw)
	plain := promptSnapshot(t, nil)
	if a, r := plain.VerifyResponses(raw, raw); r.State != Verified || r.Reason != "" || a.SourceOnly != nil {
		t.Fatalf("source without the field: %+v %+v", r, a)
	}
	all := map[int][]byte{promptLines: promptField(linesPayload(t)), promptFiles: promptField(filesPayload(t)), promptElided: promptField(elidedPayload(t))}
	for name, unknown := range map[string]map[int][]byte{
		"lines":                       {promptLines: all[promptLines]},
		"files only":                  {promptFiles: all[promptFiles]},
		"elided":                      {promptElided: all[promptElided]},
		"all three":                   all,
		"relative and absolute paths": {promptLines: promptField(structBytes(t, map[string]any{"results": []any{entry("a.go", "needle one", 5), entry("/w/b.go", "needle two", 1)}}))},
	} {
		t.Run(name, func(t *testing.T) {
			s := promptSnapshot(t, unknown)
			if s.Digest == plain.Digest {
				t.Fatal("source digest ignores the source-only bytes")
			}
			a, r := s.VerifyResponses(raw, raw)
			if r.State != Verified || r.Reason != SourceOnlyReason {
				t.Fatalf("got %+v", r)
			}
			if a.Profile != Profile || a.Reason != SourceOnlyReason || len(a.SourceOnly) != 1 {
				t.Fatalf("attestation %+v", a)
			}
			var indices []int
			for i := range unknown {
				indices = append(indices, i)
			}
			sort.Ints(indices)
			var encoded []byte
			for _, i := range indices {
				encoded = append(encoded, unknown[i]...)
			}
			sum := sha256.Sum256(encoded)
			got := a.SourceOnly[0]
			if got.Path != SourceOnlyPromptPath || got.Count != len(indices) || got.SHA256 != hex.EncodeToString(sum[:]) || len(got.Occurrences) != len(indices) {
				t.Fatalf("source-only record %+v", got)
			}
			for j, i := range indices {
				o := got.Occurrences[j]
				want := "generator_metadata[0].chat_model.message_prompts[" + string(rune('0'+i)) + "]"
				_, _, n := protowire.ConsumeTag(unknown[i])
				payload, _ := protowire.ConsumeBytes(unknown[i][n:])
				if o.Location != want || !bytes.Equal(o.Payload, payload) {
					t.Fatalf("occurrence %d: %+v, want location %s", j, o, want)
				}
			}
			sidecar := withReaderAttestation(t, promptRaw, a)
			read := ReadAttestation(sidecar)
			if r := s.CheckAttestation(sidecar, read); r.State != Verified || r.Reason != SourceOnlyReason {
				t.Fatalf("recheck %+v", r)
			}
		})
	}
	t.Run("beside field 50", func(t *testing.T) {
		js := `{"plannerConfig":{"maxOutputTokens":1024},` + promptGen[1:]
		raw := `{"cascadeId":"prompt-fixture","metadata":{},"generatorMetadata":[` + js + `],"steps":` + promptSteps + `}`
		record := promptRecord(t, js, map[int][]byte{promptLines: all[promptLines]})
		m, err := newMessage(generatorMetadataName)
		if err != nil {
			t.Fatal(err)
		}
		if err = proto.Unmarshal(record, m); err != nil {
			t.Fatal(err)
		}
		pc := m.ProtoReflect().Mutable(m.ProtoReflect().Descriptor().Fields().ByName("planner_config")).Message()
		pc.SetUnknown(plannerField(0x10, 0x00))
		if record, err = proto.Marshal(m); err != nil {
			t.Fatal(err)
		}
		s, err := promptSource(t, raw, record)
		if err != nil {
			t.Fatal(err)
		}
		a, r := s.VerifyResponses([]byte(raw), []byte(raw))
		if r.State != Verified || r.Reason != SourceOnlyReason || len(a.SourceOnly) != 2 {
			t.Fatalf("got %+v %+v", r, a)
		}
		if a.SourceOnly[0].Path != SourceOnlyPlannerPath || a.SourceOnly[0].Count != 1 || a.SourceOnly[1].Path != SourceOnlyPromptPath || a.SourceOnly[1].Count != 1 {
			t.Fatalf("source-only records %+v", a.SourceOnly)
		}
		if r := s.CheckAttestation(withReaderAttestation(t, raw, a), a); r.State != Verified || r.Reason != SourceOnlyReason {
			t.Fatalf("recheck %+v", r)
		}
	})
}

// mapEntry encodes one Struct.fields entry with extra bytes after its value.
func mapEntry(t *testing.T, key string, value *structpb.Value, extra []byte) []byte {
	t.Helper()
	v, err := proto.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	e := protowire.AppendString(protowire.AppendTag(nil, 1, protowire.BytesType), key)
	e = protowire.AppendBytes(protowire.AppendTag(e, 2, protowire.BytesType), v)
	e = append(e, extra...)
	return protowire.AppendBytes(protowire.AppendTag(nil, 1, protowire.BytesType), e)
}

// Each case must read normally without its unknown bytes and fail with them,
// so the failure comes from the field alone.
func TestSourceOnlyPromptFieldRejected(t *testing.T) {
	varint99 := protowire.AppendVarint(protowire.AppendTag(nil, 99, protowire.VarintType), 1)
	lines := func(results []any, extra map[string]any) []byte {
		v := map[string]any{"results": results}
		for k, x := range extra {
			v[k] = x
		}
		return promptField(structBytes(t, v))
	}
	good := []any{entry("/w/a.go", "needle one", 5), entry("b.go", "needle two", 1)}
	nested := func(edit func(s *structpb.Struct)) []byte {
		s, err := structpb.NewStruct(map[string]any{"results": good})
		if err != nil {
			t.Fatal(err)
		}
		edit(s)
		b, err := proto.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		return promptField(b)
	}
	resultsValue, err := structpb.NewValue(good)
	if err != nil {
		t.Fatal(err)
	}
	userPrompt := `{"chatModel":{"messagePrompts":[{"source":"CHAT_MESSAGE_SOURCE_USER","prompt":"find the needle"},`
	gen := func(prompt string) string { return userPrompt + prompt + `]}}` }
	type rejection struct {
		js      string // generator record; promptGen when empty
		index   int
		unknown []byte
	}
	for name, c := range map[string]rejection{
		// number, wire type, repetition
		"wire type 0":            {"", promptLines, protowire.AppendVarint(protowire.AppendTag(nil, 23, protowire.VarintType), 0)},
		"fixed64 wire type":      {"", promptLines, protowire.AppendFixed64(protowire.AppendTag(nil, 23, protowire.Fixed64Type), 0)},
		"field 28":               {"", promptLines, protowire.AppendBytes(protowire.AppendTag(nil, 28, protowire.BytesType), linesPayload(t))},
		"second occurrence":      {"", promptLines, append(promptField(linesPayload(t)), promptField(linesPayload(t))...)},
		"beside another unknown": {"", promptLines, append(promptField(linesPayload(t)), varint99...)},
		// the prompt
		"source user":          {gen(strings.Replace(promptJSON("call-lines", 1), "CHAT_MESSAGE_SOURCE_TOOL", "CHAT_MESSAGE_SOURCE_USER", 1)), 1, promptField(linesPayload(t))},
		"source unspecified":   {gen(strings.Replace(promptJSON("call-lines", 1), `"source":"CHAT_MESSAGE_SOURCE_TOOL",`, "", 1)), 1, promptField(linesPayload(t))},
		"missing step_idx":     {gen(strings.Replace(promptJSON("call-lines", 1), `,"stepIdx":1`, "", 1)), 1, promptField(linesPayload(t))},
		"missing tool_call_id": {gen(strings.Replace(promptJSON("call-lines", 1), `"toolCallId":"call-lines",`, "", 1)), 1, promptField(linesPayload(t))},
		"other tool_call_id":   {gen(promptJSON("call-files", 1)), 1, promptField(linesPayload(t))},
		"prompt_struct set":    {gen(strings.Replace(promptJSON("call-lines", 1), `"prompt":"tool output"`, `"promptStruct":{}`, 1)), 1, promptField(linesPayload(t))},
		// the referenced step
		"step not grep_search": {gen(promptJSON("call-command", 4)), 1, promptField(structBytes(t, map[string]any{"results": []any{entry("x")}}))},
		"step out of range":    {gen(promptJSON("call-lines", 9)), 1, promptField(linesPayload(t))},
		"step of another call": {gen(promptJSON("call-lines", 2)), 1, promptField(filesPayload(t))},
		// the payload is not a strict Struct
		"empty payload":      {"", promptLines, promptField(nil)},
		"not a struct":       {"", promptLines, promptField([]byte{0xff, 0xff})},
		"unknown top field":  {"", promptLines, promptField(append(linesPayload(t), varint99...))},
		"unknown in a value": {"", promptLines, nested(func(s *structpb.Struct) { s.Fields["results"].ProtoReflect().SetUnknown(varint99) })},
		"unknown in a list":  {"", promptLines, nested(func(s *structpb.Struct) { s.Fields["results"].GetListValue().ProtoReflect().SetUnknown(varint99) })},
		"unknown in an entry": {"", promptLines, nested(func(s *structpb.Struct) {
			s.Fields["results"].GetListValue().Values[0].GetStructValue().ProtoReflect().SetUnknown(varint99)
		})},
		"unknown in a map entry": {"", promptLines, promptField(mapEntry(t, "results", resultsValue, varint99))},
		"key repeated":           {"", promptLines, promptField(append(mapEntry(t, "results", resultsValue, nil), mapEntry(t, "results", resultsValue, nil)...))},
		// the shape
		"no results":             {"", promptLines, promptField(structBytes(t, map[string]any{}))},
		"empty results":          {"", promptLines, lines([]any{}, nil)},
		"results not a list":     {"", promptLines, promptField(structBytes(t, map[string]any{"results": "a.go"}))},
		"extra top-level key":    {"", promptLines, lines(good, map[string]any{"query": "needle"})},
		"extra entry key":        {"", promptLines, lines([]any{map[string]any{"path": "/w/a.go", "line_content": "needle one", "line_number": 5, "column": 1}, good[1]}, nil)},
		"entry without number":   {"", promptLines, lines([]any{map[string]any{"path": "/w/a.go", "line_content": "needle one"}, good[1]}, nil)},
		"entry without path":     {"", promptLines, lines([]any{map[string]any{"line_content": "needle one", "line_number": 5}, good[1]}, nil)},
		"entry not an object":    {"", promptLines, lines([]any{"/w/a.go", good[1]}, nil)},
		"number as a string":     {"", promptLines, lines([]any{entry("/w/a.go", "needle one", "5"), good[1]}, nil)},
		"content as a number":    {"", promptLines, lines([]any{entry("/w/a.go", 1, 5), good[1]}, nil)},
		"path-only in line mode": {"", promptLines, lines([]any{entry("/w/a.go"), entry("b.go")}, nil)},
		"mixed entries":          {"", promptLines, lines([]any{good[0], entry("b.go")}, nil)},
		"lines in files mode":    {"", promptFiles, lines([]any{entry("c.go", "", 1)}, nil)},
		"fewer entries":          {"", promptLines, lines(good[:1], nil)},
		"more entries":           {"", promptLines, lines(append(append([]any{}, good...), entry("b.go", "needle two", 1)), nil)},
		"entries reordered":      {"", promptLines, lines([]any{good[1], good[0]}, nil)},
		"elided count as string": {"", promptElided, lines([]any{entry("/w/d.go", "needle three", 10)}, map[string]any{"num_elided_results": "3"})},
		// the values
		"path missing from step":  {"", promptLines, lines([]any{entry("/w/z.go", "needle one", 5), good[1]}, nil)},
		"content missing":         {"", promptLines, lines([]any{entry("/w/a.go", "needle eleven", 5), good[1]}, nil)},
		"line number not +1":      {"", promptLines, lines([]any{entry("/w/a.go", "needle one", 4), good[1]}, nil)},
		"line number off by two":  {"", promptLines, lines([]any{entry("/w/a.go", "needle one", 6), good[1]}, nil)},
		"line number fractional":  {"", promptLines, lines([]any{entry("/w/a.go", "needle one", 5.5), good[1]}, nil)},
		"line number zero":        {"", promptLines, lines([]any{good[0], entry("b.go", "needle two", 0)}, nil)},
		"file path missing":       {"", promptFiles, lines([]any{entry("z.go")}, nil)},
		"elided count wrong":      {"", promptElided, lines([]any{entry("/w/d.go", "needle three", 10)}, map[string]any{"num_elided_results": 2})},
		"elided count missing":    {"", promptElided, lines([]any{entry("/w/d.go", "needle three", 10)}, nil)},
		"elided count when none":  {"", promptLines, lines(good, map[string]any{"num_elided_results": 1})},
		"elided count zero":       {"", promptLines, lines(good, map[string]any{"num_elided_results": 0})},
		"elided count fractional": {"", promptElided, lines([]any{entry("/w/d.go", "needle three", 10)}, map[string]any{"num_elided_results": 3.5})},
	} {
		t.Run(name, func(t *testing.T) {
			js := c.js
			if js == "" {
				js = promptGen
			}
			raw := `{"cascadeId":"prompt-fixture","metadata":{},"generatorMetadata":[` + js + `],"steps":` + promptSteps + `}`
			if _, err := promptSource(t, raw, promptRecord(t, js, nil)); err != nil {
				t.Fatalf("fixture without the field: %v", err)
			}
			if _, err := promptSource(t, raw, promptRecord(t, js, map[int][]byte{c.index: c.unknown})); err == nil || err.Error() != "unknown persisted protobuf fields" {
				t.Fatalf("got %v, want unknown persisted protobuf fields", err)
			}
		})
	}
}

// Field 23 is accepted on a prompt only in a generator record's chat model,
// never on another message: not in an executor record, a step column, trajectory
// metadata, parent references, battle-mode infos, or anywhere else in a
// generator record.
func TestSourceOnlyPromptFieldElsewhere(t *testing.T) {
	field := promptField(linesPayload(t))
	for name, e := range map[string]sourceOnlyEdit{
		"generator record":     {"", genQuery, generatorMetadataName, promptGen, "", field},
		"chat model":           {"", genQuery, generatorMetadataName, promptGen, "chat_model", field},
		"generator planner":    {"", genQuery, generatorMetadataName, `{"plannerConfig":{"maxOutputTokens":1024},` + promptGen[1:], "planner_config", field},
		"executor record":      {"", "INSERT INTO executor_metadata VALUES(0,?1)", executorMetadataName, `{}`, "", field},
		"executor planner":     {"", "INSERT INTO executor_metadata VALUES(0,?1)", executorMetadataName, sourceOnlyCascade, "cascade_config.planner_config", field},
		"step payload":         {"", "UPDATE steps SET step_payload=?1 WHERE idx=1", "gemini_coder.Step", `{"type":"CORTEX_STEP_TYPE_GREP_SEARCH","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-lines","name":"grep_search"}},"grepSearch":{"query":"needle","matchPerLine":true,"results":[{"relativePath":"a.go","absolutePath":"/w/a.go","lineNumber":4,"content":"needle one"},{"relativePath":"b.go","absolutePath":"/w/b.go","content":"needle two"}],"totalResults":2}}`, "", field},
		"step grep result":     {"", "UPDATE steps SET step_payload=?1 WHERE idx=1", "gemini_coder.Step", `{"type":"CORTEX_STEP_TYPE_GREP_SEARCH","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-lines","name":"grep_search"}},"grepSearch":{"query":"needle","matchPerLine":true,"results":[{"relativePath":"a.go","absolutePath":"/w/a.go","lineNumber":4,"content":"needle one"},{"relativePath":"b.go","absolutePath":"/w/b.go","content":"needle two"}],"totalResults":2}}`, "grep_search", field},
		"step metadata column": {"", "UPDATE steps SET metadata=?1 WHERE idx=1", "exa.cortex_pb.CortexStepMetadata", `{"toolCall":{"id":"call-lines","name":"grep_search"}}`, "", field},
		"trajectory metadata":  {"", "UPDATE trajectory_metadata_blob SET data=?1 WHERE id='main'", "exa.cortex_pb.CortexTrajectoryMetadata", `{}`, "", field},
		"parent reference":     {"", "INSERT INTO parent_references VALUES(0,?1)", "exa.cortex_pb.CortexTrajectoryReference", `{}`, "", field},
		"battle mode info":     {"", "INSERT INTO battle_mode_infos VALUES(0,?1)", "exa.cortex_pb.BattleModeInfo", `{}`, "", field},
	} {
		t.Run(name, func(t *testing.T) {
			clean := e
			clean.unknown = nil
			raw := promptRaw
			if e.js != promptGen && e.name == generatorMetadataName {
				raw = `{"cascadeId":"prompt-fixture","metadata":{},"generatorMetadata":[` + e.js + `],"steps":` + promptSteps + `}`
			}
			if _, err := promptSource(t, raw, nil, clean); err != nil {
				t.Fatalf("fixture without the field: %v", err)
			}
			// Where number 23 is a declared field of another type, the bytes
			// fail to parse instead of reading as unknown; either refuses.
			if _, err := promptSource(t, raw, nil, e); err == nil || err.Error() != "unknown persisted protobuf fields" && err.Error() != "invalid persisted protobuf" {
				t.Fatalf("got %v, want unknown persisted protobuf fields", err)
			}
		})
	}
}

// messagePositions walks the descriptor from md to every message of type
// target, entering a message type at most limit times on one path and never
// descending into google.protobuf.Any.
func messagePositions(md protoreflect.MessageDescriptor, target protoreflect.FullName, limit int) []plannerPosition {
	var out []plannerPosition
	onPath := map[protoreflect.FullName]int{md.FullName(): 1}
	var walk func(md protoreflect.MessageDescriptor, at plannerPosition)
	walk = func(md protoreflect.MessageDescriptor, at plannerPosition) {
		if md.FullName() == target {
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

// The allowance is pinned to one location. The walk places field 23 on every
// ChatMessagePrompt a generator or executor record, or a step, can hold (up to
// two visits of each message type per path), on a prompt
// otherwise eligible, and decodes the record on its own and inside a
// trajectory. Only the generator record's chat_model.message_prompts passes
// the prompt-level checks; the step check then applies to it as above.
func TestSourceOnlyPromptFieldLocations(t *testing.T) {
	if err := loadSchema(); err != nil {
		t.Fatal(err)
	}
	payload := promptField(linesPayload(t))
	var accepted []string
	positions := 0
	for _, root := range []struct {
		name  protoreflect.FullName
		field protoreflect.Name
		limit int
	}{{generatorMetadataName, "generator_metadata", 2}, {executorMetadataName, "executor_metadatas", 2}, {"gemini_coder.Step", "steps", 2}} {
		d, err := schemaFiles.FindDescriptorByName(root.name)
		if err != nil {
			t.Fatal(err)
		}
		for _, at := range messagePositions(d.(protoreflect.MessageDescriptor), promptName, root.limit) {
			positions++
			where := at.String(root.name)
			m := plannerRecord(t, root.name, at, nil)
			current := m.ProtoReflect()
			for _, f := range at.fields {
				switch {
				case f.IsMap():
					current = current.Mutable(f).Map().Mutable(f.MapKey().Default().MapKey()).Message()
				case f.IsList():
					current = current.Mutable(f).List().Get(0).Message()
				default:
					current = current.Mutable(f).Message()
				}
			}
			pf := current.Descriptor().Fields()
			current.Set(pf.ByName("source"), protoreflect.ValueOfEnum(chatMessageSourceTool))
			current.Set(pf.ByName("tool_call_id"), protoreflect.ValueOfString("call-lines"))
			current.Set(pf.ByName("step_idx"), protoreflect.ValueOfInt32(1))
			current.SetUnknown(payload)
			data, err := proto.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			alone := acceptedAt(t, root.name, data, where)
			traj, err := newMessage(trajectoryName)
			if err != nil {
				t.Fatal(err)
			}
			tm := traj.ProtoReflect()
			tm.Mutable(tm.Descriptor().Fields().ByName(root.field)).List().Append(protoreflect.ValueOfMessage(m.ProtoReflect()))
			if data, err = proto.Marshal(traj); err != nil {
				t.Fatal(err)
			}
			if inside := acceptedAt(t, trajectoryName, data, where); inside != alone {
				t.Errorf("%s: accepted as a record %v, inside a trajectory %v", where, alone, inside)
			}
			t.Logf("%s accepted=%v", where, alone)
			if alone {
				accepted = append(accepted, where)
			}
		}
	}
	want := "exa.cortex_pb.CortexStepGeneratorMetadata.chat_model.message_prompts"
	if strings.Join(accepted, "\n") != want {
		t.Errorf("field 23 accepted at %d of %d positions:\n%s\nwant:\n%s", len(accepted), positions, strings.Join(accepted, "\n"), want)
	}
	// A step reaches prompts only through a nested trajectory's generator
	// metadata (three paths at two visits per type); an executor record and
	// trajectory metadata reach none outside an embedded Any.
	if positions < 4 {
		t.Fatalf("walk reached only %d prompt positions", positions)
	}
}

func TestSourceOnlyPromptFieldRecheck(t *testing.T) {
	raw := []byte(promptRaw)
	s := promptSnapshot(t, map[int][]byte{promptLines: promptField(linesPayload(t)), promptElided: promptField(elidedPayload(t))})
	a, r := s.VerifyResponses(raw, raw)
	if r.State != Verified {
		t.Fatal(r)
	}
	for name, unknown := range map[string]map[int][]byte{
		"payload changed": {promptLines: promptField(structBytes(t, map[string]any{"results": []any{entry("a.go", "needle one", 5), entry("b.go", "needle two", 1)}})), promptElided: promptField(elidedPayload(t))},
		"one removed":     {promptElided: promptField(elidedPayload(t))},
		"all removed":     nil,
	} {
		t.Run(name, func(t *testing.T) {
			if r := promptSnapshot(t, unknown).CheckAttestation(raw, a); r.State != Incomplete || r.Reason != "persisted source content changed" {
				t.Fatalf("got %+v", r)
			}
		})
	}
	for name, change := range map[string]func(*Attestation){
		"count changed":   func(a *Attestation) { a.SourceOnly[0].Count = 1 },
		"path changed":    func(a *Attestation) { a.SourceOnly[0].Path = SourceOnlyPlannerPath },
		"payload changed": func(a *Attestation) { a.SourceOnly[0].Occurrences[0].Payload = filesPayload(t) },
		"location changed": func(a *Attestation) {
			a.SourceOnly[0].Occurrences[1].Location = "generator_metadata[0].chat_model.message_prompts[2]"
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := copyAttestation(t, a)
			change(changed)
			if r := s.CheckAttestation(raw, changed); r.State != Incomplete || r.Reason != "sidecar source-only record changed since verification" {
				t.Fatalf("got %+v", r)
			}
		})
	}
	t.Run("previous profile", func(t *testing.T) {
		old := copyAttestation(t, a)
		old.Profile = "sqlite-v1-format0-agy-1.3.0-19be6af38f7b-planner50-permission2"
		if r := s.CheckAttestation(raw, old); r.State != Unknown {
			t.Fatalf("got %+v", r)
		}
	})
	// Removing the field from the comparison copy hides nothing else on the
	// prompt or in the step it names.
	for name, changed := range map[string][]byte{
		"prompt text":   bytes.Replace(raw, []byte(`"prompt":"tool output","numTokens":3,"toolCallId":"call-lines"`), []byte(`"prompt":"other output","numTokens":3,"toolCallId":"call-lines"`), 1),
		"step content":  bytes.Replace(raw, []byte(`"needle one"`), []byte(`"needle 1"`), 1),
		"step line":     bytes.Replace(raw, []byte(`"lineNumber":4`), []byte(`"lineNumber":5`), 1),
		"total results": bytes.Replace(raw, []byte(`"totalResults":4`), []byte(`"totalResults":5`), 1),
		"prompt struct": bytes.Replace(raw, []byte(`"prompt":"tool output","numTokens":3,"toolCallId":"call-lines"`), []byte(`"prompt":"tool output","promptStruct":{},"numTokens":3,"toolCallId":"call-lines"`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(changed, raw) {
				t.Fatal("invalid fixture mutation")
			}
			if _, r := s.VerifyResponses(changed, raw); r.State != Incomplete {
				t.Fatalf("raw: got %+v", r)
			}
		})
	}
}

// The retained historical sources: every observed field-23 payload must be
// accepted. AGY_READER_FIELD23_SOURCES names a directory of copies of the 15
// retained CLI conversations that carry it (never the originals: reading opens
// SQLite normally). AGY_READER_FIELD23_RPC, when set, names a directory with
// <id>/raw.json full RPC captures (the response, or its trajectory) to compare
// against. The test is skipped when
// the copies are not available.
func TestSourceOnlyPromptFieldRetained(t *testing.T) {
	dir := os.Getenv("AGY_READER_FIELD23_SOURCES")
	if dir == "" {
		t.Skip("AGY_READER_FIELD23_SOURCES not set")
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.db"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no sources in %s: %v", dir, err)
	}
	rpc := os.Getenv("AGY_READER_FIELD23_RPC")
	sources, occurrences, compared := 0, 0, 0
	for _, p := range paths {
		id := strings.TrimSuffix(filepath.Base(p), ".db")
		s, err := ReadSnapshot(t.Context(), p)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		var prompt *SourceOnlyField
		for i := range s.sourceOnly {
			if s.sourceOnly[i].Path == SourceOnlyPromptPath {
				prompt = &s.sourceOnly[i]
			}
		}
		if prompt == nil {
			t.Errorf("%s: no field-23 record", id)
			continue
		}
		sources++
		occurrences += prompt.Count
		t.Logf("%s: %d field-23 payloads accepted (%s)", id[:8], prompt.Count, prompt.SHA256[:12])
		if rpc == "" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(rpc, id, "raw.json"))
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		// A capture may hold the whole RPC response; compare its trajectory.
		var response struct {
			Trajectory json.RawMessage `json:"trajectory"`
		}
		if json.Unmarshal(raw, &response) == nil && len(response.Trajectory) > 0 {
			raw = response.Trajectory
		}
		compared++
		if r := s.CompareFullRaw(raw); r.State != Verified {
			t.Errorf("%s: full raw %+v", id, r)
		}
	}
	t.Logf("%d sources, %d field-23 payloads accepted; %d full raw comparisons", sources, occurrences, compared)
	if sources != 15 || occurrences != 47 {
		t.Fatalf("accepted %d payloads in %d sources, want 47 in 15", occurrences, sources)
	}
}
