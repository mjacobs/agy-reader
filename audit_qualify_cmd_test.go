package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/testfixture"
)

// Synthetic conversations for the qualification fixtures. The "unsupported"
// sources carry a persisted executor-metadata field outside the pinned
// descriptors, as agy 1.2.15 did, while the daemon still serves an ordinary
// trajectory that fetches and renders cleanly.
const (
	qualifyVerifiedID    = "33333333-3333-4333-8333-333333333333"
	qualifyUnsupportedID = "44444444-4444-4444-8444-444444444444"
	qualifyLegacyID      = "66666666-6666-4666-8666-666666666666"
)

// unknownPersistedField is field 1000, wire type 2, zero length.
var unknownPersistedField = []byte{0xc2, 0x3e, 0x00}

func qualifyPayload(id string) []byte {
	return []byte(fmt.Sprintf(`{"cascadeId":%q,"metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":"fixture %s"}}]}`, id, id[:8]))
}

// qualifySource writes a SQLite source for id. An unsupported source fails
// native decoding exactly where the reader's completeness profile ends.
func qualifySource(t *testing.T, root, id string, unsupported bool, mtime time.Time) {
	t.Helper()
	path := filepath.Join(root, "conversations", id+".db")
	db := testfixture.Source(t, path, qualifyPayload(id))
	if unsupported {
		testfixture.Exec(t, db, "INSERT INTO executor_metadata VALUES(0,?)", unknownPersistedField)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

// qualifyDaemon serves every known conversation for load, ordinary and full
// RPC alike, so each one fetches and renders whether or not it can sync. The
// source-only fixture is served its own payload, without the declared field.
func qualifyDaemon(t *testing.T, ids ...string) *httptest.Server {
	t.Helper()
	known := map[string]bool{}
	for _, id := range ids {
		known[id] = true
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CascadeID string `json:"cascadeId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !known[req.CascadeID] {
			http.Error(w, "unknown conversation", http.StatusNotFound)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/LoadTrajectory") {
			_, _ = io.WriteString(w, `{}`)
			return
		}
		payload := qualifyPayload(req.CascadeID)
		if req.CascadeID == qualifySourceOnlyID {
			payload = sourceOnlyPayload(req.CascadeID)
		}
		fmt.Fprintf(w, `{"trajectory":%s}`, payload)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// qualifyStore is a store whose history holds an old conversation outside the
// reader's profile next to a newer conversation the reader verifies.
func qualifyStore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	now := time.Now()
	qualifySource(t, root, qualifyUnsupportedID, true, now.Add(-48*time.Hour))
	qualifySource(t, root, qualifyVerifiedID, false, now.Add(-time.Hour))
	seedPB(t, root, "conversations", qualifyLegacyID, now.Add(-72*time.Hour))
	srv := qualifyDaemon(t, qualifyVerifiedID, qualifyUnsupportedID, qualifyLegacyID)
	t.Setenv("ANTIGRAVITY_DAEMON_URL", srv.URL)
	return root
}

func runQualifySweep(t *testing.T, root string) (string, string) {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "sweep")
	var out bytes.Buffer
	if err := runAuditSweepTo(t.Context(), []string{"--root", root, "--out", dest}, &out); err != nil {
		t.Fatalf("broad sweep stopped on an unsupported historical session: %v\n%s", err, out.String())
	}
	return dest, out.String()
}

func TestAuditSweepSeparatesReaderSyncFromFetchRender(t *testing.T) {
	root := qualifyStore(t)
	dest, out := runQualifySweep(t, root)

	m := readSweepManifest(t, dest)
	if !m.Complete || m.Version != sweepManifestVersion || m.Selected != 3 || len(m.Sessions) != 3 {
		t.Fatalf("broad inspection incomplete: %+v", m)
	}
	files, err := os.ReadDir(filepath.Join(dest, "corpus"))
	if err != nil || len(files) != 3 {
		t.Fatalf("every conversation should be fetched and rendered into the corpus: %v %d", err, len(files))
	}
	if want := (sweepReaderSummary{SQLite: 2, Verified: 1, NotVerified: 1, Legacy: 1}); m.ReaderSync != want {
		t.Fatalf("reader sync summary %+v, want %+v", m.ReaderSync, want)
	}
	byID := map[string]sweepSession{}
	for _, s := range m.Sessions {
		byID[s.FileID] = s
	}
	if s := byID[qualifyVerifiedID]; s.Source != sourceSQLite || s.ReaderSync != readerSyncVerified || s.Steps != 1 {
		t.Errorf("supported conversation: %+v", s)
	}
	// A verified entry names the source it covered; audit-qualify rechecks it.
	snapshot, err := completeness.ReadSnapshot(t.Context(), filepath.Join(root, "conversations", qualifyVerifiedID+".db"))
	if err != nil {
		t.Fatal(err)
	}
	if s := byID[qualifyVerifiedID]; s.SourceDigest != snapshot.Digest || s.Profile != completeness.Profile {
		t.Errorf("verified conversation does not record its source digest and profile: %+v", s)
	}
	if s := byID[qualifyUnsupportedID]; s.SourceDigest != "" || s.Profile != "" {
		t.Errorf("unverified conversation records verification evidence: %+v", s)
	}
	if s := byID[qualifyUnsupportedID]; s.Source != sourceSQLite || s.ReaderSync != "unknown" ||
		!strings.Contains(s.ReaderSyncReason, "unknown persisted protobuf fields") || s.Steps != 1 {
		t.Errorf("rendered-but-unsyncable conversation not marked: %+v", s)
	}
	if s := byID[qualifyLegacyID]; s.Source != sourceLegacyProtobuf || s.ReaderSync != readerSyncLegacy {
		t.Errorf("legacy conversation: %+v", s)
	}
	for _, want := range []string{
		"selected=3 fetched=3 rendered=3 failures=0",
		"reader sync verified=1/2 SQLite not-verified=1 legacy-unverifiable=1",
		"NOT VERIFIED " + qualifyUnsupportedID + ": unknown: unknown persisted protobuf fields",
		"fetched and rendered is not synced",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("sweep output missing %q:\n%s", want, out)
		}
	}
	// Qualification uses ordinary sync's verification but never its write.
	sidecars, _ := filepath.Glob(filepath.Join(root, "conversations", "*.trajectory.json"))
	if len(sidecars) != 0 {
		t.Fatalf("sweep wrote live sidecars: %v", sidecars)
	}
}

func TestAuditQualifyFromSweepManifest(t *testing.T) {
	root := qualifyStore(t)
	dest, _ := runQualifySweep(t, root)
	manifest := filepath.Join(dest, "manifest.json")

	var out bytes.Buffer
	if err := runAuditQualifyTo([]string{"--root", root, "--manifest", manifest, qualifyVerifiedID}, &out); err != nil {
		t.Fatalf("verified conversation refused: %v\n%s", err, out.String())
	}
	for _, want := range []string{
		"not gating): verified=1/2 SQLite not-verified=1 legacy-unverifiable=1",
		"reader sync: " + qualifyVerifiedID + " VERIFIED (audit-sweep manifest)",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}

	refusals := map[string][]string{
		"rendered but unsyncable": {qualifyVerifiedID, qualifyUnsupportedID},
		"legacy":                  {qualifyLegacyID},
		"absent from sweep":       {"77777777-7777-4777-8777-777777777777"},
	}
	for name, ids := range refusals {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			err := runAuditQualifyTo(append([]string{"--root", root, "--manifest", manifest}, ids...), &out)
			if err == nil || !strings.Contains(err.Error(), "fetch/render success does not qualify") {
				t.Fatalf("accepted unqualified conversation: %v\n%s", err, out.String())
			}
			if !strings.Contains(out.String(), ids[len(ids)-1]+" NOT VERIFIED") {
				t.Fatalf("refusal not explained:\n%s", out.String())
			}
		})
	}

	// A manifest is evidence only for its own complete run over this root.
	var m map[string]any
	b, _ := os.ReadFile(manifest)
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		mutate func(map[string]any)
		want   string
	}{
		"fetch-render-only version 1": {func(m map[string]any) { m["manifestVersion"] = 1 }, "manifest version 1 predates reader-sync qualification; rerun audit-sweep"},
		"pre-source-only version 2":   {func(m map[string]any) { m["manifestVersion"] = 2 }, "manifest version 2 does not separate source-only acceptances"},
		"pre-source-digest version 3": {func(m map[string]any) { m["manifestVersion"] = 3 }, "manifest version 3 does not record the source digest each verification covered, so a source changed since the sweep cannot be detected; rerun audit-sweep"},
		"incomplete":                  {func(m map[string]any) { m["complete"] = false }, "manifest is incomplete"},
		"other root":                  {func(m map[string]any) { m["root"] = t.TempDir() }, "is not the audited root"},
	} {
		t.Run(name, func(t *testing.T) {
			copied := map[string]any{}
			for k, v := range m {
				copied[k] = v
			}
			c.mutate(copied)
			b, _ := json.Marshal(copied)
			path := filepath.Join(t.TempDir(), "manifest.json")
			writeFileT(t, path, b)
			err := runAuditQualifyTo([]string{"--root", root, "--manifest", path, qualifyVerifiedID}, io.Discard)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("unusable manifest accepted or refused for the wrong reason: %v", err)
			}
		})
	}
}

// A sweep's verification is evidence only for the source it read. A
// conversation that changes after the sweep, or gains a persisted field the
// profile does not support so that ordinary sync now refuses it, must not
// qualify from the old manifest.
func TestAuditQualifyFromSweepManifestRechecksCurrentSource(t *testing.T) {
	root := qualifyStore(t)
	dest, _ := runQualifySweep(t, root)
	manifest := filepath.Join(dest, "manifest.json")
	qualify := func() (string, error) {
		var out bytes.Buffer
		err := runAuditQualifyTo([]string{"--root", root, "--manifest", manifest, qualifyVerifiedID}, &out)
		return out.String(), err
	}
	if out, err := qualify(); err != nil {
		t.Fatalf("unchanged source refused: %v\n%s", err, out)
	}
	refused := func(what, want string) {
		t.Helper()
		out, err := qualify()
		if err == nil || !strings.Contains(out, qualifyVerifiedID+" NOT VERIFIED (audit-sweep manifest): unknown: "+want) {
			t.Fatalf("%s qualified from the old manifest: %v\n%s", what, err, out)
		}
	}

	// New content the profile still decodes: only the digest shows the change.
	path := filepath.Join(root, "conversations", qualifyVerifiedID+".db")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	changed := []byte(fmt.Sprintf(`{"cascadeId":%q,"metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":"written after the sweep"}}]}`, qualifyVerifiedID))
	db := testfixture.Source(t, path, changed)
	refused("changed source", "source changed since the sweep; rerun audit-sweep")

	// A persisted field outside the profile: the source no longer decodes.
	testfixture.Exec(t, db, "INSERT INTO executor_metadata VALUES(0,?)", unknownPersistedField)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	refused("undecodable source", "source unreadable since the sweep (unknown persisted protobuf fields); rerun audit-sweep")

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	refused("missing source", "SQLite source no longer present in the root; rerun audit-sweep")
}

func TestAuditQualifyFromLiveSidecars(t *testing.T) {
	root := qualifyStore(t)
	qualify := func(ids ...string) (string, error) {
		var out bytes.Buffer
		err := runAuditQualifyTo(append([]string{"--root", root}, ids...), &out)
		return out.String(), err
	}
	// The 1.2.15 audit's gap: no live sidecar was accepted because an explicit
	// corpus stood in for it. Without one, nothing is qualified.
	if out, err := qualify(qualifyVerifiedID); err == nil || !strings.Contains(out, "no sidecar") {
		t.Fatalf("missing sidecar accepted: %v\n%s", err, out)
	}

	sessions, err := discovery.ListConversationSessions(root)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]discovery.Session{}
	for _, s := range sessions {
		byID[s.CascadeID] = s
	}
	client := daemon.NewClient(os.Getenv("ANTIGRAVITY_DAEMON_URL"))
	s := byID[qualifyVerifiedID]
	traj, err := fetchSession(t.Context(), client, s)
	if err != nil {
		t.Fatal(err)
	}
	if err = cache.Write(s.SidecarPath, traj); err != nil {
		t.Fatal(err)
	}
	if out, err := qualify(qualifyVerifiedID); err != nil || !strings.Contains(out, "VERIFIED (live sidecar attestation)") {
		t.Fatalf("ordinarily synced conversation refused: %v\n%s", err, out)
	}

	// A sidecar that only proves fetching (no attestation) does not qualify,
	// and neither does a source the current profile cannot decode.
	unsupported := byID[qualifyUnsupportedID]
	writeFileT(t, unsupported.SidecarPath, qualifyPayload(qualifyUnsupportedID))
	if out, err := qualify(qualifyVerifiedID, qualifyUnsupportedID); err == nil ||
		!strings.Contains(out, qualifyUnsupportedID+" NOT VERIFIED (live sidecar attestation): unknown: unknown persisted protobuf fields") {
		t.Fatalf("unsupported source accepted: %v\n%s", err, out)
	}
	if _, err := qualify("not-a-conversation"); err == nil {
		t.Fatal("accepted invalid conversation ID")
	}
}
