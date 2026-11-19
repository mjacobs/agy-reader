package daemon_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

func TestRecoverPartialSteps(t *testing.T) {
	for _, tc := range []struct {
		name, page string
		ok         bool
	}{
		{"success", `{"steps":[{"status":"next"}]}`, true},
		{"early end", `{}`, false}, {"overrun", `{"steps":[{},{}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/GetCascadeTrajectory") {
					fmt.Fprint(w, `{"trajectory":{"cascadeId":"fixture","steps":[{}],"unknown":1234567890123456789},"numTotalSteps":2}`)
					return
				}
				calls++
				var req struct {
					CascadeID           string
					StepOffset          int
					DisableRehydration  bool
					TrajectoryVerbosity string
				}
				if !strings.HasSuffix(r.URL.Path, "/GetCascadeTrajectorySteps") || json.NewDecoder(r.Body).Decode(&req) != nil || req.CascadeID != "fixture" || req.StepOffset != 1 || !req.DisableRehydration || req.TrajectoryVerbosity != "CLIENT_TRAJECTORY_VERBOSITY_FULL" {
					t.Error("wrong continuation")
				}
				fmt.Fprint(w, tc.page)
			}))
			defer srv.Close()
			got, err := daemon.NewClient(srv.URL).GetFullRawTrajectory(t.Context(), "fixture")
			if (err == nil) != tc.ok || calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			if tc.ok && (len(got.Steps) != 2 || !strings.Contains(string(got.RawJSON), "1234567890123456789")) {
				t.Fatal("incomplete assembly")
			}
		})
	}
}
