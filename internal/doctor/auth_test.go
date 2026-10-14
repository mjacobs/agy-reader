package doctor

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthyIDECannotHideRejectedCLIAuthentication(t *testing.T) {
	var out bytes.Buffer
	reports := []Report{
		{Surface: "cli", DaemonURL: "http://localhost:1234", AuthRejected: true},
		{Surface: "ide", CSRFFound: true},
	}
	if code := WriteMultiReport(&out, reports, false); code == 0 {
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
	if code := RunTo(&out, []string{t.TempDir()}, true); code == 0 {
		t.Fatal("authentication failure must fail doctor even with no pending sessions")
	}
	if !strings.Contains(out.String(), "daemon:      reachable") || !strings.Contains(out.String(), "auth:        rejected") {
		t.Fatalf("doctor must distinguish auth failure from absent daemon: %s", out.String())
	}
}
