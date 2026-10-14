package watcher

import (
	"bytes"
	"context"
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
	"github.com/mjacobs/agy-reader/internal/session"
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
	w := Watcher{
		Ctx: context.Background(), Root: root, BaseURL: srv.URL,
		Client: session.NewDaemonClient(root, discovery.Connection{BaseURL: srv.URL}),
		Logger: log.New(&logs, "", 0), Interval: time.Second, IdleTimeout: time.Second,
	}
	if w.Tick() {
		t.Fatal("authentication failure must not expire an active daemon as idle")
	}
	if requests.Load() != 1 {
		t.Fatalf("authentication failure should stop the batch after one RPC, got %d", requests.Load())
	}
	if !strings.Contains(logs.String(), "authentication failed; sync blocked") || strings.Contains(logs.String(), "expired-token") {
		t.Fatalf("missing safe authentication diagnostic: %s", logs.String())
	}
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "fresh-token")
	w.Tick()
	for _, id := range []string{"aaa", "bbb", "ccc"} {
		if _, err := os.Stat(filepath.Join(root, "conversations", id+".trajectory.json")); err != nil {
			t.Fatalf("did not recover on unchanged URL: %v", err)
		}
	}
	if !strings.Contains(logs.String(), "3 synced") {
		t.Fatalf("expected successful batch: %s", logs.String())
	}
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "")
	other := fakeDaemon(t, nil)
	defer other.Close()
	advertiseCLIDaemon(t, root, other.URL)
	seedPB(t, root, "conversations", "ddd", time.Now().Add(-time.Hour))
	w.Tick()
	if w.BaseURL != srv.URL {
		t.Fatal("healthy watcher switched away from its authenticated daemon")
	}
	if _, err := os.Stat(filepath.Join(root, "conversations", "ddd.trajectory.json")); err != nil {
		t.Fatalf("lost working token when discovery became unavailable: %v", err)
	}
}
