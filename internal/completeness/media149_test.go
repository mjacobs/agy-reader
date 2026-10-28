package completeness

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// These fixtures test retention and loss detection. They are not evidence that
// a native agy model produced a planner media response or a structured result.
func TestMedia149PlannerMediaRetained(t *testing.T) {
	const raw = `{"cascadeId":"media149-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_PLANNER_RESPONSE","status":"CORTEX_STEP_STATUS_DONE","plannerResponse":{"response":"Here is an image.","media":[{"mimeType":"image/png","inlineData":"AQID","description":"fixture image"},{"mimeType":"image/webp","uri":"file:///fixture/image.webp"}]}}]}`
	s := options145Snapshot(t, raw)
	a, r := s.VerifyResponses([]byte(raw), []byte(raw))
	if r.State != Verified || len(a.SourceOnly) != 0 {
		t.Fatalf("planner media not retained: %+v %+v", a, r)
	}
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	if r := s.ComparePresentation(offline); r.State != Verified {
		t.Fatalf("offline media lost: %+v", r)
	}
	for _, change := range [][2]string{
		{`"inlineData":"AQID"`, `"inlineData":"AQIE"`},
		{`,"description":"fixture image"`, ``},
		{`,{"mimeType":"image/webp","uri":"file:///fixture/image.webp"}`, ``},
	} {
		options145RequireBoth(t, s, raw, strings.Replace(raw, change[0], change[1], 1))
	}
	previous := strings.TrimSuffix(Profile, "-media149-tools149")
	a.Profile = previous
	if r := s.CheckAttestation([]byte(raw), a); r.State != Unknown {
		t.Fatalf("previous profile accepted: %+v", r)
	}
}

func TestMedia149StructuredResultDecodesWithoutQualifyingProjection(t *testing.T) {
	// genai.Value is an ordinary message, not google.protobuf.Value. Retain its
	// explicit string_value member and refuse to silently discard it when a
	// known tool is projected to its legacy presentation.
	raw := strings.Replace(finish147Raw, `"result":"fixture result"`, `"resultValue":{"stringValue":"fixture structured value"},"result":"fixture result"`, 1)
	s := options145Snapshot(t, raw)
	if r := s.CompareFullRaw([]byte(raw)); r.State != Verified {
		t.Fatalf("structured source did not decode: %+v", r)
	}
	changed := strings.Replace(raw, "fixture structured value", "changed", 1)
	if r := s.CompareFullRaw([]byte(changed)); r.State != Incomplete {
		t.Fatalf("changed structured value accepted: %+v", r)
	}
	if data, err := s.OfflineJSON(); err == nil || data != nil {
		t.Fatal("unqualified structured-result projection accepted")
	}
	if _, r := s.VerifyResponses([]byte(raw), []byte(finish147Shown)); r.State != Unknown {
		t.Fatalf("structured result silently lost: %+v", r)
	}
}

func TestMedia149RejectsUnknownNestedFields(t *testing.T) {
	unknown := options145Varint(9999, 1)
	for _, tc := range []struct {
		name   protoreflect.FullName
		number protoreflect.FieldNumber
		valid  []byte
	}{
		{"exa.cortex_pb.CortexStepPlannerResponse", 20, bytesField(1, []byte("image/png")...)},
		{"exa.cortex_pb.GenericStepResult", 11, bytesField(3, []byte("retained")...)},
	} {
		t.Run(string(tc.name), func(t *testing.T) {
			if _, err := decode(tc.name, bytesField(tc.number, tc.valid...)); err != nil {
				t.Fatalf("known field: %v", err)
			}
			if _, err := decode(tc.name, bytesField(tc.number, append(tc.valid, unknown...)...)); err == nil {
				t.Fatal("unknown nested field accepted")
			}
		})
	}
}
