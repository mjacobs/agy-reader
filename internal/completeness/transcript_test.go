package completeness

import (
	"github.com/mjacobs/agy-reader/internal/testfixture"
	"path/filepath"
	"testing"
)

func TestTranscriptScope(t *testing.T) {
	p := filepath.Join(t.TempDir(), "source.db")
	db := testfixture.Source(t, p, []byte(syntheticRaw))
	testfixture.Exec(t, db, "INSERT INTO executor_metadata(idx,data) VALUES (0, zeroblob(67108865))")
	s, err := ReadSyncSnapshot(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if s.OmittedExecutors != 1 {
		t.Fatal(s.OmittedExecutors)
	}
	a, r := s.VerifyResponses([]byte(syntheticRaw), []byte(syntheticPresentation))
	if r.State != Verified || r.Reason != TranscriptReason || a.Representation != TranscriptRepresentation {
		t.Fatal(r)
	}
	if s.CheckAttestation([]byte(syntheticPresentation), a).Reason != TranscriptReason {
		t.Fatal("scope lost on recheck")
	}
	a.OmittedExecutors = 2
	if s.CheckAttestation([]byte(syntheticPresentation), a).State == Verified {
		t.Fatal("accepted changed omission count")
	}
}
