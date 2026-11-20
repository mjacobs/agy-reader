package daemon_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"google.golang.org/protobuf/encoding/protowire"
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

func TestTranscriptDistinguishesStaleTotalsFromMalformedResponse(t *testing.T) {
	fetch := func(envelope []byte) error {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(envelope)
		}))
		defer srv.Close()
		_, err := daemon.NewClient(srv.URL).GetTranscript(t.Context(), "fixture", false, 2, 0, 0)
		return err
	}

	// A valid trajectory envelope advertises one step although the current
	// SQLite snapshot contains two. Missing trajectory data is malformed.
	totals := protowire.AppendVarint(protowire.AppendTag(nil, 3, protowire.VarintType), 1)
	if err := fetch(totals); err == nil || errors.Is(err, daemon.ErrTranscriptTotalsMismatch) {
		t.Fatalf("malformed response classified as stale: %v", err)
	}
	header := protowire.AppendBytes(protowire.AppendTag(nil, 1, protowire.BytesType), nil)
	if err := fetch(append(header, totals...)); !errors.Is(err, daemon.ErrTranscriptTotalsMismatch) {
		t.Fatalf("stale totals classified as %v", err)
	}
}
