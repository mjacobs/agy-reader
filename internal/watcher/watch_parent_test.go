package watcher

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/session"
	"github.com/mjacobs/agy-reader/internal/subagent"
	"github.com/mjacobs/agy-reader/internal/testfixture"
)

// The coordinator asks for progress before the worker replies. Each round
// fetches fresh daemon payloads and runs the watcher's reconciliation pass.
func TestWatchParentMessageThenChildReplyDoesNotReverseAncestry(t *testing.T) {
	const parent = "11111111-1111-1111-1111-111111111111"
	const child = "22222222-2222-2222-2222-222222222222"
	root := t.TempDir()
	var replied atomic.Bool
	send := func(id string) daemon.Step {
		var step daemon.Step
		if err := json.Unmarshal([]byte(`{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"send_message"}},"generic":{"args":{"Recipient":"`+id+`","Message":"progress"},"result":{"result":"Message sent"}}}`), &step); err != nil {
			t.Fatal(err)
		}
		return step
	}
	receive := func(id string) daemon.Step {
		return daemon.Step{Type: "CORTEX_STEP_TYPE_SYSTEM_MESSAGE", SystemMessage: &daemon.SystemMessage{EventType: "agent_message", Message: "[Message] sender=" + id + " content=progress"}}
	}
	trajectory := func(id string) daemon.Trajectory {
		traj := daemon.Trajectory{CascadeID: id}
		if id == parent {
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
		return traj
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
		traj := trajectory(req.CascadeID)
		_, _ = w.Write(append(append([]byte(`{"trajectory":`), lineageFixtureJSON(t, traj)...), byte('}')))
	}))
	defer srv.Close()
	client := daemon.NewClient(srv.URL)
	var logs bytes.Buffer
	failures := 0
	for round := 0; round < 2; round++ {
		replied.Store(round == 1)
		for _, id := range []string{parent, child} {
			raw := lineageFixtureJSON(t, trajectory(id))
			testfixture.LegacySource(t, filepath.Join(root, "conversations", id+".pb"), raw)
		}
		synced, _, _, failed, _ := WatchTick(t.Context(), client, root, log.New(&logs, "", 0), &failures)
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

func TestParentMetadataEditsLeaveStaleSessionStale(t *testing.T) {
	const parent = "11111111-1111-1111-1111-111111111111"
	const child = "22222222-2222-2222-2222-222222222222"
	root := t.TempDir()
	verifiedAt := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	sourceTime := verifiedAt.Add(time.Hour)
	source := seedPB(t, root, "conversations", child, sourceTime)
	dir := filepath.Join(root, "conversations")
	path := filepath.Join(dir, child+".trajectory.json")
	traj := &daemon.Trajectory{CascadeID: child}
	traj.Metadata.ParentConversationID = json.RawMessage(`"` + parent + `"`)
	raw, err := json.Marshal(traj)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an existing pre-verification cache; metadata edits cannot certify it.
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	s := mkSession(child, source, path, sourceTime)
	assertStale := func() {
		t.Helper()
		if stale, reason := session.IsStale(s); !stale {
			t.Fatalf("metadata edit hid stale payload: %s", reason)
		}
	}
	assertStale()
	report, err := subagent.Backfill(dir, nil)
	if err != nil || report.Stamped != 1 {
		t.Fatalf("backfill: %+v, %v", report, err)
	}
	assertStale()
	if changed, err := cache.ClearParentCascadeID(path); err != nil || !changed {
		t.Fatalf("clear: %v, %v", changed, err)
	}
	assertStale()
}
