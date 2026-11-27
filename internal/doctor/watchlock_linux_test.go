//go:build linux

package doctor

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeProc lays out a /proc tree the way the kernel does: <pid>/cmdline,
// <pid>/environ, <pid>/fd/<n> symbolic links to the open files and
// <pid>/fdinfo/<n> files. Statting an fd link follows it to the real file, as
// it does under /proc.
type fakeProc struct {
	t    *testing.T
	root string
}

func newFakeProc(t *testing.T) *fakeProc {
	t.Helper()
	return &fakeProc{t: t, root: t.TempDir()}
}

// process adds pid with the given argv and HOME (empty = environ unreadable).
func (p *fakeProc) process(pid int, home string, argv ...string) string {
	p.t.Helper()
	dir := filepath.Join(p.root, strconv.Itoa(pid))
	for _, sub := range []string{"fd", "fdinfo"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			p.t.Fatal(err)
		}
	}
	p.write(filepath.Join(dir, "cmdline"), strings.Join(argv, "\x00")+"\x00")
	if home != "" {
		p.write(filepath.Join(dir, "environ"), "HOME="+home+"\x00")
	}
	return dir
}

// open gives the process an open file: fd n links to target and its fdinfo
// reads info.
func (p *fakeProc) open(dir string, n int, target, info string) {
	p.t.Helper()
	if err := os.Symlink(target, filepath.Join(dir, "fd", strconv.Itoa(n))); err != nil {
		p.t.Fatal(err)
	}
	p.write(filepath.Join(dir, "fdinfo", strconv.Itoa(n)), info)
}

func (p *fakeProc) write(path, data string) {
	p.t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		p.t.Fatal(err)
	}
}

func (p *fakeProc) publishRoot(procDir string, lockFD int, root string) {
	p.t.Helper()
	p.write(filepath.Join(procDir, "polling-root-"+strconv.Itoa(lockFD)), root)
}

func (p *fakeProc) scan(root string) (running, known bool) {
	p.t.Helper()
	return scanProcForWatch(p.root, 1, root, func(fdDir string, _ []os.DirEntry, lockFD string) string {
		value, _ := os.ReadFile(filepath.Join(fdDir, "..", "polling-root-"+lockFD))
		return string(value)
	})
}

// storeWithLock makes a store directory under t.TempDir(), with its lock file
// when withLock is set, and returns the store's root and the lock file's path.
func storeWithLock(t *testing.T, withLock bool) (root, lock string) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "antigravity-acp")
	if err := os.MkdirAll(filepath.Join(root, "conversations"), 0o755); err != nil {
		t.Fatal(err)
	}
	lock = filepath.Join(root, watchLockName)
	if withLock {
		if err := os.WriteFile(lock, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root, lock
}

func checkScan(t *testing.T, p *fakeProc, root string, wantRunning, wantKnown bool, why string) {
	t.Helper()
	if running, known := p.scan(root); running != wantRunning || known != wantKnown {
		t.Errorf("%s: scan = running %v, known %v; want running %v, known %v", why, running, known, wantRunning, wantKnown)
	}
}

// A watcher that holds a store's lock is syncing it: running, whatever the
// watcher was started with. A store with no lock file is left as it is.
func TestScanProcForWatchRunningWhenAWatcherHoldsTheLock(t *testing.T) {
	root, lock := storeWithLock(t, true)
	p := newFakeProc(t)
	w := p.process(100, t.TempDir(), "agy-reader", "--watch", "--root", root)
	p.open(w, 7, lock, fdinfoBase+flockLine)
	checkScan(t, p, root, false, false, "a watcher from before polling roots were recorded")
	p.publishRoot(w, 7, root)
	checkScan(t, p, root, true, true, "a watcher that holds the lock")

	// A lock held through a link to the store is the same lock.
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	checkScan(t, p, alias, true, true, "the store reached through a symlink")

	bare, _ := storeWithLock(t, false)
	checkScan(t, p, bare, false, true, "a store with no lock file")
	if _, err := os.Stat(filepath.Join(bare, watchLockName)); !os.IsNotExist(err) {
		t.Errorf("doctor created the lock file of a store nobody locked: %v", err)
	}
}

// The scenario of the issue at doctor's level: a bare watcher whose own HOME
// lists a store among the default stores, which never locked it.
func TestScanProcForWatchCoveringWatcherWithoutTheLockIsUnknown(t *testing.T) {
	home := t.TempDir()
	// A default store under the watcher's HOME that exists now.
	store := filepath.Join(home, ".gemini", "antigravity")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	// Another default store the watcher started with and locked.
	other := filepath.Join(home, ".gemini", "antigravity-cli")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	otherLock := filepath.Join(other, watchLockName)
	if err := os.WriteFile(otherLock, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	p := newFakeProc(t)
	watcher := p.process(100, home, "agy-reader", "--watch")
	p.open(watcher, 7, otherLock, fdinfoBase+flockLine)
	p.publishRoot(watcher, 7, other)

	if covered, _ := WatchProcCoverage(nil, []byte("HOME="+home+"\x00"), nil, store); !covered {
		t.Fatal("setup: a bare watcher under this HOME covers the default store by recomputation")
	}
	checkScan(t, p, other, true, true, "the store the watcher locked")
	checkScan(t, p, store, false, false, "a default store the watcher covers on paper but never locked")
	if _, err := os.Stat(filepath.Join(store, watchLockName)); !os.IsNotExist(err) {
		t.Errorf("doctor created a lock file in a store nobody locked: %v", err)
	}

	// Watchers given their roots by flag are no different: a root named with
	// --root that does not hold the lock cannot be shown as watched either.
	p.process(101, "", "agy-reader", "--watch", "--root", store)
	checkScan(t, p, store, false, false, "a root named by --root whose lock no watcher holds")
}

func TestScanProcForWatchIgnoresWhatIsNotTheWatchLock(t *testing.T) {
	root, lock := storeWithLock(t, true)
	otherRoot, otherLock := storeWithLock(t, true)

	p := newFakeProc(t)
	// A watcher holding the lock of a different store: same name, other file.
	elsewhere := p.process(100, t.TempDir(), "agy-reader", "--watch", "--root", otherRoot)
	p.open(elsewhere, 7, otherLock, fdinfoBase+flockLine)
	checkScan(t, p, root, false, true, "a lock file of another store")

	// A process that has the lock file open but is not a watcher (doctor's own
	// kind, or the test helper) does not make a watcher.
	other := p.process(200, "", "agy-reader", "doctor")
	p.open(other, 7, lock, fdinfoBase+flockLine)
	checkScan(t, p, root, false, true, "a process that is not a watcher")

	// Our own process is never the watcher.
	self := p.process(1, "", "agy-reader", "--watch")
	p.open(self, 7, lock, fdinfoBase+flockLine)
	checkScan(t, p, root, false, true, "our own process")

	// A watcher whose file named like the lock is not the store's lock file.
	decoy := filepath.Join(t.TempDir(), watchLockName)
	if err := os.WriteFile(decoy, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	liar := p.process(300, t.TempDir(), "agy-reader", "--watch", "--root", t.TempDir())
	p.open(liar, 7, decoy, fdinfoBase+flockLine)
	checkScan(t, p, root, false, true, "a different file with the same name")
}

// Having the lock file open is not holding the lock: acquireWatchLock opens the
// file before it tries to lock it, and a watcher that lost the race closes it
// again. Only an exclusive flock the kernel shows on the open file counts.
func TestScanProcForWatchRequiresAnExclusiveFlockOnTheOpenFile(t *testing.T) {
	for name, info := range map[string]string{
		"open but not locked":        fdinfoBase,
		"a shared lock":              fdinfoBase + "lock:\t1: FLOCK  ADVISORY  READ 4242 00:24:985401497 0 EOF\n",
		"a lock still being waited":  fdinfoBase + "lock:\t1: -> FLOCK  ADVISORY  WRITE 4242 00:24:985401497 0 EOF\n",
		"a record lock, not a flock": fdinfoBase + "lock:\t1: POSIX  ADVISORY  WRITE 4242 00:24:985401497 0 EOF\n",
	} {
		t.Run(name, func(t *testing.T) {
			root, lock := storeWithLock(t, true)
			p := newFakeProc(t)
			w := p.process(100, "", "agy-reader", "--watch", "--root", root)
			p.open(w, 7, lock, info)
			// The watcher is meant to cover the root but cannot be shown to hold it.
			checkScan(t, p, root, false, false, name)
		})
	}

	t.Run("the lock among other open files", func(t *testing.T) {
		root, lock := storeWithLock(t, true)
		p := newFakeProc(t)
		w := p.process(100, "", "agy-reader", "--watch", "--root", root)
		p.open(w, 0, os.DevNull, "pos:\t0\n")
		p.open(w, 3, filepath.Join(root, "conversations"), "pos:\t0\n")
		p.open(w, 9, lock, fdinfoBase+flockLine)
		p.publishRoot(w, 9, root)
		checkScan(t, p, root, true, true, "a watcher with several files open")
	})
}

func TestScanProcForWatchUnreadableEvidenceIsUnknown(t *testing.T) {
	t.Run("a watcher's open files cannot be listed", func(t *testing.T) {
		root, _ := storeWithLock(t, true)
		p := newFakeProc(t)
		w := p.process(100, "", "agy-reader", "--watch", "--root", t.TempDir())
		if err := os.RemoveAll(filepath.Join(w, "fd")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(w, "fd"), []byte("not a directory"), 0o644); err != nil {
			t.Fatal(err)
		}
		checkScan(t, p, root, false, true, "unreadable descriptors for an explicitly unrelated watcher")
		p.write(filepath.Join(w, "cmdline"), "agy-reader\x00--watch\x00--root\x00"+root+"\x00")
		checkScan(t, p, root, false, false, "unreadable descriptors for a covering watcher")
	})

	t.Run("the fdinfo of the lock cannot be read", func(t *testing.T) {
		root, lock := storeWithLock(t, true)
		p := newFakeProc(t)
		w := p.process(100, "", "agy-reader", "--watch", "--root", t.TempDir())
		p.open(w, 7, lock, "")
		if err := os.Remove(filepath.Join(w, "fdinfo", "7")); err != nil {
			t.Fatal(err)
		}
		checkScan(t, p, root, false, true, "unreadable fdinfo for an explicitly unrelated watcher")
		p.write(filepath.Join(w, "cmdline"), "agy-reader\x00--watch\x00--root\x00"+root+"\x00")
		checkScan(t, p, root, false, false, "fdinfo missing for the lock's fd")
	})

	t.Run("the lock file cannot be examined", func(t *testing.T) {
		// A store whose lock path cannot be statted for a reason other than
		// absence: here a path component that is a file.
		parent := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(parent, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		root := filepath.Join(parent, "store")
		p := newFakeProc(t)
		p.process(100, "", "agy-reader", "--watch", "--root", t.TempDir())
		checkScan(t, p, root, false, false, "a lock file that cannot be statted while a watcher runs")
		// With no watcher at all there is still nothing to show or hide.
		checkScan(t, newFakeProc(t), root, false, true, "no watcher")
	})

	t.Run("a bare watcher's environment cannot be read", func(t *testing.T) {
		root, _ := storeWithLock(t, false)
		p := newFakeProc(t)
		p.process(100, "", "agy-reader", "--watch") // no environ
		checkScan(t, p, root, false, false, "a bare watcher with no readable environment")
	})
}

// Stores no watcher covers and none holds stay "not running", and the only
// processes that can make that unknown are watchers.
func TestScanProcForWatchNoWatcherIsNotRunning(t *testing.T) {
	root, _ := storeWithLock(t, false)
	p := newFakeProc(t)
	checkScan(t, p, root, false, true, "an empty process table")

	p.process(100, "", "bash")
	p.process(101, "", "agy-reader", "doctor")
	checkScan(t, p, root, false, true, "processes that are not watchers")

	p.process(102, "", "agy-reader", "--watch", "--root", t.TempDir())
	checkScan(t, p, root, false, true, "a watcher of another root")
}

// moving a watched store leaves the watcher holding the moved
// lock inode while still polling the old root; hard-linking the lock into
// another store also shares the inode. Neither may report watch: running.
func TestScanProcForWatchRejectsMovedStoreAndHardLinkedLock(t *testing.T) {
	t.Run("moved store", func(t *testing.T) {
		base := t.TempDir()
		store := filepath.Join(base, "store")
		if err := os.MkdirAll(filepath.Join(store, "conversations"), 0o755); err != nil {
			t.Fatal(err)
		}
		lock := filepath.Join(store, watchLockName)
		if err := os.WriteFile(lock, nil, 0o600); err != nil {
			t.Fatal(err)
		}

		archive := filepath.Join(base, "archive")
		if err := os.Rename(store, archive); err != nil {
			t.Fatal(err)
		}
		// On Linux, /proc/<pid>/fd/<n> updates to the renamed directory path.
		p := newFakeProc(t)
		w := p.process(100, t.TempDir(), "agy-reader", "--watch", "--root", store)
		p.open(w, 7, filepath.Join(archive, watchLockName), fdinfoBase+flockLine)
		p.publishRoot(w, 7, store)
		checkScan(t, p, archive, false, false, "a moved store whose lock the old-root watcher still holds")
	})

	t.Run("hard-linked lock under an uncovered store", func(t *testing.T) {
		base := t.TempDir()
		store := filepath.Join(base, "store")
		other := filepath.Join(base, "other")
		for _, dir := range []string{store, other} {
			if err := os.MkdirAll(filepath.Join(dir, "conversations"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		lock := filepath.Join(store, watchLockName)
		if err := os.WriteFile(lock, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(lock, filepath.Join(other, watchLockName)); err != nil {
			t.Fatal(err)
		}

		p := newFakeProc(t)
		w := p.process(100, t.TempDir(), "agy-reader", "--watch", "--root", store)
		p.open(w, 7, lock, fdinfoBase+flockLine)
		p.publishRoot(w, 7, store)
		checkScan(t, p, store, true, true, "the watched store")
		checkScan(t, p, other, false, true, "an uncovered store sharing the lock inode via hard link")
	})

	t.Run("hard-linked lock under a covered default store", func(t *testing.T) {
		home := t.TempDir()
		cliRoot := filepath.Join(home, ".gemini", "antigravity-cli")
		ideRoot := filepath.Join(home, ".gemini", "antigravity")
		for _, dir := range []string{cliRoot, ideRoot} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		cliLock := filepath.Join(cliRoot, watchLockName)
		if err := os.WriteFile(cliLock, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(cliLock, filepath.Join(ideRoot, watchLockName)); err != nil {
			t.Fatal(err)
		}

		p := newFakeProc(t)
		w := p.process(100, home, "agy-reader", "--watch")
		p.open(w, 7, cliLock, fdinfoBase+flockLine)
		p.publishRoot(w, 7, cliRoot)
		checkScan(t, p, cliRoot, true, true, "the default store whose lock the watcher holds")
		checkScan(t, p, ideRoot, false, false, "another default store whose lock is only a hard link")
	})

	t.Run("bare watcher holding the lock with unreadable environ", func(t *testing.T) {
		root, lock := storeWithLock(t, true)
		p := newFakeProc(t)
		w := p.process(100, "", "agy-reader", "--watch") // no environ
		p.open(w, 7, lock, fdinfoBase+flockLine)
		checkScan(t, p, root, false, false, "a bare watcher holding the lock when coverage cannot be verified")
	})
}

// when .agy-reader-watch.lock is a symbolic link to a
// differently named file, the kernel exposes the target's resolved path in
// /proc/<pid>/fd/<n>. Doctor must still recognize the held lock.
func TestScanProcForWatchRecognizesSymlinkedLockFile(t *testing.T) {
	t.Run("relative symlink to a differently named file", func(t *testing.T) {
		root, lock := storeWithLock(t, false)
		target := filepath.Join(root, "reader.lock")
		if err := os.WriteFile(target, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("reader.lock", lock); err != nil {
			t.Fatal(err)
		}

		p := newFakeProc(t)
		w := p.process(100, t.TempDir(), "agy-reader", "--watch", "--root", root)
		p.open(w, 7, target, fdinfoBase+flockLine)
		p.publishRoot(w, 7, root)
		checkScan(t, p, root, true, true, "a watcher holding a symlinked lock file")

		alias := filepath.Join(t.TempDir(), "alias")
		if err := os.Symlink(root, alias); err != nil {
			t.Skipf("directory symlinks unavailable: %v", err)
		}
		checkScan(t, p, alias, true, true, "a symlinked store with a symlinked lock file")
	})

	t.Run("symlink to an external lock file", func(t *testing.T) {
		root, lock := storeWithLock(t, false)
		ext := filepath.Join(t.TempDir(), "custom.lock")
		if err := os.WriteFile(ext, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(ext, lock); err != nil {
			t.Fatal(err)
		}

		p := newFakeProc(t)
		w := p.process(100, t.TempDir(), "agy-reader", "--watch", "--root", root)
		p.open(w, 7, ext, fdinfoBase+flockLine)
		p.publishRoot(w, 7, root)
		checkScan(t, p, root, true, true, "a watcher holding a lock symlinked outside the store")
	})
}
