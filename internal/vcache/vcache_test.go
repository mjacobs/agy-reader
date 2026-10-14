//go:build linux

package vcache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/discovery"
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

func alwaysStale(discovery.Session) (bool, string) {
	return true, "stale"
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
