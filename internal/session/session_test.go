package session

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/testfixture"
)

const testCascadeID = "11111111-1111-4111-8111-111111111111"

func testPayload(text string) []byte {
	return []byte(fmt.Sprintf(`{"cascadeId":%q,"metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":%q}}]}`, testCascadeID, text))
}

func testFixture(t *testing.T) (string, discovery.Session, *sql.DB) {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, "conversations", testCascadeID+".db")
	db := testfixture.Source(t, p, testPayload("current"))
	s := discovery.Session{
		CascadeID:   testCascadeID,
		PBPath:      p,
		SidecarPath: strings.TrimSuffix(p, ".db") + ".trajectory.json",
		Bucket:      "conversations",
		ModTime:     time.Now(),
	}
	return root, s, db
}

func TestNewDaemonClientTokenWiring(t *testing.T) {
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "")
	root := t.TempDir()
	conn := discovery.Connection{BaseURL: "http://127.0.0.1:1", Token: "scan-token"}
	if c := NewDaemonClient(root, conn); c.CSRFToken != "scan-token" {
		t.Errorf("token discovered with the endpoint was lost: got %q want %q", c.CSRFToken, "scan-token")
	}
	if c := NewDaemonClient(root, discovery.Connection{BaseURL: conn.BaseURL}); c.CSRFToken != "" {
		t.Errorf("a connection with no token should not invent one, got %q", c.CSRFToken)
	}
}

func TestIsStaleChecksSidecarPresenceAndModtime(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	pbPath := filepath.Join(root, "legacy.pb")
	if err := os.WriteFile(pbPath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	sidecarPath := filepath.Join(root, "legacy.trajectory.json")
	s := discovery.Session{
		CascadeID:   "legacy",
		PBPath:      pbPath,
		SidecarPath: sidecarPath,
		Bucket:      "conversations",
		ModTime:     now,
	}

	// Missing sidecar is stale
	stale, reason := IsStale(s)
	if !stale || reason != "missing" {
		t.Fatalf("expected missing sidecar stale, got stale=%v reason=%q", stale, reason)
	}

	// Sidecar newer than source is not stale
	if err := os.WriteFile(s.SidecarPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	future := now.Add(time.Hour)
	if err := os.Chtimes(s.SidecarPath, future, future); err != nil {
		t.Fatal(err)
	}
	stale, _ = IsStale(s)
	if stale {
		t.Fatal("expected fresh when sidecar is newer than source")
	}

	// Source newer than sidecar is stale
	moreFuture := future.Add(time.Hour)
	if err := os.Chtimes(s.PBPath, moreFuture, moreFuture); err != nil {
		t.Fatal(err)
	}
	s.ModTime = moreFuture
	stale, reason = IsStale(s)
	if !stale || reason != "modtime-advanced" {
		t.Fatalf("expected modtime-advanced, got stale=%v reason=%q", stale, reason)
	}
}

func TestCompletenessVerificationAndClassification(t *testing.T) {
	root, s, _ := testFixture(t)
	// No sidecar
	result := Completeness(s)
	if result.State != completeness.Unknown {
		t.Fatalf("expected unknown completeness without sidecar, got %+v", result)
	}

	// Fake daemon
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/LoadTrajectory") {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		fmt.Fprintf(w, `{"trajectory":%s}`, testPayload("current"))
	}))
	defer srv.Close()

	client := daemon.NewClient(srv.URL)
	traj, err := Fetch(t.Context(), client, s)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if err := cache.Write(s.SidecarPath, traj); err != nil {
		t.Fatalf("cache.Write: %v", err)
	}

	result = Completeness(s)
	if result.State != completeness.Verified {
		t.Fatalf("expected verified completeness, got %+v", result)
	}

	res, class := CompletenessClassified(s)
	if res.State != completeness.Verified || class != SourceRepairable {
		t.Fatalf("classified: res=%+v class=%v", res, class)
	}
	_ = root
}

func TestSourceOnlyVerified(t *testing.T) {
	if SourceOnlyVerified("unknown", "") {
		t.Fatal("unknown cannot be source-only verified")
	}
	if !SourceOnlyVerified(string(completeness.Verified), completeness.SourceOnlyReason) {
		t.Fatal("expected source-only verified to be true")
	}
	traj := &daemon.Trajectory{
		Verification: &completeness.Attestation{
			Reason: completeness.SourceOnlyReason,
		},
	}
	if note := VerifiedNote(traj); !strings.Contains(note, "source-only") {
		t.Fatalf("expected source-only in note, got %q", note)
	}
}
