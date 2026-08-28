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
    printf '%s\n' '#!/usr/bin/env bash' 'echo `+fakeShapeFingerprint+`' > "$output"
    chmod +x "$output"
    ;;
test) ;;
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

func overriddenEnv(overrides map[string]string) []string {
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
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
