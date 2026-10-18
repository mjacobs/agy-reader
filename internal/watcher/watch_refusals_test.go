package watcher

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/session"
	"github.com/mjacobs/agy-reader/internal/testfixture"
	"github.com/mjacobs/agy-reader/internal/vcache"
)

// An unchanged source whose projection the profile does not support is fetched
// once, then held without daemon calls; a content change and an endpoint
// change each earn exactly one retry and one log line.
func TestWatchHoldsUnchangedUnknownSourceWithoutDaemonCalls(t *testing.T) {
	root, s, db := unsupportedProjectionFixture(t)
	d := newPayloadDaemon(t, unsupportedProjectionPayload("first"))
	var logs bytes.Buffer
	w := heldWatcher(t, root, d.URL, &logs)

	w.Tick()
	first := d.calls.Load()
	if first == 0 {
		t.Fatal("first tick did not fetch the unsupported source")
	}
	if got := refusalLines(&logs); got != 1 || !strings.Contains(logs.String(), "(completeness unknown: unsupported generic arguments); not retrying") {
		t.Fatalf("first refusal not held and logged once (%d):\n%s", got, logs.String())
	}
	for range 3 {
		w.Tick()
	}
	if got := d.calls.Load(); got != first {
		t.Fatalf("unchanged unsupported source fetched again: %d daemon calls after first tick's %d", got, first)
	}
	if got := refusalLines(&logs); got != 1 {
		t.Fatalf("refusal logged on every tick (%d lines):\n%s", got, logs.String())
	}
	if !strings.Contains(logs.String(), "tick: 0 synced, 1 skipped, 0 up-to-date, 1 failed") ||
		!strings.Contains(logs.String(), "tick: 0 synced, 1 skipped, 0 up-to-date, 0 failed") {
		t.Fatalf("held source not reported as skipped:\n%s", logs.String())
	}
	if w.ConsecutiveFailures != 0 {
		t.Fatalf("reachable daemon counted as failing: %d", w.ConsecutiveFailures)
	}
	if _, err := os.Stat(s.SidecarPath); !os.IsNotExist(err) {
		t.Fatalf("refused source wrote a sidecar: %v", err)
	}

	// Content change, which the daemon also serves: retried on the very next
	// tick, refused for the same reason, then held again.
	moved := unsupportedProjectionPayload("the source moved on")
	replaceStep(t, db, moved)
	d.serve(moved, moved)
	w.Tick()
	afterChange := d.calls.Load()
	if afterChange == first {
		t.Fatal("changed source was not retried on the next tick")
	}
	w.Tick()
	if d.calls.Load() != afterChange {
		t.Fatal("changed-then-refused source not held again")
	}
	if got := refusalLines(&logs); got != 2 {
		t.Fatalf("want one refusal line per change, got %d:\n%s", got, logs.String())
	}

	// Endpoint change: the new daemon gets one try.
	next := newPayloadDaemon(t, moved)
	w.BaseURL = next.URL
	w.Client = daemon.NewClient(next.URL)
	w.Tick()
	tried := next.calls.Load()
	if tried == 0 {
		t.Fatal("source not retried after the daemon endpoint changed")
	}
	w.Tick()
	if next.calls.Load() != tried || d.calls.Load() != afterChange {
		t.Fatal("source not held at the new endpoint")
	}
	if got := refusalLines(&logs); got != 3 {
		t.Fatalf("want one refusal line per change, got %d:\n%s", got, logs.String())
	}
}

// A refusal that comes from validating the daemon's response is never held. At
// the same endpoint, with the source unchanged, the watcher fetches it again
// on every tick and syncs as soon as the daemon's answer becomes valid
// (roborev 18304).
func TestWatchRetriesRefusedDaemonResponseAtSameEndpoint(t *testing.T) {
	valid := completePayload("current")
	// The typed daemon view accepts this string; the protobuf JSON parser does
	// not accept it as a Timestamp.
	malformed := bytes.Replace(valid, []byte(`"metadata":{}`), []byte(`"metadata":{"createdAt":"not a timestamp"}`), 1)
	for _, tc := range []struct {
		name, reason       string
		full, presentation []byte
	}{
		{"RPC enum outside the profile", "unsupported RPC enum name", unsupportedPayload(), unsupportedPayload()},
		{"malformed protobuf JSON", "unsupported RPC protobuf JSON", malformed, malformed},
		{"invalid presentation after full match", "unsupported RPC enum name", valid, unsupportedPayload()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, s, _ := completeFixture(t)
			d := newPayloadDaemon(t, tc.full)
			d.serve(tc.full, tc.presentation)
			var logs bytes.Buffer
			w := heldWatcher(t, root, d.URL, &logs)

			w.Tick()
			first := d.calls.Load()
			if first == 0 || !strings.Contains(logs.String(), "completeness unknown: "+tc.reason) {
				t.Fatalf("first tick did not refuse the response as %q (%d calls):\n%s", tc.reason, first, logs.String())
			}
			w.Tick()
			if second := d.calls.Load(); second <= first {
				t.Fatalf("refused response not fetched again at the same endpoint: %d then %d calls\n%s", first, second, logs.String())
			}
			if strings.Contains(logs.String(), "not retrying") {
				t.Fatalf("response refusal held:\n%s", logs.String())
			}

			d.serve(valid, valid)
			w.Tick()
			if result := session.Completeness(s); result.State != "verified" {
				t.Fatalf("valid response at the same endpoint not synced: %+v\n%s", result, logs.String())
			}
			if !strings.Contains(logs.String(), "tick: 1 synced, 0 skipped, 0 up-to-date, 0 failed") {
				t.Fatalf("sync not reported on the tick after the response became valid:\n%s", logs.String())
			}
		})
	}
}

// An mtime advance alone releases a hold, even when the content digest is the
// same.
func TestWatchRetriesHeldSourceWhenOnlyMtimeAdvances(t *testing.T) {
	root, s, _ := unsupportedProjectionFixture(t)
	d := newPayloadDaemon(t, unsupportedProjectionPayload("first"))
	var logs bytes.Buffer
	w := heldWatcher(t, root, d.URL, &logs)
	w.Tick()
	first := d.calls.Load()
	w.Tick()
	if d.calls.Load() != first {
		t.Fatal("unchanged source fetched again")
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(s.PBPath, later, later); err != nil {
		t.Fatal(err)
	}
	w.Tick()
	if d.calls.Load() == first {
		t.Fatal("source with an advanced mtime was not retried")
	}
}

// A held source's digest is read once to confirm the hold, then trusted while
// the source's files are unchanged, so an idle watch does not decode every held
// source on every tick. A metadata change that leaves the mtime alone, or a
// match older than the verification lifetime, reads the digest again.
func TestWatchRereadsHeldDigestOnlyAfterSourceFilesChangeOrMatchExpires(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("source file metadata is only compared on Linux")
	}
	for _, change := range []string{"none", "chmod", "expiry"} {
		t.Run(change, func(t *testing.T) {
			root, s, _ := unsupportedProjectionFixture(t)
			d := newPayloadDaemon(t, unsupportedProjectionPayload("first"))
			w := heldWatcher(t, root, d.URL, new(bytes.Buffer))
			w.Tick() // fetched, refused and held
			w.Tick() // digest read and matched
			held := w.Held[s.PBPath]
			if held.DigestMatchedAt.IsZero() {
				t.Fatalf("matched digest not remembered: %+v", held)
			}
			// Reading the digest now would release the hold, which refetches.
			held.SourceDigest = "not the source's digest"
			switch change {
			case "chmod":
				info, err := os.Stat(s.PBPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(s.PBPath, info.Mode().Perm()^0o040); err != nil {
					t.Fatal(err)
				}
			case "expiry":
				held.DigestMatchedAt = held.DigestMatchedAt.Add(-vcache.Lifetime(s.PBPath))
			}
			w.Held[s.PBPath] = held
			fetched := d.calls.Load()
			for range 3 {
				w.Tick()
			}
			if reread, want := d.calls.Load() != fetched, change != "none"; reread != want {
				t.Fatalf("digest read again = %v, want %v", reread, want)
			}
		})
	}
}

// A held source whose WAL has frames is read on every tick, never trusted by
// its metadata: a commit writes its WAL frames before readers can see them, so
// a read in that gap sees the previous content under the new metadata
// (agy-reader#q0mh).
func TestWatchRereadsHeldDigestWhileWALHasFrames(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("source file metadata is only compared on Linux")
	}
	root, s, db := unsupportedProjectionFixture(t)
	testfixture.Exec(t, db, "PRAGMA journal_mode=WAL")
	// Rewriting step 0 with the payload it already holds writes no WAL frame
	// when the bytes match, so the fixture adds and removes a row: the inserted
	// row's page is always new content, and the net content is unchanged.
	testfixture.Exec(t, db, "INSERT INTO battle_mode_infos(idx,data) VALUES(0,zeroblob(16))")
	testfixture.Exec(t, db, "DELETE FROM battle_mode_infos WHERE idx=0")
	if info, err := os.Stat(s.PBPath + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("fixture WAL has no frames: %v", err)
	}
	d := newPayloadDaemon(t, unsupportedProjectionPayload("first"))
	w := heldWatcher(t, root, d.URL, new(bytes.Buffer))
	w.Tick()
	first := d.calls.Load()
	for range 3 {
		w.Tick()
	}
	held, ok := w.Held[s.PBPath]
	if !ok || d.calls.Load() != first {
		t.Fatalf("unchanged source with WAL frames not held: held=%v, %d daemon calls after the first tick's %d", ok, d.calls.Load(), first)
	}
	if !held.DigestMatchedAt.IsZero() {
		t.Fatalf("match remembered while the WAL has frames: %+v", held)
	}
}

// A source refused before any RPC is held too, logged once, and released as
// soon as it becomes readable, which then syncs it.
func TestWatchReleasesUnreadableSourceOnceReadable(t *testing.T) {
	root, s, db := completeFixture(t)
	testfixture.Exec(t, db, "PRAGMA user_version=2")
	d := newPayloadDaemon(t, completePayload("current"))
	var logs bytes.Buffer
	w := heldWatcher(t, root, d.URL, &logs)
	for range 3 {
		w.Tick()
	}
	if d.calls.Load() != 0 {
		t.Fatalf("unreadable source reached the daemon: %d calls", d.calls.Load())
	}
	if got := refusalLines(&logs); got != 1 {
		t.Fatalf("unreadable source refusal logged %d times:\n%s", got, logs.String())
	}
	if held := w.Held[s.PBPath]; !held.DigestMatchedAt.IsZero() {
		t.Fatalf("hold on an unreadable source trusts its file metadata: %+v", held)
	}
	testfixture.Exec(t, db, "PRAGMA user_version=1")
	w.Tick()
	if d.calls.Load() == 0 {
		t.Fatal("readable source not retried")
	}
	if result := session.Completeness(s); result.State != "verified" {
		t.Fatalf("readable source not synced: %+v\n%s", result, logs.String())
	}
}

// Incomplete results are never held: a stale daemon response is fetched again
// every tick, and the source syncs as soon as a current daemon answers.
func TestWatchKeepsRetryingIncompleteResults(t *testing.T) {
	root, s, _ := completeFixture(t)
	stale := newPayloadDaemon(t, completePayload("stale"))
	var logs bytes.Buffer
	w := heldWatcher(t, root, stale.URL, &logs)
	w.Tick()
	first := stale.calls.Load()
	w.Tick()
	second := stale.calls.Load()
	if first == 0 || second <= first {
		t.Fatalf("incomplete result not retried: %d then %d calls", first, second)
	}
	if strings.Contains(logs.String(), "not retrying") {
		t.Fatalf("incomplete result held:\n%s", logs.String())
	}
	current := newPayloadDaemon(t, completePayload("current"))
	w.BaseURL = current.URL
	w.Client = daemon.NewClient(current.URL)
	w.Tick()
	if result := session.Completeness(s); result.State != "verified" {
		t.Fatalf("current daemon did not sync: %+v", result)
	}
}

// Sources that disappear from the root are forgotten.
func TestRefusalMemoPrunesMissingSources(t *testing.T) {
	root, s, _ := unsupportedProjectionFixture(t)
	d := newPayloadDaemon(t, unsupportedProjectionPayload("first"))
	w := heldWatcher(t, root, d.URL, new(bytes.Buffer))
	w.Tick()
	if _, ok := w.Held[s.PBPath]; !ok {
		t.Fatal("refusal not held")
	}
	for _, p := range []string{s.PBPath, s.PBPath + "-wal", s.PBPath + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	w.Tick()
	if len(w.Held) != 0 {
		t.Fatalf("missing source still held: %v", w.Held)
	}
}

// A forked conversation leaves <id>.pb from the fork beside the <id>.db that
// holds its history. The watcher must treat the id as one source, the .db:
// once its refusal is held, later ticks make no daemon calls and log no
// failed sidecar write for the .pb (agy-reader#3g5w). Before the fix the .pb
// was listed as a second, legacy source that was fetched on every tick and
// whose sidecar write was then refused against the .db.
func TestWatchTreatsForkedConversationAsOneDBSource(t *testing.T) {
	root, s, _ := unsupportedProjectionFixture(t)
	seedPB(t, root, "conversations", completeID, time.Now().Add(-time.Hour))
	d := newPayloadDaemon(t, unsupportedProjectionPayload("first"))
	var logs bytes.Buffer
	w := heldWatcher(t, root, d.URL, &logs)

	w.Tick()
	first := d.calls.Load()
	if first == 0 {
		t.Fatal("first tick did not fetch the fork's .db")
	}
	for range 3 {
		w.Tick()
	}
	if got := d.calls.Load(); got != first {
		t.Fatalf("fork fetched again after its refusal was held: %d daemon calls after the first tick's %d\n%s", got, first, logs.String())
	}
	if strings.Contains(logs.String(), "write sidecar") {
		t.Fatalf("watcher tried to write a sidecar for the fork's .pb:\n%s", logs.String())
	}
	if got := refusalLines(&logs); got != 1 {
		t.Fatalf("want one held refusal for the fork, got %d:\n%s", got, logs.String())
	}
	if !strings.Contains(logs.String(), "tick: 0 synced, 1 skipped, 0 up-to-date, 1 failed") ||
		strings.Count(logs.String(), "tick: 0 synced, 1 skipped, 0 up-to-date, 0 failed") != 3 {
		t.Fatalf("fork not counted as one held source per tick:\n%s", logs.String())
	}
	if len(w.Held) != 1 {
		t.Fatalf("want the .db alone held, got %v", w.Held)
	}
	if _, ok := w.Held[s.PBPath]; !ok {
		t.Fatalf("held source is not the .db %s: %v", s.PBPath, w.Held)
	}
}
