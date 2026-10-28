package daemon_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

func TestClientMessageMediaTypedViewAndRawPreservation(t *testing.T) {
	const media = `{"mimeType":"image/png","inlineData":"UEFZTE9BRA==","uri":"/fixture/image.png","futureMediaField":{"sequence":9007199254740993}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"trajectory":{"steps":[
			{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"media":[` + media + `]}},
			{"type":"CORTEX_STEP_TYPE_PLANNER_RESPONSE","plannerResponse":{"media":[` + media + `]}}
		]}}`))
	}))
	defer srv.Close()
	client := &daemon.Client{BaseURL: srv.URL, HTTP: srv.Client()}
	trajectory, err := client.GetCascadeTrajectory(t.Context(), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	for name, attachments := range map[string][]daemon.Media{
		"user":    trajectory.Steps[0].UserInput.Media,
		"planner": trajectory.Steps[1].PlannerResponse.Media,
	} {
		if len(attachments) != 1 || attachments[0].MimeType != "image/png" || attachments[0].URI != "/fixture/image.png" || attachments[0].InlineData != "UEFZTE9BRA==" {
			t.Errorf("%s media dropped from typed view: %+v", name, attachments)
		}
	}
	if strings.Count(string(trajectory.RawJSON), media) != 2 {
		t.Fatalf("raw attachment data was reshaped or discarded: %s", trajectory.RawJSON)
	}
}
