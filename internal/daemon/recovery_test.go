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

func TestRejectsDuplicateJSONKeysInTrajectoryAndPages(t *testing.T) {
	t.Run("paginated trajectory with duplicate metadata", func(t *testing.T) {
		stepCalls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/GetCascadeTrajectorySteps") {
				stepCalls++
				fmt.Fprint(w, `{"steps":[{"status":"ok"}]}`)
				return
			}
			fmt.Fprint(w, `{"trajectory":{"cascadeId":"fixture","metadata":{"sourceMetadata":{"sidecar":{"sidecarId":"other"}}},"metadata":{},"steps":[]},"numTotalSteps":1}`)
		}))
		defer srv.Close()

		if _, err := daemon.NewClient(srv.URL).GetFullRawTrajectory(t.Context(), "fixture"); err == nil {
			t.Fatal("GetFullRawTrajectory accepted duplicate metadata key across paginated trajectory")
		}
		if stepCalls != 0 {
			t.Fatalf("expected 0 continuation calls after invalid trajectory envelope, got %d", stepCalls)
		}
	})

	t.Run("duplicate trajectory key in outer envelope", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"trajectory":{"cascadeId":"fixture","metadata":{"other":1},"steps":[]},"trajectory":{"cascadeId":"fixture","steps":[]}}`)
		}))
		defer srv.Close()

		c := daemon.NewClient(srv.URL)
		if _, err := c.GetFullRawTrajectory(t.Context(), "fixture"); err == nil {
			t.Fatal("GetFullRawTrajectory accepted duplicate top-level trajectory key")
		}
		if _, err := c.GetCascadeTrajectory(t.Context(), "fixture"); err == nil {
			t.Fatal("GetCascadeTrajectory accepted duplicate top-level trajectory key")
		}
	})

	for _, tc := range []struct {
		name string
		page string
	}{
		{"duplicate steps key in page envelope", `{"steps":[{"status":"bad"}],"steps":[{"status":"next"}]}`},
		{"duplicate key inside paged step", `{"steps":[{"status":"bad","status":"next"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/GetCascadeTrajectory") {
					fmt.Fprint(w, `{"trajectory":{"cascadeId":"fixture","steps":[{}]},"numTotalSteps":2}`)
					return
				}
				fmt.Fprint(w, tc.page)
			}))
			defer srv.Close()

			if _, err := daemon.NewClient(srv.URL).GetFullRawTrajectory(t.Context(), "fixture"); err == nil {
				t.Fatalf("GetFullRawTrajectory accepted %s", tc.name)
			}
		})
	}
}

func TestRecoverPartialStepsAcceptsCaseInsensitiveEnvelopeFields(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/GetCascadeTrajectory") {
			fmt.Fprint(w, `{"trajectory":{"cascadeId":"fixture","steps":[{}]},"NumTotalSteps":2}`)
			return
		}
		calls++
		fmt.Fprint(w, `{"steps":[{"status":"next"}]}`)
	}))
	defer srv.Close()

	got, err := daemon.NewClient(srv.URL).GetCascadeTrajectory(t.Context(), "fixture")
	if err != nil {
		t.Fatalf("GetCascadeTrajectory failed: %v", err)
	}
	if calls != 1 || len(got.Steps) != 2 {
		t.Fatalf("calls=%d steps=%d, want 1 and 2", calls, len(got.Steps))
	}
}

func TestRejectsNonJSONResponseWithSyntaxError(t *testing.T) {
	t.Run("GetCascadeTrajectory HTML", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/LoadTrajectory") {
				fmt.Fprint(w, `{}`)
				return
			}
			fmt.Fprint(w, `<html>Sign in</html>`)
		}))
		defer srv.Close()

		_, err := daemon.NewClient(srv.URL).FetchTrajectory(t.Context(), "fixture")
		if err == nil {
			t.Fatal("expected error for HTML response")
		}
		if strings.Contains(err.Error(), "duplicate JSON object key") || !strings.Contains(err.Error(), "invalid character '<'") {
			t.Fatalf("unexpected error for HTML response: %v", err)
		}
	})

	t.Run("continuation HTML", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/GetCascadeTrajectory") {
				fmt.Fprint(w, `{"trajectory":{"cascadeId":"fixture","steps":[{}]},"numTotalSteps":2}`)
				return
			}
			fmt.Fprint(w, `<html>Sign in</html>`)
		}))
		defer srv.Close()

		_, err := daemon.NewClient(srv.URL).GetFullRawTrajectory(t.Context(), "fixture")
		if err == nil {
			t.Fatal("expected error for HTML continuation response")
		}
		if strings.Contains(err.Error(), "duplicate JSON object key") || !strings.Contains(err.Error(), "invalid character '<'") {
			t.Fatalf("unexpected error for HTML continuation response: %v", err)
		}
	})
}

func TestRejectsCaseFoldedDuplicateEnvelopeFields(t *testing.T) {
	t.Run("conflicting numTotalSteps and NumTotalSteps", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/LoadTrajectory"):
				fmt.Fprint(w, `{}`)
			case strings.HasSuffix(r.URL.Path, "/GetCascadeTrajectorySteps"):
				fmt.Fprint(w, `{"steps":[{"status":"second"}]}`)
			default:
				fmt.Fprint(w, `{"trajectory":{"cascadeId":"fixture","steps":[{"status":"first"}]},"numTotalSteps":2,"NumTotalSteps":null}`)
			}
		}))
		defer srv.Close()

		if _, err := daemon.NewClient(srv.URL).FetchTrajectory(t.Context(), "fixture"); err == nil {
			t.Fatal("FetchTrajectory accepted conflicting numTotalSteps and NumTotalSteps")
		}
	})

	t.Run("conflicting trajectory and Trajectory", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"trajectory":{"cascadeId":"fixture","steps":[]},"Trajectory":{"cascadeId":"other","steps":[]}}`)
		}))
		defer srv.Close()

		if _, err := daemon.NewClient(srv.URL).GetCascadeTrajectory(t.Context(), "fixture"); err == nil {
			t.Fatal("GetCascadeTrajectory accepted conflicting trajectory and Trajectory")
		}
	})

	t.Run("conflicting steps and Steps in continuation", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/GetCascadeTrajectory") {
				fmt.Fprint(w, `{"trajectory":{"cascadeId":"fixture","steps":[{}]},"numTotalSteps":2}`)
				return
			}
			fmt.Fprint(w, `{"steps":[{"status":"a"}],"Steps":[{"status":"b"}]}`)
		}))
		defer srv.Close()

		if _, err := daemon.NewClient(srv.URL).GetCascadeTrajectory(t.Context(), "fixture"); err == nil {
			t.Fatal("GetCascadeTrajectory accepted conflicting steps and Steps in continuation")
		}
	})
}

func TestRecoverPartialStepsAcceptsCaseInsensitiveContinuationKey(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/GetCascadeTrajectory") {
			fmt.Fprint(w, `{"trajectory":{"cascadeId":"fixture","steps":[{}]},"NumTotalSteps":2}`)
			return
		}
		calls++
		fmt.Fprint(w, `{"Steps":[{"status":"next"}]}`)
	}))
	defer srv.Close()

	got, err := daemon.NewClient(srv.URL).GetCascadeTrajectory(t.Context(), "fixture")
	if err != nil {
		t.Fatalf("GetCascadeTrajectory failed: %v", err)
	}
	if calls != 1 || len(got.Steps) != 2 {
		t.Fatalf("calls=%d steps=%d, want 1 and 2", calls, len(got.Steps))
	}
}
