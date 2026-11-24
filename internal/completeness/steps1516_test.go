package completeness

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestRelease1516TranscriptFields(t *testing.T) {
	step := bytesField(16, []byte("inspect scratch file")...)
	document := append(bytesField(6, []byte("application/pdf")...), bytesField(16, []byte("report.pdf")...)...)
	content := bytesField(13, document...)
	documentStep := bytesField(12, bytesField(2, content...)...)
	rows := []struct {
		message protoreflect.FullName
		field   protowire.Number
		payload []byte
		want    string
	}{
		{"exa.chat_pb.ChatMessagePrompt", 28, step, `{"steps":[{"description":"inspect scratch file"}]}`},
		{"exa.chat_pb.ChatMessagePrompt", 28, documentStep, `{"steps":[{"content":{"content":[{"document":{"mimeTypeString":"application/pdf","name":"report.pdf"}}]}}]}`},
		{"exa.cortex_pb.CortexStepPlannerResponse", 21, step, `{"steps":[{"description":"inspect scratch file"}]}`},
		{"exa.cortex_pb.PolicyGuardianConfig", 25, bytesField(5, options145Varint(1, 2)...), `{"coverage":{"controlFileWrite":{"guardian":"MODE_SHADOW"}}}`},
	}
	for _, row := range rows {
		t.Run(string(row.message), func(t *testing.T) {
			msg, err := decode(row.message, bytesField(row.field, row.payload...))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := protojson.Marshal(msg)
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(row.want), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("decoded %s, want %s", raw, row.want)
			}
			if _, err := decode(row.message, bytesField(row.field, options145Varint(99, 1)...)); err == nil {
				t.Fatal("unknown nested field accepted")
			}
		})
	}
}

func TestChecklist1516(t *testing.T) {
	const raw = `{"cascadeId":"fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"update_steps","argumentsJson":"{\"steps\":[{\"title\":\"Read file\",\"status\":\"done\"}]}"}},"generic":{"args":{"steps":"[{\"status\":\"done\",\"title\":\"Read file\"}]"},"result":{"result":"Created At: 2026-01-01T00:00:00Z\nCompleted At: 2026-01-01T00:00:01Z\nOK","fullOutputUri":"file:///fixture/output.txt","stepRenderInfo":{"hidden":true,"title":"1 step","markdown":"- [x] Read file\n"}}}}]}`
	require1216(t, snapshot1216(t, raw), raw, raw, Verified)
	for _, change := range [][2]string{{"done", "unknown"}, {"true", "false"}, {"\\nOK", "\\nfailed"}} {
		changed := replace1216(t, raw, change[0], change[1])
		require1216(t, snapshot1216(t, changed), changed, changed, Unknown)
	}
	conflicting := strings.Replace(raw, `"argumentsJson":"{\"steps\":`, `"argumentsJson":"{\"steps\":[],\"steps\":`, 1)
	if conflicting == raw {
		t.Fatal("fixture lacks checklist arguments")
	}
	require1216(t, snapshot1216(t, conflicting), conflicting, conflicting, Unknown)
	changed := replace1216(t, raw, "1 step", "different title")
	require1216(t, snapshot1216(t, raw), raw, changed, Incomplete)
}

func TestStructuredMetadataMapUsesWireValidation(t *testing.T) {
	wire, checked := qualifiedMapValueWire("google.protobuf.Struct.fields")
	if !checked || wire != protowire.BytesType {
		t.Fatal("structured citation metadata must validate map entries before decoding")
	}
}

func TestStructuredStepRejectsConflictingVariants(t *testing.T) {
	step := append(bytesField(6), bytesField(12)...)
	if _, err := decode("genai.Step", step); err == nil {
		t.Fatal("accepted conflicting structured variants")
	}
}

func TestStructuredLanguageQualification(t *testing.T) {
	for _, name := range []protoreflect.FullName{"genai.CodeExecutionCallContent.CodeExecutionCallArguments", "genai.CodeExecutionCallStep.CodeExecutionCallStepArguments"} {
		if _, err := decode(name, options145Varint(1, 2)); err == nil {
			t.Errorf("%s accepted BASH", name)
		}
	}
}
