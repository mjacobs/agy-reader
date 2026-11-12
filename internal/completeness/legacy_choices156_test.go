package completeness

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

const legacyChoiceTools = `{"invokeSubagent":{"modelOptions":{"options":[{"name":"fast","model":{"modelName":"fixture-model","thinkingLevel":2},"description":"fixture choice"}]}}}`

func dropLegacyChoiceFixture(raw string) string {
	return strings.NewReplacer(`"name":"fast",`, ``, `"model":{"modelName":"fixture-model","thinkingLevel":2},`, ``).Replace(raw)
}

func TestLegacyChoices156HistoricalAndCurrentRPC(t *testing.T) {
	for _, location := range []string{"generator", "executor"} {
		t.Run(location, func(t *testing.T) {
			raw := options145Raw(location, legacyChoiceTools)
			current := dropLegacyChoiceFixture(raw)
			snapshot := options145Snapshot(t, raw)
			before := proto.Clone(snapshot.trajectory)
			historical, result := snapshot.VerifyResponses([]byte(raw), []byte(raw))
			if result.State != Verified || historical.Reason != "" || len(historical.SourceOnly) != 0 {
				t.Fatalf("historical RPC: %+v %+v", result, historical)
			}
			a, result := snapshot.VerifyResponses([]byte(current), []byte(current))
			if result.State != Verified || result.Reason != SourceOnlyReason || a.Reason != SourceOnlyReason || len(a.SourceOnly) != 2 {
				t.Fatalf("current RPC: %+v %+v", result, a)
			}
			model := protowire.AppendString(protowire.AppendTag(nil, 1, protowire.BytesType), "fixture-model")
			model = protowire.AppendVarint(protowire.AppendTag(model, 2, protowire.VarintType), 2)
			for i, expected := range []struct {
				path    string
				number  protowire.Number
				payload []byte
			}{
				{SourceOnlyLegacyChoiceNamePath, 1, []byte("fast")},
				{SourceOnlyLegacyChoiceModelPath, 2, model},
			} {
				field := a.SourceOnly[i]
				encoded := protowire.AppendBytes(protowire.AppendTag(nil, expected.number, protowire.BytesType), expected.payload)
				digest := sha256.Sum256(encoded)
				if field.Path != expected.path || field.Count != 1 || field.SHA256 != hex.EncodeToString(digest[:]) || len(field.Occurrences) != 1 || !bytes.Equal(field.Occurrences[0].Payload, expected.payload) {
					t.Fatalf("source-only content differs: %+v", field)
				}
				if !strings.HasPrefix(field.Occurrences[0].Location, location+"_metadata") && location == "generator" {
					t.Fatal("wrong generator location", field.Occurrences[0].Location)
				}
				if !strings.HasPrefix(field.Occurrences[0].Location, "executor_metadatas[0].cascade_config.planner_config.") && location == "executor" {
					t.Fatal("wrong executor location", field.Occurrences[0].Location)
				}
			}
			encoded, err := json.Marshal(map[string]any{"agyReader": map[string]any{"completeness": a}})
			if err != nil {
				t.Fatal(err)
			}
			saved := ReadAttestation(encoded)
			if result = snapshot.CheckAttestation([]byte(current), saved); result.State != Verified || result.Reason != SourceOnlyReason {
				t.Fatalf("saved current attestation: %+v", result)
			}
			if result = snapshot.CheckAttestation([]byte(raw), historical); result.State != Verified || result.Reason != "" {
				t.Fatalf("historical attestation: %+v", result)
			}
			offline, err := snapshot.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			var exported struct {
				Reader struct {
					Offline struct {
						Fields []SourceOnlyField `json:"sourceOnlyFields"`
					} `json:"offline"`
				} `json:"agyReader"`
			}
			if err = json.Unmarshal(offline, &exported); err != nil {
				t.Fatal(err)
			}
			got, _ := json.Marshal(exported.Reader.Offline.Fields)
			want, _ := json.Marshal(a.SourceOnly)
			if !bytes.Equal(got, want) {
				t.Fatal("offline export lost source-only fields")
			}
			if result := snapshot.ComparePresentation(offline); result.State != Verified || result.Reason != SourceOnlyReason {
				t.Fatalf("offline presentation: %+v", result)
			}
			if !proto.Equal(before, snapshot.trajectory) || len(snapshot.sourceOnly) != 0 {
				t.Fatal("verification changed the historical snapshot")
			}
		})
	}
}

func TestLegacyChoices156RejectsLossAndForgedAttestations(t *testing.T) {
	raw := options145Raw("generator", legacyChoiceTools)
	current := dropLegacyChoiceFixture(raw)
	snapshot := options145Snapshot(t, raw)
	a, result := snapshot.VerifyResponses([]byte(current), []byte(current))
	if result.State != Verified {
		t.Fatal(result)
	}
	for name, changed := range map[string]string{
		"description changed":      strings.Replace(current, "fixture choice", "changed", 1),
		"description missing":      strings.Replace(current, `"description":"fixture choice"`, ``, 1),
		"partial name removal":     strings.Replace(raw, `"name":"fast",`, ``, 1),
		"partial model removal":    strings.Replace(raw, `"model":{"modelName":"fixture-model","thinkingLevel":2},`, ``, 1),
		"changed historical model": strings.Replace(raw, "fixture-model", "other-model", 1),
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, snapshot, changed, changed, Incomplete)
			require1216(t, snapshot, current, changed, Incomplete)
		})
	}
	for _, location := range []string{"static config", "user config"} {
		historical := options145Raw(location, legacyChoiceTools)
		s := options145Snapshot(t, historical)
		require1216(t, s, historical, historical, Verified)
		missing := dropLegacyChoiceFixture(historical)
		require1216(t, s, missing, missing, Incomplete)
	}
	for _, change := range []func(*Attestation){
		func(a *Attestation) { a.SourceOnly = nil; a.Reason = "" },
		func(a *Attestation) { a.SourceOnly = a.SourceOnly[:1] },
		func(a *Attestation) { a.SourceOnly[0].Path = "other" },
		func(a *Attestation) { a.SourceOnly[0].Count++ },
		func(a *Attestation) { a.SourceOnly[0].SHA256 = "changed" },
		func(a *Attestation) { a.SourceOnly[0].Occurrences[0].Location = "other" },
		func(a *Attestation) { a.SourceOnly[0].Occurrences[0].Payload = []byte("changed") },
	} {
		encoded, _ := json.Marshal(a)
		var bad Attestation
		if err := json.Unmarshal(encoded, &bad); err != nil {
			t.Fatal(err)
		}
		change(&bad)
		if r := snapshot.CheckAttestation([]byte(current), &bad); r.State != Incomplete {
			t.Fatalf("forged record accepted: %+v", r)
		}
	}
	changedSource := options145Snapshot(t, strings.Replace(raw, "fixture-model", "changed-model", 1))
	if changedSource.Digest == snapshot.Digest {
		t.Fatal("source digest ignored a legacy model")
	}
	if r := changedSource.CheckAttestation([]byte(current), a); r.State != Incomplete {
		t.Fatalf("changed source accepted: %+v", r)
	}
	for _, malformed := range []string{
		strings.Replace(raw, `"name":"fast",`, ``, 1),
		strings.Replace(raw, `"model":{"modelName":"fixture-model","thinkingLevel":2},`, ``, 1),
		strings.Replace(raw, `"name":"fast"`, `"name":""`, 1),
		strings.Replace(raw, `"modelName":"fixture-model"`, `"modelName":""`, 1),
		strings.Replace(raw, `"description":"fixture choice"`, `"description":"fixture choice","inherit":{}`, 1),
	} {
		require1216(t, options145Snapshot(t, malformed), current, current, Incomplete)
	}
	// Unknown persisted fields inside a legacy value cannot be hidden by
	// the source-only declaration.
	message := trajectory1216(t, raw)
	generator := tool143Get(message.ProtoReflect(), "generator_metadata").List().Get(0).Message()
	for _, field := range []protoreflect.Name{"planner_config", "tool_config", "invoke_subagent", "model_options"} {
		generator = tool143Get(generator, field).Message()
	}
	option := tool143Get(generator, "options").List().Get(0).Message()
	model := tool143Get(option, "model").Message()
	model.SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 999, protowire.VarintType), 1))
	if _, err := finishSnapshot(message); err == nil {
		t.Fatal("unknown persisted legacy model field accepted")
	}
}

func TestLegacyChoices156PreservesNewKinds(t *testing.T) {
	tools := strings.Replace(legacyChoiceTools, `"description":"fixture choice"}]`, `"description":"fixture choice"},{"namedModel":{"name":"new","model":{"modelName":"new-model"}}}]`, 1)
	raw := options145Raw("executor", tools)
	current := dropLegacyChoiceFixture(raw)
	snapshot := options145Snapshot(t, raw)
	require1216(t, snapshot, current, current, Verified)
	changed := strings.Replace(current, "new-model", "changed-model", 1)
	require1216(t, snapshot, changed, changed, Incomplete)
}

func TestLegacyChoices156DescriptorProvenance(t *testing.T) {
	read := func(path, digest string) *descriptorpb.DescriptorProto {
		t.Helper()
		data, err := os.ReadFile("../../artifacts/proto-snapshots/" + path + "/closure.protoset")
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != digest {
			t.Fatal("snapshot changed")
		}
		var set descriptorpb.FileDescriptorSet
		if err := proto.Unmarshal(data, &set); err != nil {
			t.Fatal(err)
		}
		for _, file := range set.File {
			if file.GetPackage() != "exa.cortex_pb" {
				continue
			}
			for _, message := range file.MessageType {
				if message.GetName() == "SubagentModelOption" {
					return message
				}
			}
		}
		t.Fatal("model option descriptor missing")
		return nil
	}
	old := read("agy-1.5.0-b9043860a06a", "817e08196b0d1e8bc5c206153e1755722eed24d69c6658fcacd6e066e4614bf8")
	if old.Field[0].GetName() != "name" || old.Field[0].GetNumber() != 1 || old.Field[0].GetType() != descriptorpb.FieldDescriptorProto_TYPE_STRING || old.Field[1].GetName() != "model" || old.Field[1].GetNumber() != 2 || old.Field[1].GetTypeName() != ".exa.cortex_pb.ModelCatalogKey" {
		t.Fatal("historical model field definitions changed")
	}
	for _, current := range []*descriptorpb.DescriptorProto{
		read("agy-1.5.6-7e5bbdcbf689", "e44e34f43926494831dd395d43d51c3afaf45e596ec71850fb980ef754880346"),
		read("agy-1.5.7-5df525a2d690", "f3de51c7f2a7317d073cabcea13cd42a7f7f1adf3986c3bf5b3f07035b120d46"),
	} {
		for _, number := range []int32{1, 2} {
			reserved := false
			for _, interval := range current.ReservedRange {
				reserved = reserved || interval.GetStart() <= number && number < interval.GetEnd()
			}
			if !reserved {
				t.Fatal("retired model number is not reserved", number)
			}
		}
		for _, name := range []string{"name", "model"} {
			reserved := false
			for _, value := range current.ReservedName {
				reserved = reserved || value == name
			}
			if !reserved {
				t.Fatal("retired model name is not reserved", name)
			}
		}

	}
}
