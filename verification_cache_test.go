//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/testfixture"
)

func verifiedFixture(t *testing.T) (string, discovery.Session, *verificationCache) {
	t.Helper()
	root, s, _ := completeFixture(t)
	srv := completeServer(t, completePayload("current"), nil)
	traj, err := fetchSession(t.Context(), daemon.NewClient(srv.URL), s)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.WriteVerified(s.SidecarPath, traj, s.ModTime); err != nil {
		t.Fatal(err)
	}
	return root, s, openVerificationCache(root, t.TempDir(), "test-boot")
}

func TestVerificationCacheRestartAndIndependentCheck(t *testing.T) {
	root, s, c := verifiedFixture(t)
	if stale, reason := c.isStale(s); stale {
		t.Fatal(reason)
	}
	if c.checked != 1 || len(c.entries) != 1 {
		t.Fatalf("not cached: %+v", c)
	}
	if err := c.save(); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(c.path)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("cache permissions: %v", info.Mode())
	}
	restarted := openVerificationCache(root, filepath.Dir(filepath.Dir(c.path)), "test-boot")
	if stale, reason := restarted.isStale(s); stale || reason != "verified-cached" || restarted.checked != 0 {
		t.Fatalf("restart missed: %v %s", stale, reason)
	}
	if stale, reason := isStale(s); stale || reason != "verified" {
		t.Fatalf("independent check: %v %s", stale, reason)
	}
	// A hit must not keep extending the full-verification deadline.
	if !restarted.entries[s.PBPath].CheckedAt.Equal(c.entries[s.PBPath].CheckedAt) {
		t.Fatal("hit renewed verification")
	}
}

func TestVerificationCacheInvalidation(t *testing.T) {
	for _, action := range []string{"same-size-preserved-mtime", "atomic-replacement", "missing-sidecar", "expiry", "clock-backwards"} {
		t.Run(action, func(t *testing.T) {
			_, s, c := verifiedFixture(t)
			c.isStale(s)
			info, _ := os.Stat(s.SidecarPath)
			switch action {
			case "same-size-preserved-mtime", "atomic-replacement":
				raw, _ := os.ReadFile(s.SidecarPath)
				raw = bytes.ReplaceAll(raw, []byte("current"), []byte("altered"))
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
				c.now = func() time.Time { return now }
			case "clock-backwards":
				now := c.entries[s.PBPath].CheckedAt.Add(-time.Second)
				c.now = func() time.Time { return now }
			}
			stale, _ := c.isStale(s)
			if c.reused != 0 || c.checked != 2 {
				t.Fatalf("missed invalidation: reused=%d checked=%d", c.reused, c.checked)
			}
			if action != "expiry" && action != "clock-backwards" && !stale {
				t.Fatal("changed sidecar accepted")
			}
		})
	}
}

func TestVerificationCacheWALOnlyCommit(t *testing.T) {
	root, s, db := completeFixture(t)
	testfixture.Exec(t, db, "PRAGMA journal_mode=WAL")
	testfixture.Exec(t, db, "PRAGMA wal_autocheckpoint=0")
	srv := completeServer(t, completePayload("current"), nil)
	traj, err := fetchSession(t.Context(), daemon.NewClient(srv.URL), s)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.WriteVerified(s.SidecarPath, traj, s.ModTime); err != nil {
		t.Fatal(err)
	}
	c := openVerificationCache(root, t.TempDir(), "test-boot")
	if stale, reason := c.isStale(s); stale {
		t.Fatal(reason)
	}
	before, _ := os.Stat(s.PBPath)
	replaceCompleteStep(t, db, "changed")
	after, _ := os.Stat(s.PBPath)
	if !before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() {
		t.Fatal("test did not isolate a WAL-only write")
	}
	if stale, _ := c.isStale(s); !stale || c.reused != 0 {
		t.Fatal("WAL update reused old verification")
	}
	// Failed checks remain retryable; they never become successful memo entries.
	c.isStale(s)
	if c.checked != 3 || len(c.entries) != 0 {
		t.Fatal("incomplete result cached")
	}
}

func TestVerificationCacheDiskFailuresAndCompatibility(t *testing.T) {
	for _, action := range []string{"corrupt", "version", "profile", "boot", "root", "unwritable"} {
		t.Run(action, func(t *testing.T) {
			root, s, c := verifiedFixture(t)
			c.isStale(s)
			if err := c.save(); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Dir(filepath.Dir(c.path))
			if action == "unwritable" {
				c.path = filepath.Join(s.PBPath, "cannot-create.json")
				c.dirty = true
				if c.save() == nil {
					t.Fatal("expected save failure")
				}
				if stale, reason := c.isStale(s); stale || reason != "verified-cached" {
					t.Fatal("save failure lost memory cache")
				}
				return
			}
			raw, _ := os.ReadFile(c.path)
			var disk verificationDiskCache
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
			next := openVerificationCache(root, dir, "test-boot")
			if stale, reason := next.isStale(s); stale || reason == "verified-cached" || next.checked != 1 {
				t.Fatalf("bad cache reused: %v %s", stale, reason)
			}
		})
	}
}

func TestWatchVerificationCacheAndPruning(t *testing.T) {
	root, s, c := verifiedFixture(t)
	d := newPayloadDaemon(t, completePayload("current"))
	var logs bytes.Buffer
	w := heldWatcher(t, root, d.URL, &logs)
	w.verification = c
	w.tick()
	if c.checked != 1 {
		t.Fatal("first watch did not verify")
	}
	w.tick()
	if c.reused != 1 || c.checked != 0 || !bytes.Contains(logs.Bytes(), []byte("verification: 1 reused, 0 checked")) {
		t.Fatalf("watch did not reuse: %s", logs.String())
	}
	if err := os.Remove(s.PBPath); err != nil {
		t.Fatal(err)
	}
	w.tick()
	if len(c.entries) != 0 {
		t.Fatal("deleted source retained")
	}
}

func TestVerificationCacheDoesNotRememberUnstableCheck(t *testing.T) {
	_, s, c := verifiedFixture(t)
	// Change metadata between the pre-check fingerprint and full verification,
	// keeping the payload valid. The result can be used now but not memoized.
	c.now = func() time.Time {
		info, err := os.Stat(s.SidecarPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(s.SidecarPath, info.ModTime(), info.ModTime().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		return time.Now()
	}
	if stale, reason := c.isStale(s); stale {
		t.Fatal(reason)
	}
	if len(c.entries) != 0 {
		t.Fatal("unstable check was cached")
	}
}
