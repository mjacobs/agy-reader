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
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/completeness"
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
func completeServer(t *testing.T, payload []byte, onRaw func()) *httptest.Server {
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
		if req.DisableRehydration {
			if req.Verbosity != "CLIENT_TRAJECTORY_VERBOSITY_FULL" {
				t.Error("raw fetch not full")
			}
			if onRaw != nil {
				onRaw()
			}
		}
		fmt.Fprintf(w, `{"trajectory":%s}`, payload)
	}))
	t.Cleanup(srv.Close)
	return srv
}
func replaceCompleteStep(t *testing.T, db *sql.DB, text string) {
	t.Helper()
	m := testfixture.Message(t, "gemini_coder.Trajectory", completePayload(text)).ProtoReflect()
	steps := m.Get(m.Descriptor().Fields().ByName("steps")).List()
	testfixture.Exec(t, db, "UPDATE steps SET step_payload=? WHERE idx=0", testfixture.Binary(t, steps.Get(0).Message()))
}
func TestCompletenessRecoveryAndPinnedEndpoint(t *testing.T) {
	root, s, _ := completeFixture(t)
	stale := completeServer(t, completePayload("stale"), nil)
	current := completeServer(t, completePayload("current"), nil)
	calls := 0
	candidates := func(got string) []discovery.Connection {
		calls++
		if got != root {
			t.Fatal("wrong root")
		}
		return []discovery.Connection{{BaseURL: stale.URL}, {BaseURL: current.URL}}
	}
	client := daemon.NewClient(stale.URL)
	if _, err := fetchSessionCandidates(t.Context(), client, root, s, true, candidates); err == nil || calls != 0 {
		t.Fatalf("pin escaped: %v calls=%d", err, calls)
	}
	traj, err := fetchSessionCandidates(t.Context(), client, root, s, false, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if client.BaseURL != current.URL || calls != 1 {
		t.Fatal("recovery did not adopt verified endpoint")
	}
	if err = cache.Write(s.SidecarPath, traj); err != nil {
		t.Fatal(err)
	}
	if result := sessionCompleteness(s); result.State != completeness.Verified {
		t.Fatal(result)
	}
}
func TestCompletenessWatchRetryAndCheckpoint(t *testing.T) {
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "pinned")
	root, s, db := completeFixture(t)
	stale := completeServer(t, completePayload("stale"), nil)
	logger := log.New(io.Discard, "", 0)
	failures := 0
	synced, _, _, failed, _ := watchTick(t.Context(), daemon.NewClient(stale.URL), root, logger, &failures)
	if synced != 0 || failed != 1 {
		t.Fatalf("partial success accepted: %d %d", synced, failed)
	}
	if _, err := os.Stat(s.SidecarPath); !os.IsNotExist(err) {
		t.Fatalf("unverified sidecar written: %v", err)
	}
	current := completeServer(t, completePayload("current"), nil)
	client := daemon.NewClient(current.URL)
	synced, _, _, failed, _ = watchTick(t.Context(), client, root, logger, &failures)
	if synced != 1 || failed != 0 {
		t.Fatalf("retry failed: %d %d", synced, failed)
	}
	before, err := os.Stat(s.SidecarPath)
	if err != nil {
		t.Fatal(err)
	}
	testfixture.Exec(t, db, "PRAGMA journal_mode=WAL;PRAGMA wal_autocheckpoint=0;UPDATE trajectory_meta SET cascade_id=cascade_id;PRAGMA wal_checkpoint(TRUNCATE)")
	synced, _, fresh, failed, _ := watchTick(t.Context(), client, root, logger, &failures)
	after, _ := os.Stat(s.SidecarPath)
	if synced != 0 || fresh != 1 || failed != 0 || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("checkpoint rewrote verified sidecar")
	}
	replaceCompleteStep(t, db, "same count new final answer")
	if stale, _ := isStale(s); !stale {
		t.Fatal("same-count WAL update invisible")
	}
	synced, _, _, failed, _ = watchTick(t.Context(), client, root, logger, &failures)
	after, _ = os.Stat(s.SidecarPath)
	if synced != 0 || failed != 1 || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("stale RPC advanced sidecar")
	}
}
func TestCompletenessSourceBracketsAndWriteGuard(t *testing.T) {
	_, s, db := completeFixture(t)
	srv := completeServer(t, completePayload("current"), func() { replaceCompleteStep(t, db, "changed during fetch") })
	if _, err := fetchSession(t.Context(), daemon.NewClient(srv.URL), s); err == nil || !strings.Contains(err.Error(), "changed during fetch") {
		t.Fatalf("source bracket: %v", err)
	}
	srv2 := completeServer(t, completePayload("changed during fetch"), nil)
	traj, err := fetchSession(t.Context(), daemon.NewClient(srv2.URL), s)
	if err != nil {
		t.Fatal(err)
	}
	replaceCompleteStep(t, db, "changed before write")
	if err = cache.Write(s.SidecarPath, traj); err == nil {
		t.Fatal("write guard accepted superseded attestation")
	}
	if _, err = os.Stat(s.SidecarPath); !os.IsNotExist(err) {
		t.Fatal("write guard touched sidecar")
	}
}
func TestCompletenessFallbackAndRawPreservation(t *testing.T) {
	root, s, db := completeFixture(t)
	payload := bytes.Replace(completePayload("current"), []byte(`"metadata":{}`), []byte(`"metadata":{},"future":{"n":9007199254740993,"nested":{"b":2,"a":1}}`), 1)
	srv := completeServer(t, payload, nil)
	traj, err := fetchSession(t.Context(), daemon.NewClient(srv.URL), s)
	if err != nil {
		t.Fatal(err)
	}
	if err = cache.Write(s.SidecarPath, traj); err != nil {
		t.Fatal(err)
	}
	const parent = "22222222-2222-4222-8222-222222222222"
	if _, err = cache.StampParentCascadeID(s.SidecarPath, parent); err != nil {
		t.Fatal(err)
	}
	if err = cache.Write(s.SidecarPath, traj); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(s.SidecarPath)
	if err != nil {
		t.Fatal(err)
	}
	stamp, _ := os.Stat(s.SidecarPath)
	if !bytes.Contains(before, []byte("9007199254740993")) || !bytes.Contains(before, []byte(parent)) {
		t.Fatal("raw/reader values lost")
	}
	if result := sessionCompleteness(s); result.State != completeness.Verified {
		t.Fatal(result)
	}
	testfixture.Exec(t, db, "PRAGMA user_version=2")
	fallback, err := fetchDiskSession(t.Context(), discovery.Connection{BaseURL: srv.URL}, root, s)
	if err != nil || !fallback.FromCache {
		t.Fatalf("cache fallback: %v", err)
	}
	if err = cache.Write(s.SidecarPath, fallback); err == nil {
		t.Fatal("fallback granted write authority")
	}
	after, _ := os.ReadFile(s.SidecarPath)
	now, _ := os.Stat(s.SidecarPath)
	if !bytes.Equal(before, after) || !stamp.ModTime().Equal(now.ModTime()) {
		t.Fatal("fallback refreshed cache")
	}
	r := doctorReport{}
	populateSidecarCoverage(&r, root)
	if r.total != 1 || r.stale != 0 || r.unknown != 1 || r.fresh != 0 {
		t.Fatalf("unknown coverage: %+v", r)
	}
	var out bytes.Buffer
	if writeMultiDoctorReport(&out, []doctorReport{{total: 1, fresh: 1}, r}, false) != 1 {
		t.Fatal("healthy root hid unknown source")
	}
}
func TestCompletenessDoctorClassifiesOneObservation(t *testing.T) {
	root, s, db := completeFixture(t)
	srv := completeServer(t, completePayload("current"), nil)
	traj, err := fetchSession(t.Context(), daemon.NewClient(srv.URL), s)
	if err != nil {
		t.Fatal(err)
	}
	if err = cache.Write(s.SidecarPath, traj); err != nil {
		t.Fatal(err)
	}
	pb := seedPB(t, root, "conversations", "legacy", time.Now().Add(-time.Hour))
	if err = os.WriteFile(strings.TrimSuffix(pb, ".pb")+".trajectory.json", []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	r := doctorReport{}
	populateSidecarCoverage(&r, root)
	if r.total != 2 || r.fresh != 2 || r.verified != 1 || r.legacyFresh != 1 {
		t.Fatalf("verified/legacy: %+v", r)
	}
	replaceCompleteStep(t, db, "changed")
	r = doctorReport{}
	populateSidecarCoverage(&r, root)
	if r.total != 2 || r.fresh != 1 || r.stale != 1 || r.incomplete != 1 || r.legacyFresh != 1 {
		t.Fatalf("incomplete: %+v", r)
	}
}

func TestCompletenessRecoveryBoundAndWatcherLiveness(t *testing.T) {
	root, s, _ := completeFixture(t)
	stale := completeServer(t, completePayload("stale"), nil)
	counts := make([]int, 10)
	conns := make([]discovery.Connection, 10)
	for i := range conns {
		i := i
		server := completeServer(t, completePayload("stale"), func() { counts[i]++ })
		conns[i] = discovery.Connection{BaseURL: server.URL}
	}
	if _, err := fetchSessionCandidates(t.Context(), daemon.NewClient(stale.URL), root, s, false, func(string) []discovery.Connection { return conns }); err == nil {
		t.Fatal("stale accepted")
	}
	for i, count := range counts {
		if (i < 8) != (count > 0) {
			t.Fatalf("candidate %d: %d probes", i, count)
		}
	}
	w := watcher{ctx: t.Context(), root: root, baseURL: stale.URL, client: daemon.NewClient(stale.URL), urlPinned: true, logger: log.New(io.Discard, "", 0), interval: time.Second, idleTimeout: time.Second, consecutiveFailures: 2}
	for range 3 {
		if w.tick() || w.consecutiveFailures != 0 {
			t.Fatal("reachable incomplete daemon entered idle exit")
		}
	}
}

func TestCompletenessReplacesOldEvidence(t *testing.T) {
	_, s, _ := completeFixture(t)
	srv := completeServer(t, completePayload("current"), nil)
	traj, err := fetchSession(t.Context(), daemon.NewClient(srv.URL), s)
	if err != nil {
		t.Fatal(err)
	}
	old := []byte(`{"agyReader":{"parentCascadeId":"22222222-2222-4222-8222-222222222222","future":{"n":9007199254740993},"completeness":{"profile":"obsolete"}}}`)
	if err = os.WriteFile(s.SidecarPath, old, 0600); err != nil {
		t.Fatal(err)
	}
	if err = cache.Write(s.SidecarPath, traj); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.SidecarPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("9007199254740993")) || !bytes.Contains(data, []byte("22222222-2222-4222-8222-222222222222")) {
		t.Fatal("reader siblings lost")
	}
	if result := sessionCompleteness(s); result.State != completeness.Verified {
		t.Fatal(result)
	}
}

func TestCompletenessUnknownSourceDoesNotHideDeadDaemon(t *testing.T) {
	root, s, db := completeFixture(t)
	testfixture.Exec(t, db, "PRAGMA user_version=2")
	srv := completeServer(t, completePayload("current"), nil)
	w := watcher{ctx: t.Context(), root: root, baseURL: srv.URL, client: daemon.NewClient(srv.URL), urlPinned: true, logger: log.New(io.Discard, "", 0), interval: time.Second, idleTimeout: time.Second, consecutiveFailures: 2}
	if w.tick() || w.consecutiveFailures != 0 {
		t.Fatal("reachable endpoint treated as idle on source rejection")
	}
	srv.Close()
	if !w.tick() || w.consecutiveFailures != 1 {
		t.Fatal("local source failure hid dead endpoint")
	}
	if _, err := os.Stat(s.SidecarPath); !os.IsNotExist(err) {
		t.Fatal("unknown source wrote sidecar")
	}
}
func TestCompletenessRefreshRepairsMalformedReaderNamespace(t *testing.T) {
	for _, namespace := range []string{`[]`, `"broken"`, `3`, `null`} {
		t.Run(namespace, func(t *testing.T) {
			_, s, _ := completeFixture(t)
			srv := completeServer(t, completePayload("current"), nil)
			traj, err := fetchSession(t.Context(), daemon.NewClient(srv.URL), s)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(s.SidecarPath, []byte(`{"agyReader":`+namespace+`}`), 0600); err != nil {
				t.Fatal(err)
			}
			if err = cache.Write(s.SidecarPath, traj); err != nil {
				t.Fatal(err)
			}
			if result := sessionCompleteness(s); result.State != completeness.Verified {
				t.Fatal(result)
			}
		})
	}
}

func TestCompletenessEscapedSidecarAboveRPCSizeLimit(t *testing.T) {
	_, session, _ := completeFixture(t)
	// This future daemon field is below the RPC bound but becomes >64 MiB
	// when the cache encoder escapes each '<' as six bytes.
	payload := bytes.TrimSuffix(completePayload("current"), []byte("}"))
	payload = append(payload, []byte(`,"future":"`)...)
	payload = append(payload, bytes.Repeat([]byte("<"), 12<<20)...)
	payload = append(payload, []byte(`"}`)...)
	srv := completeServer(t, payload, nil)
	traj, err := fetchSession(t.Context(), daemon.NewClient(srv.URL), session)
	if err != nil {
		t.Fatal(err)
	}
	if err = cache.Write(session.SidecarPath, traj); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(session.SidecarPath)
	if err != nil || info.Size() <= 64<<20 {
		t.Fatalf("fixture must exceed RPC bound: %v %v", info, err)
	}
	if result := sessionCompleteness(session); result.State != completeness.Verified {
		t.Fatal(result)
	}
}

// A supported source whose sidecar has not been written yet is work for
// watch, so doctor counts it as stale and keeps the watch hint.
func TestCompletenessDoctorMissingSidecarIsStaleNotUnsupported(t *testing.T) {
	root, _, _ := completeFixture(t)
	r := doctorReport{}
	populateSidecarCoverage(&r, root)
	if r.total != 1 || r.sqlite != 1 || r.stale != 1 || r.unsupported != 0 || r.unknown != 1 || r.fresh != 0 {
		t.Fatalf("missing sidecar coverage: %+v", r)
	}
	var out bytes.Buffer
	writeDoctorReport(&out, r)
	if !strings.Contains(out.String(), "1 missing/stale ->") || !strings.Contains(out.String(), "agy-reader --watch") || strings.Contains(out.String(), "unsupported") {
		t.Fatalf("missing sidecar must get the watch hint, not the compatibility hint:\n%s", out.String())
	}
}
