package watcher

import (
	"bytes"
	"context"
	"encoding/json"
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

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/session"
	"github.com/mjacobs/agy-reader/internal/testfixture"
)

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
	synced, skipped, upToDate, failed, _ := WatchTick(ctx, client, root, logger, &failures)

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
		var got daemon.Trajectory
		if err := json.Unmarshal(data, &got); err != nil || got.CascadeID != id {
			t.Errorf("sidecar for %s missing cascadeId, got: %s (err=%v)", id, data, err)
		}
	}
}

func TestWatchTickResolvesParentsAfterWritingWholeBatch(t *testing.T) {
	const (
		parent  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		childA  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		childB  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		execID  = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		promptA = "Inspect the IDE sessions."
		promptB = "Inspect the CLI sessions."
	)
	root := t.TempDir()
	for _, id := range []string{parent, childA, childB} {
		seedPB(t, root, "conversations", id, time.Now())
	}
	invokeArgs, err := json.Marshal(map[string]any{"Subagents": []map[string]any{
		{"Prompt": promptA, "TypeName": "self"},
		{"Prompt": promptB, "TypeName": "self"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	trajectories := map[string]daemon.Trajectory{
		parent: {
			CascadeID: parent,
			Steps: []daemon.Step{
				{
					Type:     "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
					Metadata: daemon.StepMetadata{ExecutionID: execID},
					PlannerResponse: &daemon.PlannerResponse{ToolCalls: []daemon.ToolCall{{
						Name: "invoke_subagent", ArgumentsJSON: string(invokeArgs),
					}}},
				},
				{Type: "CORTEX_STEP_TYPE_INVOKE_SUBAGENT", Metadata: daemon.StepMetadata{ExecutionID: execID}},
				{
					Type:            "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
					Metadata:        daemon.StepMetadata{ExecutionID: execID},
					PlannerResponse: &daemon.PlannerResponse{Response: "Spawned " + childA + " and " + childB},
				},
				{Type: "CORTEX_STEP_TYPE_SYSTEM_MESSAGE", SystemMessage: &daemon.SystemMessage{EventType: "agent_message", Message: "[Message] sender=" + childA + " content=done"}},
				{Type: "CORTEX_STEP_TYPE_SYSTEM_MESSAGE", SystemMessage: &daemon.SystemMessage{EventType: "agent_message", Message: "[Message] sender=" + childB + " content=done"}},
			},
		},
		childA: modernChildTrajectory(t, childA, parent, promptA),
		childB: modernChildTrajectory(t, childB, parent, promptB),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/LoadTrajectory") {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		var req daemon.GetCascadeTrajectoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		_ = json.NewEncoder(w).Encode(daemon.GetCascadeTrajectoryResponse{Trajectory: trajectories[req.CascadeID]})
	}))
	defer srv.Close()
	client := daemon.NewClient(srv.URL)
	client.HTTP = srv.Client()
	var logs bytes.Buffer
	failures := 0
	synced, skipped, upToDate, failed, _ := WatchTick(t.Context(), client, root, log.New(&logs, "", 0), &failures)
	if synced != 3 || skipped != 0 || upToDate != 0 || failed != 0 {
		t.Fatalf("unexpected counts: synced=%d skipped=%d upToDate=%d failed=%d\n%s", synced, skipped, upToDate, failed, logs.String())
	}
	for _, child := range []string{childA, childB} {
		traj, err := cache.Read(filepath.Join(root, "conversations", child+".trajectory.json"))
		if err != nil {
			t.Fatal(err)
		}
		if got := traj.ParentCascadeID(); got != parent {
			t.Errorf("child %s parent = %q, want %q", child, got, parent)
		}
	}
}

func modernChildTrajectory(t *testing.T, child, parent, prompt string) daemon.Trajectory {
	t.Helper()
	generic, err := json.Marshal(map[string]any{"args": map[string]any{
		"Message": "done", "Recipient": parent,
	}})
	if err != nil {
		t.Fatal(err)
	}
	return daemon.Trajectory{
		CascadeID: child,
		Steps: []daemon.Step{
			{Type: "CORTEX_STEP_TYPE_USER_INPUT", UserInput: &daemon.UserInput{UserResponse: prompt}},
			{Type: "CORTEX_STEP_TYPE_GENERIC", Generic: generic},
		},
	}
}

func TestWatchTickIgnoresImplicitSessions(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	seedPB(t, root, "conversations", "conversation", now.Add(-time.Hour))
	seedPB(t, root, "implicit", "implicit", now.Add(-time.Hour))

	fetched := map[string]bool{}
	srv := fakeDaemon(t, func(id string) { fetched[id] = true })
	defer srv.Close()

	client := daemon.NewClient(srv.URL)
	client.HTTP = srv.Client()
	logger := log.New(os.Stderr, "test: ", 0)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	failures := 0
	synced, _, _, failed, _ := WatchTick(ctx, client, root, logger, &failures)
	if synced != 1 || failed != 0 {
		t.Fatalf("expected 1 synced, got %d (failed=%d)", synced, failed)
	}
	if !fetched["conversation"] {
		t.Errorf("expected conversation session to sync")
	}
	if fetched["implicit"] {
		t.Errorf("implicit session must never sync via --watch")
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
	synced, _, _, failed, _ := WatchTick(ctx, client, root, logger, &failures)
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
	stale, reason := session.IsStale(s)
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
	stale, _ = session.IsStale(s)
	if stale {
		t.Errorf("expected not stale when sidecar newer")
	}

	// .pb advances past sidecar: stale.
	pbFuture := future.Add(1 * time.Hour)
	if err := os.Chtimes(pbPath, pbFuture, pbFuture); err != nil {
		t.Fatal(err)
	}
	s.ModTime = pbFuture
	stale, reason = session.IsStale(s)
	if !stale || reason != "modtime-advanced" {
		t.Errorf("expected modtime-advanced, got stale=%v reason=%q", stale, reason)
	}
}

func TestWatchTickSyncsStaleDBWithWal(t *testing.T) {
	root := t.TempDir()
	now := time.Now().Truncate(time.Second)

	dbPath := seedDB(t, root, "conversations", "ddd", now.Add(-2*time.Hour), now)
	sidecarPath := strings.TrimSuffix(dbPath, ".db") + ".trajectory.json"

	if err := os.WriteFile(sidecarPath, []byte(`{"cascadeId":"ddd-old"}`), 0o644); err != nil {
		t.Fatal(err)
	}
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
	synced, skipped, upToDate, failed, _ := WatchTick(ctx, client, root, logger, &failures)

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

	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("read sidecar for ddd: %v", err)
	}
	var got daemon.Trajectory
	if err := json.Unmarshal(data, &got); err != nil || got.CascadeID != "ddd" {
		t.Errorf("sidecar for ddd missing updated cascadeId, got: %s (err=%v)", data, err)
	}
}

func TestWatchTickDoesNotResyncWhenDaemonContentMatchesExistingSidecar(t *testing.T) {
	root := t.TempDir()
	now := time.Now().Truncate(time.Second)

	dbPath := seedDB(t, root, "conversations", "eee", now.Add(-10*time.Minute), time.Time{})
	sidecarPath := strings.TrimSuffix(dbPath, ".db") + ".trajectory.json"

	if err := os.WriteFile(sidecarPath, []byte("{\"cascadeId\":\"eee\",\"steps\":[]}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(sidecarPath, now.Add(-20*time.Minute), now.Add(-20*time.Minute)); err != nil {
		t.Fatal(err)
	}

	fetchCount := 0
	srv := fakeDaemon(t, func(id string) { fetchCount++ })
	defer srv.Close()

	client := daemon.NewClient(srv.URL)
	client.HTTP = srv.Client()
	logger := log.New(io.Discard, "", 0)
	ctx := t.Context()

	failures := 0
	synced, _, upToDate, _, _ := WatchTick(ctx, client, root, logger, &failures)
	if synced != 1 || upToDate != 0 || fetchCount != 2 {
		t.Fatalf("tick 1: got synced=%d upToDate=%d fetchCount=%d want 1, 0, 2", synced, upToDate, fetchCount)
	}

	synced, _, upToDate, _, _ = WatchTick(ctx, client, root, logger, &failures)
	if synced != 0 || upToDate != 1 || fetchCount != 2 {
		t.Fatalf("tick 2: got synced=%d upToDate=%d fetchCount=%d want 0, 1, 2 (re-synced unnecessarily!)", synced, upToDate, fetchCount)
	}

	written, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(sidecarPath, now.Add(-20*time.Minute), now.Add(-20*time.Minute)); err != nil {
		t.Fatal(err)
	}
	synced, _, upToDate, _, _ = WatchTick(ctx, client, root, logger, &failures)
	if synced != 0 || upToDate != 1 || fetchCount != 2 {
		t.Fatalf("tick 3: got synced=%d upToDate=%d fetchCount=%d want 0, 1, 2", synced, upToDate, fetchCount)
	}
	verified, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(written, verified) {
		t.Errorf("tick 3 rewrote a byte-identical sidecar:\n got: %s\nwant: %s", verified, written)
	}
	info, err := os.Stat(sidecarPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(now.Add(-20 * time.Minute)) {
		t.Errorf("tick 3 sidecar mtime: got %v want %v (source mtime)", info.ModTime(), now.Add(-20*time.Minute))
	}

	synced, _, upToDate, _, _ = WatchTick(ctx, client, root, logger, &failures)
	if synced != 0 || upToDate != 1 || fetchCount != 2 {
		t.Fatalf("tick 4: got synced=%d upToDate=%d fetchCount=%d want 0, 1, 2", synced, upToDate, fetchCount)
	}
}

func TestWatcherTickPendingWhenDaemonAbsent(t *testing.T) {
	root := t.TempDir()
	var logbuf bytes.Buffer
	logger := log.New(&logbuf, "", 0)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	w := &Watcher{
		Ctx:      ctx,
		Root:     root,
		BaseURL:  "",
		Client:   daemon.NewClient(""),
		Logger:   logger,
		Interval: 30 * time.Second,
	}

	w.Tick()

	if w.BaseURL != "" {
		t.Errorf("baseURL should stay empty while the daemon is absent, got %q", w.BaseURL)
	}
	if w.ConsecutiveFailures != 0 {
		t.Errorf("pending discovery must not count as a failure, got consecutiveFailures=%d", w.ConsecutiveFailures)
	}
	if out := logbuf.String(); !strings.Contains(out, "agy daemon not found yet; retrying every 30s") {
		t.Errorf("expected a pending line naming the retry cadence, got: %q", out)
	}
}

func TestWatcherTickAutoDiscoversAndSyncsFromColdStart(t *testing.T) {
	root := t.TempDir()
	seedPB(t, root, "conversations", "aaa", time.Now().Add(-1*time.Hour))

	var logbuf bytes.Buffer
	logger := log.New(&logbuf, "", 0)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	w := &Watcher{
		Ctx:     ctx,
		Root:    root,
		BaseURL: "",
		Client:  daemon.NewClient(""),
		Logger:  logger,
	}

	w.Tick()
	if w.BaseURL != "" {
		t.Fatalf("expected still-pending baseURL on cold start, got %q", w.BaseURL)
	}

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

	w.Tick()

	if w.BaseURL == "" {
		t.Fatal("expected baseURL to be auto-discovered, still empty")
	}
	sidecar := filepath.Join(root, "conversations", "aaa.trajectory.json")
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("expected sidecar synced after auto-discovery: %v", err)
	}
	if w.ConsecutiveFailures != 0 {
		t.Errorf("consecutiveFailures should be 0 after a clean cold-start sync, got %d", w.ConsecutiveFailures)
	}
	out := logbuf.String()
	if strings.Contains(out, "recovered after") {
		t.Errorf("cold-start recovery must not log a phantom failure recovery, got: %q", out)
	}
	if !strings.Contains(out, "agy daemon discovered at") {
		t.Errorf("cold-start discovery should announce 'agy daemon discovered at', got: %q", out)
	}
	if strings.Contains(out, "moved") {
		t.Errorf("cold-start discovery must not be reported as a move, got: %q", out)
	}
}

func TestRunWatchLoopIdleTimeoutExitsCleanly(t *testing.T) {
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "")
	root := t.TempDir()

	done := make(chan error, 1)
	go func() {
		done <- RunLoop(context.Background(), []string{root}, 20*time.Millisecond, 60*time.Millisecond)
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

func TestRunWatchLoopRunsForeverWhenIdleTimeoutDisabled(t *testing.T) {
	t.Setenv("ANTIGRAVITY_DAEMON_URL", "")
	root := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunLoop(ctx, []string{root}, 10*time.Millisecond, 0)
	}()

	select {
	case err := <-done:
		t.Fatalf("loop exited on its own with idle-timeout disabled (err=%v)", err)
	case <-time.After(150 * time.Millisecond):
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil after context cancel, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("loop did not return after context cancellation")
	}
}

func TestWatcherIdleStreakResetsWhenDaemonRecovers(t *testing.T) {
	const interval = 20 * time.Millisecond
	root := t.TempDir()
	seedPB(t, root, "conversations", "aaa", time.Now().Add(-time.Hour))

	logger := log.New(&bytes.Buffer{}, "", 0)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	w := &Watcher{
		Ctx:         ctx,
		Root:        root,
		BaseURL:     "",
		Client:      daemon.NewClient(""),
		Logger:      logger,
		Interval:    interval,
		IdleTimeout: 3 * interval,
	}

	tick1 := w.Tick()
	tick2 := w.Tick()
	if tick1 || tick2 {
		t.Fatal("pending ticks before the timeout must not stop the loop")
	}

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

	if w.Tick() {
		t.Fatal("a tick that recovers the daemon must not trip the idle timeout")
	}
	if w.IdleTicks != 0 {
		t.Errorf("idleTicks should be reset to 0 after daemon recovery, got %d", w.IdleTicks)
	}
	if w.BaseURL == "" {
		t.Errorf("daemon should be discovered after recovery tick")
	}

	srv.Close()
	_ = os.Remove(filepath.Join(root, "cli.log"))

	tick1 = w.Tick()
	tick2 = w.Tick()
	if tick1 || tick2 {
		t.Fatal("should not trip after only 2 subsequent idle ticks")
	}
	if !w.Tick() {
		t.Fatal("should trip on the 3rd consecutive idle tick after the reset")
	}
}

func TestRediscoverDaemonConnection(t *testing.T) {
	root := t.TempDir()
	logger := log.New(os.Stderr, "test: ", 0)

	if _, ok := RediscoverDaemonConnection(root, "http://127.0.0.1:1", logger); ok {
		t.Error("expected rediscovery to fail without cli.log")
	}

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

	next, ok := RediscoverDaemonConnection(root, "http://127.0.0.1:1", logger)
	if !ok {
		t.Fatal("expected rediscovery to find the new daemon")
	}
	want := "http://127.0.0.1:" + port
	if next.BaseURL != want {
		t.Errorf("got %q want %q", next.BaseURL, want)
	}

	if _, ok := RediscoverDaemonConnection(root, want, logger); ok {
		t.Error("expected ok=false when daemon URL is unchanged")
	}
}

func TestRediscoverDaemonConnectionLogWording(t *testing.T) {
	root := t.TempDir()

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

	var first bytes.Buffer
	if _, ok := RediscoverDaemonConnection(root, "", log.New(&first, "", 0)); !ok {
		t.Fatal("expected first discovery to succeed")
	}
	if out := first.String(); !strings.Contains(out, "discovered at "+url) {
		t.Errorf("cold-start discovery should log 'discovered at %s', got: %q", url, out)
	}
	if strings.Contains(first.String(), "moved") {
		t.Errorf("cold-start discovery must not say 'moved', got: %q", first.String())
	}

	var moved bytes.Buffer
	if _, ok := RediscoverDaemonConnection(root, "http://127.0.0.1:1", log.New(&moved, "", 0)); !ok {
		t.Fatal("expected rediscovery to find the relocated daemon")
	}
	if out := moved.String(); !strings.Contains(out, "moved") {
		t.Errorf("a real port move should log 'moved', got: %q", out)
	}
}

func TestNewDaemonClientTokenWiring(t *testing.T) {
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "")
	cliRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(cliRoot, "cli.log"), []byte("log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c := session.NewDaemonClient(cliRoot, discovery.Connection{BaseURL: "http://127.0.0.1:1"}); c.CSRFToken != "" {
		t.Errorf("CLI root client should have no CSRF token, got %q", c.CSRFToken)
	}

	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "pinned-token")
	if c := session.NewDaemonClient(cliRoot, discovery.Connection{BaseURL: "http://127.0.0.1:1"}); c.CSRFToken != "pinned-token" {
		t.Errorf("pinned token should be attached, got %q", c.CSRFToken)
	}
}

func TestWatchTickMismatchedInternalCascadeID(t *testing.T) {
	const (
		filenameID = "11111111-1111-4111-8111-111111111111"
		internalID = "22222222-2222-4222-8222-222222222222"
	)
	root := t.TempDir()
	convDir := filepath.Join(root, "conversations")
	if err := os.MkdirAll(convDir, 0o755); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(convDir, filenameID+".db")
	testfixture.Source(t, dbPath, []byte(`{"cascadeId":"`+internalID+`","metadata":{}}`))

	var loadCalled, filenameGetCalled, internalGetCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/LoadTrajectory"):
			loadCalled = true
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/GetCascadeTrajectory"):
			var req daemon.GetCascadeTrajectoryRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.CascadeID == filenameID {
				filenameGetCalled = true
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"code":"unknown","message":"trajectory not found"}`))
				return
			}
			if req.CascadeID == internalID {
				internalGetCalled = true
				resp := daemon.GetCascadeTrajectoryResponse{
					Trajectory: daemon.Trajectory{
						CascadeID: internalID,
						Steps:     []daemon.Step{},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := daemon.NewClient(srv.URL)
	client.HTTP = srv.Client()
	failures := 0
	logger := log.New(io.Discard, "", 0)

	synced, skipped, upToDate, failed, authBlocked := WatchTick(t.Context(), client, root, logger, &failures)
	if !loadCalled {
		t.Error("LoadTrajectory not called")
	}
	if filenameGetCalled {
		t.Error("GetCascadeTrajectory must use source identity directly")
	}
	if !internalGetCalled {
		t.Error("GetCascadeTrajectory(internalID) not called")
	}
	if synced != 1 || failed != 0 || skipped != 0 || upToDate != 0 || authBlocked {
		t.Errorf("tick 1 counts: synced=%d skipped=%d upToDate=%d failed=%d authBlocked=%v",
			synced, skipped, upToDate, failed, authBlocked)
	}

	sidecarPath := filepath.Join(convDir, filenameID+".trajectory.json")
	if !cache.Exists(sidecarPath) {
		t.Fatalf("sidecar was not written to %s", sidecarPath)
	}

	synced, skipped, upToDate, failed, authBlocked = WatchTick(t.Context(), client, root, logger, &failures)
	if synced != 0 || upToDate != 1 || failed != 0 || skipped != 0 || authBlocked {
		t.Errorf("tick 2 counts: synced=%d skipped=%d upToDate=%d failed=%d authBlocked=%v", synced, skipped, upToDate, failed, authBlocked)
	}
}

func TestRediscoverDaemonConnectionKeepsTokenOnUnchangedEndpoint(t *testing.T) {
	root := t.TempDir()
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
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "scanned-token")

	next, moved := RediscoverDaemonConnection(root, url, log.New(io.Discard, "", 0))
	if moved {
		t.Error("an unchanged endpoint must not report a move")
	}
	if next.BaseURL != url {
		t.Errorf("BaseURL = %q, want the unchanged endpoint %q", next.BaseURL, url)
	}
	if next.Token != "scanned-token" {
		t.Errorf("Token = %q, want the token from the same scan", next.Token)
	}
}

// An idle tick (every conversation already synced) after agy exits probes the
// daemon and counts a failure, but logs the unreachable line only once per
// failure streak. A tick that reaches the daemon ends the streak, so the next
// outage logs the line again.
func TestWatchTickIdleProbeLogsUnreachableOncePerStreak(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	pb := seedPB(t, root, "conversations", "synced", now.Add(-time.Hour))
	sidecar := strings.TrimSuffix(pb, ".pb") + ".trajectory.json"
	if err := os.WriteFile(sidecar, []byte(`{"cascadeId":"synced"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(sidecar, now, now); err != nil {
		t.Fatal(err)
	}

	live := fakeDaemon(t, nil)
	defer live.Close()
	liveClient := daemon.NewClient(live.URL)
	liveClient.HTTP = live.Client()

	dead := fakeDaemon(t, nil)
	dead.Close()
	deadClient := daemon.NewClient(dead.URL)

	var logs bytes.Buffer
	logger := log.New(&logs, "", 0)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	const line = "daemon unreachable during a tick without daemon contact"
	failures := 0
	idleTicks := func(client *daemon.Client, n int) {
		t.Helper()
		for range n {
			synced, _, upToDate, failed, _ := WatchTick(ctx, client, root, logger, &failures)
			if synced != 0 || failed != 0 || upToDate != 1 {
				t.Fatalf("expected an idle tick (1 up-to-date), got synced=%d upToDate=%d failed=%d", synced, upToDate, failed)
			}
		}
	}

	idleTicks(deadClient, 4)
	if got := strings.Count(logs.String(), line); got != 1 {
		t.Fatalf("expected the unreachable line once across 4 idle ticks, got %d:\n%s", got, logs.String())
	}
	if failures != 4 {
		t.Fatalf("each failed probe must still count toward the idle timeout: consecutiveFailures=%d, want 4", failures)
	}

	idleTicks(liveClient, 1)
	if failures != 0 {
		t.Fatalf("a tick that reaches the daemon must reset the streak, got consecutiveFailures=%d", failures)
	}

	idleTicks(deadClient, 3)
	if got := strings.Count(logs.String(), line); got != 2 {
		t.Fatalf("expected the unreachable line again after the streak reset (2 total), got %d:\n%s", got, logs.String())
	}
	if failures != 3 {
		t.Fatalf("consecutiveFailures=%d after the second outage, want 3", failures)
	}
}
