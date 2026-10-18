//go:build linux

package vcache

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/discovery"
)

// lookupAt runs one cache lookup with the clock at now and reports whether the
// remembered result was reused instead of checked in full.
func lookupAt(t *testing.T, c *Cache, s discovery.Session, now time.Time) bool {
	t.Helper()
	c.SetNow(func() time.Time { return now })
	stale, reason := c.IsStale(s, alwaysFresh)
	if stale {
		t.Fatalf("stale at %v: %s", now, reason)
	}
	return reason == "verified-cached"
}

// changedAtOf is when the fixture's files last changed, as the cache sees it.
func changedAtOf(t *testing.T, s discovery.Session) time.Time {
	t.Helper()
	files, ok := Fingerprint(s)
	if !ok {
		t.Fatal("no fingerprint")
	}
	changed, ok := files.changedAt()
	if !ok {
		t.Fatalf("no change time: %+v", files)
	}
	return changed
}

// How long a checked result is reused depends on how long the session had
// been quiet when it was checked (agy-reader#5kdz). A session that changed
// recently keeps the four-to-five-minute limit. One quiet for an hour is
// reused for a quarter of that, 15 minutes. One quiet for a day or a month
// reaches the cap, which stays below six hours.
func TestReuseGrowsWithQuietTime(t *testing.T) {
	for _, tc := range []struct {
		name  string
		quiet time.Duration
		want  func(path string) time.Duration
	}{
		{"changed-recently", 0, Lifetime},
		{"changed-ten-minutes-ago", 10 * time.Minute, Lifetime},
		{"quiet-for-an-hour", time.Hour, func(string) time.Duration { return 15 * time.Minute }},
		{"quiet-for-a-day", 24 * time.Hour, QuietLifetime},
		{"quiet-for-a-month", 30 * 24 * time.Hour, QuietLifetime},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, s, c := dummyFixture(t)
			checkedAt := changedAtOf(t, s).Add(tc.quiet)
			if lookupAt(t, c, s, checkedAt) {
				t.Fatal("first lookup reused")
			}
			limit := tc.want(s.PBPath)
			if limit > MaxLifetime {
				t.Fatalf("expected limit %v exceeds %v", limit, MaxLifetime)
			}
			if got := ReuseLimit(s.PBPath, c.entries[s.PBPath].Files, checkedAt); got != limit {
				t.Fatalf("ReuseLimit = %v, want %v", got, limit)
			}
			if !lookupAt(t, c, s, checkedAt.Add(limit-time.Second)) {
				t.Fatalf("not reused %v after the check, within the %v limit", limit-time.Second, limit)
			}
			if lookupAt(t, c, s, checkedAt.Add(limit)) {
				t.Fatalf("still reused %v after the check, at the limit", limit)
			}
		})
	}
}

// The short limit is spread over minute four to five and the cap over hour
// five to six, per source, and no limit reaches six hours however long the
// session has been quiet.
func TestReuseLimitBounds(t *testing.T) {
	changed := time.Unix(1_700_000_000, 0)
	files := Files{Source: File{Exists: true, Size: 1, Mtime: changed.UnixNano(), Ctime: changed.UnixNano()}}
	for i := range 2000 {
		path := fmt.Sprintf("/store/conversations/%04d.db", i)
		if l := Lifetime(path); l < 4*time.Minute || l >= 5*time.Minute {
			t.Fatalf("Lifetime(%s) = %v", path, l)
		}
		if q := QuietLifetime(path); q < 5*time.Hour || q >= MaxLifetime {
			t.Fatalf("QuietLifetime(%s) = %v", path, q)
		}
		for _, quiet := range []time.Duration{-time.Hour, 0, time.Minute, time.Hour, 20 * time.Hour, 100 * 365 * 24 * time.Hour} {
			l := ReuseLimit(path, files, changed.Add(quiet))
			if l < Lifetime(path) || l >= MaxLifetime {
				t.Fatalf("ReuseLimit(%s, quiet %v) = %v", path, quiet, l)
			}
		}
	}
}

// Quiet time counts from the later of each file's mtime and ctime. Setting
// the mtimes back two days, which anyone can do, leaves the ctime at the
// moment of the change, so the session still counts as just changed.
func TestReuseIgnoresMtimeSetBack(t *testing.T) {
	_, s, c := dummyFixture(t)
	old := time.Now().Add(-48 * time.Hour)
	for _, path := range []string{s.PBPath, s.SidecarPath} {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	lookupAt(t, c, s, now)
	if lookupAt(t, c, s, now.Add(Lifetime(s.PBPath))) {
		t.Fatal("mtimes set back lengthened reuse past the short limit")
	}
}

// A WAL with content counts towards quiet time: the session changed when its
// last frame was written, whatever the database file's own times say.
func TestReuseCountsWALWithContent(t *testing.T) {
	_, s, _ := dummyFixture(t)
	files, ok := Fingerprint(s)
	if !ok {
		t.Fatal("no fingerprint")
	}
	walChanged := time.Unix(0, files.Source.Ctime).Add(30 * time.Hour)
	checkedAt := walChanged.Add(time.Minute)
	files.WAL = File{Exists: true, Size: 8272, Mtime: walChanged.UnixNano(), Ctime: walChanged.UnixNano()}
	if got := ReuseLimit(s.PBPath, files, checkedAt); got != Lifetime(s.PBPath) {
		t.Fatalf("WAL written a minute before the check: limit %v, want %v", got, Lifetime(s.PBPath))
	}
	files.WAL.Size = 0
	if got, want := ReuseLimit(s.PBPath, files, checkedAt), QuietLifetime(s.PBPath); got != want {
		t.Fatalf("empty WAL counted: limit %v, want %v", got, want)
	}
}

// A long-quiet session that is reused for hours is still checked in full at
// once when anything the fingerprint sees changes (agy-reader#8k4s): a
// metadata-only change to the source or the sidecar, or a new WAL-index header.
func TestQuietSessionRechecksVisibleChangeAtOnce(t *testing.T) {
	for _, change := range []string{"none", "source-chmod", "sidecar-rewrite", "wal-index-header"} {
		t.Run(change, func(t *testing.T) {
			_, s, c := dummyFixture(t)
			if err := os.WriteFile(s.PBPath+"-wal", testWAL("cursalts", nil), 0600); err != nil {
				t.Fatal(err)
			}
			writeShmHeader(t, s, testWALIndex(1))
			checkedAt := changedAtOf(t, s).Add(24 * time.Hour)
			lookupAt(t, c, s, checkedAt)
			if !lookupAt(t, c, s, checkedAt.Add(time.Hour)) {
				t.Fatal("quiet session not reused an hour after its check")
			}
			switch change {
			case "source-chmod":
				if err := os.Chmod(s.PBPath, 0o640); err != nil {
					t.Fatal(err)
				}
			case "sidecar-rewrite":
				info, err := os.Stat(s.SidecarPath)
				if err != nil {
					t.Fatal(err)
				}
				raw, err := os.ReadFile(s.SidecarPath)
				if err != nil {
					t.Fatal(err)
				}
				copy(raw[len(raw)-5:], "54321")
				if err := os.WriteFile(s.SidecarPath, raw, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(s.SidecarPath, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			case "wal-index-header":
				writeShmHeader(t, s, testWALIndex(2))
			}
			reused := lookupAt(t, c, s, checkedAt.Add(time.Hour+time.Minute))
			if want := change == "none"; reused != want {
				t.Fatalf("reused after %s = %v, want %v", change, reused, want)
			}
		})
	}
}
