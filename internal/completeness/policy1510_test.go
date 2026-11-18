package completeness

import (
	"path/filepath"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
)

func TestPolicy1510RetainedAndCompared(t *testing.T) {
	raw := `{"cascadeId":"fixture","metadata":{},"steps":[],"executorMetadatas":[` +
		`{"cascadeConfig":{"executorConfig":{"commandAssessorConfig":{"policyGuardianConfig":` +
		`{"deployment":"fixture","enableIrreversibleVetting":false,"enableReversibilityAdjudication":true}` +
		`}}}}]}`
	path := filepath.Join(t.TempDir(), "source.db")
	testfixture.Source(t, path, []byte(raw))
	snapshot, err := ReadSnapshot(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	attestation, result := snapshot.VerifyResponses([]byte(raw), []byte(raw))
	if result.State != Verified || len(attestation.SourceOnly) != 0 {
		t.Fatalf("policy fields did not compare normally: %+v", result)
	}
	for _, field := range []string{`"deployment":"fixture",`, `"enableIrreversibleVetting":false,`, `,"enableReversibilityAdjudication":true`} {
		t.Run(field, func(t *testing.T) {
			changed := replace1216(t, raw, field, "")
			require1216(t, snapshot, changed, raw, Incomplete)
			require1216(t, snapshot, raw, changed, Incomplete)
		})
	}
	offline, err := snapshot.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, snapshot, raw, string(offline), Verified)
}

func TestPolicy1510RejectsUnqualifiedWireFields(t *testing.T) {
	// Only the observed fields are added. In particular, field 20 stays unknown.
	for _, data := range [][]byte{options145Varint(20, 1), options145Varint(15, 1), bytesField(18, 'x')} {
		if _, err := decode("exa.cortex_pb.PolicyGuardianConfig", data); err == nil {
			t.Fatal("unqualified policy wire field accepted")
		}
	}
}
