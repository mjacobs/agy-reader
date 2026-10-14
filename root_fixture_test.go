package main

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
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/testfixture"
	"github.com/mjacobs/agy-reader/internal/watcher"
	"google.golang.org/protobuf/encoding/protowire"
)

const (
	completeID           = "11111111-1111-4111-8111-111111111111"
	qualifyVerifiedID    = "33333333-3333-4333-8333-333333333333"
	qualifyUnsupportedID = "44444444-4444-4444-8444-444444444444"
	qualifyLegacyID      = "66666666-6666-4666-8666-666666666666"
	qualifySourceOnlyID  = "88888888-8888-4888-8888-888888888888"
)

const sourceOnlyGeneratorRecord = `{"plannerConfig":{"maxOutputTokens":1024}}`

var unknownPersistedField = []byte{0xc2, 0x3e, 0x00}

func assertOutput(t *testing.T, what, out string, want, never []string) {
	t.Helper()
	for _, s := range want {
		if !strings.Contains(out, s) {
			t.Errorf("%s missing %q:\n%s", what, s, out)
		}
	}
	for _, s := range never {
		if strings.Contains(out, s) {
			t.Errorf("%s contains %q:\n%s", what, s, out)
		}
	}
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

func qualifyPayload(id string) []byte {
	return []byte(fmt.Sprintf(`{"cascadeId":%q,"metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":"fixture %s"}}]}`, id, id[:8]))
}

func sourceOnlyPayload(id string) []byte {
	return []byte(fmt.Sprintf(`{"cascadeId":%q,"metadata":{},"generatorMetadata":[%s],"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":"fixture %s"}}]}`, id, sourceOnlyGeneratorRecord, id[:8]))
}

func qualifySource(t *testing.T, root, id string, unsupported bool, mtime time.Time) {
	t.Helper()
	path := filepath.Join(root, "conversations", id+".db")
	db := testfixture.Source(t, path, qualifyPayload(id))
	if unsupported {
		testfixture.Exec(t, db, "INSERT INTO executor_metadata VALUES(0,?)", unknownPersistedField)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func sourceOnlySource(t *testing.T, root, id string, mtime time.Time) {
	t.Helper()
	path := filepath.Join(root, "conversations", id+".db")
	db := testfixture.Source(t, path, sourceOnlyPayload(id))
	record := testfixture.Message(t, "exa.cortex_pb.CortexStepGeneratorMetadata", []byte(sourceOnlyGeneratorRecord)).ProtoReflect()
	planner := record.Mutable(record.Descriptor().Fields().ByName("planner_config")).Message()
	planner.SetUnknown(protowire.AppendBytes(protowire.AppendTag(nil, 50, protowire.BytesType), nil))
	testfixture.Exec(t, db, "UPDATE gen_metadata SET data=?1,size=length(?1) WHERE idx=0", testfixture.Binary(t, record))
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func qualifyDaemon(t *testing.T, ids ...string) *httptest.Server {
	t.Helper()
	known := map[string]bool{}
	for _, id := range ids {
		known[id] = true
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CascadeID string `json:"cascadeId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !known[req.CascadeID] {
			http.Error(w, "unknown conversation", http.StatusNotFound)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/LoadTrajectory") {
			_, _ = io.WriteString(w, `{}`)
			return
		}
		payload := qualifyPayload(req.CascadeID)
		if req.CascadeID == qualifySourceOnlyID {
			payload = sourceOnlyPayload(req.CascadeID)
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

func fakeDaemon(t *testing.T, onFetch func(cascadeID string)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/LoadTrajectory"):
			var req struct {
				CascadeID string `json:"cascadeId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if onFetch != nil {
				onFetch(req.CascadeID)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"trajectory": {"cascadeId": "` + req.CascadeID + `"}}`))
		case strings.HasSuffix(r.URL.Path, "/GetCascadeTrajectory"):
			var req struct {
				CascadeID string `json:"cascadeId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if onFetch != nil {
				onFetch(req.CascadeID)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"trajectory": {"cascadeId": "` + req.CascadeID + `"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func heldWatcher(t *testing.T, root, url string, logs *bytes.Buffer) *watcher.Watcher {
	t.Helper()
	return &watcher.Watcher{
		Ctx:       t.Context(),
		Root:      root,
		BaseURL:   url,
		Client:    daemon.NewClient(url),
		UrlPinned: true,
		Logger:    log.New(logs, "", 0),
		Interval:  time.Second,
	}
}
