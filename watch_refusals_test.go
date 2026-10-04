package main

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
	"sync/atomic"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/testfixture"
)

// unsupportedProjectionPayload is a send_message step with an argument the
// supported projection does not qualify. A daemon that returns it unchanged
// passes the full comparison, and then the projection of the source itself
// refuses it as unknown ("unsupported generic arguments"), whatever the daemon
// answers.
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

// replaceStep rewrites step 0 of the source with step 0 of payload. The step's
// header and side columns must stay the same.
func replaceStep(t *testing.T, db *sql.DB, payload []byte) {
	t.Helper()
	m := testfixture.Message(t, "gemini_coder.Trajectory", payload).ProtoReflect()
	steps := m.Get(m.Descriptor().Fields().ByName("steps")).List()
	testfixture.Exec(t, db, "UPDATE steps SET step_payload=? WHERE idx=0", testfixture.Binary(t, steps.Get(0).Message()))
}

// unsupportedPayload carries a step type name the supported profile does not
// know. It is a refusal of the daemon's response, not of the source.
func unsupportedPayload() []byte {
	return []byte(fmt.Sprintf(`{"cascadeId":%q,"metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_NOT_IN_THIS_PROFILE"}]}`, completeID))
}

// payloadDaemon serves switchable trajectory payloads and counts every request
// it receives, LoadTrajectory included. full answers the disableRehydration
// RPC, presentation the ordinary one.
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

func heldWatcher(t *testing.T, root, url string, logs *bytes.Buffer) *watcher {
	t.Helper()
	return &watcher{
		ctx:       t.Context(),
		root:      root,
		baseURL:   url,
		client:    daemon.NewClient(url),
		urlPinned: true,
		logger:    log.New(logs, "", 0),
		interval:  time.Second,
	}
}

func refusalLines(logs *bytes.Buffer) int {
	return strings.Count(logs.String(), "watch: skip ")
}

// An unchanged source whose projection the profile does not support is fetched
// once, then held without daemon calls; a content change and an endpoint
// change each earn exactly one retry and one log line.
func TestWatchHoldsUnchangedUnknownSourceWithoutDaemonCalls(t *testing.T) {
	root, s, db := unsupportedProjectionFixture(t)
	d := newPayloadDaemon(t, unsupportedProjectionPayload("first"))
	var logs bytes.Buffer
	w := heldWatcher(t, root, d.URL, &logs)

	w.tick()
	first := d.calls.Load()
	if first == 0 {
		t.Fatal("first tick did not fetch the unsupported source")
	}
	if got := refusalLines(&logs); got != 1 || !strings.Contains(logs.String(), "(completeness unknown: unsupported generic arguments); not retrying") {
		t.Fatalf("first refusal not held and logged once (%d):\n%s", got, logs.String())
	}
	for range 3 {
		w.tick()
	}
	if got := d.calls.Load(); got != first {
		t.Fatalf("unchanged unsupported source fetched again: %d daemon calls after first tick's %d", got, first)
	}
	if got := refusalLines(&logs); got != 1 {
		t.Fatalf("refusal logged on every tick (%d lines):\n%s", got, logs.String())
	}
	if !strings.Contains(logs.String(), "tick: 0 synced, 1 skipped, 0 up-to-date, 1 failed") ||
		!strings.Contains(logs.String(), "tick: 0 synced, 1 skipped, 0 up-to-date, 0 failed") {
		t.Fatalf("held source not reported as skipped:\n%s", logs.String())
	}
	if w.consecutiveFailures != 0 {
		t.Fatalf("reachable daemon counted as failing: %d", w.consecutiveFailures)
	}
	if _, err := os.Stat(s.SidecarPath); !os.IsNotExist(err) {
		t.Fatalf("refused source wrote a sidecar: %v", err)
	}

	// Content change, which the daemon also serves: retried on the very next
	// tick, refused for the same reason, then held again.
	moved := unsupportedProjectionPayload("the source moved on")
	replaceStep(t, db, moved)
	d.serve(moved, moved)
	w.tick()
	afterChange := d.calls.Load()
	if afterChange == first {
		t.Fatal("changed source was not retried on the next tick")
	}
	w.tick()
	if d.calls.Load() != afterChange {
		t.Fatal("changed-then-refused source not held again")
	}
	if got := refusalLines(&logs); got != 2 {
		t.Fatalf("want one refusal line per change, got %d:\n%s", got, logs.String())
	}

	// Endpoint change: the new daemon gets one try.
	next := newPayloadDaemon(t, moved)
	w.baseURL = next.URL
	w.client = daemon.NewClient(next.URL)
	w.tick()
	tried := next.calls.Load()
	if tried == 0 {
		t.Fatal("source not retried after the daemon endpoint changed")
	}
	w.tick()
	if next.calls.Load() != tried || d.calls.Load() != afterChange {
		t.Fatal("source not held at the new endpoint")
	}
	if got := refusalLines(&logs); got != 3 {
		t.Fatalf("want one refusal line per change, got %d:\n%s", got, logs.String())
	}
}

// A refusal that comes from validating the daemon's response is never held. At
// the same endpoint, with the source unchanged, the watcher fetches it again
// on every tick and syncs as soon as the daemon's answer becomes valid
// (roborev 18304).
func TestWatchRetriesRefusedDaemonResponseAtSameEndpoint(t *testing.T) {
	valid := completePayload("current")
	// The typed daemon view accepts this string; the protobuf JSON parser does
	// not accept it as a Timestamp.
	malformed := bytes.Replace(valid, []byte(`"metadata":{}`), []byte(`"metadata":{"createdAt":"not a timestamp"}`), 1)
	for _, tc := range []struct {
		name, reason       string
		full, presentation []byte
	}{
		{"RPC enum outside the profile", "unsupported RPC enum name", unsupportedPayload(), unsupportedPayload()},
		{"malformed protobuf JSON", "unsupported RPC protobuf JSON", malformed, malformed},
		{"invalid presentation after full match", "unsupported RPC enum name", valid, unsupportedPayload()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, s, _ := completeFixture(t)
			d := newPayloadDaemon(t, tc.full)
			d.serve(tc.full, tc.presentation)
			var logs bytes.Buffer
			w := heldWatcher(t, root, d.URL, &logs)

			w.tick()
			first := d.calls.Load()
			if first == 0 || !strings.Contains(logs.String(), "completeness unknown: "+tc.reason) {
				t.Fatalf("first tick did not refuse the response as %q (%d calls):\n%s", tc.reason, first, logs.String())
			}
			w.tick()
			if second := d.calls.Load(); second <= first {
				t.Fatalf("refused response not fetched again at the same endpoint: %d then %d calls\n%s", first, second, logs.String())
			}
			if strings.Contains(logs.String(), "not retrying") {
				t.Fatalf("response refusal held:\n%s", logs.String())
			}

			d.serve(valid, valid)
			w.tick()
			if result := sessionCompleteness(s); result.State != "verified" {
				t.Fatalf("valid response at the same endpoint not synced: %+v\n%s", result, logs.String())
			}
			if !strings.Contains(logs.String(), "tick: 1 synced, 0 skipped, 0 up-to-date, 0 failed") {
				t.Fatalf("sync not reported on the tick after the response became valid:\n%s", logs.String())
			}
		})
	}
}

// An mtime advance alone releases a hold, even when the content digest is the
// same.
func TestWatchRetriesHeldSourceWhenOnlyMtimeAdvances(t *testing.T) {
	root, s, _ := unsupportedProjectionFixture(t)
	d := newPayloadDaemon(t, unsupportedProjectionPayload("first"))
	var logs bytes.Buffer
	w := heldWatcher(t, root, d.URL, &logs)
	w.tick()
	first := d.calls.Load()
	w.tick()
	if d.calls.Load() != first {
		t.Fatal("unchanged source fetched again")
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(s.PBPath, later, later); err != nil {
		t.Fatal(err)
	}
	w.tick()
	if d.calls.Load() == first {
		t.Fatal("source with an advanced mtime was not retried")
	}
}

// A source refused before any RPC is held too, logged once, and released as
// soon as it becomes readable, which then syncs it.
func TestWatchReleasesUnreadableSourceOnceReadable(t *testing.T) {
	root, s, db := completeFixture(t)
	testfixture.Exec(t, db, "PRAGMA user_version=2")
	d := newPayloadDaemon(t, completePayload("current"))
	var logs bytes.Buffer
	w := heldWatcher(t, root, d.URL, &logs)
	for range 3 {
		w.tick()
	}
	if d.calls.Load() != 0 {
		t.Fatalf("unreadable source reached the daemon: %d calls", d.calls.Load())
	}
	if got := refusalLines(&logs); got != 1 {
		t.Fatalf("unreadable source refusal logged %d times:\n%s", got, logs.String())
	}
	testfixture.Exec(t, db, "PRAGMA user_version=1")
	w.tick()
	if d.calls.Load() == 0 {
		t.Fatal("readable source not retried")
	}
	if result := sessionCompleteness(s); result.State != "verified" {
		t.Fatalf("readable source not synced: %+v\n%s", result, logs.String())
	}
}

// Incomplete results are never held: a stale daemon response is fetched again
// every tick, and the source syncs as soon as a current daemon answers.
func TestWatchKeepsRetryingIncompleteResults(t *testing.T) {
	root, s, _ := completeFixture(t)
	stale := newPayloadDaemon(t, completePayload("stale"))
	var logs bytes.Buffer
	w := heldWatcher(t, root, stale.URL, &logs)
	w.tick()
	first := stale.calls.Load()
	w.tick()
	second := stale.calls.Load()
	if first == 0 || second <= first {
		t.Fatalf("incomplete result not retried: %d then %d calls", first, second)
	}
	if strings.Contains(logs.String(), "not retrying") {
		t.Fatalf("incomplete result held:\n%s", logs.String())
	}
	current := newPayloadDaemon(t, completePayload("current"))
	w.baseURL = current.URL
	w.client = daemon.NewClient(current.URL)
	w.tick()
	if result := sessionCompleteness(s); result.State != "verified" {
		t.Fatalf("current daemon did not sync: %+v", result)
	}
}

// Sources that disappear from the root are forgotten.
func TestRefusalMemoPrunesMissingSources(t *testing.T) {
	root, s, _ := unsupportedProjectionFixture(t)
	d := newPayloadDaemon(t, unsupportedProjectionPayload("first"))
	w := heldWatcher(t, root, d.URL, new(bytes.Buffer))
	w.tick()
	if _, ok := w.held[s.PBPath]; !ok {
		t.Fatal("refusal not held")
	}
	for _, p := range []string{s.PBPath, s.PBPath + "-wal", s.PBPath + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	w.tick()
	if len(w.held) != 0 {
		t.Fatalf("missing source still held: %v", w.held)
	}
}
