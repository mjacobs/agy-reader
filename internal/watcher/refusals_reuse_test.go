package watcher

import (
	"bytes"
	"os"
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
