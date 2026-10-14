package doctor

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/session"
	"github.com/mjacobs/agy-reader/internal/testfixture"
	"github.com/mjacobs/agy-reader/internal/watcher"
)

const completeID = "11111111-1111-4111-8111-111111111111"

func completePayload(text string) []byte {
	return []byte(fmt.Sprintf(`{"cascadeId":%q,"metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":%q}}]}`, completeID, text))
}

func mkSession(id, pb, sidecar string, mtime time.Time) discovery.Session {
	return discovery.Session{
		CascadeID:   id,
		PBPath:      pb,
		SidecarPath: sidecar,
		Bucket:      "conversations",
		ModTime:     mtime,
	}
}

func completeFixture(t *testing.T) (string, discovery.Session, *sql.DB) {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, "conversations", completeID+".db")
	db := testfixture.Source(t, p, completePayload("current"))
	return root, mkSession(completeID, p, strings.TrimSuffix(p, ".db")+".trajectory.json", time.Now()), db
}

func completeServer(t *testing.T, payload []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/LoadTrajectory") {
			io.WriteString(w, `{}`)
			return
		}
		var req struct {
			DisableRehydration bool   `json:"disableRehydration"`
			CascadeID          string `json:"cascadeId"`
			Verbosity          string `json:"trajectoryVerbosity"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if req.CascadeID != completeID {
			t.Errorf("wrong persisted identity %s", req.CascadeID)
		}
		fmt.Fprintf(w, `{"trajectory":%s}`, payload)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func unsupportedProjectionPayload(text string) []byte {
	return []byte(fmt.Sprintf(`{"cascadeId":%q,"metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"send_message"}},"generic":{"args":{"Message":%q,"Recipient":"parent","Unqualified":"x"},"result":{"result":"Message sent"}}}]}`, completeID, text))
}

func unsupportedProjectionFixture(t *testing.T) (string, discovery.Session, *sql.DB) {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, "conversations", completeID+".db")
	db := testfixture.Source(t, p, unsupportedProjectionPayload("first"))
	return root, mkSession(completeID, p, strings.TrimSuffix(p, ".db")+".trajectory.json", time.Now()), db
}

// watchRepairs runs one watch tick against a daemon that serves the supported
// fixture and requires it to verify s: the advice doctor gave was repairable.
func watchRepairs(t *testing.T, root string, s discovery.Session) {
	t.Helper()
	srv := completeServer(t, completePayload("current"))
	failures := 0
	synced, _, _, failed, _ := watcher.WatchTickPolicy(t.Context(), daemon.NewClient(srv.URL), root, log.New(io.Discard, "", 0), &failures, true, nil)
	if result := session.Completeness(s); synced != 1 || failed != 0 || result.State != completeness.Verified {
		t.Fatalf("watch did not repair the supported source: synced=%d failed=%d result=%+v", synced, failed, result)
	}
}

// A supported source whose existing sidecar has no completeness metadata, or
// metadata minted under an older profile, is refreshed by one watch tick.
// Doctor must count it as missing/stale with the watch hint, not unsupported
// (review finding rv-pfc9.1).
func TestCompletenessDoctorRefreshableSidecarKeepsWatchHint(t *testing.T) {
	for name, sidecar := range map[string][]byte{
		"no completeness metadata": completePayload("current"),
		"obsolete profile":         append(bytes.TrimSuffix(completePayload("current"), []byte("}")), []byte(`,"agyReader":{"completeness":{"profile":"obsolete"}}}`)...),
	} {
		t.Run(name, func(t *testing.T) {
			root, s, _ := completeFixture(t)
			if err := os.WriteFile(s.SidecarPath, sidecar, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, class := session.CompletenessClassified(s); class != session.SourceRepairable {
				t.Fatalf("class = %d, want repairable", class)
			}
			r := Report{DaemonURL: "http://127.0.0.1:1"}
			PopulateSidecarCoverage(&r, root)
			if r.Total != 1 || r.SQLite != 1 || r.Unknown != 1 || r.Stale != 1 || r.Unsupported != 0 || r.Unreadable != 0 {
				t.Fatalf("refreshable sidecar coverage: %+v", r)
			}
			var out bytes.Buffer
			if WriteReport(&out, r) == 0 {
				t.Fatal("unverified conversation passed doctor")
			}
			if !strings.Contains(out.String(), "1 missing/stale -> run: agy-reader --watch") || strings.Contains(out.String(), "unsupported") {
				t.Fatalf("refreshable sidecar must get the watch hint:\n%s", out.String())
			}
			watchRepairs(t, root, s)
		})
	}
}

// A supported source held by an exclusive SQLite lock past the busy timeout
// cannot be read now. That is neither unsupported nor proof the sidecar is
// stale: doctor asks for a retry, keeps a nonzero status, and once the lock
// clears one watch tick verifies the source (review finding rv-pfc9.2).
func TestCompletenessDoctorTemporaryLockIsNotUnsupported(t *testing.T) {
	root, s, db := completeFixture(t)
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

	result, class := session.CompletenessClassified(s)
	if !strings.Contains(strings.ToLower(result.Reason), "locked") {
		t.Fatalf("fixture did not produce a SQLite lock: %+v", result)
	}
	if class != session.SourceUnreadable {
		t.Fatalf("locked source class = %d, want unreadable", class)
	}
	r := Report{DaemonURL: "http://127.0.0.1:1"}
	PopulateSidecarCoverage(&r, root)
	if r.Total != 1 || r.SQLite != 1 || r.Unknown != 1 || r.Unreadable != 1 || r.Unsupported != 0 || r.Stale != 0 || r.Fresh != 0 {
		t.Fatalf("locked source coverage: %+v", r)
	}
	var out bytes.Buffer
	if WriteReport(&out, r) == 0 {
		t.Fatal("locked, unverified conversation passed doctor")
	}
	text := out.String()
	if !strings.Contains(text, "1 unreadable -> could not read the conversation database (") ||
		!strings.Contains(text, "rerun doctor once agy is idle, or check file access") {
		t.Fatalf("locked source must get the retry/access advice:\n%s", text)
	}
	if strings.Contains(text, "unsupported") || strings.Contains(text, "compatibility update") {
		t.Fatalf("locked source must not get compatibility-update advice:\n%s", text)
	}

	if _, err = conn.ExecContext(t.Context(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if err = conn.Close(); err != nil {
		t.Fatal(err)
	}
	watchRepairs(t, root, s)
}

// A source the profile refuses (here PRAGMA user_version=2) keeps the
// compatibility-update advice and no watch hint.
func TestCompletenessDoctorRefusedSourceIsUnsupported(t *testing.T) {
	root, s, db := completeFixture(t)
	testfixture.Exec(t, db, "PRAGMA user_version=2")
	if _, class := session.CompletenessClassified(s); class != session.SourceUnsupported {
		t.Fatalf("refused source class = %d, want unsupported", class)
	}
	r := Report{DaemonURL: "http://127.0.0.1:1"}
	PopulateSidecarCoverage(&r, root)
	if r.Unknown != 1 || r.Unsupported != 1 || r.Stale != 0 || r.Unreadable != 0 {
		t.Fatalf("refused source coverage: %+v", r)
	}
	var out bytes.Buffer
	if WriteReport(&out, r) == 0 {
		t.Fatal("unsupported conversation passed doctor")
	}
	if !strings.Contains(out.String(), "1 unsupported -> needs an agy-reader compatibility update (see README)") || strings.Contains(out.String(), "missing/stale") {
		t.Fatalf("refused source must get the compatibility hint only:\n%s", out.String())
	}
}

// A source whose projection the profile refuses is unsupported even without a
// sidecar: watch refuses it on every fetch and never writes one, so the watch
// hint would never clear.
func TestCompletenessDoctorRefusedProjectionIsUnsupported(t *testing.T) {
	root, s, _ := unsupportedProjectionFixture(t)
	result, class := session.CompletenessClassified(s)
	if class != session.SourceUnsupported || result.State != completeness.Unknown {
		t.Fatalf("refused projection: class=%d result=%+v", class, result)
	}
	r := Report{DaemonURL: "http://127.0.0.1:1"}
	PopulateSidecarCoverage(&r, root)
	if r.Unknown != 1 || r.Unsupported != 1 || r.Stale != 0 || r.Unreadable != 0 {
		t.Fatalf("refused projection coverage: %+v", r)
	}
}

// A source with an empty persisted cascade identity is refused by every fetch
// (fetchSession) and never gets a sidecar, so an old sidecar beside it is not
// watch work either (review finding rv-vaw0.1).
func TestCompletenessDoctorEmptyIdentityIsUnsupported(t *testing.T) {
	root, s, db := completeFixture(t)
	testfixture.Exec(t, db, "UPDATE trajectory_meta SET cascade_id=''")
	if err := os.WriteFile(s.SidecarPath, completePayload("current"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, class := session.CompletenessClassified(s)
	if class != session.SourceUnsupported || result.Reason != "persisted cascade identity is empty" {
		t.Fatalf("empty identity: class=%d result=%+v", class, result)
	}
	r := Report{DaemonURL: "http://127.0.0.1:1"}
	PopulateSidecarCoverage(&r, root)
	if r.Unknown != 1 || r.Unsupported != 1 || r.Stale != 0 || r.Unreadable != 0 {
		t.Fatalf("empty identity coverage: %+v", r)
	}
}
