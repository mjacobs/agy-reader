package vcache

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
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

const Version = 2

// Bind persisted reuse to both native verification policies.
const cacheProfile = completeness.Profile + "\n" + completeness.LegacyProfile

// Cache is a disposable watch optimization, not a completeness attestation.
// Doctor and one-shot reads always use the content verifier. Only successful
// checks are remembered, and metadata reuse expires (ReuseLimit): within five
// minutes for a session that changed recently, and always within six hours.
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
	// WALIndex is the WAL-index header SQLite publishes in the source's -shm
	// file, hex-encoded, or "" when it could not be read consistently.
	WALIndex string
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
	if dec.Decode(&disk) != nil || dec.Decode(new(any)) != io.EOF || disk.Version != Version || disk.Profile != cacheProfile || disk.Boot != boot || disk.Root != root {
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

// Lifetime is the reuse limit for a source that changed recently; ReuseLimit
// lengthens it for one that has been quiet, up to QuietLifetime. It is spread
// over minute four to five, deterministically per source, so the first scan
// does not create a synchronized five-minute full rescan.
func Lifetime(path string) time.Duration {
	return 4*time.Minute + stagger(path, time.Minute)
}

// MaxLifetime is the longest a checked result is ever reused (agy-reader#5kdz).
const MaxLifetime = 6 * time.Hour

// QuietLifetime is the cap on reuse for a long-quiet source, spread over hour
// five to six like Lifetime, so sessions first checked together are not all
// checked again in the same minute. It is always below MaxLifetime.
func QuietLifetime(path string) time.Duration {
	return MaxLifetime - time.Hour + stagger(path, time.Hour)
}

// stagger returns a fixed fraction of span, below span, chosen by path.
func stagger(path string, span time.Duration) time.Duration {
	h := sha256.Sum256([]byte(path))
	return time.Duration(uint16(h[0])<<8|uint16(h[1])) * span / 65536
}

// ReuseLimit is how long a check made at checkedAt, of a source whose
// fingerprint was files, may stand in for another while the fingerprint stays
// the same. A session that changed recently keeps Lifetime, four to five
// minutes. One that had been quiet for longer when it was checked may be
// reused for a quarter of that quiet time, up to QuietLifetime: quiet for an
// hour, 15 minutes; quiet for a day or more, the cap. Every hole found so far
// (agy-reader#8k4s, agy-reader#q0mh) lay in a session that had just changed,
// and those keep the short limit, while long-quiet sessions, nearly all of an
// idle watcher's cost, are no longer checked in full every few minutes.
//
// The limit only bounds how long a change the fingerprint cannot see goes
// unnoticed: any change it can see (file metadata, the WAL-index header)
// already forces a full check at once.
func ReuseLimit(path string, files Files, checkedAt time.Time) time.Duration {
	limit := Lifetime(path)
	changed, ok := files.changedAt()
	if !ok {
		return limit
	}
	if grown := min(checkedAt.Sub(changed)/4, QuietLifetime(path)); grown > limit {
		limit = grown
	}
	return limit
}

// changedAt returns when the files holding the session's content last changed:
// the source, a WAL or rollback journal with content, and the sidecar. Each
// counts the later of its mtime and ctime. Only the kernel sets ctime, to the
// current time, so a timestamp set back with utimes cannot lengthen reuse. An
// empty WAL or journal holds no content; SQLite creates and truncates them as
// connections open, checkpoint and close. ok is false when the source has no
// timestamps, which leaves the short limit.
func (f Files) changedAt() (time.Time, bool) {
	if !f.Source.Exists || f.Source.Ctime == 0 {
		return time.Time{}, false
	}
	latest := max(f.Source.Mtime, f.Source.Ctime)
	for _, file := range []File{f.WAL, f.Journal} {
		if file.Exists && file.Size > 0 {
			latest = max(latest, file.Mtime, file.Ctime)
		}
	}
	if f.Sidecar.Exists {
		latest = max(latest, f.Sidecar.Mtime, f.Sidecar.Ctime)
	}
	return time.Unix(0, latest), true
}

func Fingerprint(s discovery.Session) (Files, bool) {
	files, ok := SourceFingerprint(s.PBPath)
	if !ok {
		return Files{}, false
	}
	if files.Sidecar, ok = statFile(s.SidecarPath, true); !ok {
		return Files{}, false
	}
	return files, true
}

// SourceFingerprint is Fingerprint for the source alone: Sidecar stays the
// zero File, so a source that has no sidecar still has a fingerprint.
func SourceFingerprint(path string) (Files, bool) {
	var files Files
	if filepath.Ext(path) == ".pb" {
		// A newly created SQLite source supersedes the legacy file even when
		// the encrypted bytes and its sidecar have not changed.
		if _, err := os.Stat(strings.TrimSuffix(path, ".pb") + ".db"); !os.IsNotExist(err) {
			return files, false
		}
		var ok bool
		files.Source, ok = statFile(path, true)
		return files, ok
	}
	for _, item := range []struct {
		path     string
		out      *File
		required bool
	}{
		{path, &files.Source, true}, {path + "-wal", &files.WAL, false}, {path + "-journal", &files.Journal, false},
	} {
		state, ok := statFile(item.path, item.required)
		if !ok {
			return Files{}, false
		}
		*item.out = state
	}
	files.WALIndex = walIndexHeader(path)
	return files, true
}

// The WAL-index header at the start of -shm, as SQLite documents it
// (https://www.sqlite.org/walformat.html, "The WAL-Index Header") and writes it
// (walIndexWriteHdr in src/wal.c): two copies of a 48-byte header at offsets 0
// and 48, then checkpoint information and read marks from offset 96. Only a
// connection holding the write lock rewrites the copies, second copy first:
// to publish a commit (which also increments iChange), to restart the WAL, or
// to rebuild the index during recovery when the database is first opened.
// Readers write only the read marks after offset 96. Fields are in the host's
// byte order.
const (
	walIndexHeaderSize = 48
	walIndexVersion    = 3007000 // iVersion, bytes 0..3
	walIndexIsInit     = 12      // isInit, 1 once the index is initialized
	walIndexChecksum   = 40      // aCksum, two words over bytes 0..39
	walIndexPageSize   = 14      // szPage, or 1 for 65536
	walIndexMxFrame    = 16      // mxFrame, the number of published frames

	walHeaderSize      = 32 // WAL_HDRSIZE
	walHeaderSalt      = 16 // the WAL file header's two salts, bytes 16..23
	walFrameHeaderSize = 24 // WAL_FRAME_HDRSIZE; salts at bytes 8..15
)

// walIndexHeader returns the published WAL-index header of the database at
// path, or "" when there is none to trust:
//   - no -shm file (exclusive locking mode, or no WAL), an unreadable or short
//     one, or an uninitialized or unknown header;
//   - a read torn by a concurrent publish: the copies differ or the checksum
//     fails;
//   - a frame of the current WAL after the published ones. That is a commit
//     still being written or never published, for example because its writer
//     stopped before publishing it. The next connection to open the database
//     after the writer is gone rebuilds the index from the WAL and can make
//     such a commit visible without this header changing first.
//
// The files are opened and closed with plain file calls. Closing any
// descriptor releases every POSIX lock this process holds on the file, so this
// must not run while this process has the database open through SQLite; the
// watcher only fingerprints a source around a check, never during one.
func walIndexHeader(path string) string {
	f, err := os.Open(path + "-shm")
	if err != nil {
		return ""
	}
	defer f.Close()
	var buf [2 * walIndexHeaderSize]byte
	// SQLite's own readers retry a torn read; a few tries cover a publish
	// that was in progress.
	for range 3 {
		if _, err := f.ReadAt(buf[:], 0); err != nil {
			return ""
		}
		header, second := buf[:walIndexHeaderSize], buf[walIndexHeaderSize:]
		if !bytes.Equal(header, second) {
			continue
		}
		if !validWALIndexHeader(header) || unpublishedFrame(path+"-wal", header) {
			return ""
		}
		return hex.EncodeToString(header)
	}
	return ""
}

// unpublishedFrame reports whether the WAL holds a frame after the published
// ones that belongs to the current WAL: one whose salts match the salts in the
// WAL file's own header, the test SQLite applies when it rebuilds the index
// (walIndexRecover and walDecodeFrame in src/wal.c). The published header's
// salts are not used: before the first commit in a new WAL is published, they
// are still zero. A frame left over from before the WAL was restarted carries
// older salts and does not count. The test is conservative: frames that a
// rolled-back transaction spilled also count until the next commit overwrites
// them, and a partly written header or frame header counts too.
func unpublishedFrame(walPath string, header []byte) bool {
	f, err := os.Open(walPath)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return true
	}
	defer f.Close()
	var walHeader [walHeaderSize]byte
	switch n, err := f.ReadAt(walHeader[:], 0); {
	case n == 0 && err == io.EOF:
		return false
	case n < len(walHeader):
		return true
	}
	pageSize := int64(binary.NativeEndian.Uint16(header[walIndexPageSize:]))
	if pageSize == 1 {
		pageSize = 65536
	}
	offset := walHeaderSize + int64(binary.NativeEndian.Uint32(header[walIndexMxFrame:]))*(pageSize+walFrameHeaderSize)
	var frame [walFrameHeaderSize]byte
	switch n, err := f.ReadAt(frame[:], offset); {
	case n == 0 && err == io.EOF:
		return false
	case n < len(frame):
		return true
	}
	return bytes.Equal(frame[8:16], walHeader[walHeaderSalt:walHeaderSalt+8])
}

func validWALIndexHeader(h []byte) bool {
	if binary.NativeEndian.Uint32(h[0:]) != walIndexVersion || h[walIndexIsInit] != 1 {
		return false
	}
	s1, s2 := walIndexHeaderChecksum(h[:walIndexChecksum])
	return s1 == binary.NativeEndian.Uint32(h[walIndexChecksum:]) && s2 == binary.NativeEndian.Uint32(h[walIndexChecksum+4:])
}

// walIndexHeaderChecksum is walChecksumBytes in src/wal.c for native byte
// order with no initial value, over a length that is a multiple of 8.
func walIndexHeaderChecksum(b []byte) (s1, s2 uint32) {
	for i := 0; i+8 <= len(b); i += 8 {
		s1 += binary.NativeEndian.Uint32(b[i:]) + s2
		s2 += binary.NativeEndian.Uint32(b[i+4:]) + s1
	}
	return s1, s2
}

// statFile returns the zero File for a missing file that is not required.
func statFile(path string, required bool) (File, bool) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) && !required {
		return File{}, true
	}
	if err != nil || !info.Mode().IsRegular() {
		return File{}, false
	}
	return verificationStat(info)
}

// reusable reports whether file metadata may stand in for a content check.
// SQLite commits in WAL mode in two steps: it writes the new frames to the WAL,
// then publishes them in the WAL-index header in -shm. A check between the two
// steps sees the WAL's new size and mtime but verifies the previous content
// (agy-reader#8k4s). The fingerprint therefore includes the published header,
// which is not trusted while the WAL holds frames after the published ones, so
// nothing is remembered in that gap and any later publish is seen. When there
// is no header to trust, only an empty WAL makes the metadata enough. A
// rollback journal with content means a rollback-mode commit is in progress,
// so its result is never reused.
func reusable(f Files) bool {
	return f.Journal.Size == 0 && (f.WAL.Size == 0 || f.WALIndex != "")
}

func (c *Cache) IsStale(s discovery.Session, checkStale func(discovery.Session) (bool, string)) (bool, string) {
	if c == nil || (filepath.Ext(s.PBPath) != ".db" && filepath.Ext(s.PBPath) != ".pb") {
		return checkStale(s)
	}
	before, ok := Fingerprint(s)
	now := c.now()
	if entry, found := c.entries[s.PBPath]; found {
		age := now.Sub(entry.CheckedAt)
		if ok && reusable(before) && entry.Files == before && !entry.CheckedAt.IsZero() && age >= 0 && age < ReuseLimit(s.PBPath, entry.Files, entry.CheckedAt) {
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
		if stable && before == after && reusable(after) {
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
	disk := DiskCache{Version: Version, Profile: cacheProfile, Boot: c.boot, Root: c.root, Entries: c.entries}
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
