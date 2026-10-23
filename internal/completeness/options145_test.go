package completeness

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// These are synthetic SQLite regression fixtures, not evidence that an agy
// invocation produced or exercised the options. Native qualification separately
// supplies agy-written sources and independently captured full/default RPCs.
const options145Tools = `{"antigravityBrowser":{"enableBrowserSubagentAutoInvoke":true},"viewFile":{"skipPluginLogo":true},"toolOutput":{"perTool":{"read_file":{"maxOutputBytes":2048},"run_command":{"maxOutputBytes":-1},"empty":{},"zero":{"maxOutputBytes":0}}}}`

func options145Raw(location, tools string) string {
	planner := `{"toolConfig":` + tools + `}`
	cascade := `{"plannerConfig":` + planner + `}`
	metadata := `"metadata":{}`
	steps := `"steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","runCommand":{"commandLine":"echo fixture"}}]`
	extra := ""
	switch location {
	case "static config":
		metadata = `"metadata":{"staticConfig":{"cascadeConfig":` + cascade + `}}`
	case "agent script":
		metadata = `"metadata":{"agentScript":{"config":{"cascadeConfig":` + cascade + `}}}`
	case "generator":
		extra = `,"generatorMetadata":[{"plannerConfig":` + planner + `}]`
	case "executor":
		extra = `,"executorMetadatas":[{"cascadeConfig":` + cascade + `}]`
	case "request override":
		extra = `,"executorMetadatas":[{"requestOverrideConfig":` + cascade + `}]`
	case "user config":
		steps = `"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userConfig":` + cascade + `}}]`
	case "last user config":
		steps = `"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"lastUserConfig":` + cascade + `}}]`
	default:
		panic("unrecognized option fixture location: " + location)
	}
	return `{"cascadeId":"options145-fixture",` + metadata + `,` + steps + extra + `}`
}

func options145Snapshot(t *testing.T, raw string) *Snapshot {
	t.Helper()
	p := filepath.Join(t.TempDir(), "source.db")
	testfixture.Source(t, p, []byte(raw))
	s, err := ReadSnapshot(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func options145RequireBoth(t *testing.T, s *Snapshot, raw, changed string) {
	t.Helper()
	for _, full := range []bool{true, false} {
		name := "default"
		a, b := raw, changed
		if full {
			name, a, b = "full", changed, raw
		}
		t.Run(name, func(t *testing.T) {
			if _, r := s.VerifyResponses([]byte(a), []byte(b)); r.State != Incomplete {
				t.Fatalf("changed %s RPC: %+v", name, r)
			}
		})
	}
}

func TestOptions145PersistedConfigRetained(t *testing.T) {
	for _, location := range []string{"static config", "agent script", "generator", "executor", "request override", "user config", "last user config"} {
		t.Run(location, func(t *testing.T) {
			raw := options145Raw(location, options145Tools)
			s := options145Snapshot(t, raw)
			before := proto.Clone(s.trajectory)
			a, r := s.VerifyResponses([]byte(raw), []byte(raw))
			if r.State != Verified || r.Reason != "" || len(a.SourceOnly) != 0 {
				t.Fatalf("options must be retained, not source-only: %+v %+v", r, a)
			}
			if !proto.Equal(before, s.trajectory) {
				t.Fatal("verification changed persisted configuration")
			}
			if r := s.CheckAttestation([]byte(raw), a); r.State != Verified {
				t.Fatal(r)
			}
			for name, change := range map[string][2]string{
				"browser changed":  {`"enableBrowserSubagentAutoInvoke":true`, `"enableBrowserSubagentAutoInvoke":false`},
				"browser dropped":  {`"enableBrowserSubagentAutoInvoke":true`, ``},
				"logo changed":     {`"skipPluginLogo":true`, `"skipPluginLogo":false`},
				"logo dropped":     {`"skipPluginLogo":true`, ``},
				"positive changed": {`"maxOutputBytes":2048`, `"maxOutputBytes":2049`},
				"positive dropped": {`"maxOutputBytes":2048`, ``},
				"negative changed": {`"maxOutputBytes":-1`, `"maxOutputBytes":1`},
				"negative dropped": {`"maxOutputBytes":-1`, ``},
				"zero dropped":     {`"maxOutputBytes":0`, ``},
				"absent changed":   {`"empty":{}`, `"empty":{"maxOutputBytes":0}`},
				"entry dropped":    {`"empty":{},`, ``},
				"key changed":      {`"read_file":`, `"other_tool":`},
			} {
				t.Run(name, func(t *testing.T) {
					changed := strings.Replace(raw, change[0], change[1], 1)
					options145RequireBoth(t, s, raw, changed)
				})
			}
		})
	}
}

func TestOptions145OptionalBoolPresence(t *testing.T) {
	for _, field := range []string{"enableBrowserSubagentAutoInvoke", "skipPluginLogo"} {
		t.Run(field, func(t *testing.T) {
			presentTools := strings.Replace(options145Tools, fmt.Sprintf(`"%s":true`, field), fmt.Sprintf(`"%s":false`, field), 1)
			absentTools := strings.Replace(presentTools, fmt.Sprintf(`"%s":false`, field), "", 1)
			present, absent := options145Raw("executor", presentTools), options145Raw("executor", absentTools)
			p, a := options145Snapshot(t, present), options145Snapshot(t, absent)
			if p.Digest == a.Digest {
				t.Fatal("explicit false and absent option have the same source digest")
			}
			for _, pair := range []struct {
				s        *Snapshot
				raw, bad string
			}{{p, present, absent}, {a, absent, present}} {
				if _, r := pair.s.VerifyResponses([]byte(pair.raw), []byte(pair.raw)); r.State != Verified {
					t.Fatal(r)
				}
				options145RequireBoth(t, pair.s, pair.raw, pair.bad)
			}
		})
	}
}

func options145Varint(number protowire.Number, value uint64) []byte {
	return protowire.AppendVarint(protowire.AppendTag(nil, number, protowire.VarintType), value)
}

func options145Entry(value []byte) []byte {
	key := bytesField(1, []byte("read_file")...)
	return append(key, bytesField(2, value...)...)
}

// Wrap wire bytes directly so unknown map-entry fields cannot disappear while
// a fixture is constructed through the protobuf map API.
func options145Executor(output []byte) []byte {
	return bytesField(10, bytesField(1, bytesField(13, bytesField(56, output...)...)...)...)
}

func TestOptions145PerToolRejectsUnknownAndMalformedWire(t *testing.T) {
	validValue := options145Varint(1, 2048)
	validEntry := options145Entry(validValue)
	validOutput := bytesField(2, validEntry...)
	for name, output := range map[string][]byte{
		"unknown override field": bytesField(2, options145Entry(append(validValue, options145Varint(99, 1)...))...),
		"unknown entry field":    bytesField(2, append(append([]byte(nil), validEntry...), options145Varint(99, 1)...)...),
		"wrong map wire":         options145Varint(2, 1),
		"wrong key wire":         bytesField(2, append(options145Varint(1, 1), bytesField(2, validValue...)...)...),
		"wrong value wire":       bytesField(2, append(bytesField(1, []byte("read_file")...), options145Varint(2, 1)...)...),
		"wrong override wire":    bytesField(2, options145Entry(bytesField(1, 'x'))...),
		"truncated override":     bytesField(2, options145Entry(protowire.AppendTag(nil, 1, protowire.VarintType))...),
		"truncated entry key":    bytesField(2, []byte{0x0a, 0x03, 'x'}...),
		"truncated map entry":    append(protowire.AppendTag(nil, 2, protowire.BytesType), 0x7f),
		"duplicate wrong key":    bytesField(2, append(append([]byte(nil), validEntry...), options145Varint(1, 1)...)...),
		"duplicate wrong value":  bytesField(2, append(append([]byte(nil), validEntry...), options145Varint(2, 1)...)...),
		"duplicate key member":   bytesField(2, append(append([]byte(nil), validEntry...), bytesField(1, []byte("other_tool")...)...)...),
		"duplicate value member": bytesField(2, append(append([]byte(nil), validEntry...), bytesField(2, validValue...)...)...),
		"duplicate map key":      append(append([]byte(nil), validOutput...), validOutput...),
		"duplicate default key":  append(bytesField(2), bytesField(2, bytesField(1)...)...),
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "source.db")
			db := testfixture.Source(t, p, []byte(options145Raw("executor", options145Tools)))
			testfixture.Exec(t, db, "UPDATE executor_metadata SET data=? WHERE idx=0", options145Executor(validOutput))
			if _, err := ReadSnapshot(t.Context(), p); err != nil {
				t.Fatalf("valid control: %v", err)
			}
			testfixture.Exec(t, db, "UPDATE executor_metadata SET data=? WHERE idx=0", options145Executor(output))
			if _, err := ReadSnapshot(t.Context(), p); err == nil {
				t.Fatal("accepted unknown or malformed persisted per_tool wire content")
			}
		})
	}
}

func TestOptions145PerToolDefaultMembers(t *testing.T) {
	for name, tc := range map[string]struct {
		entry []byte
		json  string
	}{
		"both absent":  {nil, `"":{}`},
		"value absent": {bytesField(1, []byte("read_file")...), `"read_file":{}`},
		"key absent":   {bytesField(2, options145Varint(1, 2048)...), `"":{"maxOutputBytes":2048}`},
	} {
		t.Run(name, func(t *testing.T) {
			raw := options145Raw("executor", `{"toolOutput":{"perTool":{`+tc.json+`}}}`)
			p := filepath.Join(t.TempDir(), "source.db")
			db := testfixture.Source(t, p, []byte(raw))
			testfixture.Exec(t, db, "UPDATE executor_metadata SET data=? WHERE idx=0", options145Executor(bytesField(2, tc.entry...)))
			s, err := ReadSnapshot(t.Context(), p)
			if err != nil {
				t.Fatal(err)
			}
			if _, r := s.VerifyResponses([]byte(raw), []byte(raw)); r.State != Verified {
				t.Fatalf("protobuf map defaults: %+v", r)
			}
		})
	}
}

func TestOptions145PerToolAcrossMergedMessageFragments(t *testing.T) {
	output := func(key string, value uint64) []byte {
		entry := append(bytesField(1, []byte(key)...), bytesField(2, options145Varint(1, value)...)...)
		return bytesField(2, entry...)
	}
	// Each split writes two occurrences of a singular message field. Protobuf
	// merges their children, so map keys must be checked across both fragments.
	fragments := func(split int, left, right []byte) []byte {
		path := []protowire.Number{10, 1, 13, 56} // cascade, planner, tool, output
		for i := len(path) - 1; i >= split; i-- {
			left, right = bytesField(path[i], left...), bytesField(path[i], right...)
		}
		joined := append(left, right...)
		for i := split - 1; i >= 0; i-- {
			joined = bytesField(path[i], joined...)
		}
		return joined
	}
	for split, name := range []string{"cascade_config", "planner_config", "tool_config", "tool_output"} {
		t.Run(name, func(t *testing.T) {
			raw := options145Raw("executor", `{"toolOutput":{"perTool":{"read_file":{"maxOutputBytes":2048},"run_command":{"maxOutputBytes":0}}}}`)
			p := filepath.Join(t.TempDir(), "source.db")
			db := testfixture.Source(t, p, []byte(raw))
			good := fragments(split, output("read_file", 2048), output("run_command", 0))
			testfixture.Exec(t, db, "UPDATE executor_metadata SET data=? WHERE idx=0", good)
			s, err := ReadSnapshot(t.Context(), p)
			if err != nil {
				t.Fatalf("distinct keys in merged fragments: %v", err)
			}
			if _, r := s.VerifyResponses([]byte(raw), []byte(raw)); r.State != Verified {
				t.Fatalf("merged configuration values differ: %+v", r)
			}
			for _, value := range []uint64{0, 2048} {
				bad := fragments(split, output("read_file", 2048), output("read_file", value))
				testfixture.Exec(t, db, "UPDATE executor_metadata SET data=? WHERE idx=0", bad)
				if _, err := ReadSnapshot(t.Context(), p); err == nil {
					t.Fatalf("duplicate key accepted across %s fragments (second value %d)", name, value)
				}
			}
		})
	}
}

func TestOptions145RepeatedMessagesUseBoundedState(t *testing.T) {
	m, err := newMessage("exa.cortex_pb.ExecutorMetadata")
	if err != nil {
		t.Fatal(err)
	}
	desc := m.ProtoReflect().Descriptor()
	for name, fragment := range map[string][]byte{
		"empty cascade": bytesField(10),
		"nested output": options145Executor(nil),
	} {
		t.Run(name, func(t *testing.T) {
			// Protobuf merges these into the same singular messages. The wire
			// guard must not retain a slice header for every occurrence.
			large := bytes.Repeat(fragment, 4096)
			allocations := func(data []byte) float64 {
				return testing.AllocsPerRun(5, func() {
					if err := checkQualifiedMapsWire(data, desc, 0); err != nil {
						t.Fatal(err)
					}
				})
			}
			small, repeated := allocations(fragment), allocations(large)
			t.Logf("allocations: one fragment %.0f, 4096 fragments %.0f", small, repeated)
			if repeated > small+2 {
				t.Fatalf("validator retains repeated message fragments: %.0f > %.0f + 2 allocations", repeated, small)
			}
		})
	}
}

func TestOptions145PreservesMessageMapDepth(t *testing.T) {
	for _, levels := range []int{31, 32, 33, 34, 40, 45} {
		t.Run(fmt.Sprint(levels), func(t *testing.T) {
			object := strings.Repeat(`{"a":`, levels) + `1` + strings.Repeat(`}`, levels)
			m := testfixture.Message(t, "gemini_coder.Step", []byte(`{"rpcAction":{"arguments":`+object+`}}`))
			wire, err := proto.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			// Compare against the pre-guard decoder at the nesting boundary.
			// protobuf-go v1.36.11 counts synthetic map entries in its limit.
			baseline, err := newMessage("gemini_coder.Step")
			if err != nil {
				t.Fatal(err)
			}
			baselineErr := (proto.UnmarshalOptions{RecursionLimit: maxDepth, Resolver: schemaTypes}).Unmarshal(wire, baseline)
			if baselineErr == nil {
				baselineErr = normalize(baseline.ProtoReflect(), 0)
			}
			_, guardedErr := decode("gemini_coder.Step", wire)
			t.Logf("depth %d: baseline accepted=%t, guarded accepted=%t", levels, baselineErr == nil, guardedErr == nil)
			if (baselineErr == nil) != (guardedErr == nil) {
				t.Fatalf("nesting behavior changed: baseline=%v, guarded=%v", baselineErr, guardedErr)
			}
		})
	}
}

func TestOptions145PreservesHistoricalDefinitions(t *testing.T) {
	m, err := decode("exa.cortex_pb.PolicyGuardianConfig", options145Varint(14, 1))
	if err != nil {
		t.Fatalf("historical PolicyGuardianConfig.use_v2: %v", err)
	}
	f := m.ProtoReflect().Descriptor().Fields().ByNumber(14)
	if f == nil || f.Name() != "use_v2" || !m.ProtoReflect().Get(f).Bool() {
		t.Fatal("historical PolicyGuardianConfig field 14 was lost")
	}
}

func TestOptions145DoesNotQualifyUnrelatedAdditions(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		wire          []byte
	}{
		{"BASH language", "genai.CodeExecutionCallContent.CodeExecutionCallArguments", options145Varint(1, 2)},
		{"generic result_struct", "exa.cortex_pb.GenericStepResult", bytesField(10)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := decode(protoreflect.FullName(tc.message), tc.wire); err == nil {
				t.Fatal("unqualified addition accepted")
			}
		})
	}
}

func TestOptions145InvalidatesPreviousProfile(t *testing.T) {
	const previous = "sqlite-v1-format0-agy-1.3.4-5b93565c5f04-planner50-permission2-prompt23-tools143-permission1-tools144-skill1-history2-ide225"
	if Profile == previous {
		t.Fatal("option qualification did not change the profile")
	}
	s, _ := fixture(t)
	a, r := s.VerifyResponses([]byte(syntheticRaw), []byte(syntheticPresentation))
	if r.State != Verified {
		t.Fatal(r)
	}
	a.Profile = previous
	if r := s.CheckAttestation([]byte(syntheticPresentation), a); r.State != Unknown {
		t.Fatalf("previous profile accepted: %+v", r)
	}
}
