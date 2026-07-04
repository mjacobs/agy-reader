package daemon_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

func TestClientFetchTrajectoryRoundTrip(t *testing.T) {
	const cascadeID = "test-cascade-1"
	var loaded, got bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected JSON content type, got %q", ct)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/LoadTrajectory"):
			loaded = true
			var req daemon.LoadTrajectoryRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode load req: %v", err)
			}
			if req.CascadeID != cascadeID {
				t.Errorf("got cascadeId %q want %q", req.CascadeID, cascadeID)
			}
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/GetCascadeTrajectory"):
			got = true
			resp := daemon.GetCascadeTrajectoryResponse{
				Trajectory: daemon.Trajectory{
					CascadeID: cascadeID,
					Source:    "CORTEX_TRAJECTORY_SOURCE_CLI",
					Steps: []daemon.Step{
						{
							Type:      "CORTEX_STEP_TYPE_USER_INPUT",
							Status:    "COMPLETED",
							UserInput: &daemon.UserInput{UserResponse: "hello"},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &daemon.Client{BaseURL: srv.URL, HTTP: srv.Client()}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	traj, err := c.FetchTrajectory(ctx, cascadeID)
	if err != nil {
		t.Fatalf("FetchTrajectory: %v", err)
	}
	if !loaded || !got {
		t.Fatalf("expected both endpoints to be hit (loaded=%v got=%v)", loaded, got)
	}
	if traj.CascadeID != cascadeID {
		t.Errorf("got cascadeId %q want %q", traj.CascadeID, cascadeID)
	}
	if len(traj.Steps) != 1 || traj.Steps[0].UserInput == nil || traj.Steps[0].UserInput.UserResponse != "hello" {
		t.Errorf("unexpected steps: %+v", traj.Steps)
	}
}

func TestClientDaemonError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"bad things"}`))
	}))
	defer srv.Close()
	c := &daemon.Client{BaseURL: srv.URL, HTTP: srv.Client()}
	_, err := c.GetCascadeTrajectory(t.Context(), "anything")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bad things") {
		t.Errorf("expected error message to surface, got %v", err)
	}
}

// The IDE daemon is launched with --csrf_token and rejects RPCs without the
// matching header; a client configured with a token must attach it to every
// request.
func TestClientCSRFHeaderAttached(t *testing.T) {
	const token = "f2aa7793-2983-4623-921c-701917453d7e"
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get(daemon.CSRFHeader); got != token {
			t.Errorf("call %d: got %s %q, want %q", calls, daemon.CSRFHeader, got, token)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := &daemon.Client{BaseURL: srv.URL, HTTP: srv.Client(), CSRFToken: token}
	if _, err := c.FetchTrajectory(t.Context(), "some-cascade"); err != nil {
		t.Fatalf("FetchTrajectory: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected the header on both RPCs, got %d call(s)", calls)
	}
}

// The CLI daemon is launched WITHOUT --csrf_token and works with no header
// today. A tokenless client must not send the header at all — not even
// empty — so the CLI path cannot regress.
func TestClientNoCSRFHeaderByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if vals := r.Header.Values(daemon.CSRFHeader); len(vals) != 0 {
			t.Errorf("unexpected %s header: %q", daemon.CSRFHeader, vals)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := daemon.NewClient(srv.URL)
	c.HTTP = srv.Client()
	if _, err := c.FetchTrajectory(t.Context(), "some-cascade"); err != nil {
		t.Fatalf("FetchTrajectory: %v", err)
	}
}

// TestClientLiveDaemon is the real smoke test. Gated by AGY_READER_LIVE=1
// because it requires `agy` to be running locally. Also requires
// AGY_READER_TEST_UUID to name a session id known to the daemon.
func TestClientLiveDaemon(t *testing.T) {
	if os.Getenv("AGY_READER_LIVE") != "1" {
		t.Skip("set AGY_READER_LIVE=1 to run against the local Antigravity-CLI daemon")
	}
	uuid := os.Getenv("AGY_READER_TEST_UUID")
	if uuid == "" {
		t.Skip("set AGY_READER_TEST_UUID=<cascade-id> to run live test")
	}
	base := os.Getenv("ANTIGRAVITY_DAEMON_URL")
	if base == "" {
		t.Skip("set ANTIGRAVITY_DAEMON_URL=<base-url> to run live test")
	}
	c := daemon.NewClient(base)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	traj, err := c.FetchTrajectory(ctx, uuid)
	if err != nil {
		t.Fatalf("FetchTrajectory: %v", err)
	}
	if traj.CascadeID == "" {
		t.Errorf("trajectory had empty cascadeId")
	}
	if len(traj.Steps) == 0 {
		t.Errorf("trajectory had zero steps")
	}
}
