package completeness

// Native fuzz targets for the parsers that read untrusted input: raw RPC JSON
// and tool-call arguments, SQLite source files, and protobuf bytes decoded
// through the embedded descriptors. Every seed is synthetic and built here or
// from the synthetic fixtures beside these tests; none comes from a real
// session. Run one target with, for example,
//
//	go test -run='^$' -fuzz=FuzzJSONDecoder -fuzztime=60s ./internal/completeness
//
// A crasher found by the fuzzer is kept under testdata/fuzz/<Target>/ and
// replays as an ordinary test case in go test.

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mjacobs/agy-reader/internal/testfixture"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/structpb"
)

// FuzzJSONDecoder covers the JSON token walk that refuses repeated object keys
// (objectKeysUnique), the escape check that refuses unpaired surrogates and
// invalid UTF-8 (validJSONText, literalString), the strict artifact-metadata
// decoder, the tool-argument comparison built on them, and the RPC trajectory
// JSON entry point.
func FuzzJSONDecoder(f *testing.F) {
	for _, seed := range []string{
		`{}`, `[]`, `null`, `0`, `1e400`, `-0.0e-0`, `"plain"`, `""`,
		`{"a":1,"b":2}`, `{"a":1,"a":2}`, `{"a":1,"A":1}`,
		`{"a":{"b":1,"b":2}}`, `[{"k":1},{"k":1}]`, `{"a":[{"b":{"c":{}}}],"d":[]}`,
		`{"a":1} x`, `[1] ]`, `{"a":1}}`, `{"a" 1}`, `{1:2}`, `[[[[[[[[[[]]]]]]]]]]`,
		`"😀"`, `"\ud800"`, `"\udc00"`, `"\ud800A"`, `"\ud800\ud800"`,
		`"\\ud800"`, `"�"`, `"�"`, "\"\xef\xbf\xbd\"", "\"\xff\"", "\"\xed\xa0\x80\"",
		`"\u00"`, `"\x"`, `"\"\\\/\b\f\n\r\t"`,
		`{"RequestFeedback":true,"Summary":"s","UserFacing":false}`,
		`{"Summary":"a","summary":"b"}`, `{"Summary":null}`, `{"UserFacing":true,"UserFacing":true}`,
		`{"Summary":"\ud800"}`,
		syntheticRaw, syntheticPresentation,
		`{"cascadeId":"c","steps":[{"type":"CORTEX_STEP_TYPE_NOT_DECLARED"}]}`,
		`{"cascadeId":"c","agyReader":{"x":1},"steps":[]}`,
		`{"steps":{"type":1}}`, `{"steps":[null]}`, `{"metadata":[]}`,
	} {
		f.Add([]byte(seed))
	}
	if err := loadSchema(); err != nil {
		f.Fatal(err)
	}
	m, err := newMessage(trajectoryName)
	if err != nil {
		f.Fatal(err)
	}
	trajectory := m.ProtoReflect().Descriptor()

	f.Fuzz(func(t *testing.T, data []byte) {
		unique := objectKeysUnique(data)
		valid := json.Valid(data)
		// Wrapping valid JSON must not change the walk's answer, and a repeated
		// key added around it must always be refused. Long inputs are skipped
		// so the wrapper cannot cross the walk's nesting limit.
		if valid && len(data) < 4096 {
			if got := objectKeysUnique(wrap(`[`, data, `]`)); got != unique {
				t.Fatalf("array wrapper changed key uniqueness: %v, unwrapped %v", got, unique)
			}
			if got := objectKeysUnique(wrap(`{"a":`, data, `,"b":0}`)); got != unique {
				t.Fatalf("object wrapper changed key uniqueness: %v, unwrapped %v", got, unique)
			}
			if objectKeysUnique(wrap(`{"a":`, data, `,"a":0}`)) {
				t.Fatal("repeated key around a valid value accepted")
			}
			// Re-encoding through a Go map leaves every key once.
			var v any
			d := json.NewDecoder(bytes.NewReader(data))
			d.UseNumber()
			if d.Decode(&v) == nil {
				if out, err := json.Marshal(v); err == nil && !objectKeysUnique(out) {
					t.Fatalf("re-encoded value refused: %q", out)
				}
			}
		}

		if text, ok := literalString(data); ok {
			if !utf8.ValidString(text) {
				t.Fatal("accepted string decodes to invalid UTF-8")
			}
			// encoding/json writes U+FFFD for each malformed sequence it
			// replaces, so an accepted string must hold exactly the U+FFFD
			// characters the text spelled out.
			if got, want := countReplacement(text), spelledReplacement(data); got != want {
				t.Fatalf("accepted string has %d U+FFFD, text spells %d", got, want)
			}
			if !jsonArgumentEqual(data, text) {
				t.Fatal("accepted string differs from its own text")
			}
		}
		if len(data) > 0 && data[0] != '"' {
			want := valid && unique && validJSONText(data)
			if got := jsonArgumentEqual(data, string(data)); got != want {
				t.Fatalf("argument compared with itself: %v, want %v", got, want)
			}
		}

		if meta, ok := decodeArtifactMetadata(string(data)); ok {
			again, ok := decodeArtifactMetadata(encodeArtifactMetadata(t, meta))
			if !ok || again != meta {
				t.Fatalf("artifact metadata did not survive re-encoding: %+v, then %+v", meta, again)
			}
		}

		_ = validateRPCEnums(data, trajectory)
		_, _ = rpcMessage(data)
	})
}

func wrap(before string, data []byte, after string) []byte {
	out := append([]byte(before), data...)
	return append(out, after...)
}

func countReplacement(s string) int {
	n := 0
	for _, r := range s {
		if r == utf8.RuneError {
			n++
		}
	}
	return n
}

// spelledReplacement counts the U+FFFD characters JSON string text spells
// itself, as UTF-8 bytes or as a � escape in either case.
func spelledReplacement(raw []byte) int {
	n := 0
	for i := 0; i < len(raw); {
		if raw[i] == '\\' {
			if i+6 <= len(raw) && raw[i+1] == 'u' {
				if v, err := strconv.ParseUint(string(raw[i+2:i+6]), 16, 16); err == nil && v == utf8.RuneError {
					n++
				}
				i += 6
				continue
			}
			i += 2
			continue
		}
		r, size := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && size == 3 {
			n++
		}
		i += size
	}
	return n
}

func encodeArtifactMetadata(t *testing.T, meta artifactMetadataArg) string {
	t.Helper()
	obj := map[string]any{}
	for _, key := range splitKeys(meta.Keys) {
		switch key {
		case "RequestFeedback":
			obj[key] = meta.RequestFeedback
		case "Summary":
			obj[key] = meta.Summary
		case "UserFacing":
			obj[key] = meta.UserFacing
		default:
			t.Fatalf("decoded metadata names unexpected key %q", key)
		}
	}
	out, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func splitKeys(keys string) []string {
	if keys == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i <= len(keys); i++ {
		if i == len(keys) || keys[i] == ',' {
			out = append(out, keys[start:i])
			start = i + 1
		}
	}
	return out
}

// FuzzSQLiteExtractor feeds whole SQLite files to ReadSnapshot. Seeds are
// small synthetic sources: the supported layout, an unsupported user_version,
// a missing table, an extra column, a bad step format, an empty database and
// non-database bytes. The fuzzer then mutates pages, headers and cells.
//
// Each execution runs the whole SQLite engine under coverage tracking, so it is
// slow, and with the default 60-second minimization of new inputs a short run
// can spend most of its time at 0 execs/sec. Pass -fuzzminimizetime=3s: a
// 60-second run then makes about a million executions instead of tens of
// thousands.
func FuzzSQLiteExtractor(f *testing.F) {
	small := "PRAGMA page_size=512;" + testfixture.Schema
	seed := func(raw string, schema string, edits ...string) {
		p := filepath.Join(f.TempDir(), "seed.db")
		db := testfixture.SourceWithSchema(f, p, []byte(raw), schema)
		for _, q := range edits {
			testfixture.Exec(f, db, q)
		}
		if err := db.Close(); err != nil {
			f.Fatal(err)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	seed(syntheticRaw, small)
	seed(sourceOnlyRaw, small)
	seed(syntheticRaw, small, "PRAGMA user_version=2")
	seed(syntheticRaw, small, "DROP TABLE battle_mode_infos")
	seed(syntheticRaw, small, "ALTER TABLE steps ADD COLUMN future TEXT")
	seed(syntheticRaw, small, "UPDATE steps SET step_format=1")
	seed(syntheticRaw, small, "UPDATE steps SET idx=2")
	seed(syntheticRaw, small, "INSERT INTO trajectory_meta VALUES('t','c',0,0)")
	seed(syntheticRaw, small, "UPDATE trajectory_metadata_blob SET id='other'")
	seed(sourceOnlyRaw, small, "UPDATE gen_metadata SET size=size+1")
	seed(syntheticRaw, small, "UPDATE steps SET step_payload=X'ff'")
	f.Add([]byte{})
	f.Add([]byte("SQLite format 3\x00"))
	f.Add([]byte("not a database"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		p := filepath.Join(t.TempDir(), "source.db")
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		s, err := ReadSnapshot(ctx, p)
		if ctx.Err() != nil {
			t.Fatal("source read did not finish in time")
		}
		// The read must stay read-only: the file is unchanged afterwards.
		after, readErr := os.ReadFile(p)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(after, data) {
			t.Fatal("ReadSnapshot changed the source file")
		}
		if err != nil {
			if s != nil {
				t.Fatal("snapshot returned with an error")
			}
			return
		}
		// Only user_version 1 is supported. With no WAL file beside it, the
		// database header holds the version the read saw.
		if len(data) < 100 || binary.BigEndian.Uint32(data[60:64]) != 1 {
			t.Fatal("source accepted without user_version 1 in its header")
		}
		if len(s.Digest) != 64 || s.StepCount < 0 {
			t.Fatalf("malformed snapshot: digest %q, %d steps", s.Digest, s.StepCount)
		}
		_ = s.CascadeID()
		_ = s.CompareFullRaw([]byte(syntheticRaw))
	})
}

// TestDecodeRefusesRepeatedMapKeyWithWrongWireType replays the crasher kept at
// testdata/fuzz/FuzzDescriptorParser/df15c77f1523feba. It is a 10-byte
// trajectory whose nested map entry gives its key twice, the second time with
// the wrong wire type; the protobuf decoder used to panic on it. decode must
// refuse it, and so must an embedded Any that carries the same bytes.
func TestDecodeRefusesRepeatedMapKeyWithWrongWireType(t *testing.T) {
	crasher := []byte("R\bZ\x06\n\x00\b000")
	msg, err := decode(trajectoryName, crasher)
	if err == nil || msg != nil {
		t.Fatalf("decode = %v, %v; want nil and an error", msg, err)
	}

	anyMsg, err := newMessage("google.protobuf.Any")
	if err != nil {
		t.Fatal(err)
	}
	m := anyMsg.ProtoReflect()
	fields := m.Descriptor().Fields()
	m.Set(fields.ByName("type_url"), protoreflect.ValueOfString("type.googleapis.com/"+string(trajectoryName)))
	m.Set(fields.ByName("value"), protoreflect.ValueOfBytes(crasher))
	if err := normalize(m, 0); err == nil {
		t.Fatal("normalize accepted an Any carrying the crasher")
	}
}

// FuzzDescriptorParser covers the protobuf side: persisted records decoded and
// normalized through the embedded descriptors (including embedded Any payloads
// and the declared source-only fields 50 and 23), and FileDescriptorSet
// parsing as loadSchema performs it. The descriptor extractor that reads agy
// binaries is Python (scripts/extract_descriptors.py) and has its own tests.
func FuzzDescriptorParser(f *testing.F) {
	if err := loadSchema(); err != nil {
		f.Fatal(err)
	}
	m, err := newMessage(trajectoryName)
	if err != nil {
		f.Fatal(err)
	}
	fields := m.ProtoReflect().Descriptor().Fields()
	names := []protoreflect.FullName{
		trajectoryName,
		"gemini_coder.Step",
		"exa.cortex_pb.StepRenderInfo",
		generatorMetadataName,
		executorMetadataName,
		fields.ByName("metadata").Message().FullName(),
	}
	const descriptorKind = 6 // kind%7 == 6 parses a FileDescriptorSet

	encode := func(name protoreflect.FullName, js string) []byte {
		msg := testfixture.Message(f, string(name), []byte(js))
		b, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg)
		if err != nil {
			f.Fatal(err)
		}
		return b
	}
	trajectoryKind := uint8(0)
	f.Add(trajectoryKind, []byte{})
	f.Add(trajectoryKind, encode(trajectoryName, syntheticRaw))
	f.Add(trajectoryKind, encode(trajectoryName, sourceOnlyRaw))
	f.Add(trajectoryKind, promptTrajectory(f))
	f.Add(uint8(1), encode("gemini_coder.Step", `{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","runCommand":{"commandLine":"echo fixture"}}`))
	f.Add(uint8(1), []byte{0x08, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x01})
	f.Add(uint8(2), []byte{})
	f.Add(uint8(3), withUnknown(f, generatorMetadataName, sourceOnlyGen, "planner_config", plannerField(0x10, 0x00, 0x20, 0x00)))
	f.Add(uint8(3), withUnknown(f, generatorMetadataName, sourceOnlyGen, "planner_config", plannerField(0x20, 0x00, 0x10, 0x00)))
	f.Add(uint8(4), withUnknown(f, executorMetadataName, sourceOnlyCascade, "cascade_config.planner_config", plannerField()))
	f.Add(uint8(4), withUnknown(f, executorMetadataName, sourceOnlyOverride, "request_override_config.planner_config", plannerField()))
	f.Add(uint8(5), encode(names[5], `{}`))
	f.Add(uint8(descriptorKind), seedDescriptorSet(f))
	f.Add(uint8(descriptorKind), []byte{0x0a, 0x02, 0x0a, 0x00})

	f.Fuzz(func(t *testing.T, kind uint8, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		if kind%7 == descriptorKind {
			parseDescriptorSet(data)
			return
		}
		name := names[int(kind%7)]
		msg, err := decode(name, data)
		if err != nil {
			if msg != nil {
				t.Fatal("message returned with an error")
			}
			return
		}
		// A decoded record re-encodes to bytes that decode to the same record:
		// normalization is stable.
		b, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg)
		if err != nil {
			t.Fatalf("decoded %s does not re-encode: %v", name, err)
		}
		again, err := decode(name, b)
		if err != nil {
			t.Fatalf("re-encoded %s refused: %v", name, err)
		}
		if !proto.Equal(msg, again) {
			t.Fatalf("re-encoded %s decodes differently", name)
		}
		if name == trajectoryName {
			if s, err := finishSnapshot(msg); err == nil {
				_ = s.CascadeID()
				_ = s.CompareFullRaw([]byte(syntheticRaw))
			}
		}
	})
}

// parseDescriptorSet repeats loadSchema's parse of a FileDescriptorSet and
// then resolves every message it declares.
func parseDescriptorSet(data []byte) {
	var set descriptorpb.FileDescriptorSet
	if proto.Unmarshal(data, &set) != nil {
		return
	}
	files, err := protodesc.NewFiles(&set)
	if err != nil {
		return
	}
	types := dynamicpb.NewTypes(files)
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for i := 0; i < fd.Messages().Len(); i++ {
			if mt, err := types.FindMessageByName(fd.Messages().Get(i).FullName()); err == nil {
				_ = mt.New()
			}
		}
		return true
	})
}

func seedDescriptorSet(t testing.TB) []byte {
	t.Helper()
	set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{{
		Name:    proto.String("seed.proto"),
		Package: proto.String("seed"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Record"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{Name: proto.String("name"), JsonName: proto.String("name"), Number: proto.Int32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
				{Name: proto.String("kind"), JsonName: proto.String("kind"), Number: proto.Int32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(), TypeName: proto.String(".seed.Kind")},
				{Name: proto.String("children"), JsonName: proto.String("children"), Number: proto.Int32(3), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".seed.Record")},
			},
		}},
		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name:  proto.String("Kind"),
			Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("KIND_UNSPECIFIED"), Number: proto.Int32(0)}},
		}},
	}}}
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// withUnknown encodes the record js with unknown appended to the message at
// path (dot-separated singular field names).
func withUnknown(t testing.TB, name protoreflect.FullName, js, path string, unknown []byte) []byte {
	t.Helper()
	msg := testfixture.Message(t, string(name), []byte(js)).ProtoReflect()
	current := msg
	for _, field := range splitPath(path) {
		current = current.Mutable(current.Descriptor().Fields().ByName(protoreflect.Name(field))).Message()
	}
	current.SetUnknown(append(append([]byte(nil), current.GetUnknown()...), unknown...))
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg.Interface())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func splitPath(path string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '.' {
			out = append(out, path[start:i])
			start = i + 1
		}
	}
	return out
}

// promptTrajectory is the synthetic prompt23 trajectory with the field-23
// payload its grep_search step would have produced on the first tool prompt.
func promptTrajectory(t testing.TB) []byte {
	t.Helper()
	msg := testfixture.Message(t, string(trajectoryName), []byte(promptRaw)).ProtoReflect()
	gens := msg.Get(msg.Descriptor().Fields().ByName("generator_metadata")).List()
	gen := gens.Get(0).Message()
	chat := gen.Mutable(gen.Descriptor().Fields().ByName("chat_model")).Message()
	prompt := chat.Mutable(chat.Descriptor().Fields().ByName("message_prompts")).List().Get(promptLines).Message()
	s, err := structpb.NewStruct(map[string]any{"results": []any{
		map[string]any{"path": "/w/a.go", "line_content": "needle one", "line_number": 5},
		map[string]any{"path": "b.go", "line_content": "needle two", "line_number": 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	prompt.SetUnknown(protowire.AppendBytes(protowire.AppendTag(nil, promptSourceOnly, protowire.BytesType), payload))
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg.Interface())
	if err != nil {
		t.Fatal(err)
	}
	return b
}
