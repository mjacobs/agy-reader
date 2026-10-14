package cli

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

func writeFileT(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, b, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func seedPB(t *testing.T, root, bucket, id string, mtime time.Time) string {
	t.Helper()
	dir := filepath.Join(root, bucket)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, id+".pb")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	if !mtime.IsZero() {
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func markCLIRoot(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "cli.log"), []byte("log\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func markIDERoot(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "antigravity_state.pbtxt"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
}

func advertiseCLIDaemon(t *testing.T, root, srvURL string) {
	t.Helper()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(srvURL, "http://"))
	if err != nil {
		t.Fatalf("parse fake daemon url %q: %v", srvURL, err)
	}
	logLine := "Language server listening on random port at " + port + " for HTTP\n"
	if err := os.WriteFile(filepath.Join(root, "cli.log"), []byte(logLine), 0644); err != nil {
		t.Fatal(err)
	}
}

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
