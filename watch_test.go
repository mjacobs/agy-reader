package main

import (
	"bytes"
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

// TestWatcherTickPendingWhenDaemonAbsent covers the graceful-startup path: a
// watcher started with no daemon URL (agy not running yet, e.g. systemd boot
// before agy) must not panic or fetch, must stay pending, and must not record a
// failure — discovery being pending is not a failed tick.
func TestWatcherTickPendingWhenDaemonAbsent(t *testing.T) {
	root := t.TempDir() // no cli.log => auto-discovery has nothing to find
	var logbuf bytes.Buffer
	logger := log.New(&logbuf, "", 0)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	w := &watcher{
		ctx:      ctx,
		root:     root,
		baseURL:  "",
		client:   daemon.NewClient(""),
		logger:   logger,
		interval: 30 * time.Second,
	}

	w.tick()

	if w.baseURL != "" {
		t.Errorf("baseURL should stay empty while the daemon is absent, got %q", w.baseURL)
	}
	if w.consecutiveFailures != 0 {
		t.Errorf("pending discovery must not count as a failure, got consecutiveFailures=%d", w.consecutiveFailures)
	}
	// The pending line must tell the operator the watcher will keep auto-discovering,
	// and at what cadence — the original "auto-discovery pending" wording left users
	// unsure whether the watcher would ever retry once agy came up.
	if out := logbuf.String(); !strings.Contains(out, "agy daemon not found yet; retrying every 30s") {
		t.Errorf("expected a pending line naming the retry cadence, got: %q", out)
	}
}

// TestWatcherTickAutoDiscoversAndSyncsFromColdStart covers the self-correction:
// a watcher that started before the daemon existed must auto-discover it once
// agy comes up and sync the backlog — without logging a phantom failure
// recovery, since the pending period was never a real fetch failure.
func TestWatcherTickAutoDiscoversAndSyncsFromColdStart(t *testing.T) {
	root := t.TempDir()
	// Missing-sidecar session that should sync once the daemon appears.
	seedPB(t, root, "conversations", "aaa", time.Now().Add(-1*time.Hour))

	var logbuf bytes.Buffer
	logger := log.New(&logbuf, "", 0)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	w := &watcher{
		ctx:     ctx,
		root:    root,
		baseURL: "",
		client:  daemon.NewClient(""),
		logger:  logger,
	}

	// 1) Cold start: daemon not up yet (no cli.log). Stays pending, no sync.
	w.tick()
	if w.baseURL != "" {
		t.Fatalf("expected still-pending baseURL on cold start, got %q", w.baseURL)
	}

	// 2) agy comes up: a real daemon now listens and cli.log advertises its port.
	srv := fakeDaemon(t, nil)
	defer srv.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("parse fake daemon url %q: %v", srv.URL, err)
	}
	logLine := "Language server listening on random port at " + port + " for HTTP\n"
	if err := os.WriteFile(filepath.Join(root, "cli.log"), []byte(logLine), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3) Next tick auto-discovers the daemon and syncs the stale session.
	w.tick()

	if w.baseURL == "" {
		t.Fatal("expected baseURL to be auto-discovered, still empty")
	}
	sidecar := filepath.Join(root, "conversations", "aaa.trajectory.json")
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("expected sidecar synced after auto-discovery: %v", err)
	}
	if w.consecutiveFailures != 0 {
		t.Errorf("consecutiveFailures should be 0 after a clean cold-start sync, got %d", w.consecutiveFailures)
	}
	out := logbuf.String()
	if strings.Contains(out, "recovered after") {
		t.Errorf("cold-start recovery must not log a phantom failure recovery, got: %q", out)
	}
	// A first-time discovery from a cold start must read as "discovered", not the
	// confusing "daemon moved  -> URL" (empty old URL) the original wording produced.
	if !strings.Contains(out, "agy daemon discovered at") {
		t.Errorf("cold-start discovery should announce 'agy daemon discovered at', got: %q", out)
	}
	if strings.Contains(out, "moved") {
		t.Errorf("cold-start discovery must not be reported as a move, got: %q", out)
	}
}

// TestRunWatchLoopIdleTimeoutExitsCleanly covers the event-driven lifecycle:
// when --watch-idle-timeout is set and the daemon never appears, the loop must
// give up and return nil (clean exit) instead of polling forever — so a
// path-triggered systemd unit relaunches it on the next agy activity.
func TestRunWatchLoopIdleTimeoutExitsCleanly(t *testing.T) {
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "") // unpinned => discovery is attempted
	root := t.TempDir()                    // no cli.log => daemon is never found

	done := make(chan error, 1)
	go func() {
		done <- runWatchLoop(context.Background(), root, "", 20*time.Millisecond, 60*time.Millisecond)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected clean nil exit on idle timeout, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runWatchLoop did not exit within 5s despite the idle timeout")
	}
}

// TestRunWatchLoopRunsForeverWhenIdleTimeoutDisabled guards the default: with
// idle-timeout unset (0), the loop must keep polling through a missing daemon
// and only stop on context cancellation (SIGINT/SIGTERM in production).
func TestRunWatchLoopRunsForeverWhenIdleTimeoutDisabled(t *testing.T) {
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "")
	root := t.TempDir() // daemon never appears

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runWatchLoop(ctx, root, "", 10*time.Millisecond, 0)
	}()

	// Many idle ticks elapse; with auto-exit disabled it must still be running.
	select {
	case err := <-done:
		t.Fatalf("loop exited on its own with idle-timeout disabled (err=%v)", err)
	case <-time.After(150 * time.Millisecond):
	}

	cancel() // the only way out when idle-timeout is disabled
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil after context cancel, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("loop did not return after context cancellation")
	}
}

// TestWatcherIdleStreakResetsWhenDaemonRecovers proves the idle timer restarts
// after the daemon comes back, so an active agy session never trips the
// auto-exit. With idleTimeout = 3*interval the loop would stop on the 3rd
// consecutive idle tick; a recovery in between must reset that count.
func TestWatcherIdleStreakResetsWhenDaemonRecovers(t *testing.T) {
	const interval = 20 * time.Millisecond
	root := t.TempDir()
	seedPB(t, root, "conversations", "aaa", time.Now().Add(-time.Hour))

	logger := log.New(&bytes.Buffer{}, "", 0)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	w := &watcher{
		ctx:         ctx,
		root:        root,
		baseURL:     "",
		client:      daemon.NewClient(""),
		logger:      logger,
		interval:    interval,
		idleTimeout: 3 * interval, // trips on the 3rd consecutive idle tick
	}

	// Two pending ticks (daemon absent) — not enough to trip on their own.
	if w.tick() || w.tick() {
		t.Fatal("pending ticks before the timeout must not stop the loop")
	}

	// agy comes up: a real daemon listens and cli.log advertises its port.
	srv := fakeDaemon(t, nil)
	defer srv.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("parse fake daemon url %q: %v", srv.URL, err)
	}
	if err := os.WriteFile(filepath.Join(root, "cli.log"),
		[]byte("Language server listening on random port at "+port+" for HTTP\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Recovery tick: discovers + syncs, which must reset the idle streak.
	if w.tick() {
		t.Fatal("recovery tick should not stop the loop")
	}
	if _, err := os.Stat(filepath.Join(root, "conversations", "aaa.trajectory.json")); err != nil {
		t.Fatalf("expected sidecar synced on recovery: %v", err)
	}

	// Daemon dies again. The very next idle tick must NOT stop: if the streak
	// had not reset it would now be the 3rd idle tick and trip the timeout.
	srv.Close()
	if w.tick() {
		t.Fatal("idle streak did not reset on recovery: loop stopped one tick after the daemon came back")
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

// TestRediscoverDaemonURLLogWording pins the operator-facing wording: a first
// discovery (empty current — agy started after the watcher) reads as
// "discovered at <url>", while a genuine port change reads as "moved". The old
// single "moved %s -> %s" line rendered "daemon moved  -> URL" on cold start,
// which read as noise rather than "agy was found".
func TestRediscoverDaemonURLLogWording(t *testing.T) {
	root := t.TempDir()

	// A live listener discovery will resolve to, advertised via cli.log.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	if err := os.WriteFile(filepath.Join(root, "cli.log"),
		[]byte("Language server listening on random port at "+port+" for HTTP\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	url := "http://127.0.0.1:" + port

	// Cold start (current == ""): "discovered at <url>", never "moved".
	var first bytes.Buffer
	if _, ok := rediscoverDaemonURL(root, "", log.New(&first, "", 0)); !ok {
		t.Fatal("expected first discovery to succeed")
	}
	if out := first.String(); !strings.Contains(out, "discovered at "+url) {
		t.Errorf("cold-start discovery should log 'discovered at %s', got: %q", url, out)
	}
	if strings.Contains(first.String(), "moved") {
		t.Errorf("cold-start discovery must not say 'moved', got: %q", first.String())
	}

	// Genuine relocation (current is a different, stale URL): "moved".
	var moved bytes.Buffer
	if _, ok := rediscoverDaemonURL(root, "http://127.0.0.1:1", log.New(&moved, "", 0)); !ok {
		t.Fatal("expected rediscovery to find the relocated daemon")
	}
	if out := moved.String(); !strings.Contains(out, "moved") {
		t.Errorf("a real port move should log 'moved', got: %q", out)
	}
}
