package vcache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/discovery"
)

const Version = 1

// Cache is a disposable watch optimization, not a completeness attestation.
// Doctor and one-shot reads always use the content verifier. Only successful
// checks are remembered, and metadata reuse expires within five minutes.
type Cache struct {
	path            string
	boot            string
	root            string
	entries         map[string]Entry
	dirty           bool
	now             func() time.Time
	reused, checked int
}

type File struct {
	Exists             bool
	Device, Inode      uint64
	Size, Mtime, Ctime int64
}

type Files struct {
	Source, WAL, Journal, Sidecar File
}

type Entry struct {
	Files     Files
	CheckedAt time.Time
}

type DiskCache struct {
	Version             int
	Profile, Boot, Root string
	Entries             map[string]Entry
}

func New(root string) *Cache {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = ""
	}
	return Open(root, dir, BootID())
}

func Open(root, dir, boot string) *Cache {
	root, err := filepath.Abs(root)
	c := &Cache{root: root, boot: boot, entries: make(map[string]Entry), now: time.Now}
	if err != nil || dir == "" || boot == "" {
		return c
	}
	key := sha256.Sum256([]byte(root))
	c.path = filepath.Join(dir, "agy-reader", fmt.Sprintf("verification-%x.json", key))
	f, err := os.Open(c.path)
	if err != nil {
		return c
	}
	defer f.Close()
	var disk DiskCache
	// A damaged or unexpectedly large cache is a miss, never a watch failure.
	dec := json.NewDecoder(io.LimitReader(f, 16<<20))
	if dec.Decode(&disk) != nil || dec.Decode(new(any)) != io.EOF || disk.Version != Version || disk.Profile != completeness.Profile || disk.Boot != boot || disk.Root != root {
		return c
	}
	if disk.Entries != nil {
		c.entries = disk.Entries
	}
	return c
}

// SetNow overrides the current time function (useful in tests).
func (c *Cache) SetNow(now func() time.Time) {
	if c != nil {
		c.now = now
	}
}

// Stats returns the number of reused and checked entries.
func (c *Cache) Stats() (reused, checked int) {
	if c == nil {
		return 0, 0
	}
	return c.reused, c.checked
}

// ResetStats resets the counters for reused and checked entries.
func (c *Cache) ResetStats() {
	if c != nil {
		c.reused, c.checked = 0, 0
	}
}

// Lifetime is spread over minute four to five, deterministically per source, so
// the first scan does not create a synchronized five-minute full rescan.
func Lifetime(path string) time.Duration {
	h := sha256.Sum256([]byte(path))
	return 4*time.Minute + time.Duration(uint16(h[0])<<8|uint16(h[1]))*time.Minute/65536
}

func Fingerprint(s discovery.Session) (Files, bool) {
	var files Files
	for _, item := range []struct {
		path     string
		out      *File
		required bool
	}{
		{s.PBPath, &files.Source, true}, {s.PBPath + "-wal", &files.WAL, false},
		{s.PBPath + "-journal", &files.Journal, false}, {s.SidecarPath, &files.Sidecar, true},
	} {
		info, err := os.Stat(item.path)
		if os.IsNotExist(err) && !item.required {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return Files{}, false
		}
		state, ok := verificationStat(info)
		if !ok {
			return Files{}, false
		}
		*item.out = state
	}
	return files, true
}

func (c *Cache) IsStale(s discovery.Session, checkStale func(discovery.Session) (bool, string)) (bool, string) {
	if c == nil || !strings.HasSuffix(s.PBPath, ".db") {
		return checkStale(s)
	}
	before, ok := Fingerprint(s)
	now := c.now()
	if entry, found := c.entries[s.PBPath]; found {
		age := now.Sub(entry.CheckedAt)
		if ok && entry.Files == before && !entry.CheckedAt.IsZero() && age >= 0 && age < Lifetime(s.PBPath) {
			c.reused++
			return false, "verified-cached"
		}
		delete(c.entries, s.PBPath)
		c.dirty = true
	}
	c.checked++
	stale, reason := checkStale(s)
	// Never remember an incomplete/unknown result, nor pair a result with file
	// metadata captured after a concurrent change. Rewrites after this check
	// invalidate the next lookup, including atomic sidecar replacement.
	if !stale && ok {
		after, stable := Fingerprint(s)
		if stable && before == after {
			c.entries[s.PBPath] = Entry{Files: after, CheckedAt: now}
			c.dirty = true
		}
	}
	return stale, reason
}

func (c *Cache) Prune(sessions []discovery.Session) {
	if c == nil {
		return
	}
	present := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		present[s.PBPath] = true
	}
	for path := range c.entries {
		if !present[path] {
			delete(c.entries, path)
			c.dirty = true
		}
	}
}

// Save once per changed pass. Concurrent watchers may overwrite each other's
// optimization entries; atomic replacement keeps readers safe, and a lost entry
// only costs another full verification. No transcript or credential is stored.
func (c *Cache) Save() error {
	if c == nil || !c.dirty || c.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(c.path), ".verification-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	disk := DiskCache{Version: Version, Profile: completeness.Profile, Boot: c.boot, Root: c.root, Entries: c.entries}
	if err := json.NewEncoder(f).Encode(disk); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), c.path); err != nil {
		return err
	}
	c.dirty = false
	return nil
}
