package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/testfixture"
	"google.golang.org/protobuf/encoding/protowire"
)

// A synthetic conversation whose source stores the declared source-only
// CascadePlannerConfig field 50 (empty payload) on its generator record's
// planner configuration. The daemon's responses omit it, as the real daemon
// does, so the reader verifies it with the source-only reason.
const qualifySourceOnlyID = "88888888-8888-4888-8888-888888888888"

const sourceOnlyGeneratorRecord = `{"plannerConfig":{"maxOutputTokens":1024}}`

func sourceOnlyPayload(id string) []byte {
	return []byte(fmt.Sprintf(`{"cascadeId":%q,"metadata":{},"generatorMetadata":[%s],"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":"fixture %s"}}]}`, id, sourceOnlyGeneratorRecord, id[:8]))
}

func sourceOnlySource(t *testing.T, root, id string, mtime time.Time) {
	t.Helper()
	path := filepath.Join(root, "conversations", id+".db")
	db := testfixture.Source(t, path, sourceOnlyPayload(id))
	record := testfixture.Message(t, "exa.cortex_pb.CortexStepGeneratorMetadata", []byte(sourceOnlyGeneratorRecord)).ProtoReflect()
	planner := record.Mutable(record.Descriptor().Fields().ByName("planner_config")).Message()
	planner.SetUnknown(protowire.AppendBytes(protowire.AppendTag(nil, 50, protowire.BytesType), nil))
	testfixture.Exec(t, db, "UPDATE gen_metadata SET data=?1,size=length(?1) WHERE idx=0", testfixture.Binary(t, record))
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

// sourceOnlyStore holds a plain verified conversation next to the source-only
// one, served by a daemon that knows both.
func sourceOnlyStore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	now := time.Now()
	qualifySource(t, root, qualifyVerifiedID, false, now.Add(-2*time.Hour))
	sourceOnlySource(t, root, qualifySourceOnlyID, now.Add(-time.Hour))
	t.Setenv("ANTIGRAVITY_DAEMON_URL", qualifyDaemon(t, qualifyVerifiedID, qualifySourceOnlyID).URL)
	return root
}

func assertOutput(t *testing.T, what, out string, want, never []string) {
	t.Helper()
	for _, s := range want {
		if !strings.Contains(out, s) {
			t.Errorf("%s missing %q:\n%s", what, s, out)
		}
	}
	for _, s := range never {
		if strings.Contains(out, s) {
			t.Errorf("%s contains %q:\n%s", what, s, out)
		}
	}
}

// audit-sweep records the reason and counts the source apart from plain
// verified, and audit-qualify from that manifest labels it; it still qualifies.
func TestSourceOnlyAcceptanceInSweepAndManifestQualification(t *testing.T) {
	root := sourceOnlyStore(t)
	dest, out := runQualifySweep(t, root)
	m := readSweepManifest(t, dest)
	if want := (sweepReaderSummary{SQLite: 2, Verified: 1, VerifiedSourceOnly: 1}); m.ReaderSync != want {
		t.Fatalf("reader sync summary %+v, want %+v", m.ReaderSync, want)
	}
	for _, s := range m.Sessions {
		switch s.FileID {
		case qualifyVerifiedID:
			if s.ReaderSync != readerSyncVerified || s.ReaderSyncReason != "" {
				t.Errorf("plain verified entry: %+v", s)
			}
		case qualifySourceOnlyID:
			if s.ReaderSync != readerSyncVerified || s.ReaderSyncReason != completeness.SourceOnlyReason {
				t.Errorf("source-only entry recorded as plain verified: %+v", s)
			}
		}
	}
	assertOutput(t, "sweep output", out,
		[]string{"reader sync verified=1/2 SQLite verified-with-source-only-fields=1 not-verified=0 legacy-unverifiable=0"},
		[]string{"NOT VERIFIED"})

	var q bytes.Buffer
	if err := runAuditQualifyTo([]string{"--root", root, "--manifest", filepath.Join(dest, "manifest.json"), qualifyVerifiedID, qualifySourceOnlyID}, &q); err != nil {
		t.Fatalf("source-only acceptance no longer qualifies: %v\n%s", err, q.String())
	}
	assertOutput(t, "audit-qualify output", q.String(), []string{
		"reader sync (whole sweep, not gating): verified=1/2 SQLite verified-with-source-only-fields=1 not-verified=0 legacy-unverifiable=0",
		"reader sync: " + qualifyVerifiedID + " VERIFIED (audit-sweep manifest)\n",
		"reader sync: " + qualifySourceOnlyID + " VERIFIED WITH SOURCE-ONLY FIELDS (audit-sweep manifest): " + completeness.SourceOnlyReason + "\n",
	}, []string{qualifySourceOnlyID + " VERIFIED (", qualifySourceOnlyID + " NOT VERIFIED"})
}

// watch syncs the source with a note on its log line and then treats it as
// fresh; audit-qualify from the live sidecar labels it; doctor counts it on
// its own without calling the store unhealthy.
func TestSourceOnlyAcceptanceInWatchLiveQualificationAndDoctor(t *testing.T) {
	root := sourceOnlyStore(t)
	client := daemon.NewClient(os.Getenv("ANTIGRAVITY_DAEMON_URL"))
	var logs bytes.Buffer
	failures := 0
	synced, skipped, upToDate, failed, _ := watchTick(t.Context(), client, root, log.New(&logs, "", 0), &failures)
	if synced != 2 || skipped != 0 || upToDate != 0 || failed != 0 {
		t.Fatalf("first tick: synced=%d skipped=%d up-to-date=%d failed=%d\n%s", synced, skipped, upToDate, failed, logs.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		switch {
		case strings.HasPrefix(line, "synced "+qualifySourceOnlyID+" "):
			if !strings.HasSuffix(line, "; "+completeness.SourceOnlyReason) {
				t.Errorf("source-only sync logged as a plain write: %s", line)
			}
		case strings.HasPrefix(line, "synced "+qualifyVerifiedID+" "):
			if strings.Contains(line, completeness.SourceOnlyReason) {
				t.Errorf("plain sync carries the source-only note: %s", line)
			}
		}
	}
	if !strings.Contains(logs.String(), "synced "+qualifySourceOnlyID) || !strings.Contains(logs.String(), "synced "+qualifyVerifiedID) {
		t.Fatalf("missing synced lines:\n%s", logs.String())
	}

	// Freshness is unchanged: the source-only sidecar is up to date.
	logs.Reset()
	if synced, _, upToDate, failed, _ = watchTick(t.Context(), client, root, log.New(&logs, "", 0), &failures); synced != 0 || upToDate != 2 || failed != 0 {
		t.Fatalf("second tick re-synced a verified source: synced=%d up-to-date=%d failed=%d\n%s", synced, upToDate, failed, logs.String())
	}
	sessions, err := discovery.ListConversationSessions(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sessions {
		if stale, reason := isStale(s); stale {
			t.Errorf("%s stale (%s)", s.CascadeID, reason)
		}
	}

	var q bytes.Buffer
	if err := runAuditQualifyTo([]string{"--root", root, qualifyVerifiedID, qualifySourceOnlyID}, &q); err != nil {
		t.Fatalf("source-only acceptance no longer qualifies: %v\n%s", err, q.String())
	}
	assertOutput(t, "audit-qualify output", q.String(), []string{
		"reader sync: " + qualifyVerifiedID + " VERIFIED (live sidecar attestation)\n",
		"reader sync: " + qualifySourceOnlyID + " VERIFIED WITH SOURCE-ONLY FIELDS (live sidecar attestation): " + completeness.SourceOnlyReason + "\n",
	}, []string{qualifySourceOnlyID + " VERIFIED (", qualifySourceOnlyID + " NOT VERIFIED"})

	r := doctorReport{daemonURL: "http://127.0.0.1:51847", agyVer: "1.0.13", recordedVer: "1.0.13", watchKnown: true, watchRunning: true}
	populateSidecarCoverage(&r, root)
	if r.total != 2 || r.sqlite != 2 || r.verified != 1 || r.verifiedSourceOnly != 1 || r.fresh != 2 || r.stale != 0 || r.incomplete != 0 || r.unknown != 0 {
		t.Fatalf("doctor coverage: %+v", r)
	}
	var out bytes.Buffer
	if problems := writeDoctorReport(&out, r); problems != 0 {
		t.Fatalf("source-only acceptance made doctor unhealthy (%d):\n%s", problems, out.String())
	}
	assertOutput(t, "doctor output", out.String(),
		[]string{"sidecars:    SQLite 1 verified, 1 verified with source-only fields, 0 incomplete, 0 unknown (supported daemon projection)"},
		[]string{"SQLite 2 verified"})
}

// Without a source-only acceptance the doctor line is unchanged.
func TestDoctorLineWithoutSourceOnlyAcceptance(t *testing.T) {
	var out bytes.Buffer
	writeDoctorReport(&out, doctorReport{daemonURL: "http://127.0.0.1:51847", agyVer: "1.0.13", recordedVer: "1.0.13", total: 2, fresh: 2, sqlite: 2, verified: 2})
	assertOutput(t, "doctor output", out.String(),
		[]string{"sidecars:    SQLite 2 verified, 0 incomplete, 0 unknown (supported daemon projection)"},
		[]string{"source-only"})
}

// A version 2 manifest, as written before source-only acceptances were counted
// apart, records such a conversation as plain verified with no reason and
// counts it in verified. audit-qualify must refuse it and ask for a new sweep
// rather than print it as plain VERIFIED.
func TestAuditQualifyRefusesManifestsBeforeSourceOnlyCounts(t *testing.T) {
	root := sourceOnlyStore(t)
	dest, _ := runQualifySweep(t, root)
	current := filepath.Join(dest, "manifest.json")
	qualify := func(path string) (string, error) {
		var out bytes.Buffer
		err := runAuditQualifyTo([]string{"--root", root, "--manifest", path, qualifyVerifiedID, qualifySourceOnlyID}, &out)
		return out.String(), err
	}
	if out, err := qualify(current); err != nil {
		t.Fatalf("current manifest refused: %v\n%s", err, out)
	}

	m := readSweepManifest(t, dest)
	if m.Version != 3 {
		t.Fatalf("manifest version %d, want 3", m.Version)
	}
	// Rewrite it as the previous writer would have.
	old := m
	old.Version = 2
	old.ReaderSync = sweepReaderSummary{SQLite: 2, Verified: 2}
	old.Sessions = append([]sweepSession{}, m.Sessions...)
	for i := range old.Sessions {
		old.Sessions[i].ReaderSyncReason = ""
	}
	b, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "manifest.json")
	writeFileT(t, path, b)
	out, err := qualify(path)
	if err == nil || !strings.Contains(err.Error(), "manifest version 2 does not separate source-only acceptances from verified; rerun audit-sweep") {
		t.Fatalf("version 2 manifest accepted or refused for the wrong reason: %v\n%s", err, out)
	}
	if strings.Contains(out, "VERIFIED") {
		t.Fatalf("version 2 manifest qualified a conversation:\n%s", out)
	}

	old.Version = 1
	if b, err = json.Marshal(old); err != nil {
		t.Fatal(err)
	}
	writeFileT(t, path, b)
	if _, err := qualify(path); err == nil || !strings.Contains(err.Error(), "manifest version 1 predates reader-sync qualification; rerun audit-sweep") {
		t.Fatalf("version 1 manifest: %v", err)
	}
}
