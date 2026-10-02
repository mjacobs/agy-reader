package completeness

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// TestNativeEvidence consumes only an explicitly supplied disposable native
// fixture. It never discovers or contacts a daemon or opens a live source path.
func TestNativeEvidence(t *testing.T) {
	path := os.Getenv("AGY_READER_SCRATCH_DB")
	if path == "" {
		t.Skip("explicit isolated native fixture not supplied")
	}
	s, err := ReadSnapshot(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	read := func(key string) []byte {
		t.Helper()
		b, err := os.ReadFile(os.Getenv(key))
		if err != nil {
			t.Fatal(err)
		}
		var v struct {
			Trajectory json.RawMessage `json:"trajectory"`
		}
		if err = json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		return v.Trajectory
	}
	raw, presentation := read("AGY_READER_SCRATCH_RAW"), read("AGY_READER_SCRATCH_PRESENTATION")
	a, r := s.VerifyResponses(raw, presentation)
	if r.State != Verified {
		t.Fatalf("native verification: %+v", r)
	}
	if r = s.CheckAttestation(presentation, a); r.State != Verified {
		t.Fatalf("cached native verification: %+v", r)
	}
	t.Logf("verified %d native steps, source %s", s.StepCount, s.Digest)
}
