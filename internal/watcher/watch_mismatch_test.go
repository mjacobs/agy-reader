package watcher

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/session"
)

func TestWatchBacksOffMismatchAndRetriesSameEndpoint(t *testing.T) {
	now := time.Now()
	old := refusalNow
	refusalNow = func() time.Time { return now }
	t.Cleanup(func() { refusalNow = old })
	root, s, _ := completeFixture(t)
	d := newPayloadDaemon(t, completePayload("stale"))
	var logs bytes.Buffer
	w := heldWatcher(t, root, d.URL, &logs)
	w.Tick()
	first := d.calls.Load()
	if first == 0 || !strings.Contains(logs.String(), "retrying in 30s") {
		t.Fatalf("no bounded mismatch refusal: %s", logs.String())
	}
	for range 5 {
		now = now.Add(time.Second)
		w.Tick()
	}
	if got := d.calls.Load(); got != first {
		t.Fatalf("unchanged mismatch fetched every tick: %d -> %d", first, got)
	}
	if refusalLines(&logs) != 1 {
		t.Fatalf("repeated mismatch logs: %s", logs.String())
	}
	if _, err := os.Stat(s.SidecarPath); !os.IsNotExist(err) {
		t.Fatalf("mismatch wrote sidecar: %v", err)
	}
	d.serve(completePayload("current"), completePayload("current"))
	now = now.Add(mismatchRetryDelay)
	w.Tick()
	if r := session.Completeness(s); r.State != "verified" {
		t.Fatalf("same endpoint did not recover: %+v; %s", r, logs.String())
	}
}

func TestMismatchBackoffReleasesOnChanges(t *testing.T) {
	for _, kind := range []string{"source", "endpoint", "sidecar", "profile"} {
		t.Run(kind, func(t *testing.T) {
			root, s, db := completeFixture(t)
			d := newPayloadDaemon(t, completePayload("stale"))
			w := heldWatcher(t, root, d.URL, new(bytes.Buffer))
			w.Tick()
			first := d.calls.Load()
			switch kind {
			case "source":
				replaceStep(t, db, completePayload("changed"))
			case "endpoint":
				next := newPayloadDaemon(t, completePayload("current"))
				w.BaseURL = next.URL
				w.Client.BaseURL = next.URL
				d = next
				first = 0
			case "sidecar":
				if err := os.WriteFile(s.SidecarPath, []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			case "profile":
				held := w.Held[s.PBPath]
				held.Profile = "old-profile"
				w.Held[s.PBPath] = held
			}
			w.Tick()
			if d.calls.Load() <= first {
				t.Fatal("change did not release mismatch backoff")
			}
		})
	}
}

func TestSyncLogExplainsMissingSidecar(t *testing.T) {
	root, _, _ := completeFixture(t)
	d := newPayloadDaemon(t, completePayload("current"))
	var logs bytes.Buffer
	w := heldWatcher(t, root, d.URL, &logs)
	w.Tick()
	if !strings.Contains(logs.String(), "[sidecar unavailable]") || strings.Contains(logs.String(), "[unknown]") {
		t.Fatalf("sync log lost the staleness reason: %s", logs.String())
	}
}
