package main

import (
	"bytes"
	"context"
	"errors"
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
)

func TestWatchAuthenticationFailureStopsBatchAndRecoversOnSamePort(t *testing.T) {
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "expired-token")
	root := t.TempDir()
	for _, id := range []string{"aaa", "bbb", "ccc"} {
		seedPB(t, root, "conversations", id, time.Now().Add(-time.Hour))
	}
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get(daemon.CSRFHeader) != "fresh-token" {
			http.Error(w, "expired-token", http.StatusUnauthorized)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/LoadTrajectory") {
			_, _ = w.Write([]byte(`{}`))
		} else {
			_, _ = w.Write([]byte(`{"trajectory":{"steps":[]}}`))
		}
	}))
	defer srv.Close()
	advertiseCLIDaemon(t, root, srv.URL)
	var logs bytes.Buffer
	w := watcher{
		ctx: context.Background(), root: root, baseURL: srv.URL,
		client: newDaemonClient(root, srv.URL),
		logger: log.New(&logs, "", 0), interval: time.Second, idleTimeout: time.Second,
	}
	if w.tick() {
		t.Fatal("authentication failure must not expire an active daemon as idle")
	}
	if requests.Load() != 1 {
		t.Fatalf("authentication failure should stop the batch after one RPC, got %d", requests.Load())
	}
	if !strings.Contains(logs.String(), "authentication failed; sync blocked") || strings.Contains(logs.String(), "expired-token") {
		t.Fatalf("missing safe authentication diagnostic: %s", logs.String())
	}
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "fresh-token")
	w.tick()
	for _, id := range []string{"aaa", "bbb", "ccc"} {
		if _, err := os.Stat(filepath.Join(root, "conversations", id+".trajectory.json")); err != nil {
			t.Fatalf("did not recover on unchanged URL: %v", err)
		}
	}
	if !strings.Contains(logs.String(), "3 synced") {
		t.Fatalf("expected successful batch: %s", logs.String())
	}
	// Exported credentials can disappear when a tool process exits. A
	// watcher retains its working in-memory token for an unchanged endpoint.
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "")
	other := fakeDaemon(t, nil)
	defer other.Close()
	advertiseCLIDaemon(t, root, other.URL)
	seedPB(t, root, "conversations", "ddd", time.Now().Add(-time.Hour))
	w.tick()
	if w.baseURL != srv.URL {
		t.Fatal("healthy watcher switched away from its authenticated daemon")
	}
	if _, err := os.Stat(filepath.Join(root, "conversations", "ddd.trajectory.json")); err != nil {
		t.Fatalf("lost working token when discovery became unavailable: %v", err)
	}
}

func TestHealthyIDECannotHideRejectedCLIAuthentication(t *testing.T) {
	var out bytes.Buffer
	reports := []doctorReport{
		{surface: "cli", daemonURL: "http://localhost:1234", authRejected: true},
		{surface: "ide", csrfFound: true},
	}
	if code := writeMultiDoctorReport(&out, reports, false); code == 0 {
		t.Fatalf("authentication failure was hidden by a healthy root: %s", out.String())
	}
}

func TestDoctorReportsCLIAuthenticationFailure(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/GetAllCascadeTrajectories") {
			t.Errorf("doctor should use read-only authentication check, got %s", r.URL.Path)
		}
		http.Error(w, "missing CSRF token", http.StatusUnauthorized)
	}))
	defer srv.Close()
	t.Setenv("ANTIGRAVITY_DAEMON_URL", srv.URL)
	var out bytes.Buffer
	if code := runDoctorTo(&out, []string{t.TempDir()}, true); code == 0 {
		t.Fatal("authentication failure must fail doctor even with no pending sessions")
	}
	if !strings.Contains(out.String(), "daemon:      reachable") || !strings.Contains(out.String(), "auth:        rejected") {
		t.Fatalf("doctor must distinguish auth failure from absent daemon: %s", out.String())
	}
}

func TestAuthenticationErrorsPreserveClassificationWithoutResponseSecrets(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "sensitive-token", status)
		}))
		client := daemon.NewClient(srv.URL)
		_, err := client.FetchTrajectory(t.Context(), "aaa")
		srv.Close()
		if !errors.Is(err, daemon.ErrAuthentication) || strings.Contains(err.Error(), "sensitive-token") {
			t.Fatalf("expected sanitized, classifiable error for HTTP %d", status)
		}
	}
}
