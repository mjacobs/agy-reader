package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/daemon"
)

// The coordinator asks for progress before the worker replies. Each round
// fetches fresh daemon payloads and runs the watcher's reconciliation pass.
func TestWatchParentMessageThenChildReplyDoesNotReverseAncestry(t *testing.T) {
	const parent = "11111111-1111-1111-1111-111111111111"
	const child = "22222222-2222-2222-2222-222222222222"
	root := t.TempDir()
	var replied atomic.Bool
	send := func(id string) daemon.Step {
		return daemon.Step{Type: "CORTEX_STEP_TYPE_GENERIC", Generic: json.RawMessage(`{"args":{"Recipient":"` + id + `","Message":"progress"}}`)}
	}
	receive := func(id string) daemon.Step {
		return daemon.Step{Type: "CORTEX_STEP_TYPE_SYSTEM_MESSAGE", SystemMessage: &daemon.SystemMessage{EventType: "agent_message", Message: "[Message] sender=" + id + " content=progress"}}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/LoadTrajectory") {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		var req daemon.GetCascadeTrajectoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		traj := daemon.Trajectory{CascadeID: req.CascadeID}
		if req.CascadeID == parent {
			traj.Steps = []daemon.Step{
				{Type: "CORTEX_STEP_TYPE_INVOKE_SUBAGENT", InvokeSubagent: json.RawMessage(`{"results":[{"conversationId":"` + child + `"}]}`)}, send(child),
			}
			if replied.Load() {
				traj.Steps = append(traj.Steps, receive(child))
			}
		} else {
			traj.Metadata.ParentConversationID = json.RawMessage(`"` + parent + `"`)
			traj.Steps = []daemon.Step{receive(parent)}
			if replied.Load() {
				traj.Steps = append(traj.Steps, send(parent))
			}
		}
		_ = json.NewEncoder(w).Encode(daemon.GetCascadeTrajectoryResponse{Trajectory: traj})
	}))
	defer srv.Close()
	client := daemon.NewClient(srv.URL)
	var logs bytes.Buffer
	failures := 0
	for round := 0; round < 2; round++ {
		replied.Store(round == 1)
		for _, id := range []string{parent, child} {
			seedPB(t, root, "conversations", id, time.Now().Add(time.Duration(round)*time.Minute))
		}
		synced, _, _, failed, _ := watchTick(t.Context(), client, root, log.New(&logs, "", 0), &failures)
		if synced != 2 || failed != 0 {
			t.Fatalf("round %d: synced=%d failed=%d\n%s", round, synced, failed, logs.String())
		}
		for id, want := range map[string]string{parent: "", child: parent} {
			traj, err := cache.Read(filepath.Join(root, "conversations", id+".trajectory.json"))
			if err != nil {
				t.Fatal(err)
			}
			if got := traj.StampedParentCascadeID(); got != want {
				t.Fatalf("round %d: %s parent=%s want=%s", round, id, got, want)
			}
		}
	}
}
