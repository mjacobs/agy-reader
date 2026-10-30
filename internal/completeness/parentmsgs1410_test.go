package completeness

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
	"google.golang.org/protobuf/proto"
)

// Synthetic regression records exercise loss detection for captured parent messages.
// All values are synthetic; capture text, paths, and IDs are never fixture material.
const parentmsgs1410Messages = `[{"text":"Synthetic parent instruction alpha.","media":[{"mimeType":"image/png","inlineData":"AQID","description":"first attachment","uri":"file:///fixture/synthetic/image.png"}]},{"text":"Synthetic parent instruction beta.","media":[{"mimeType":"text/plain","displayName":"context.txt","uri":"file:///fixture/synthetic/context.txt"}]},{"text":"Synthetic parent instruction gamma with no media."}]`

func parentmsgs1410Raw(checkpoint string) string {
	return `{"cascadeId":"parentmsgs1410-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_CHECKPOINT","checkpoint":{` + checkpoint + `}}]}`
}

func parentmsgs1410Change(t *testing.T, raw string, change func(map[string]any)) string {
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

func parentmsgs1410Checkpoint(value map[string]any) map[string]any {
	return value["steps"].([]any)[0].(map[string]any)["checkpoint"].(map[string]any)
}

func TestParentMsgs1410PreservesCapturedParentMessages(t *testing.T) {
	for name, payload := range map[string]string{
		"parent messages only": `"capturedParentMessages":` + parentmsgs1410Messages,
		"both user and parent": `"capturedUserRequests":[{"text":"synthetic user prompt"}],"capturedParentMessages":` + parentmsgs1410Messages,
		"empty parent message": `"capturedParentMessages":[{}]`,
	} {
		t.Run(name, func(t *testing.T) {
			raw := parentmsgs1410Raw(payload)
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
		})
	}
}

func TestParentMsgs1410OfflineJSONExport(t *testing.T) {
	raw := parentmsgs1410Raw(`"capturedParentMessages":` + parentmsgs1410Messages)
	s := options145Snapshot(t, raw)

	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatalf("OfflineJSON failed: %v", err)
	}
	if r := s.ComparePresentation(offline); r.State != Verified {
		t.Fatalf("offline comparison failed: %+v", r)
	}

	var value map[string]any
	if err := json.Unmarshal(offline, &value); err != nil {
		t.Fatalf("unmarshal offline JSON: %v", err)
	}
	cp := parentmsgs1410Checkpoint(value)
	cpm, ok := cp["capturedParentMessages"].([]any)
	if !ok || len(cpm) != 3 {
		t.Fatalf("expected 3 parent messages in offline JSON, got %v", cp["capturedParentMessages"])
	}
	msg0 := cpm[0].(map[string]any)
	if msg0["text"] != "Synthetic parent instruction alpha." {
		t.Fatalf("unexpected text: %v", msg0["text"])
	}
	media := msg0["media"].([]any)
	if len(media) != 1 || media[0].(map[string]any)["mimeType"] != "image/png" {
		t.Fatalf("unexpected media: %v", media)
	}
}

func TestParentMsgs1410RejectsChangedCapturedContent(t *testing.T) {
	raw := parentmsgs1410Raw(`"capturedParentMessages":` + parentmsgs1410Messages)
	s := options145Snapshot(t, raw)

	changes := map[string]func([]any){
		"message reordered": func(msgs []any) {
			msgs[0], msgs[1] = msgs[1], msgs[0]
		},
		"message text changed": func(msgs []any) {
			msgs[0].(map[string]any)["text"] = "Changed instruction text."
		},
		"message text removed": func(msgs []any) {
			delete(msgs[0].(map[string]any), "text")
		},
		"message removed": func(msgs []any) {
			msgs[0] = msgs[len(msgs)-1]
		},
		"media inline data changed": func(msgs []any) {
			msgs[0].(map[string]any)["media"].([]any)[0].(map[string]any)["inlineData"] = "AQIE"
		},
		"media uri changed": func(msgs []any) {
			msgs[0].(map[string]any)["media"].([]any)[0].(map[string]any)["uri"] = "file:///fixture/synthetic/changed.png"
		},
		"media description changed": func(msgs []any) {
			msgs[0].(map[string]any)["media"].([]any)[0].(map[string]any)["description"] = "changed description"
		},
		"all media removed": func(msgs []any) {
			delete(msgs[0].(map[string]any), "media")
		},
	}

	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			changed := parentmsgs1410Change(t, raw, func(v map[string]any) {
				change(parentmsgs1410Checkpoint(v)["capturedParentMessages"].([]any))
			})
			options145RequireBoth(t, s, raw, changed)
		})
	}

	t.Run("message added", func(t *testing.T) {
		added := parentmsgs1410Change(t, raw, func(v map[string]any) {
			msgs := parentmsgs1410Checkpoint(v)["capturedParentMessages"].([]any)
			parentmsgs1410Checkpoint(v)["capturedParentMessages"] = append(msgs, map[string]any{
				"text": "Unexpected added parent message.",
			})
		})
		options145RequireBoth(t, s, raw, added)
	})

	t.Run("field removed entirely", func(t *testing.T) {
		dropped := parentmsgs1410Change(t, raw, func(v map[string]any) {
			delete(parentmsgs1410Checkpoint(v), "capturedParentMessages")
		})
		options145RequireBoth(t, s, raw, dropped)
	})
}

func TestParentMsgs1410RejectsUnknownNestedWire(t *testing.T) {
	unknown := options145Varint(9999, 1)
	goodText := bytesField(1, []byte("synthetic parent instruction")...)
	for name, parentMsg := range map[string][]byte{
		"unknown field in parent message": append(append([]byte(nil), goodText...), unknown...),
		"unknown field in media":          append(append([]byte(nil), goodText...), bytesField(2, unknown...)...),
		"unknown media enum":              bytesField(2, options145Varint(16, 9999)...),
	} {
		t.Run(name, func(t *testing.T) {
			raw := parentmsgs1410Raw(`"capturedParentMessages":[{"text":"synthetic parent instruction"}]`)
			p := filepath.Join(t.TempDir(), "source.db")
			db := testfixture.Source(t, p, []byte(raw))
			step := testfixture.Message(t, "gemini_coder.Step", []byte(`{"type":"CORTEX_STEP_TYPE_CHECKPOINT"}`))
			checkpointField := step.ProtoReflect().Descriptor().Fields().ByName("checkpoint")
			control := append(testfixture.Binary(t, step.ProtoReflect()), bytesField(checkpointField.Number(), bytesField(31, goodText...)...)...)
			testfixture.Exec(t, db, "UPDATE steps SET step_payload=? WHERE idx=0", control)
			if _, err := ReadSnapshot(t.Context(), p); err != nil {
				t.Fatalf("valid persisted checkpoint control: %v", err)
			}
			body := append(testfixture.Binary(t, step.ProtoReflect()), bytesField(checkpointField.Number(), bytesField(31, parentMsg...)...)...)
			testfixture.Exec(t, db, "UPDATE steps SET step_payload=? WHERE idx=0", body)
			if _, err := ReadSnapshot(t.Context(), p); err == nil {
				t.Fatal("accepted unsupported nested parent message content")
			}
		})
	}
	valid := bytesField(31, goodText...)
	if _, err := decode("exa.cortex_pb.CortexStepCheckpoint", valid); err != nil {
		t.Fatalf("known parent message control: %v", err)
	}
	invalid := bytesField(31, append(goodText, unknown...)...)
	if _, err := decode("exa.cortex_pb.CortexStepCheckpoint", invalid); err == nil {
		t.Fatal("unknown nested field accepted in decode")
	}
}

func TestParentMsgs1410InvalidatesPreviousProfile(t *testing.T) {
	previous := strings.TrimSuffix(Profile, "-parentmsgs1410")
	if Profile == previous {
		t.Fatal("parentmsgs1410 did not change profile")
	}
	raw := parentmsgs1410Raw(`"capturedParentMessages":` + parentmsgs1410Messages)
	s := options145Snapshot(t, raw)
	a, r := s.VerifyResponses([]byte(raw), []byte(raw))
	if r.State != Verified {
		t.Fatalf("verification failed: %+v", r)
	}
	a.Profile = previous
	if r := s.CheckAttestation([]byte(raw), a); r.State != Unknown {
		t.Fatalf("accepted previous profile attestation: %+v", r)
	}
}
