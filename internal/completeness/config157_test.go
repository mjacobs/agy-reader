package completeness

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
)

func TestTemplateOverrides157ComparedInGeneratorAndExecutor(t *testing.T) {
	planner := `{"promptSectionCustomizationConfig":{"templateOverrides":{"fixture":"fixture template"}}}`
	for _, metadata := range []string{`"generatorMetadata":[{"plannerConfig":` + planner + `}]`, `"executorMetadatas":[{"cascadeConfig":{"plannerConfig":` + planner + `}}]`} {
		raw := `{"cascadeId":"fixture","metadata":{},` + metadata + `,"steps":[]}`
		p := filepath.Join(t.TempDir(), "source.db")
		testfixture.Source(t, p, []byte(raw))
		snapshot, err := ReadSnapshot(t.Context(), p)
		if err != nil {
			t.Fatal(err)
		}
		if a, r := snapshot.VerifyResponses([]byte(raw), []byte(raw)); r.State != Verified || len(a.SourceOnly) != 0 {
			t.Fatalf("map did not compare normally: %+v", r)
		}
		changed := strings.Replace(raw, "fixture template", "changed template", 1)
		if r := snapshot.CompareFullRaw([]byte(changed)); r.State != Incomplete {
			t.Fatalf("altered map passed full comparison: %+v", r)
		}
		if r := snapshot.ComparePresentation([]byte(changed)); r.State != Incomplete {
			t.Fatalf("altered map passed presentation: %+v", r)
		}
	}
}

func TestTemplateOverrides157RejectsLossyMapWire(t *testing.T) {
	entry := append(bytesField(1, []byte("fixture")...), bytesField(2, []byte("template")...)...)
	valid := bytesField(5, entry...)
	if _, err := decode("exa.cortex_pb.PromptSectionCustomizationConfig", valid); err != nil {
		t.Fatal(err)
	}
	for name, wire := range map[string][]byte{
		"unknown entry field":  bytesField(5, append(append([]byte{}, entry...), options145Varint(99, 1)...)...),
		"duplicate map key":    append(append([]byte{}, valid...), valid...),
		"wrong value wire":     bytesField(5, append(bytesField(1, []byte("fixture")...), options145Varint(2, 1)...)...),
		"repeated value":       bytesField(5, append(append([]byte{}, entry...), bytesField(2, 'x')...)...),
		"unknown config field": append(append([]byte{}, valid...), options145Varint(99, 1)...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decode("exa.cortex_pb.PromptSectionCustomizationConfig", wire); err == nil {
				t.Fatal("accepted lossy map encoding")
			}
		})
	}
}

func TestPlanExecute157InvalidatesPreviousAttestations(t *testing.T) {
	previous := strings.Replace(Profile, "-planexecute157", "", 1)
	if previous == Profile {
		t.Fatal("plan/execute qualification did not advance profile")
	}
	snapshot, _ := fixture(t)
	attestation, result := snapshot.VerifyResponses([]byte(syntheticRaw), []byte(syntheticPresentation))
	if result.State != Verified {
		t.Fatal(result)
	}
	attestation.Profile = previous
	if result := snapshot.CheckAttestation([]byte(syntheticPresentation), attestation); result.State != Unknown {
		t.Fatalf("old profile accepted: %+v", result)
	}
}
