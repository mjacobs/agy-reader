package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/doctor"
	"github.com/mjacobs/agy-reader/internal/testfixture"
	"github.com/mjacobs/agy-reader/internal/watcher"
	"google.golang.org/protobuf/encoding/protowire"
)

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

func sourceOnlyStore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	now := time.Now()
	qualifySource(t, root, qualifyVerifiedID, false, now.Add(-2*time.Hour))
	sourceOnlySource(t, root, qualifySourceOnlyID, now.Add(-time.Hour))
	t.Setenv("ANTIGRAVITY_DAEMON_URL", qualifyDaemon(t, qualifyVerifiedID, qualifySourceOnlyID).URL)
	return root
}

func assertOutputStrings(t *testing.T, out string, wants, notWants []string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	for _, notWant := range notWants {
		if strings.Contains(out, notWant) {
			t.Errorf("output unexpectedly contained %q:\n%s", notWant, out)
		}
	}
}

func TestSourceOnlyAcceptanceInSweepAndManifestQualification(t *testing.T) {
	root := sourceOnlyStore(t)
	dest := filepath.Join(t.TempDir(), "sweep")
	var out bytes.Buffer
	if err := RunAuditSweepTo(t.Context(), []string{"--root", root, "--out", dest}, &out); err != nil {
		t.Fatalf("sweep failed: %v\n%s", err, out.String())
	}
	assertOutputStrings(t, out.String(), []string{
		"selected=2 fetched=2 rendered=2 failures=0",
		"reader sync verified=1/2 SQLite verified-with-source-only-fields=1 not-verified=0 legacy-unverifiable=0",
	}, []string{"NOT VERIFIED"})

	m := readSweepManifest(t, dest)
	if m.ReaderSync.VerifiedSourceOnly != 1 || m.ReaderSync.Verified != 1 {
		t.Fatalf("manifest reader sync summary: %+v", m.ReaderSync)
	}
	for _, s := range m.Sessions {
		switch s.FileID {
		case qualifySourceOnlyID:
			if s.ReaderSync != ReaderSyncVerified || s.ReaderSyncReason != completeness.SourceOnlyReason {
				t.Errorf("source-only sweep session: %+v", s)
			}
		case qualifyVerifiedID:
			if s.ReaderSync != ReaderSyncVerified || s.ReaderSyncReason != "" {
				t.Errorf("plain verified sweep session: %+v", s)
			}
		}
	}

	var q bytes.Buffer
	if err := RunAuditQualifyTo([]string{"--root", root, "--manifest", filepath.Join(dest, "manifest.json"), qualifyVerifiedID, qualifySourceOnlyID}, &q); err != nil {
		t.Fatalf("qualification failed: %v\n%s", err, q.String())
	}
	assertOutputStrings(t, q.String(), []string{
		"reader sync (whole sweep, not gating): verified=1/2 SQLite verified-with-source-only-fields=1 not-verified=0 legacy-unverifiable=0",
		"reader sync: " + qualifyVerifiedID + " VERIFIED (audit-sweep manifest)\n",
		"reader sync: " + qualifySourceOnlyID + " VERIFIED WITH SOURCE-ONLY FIELDS (audit-sweep manifest): " + completeness.SourceOnlyReason + "\n",
	}, []string{qualifySourceOnlyID + " VERIFIED (", qualifySourceOnlyID + " NOT VERIFIED"})
}

func TestSourceOnlyAcceptanceInWatchLiveQualificationAndDoctor(t *testing.T) {
	root := sourceOnlyStore(t)
	var logs bytes.Buffer
	failures := 0
	client := daemon.NewClient(os.Getenv("ANTIGRAVITY_DAEMON_URL"))
	synced, _, _, failed, _ := watcher.WatchTickPolicy(t.Context(), client, root, log.New(&logs, "", 0), &failures, true, nil)
	if synced != 2 || failed != 0 {
		t.Fatalf("watch sync failed: synced=%d failed=%d\n%s", synced, failed, logs.String())
	}
	for _, line := range strings.Split(logs.String(), "\n") {
		switch {
		case strings.HasPrefix(line, "synced "+qualifyVerifiedID+" "):
			if strings.Contains(line, ";") {
				t.Errorf("plain sync logged note: %s", line)
			}
		case strings.HasPrefix(line, "synced "+qualifySourceOnlyID+" "):
			if !strings.Contains(line, "; "+completeness.SourceOnlyReason) {
				t.Errorf("source-only sync missing note: %s", line)
			}
		}
	}
	if !strings.Contains(logs.String(), "synced "+qualifySourceOnlyID) || !strings.Contains(logs.String(), "synced "+qualifyVerifiedID) {
		t.Fatalf("watch did not log both syncs:\n%s", logs.String())
	}

	report := doctor.Report{DaemonURL: client.BaseURL, AgyVer: "1.0.13", RecordedVer: "1.0.13", WatchKnown: true, WatchRunning: true}
	doctor.PopulateSidecarCoverage(&report, root)
	if report.Total != 2 || report.Fresh != 2 || report.Verified != 1 || report.VerifiedSourceOnly != 1 {
		t.Fatalf("doctor report coverage: %+v", report)
	}
	var doc bytes.Buffer
	if code := doctor.WriteReport(&doc, report); code != 0 {
		t.Fatalf("doctor failed for healthy root: %d\n%s", code, doc.String())
	}
	assertOutputStrings(t, doc.String(), []string{
		"sidecars:    SQLite 1 verified, 1 verified with source-only fields, 0 incomplete, 0 unknown (supported daemon projection)",
	}, []string{"SQLite 2 verified"})

	var q bytes.Buffer
	if err := RunAuditQualifyTo([]string{"--root", root, qualifyVerifiedID, qualifySourceOnlyID}, &q); err != nil {
		t.Fatalf("live qualification failed: %v\n%s", err, q.String())
	}
	assertOutputStrings(t, q.String(), []string{
		"reader sync: " + qualifyVerifiedID + " VERIFIED (live sidecar attestation)\n",
		"reader sync: " + qualifySourceOnlyID + " VERIFIED WITH SOURCE-ONLY FIELDS (live sidecar attestation): " + completeness.SourceOnlyReason + "\n",
	}, []string{qualifySourceOnlyID + " VERIFIED (", qualifySourceOnlyID + " NOT VERIFIED"})
}

func TestDoctorLineWithoutSourceOnlyAcceptance(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	qualifySource(t, root, qualifyVerifiedID, false, now.Add(-time.Hour))
	t.Setenv("ANTIGRAVITY_DAEMON_URL", qualifyDaemon(t, qualifyVerifiedID).URL)
	failures := 0
	synced, _, _, failed, _ := watcher.WatchTickPolicy(t.Context(), daemon.NewClient(os.Getenv("ANTIGRAVITY_DAEMON_URL")), root, log.New(io.Discard, "", 0), &failures, true, nil)
	if synced != 1 || failed != 0 {
		t.Fatalf("watch sync failed: synced=%d failed=%d", synced, failed)
	}
	report := doctor.Report{DaemonURL: os.Getenv("ANTIGRAVITY_DAEMON_URL")}
	doctor.PopulateSidecarCoverage(&report, root)
	var doc bytes.Buffer
	if code := doctor.WriteReport(&doc, report); code != 0 {
		t.Fatalf("doctor failed: %d\n%s", code, doc.String())
	}
	assertOutputStrings(t, doc.String(), []string{"SQLite 1 verified,"}, []string{"source-only"})
}

func TestAuditQualifyRefusesManifestsBeforeSourceOnlyCounts(t *testing.T) {
	root := sourceOnlyStore(t)
	dest := filepath.Join(t.TempDir(), "sweep")
	if err := RunAuditSweepTo(t.Context(), []string{"--root", root, "--out", dest}, io.Discard); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dest, "manifest.json")
	var m map[string]any
	b, _ := os.ReadFile(manifest)
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	m["manifestVersion"] = 2
	b, _ = json.Marshal(m)
	path := filepath.Join(t.TempDir(), "manifest.json")
	writeFileT(t, path, b)
	var out bytes.Buffer
	err := RunAuditQualifyTo([]string{"--root", root, "--manifest", path, qualifyVerifiedID, qualifySourceOnlyID}, &out)
	if err == nil || !strings.Contains(err.Error(), "manifest version 2 does not separate source-only acceptances") {
		t.Fatalf("accepted version 2 manifest: %v\n%s", err, out.String())
	}
}
