package watcher

import (
	"bytes"
	"errors"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/testfixture"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/vcache"
)

// A held unsupported source follows the verification cache's reuse rule
// (agy-reader#5kdz): once its digest has matched, unchanged file metadata
// stands in for reading it again for four to five minutes if the source had
// just changed, a quarter of its quiet time if it had been quiet for an hour,
// and up to the cap, below six hours, if it had been quiet for a day. A change
// the fingerprint sees still reads the digest again on the next tick.
func TestWatchHeldDigestReuseGrowsWithQuietTime(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("source file metadata is only compared on Linux")
	}
	for _, tc := range []struct {
		name   string
		quiet  time.Duration
		limit  func(path string) time.Duration
		change bool
	}{
		{"changed-recently", 0, vcache.Lifetime, false},
		{"quiet-for-an-hour", time.Hour, func(string) time.Duration { return 15 * time.Minute }, false},
		{"quiet-for-a-day", 24 * time.Hour, vcache.QuietLifetime, false},
		{"quiet-for-a-day-then-chmod", 24 * time.Hour, vcache.QuietLifetime, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clock := time.Now()
			refusalNow = func() time.Time { return clock }
			t.Cleanup(func() { refusalNow = time.Now })

			root, s, _ := unsupportedProjectionFixture(t)
			d := newPayloadDaemon(t, unsupportedProjectionPayload("first"))
			w := heldWatcher(t, root, d.URL, new(bytes.Buffer))
			w.Tick() // fetched, refused and held
			files, ok := vcache.SourceFingerprint(s.PBPath)
			if !ok {
				t.Fatal("no source fingerprint")
			}
			clock = time.Unix(0, max(files.Source.Mtime, files.Source.Ctime)).Add(tc.quiet)
			w.Tick() // digest read and matched
			held := w.Held[s.PBPath]
			matched := held.DigestMatchedAt
			if !matched.Equal(clock) {
				t.Fatalf("matched digest not remembered at the tick's time: %+v", held)
			}
			limit := tc.limit(s.PBPath)
			if got := vcache.ReuseLimit(s.PBPath, held.SourceFiles, matched); got != limit || limit >= vcache.MaxLifetime {
				t.Fatalf("ReuseLimit = %v, want %v below %v", got, limit, vcache.MaxLifetime)
			}
			// Reading the digest now would release the hold, which refetches.
			held.SourceDigest = "not the source's digest"
			w.Held[s.PBPath] = held
			fetched := d.calls.Load()

			if tc.change {
				info, err := os.Stat(s.PBPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(s.PBPath, info.Mode().Perm()^0o040); err != nil {
					t.Fatal(err)
				}
				clock = matched.Add(time.Minute)
				w.Tick()
				if d.calls.Load() == fetched {
					t.Fatal("metadata change on a quiet held source did not read the digest again")
				}
				return
			}
			clock = matched.Add(limit - time.Second)
			w.Tick()
			if d.calls.Load() != fetched {
				t.Fatalf("digest read again %v after the match, within the %v limit", limit-time.Second, limit)
			}
			clock = matched.Add(limit)
			w.Tick()
			if d.calls.Load() == fetched {
				t.Fatalf("digest not read again %v after the match, at the limit", limit)
			}
		})
	}
}

func TestLegacyRefusalReuseAndRepair(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("file metadata reuse requires Linux")
	}
	for _, action := range []string{"expiry", "repair"} {
		t.Run(action, func(t *testing.T) {
			clock := time.Now()
			refusalNow = func() time.Time { return clock }
			t.Cleanup(func() { refusalNow = time.Now })
			path := filepath.Join(t.TempDir(), "legacy.pb")
			if err := os.WriteFile(path, []byte("invalid encrypted source"), 0600); err != nil {
				t.Fatal(err)
			}
			info, _ := os.Stat(path)
			s := discovery.Session{PBPath: path, SidecarPath: path + ".json", ModTime: info.ModTime()}
			m := RefusalMemo{}
			m.Hold(s, "endpoint", "")
			if !m.Holds(s, "endpoint") {
				t.Fatal("invalid legacy not held")
			}
			held := m[path]
			if held.DigestMatchedAt.IsZero() {
				t.Fatal("legacy refusal did not remember stable check")
			}
			// If read again, the true empty digest would release this sentinel hold.
			held.SourceDigest = "sentinel"
			m[path] = held
			clock = clock.Add(time.Minute)
			if !m.Holds(s, "endpoint") {
				t.Fatal("unchanged legacy refusal was read again")
			}
			if action == "expiry" {
				clock = clock.Add(vcache.MaxLifetime)
			} else {
				testfixture.LegacySource(t, path, []byte(`{"cascadeId":"11111111-1111-4111-8111-111111111111"}`))
				if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			}
			if m.Holds(s, "endpoint") {
				t.Fatal("legacy refusal survived expiry or repaired source")
			}
		})
	}
}

func TestLegacyTransientRefusalIsNotReused(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("file metadata reuse requires Linux")
	}
	path := filepath.Join(t.TempDir(), "legacy.pb")
	testfixture.LegacySource(t, path, []byte(`{"cascadeId":"11111111-1111-4111-8111-111111111111"}`))
	info, _ := os.Stat(path)
	s := discovery.Session{PBPath: path, SidecarPath: path + ".json", ModTime: info.ModTime()}
	m := RefusalMemo{}
	m.Hold(s, "endpoint", "")
	for range 2 {
		if !m.holds(s, "endpoint", func(string) (string, error) { return "", errors.New("temporary descriptor exhaustion") }) {
			t.Fatal("temporary failure should stay quiet while retried")
		}
		if !m[path].DigestMatchedAt.IsZero() {
			t.Fatal("transient error reused from metadata")
		}
	}
	// Recovery is discovered by the next tick, without metadata changing.
	if m.Holds(s, "endpoint") {
		t.Fatal("readable source still held")
	}

}
