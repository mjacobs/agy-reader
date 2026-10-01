//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/testfixture"
)

const fakeShapeFingerprint = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type auditFixture struct {
	repo       string
	root       string
	script     string
	sidecar    string
	compatFile string
	binDir     string
}

func newAuditFixture(t *testing.T) *auditFixture {
	t.Helper()
	repo := t.TempDir()
	scriptDir := filepath.Join(repo, "skills", "agy-format-audit", "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptData, err := os.ReadFile(filepath.Join("skills", "agy-format-audit", "scripts", "audit_format.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(scriptDir, "audit_format.sh")
	if err := os.WriteFile(script, scriptData, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module auditfixture\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	compatFile := filepath.Join(repo, "COMPATIBILITY.md")
	if err := os.WriteFile(compatFile, []byte("sentinel compatibility record\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCommand(t, repo, "git", "init", "-q")

	root := filepath.Join(repo, "agy-root")
	conversations := filepath.Join(root, "conversations")
	if err := os.MkdirAll(conversations, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cli.log"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(conversations, "11111111-1111-1111-1111-111111111111.db")
	if err := os.WriteFile(dbPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	binDir := filepath.Join(repo, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, filepath.Join(binDir, "agy"), `#!/usr/bin/env bash
case "${1:-}" in
--version) echo 9.9.9 ;;
changelog) printf '9.9.9:\nfixture release\n' ;;
*) exit 1 ;;
esac
`)
	writeExecutable(t, filepath.Join(binDir, "sqlite3"), `#!/usr/bin/env bash
query="${2:-}"
case "$query" in
"PRAGMA user_version;") echo 1 ;;
"SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;")
    printf '%s\n' battle_mode_infos executor_metadata gen_metadata parent_references steps trajectory_meta trajectory_metadata_blob
    ;;
"SELECT name FROM sqlite_master WHERE type='index' AND sql IS NOT NULL ORDER BY name;")
    printf '%s\n' idx_steps_status idx_steps_step_type
    ;;
"SELECT sql FROM sqlite_master WHERE sql IS NOT NULL ORDER BY type, name;")
    printf '%s\n' 'CREATE TABLE steps (status TEXT, step_type TEXT)' 'CREATE INDEX idx_steps_status ON steps(status)' 'CREATE INDEX idx_steps_step_type ON steps(step_type)'
    ;;
*) exit 1 ;;
esac
`)
	writeExecutable(t, filepath.Join(binDir, "sleep"), `#!/usr/bin/env bash
set -eu
if [ -n "${AUDIT_TEST_SLEEP_ENTERED:-}" ]; then
    : > "$AUDIT_TEST_SLEEP_ENTERED"
    while [ ! -e "$AUDIT_TEST_SLEEP_RELEASE" ]; do
        /bin/sleep 0.01
    done
    exit 0
fi
exec /bin/sleep "$@"
`)
	writeExecutable(t, filepath.Join(binDir, "go"), `#!/usr/bin/env bash
set -eu
case "${1:-}" in
build)
    shift
    output=""
    while [ "$#" -gt 0 ]; do
        if [ "$1" = "-o" ]; then
            output="$2"
            break
        fi
        shift
    done
    test -n "$output"
    if [ -n "${AUDIT_TEST_READER_BIN:-}" ]; then
        # Run the real agy-reader commands through the test binary.
        printf '%s\n' '#!/usr/bin/env bash' 'AGY_READER_TEST_AS_CLI=1 exec "$AUDIT_TEST_READER_BIN" "$@"' > "$output"
    else
        cat > "$output" <<'FAKE'
#!/usr/bin/env bash
case "${1:-}" in
audit-qualify)
    echo "reader sync: fixture ${AUDIT_TEST_QUALIFY_RC:-0}"
    exit "${AUDIT_TEST_QUALIFY_RC:-0}"
    ;;
*) echo `+fakeShapeFingerprint+` ;;
esac
FAKE
    fi
    chmod +x "$output"
    ;;
test)
    if [ -n "${AUDIT_TEST_GO_ENV_LOG:-}" ]; then
        printf 'url=%s token=%s qualify=%s corpus=%s wait=%s\n' "${ANTIGRAVITY_DAEMON_URL-unset}" "${ANTIGRAVITY_CSRF_TOKEN-unset}" \
            "${AGY_QUALIFY_SESSIONS-unset}" "${AGY_SIDECAR_CORPUS-unset}" "${AGY_SIDECAR_WAIT_SECONDS-unset}" > "$AUDIT_TEST_GO_ENV_LOG"
    fi
    if [ -n "${AUDIT_TEST_GO_FAIL:-}" ]; then
        echo "--- FAIL: TestFixtureDiagnostic"
        echo "    fixture failure detail"
        exit 1
    fi
    ;;
*) exit 1 ;;
esac
`)

	return &auditFixture{
		repo:       repo,
		root:       root,
		script:     script,
		sidecar:    filepath.Join(conversations, "11111111-1111-1111-1111-111111111111.trajectory.json"),
		compatFile: compatFile,
		binDir:     binDir,
	}
}

func TestAuditWaitsForNewestSidecar(t *testing.T) {
	fixture := newAuditFixture(t)
	sleepEntered := filepath.Join(fixture.repo, "sleep-entered")
	sleepRelease := filepath.Join(fixture.repo, "sleep-release")
	written := make(chan error, 1)
	go func() {
		var result error
		defer func() {
			if releaseErr := os.WriteFile(sleepRelease, nil, 0o600); result == nil {
				result = releaseErr
			}
			written <- result
		}()
		if result = waitForPath(sleepEntered, 5*time.Second); result != nil {
			return
		}
		result = os.WriteFile(fixture.sidecar, []byte("{\"steps\":[]}\n"), 0o600)
	}()

	output, err := fixture.runWithOverrides("3", map[string]string{
		"AUDIT_TEST_SLEEP_ENTERED": sleepEntered,
		"AUDIT_TEST_SLEEP_RELEASE": sleepRelease,
	})
	if writeErr := <-written; writeErr != nil {
		t.Fatal(writeErr)
	}
	if err != nil {
		t.Fatalf("audit failed: %v\n%s", err, output)
	}
	for _, want := range []string{
		"waiting up to 3s for agy-reader's watcher",
		"Sidecar appeared after",
		"Shape fingerprint: " + fakeShapeFingerprint,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
}

func waitForPath(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAuditReportsTrueStepsCount(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not installed; audit_format.sh falls back to step count unavailable")
	}
	fixture := newAuditFixture(t)
	// Nested "type" keys (in step details and in unrelated metadata) must not
	// inflate the reported count: the steps array here has 3 entries but
	// contains 7 "type" keys in total.
	sidecar := `{
		"steps": [
			{"type": "a", "detail": {"type": "nested1"}},
			{"type": "b", "detail": {"type": "nested2"}},
			{"type": "c"}
		],
		"metadata": {"type": "session", "extra": {"type": "meta2"}}
	}`
	if err := os.WriteFile(fixture.sidecar, []byte(sidecar), 0o600); err != nil {
		t.Fatal(err)
	}

	output, err := fixture.run("0")
	if err != nil {
		t.Fatalf("audit failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "(3 steps detected)") {
		t.Fatalf("output does not report the true steps array length (3):\n%s", output)
	}
	if strings.Contains(output, "(7 steps detected)") {
		t.Fatalf("output counted every \"type\" key instead of steps array entries:\n%s", output)
	}
}

func TestAuditRecordFailsClosedWithoutSidecarShape(t *testing.T) {
	fixture := newAuditFixture(t)
	output, err := fixture.run("0", "--record")
	if err == nil {
		t.Fatalf("audit unexpectedly recorded without a sidecar shape:\n%s", output)
	}
	if !strings.Contains(output, "Audit did not fully pass; no compatibility record produced.") {
		t.Fatalf("output missing fail-closed diagnostic:\n%s", output)
	}
	content, readErr := os.ReadFile(fixture.compatFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got, want := string(content), "sentinel compatibility record\n"; got != want {
		t.Fatalf("compatibility record changed: got %q want %q", got, want)
	}
}

func TestAuditExplicitCorpusDoesNotWaitForNewestSidecar(t *testing.T) {
	fixture := newAuditFixture(t)
	corpus := filepath.Join(fixture.repo, "swept-corpus")
	if err := os.MkdirAll(corpus, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corpus, "fixture.trajectory.json"), []byte("{\"steps\":[]}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	output, err := fixture.runWithOverrides("3", map[string]string{
		"AGY_SIDECAR_CORPUS": corpus,
	}, "--corpus-swept")
	if err != nil {
		t.Fatalf("audit failed: %v\n%s", err, output)
	}
	if strings.Contains(output, "waiting up to") {
		t.Fatalf("explicit corpus audit waited on the live newest session:\n%s", output)
	}
	for _, want := range []string{
		"configured corpus will be audited instead",
		"Shape fingerprint: " + fakeShapeFingerprint,
		"scope: explicit-version-scoped",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
}

func (f *auditFixture) run(waitSeconds string, args ...string) (string, error) {
	return f.runWithOverrides(waitSeconds, nil, args...)
}

func (f *auditFixture) runWithOverrides(waitSeconds string, overrides map[string]string, args ...string) (string, error) {
	cmd := exec.Command("bash", append([]string{f.script}, args...)...)
	cmd.Dir = f.repo
	env := map[string]string{
		"AGY_SIDECAR_WAIT_SECONDS": waitSeconds,
		"AGY_SIDECAR_CORPUS":       "", // Each fixture owns its corpus, even during a live audit.
		"ANTIGRAVITY_CLI_ROOT":     f.root,
		"HOME":                     filepath.Join(f.repo, "home"),
		"PATH":                     f.binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TMPDIR":                   f.repo,
	}
	for key, value := range overrides {
		env[key] = value
	}
	cmd.Env = overriddenEnv(env)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// overriddenEnv is the caller's environment minus every audit input the helper
// or these fixtures read, plus overrides. The documented audit runs this suite
// with AGY_QUALIFY_SESSIONS naming live conversations; inheriting it (or any
// other AGY_*, AUDIT_TEST_* or AGENTSVIEW_DIR value) would audit the fixture
// store against the caller's settings.
func overriddenEnv(overrides map[string]string) []string {
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if strings.HasPrefix(key, "AGY_") || strings.HasPrefix(key, "AUDIT_TEST_") || key == "AGENTSVIEW_DIR" {
			continue
		}
		if _, replaced := overrides[key]; !replaced {
			env = append(env, item)
		}
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return env
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func runCommand(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

func TestAuditDoesNotRequireGNUFindOrSha256sum(t *testing.T) {
	fixture := newAuditFixture(t)
	writeFileT(t, fixture.sidecar, []byte(`{"steps":[]}`))
	for _, name := range []string{"find", "sha256sum"} {
		writeExecutable(t, filepath.Join(fixture.binDir, name), "#!/bin/sh\necho unexpected GNU utility >&2\nexit 99\n")
	}
	output, err := fixture.run("0", "--record")
	if err != nil {
		t.Fatalf("portable audit failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Recorded to:") {
		t.Fatalf("record missing: %s", output)
	}
}

// The suites see neither daemon pinning nor this audit's own inputs, such as
// the live scenario IDs the documented workflow passes in AGY_QUALIFY_SESSIONS.
func TestAuditIsolatesGoTestsFromDaemonOverrides(t *testing.T) {
	fixture := newAuditFixture(t)
	writeFileT(t, fixture.sidecar, []byte(`{"steps":[]}`))
	envLog := filepath.Join(fixture.repo, "go-test-env")
	output, err := fixture.runWithOverrides("0", map[string]string{
		"ANTIGRAVITY_DAEMON_URL": "http://127.0.0.1:9",
		"ANTIGRAVITY_CSRF_TOKEN": "dummy-token",
		"AGY_QUALIFY_SESSIONS":   "77777777-7777-4777-8777-777777777777",
		"AUDIT_TEST_GO_ENV_LOG":  envLog,
	})
	if err != nil {
		t.Fatalf("audit failed: %v\n%s", err, output)
	}
	got, err := os.ReadFile(envLog)
	if err != nil {
		t.Fatalf("go test never ran: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(got)) != "url=unset token=unset qualify=unset corpus=unset wait=unset" {
		t.Errorf("go test inherited overrides: %q", got)
	}
}

func TestAuditReportsGoTestFailureDetail(t *testing.T) {
	fixture := newAuditFixture(t)
	writeFileT(t, fixture.sidecar, []byte(`{"steps":[]}`))
	output, err := fixture.runWithOverrides("0", map[string]string{"AUDIT_TEST_GO_FAIL": "1"})
	if err == nil {
		t.Fatalf("audit passed despite failing tests:\n%s", output)
	}
	for _, want := range []string{
		"agy-reader unit tests: FAIL (exit 1)",
		"--- FAIL: TestFixtureDiagnostic",
		"fixture failure detail",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
}

// sweptNewest selects the newest conversation in auditSweptStore.
type sweptNewest int

const (
	newestVerified sweptNewest = iota
	newestUnsupported
	// newestUnsupportedInWAL: the unsupported conversation's main database is
	// older than the verified one, but its nonempty, uncheckpointed -wal is the
	// newest write in the store.
	newestUnsupportedInWAL
)

const sweptUnsupportedNewestID = "55555555-5555-4555-8555-555555555555"

// auditSweptStore runs the real audit-sweep over a store whose newest
// conversation is chosen by newest, returning the store and its swept corpus.
func auditSweptStore(t *testing.T, fixture *auditFixture, newest sweptNewest) (string, string) {
	t.Helper()
	root := filepath.Join(fixture.repo, "swept-root")
	now := time.Now()
	// History always holds a conversation the reader cannot verify; the newest
	// conversation is the one the audit is qualifying.
	qualifySource(t, root, qualifyUnsupportedID, true, now.Add(-48*time.Hour))
	newestID := qualifyVerifiedID
	switch newest {
	case newestVerified:
		qualifySource(t, root, qualifyVerifiedID, false, now.Add(-time.Minute))
	case newestUnsupported:
		newestID = sweptUnsupportedNewestID
		qualifySource(t, root, qualifyVerifiedID, false, now.Add(-24*time.Hour))
		qualifySource(t, root, newestID, true, now.Add(-time.Minute))
	case newestUnsupportedInWAL:
		newestID = sweptUnsupportedNewestID
		qualifySource(t, root, qualifyVerifiedID, false, now.Add(-2*time.Hour))
		qualifyWALSource(t, root, newestID, now.Add(-3*time.Hour), now.Add(-time.Hour))
	}
	writeFileT(t, filepath.Join(root, "cli.log"), nil)
	srv := qualifyDaemon(t, qualifyUnsupportedID, qualifyVerifiedID, newestID)
	t.Setenv("ANTIGRAVITY_DAEMON_URL", srv.URL)
	dest := filepath.Join(fixture.repo, "sweep")
	var out strings.Builder
	if err := runAuditSweepTo(t.Context(), []string{"--root", root, "--out", dest}, &out); err != nil {
		t.Fatalf("sweep: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "failures=0") {
		t.Fatalf("fixture should fetch and render everything:\n%s", out.String())
	}
	return root, filepath.Join(dest, "corpus")
}

// qualifyWALSource writes an unsupported SQLite source the way agy leaves a
// conversation it still holds open: the unsupported executor metadata is
// committed only to a nonempty, uncheckpointed -wal. The connection stays open
// until the test ends so nothing checkpoints the WAL away.
func qualifyWALSource(t *testing.T, root, id string, dbTime, walTime time.Time) {
	t.Helper()
	path := filepath.Join(root, "conversations", id+".db")
	db := testfixture.Source(t, path, qualifyPayload(id))
	testfixture.Exec(t, db, "PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0")
	testfixture.Exec(t, db, "INSERT INTO executor_metadata VALUES(0,?)", unknownPersistedField)
	if info, err := os.Stat(path + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("fixture WAL is missing or empty: %v", err)
	}
	for p, mtime := range map[string]time.Time{path: dbTime, path + "-wal": walTime} {
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *auditFixture) runReal(t *testing.T, root, corpus string, extra map[string]string, args ...string) (string, error) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"ANTIGRAVITY_CLI_ROOT":  root,
		"AGY_SIDECAR_CORPUS":    corpus,
		"AUDIT_TEST_READER_BIN": self,
	}
	for k, v := range extra {
		env[k] = v
	}
	return f.runWithOverrides("0", env, args...)
}

// The agy 1.2.15/1.2.16 audits: every conversation fetched and rendered and the
// explicit corpus was asserted swept, yet the newest scenario failed native
// verification and ordinary sync. No record may be offered or written.
func TestAuditRefusesRecordWhenRenderedConversationCannotSync(t *testing.T) {
	fixture := newAuditFixture(t)
	root, corpus := auditSweptStore(t, fixture, newestUnsupported)
	output, err := fixture.runReal(t, root, corpus, nil, "--record", "--corpus-swept")
	if err == nil {
		t.Fatalf("audit offered a record for an unsyncable conversation:\n%s", output)
	}
	for _, want := range []string{
		"scope: explicit-version-scoped",
		sweptUnsupportedNewestID + " NOT VERIFIED (audit-sweep manifest): unknown: unknown persisted protobuf fields",
		"Reader sync: FAIL",
		"no compatibility record produced",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "Read-only run. To record") || strings.Contains(output, "Recorded to:") {
		t.Errorf("record offered:\n%s", output)
	}
	if content, _ := os.ReadFile(fixture.compatFile); string(content) != "sentinel compatibility record\n" {
		t.Fatalf("compatibility record changed: %q", content)
	}

	// Without the sweep manifest the helper falls back to the live sidecar,
	// which ordinary sync never wrote; an explicit corpus is no substitute.
	if err := os.Rename(filepath.Join(filepath.Dir(corpus), "manifest.json"), filepath.Join(fixture.repo, "moved-manifest.json")); err != nil {
		t.Fatal(err)
	}
	output, err = fixture.runReal(t, root, corpus, nil, "--corpus-swept")
	if err == nil || !strings.Contains(output, "NOT VERIFIED (live sidecar attestation): unknown: no sidecar") {
		t.Fatalf("explicit corpus replaced the missing live sidecar: %v\n%s", err, output)
	}
}

// Broad historical inspection must keep working when old conversations fall
// outside the current profile: they are reported, and only the newest and the
// explicitly named conversations gate the record.
func TestAuditRecordsDespiteUnsupportedHistoricalConversations(t *testing.T) {
	fixture := newAuditFixture(t)
	root, corpus := auditSweptStore(t, fixture, newestVerified)
	output, err := fixture.runReal(t, root, corpus, nil, "--record", "--corpus-swept")
	if err != nil {
		t.Fatalf("historical unsupported conversation blocked the audit: %v\n%s", err, output)
	}
	for _, want := range []string{
		"not gating): verified=1/2 SQLite not-verified=1 legacy-unverifiable=0",
		qualifyVerifiedID + " VERIFIED (audit-sweep manifest)",
		"Reader sync: PASS (1 conversation(s) natively verified via audit-sweep manifest)",
		"Recorded to:",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
	content, err := os.ReadFile(fixture.compatFile)
	if err != nil || !strings.Contains(string(content), "**Reader sync qualified:** 1 conversation(s) natively verified") {
		t.Fatalf("record lacks reader-sync provenance: %v\n%s", err, content)
	}

	// Naming the old conversation as a scenario makes it gate the record.
	writeFileT(t, fixture.compatFile, []byte("sentinel compatibility record\n"))
	output, err = fixture.runReal(t, root, corpus, map[string]string{
		"AGY_QUALIFY_SESSIONS": qualifyUnsupportedID,
	}, "--record", "--corpus-swept")
	if err == nil || !strings.Contains(output, qualifyUnsupportedID+" NOT VERIFIED") {
		t.Fatalf("named unsupported conversation did not gate the record: %v\n%s", err, output)
	}
	if content, _ := os.ReadFile(fixture.compatFile); string(content) != "sentinel compatibility record\n" {
		t.Fatalf("compatibility record changed: %q", content)
	}
}

// agy keeps an open conversation's newest writes in its -wal until a
// checkpoint, so its main database can be older than another conversation's.
// The record gate must still treat it as the newest conversation, as discovery
// does, or an unsyncable conversation agy is still writing escapes the gate.
func TestAuditGatesNewestConversationByUncheckpointedWAL(t *testing.T) {
	fixture := newAuditFixture(t)
	root, corpus := auditSweptStore(t, fixture, newestUnsupportedInWAL)
	sessions, err := discovery.ListConversationSessions(root)
	if err != nil || len(sessions) == 0 || sessions[0].CascadeID != sweptUnsupportedNewestID {
		t.Fatalf("fixture: discovery should rank the WAL conversation newest: %v %+v", err, sessions)
	}
	output, err := fixture.runReal(t, root, corpus, nil, "--record", "--corpus-swept")
	if err == nil {
		t.Fatalf("audit recorded although the newest conversation cannot sync:\n%s", output)
	}
	for _, want := range []string{
		"Auditing Latest Session: " + sweptUnsupportedNewestID,
		sweptUnsupportedNewestID + " NOT VERIFIED (audit-sweep manifest): unknown: unknown persisted protobuf fields",
		"Reader sync: FAIL",
		"no compatibility record produced",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
	if content, _ := os.ReadFile(fixture.compatFile); string(content) != "sentinel compatibility record\n" {
		t.Fatalf("compatibility record changed: %q", content)
	}
}

// The helper's newest conversation follows discovery's rule: a nonempty -wal
// or -journal counts, an empty -wal (agy merely opened the file) does not, and
// -shm never does because readers, including the audit itself, update it.
func TestAuditNewestSessionCountsOnlyCommittedSidecarWrites(t *testing.T) {
	const otherID = "22222222-2222-4222-8222-222222222222"
	now := time.Now()
	for _, tc := range []struct {
		name   string
		suffix string
		data   []byte
		want   string
	}{
		{"nonempty wal", "-wal", []byte("frames"), otherID},
		{"nonempty journal", "-journal", []byte("pages"), otherID},
		{"empty wal", "-wal", nil, "11111111-1111-1111-1111-111111111111"},
		{"shm", "-shm", []byte("index"), "11111111-1111-1111-1111-111111111111"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newAuditFixture(t)
			conversations := filepath.Join(fixture.root, "conversations")
			newestDB := filepath.Join(conversations, "11111111-1111-1111-1111-111111111111.db")
			otherDB := filepath.Join(conversations, otherID+".db")
			writeFileT(t, otherDB, nil)
			writeFileT(t, otherDB+tc.suffix, tc.data)
			for p, mtime := range map[string]time.Time{
				otherDB:             now.Add(-3 * time.Hour),
				newestDB:            now.Add(-2 * time.Hour),
				otherDB + tc.suffix: now.Add(-time.Hour),
			} {
				if err := os.Chtimes(p, mtime, mtime); err != nil {
					t.Fatal(err)
				}
			}
			output, _ := fixture.run("0")
			if !strings.Contains(output, "Auditing Latest Session: "+tc.want+"\n") {
				t.Fatalf("newest session should be %s:\n%s", tc.want, output)
			}
		})
	}
}

// The documented audit sets AGY_QUALIFY_SESSIONS to live scenario IDs and then
// runs this suite. Helper fixtures must not inherit the caller's audit inputs,
// or a valid live ID would fail the audit's own unit-test step.
func TestAuditFixturesIgnoreCallerAuditInputs(t *testing.T) {
	t.Setenv("AGY_QUALIFY_SESSIONS", "77777777-7777-4777-8777-777777777777")
	t.Setenv("AGENTSVIEW_DIR", t.TempDir())
	fixture := newAuditFixture(t)
	root, corpus := auditSweptStore(t, fixture, newestVerified)
	output, err := fixture.runReal(t, root, corpus, nil, "--corpus-swept")
	if err != nil {
		t.Fatalf("caller's audit inputs leaked into the fixture: %v\n%s", err, output)
	}
	for _, want := range []string{
		"Reader sync: PASS (1 conversation(s) natively verified",
		"Skipping agentsview integration tests (AGENTSVIEW_DIR not specified).",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
}
