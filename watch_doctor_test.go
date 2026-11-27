//go:build linux

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// watchEnv is the environment of a user's watcher or doctor: a home directory
// of its own and none of the variables that pin a root or a daemon.
// AGY_READER_TEST_AS_CLI makes the test binary run the real command. The child
// gets GOMAXPROCS=2 because the package's proxy tests already start about
// twenty child processes at once and the gate runs under a task limit.
func watchEnv(home string) []string {
	env := []string{}
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "HOME", "XDG_CONFIG_HOME", "GOMAXPROCS", "PATH", "ANTIGRAVITY_CLI_ROOT", "ANTIGRAVITY_DAEMON_URL", "ANTIGRAVITY_CSRF_TOKEN":
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"AGY_READER_TEST_AS_CLI=1",
		"GOMAXPROCS=2",
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"), // no IDE logs
		"PATH=", // doctor must not run the real agy
	)
}

func startWatch(t *testing.T, home string, extraArgs ...string) (logs *lockedBuffer, exited <-chan struct{}) {
	t.Helper()
	args := append([]string{"--watch", "--watch-interval=100ms"}, extraArgs...)
	cmd := exec.Command(os.Args[0], args...)
	cmd.Args[0] = "agy-reader"
	cmd.Env = watchEnv(home)
	logs = &lockedBuffer{}
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		cmd.Process.Kill()
		<-done
	})
	return logs, done
}

// startBareWatch runs `agy-reader --watch` as a child process the way a user's
// systemd unit does: the test binary in CLI mode, named agy-reader so doctor's
// /proc scan recognises it. It is killed when the test ends.
func startBareWatch(t *testing.T, home string) (logs *lockedBuffer, exited <-chan struct{}) {
	t.Helper()
	return startWatch(t, home)
}

// cliStoreWithSession gives home the agy CLI store with one session and a live
// daemon, and returns the root and the path its sidecar will be written to.
func cliStoreWithSession(t *testing.T, home string) (root, sidecar string) {
	t.Helper()
	root = filepath.Join(home, ".gemini", "antigravity-cli")
	seedPB(t, root, "conversations", "cli-aaa", time.Now().Add(-time.Hour))
	srv := fakeDaemon(t, nil)
	t.Cleanup(srv.Close)
	advertiseCLIDaemon(t, root, srv.URL)
	return root, filepath.Join(root, "conversations", "cli-aaa.trajectory.json")
}

// storeWithSession populates root with one session and a live CLI daemon, and
// returns the path its sidecar will be written to.
func storeWithSession(t *testing.T, root, id string) string {
	t.Helper()
	seedPB(t, root, "conversations", id, time.Now().Add(-time.Hour))
	srv := fakeDaemon(t, nil)
	t.Cleanup(srv.Close)
	advertiseCLIDaemon(t, root, srv.URL)
	return filepath.Join(root, "conversations", id+".trajectory.json")
}

// createT3Store does what T3 Code does the first time a user opens an
// Antigravity conversation: it creates providers/antigravity/<instance>/
// antigravity-acp, whose language server writes log/acp-server-*.log, and a
// conversation appears in it. The store is built beside its place and renamed
// into it, so a watcher never sees a half-made store.
func createT3Store(t *testing.T, home string) (root, sidecar string) {
	t.Helper()
	root = filepath.Join(home, ".t3", "userdata", "providers", "antigravity", "instance1", "antigravity-acp")
	staged := filepath.Join(home, "staged", "antigravity-acp")
	seedPB(t, staged, "conversations", "acp-bbb", time.Now().Add(-time.Hour))
	srv := fakeDaemon(t, nil)
	t.Cleanup(srv.Close)
	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")
	writeFileT(t, filepath.Join(staged, "log", "acp-server-20260101_000000-111.log"),
		[]byte("Language server listening on random port at "+port+" for HTTP\n"))
	if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(staged, root); err != nil {
		t.Fatal(err)
	}
	return root, filepath.Join(root, "conversations", "acp-bbb.trajectory.json")
}

// waitForSidecar waits until the watcher has written path, which it does only
// after it has taken the store's lock and ticked.
func waitForSidecar(t *testing.T, path string, logs *lockedBuffer, exited <-chan struct{}, why string) {
	t.Helper()
	deadline := time.After(15 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		select {
		case <-exited:
			t.Fatalf("watcher exited: %s\n%s", why, logs.String())
		case <-deadline:
			t.Fatalf("%s\nwatcher log:\n%s", why, logs.String())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// doctorWatchLine runs `agy-reader --root <root> doctor` as the real command
// and returns its `watch:` line. The exit code is not the point here: a store
// with stale sidecars exits 1.
func doctorWatchLine(t *testing.T, home, root string) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "--root", root, "doctor")
	cmd.Env = watchEnv(home)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run()
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "watch:") {
			return strings.TrimSpace(line)
		}
	}
	t.Fatalf("doctor printed no watch line for %s:\n%s", root, out.String())
	return ""
}

func watchStatus(line string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "watch:"))
}

// ensure the positive watch assertion rejects "not running".
func TestWatchStatusRejectsNotRunning(t *testing.T) {
	if got := watchStatus("watch:       running"); got != "running" {
		t.Fatalf("watchStatus(running) = %q, want %q", got, "running")
	}
	for _, line := range []string{
		"watch:       not running (optional: agy-reader --watch)",
		"watch:       unknown (optional: agy-reader --watch)",
	} {
		if got := watchStatus(line); got == "running" {
			t.Fatalf("watchStatus(%q) = %q; must not match running", line, got)
		}
	}
}

// a bare `agy-reader --watch` resolves its roots once, so a
// T3 Code store created after it started is never polled. Doctor recomputed the
// default stores from the watcher's HOME and printed `watch: running` for the
// new store anyway. It may say running only for a store the watcher holds the
// lock of; for the new one it cannot show that, so it says unknown.
func TestDoctorDoesNotCallAT3StoreCreatedAfterTheWatcherStartedWatched(t *testing.T) {
	home := t.TempDir()
	cliRoot, cliSidecar := cliStoreWithSession(t, home)

	logs, exited := startBareWatch(t, home)
	// The watcher is up and ticking once it has synced the store it started with.
	waitForSidecar(t, cliSidecar, logs, exited, "the CLI store's session never synced")

	acpRoot, acpSidecar := createT3Store(t, home)
	// Several more ticks, each of which a watcher that looked for new stores
	// would have used to find this one.
	ticks := strings.Count(logs.String(), "tick: ")
	waitForLog(t, logs, exited, "three more ticks", func(s string) bool {
		return strings.Count(s, "tick: ") >= ticks+3
	})
	if _, err := os.Stat(acpSidecar); err == nil {
		t.Fatalf("the watcher synced a store created after it started; this test is about one that never does\n%s", logs.String())
	}

	if line := doctorWatchLine(t, home, acpRoot); !strings.HasPrefix(watchStatus(line), "unknown") {
		t.Fatalf("doctor printed %q for a store the watcher never picked up; want watch: unknown\nwatcher log:\n%s", line, logs.String())
	}
	if _, err := os.Stat(filepath.Join(acpRoot, ".agy-reader-watch.lock")); err == nil {
		t.Fatal("doctor created the watch lock of a store nobody watches")
	}
	// The store the watcher did pick up is still reported as watched.
	if line := doctorWatchLine(t, home, cliRoot); watchStatus(line) != "running" {
		t.Fatalf("doctor printed %q for the store the watcher started with; want watch: running", line)
	}
}

// A store the watcher picked up when it started, T3 Code's included, is still
// reported as watched, and doctor never takes its lock: the watcher keeps it.
func TestDoctorReportsStoresTheWatcherPickedUpAtStartAsRunning(t *testing.T) {
	home := t.TempDir()
	cliRoot, cliSidecar := cliStoreWithSession(t, home)
	acpRoot, acpSidecar := createT3Store(t, home)

	logs, exited := startBareWatch(t, home)
	waitForSidecar(t, cliSidecar, logs, exited, "the CLI store's session never synced")
	waitForSidecar(t, acpSidecar, logs, exited, "the T3 Code store's session never synced")

	for _, root := range []string{cliRoot, acpRoot} {
		if line := doctorWatchLine(t, home, root); watchStatus(line) != "running" {
			t.Fatalf("doctor printed %q for %s, which the watcher syncs; want watch: running\nwatcher log:\n%s", line, root, logs.String())
		}
	}
	// Doctor left the lock with the watcher: the watcher goes on syncing, and
	// no watcher of ours is told another process owns a store.
	if s := logs.String(); strings.Contains(s, "another watcher owns this store") {
		t.Fatalf("a watcher was kept out of its store:\n%s", s)
	}
}

// moving a watched store leaves the watcher holding the moved
// lock inode while still polling the original root path; doctor must not report
// watch: running for the moved store.
func TestDoctorDoesNotCallAMovedStoreWatched(t *testing.T) {
	home := t.TempDir()
	base := t.TempDir()
	store := filepath.Join(base, "store")
	sidecar := storeWithSession(t, store, "cli-move")

	logs, exited := startWatch(t, home, "--root", store)
	waitForSidecar(t, sidecar, logs, exited, "the store's session never synced")

	archive := filepath.Join(base, "archive")
	if err := os.Rename(store, archive); err != nil {
		t.Fatal(err)
	}
	ticks := strings.Count(logs.String(), "tick: ")
	waitForLog(t, logs, exited, "another watch tick after moving the store", func(s string) bool {
		return strings.Count(s, "tick: ") >= ticks+2
	})

	line := doctorWatchLine(t, home, archive)
	if status := watchStatus(line); status == "running" || !strings.HasPrefix(status, "unknown") {
		t.Fatalf("doctor printed %q for a moved store; want watch: unknown\nwatcher log:\n%s", line, logs.String())
	}
}

// hard-linking a watched store's lock into another store
// shares the inode, so doctor must not report the other store as watched.
func TestDoctorDoesNotCallAHardLinkedLockWatched(t *testing.T) {
	home := t.TempDir()
	base := t.TempDir()
	store := filepath.Join(base, "store")
	sidecar := storeWithSession(t, store, "cli-hardlink")

	logs, exited := startWatch(t, home, "--root", store)
	waitForSidecar(t, sidecar, logs, exited, "the store's session never synced")

	other := filepath.Join(base, "other")
	if err := os.MkdirAll(filepath.Join(other, "conversations"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(store, ".agy-reader-watch.lock"), filepath.Join(other, ".agy-reader-watch.lock")); err != nil {
		t.Fatal(err)
	}

	line := doctorWatchLine(t, home, other)
	if status := watchStatus(line); !strings.HasPrefix(status, "not running") && !strings.HasPrefix(status, "unknown") {
		t.Fatalf("doctor printed %q for another store with a hard-linked lock; want watch: not running or unknown\nwatcher log:\n%s", line, logs.String())
	}
}

// when .agy-reader-watch.lock is a symlink to a differently
// named file, doctor still sees the watcher holding the lock.
func TestDoctorRecognizesSymlinkedWatchLock(t *testing.T) {
	home := t.TempDir()
	store := filepath.Join(t.TempDir(), "store")
	sidecar := storeWithSession(t, store, "cli-symlink-lock")
	target := filepath.Join(store, "reader.lock")
	writeFileT(t, target, nil)
	if err := os.Symlink("reader.lock", filepath.Join(store, ".agy-reader-watch.lock")); err != nil {
		t.Fatal(err)
	}

	logs, exited := startWatch(t, home, "--root", store)
	waitForSidecar(t, sidecar, logs, exited, "the store's session never synced")

	if line := doctorWatchLine(t, home, store); watchStatus(line) != "running" {
		t.Fatalf("doctor printed %q for a store with a symlinked lock; want watch: running\nwatcher log:\n%s", line, logs.String())
	}
}

func TestDoctorDoesNotCallARenamedDefaultT3StoreWatched(t *testing.T) {
	home := t.TempDir()
	root, sidecar := createT3Store(t, home)
	logs, exited := startBareWatch(t, home)
	waitForSidecar(t, sidecar, logs, exited, "the original T3 store never synced")
	if line := doctorWatchLine(t, home, root); watchStatus(line) != "running" {
		t.Fatalf("doctor printed %q for the original T3 store", line)
	}

	instance := filepath.Dir(root)
	moved := filepath.Join(filepath.Dir(instance), "moved")
	if err := os.Rename(instance, moved); err != nil {
		t.Fatal(err)
	}
	line := doctorWatchLine(t, home, filepath.Join(moved, filepath.Base(root)))
	if !strings.HasPrefix(watchStatus(line), "unknown") {
		t.Fatalf("doctor printed %q for a renamed default T3 store; want watch: unknown", line)
	}
}

func TestDoctorDoesNotAttributeASharedLockToBothStores(t *testing.T) {
	home := t.TempDir()
	base := t.TempDir()
	first, second := filepath.Join(base, "first"), filepath.Join(base, "second")
	firstSidecar := storeWithSession(t, first, "first")
	secondSidecar := storeWithSession(t, second, "second")
	lock := filepath.Join(base, "shared.lock")
	writeFileT(t, lock, []byte("original lock contents"))
	for _, root := range []string{first, second} {
		if err := os.Symlink(lock, filepath.Join(root, ".agy-reader-watch.lock")); err != nil {
			t.Fatal(err)
		}
	}
	logs, exited := startWatch(t, home, "--root", first, "--root", second)
	waitForSidecar(t, firstSidecar, logs, exited, "the first store never synced")
	waitForLog(t, logs, exited, "second store waiting for the shared lock", func(s string) bool {
		return strings.Contains(s, "another watcher owns this store")
	})

	if line := doctorWatchLine(t, home, first); watchStatus(line) != "running" {
		t.Fatalf("doctor printed %q for the store holding the shared lock", line)
	}
	if line := doctorWatchLine(t, home, second); !strings.HasPrefix(watchStatus(line), "unknown") {
		t.Fatalf("doctor printed %q for the store waiting for the shared lock; want watch: unknown", line)
	}
	if _, err := os.Stat(secondSidecar); !os.IsNotExist(err) {
		t.Fatalf("expected no sidecar in the waiting store, stat error: %v", err)
	}
	if contents, err := os.ReadFile(lock); err != nil || string(contents) != "original lock contents" {
		t.Fatalf("shared lock contents changed: %q, error %v", contents, err)
	}
}
