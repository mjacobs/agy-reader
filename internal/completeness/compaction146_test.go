package completeness

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Synthetic regression records exercise loss detection. Native qualification
// separately uses agy-written SQLite and independent full/default RPC captures.
const compaction146Captured = `[{"text":"first request","media":[{"mimeType":"image/png","inlineData":"AQID","description":"inline sample","uri":"https://example.invalid/media","thumbnail":"BAU=","durationSeconds":1.5,"mediaId":"sample-one","displayName":"sample","startPage":0,"endPage":2,"startTimeMs":"10","endTimeMs":"20","fps":24,"mediaResolution":"MEDIA_RESOLUTION_HIGH","contentLayers":["VIDEO_CONTENT_LAYER_FRAMES","VIDEO_CONTENT_LAYER_AUDIO"],"ingestionOptions":{"@type":"type.googleapis.com/exa.codeium_common_pb.Blobref","blobId":"options-example"}},{"mimeType":"image/png","blobref":{"blobId":"sample-blob"}}]},{"text":"second request","media":[{"mimeType":"text/plain","inlineData":"Bg=="}]}]`

func compaction146Raw(checkpoint string) string {
	return `{"cascadeId":"compaction146-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_CHECKPOINT","checkpoint":{` + checkpoint + `}}]}`
}

func compaction146Change(t *testing.T, raw string, change func(map[string]any)) string {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatal(err)
	}
	change(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func compaction146Checkpoint(value map[string]any) map[string]any {
	return value["steps"].([]any)[0].(map[string]any)["checkpoint"].(map[string]any)
}

func TestCompaction146PreservesHistoricalAndCapturedRequests(t *testing.T) {
	for name, payload := range map[string]string{
		"historical":    `"userRequests":["old first","old second"]`,
		"captured":      `"capturedUserRequests":` + compaction146Captured,
		"both":          `"userRequests":["old first","old second"],"capturedUserRequests":` + compaction146Captured,
		"empty request": `"capturedUserRequests":[{}]`,
	} {
		t.Run(name, func(t *testing.T) {
			raw := compaction146Raw(payload)
			s := options145Snapshot(t, raw)
			before := proto.Clone(s.trajectory)
			a, r := s.VerifyResponses([]byte(raw), []byte(raw))
			if r.State != Verified || len(a.SourceOnly) != 0 {
				t.Fatalf("not exactly retained: %+v %+v", a, r)
			}
			if !proto.Equal(before, s.trajectory) {
				t.Fatal("verification modified source")
			}
			if r := s.CheckAttestation([]byte(raw), a); r.State != Verified {
				t.Fatal(r)
			}
			for _, key := range []string{"userRequests", "capturedUserRequests"} {
				if !strings.Contains(payload, `"`+key+`"`) {
					continue
				}
				t.Run("drop "+key, func(t *testing.T) {
					changed := compaction146Change(t, raw, func(v map[string]any) { delete(compaction146Checkpoint(v), key) })
					options145RequireBoth(t, s, raw, changed)
				})
			}
		})
	}
}

func TestCompaction146RejectsChangedCapturedContent(t *testing.T) {
	raw := compaction146Raw(`"capturedUserRequests":` + compaction146Captured)
	s := options145Snapshot(t, raw)
	changes := map[string]func([]any){
		"request order":     func(r []any) { r[0], r[1] = r[1], r[0] },
		"text changed":      func(r []any) { r[0].(map[string]any)["text"] = "changed" },
		"text removed":      func(r []any) { delete(r[0].(map[string]any), "text") },
		"all media removed": func(r []any) { delete(r[0].(map[string]any), "media") },
		"media removed":     func(r []any) { v := r[0].(map[string]any); v["media"] = v["media"].([]any)[1:] },
		"media order":       func(r []any) { m := r[0].(map[string]any)["media"].([]any); m[0], m[1] = m[1], m[0] },
		"blob changed": func(r []any) {
			r[0].(map[string]any)["media"].([]any)[1].(map[string]any)["blobref"] = map[string]any{"blobId": "different-blob"}
		},
		"Any changed": func(r []any) {
			r[0].(map[string]any)["media"].([]any)[0].(map[string]any)["ingestionOptions"].(map[string]any)["blobId"] = "changed"
		},
	}
	for _, key := range []string{"mimeType", "inlineData", "description", "uri", "thumbnail", "durationSeconds", "mediaId", "displayName", "startPage", "endPage", "startTimeMs", "endTimeMs", "fps", "mediaResolution", "contentLayers", "ingestionOptions"} {
		changes[key+" removed"] = func(r []any) { delete(r[0].(map[string]any)["media"].([]any)[0].(map[string]any), key) }
	}
	changes["inline changed"] = func(r []any) { r[0].(map[string]any)["media"].([]any)[0].(map[string]any)["inlineData"] = "AQIE" }
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			changed := compaction146Change(t, raw, func(v map[string]any) { change(compaction146Checkpoint(v)["capturedUserRequests"].([]any)) })
			options145RequireBoth(t, s, raw, changed)
		})
	}
}

func TestCompaction146OptionalConfigurationPresence(t *testing.T) {
	for _, field := range []string{"contextWindow", "maxUserRequestMediaTokens"} {
		values := []string{`""`, `"128k"`}
		if field == "maxUserRequestMediaTokens" {
			values = []string{"0", "2048", "-1"}
		}
		for _, value := range values {
			t.Run(field+"="+value, func(t *testing.T) {
				raw := strings.TrimSuffix(compaction146Raw(`"capturedUserRequests":`+compaction146Captured), "}") + `,"executorMetadatas":[{"cascadeConfig":{"checkpointConfig":{"` + field + `":` + value + `}}}]}`
				absent := strings.Replace(raw, `"`+field+`":`+value, "", 1)
				presentSource, absentSource := options145Snapshot(t, raw), options145Snapshot(t, absent)
				if presentSource.Digest == absentSource.Digest {
					t.Fatal("explicit option and absent option have same source digest")
				}
				for _, pair := range []struct {
					s        *Snapshot
					raw, bad string
				}{{presentSource, raw, absent}, {absentSource, absent, raw}} {
					a, r := pair.s.VerifyResponses([]byte(pair.raw), []byte(pair.raw))
					if r.State != Verified || len(a.SourceOnly) != 0 {
						t.Fatalf("option not retained: %+v", r)
					}
					options145RequireBoth(t, pair.s, pair.raw, pair.bad)
				}
			})
		}
	}
}

func TestCompaction146RejectsUnknownNestedWireAndAny(t *testing.T) {
	unknown := options145Varint(9999, 1)
	anyPayload := func(name string, payload []byte) []byte {
		return append(bytesField(1, []byte("type.googleapis.com/"+name)...), bytesField(2, payload...)...)
	}
	goodText := bytesField(1, []byte("first request")...)
	for name, request := range map[string][]byte{
		"captured request":             append(append([]byte(nil), goodText...), unknown...),
		"media":                        append(append([]byte(nil), goodText...), bytesField(2, unknown...)...),
		"blobref":                      bytesField(2, bytesField(3, unknown...)...),
		"unknown media enum":           bytesField(2, options145Varint(16, 9999)...),
		"Any envelope":                 bytesField(2, bytesField(10, unknown...)...),
		"unknown Any type":             bytesField(2, bytesField(10, anyPayload("example.Unsupported", nil)...)...),
		"unknown Any payload":          bytesField(2, bytesField(10, anyPayload("exa.codeium_common_pb.Blobref", unknown)...)...),
		"source-only field inside Any": bytesField(2, bytesField(10, anyPayload("exa.cortex_pb.CascadePlannerConfig", bytesField(50))...)...),
	} {
		t.Run(name, func(t *testing.T) {
			// Exercise the persisted SQLite entry point, not just standalone decode.
			raw := compaction146Raw(`"capturedUserRequests":[{"text":"first request"}]`)
			p := filepath.Join(t.TempDir(), "source.db")
			db := testfixture.Source(t, p, []byte(raw))
			step := testfixture.Message(t, "gemini_coder.Step", []byte(`{"type":"CORTEX_STEP_TYPE_CHECKPOINT"}`))
			checkpointField := step.ProtoReflect().Descriptor().Fields().ByName("checkpoint")
			control := append(testfixture.Binary(t, step.ProtoReflect()), bytesField(checkpointField.Number(), bytesField(30, goodText...)...)...)
			testfixture.Exec(t, db, "UPDATE steps SET step_payload=? WHERE idx=0", control)
			if _, err := ReadSnapshot(t.Context(), p); err != nil {
				t.Fatalf("valid persisted checkpoint control: %v", err)
			}
			body := append(testfixture.Binary(t, step.ProtoReflect()), bytesField(checkpointField.Number(), bytesField(30, request...)...)...)
			testfixture.Exec(t, db, "UPDATE steps SET step_payload=? WHERE idx=0", body)
			if _, err := ReadSnapshot(t.Context(), p); err == nil {
				t.Fatal("accepted unsupported nested checkpoint content")
			}
		})
	}
	valid := bytesField(30, append(goodText, bytesField(2, bytesField(10, anyPayload("exa.codeium_common_pb.Blobref", bytesField(1, []byte("safe")...))...)...)...)...)
	if _, err := decode("exa.cortex_pb.CortexStepCheckpoint", valid); err != nil {
		t.Fatalf("known Any control: %v", err)
	}
}

func TestCompaction146InvalidatesPreviousAttestation(t *testing.T) {
	const previous = "sqlite-v1-format0-agy-1.3.4-5b93565c5f04-planner50-permission2-prompt23-tools143-permission1-tools144-skill1-history2-ide225-options145"
	if Profile == previous {
		t.Fatal("checkpoint qualification did not change profile")
	}
	raw := compaction146Raw(`"userRequests":["historical request"]`)
	s := options145Snapshot(t, raw)
	a, r := s.VerifyResponses([]byte(raw), []byte(raw))
	if r.State != Verified {
		t.Fatal(r)
	}
	a.Profile = previous
	if r := s.CheckAttestation([]byte(raw), a); r.State != Unknown {
		t.Fatalf("accepted previous attestation: %+v", r)
	}
}

func TestCompaction146DoesNotQualifyUnrelatedFields(t *testing.T) {
	if _, err := decode(protoreflect.FullName("exa.cortex_pb.GenericStepResult"), bytesField(10)); err == nil {
		t.Fatal("unqualified generic result accepted")
	}
}

func compaction146EnablementRaw(entries string, generator bool) string {
	planner := `{"customizationDiscoveryConfig":{"kindEnablement":{` + entries + `}}}`
	extra := `,"executorMetadatas":[{"cascadeConfig":{"plannerConfig":` + planner + `}}]`
	if generator {
		extra = `,"generatorMetadata":[{"plannerConfig":` + planner + `}]`
	}
	return strings.TrimSuffix(compaction146Raw(`"userRequests":["fixture"]`), "}") + extra + "}"
}

func TestCompaction146KindEnablementRetained(t *testing.T) {
	for _, generator := range []bool{false, true} {
		name := "executor"
		if generator {
			name = "generator"
		}
		t.Run(name, func(t *testing.T) {
			raw := compaction146EnablementRaw(`"skill":true,"flow":false`, generator)
			s := options145Snapshot(t, raw)
			a, r := s.VerifyResponses([]byte(raw), []byte(raw))
			if r.State != Verified || len(a.SourceOnly) != 0 {
				t.Fatalf("enablement map not retained: %+v", r)
			}
			for name, change := range map[string][2]string{
				"true changed":  {`"skill":true`, `"skill":false`},
				"false changed": {`"flow":false`, `"flow":true`},
				"false removed": {`,"flow":false`, ``},
				"key changed":   {`"skill":true`, `"other":true`},
			} {
				t.Run(name, func(t *testing.T) { options145RequireBoth(t, s, raw, strings.Replace(raw, change[0], change[1], 1)) })
			}
		})
	}
}

func compaction146EnablementEntry(key string, enabled uint64) []byte {
	return append(bytesField(1, []byte(key)...), options145Varint(2, enabled)...)
}

func compaction146EnablementExecutor(entries []byte) []byte {
	return bytesField(10, bytesField(1, bytesField(57, entries...)...)...)
}

func TestCompaction146KindEnablementRejectsMalformedWire(t *testing.T) {
	entry := compaction146EnablementEntry("skill", 1)
	valid := bytesField(12, entry...)
	for name, entries := range map[string][]byte{
		"unknown member":         bytesField(12, append(append([]byte(nil), entry...), options145Varint(99, 1)...)...),
		"wrong map wire":         options145Varint(12, 1),
		"wrong key wire":         bytesField(12, append(options145Varint(1, 1), options145Varint(2, 1)...)...),
		"wrong value wire":       bytesField(12, append(bytesField(1, []byte("skill")...), bytesField(2, 'x')...)...),
		"duplicate key member":   bytesField(12, append(append([]byte(nil), entry...), bytesField(1, []byte("other")...)...)...),
		"duplicate value member": bytesField(12, append(append([]byte(nil), entry...), options145Varint(2, 0)...)...),
		"duplicate wrong key":    bytesField(12, append(append([]byte(nil), entry...), options145Varint(1, 1)...)...),
		"duplicate wrong value":  bytesField(12, append(append([]byte(nil), entry...), bytesField(2, 'x')...)...),
		"duplicate map key":      append(append([]byte(nil), valid...), valid...),
		"duplicate default key":  append(bytesField(12), bytesField(12, bytesField(1)...)...),
		"truncated entry":        {0x62, 0x7f},
		"truncated key":          bytesField(12, 0x0a, 0x7f),
		"truncated value":        bytesField(12, 0x10, 0x80),
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "source.db")
			db := testfixture.Source(t, p, []byte(compaction146EnablementRaw(`"skill":true`, false)))
			testfixture.Exec(t, db, "UPDATE executor_metadata SET data=? WHERE idx=0", compaction146EnablementExecutor(valid))
			if _, err := ReadSnapshot(t.Context(), p); err != nil {
				t.Fatalf("valid control: %v", err)
			}
			testfixture.Exec(t, db, "UPDATE executor_metadata SET data=? WHERE idx=0", compaction146EnablementExecutor(entries))
			if _, err := ReadSnapshot(t.Context(), p); err == nil {
				t.Fatal("accepted malformed persisted kind enablement map")
			}
		})
	}
}

func TestCompaction146KindEnablementDefaultMembers(t *testing.T) {
	for name, tc := range map[string]struct {
		entry []byte
		json  string
	}{
		"both absent":  {nil, `"":false`},
		"value absent": {bytesField(1, []byte("skill")...), `"skill":false`},
		"key absent":   {options145Varint(2, 1), `"":true`},
	} {
		t.Run(name, func(t *testing.T) {
			raw := compaction146EnablementRaw(tc.json, false)
			p := filepath.Join(t.TempDir(), "source.db")
			db := testfixture.Source(t, p, []byte(raw))
			testfixture.Exec(t, db, "UPDATE executor_metadata SET data=? WHERE idx=0", compaction146EnablementExecutor(bytesField(12, tc.entry...)))
			s, err := ReadSnapshot(t.Context(), p)
			if err != nil {
				t.Fatal(err)
			}
			if _, r := s.VerifyResponses([]byte(raw), []byte(raw)); r.State != Verified {
				t.Fatal(r)
			}
		})
	}
}

func TestCompaction146KindEnablementAcrossMergedFragments(t *testing.T) {
	fragments := func(split int, left, right []byte) []byte {
		path := []protoreflect.FieldNumber{10, 1, 57}
		for i := len(path) - 1; i >= split; i-- {
			left, right = bytesField(path[i], left...), bytesField(path[i], right...)
		}
		joined := append(left, right...)
		for i := split - 1; i >= 0; i-- {
			joined = bytesField(path[i], joined...)
		}
		return joined
	}
	for split, name := range []string{"cascade config", "planner config", "discovery config"} {
		t.Run(name, func(t *testing.T) {
			raw := compaction146EnablementRaw(`"skill":true,"flow":false`, false)
			p := filepath.Join(t.TempDir(), "source.db")
			db := testfixture.Source(t, p, []byte(raw))
			left := bytesField(12, compaction146EnablementEntry("skill", 1)...)
			right := bytesField(12, compaction146EnablementEntry("flow", 0)...)
			testfixture.Exec(t, db, "UPDATE executor_metadata SET data=? WHERE idx=0", fragments(split, left, right))
			s, err := ReadSnapshot(t.Context(), p)
			if err != nil {
				t.Fatalf("distinct keys in merged fragments: %v", err)
			}
			if _, r := s.VerifyResponses([]byte(raw), []byte(raw)); r.State != Verified {
				t.Fatal(r)
			}
			testfixture.Exec(t, db, "UPDATE executor_metadata SET data=? WHERE idx=0", fragments(split, left, left))
			if _, err := ReadSnapshot(t.Context(), p); err == nil {
				t.Fatal("duplicate map key accepted across merged fragments")
			}
		})
	}
}
