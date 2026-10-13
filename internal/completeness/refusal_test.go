package completeness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
)

// Every way the profile refuses a source is an UnsupportedError, and the
// message is unchanged by the classification.
func TestReadSnapshotRefusalIsUnsupported(t *testing.T) {
	for name, tc := range map[string]struct{ query, want string }{
		"version":          {"PRAGMA user_version=2", "unsupported SQLite source version"},
		"format":           {"UPDATE steps SET step_format=1", "unsupported step format"},
		"generated column": {"ALTER TABLE steps ADD COLUMN future TEXT GENERATED ALWAYS AS ('unknown') VIRTUAL", "unsupported SQLite source columns"},
		"extra table":      {"CREATE TABLE future (x)", "unsupported SQLite source tables"},
		"gap":              {"UPDATE steps SET idx=2", "unsupported step indexing"},
		"header":           {"UPDATE steps SET status=123456", ""},
		"render":           {"UPDATE steps SET render_info=X'01'", ""},
		"subtrajectory":    {"UPDATE steps SET has_subtrajectory=1", "unsupported decomposed step representation"},
		"null step type":   {"UPDATE steps SET step_type=NULL", ""},
		"payload":          {"UPDATE steps SET step_payload=X'FFFF'", "invalid persisted protobuf"},
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "s.db")
			db := testfixture.Source(t, p, []byte(syntheticRaw))
			testfixture.Exec(t, db, tc.query)
			_, err := ReadSnapshot(t.Context(), p)
			if err == nil || !IsUnsupported(err) {
				t.Fatalf("refused source not classified unsupported: %v", err)
			}
			if tc.want != "" && err.Error() != tc.want {
				t.Fatalf("message changed: %q, want %q", err.Error(), tc.want)
			}
		})
	}
	// A file that is not a SQLite database is outside the sqlite-v1 profile.
	p := filepath.Join(t.TempDir(), "not.db")
	if err := os.WriteFile(p, []byte("not-a-db"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSnapshot(t.Context(), p); err == nil || !IsUnsupported(err) {
		t.Fatalf("non-SQLite file not classified unsupported: %v", err)
	}
}

// A supported source that cannot be read now is not a refusal: an exclusive
// lock past the busy timeout (SQLITE_BUSY), or a source that does not exist.
func TestReadSnapshotTemporaryLockIsNotUnsupported(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	db := testfixture.Source(t, p, []byte(syntheticRaw))
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, q := range []string{"PRAGMA journal_mode=DELETE", "BEGIN EXCLUSIVE"} {
		if _, err = conn.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	_, err = ReadSnapshot(t.Context(), p)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "locked") {
		t.Fatalf("fixture did not produce a SQLite lock: %v", err)
	}
	if IsUnsupported(err) {
		t.Fatalf("temporary lock classified unsupported: %v", err)
	}
	if _, err = conn.ExecContext(t.Context(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadSnapshot(t.Context(), p); err != nil {
		t.Fatalf("source unreadable after the lock cleared: %v", err)
	}

	missing := filepath.Join(t.TempDir(), "missing.db")
	if _, err = ReadSnapshot(t.Context(), missing); err == nil || IsUnsupported(err) {
		t.Fatalf("missing source: %v", err)
	}
}

// Refusal separates a refused projection from missing or obsolete evidence,
// which CheckAttestation reports as Unknown alike.
func TestSnapshotRefusalSeparatesProjectionFromEvidence(t *testing.T) {
	supported, _ := fixture(t)
	if err := supported.Refusal(); err != nil {
		t.Fatalf("supported projection refused: %v", err)
	}
	if r := supported.CheckAttestation([]byte(syntheticPresentation), nil); r.State != Unknown {
		t.Fatalf("missing attestation: %+v", r)
	}
	if r := supported.CheckAttestation([]byte(syntheticPresentation), &Attestation{Profile: "obsolete"}); r.State != Unknown {
		t.Fatalf("obsolete attestation: %+v", r)
	}

	p := filepath.Join(t.TempDir(), "no_identity.db")
	db := testfixture.Source(t, p, []byte(syntheticRaw))
	testfixture.Exec(t, db, "UPDATE trajectory_meta SET cascade_id=''")
	anonymous, err := ReadSnapshot(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if err = anonymous.Refusal(); !IsUnsupported(err) || err.Error() != "persisted cascade identity is empty" {
		t.Fatalf("empty identity: %v", err)
	}

	const unsupportedRaw = `{"cascadeId":"11111111-1111-4111-8111-111111111111","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"send_message"}},"generic":{"args":{"Message":"hi","Recipient":"parent","Unqualified":"x"},"result":{"result":"Message sent"}}}]}`
	p = filepath.Join(t.TempDir(), "unsupported_projection.db")
	testfixture.Source(t, p, []byte(unsupportedRaw))
	refused, err := ReadSnapshot(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if err = refused.Refusal(); !IsUnsupported(err) || err.Error() != "unsupported generic arguments" {
		t.Fatalf("refused projection: %v", err)
	}
}
