//go:build linux

package vcache

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/discovery"
	_ "modernc.org/sqlite"
)

func dummyFixture(t *testing.T) (string, discovery.Session, *Cache) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session.db")
	sidecarPath := filepath.Join(dir, "session.db.json")
	if err := os.WriteFile(dbPath, []byte("database content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecarPath, []byte(`{"version":1,"content":"current"}`), 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(dbPath)
	s := discovery.Session{
		PBPath:      dbPath,
		SidecarPath: sidecarPath,
		ModTime:     info.ModTime(),
	}
	c := Open(dir, t.TempDir(), "test-boot")
	return dir, s, c
}

func alwaysFresh(discovery.Session) (bool, string) {
	return false, "verified"
}

func TestCacheRestartAndIndependentCheck(t *testing.T) {
	dir, s, c := dummyFixture(t)
	if stale, reason := c.IsStale(s, alwaysFresh); stale {
		t.Fatal(reason)
	}
	reused, checked := c.Stats()
	if checked != 1 || len(c.entries) != 1 {
		t.Fatalf("not cached: reused=%d checked=%d entries=%d", reused, checked, len(c.entries))
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(c.path)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("cache permissions: %v", info.Mode())
	}
	restarted := Open(dir, filepath.Dir(filepath.Dir(c.path)), "test-boot")
	called := false
	stale, reason := restarted.IsStale(s, func(discovery.Session) (bool, string) {
		called = true
		return false, "verified"
	})
	if stale || reason != "verified-cached" || called {
		t.Fatalf("restart missed: stale=%v reason=%s called=%v", stale, reason, called)
	}
	rReused, rChecked := restarted.Stats()
	if rReused != 1 || rChecked != 0 {
		t.Fatalf("unexpected stats: reused=%d checked=%d", rReused, rChecked)
	}
	// A hit must not keep extending the full-verification deadline.
	if !restarted.entries[s.PBPath].CheckedAt.Equal(c.entries[s.PBPath].CheckedAt) {
		t.Fatal("hit renewed verification")
	}
}

func TestCacheInvalidation(t *testing.T) {
	for _, action := range []string{"same-size-preserved-mtime", "atomic-replacement", "missing-sidecar", "expiry", "clock-backwards"} {
		t.Run(action, func(t *testing.T) {
			_, s, c := dummyFixture(t)
			c.IsStale(s, alwaysFresh)
			info, _ := os.Stat(s.SidecarPath)
			switch action {
			case "same-size-preserved-mtime", "atomic-replacement":
				raw, _ := os.ReadFile(s.SidecarPath)
				// Alter content preserving length
				copy(raw[len(raw)-5:], []byte("12345"))
				path := s.SidecarPath
				if action == "atomic-replacement" {
					path += ".replacement"
				}
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
				if path != s.SidecarPath {
					if err := os.Rename(path, s.SidecarPath); err != nil {
						t.Fatal(err)
					}
				}
			case "missing-sidecar":
				if err := os.Remove(s.SidecarPath); err != nil {
					t.Fatal(err)
				}
			case "expiry":
				now := c.entries[s.PBPath].CheckedAt.Add(5 * time.Minute)
				c.SetNow(func() time.Time { return now })
			case "clock-backwards":
				now := c.entries[s.PBPath].CheckedAt.Add(-time.Second)
				c.SetNow(func() time.Time { return now })
			}
			stale, _ := c.IsStale(s, func(s discovery.Session) (bool, string) {
				if _, err := os.Stat(s.SidecarPath); err != nil {
					return true, "missing-sidecar"
				}
				return false, "verified"
			})
			reused, checked := c.Stats()
			if reused != 0 || checked != 2 {
				t.Fatalf("missed invalidation: reused=%d checked=%d", reused, checked)
			}
			if action == "missing-sidecar" && !stale {
				t.Fatal("missing sidecar accepted")
			}
		})
	}
}

func TestCacheDiskFailuresAndCompatibility(t *testing.T) {
	for _, action := range []string{"corrupt", "version", "profile", "boot", "root", "unwritable"} {
		t.Run(action, func(t *testing.T) {
			dir, s, c := dummyFixture(t)
			c.IsStale(s, alwaysFresh)
			if err := c.Save(); err != nil {
				t.Fatal(err)
			}
			cacheDir := filepath.Dir(filepath.Dir(c.path))
			if action == "unwritable" {
				c.path = filepath.Join(s.PBPath, "cannot-create.json")
				c.dirty = true
				if c.Save() == nil {
					t.Fatal("expected save failure")
				}
				if stale, reason := c.IsStale(s, alwaysFresh); stale || reason != "verified-cached" {
					t.Fatal("save failure lost memory cache")
				}
				return
			}
			raw, _ := os.ReadFile(c.path)
			var disk DiskCache
			if err := json.Unmarshal(raw, &disk); err != nil {
				t.Fatal(err)
			}
			switch action {
			case "version":
				disk.Version++
			case "profile":
				disk.Profile = "other-profile"
			case "boot":
				disk.Boot = "previous-boot"
			case "root":
				disk.Root = "other-root"
			}
			raw, _ = json.Marshal(disk)
			if action == "corrupt" {
				raw = []byte("{")
			}
			if err := os.WriteFile(c.path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			next := Open(dir, cacheDir, "test-boot")
			called := false
			stale, reason := next.IsStale(s, func(discovery.Session) (bool, string) {
				called = true
				return false, "verified"
			})
			if stale || reason == "verified-cached" || !called {
				t.Fatalf("bad cache reused: %v %s called=%v", stale, reason, called)
			}
		})
	}
}

func TestCachePruning(t *testing.T) {
	_, s, c := dummyFixture(t)
	c.IsStale(s, alwaysFresh)
	if len(c.entries) != 1 {
		t.Fatal("not entered")
	}
	c.Prune([]discovery.Session{})
	if len(c.entries) != 0 {
		t.Fatal("session not pruned")
	}
}

func TestCacheDoesNotRememberUnstableCheck(t *testing.T) {
	_, s, c := dummyFixture(t)
	// Change metadata between the pre-check fingerprint and full verification.
	c.SetNow(func() time.Time {
		info, err := os.Stat(s.SidecarPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(s.SidecarPath, info.ModTime(), info.ModTime().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		return time.Now()
	})
	if stale, reason := c.IsStale(s, alwaysFresh); stale {
		t.Fatal(reason)
	}
	if len(c.entries) != 0 {
		t.Fatal("unstable check was cached")
	}
}

func TestCacheNonDatabaseSource(t *testing.T) {
	dir := t.TempDir()
	c := Open(dir, t.TempDir(), "test-boot")
	pbSession := discovery.Session{PBPath: filepath.Join(dir, "legacy.pb")}
	called := false
	c.IsStale(pbSession, func(discovery.Session) (bool, string) {
		called = true
		return false, "legacy"
	})
	if !called {
		t.Fatal("legacy .pb bypassed callback")
	}
	if len(c.entries) != 0 {
		t.Fatal("legacy .pb entered sqlite cache")
	}
}

func TestSourceFingerprintNeedsNoSidecar(t *testing.T) {
	_, s, _ := dummyFixture(t)
	if err := os.Remove(s.SidecarPath); err != nil {
		t.Fatal(err)
	}
	files, ok := SourceFingerprint(s.PBPath)
	if !ok || !files.Source.Exists || files.Sidecar.Exists {
		t.Fatalf("source without a sidecar not fingerprinted: ok=%v files=%+v", ok, files)
	}
	if _, ok := Fingerprint(s); ok {
		t.Fatal("Fingerprint accepted a missing sidecar")
	}
}

// walFixture replaces the fixture's source with a real WAL-mode database that a
// writer keeps open with automatic checkpoints off, the state of a session agy
// has open: its WAL keeps frames and -shm holds the published WAL-index header.
func walFixture(t *testing.T) (discovery.Session, *Cache, *sql.DB) {
	t.Helper()
	_, s, c := dummyFixture(t)
	if err := os.Remove(s.PBPath); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", s.PBPath)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	for _, q := range []string{"PRAGMA journal_mode=WAL", "PRAGMA wal_autocheckpoint=0", "CREATE TABLE steps(idx INTEGER)", "INSERT INTO steps VALUES(0)"} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if info, err := os.Stat(s.PBPath + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("fixture WAL has no frames: %v", err)
	}
	return s, c, db
}

func shmHeader(t *testing.T, s discovery.Session) []byte {
	t.Helper()
	f, err := os.Open(s.PBPath + "-shm")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b := make([]byte, 2*walIndexHeaderSize)
	if _, err := f.ReadAt(b, 0); err != nil {
		t.Fatal(err)
	}
	return b
}

func writeShmHeader(t *testing.T, s discovery.Session, b []byte) {
	t.Helper()
	f, err := os.OpenFile(s.PBPath+"-shm", os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteAt(b, 0); err != nil {
		t.Fatal(err)
	}
}

// A session agy has open, with frames in its WAL and nothing changing, is
// checked once and then reused while the database, WAL and published
// WAL-index header are unchanged. The real SQLite header also proves the
// header checksum is read correctly: a header that failed it would leave the
// session checked on every lookup.
func TestCacheReusesOpenSessionWhileWALIndexUnchanged(t *testing.T) {
	s, c, db := walFixture(t)
	if files, ok := Fingerprint(s); !ok || files.WALIndex == "" {
		t.Fatalf("real WAL-index header not read: ok=%v files=%+v", ok, files)
	}
	calls := 0
	check := func(discovery.Session) (bool, string) {
		calls++
		return false, "verified"
	}
	for range 4 {
		if stale, _ := c.IsStale(s, check); stale {
			t.Fatal("stale")
		}
	}
	if reused, checked := c.Stats(); reused != 3 || checked != 1 || calls != 1 {
		t.Fatalf("open, unchanged session not reused: reused=%d checked=%d calls=%d", reused, checked, calls)
	}
	// An entry saved by an earlier build, without the header, is not reused.
	entry := c.entries[s.PBPath]
	entry.Files.WALIndex = ""
	c.entries[s.PBPath] = entry
	if _, reason := c.IsStale(s, check); reason != "verified" || calls != 2 {
		t.Fatalf("entry without the header reused: reason=%s calls=%d", reason, calls)
	}
	// A commit is checked in full.
	if _, err := db.Exec("INSERT INTO steps VALUES(1)"); err != nil {
		t.Fatal(err)
	}
	if _, reason := c.IsStale(s, check); reason != "verified" || calls != 3 {
		t.Fatalf("commit not checked: reason=%s calls=%d", reason, calls)
	}
}

// A commit stalled after writing its WAL frames and before publishing them
// (agy-reader#8k4s): a check in the gap verifies the previous content under the
// new WAL size and mtime. That result is never reused. While the writer lives,
// it publishes the commit by changing only the WAL-index header; if it dies
// first, the next connection to open the database rebuilds the index from the
// WAL and makes the commit visible with no change on disk at all.
//
// SQLite cannot be paused at that point from a test, so the stall is
// reproduced with real files: the test commits a row, then writes the header
// from before the commit back into -shm. Readers then see exactly the stalled
// state, the new frames in the WAL but not yet visible. A copy of those files
// with no connection open stands for a writer that died in the gap.
//
// The stall is reproduced twice: after earlier commits, and as the first
// commit in a new WAL, when the published header has no frames and no salts.
func TestCacheNeverReusesAcrossAStalledCommit(t *testing.T) {
	for _, fresh := range []bool{false, true} {
		t.Run(map[bool]string{false: "after-earlier-commits", true: "first-commit-in-new-wal"}[fresh], func(t *testing.T) {
			stalledCommit(t, fresh)
		})
	}
}

func stalledCommit(t *testing.T, freshWAL bool) {
	s, c, db := walFixture(t)
	if freshWAL {
		// Closing the last connection checkpoints and removes the WAL; the
		// next writer starts a new one.
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		var err error
		if db, err = sql.Open("sqlite", s.PBPath); err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(1)
		t.Cleanup(func() { db.Close() })
		var n int
		if err := db.QueryRow("PRAGMA wal_autocheckpoint=0").Scan(&n); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow("SELECT count(*) FROM steps").Scan(&n); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(s.PBPath + "-wal"); err != nil || info.Size() != 0 {
			t.Fatalf("new WAL not empty: %v", err)
		}
	}
	before := shmHeader(t, s)
	if _, err := db.Exec("INSERT INTO steps VALUES(1)"); err != nil {
		t.Fatal(err)
	}
	published := shmHeader(t, s)
	if bytes.Equal(before, published) {
		t.Fatal("commit did not change the WAL-index header")
	}
	writeShmHeader(t, s, before)
	// The sidecar is verified for the content before the commit: the
	// newest visible step is 0.
	check := func(s discovery.Session) (bool, string) {
		reader, err := sql.Open("sqlite", "file:"+s.PBPath+"?mode=ro")
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		var newest int
		if err := reader.QueryRow("SELECT max(idx) FROM steps").Scan(&newest); err != nil {
			t.Fatal(err)
		}
		if newest != 0 {
			return true, "content-changed"
		}
		return false, "verified"
	}
	for range 2 {
		if stale, reason := c.IsStale(s, check); stale || reason != "verified" {
			t.Fatalf("stalled commit: stale=%v reason=%s, want the previous content checked in full", stale, reason)
		}
	}
	if len(c.entries) != 0 {
		t.Fatalf("result remembered during a stalled commit: %+v", c.entries)
	}
	// A writer that died in the gap: a fresh reader recovers the commit.
	dir := t.TempDir()
	crashed := discovery.Session{PBPath: filepath.Join(dir, "session.db"), SidecarPath: s.SidecarPath}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		raw, err := os.ReadFile(s.PBPath + suffix)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(crashed.PBPath+suffix, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if stale, reason := check(crashed); !stale {
		t.Fatalf("a fresh reader did not recover the unpublished commit: %s", reason)
	}
	// While the writer lives, publishing the commit is seen.
	writeShmHeader(t, s, published)
	if stale, reason := c.IsStale(s, check); !stale || reason != "content-changed" {
		t.Fatalf("published commit missed: stale=%v reason=%s", stale, reason)
	}
}

// testWALIndex builds both copies of a valid WAL-index header.
func testWALIndex(iChange uint32) []byte {
	h := make([]byte, walIndexHeaderSize)
	binary.NativeEndian.PutUint32(h[0:], walIndexVersion)
	binary.NativeEndian.PutUint32(h[8:], iChange)
	h[walIndexIsInit] = 1
	binary.NativeEndian.PutUint16(h[14:], 4096)
	binary.NativeEndian.PutUint32(h[16:], 3) // mxFrame
	s1, s2 := walIndexHeaderChecksum(h[:walIndexChecksum])
	binary.NativeEndian.PutUint32(h[walIndexChecksum:], s1)
	binary.NativeEndian.PutUint32(h[walIndexChecksum+4:], s2)
	return append(h, h...)
}

// testWAL builds a WAL file whose header carries salts and which holds the
// three frames testWALIndex publishes, followed by after.
func testWAL(salts string, after []byte) []byte {
	wal := make([]byte, walHeaderSize+3*(4096+walFrameHeaderSize))
	copy(wal[walHeaderSalt:walHeaderSalt+8], salts)
	return append(wal, after...)
}

// A change to the WAL-index header alone, with the database and WAL files
// untouched, forces a full check; rewriting the same header does not.
func TestCacheRechecksWhenOnlyWALIndexHeaderChanges(t *testing.T) {
	_, s, c := dummyFixture(t)
	if err := os.WriteFile(s.PBPath+"-wal", testWAL("cursalts", nil), 0600); err != nil {
		t.Fatal(err)
	}
	writeShmHeader(t, s, testWALIndex(1))
	c.IsStale(s, alwaysFresh)
	writeShmHeader(t, s, testWALIndex(1))
	if _, reason := c.IsStale(s, alwaysFresh); reason != "verified-cached" {
		t.Fatalf("unchanged header not reused: %s", reason)
	}
	writeShmHeader(t, s, testWALIndex(2))
	if _, reason := c.IsStale(s, alwaysFresh); reason != "verified" {
		t.Fatalf("changed header reused: %s", reason)
	}
}

// A frame after the published ones that carries the salts of the WAL file's
// own header is a commit not yet published, or one whose writer died before
// publishing it, so the header is not trusted. That includes the first commit
// in a new WAL, before the published header has any salts. A frame left over
// from before the WAL was restarted carries older salts and does not count.
func TestWALIndexDistrustsUnpublishedFrames(t *testing.T) {
	frame := func(salts string) []byte {
		f := make([]byte, walFrameHeaderSize)
		copy(f[8:16], salts)
		return f
	}
	// A new WAL: nothing published yet, so mxFrame, szPage and the salts in
	// the WAL-index header are zero.
	fresh := testWALIndex(1)
	for _, h := range [][]byte{fresh, fresh[walIndexHeaderSize:]} {
		binary.NativeEndian.PutUint16(h[14:], 0)
		binary.NativeEndian.PutUint32(h[16:], 0)
		s1, s2 := walIndexHeaderChecksum(h[:walIndexChecksum])
		binary.NativeEndian.PutUint32(h[walIndexChecksum:], s1)
		binary.NativeEndian.PutUint32(h[walIndexChecksum+4:], s2)
	}
	freshWAL := make([]byte, walHeaderSize)
	copy(freshWAL[walHeaderSalt:], "newsalts")
	for name, tc := range map[string]struct {
		shm, wal []byte
		trusted  bool
	}{
		"no-frame-after": {testWALIndex(1), testWAL("cursalts", nil), true},
		"leftover-frame": {testWALIndex(1), testWAL("cursalts", frame("oldsalts")), true},
		"current-frame":  {testWALIndex(1), testWAL("cursalts", frame("cursalts")), false},
		"partial-frame":  {testWALIndex(1), testWAL("cursalts", frame("cursalts")[:10]), false},
		"partial-header": {testWALIndex(1), make([]byte, 10), false},
		"empty-new-wal":  {fresh, nil, true},
		"first-commit":   {fresh, append(freshWAL, frame("newsalts")...), false},
	} {
		t.Run(name, func(t *testing.T) {
			_, s, _ := dummyFixture(t)
			writeShmHeader(t, s, tc.shm)
			if err := os.WriteFile(s.PBPath+"-wal", tc.wal, 0600); err != nil {
				t.Fatal(err)
			}
			if files, _ := Fingerprint(s); (files.WALIndex != "") != tc.trusted {
				t.Fatalf("header trusted=%v, want %v", files.WALIndex != "", tc.trusted)
			}
		})
	}
}

// Without a WAL-index header to trust, the cache falls back to reusing a
// result only while the WAL and journal are empty.
func TestCacheFallsBackWithoutReadableWALIndex(t *testing.T) {
	torn := testWALIndex(1)
	copy(torn[walIndexHeaderSize:], testWALIndex(2))
	badChecksum := testWALIndex(1)
	badChecksum[walIndexChecksum]++
	badChecksum[walIndexHeaderSize+walIndexChecksum]++
	uninitialized := testWALIndex(1)
	uninitialized[walIndexIsInit], uninitialized[walIndexHeaderSize+walIndexIsInit] = 0, 0
	for name, shm := range map[string][]byte{
		"no-shm":          nil,
		"short-shm":       testWALIndex(1)[:walIndexHeaderSize+8],
		"copies-differ":   torn,
		"bad-checksum":    badChecksum,
		"not-initialized": uninitialized,
	} {
		t.Run(name, func(t *testing.T) {
			_, s, c := dummyFixture(t)
			if shm != nil {
				writeShmHeader(t, s, shm)
			}
			if files, _ := Fingerprint(s); files.WALIndex != "" {
				t.Fatalf("header trusted: %s", files.WALIndex)
			}
			// An empty WAL is still reused.
			if err := os.WriteFile(s.PBPath+"-wal", nil, 0600); err != nil {
				t.Fatal(err)
			}
			c.IsStale(s, alwaysFresh)
			if _, reason := c.IsStale(s, alwaysFresh); reason != "verified-cached" {
				t.Fatalf("empty WAL not reused: %s", reason)
			}
			// A WAL with frames is checked on every lookup.
			if err := os.WriteFile(s.PBPath+"-wal", []byte("frames"), 0600); err != nil {
				t.Fatal(err)
			}
			c.ResetStats()
			for range 3 {
				c.IsStale(s, alwaysFresh)
			}
			if reused, checked := c.Stats(); reused != 0 || checked != 3 || len(c.entries) != 0 {
				t.Fatalf("WAL frames reused without a header: reused=%d checked=%d entries=%d", reused, checked, len(c.entries))
			}
		})
	}
}

// The same holds for a rollback journal with content: a check during a
// rollback-mode commit is not remembered.
func TestCacheRechecksWhileJournalHasContent(t *testing.T) {
	_, s, c := dummyFixture(t)
	if err := os.WriteFile(s.PBPath+"-journal", []byte("journal content"), 0600); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		c.IsStale(s, alwaysFresh)
	}
	if reused, checked := c.Stats(); reused != 0 || checked != 3 || len(c.entries) != 0 {
		t.Fatalf("result reused while the journal has content: reused=%d checked=%d entries=%d", reused, checked, len(c.entries))
	}
}
