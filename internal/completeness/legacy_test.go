package completeness

import (
	"context"
	"github.com/mjacobs/agy-reader/internal/testfixture"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
	"testing"
)

const legacyFixture = `{"cascadeId":"11111111-1111-4111-8111-111111111111","steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":"legacy fixture"}}]}`

func TestLegacySnapshotAttestation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.pb")
	testfixture.LegacySource(t, path, []byte(legacyFixture))
	s, err := ReadSnapshot(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	a, r := s.VerifyResponses([]byte(legacyFixture), []byte(legacyFixture))
	if r.State != Verified || a.Profile != LegacyProfile || s.StepCount != 1 {
		t.Fatalf("verification: %+v %+v", r, a)
	}
	if r = s.CheckAttestation([]byte(legacyFixture), a); r.State != Verified {
		t.Fatal(r)
	}
	if err := os.WriteFile(path[:len(path)-3]+".db", []byte("new source"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSnapshot(t.Context(), path); err == nil {
		t.Fatal("legacy source shadowed by SQLite accepted")
	}
	if err := os.Remove(path[:len(path)-3] + ".db"); err != nil {
		t.Fatal(err)
	}
	a.Profile = Profile
	if r = s.CheckAttestation([]byte(legacyFixture), a); r.State != Unknown {
		t.Fatal("SQLite profile accepted for legacy")
	}
	a.Profile = LegacyProfile
	testfixture.LegacySource(t, path, []byte(legacyFixture)) // different nonce, same protobuf
	changed, err := ReadSnapshot(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Digest == s.Digest || changed.CheckAttestation([]byte(legacyFixture), a).State != Incomplete {
		t.Fatal("rewritten ciphertext retained old evidence")
	}
	if _, r = s.VerifyResponses([]byte(`{"cascadeId":"wrong"}`), []byte(legacyFixture)); r.State != Incomplete {
		t.Fatal("full response mismatch accepted")
	}
	if _, r = s.VerifyResponses([]byte(legacyFixture), []byte(`{"cascadeId":"11111111-1111-4111-8111-111111111111"}`)); r.State != Incomplete {
		t.Fatal("dropped presentation step accepted")
	}
}

func TestLegacyRejectsUnsupportedSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.pb")
	testfixture.LegacySource(t, path, []byte(legacyFixture))
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"empty", nil}, {"short", original[:27]}, {"truncated", original[:len(original)-1]},
		{"plaintext", []byte(legacyFixture)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadSnapshot(t.Context(), path); !IsUnsupported(err) {
				t.Fatalf("got %v", err)
			}
		})
	}
	for _, pos := range []int{0, 12, len(original) - 1} {
		data := append([]byte(nil), original...)
		data[pos] ^= 1
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadSnapshot(t.Context(), path); !IsUnsupported(err) {
			t.Fatalf("tamper %d: %v", pos, err)
		}
	}
	m := testfixture.Message(t, "gemini_coder.Trajectory", []byte(legacyFixture))
	plain, err := proto.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	plain = protowire.AppendTag(plain, 19000, protowire.VarintType)
	plain = protowire.AppendVarint(plain, 1)
	testfixture.LegacyBytes(t, path, plain)
	if _, err := ReadSnapshot(t.Context(), path); !IsUnsupported(err) {
		t.Fatalf("unknown field: %v", err)
	}
	testfixture.LegacyBytes(t, path, []byte{0xff})
	if _, err := ReadSnapshot(t.Context(), path); !IsUnsupported(err) {
		t.Fatalf("invalid protobuf: %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(maxBytes + 29); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := ReadSnapshot(t.Context(), path); !IsUnsupported(err) {
		t.Fatalf("size limit: %v", err)
	}
	if _, err := ReadSnapshot(t.Context(), path+"missing.pb"); err == nil || IsUnsupported(err) {
		t.Fatalf("missing source classified as unsupported: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := ReadSnapshot(ctx, path); err != context.Canceled {
		t.Fatalf("canceled: %v", err)
	}
}
