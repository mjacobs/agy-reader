package main

import (
	"context"
	"encoding/json"
	"log"
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
	stalePB := seedPB(t, root, "implicit", "ccc", now)
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
		var dir string
		if id == "aaa" {
			dir = filepath.Join(root, "conversations")
		} else {
			dir = filepath.Join(root, "implicit")
		}
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
