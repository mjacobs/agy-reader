package main

import (
	"context"
	"encoding/json"
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
)

func mkSession(id, pb, sidecar string, mtime time.Time) discovery.Session {
	return discovery.Session{
		CascadeID:   id,
		PBPath:      pb,
		SidecarPath: sidecar,
		Bucket:      "conversations",
		ModTime:     mtime,
	}
}

// fakeDaemon returns a server that responds successfully to both endpoints
// for any cascadeId. perCall lets the caller observe what was fetched.
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

func TestWatchTickSyncsMissingAndStale(t *testing.T) {
	root := t.TempDir()
	now := time.Now()

	// 1) Missing sidecar — should sync.
	seedPB(t, root, "conversations", "aaa", now.Add(-1*time.Hour))
	// 2) Up-to-date — sidecar newer than .pb, should be a no-op.
	upToDatePB := seedPB(t, root, "conversations", "bbb", now.Add(-2*time.Hour))
	upToDateSidecar := strings.TrimSuffix(upToDatePB, ".pb") + ".trajectory.json"
	if err := os.WriteFile(upToDateSidecar, []byte(`{"cascadeId":"bbb"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(upToDateSidecar, now, now); err != nil {
		t.Fatal(err)
	}
	// 3) Stale — .pb newer than sidecar, should re-sync.
	stalePB := seedPB(t, root, "conversations", "ccc", now)
	staleSidecar := strings.TrimSuffix(stalePB, ".pb") + ".trajectory.json"
	if err := os.WriteFile(staleSidecar, []byte(`{"cascadeId":"ccc-old"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(staleSidecar, now.Add(-3*time.Hour), now.Add(-3*time.Hour)); err != nil {
		t.Fatal(err)
	}

	fetched := map[string]bool{}
	srv := fakeDaemon(t, func(id string) { fetched[id] = true })
	defer srv.Close()

	client := daemon.NewClient(srv.URL)
	client.HTTP = srv.Client()
	logger := log.New(os.Stderr, "test: ", 0)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	failures := 0
	synced, skipped, upToDate, failed := watchTick(ctx, client, root, logger, &failures)

	if synced != 2 {
		t.Errorf("synced: got %d want 2", synced)
	}
	if upToDate != 1 {
		t.Errorf("upToDate: got %d want 1", upToDate)
	}
	if skipped != 0 || failed != 0 {
		t.Errorf("unexpected skipped=%d failed=%d", skipped, failed)
	}
	if !fetched["aaa"] || !fetched["ccc"] {
		t.Errorf("expected to fetch aaa and ccc, got %v", fetched)
	}
	if fetched["bbb"] {
		t.Errorf("did not expect to fetch bbb (up-to-date)")
	}

	// Verify sidecar contents got written.
	for _, id := range []string{"aaa", "ccc"} {
		dir := filepath.Join(root, "conversations")
		data, err := os.ReadFile(filepath.Join(dir, id+".trajectory.json"))
		if err != nil {
			t.Errorf("read sidecar for %s: %v", id, err)
			continue
		}
		if !strings.Contains(string(data), `"cascadeId": "`+id+`"`) {
			t.Errorf("sidecar for %s missing cascadeId, got: %s", id, data)
		}
	}
}

func TestWatchTickIgnoresImplicitSessions(t *testing.T) {
	root := t.TempDir()
	seedPB(t, root, "implicit", "implicit-only", time.Now())

	fetched := map[string]bool{}
	srv := fakeDaemon(t, func(id string) { fetched[id] = true })
	defer srv.Close()

	client := daemon.NewClient(srv.URL)
	client.HTTP = srv.Client()
	logger := log.New(os.Stderr, "test: ", 0)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	failures := 0
	synced, skipped, upToDate, failed := watchTick(ctx, client, root, logger, &failures)

	if synced != 0 || skipped != 0 || upToDate != 0 || failed != 0 {
		t.Errorf("unexpected counts: synced=%d skipped=%d upToDate=%d failed=%d", synced, skipped, upToDate, failed)
	}
	if len(fetched) != 0 {
		t.Errorf("implicit sessions should not be fetched, got %v", fetched)
	}
}

func TestListSessionsForDisplayRequiresExplicitImplicitOptIn(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	seedPB(t, root, "conversations", "conversation", now.Add(-time.Hour))
	seedPB(t, root, "implicit", "implicit", now)

	got, err := listSessionsForDisplay(root, false)
	if err != nil {
		t.Fatalf("listSessionsForDisplay default: %v", err)
	}
	if len(got) != 1 || got[0].CascadeID != "conversation" {
		t.Fatalf("default list should include only conversations, got %+v", got)
	}

	got, err = listSessionsForDisplay(root, true)
	if err != nil {
		t.Fatalf("listSessionsForDisplay include implicit: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("include implicit should include both buckets, got %+v", got)
	}
	if got[0].CascadeID != "implicit" || got[0].Bucket != "implicit" {
		t.Fatalf("expected newest implicit session first with opt-in, got %+v", got)
	}
}

func TestWatchTickDaemonDown(t *testing.T) {
	root := t.TempDir()
	seedPB(t, root, "conversations", "aaa", time.Now())

	// Closed server simulates connection refused.
	srv := fakeDaemon(t, nil)
	srv.Close()

	client := daemon.NewClient(srv.URL)
	logger := log.New(os.Stderr, "test: ", 0)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	failures := 0
	synced, _, _, failed := watchTick(ctx, client, root, logger, &failures)
	if synced != 0 {
		t.Errorf("expected 0 synced, got %d", synced)
	}
	if failed == 0 {
		t.Errorf("expected at least one failure")
	}
	if failures != 1 {
		t.Errorf("expected consecutiveFailures=1, got %d", failures)
	}
}

func TestIsStale(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	pbPath := seedPB(t, root, "conversations", "xxx", now)
	sidecarPath := strings.TrimSuffix(pbPath, ".pb") + ".trajectory.json"

	// No sidecar: stale.
	s := mkSession("xxx", pbPath, sidecarPath, now)
	stale, reason := isStale(s)
	if !stale || reason != "missing" {
		t.Errorf("expected missing, got stale=%v reason=%q", stale, reason)
	}

	// Sidecar newer than .pb: not stale.
	if err := os.WriteFile(sidecarPath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := now.Add(1 * time.Hour)
	if err := os.Chtimes(sidecarPath, future, future); err != nil {
		t.Fatal(err)
	}
	stale, _ = isStale(s)
	if stale {
		t.Errorf("expected not stale when sidecar newer")
	}

	// .pb advances past sidecar: stale.
	pbFuture := future.Add(1 * time.Hour)
	if err := os.Chtimes(pbPath, pbFuture, pbFuture); err != nil {
		t.Fatal(err)
	}
	s.ModTime = pbFuture
	stale, reason = isStale(s)
	if !stale || reason != "modtime-advanced" {
		t.Errorf("expected modtime-advanced, got stale=%v reason=%q", stale, reason)
	}
}

func seedDB(t *testing.T, root, bucket, id string, mtime time.Time, walMtime time.Time) string {
	t.Helper()
	dir := filepath.Join(root, bucket)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, id+".db")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if !mtime.IsZero() {
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	if !walMtime.IsZero() {
		// Non-empty: an empty WAL (fully checkpointed) is ignored by discovery.
		walPath := path + "-wal"
		if err := os.WriteFile(walPath, []byte("frame"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(walPath, walMtime, walMtime); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestWatchTickSyncsStaleDBWithWal(t *testing.T) {
	root := t.TempDir()
	now := time.Now().Truncate(time.Second)

	// DB is old, but WAL is brand new. Sidecar is matching DB time, so it's stale relative to WAL.
	dbPath := seedDB(t, root, "conversations", "ddd", now.Add(-2*time.Hour), now)
	sidecarPath := strings.TrimSuffix(dbPath, ".db") + ".trajectory.json"

	if err := os.WriteFile(sidecarPath, []byte(`{"cascadeId":"ddd-old"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Sidecar modified time matches old db file time
	if err := os.Chtimes(sidecarPath, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}

	fetched := map[string]bool{}
	srv := fakeDaemon(t, func(id string) { fetched[id] = true })
	defer srv.Close()

	client := daemon.NewClient(srv.URL)
	client.HTTP = srv.Client()
	logger := log.New(os.Stderr, "test: ", 0)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	failures := 0
	synced, skipped, upToDate, failed := watchTick(ctx, client, root, logger, &failures)

	if synced != 1 {
		t.Errorf("synced: got %d want 1", synced)
	}
	if upToDate != 0 {
		t.Errorf("upToDate: got %d want 0", upToDate)
	}
	if skipped != 0 || failed != 0 {
		t.Errorf("unexpected skipped=%d failed=%d", skipped, failed)
	}
	if !fetched["ddd"] {
		t.Errorf("expected to fetch ddd, got %v", fetched)
	}

	// Verify sidecar contents got updated.
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("read sidecar for ddd: %v", err)
	}
	if !strings.Contains(string(data), `"cascadeId": "ddd"`) {
		t.Errorf("sidecar for ddd missing updated cascadeId, got: %s", data)
	}
}

func TestRediscoverDaemonURL(t *testing.T) {
	root := t.TempDir()
	logger := log.New(os.Stderr, "test: ", 0)

	// No cli.log yet: discovery fails, keep current URL.
	if _, ok := rediscoverDaemonURL(root, "http://127.0.0.1:1", logger); ok {
		t.Error("expected rediscovery to fail without cli.log")
	}

	// Live listener simulating the relocated daemon.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	logLine := "Language server listening on random port at " + port + " for HTTP\n"
	if err := os.WriteFile(filepath.Join(root, "cli.log"), []byte(logLine), 0o644); err != nil {
		t.Fatal(err)
	}

	next, ok := rediscoverDaemonURL(root, "http://127.0.0.1:1", logger)
	if !ok {
		t.Fatal("expected rediscovery to find the new daemon")
	}
	want := "http://127.0.0.1:" + port
	if next != want {
		t.Errorf("got %q want %q", next, want)
	}

	// Same URL as current: not a move, ok=false.
	if _, ok := rediscoverDaemonURL(root, want, logger); ok {
		t.Error("expected ok=false when daemon URL is unchanged")
	}
}
