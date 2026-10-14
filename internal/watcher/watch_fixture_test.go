package watcher

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/testfixture"
)

const completeID = "11111111-1111-4111-8111-111111111111"

func completePayload(text string) []byte {
	return []byte(fmt.Sprintf(`{"cascadeId":%q,"metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":%q}}]}`, completeID, text))
}

func completeFixture(t *testing.T) (string, discovery.Session, *sql.DB) {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, "conversations", completeID+".db")
	db := testfixture.Source(t, p, completePayload("current"))
	return root, mkSession(completeID, p, strings.TrimSuffix(p, ".db")+".trajectory.json", time.Now()), db
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

func fakeDaemon(t *testing.T, onFetch func(cascadeID string)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/LoadTrajectory"):
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/GetCascadeTrajectory"):
			var req daemon.GetCascadeTrajectoryRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode request: %v", err)
				return
			}
			if onFetch != nil {
				onFetch(req.CascadeID)
			}
			resp := daemon.GetCascadeTrajectoryResponse{
				Trajectory: daemon.Trajectory{
					CascadeID: req.CascadeID,
					Steps: []daemon.Step{
						{Type: "CORTEX_STEP_TYPE_USER_INPUT", UserInput: &daemon.UserInput{UserResponse: "hi"}},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
}

func seedPB(t *testing.T, root, bucket, id string, mtime time.Time) string {
	t.Helper()
	dir := filepath.Join(root, bucket)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, id+".pb")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if !mtime.IsZero() {
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func seedDB(t *testing.T, root, bucket, id string, mtime, walMtime time.Time) string {
	t.Helper()
	path := filepath.Join(root, bucket, id+".db")
	raw := []byte(`{"cascadeId":"` + id + `","steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":"hi"},"metadata":{}}],"metadata":{}}`)
	db := testfixture.Source(t, path, raw)
	if !walMtime.IsZero() {
		testfixture.Exec(t, db, "PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; UPDATE trajectory_meta SET cascade_id=cascade_id")
		if err := os.Chtimes(path+"-wal", walMtime, walMtime); err != nil {
			t.Fatal(err)
		}
	}
	if !mtime.IsZero() {
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func advertiseCLIDaemon(t *testing.T, root, srvURL string) {
	t.Helper()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(srvURL, "http://"))
	if err != nil {
		t.Fatalf("parse fake daemon url %q: %v", srvURL, err)
	}
	logLine := "Language server listening on random port at " + port + " for HTTP\n"
	if err := os.WriteFile(filepath.Join(root, "cli.log"), []byte(logLine), 0o644); err != nil {
		t.Fatal(err)
	}
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

func replaceStep(t *testing.T, db *sql.DB, payload []byte) {
	t.Helper()
	m := testfixture.Message(t, "gemini_coder.Trajectory", payload).ProtoReflect()
	steps := m.Get(m.Descriptor().Fields().ByName("steps")).List()
	testfixture.Exec(t, db, "UPDATE steps SET step_payload=? WHERE idx=0", testfixture.Binary(t, steps.Get(0).Message()))
}

func unsupportedPayload() []byte {
	return []byte(fmt.Sprintf(`{"cascadeId":%q,"metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_NOT_IN_THIS_PROFILE"}]}`, completeID))
}

type payloadDaemon struct {
	URL          string
	calls        atomic.Int64
	full         atomic.Pointer[[]byte]
	presentation atomic.Pointer[[]byte]
}

func newPayloadDaemon(t *testing.T, payload []byte) *payloadDaemon {
	t.Helper()
	d := &payloadDaemon{}
	d.serve(payload, payload)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.calls.Add(1)
		if strings.HasSuffix(r.URL.Path, "/LoadTrajectory") {
			io.WriteString(w, `{}`)
			return
		}
		var req struct {
			DisableRehydration bool `json:"disableRehydration"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		payload := d.presentation.Load()
		if req.DisableRehydration {
			payload = d.full.Load()
		}
		fmt.Fprintf(w, `{"trajectory":%s}`, *payload)
	}))
	t.Cleanup(srv.Close)
	d.URL = srv.URL
	return d
}

func (d *payloadDaemon) serve(full, presentation []byte) {
	d.full.Store(&full)
	d.presentation.Store(&presentation)
}

func heldWatcher(t *testing.T, root, url string, logs *bytes.Buffer) *Watcher {
	t.Helper()
	return &Watcher{
		Ctx:       t.Context(),
		Root:      root,
		BaseURL:   url,
		Client:    daemon.NewClient(url),
		UrlPinned: true,
		Logger:    log.New(logs, "", 0),
		Interval:  time.Second,
	}
}

func refusalLines(logs *bytes.Buffer) int {
	return strings.Count(logs.String(), "watch: skip ")
}
