package discovery

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadSQLiteCascadeIDInvalidFiles(t *testing.T) {
	dir := t.TempDir()

	// Non-existent file
	if _, err := ReadSQLiteCascadeID(filepath.Join(dir, "missing.db")); err == nil {
		t.Error("expected error for non-existent file, got nil")
	}

	// Empty file
	emptyFile := filepath.Join(dir, "empty.db")
	if err := os.WriteFile(emptyFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	id, err := ReadSQLiteCascadeID(emptyFile)
	if err != nil || id != "" {
		t.Errorf("empty file: expected ('', nil), got (%q, %v)", id, err)
	}

	// Short file (< 100 bytes)
	shortFile := filepath.Join(dir, "short.db")
	if err := os.WriteFile(shortFile, []byte("SQLite format 3\x00short"), 0o644); err != nil {
		t.Fatal(err)
	}
	id, err = ReadSQLiteCascadeID(shortFile)
	if err != nil || id != "" {
		t.Errorf("short file: expected ('', nil), got (%q, %v)", id, err)
	}

	// Non-sqlite magic header
	nonSqlite := filepath.Join(dir, "not_sqlite.db")
	dummy := make([]byte, 200)
	copy(dummy, "NOT SQLite header")
	if err := os.WriteFile(nonSqlite, dummy, 0o644); err != nil {
		t.Fatal(err)
	}
	id, err = ReadSQLiteCascadeID(nonSqlite)
	if err != nil || id != "" {
		t.Errorf("non-sqlite file: expected ('', nil), got (%q, %v)", id, err)
	}
}

func TestReadSQLiteCascadeIDSynthetic(t *testing.T) {
	const (
		pageSize = 4096
		trajID   = "11111111-2222-3333-4444-555555555555"
		cascID   = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	)

	// Page 1: DB header
	hdr := make([]byte, pageSize)
	copy(hdr[:16], "SQLite format 3\x00")
	binary.BigEndian.PutUint16(hdr[16:18], pageSize)
	hdr[18] = 1
	hdr[19] = 1

	// Page 2: leaf table page
	page2 := make([]byte, pageSize)
	page2[0] = 0x0D                           // leaf table
	binary.BigEndian.PutUint16(page2[3:5], 1) // 1 cell

	const cellOffset = 3800
	binary.BigEndian.PutUint16(page2[8:10], cellOffset)

	// Cell payload:
	// record header: 3 bytes: len(3), serialType0(85 = 36-byte str), serialType1(85 = 36-byte str)
	recHdr := []byte{3, 85, 85}
	var payload []byte
	payload = append(payload, recHdr...)
	payload = append(payload, []byte(trajID)...)
	payload = append(payload, []byte(cascID)...)

	var cell []byte
	cell = append(cell, byte(len(payload))) // varint payload size
	cell = append(cell, 1)                  // varint rowid
	cell = append(cell, payload...)

	copy(page2[cellOffset:], cell)

	tmp := filepath.Join(t.TempDir(), "synthetic.db")
	content := append(hdr, page2...)
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadSQLiteCascadeID(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != cascID {
		t.Errorf("got %q, want %q", got, cascID)
	}
}

func TestReadSQLiteCascadeIDDiffersFromFilename(t *testing.T) {
	const filenameID = "11111111-1111-4111-8111-111111111111"
	const internalID = "22222222-2222-4222-8222-222222222222"
	dbPath := filepath.Join(t.TempDir(), filenameID+".db")
	if err := os.WriteFile(dbPath, syntheticDB(t, 4096, filenameID, internalID), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSQLiteCascadeID(dbPath)
	if err != nil || got != internalID {
		t.Fatalf("ReadSQLiteCascadeID = %q, %v; want %q, nil", got, err, internalID)
	}
}

// syntheticDB builds a minimal SQLite file whose page 2 is a leaf table page
// holding one (trajectory_id, cascade_id) row.
func syntheticDB(t *testing.T, pageSize int, trajID, cascID string) []byte {
	t.Helper()
	hdr := make([]byte, pageSize)
	copy(hdr[:16], "SQLite format 3\x00")
	binary.BigEndian.PutUint16(hdr[16:18], uint16(pageSize))
	hdr[18], hdr[19] = 1, 1
	return append(hdr, syntheticLeafPage(t, pageSize, trajID, cascID)...)
}

func syntheticLeafPage(t *testing.T, pageSize int, trajID, cascID string) []byte {
	t.Helper()
	page := make([]byte, pageSize)
	page[0] = 0x0D
	binary.BigEndian.PutUint16(page[3:5], 1)
	cellOffset := pageSize - 256
	binary.BigEndian.PutUint16(page[8:10], uint16(cellOffset))
	payload := append([]byte{3, byte(13 + 2*len(trajID)), byte(13 + 2*len(cascID))}, trajID...)
	payload = append(payload, cascID...)
	cell := append([]byte{byte(len(payload)), 1}, payload...)
	copy(page[cellOffset:], cell)
	return page
}

// An open session keeps its newest trajectory_meta row in the write-ahead log
// until a checkpoint folds it in, so the WAL has to win over the main file.
// Frames from a recycled WAL generation (mismatched salt) and frames after the
// last commit frame must be ignored.
func TestReadSQLiteCascadeIDPrefersCommittedWALPage(t *testing.T) {
	const (
		pageSize    = 1024
		trajID      = "11111111-2222-3333-4444-555555555555"
		checkointed = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
		inWAL       = "bbbbbbbb-cccc-dddd-eeee-ffffffffffff"
		uncommitted = "cccccccc-dddd-eeee-ffff-000000000000"
		staleSalt   = "dddddddd-eeee-ffff-0000-111111111111"
	)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session.db")
	if err := os.WriteFile(dbPath, syntheticDB(t, pageSize, trajID, checkointed), 0o644); err != nil {
		t.Fatal(err)
	}

	salt := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	// Both checksum byte orders must be readable: a reader that maps the magic
	// the wrong way round rejects one variant's header and silently ignores
	// the whole log.
	for _, v := range walVariants {
		b := newWALBuilder(v.magic, v.order, pageSize, salt)
		b.frame(2, 2, salt, syntheticLeafPage(t, pageSize, trajID, inWAL))                   // committed
		b.frame(2, 0, salt, syntheticLeafPage(t, pageSize, trajID, uncommitted))             // no commit frame yet
		b.frame(2, 2, []byte("87654321"), syntheticLeafPage(t, pageSize, trajID, staleSalt)) // recycled generation
		if err := os.WriteFile(dbPath+"-wal", b.buf, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := ReadSQLiteCascadeID(dbPath)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", v.name, err)
		}
		if got != inWAL {
			t.Errorf("%s: got %q, want the newest committed WAL row %q", v.name, got, inWAL)
		}
	}

	// Without a WAL the main database is authoritative again.
	if err := os.Remove(dbPath + "-wal"); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadSQLiteCascadeID(dbPath); err != nil || got != checkointed {
		t.Errorf("after checkpoint: got (%q, %v), want %q", got, err, checkointed)
	}
}

// Corrupt or hostile session files must not panic discovery: a bad page size,
// an oversized payload varint, or an oversized column length all have to fall
// back to "no internal id".
func TestReadSQLiteCascadeIDRejectsMalformedPagesWithoutPanicking(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]func([]byte) []byte{
		"zero page size": func(db []byte) []byte {
			binary.BigEndian.PutUint16(db[16:18], 0)
			return db
		},
		"non power of two page size": func(db []byte) []byte {
			binary.BigEndian.PutUint16(db[16:18], 1000)
			return db
		},
		"page size below minimum": func(db []byte) []byte {
			binary.BigEndian.PutUint16(db[16:18], 256)
			return db
		},
		"oversized payload varint": func(db []byte) []byte {
			cell := 1024 + 1024 - 256
			copy(db[cell:], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 1})
			return db
		},
		"oversized column length": func(db []byte) []byte {
			cell := 1024 + 1024 - 256
			// payload size, rowid, then a header claiming two huge text columns.
			copy(db[cell:], []byte{10, 1, 3, 0xff, 0x7f, 0xff, 0x7f})
			return db
		},
		"truncated after header": func(db []byte) []byte { return db[:100] },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".db")
			db := mutate(syntheticDB(t, 1024, "11111111-2222-3333-4444-555555555555",
				"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"))
			if err := os.WriteFile(path, db, 0o644); err != nil {
				t.Fatal(err)
			}
			id, err := ReadSQLiteCascadeID(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id == "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
				t.Fatalf("malformed file should not yield the intact id")
			}
		})
	}
}

// A filename id must never be answered by a database that merely carries the
// same value as its internal cascade_id: an imported or copied session can do
// exactly that, and the newest-first listing would otherwise hand back the
// wrong transcript.
func TestFindByIDPrefersExactFilenameMatchOverInternalID(t *testing.T) {
	const (
		wanted = "11111111-2222-3333-4444-555555555555"
		other  = "99999999-8888-7777-6666-555555555555"
	)
	root := t.TempDir()
	dir := filepath.Join(root, "conversations")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The impostor is newer, so it sorts first in the listing.
	older := filepath.Join(dir, wanted+".pb")
	newer := filepath.Join(dir, other+".db")
	if err := os.WriteFile(older, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newer, syntheticDB(t, 1024, "trajectory", wanted), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(older, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	got, found, err := FindByID(root, wanted)
	if err != nil || !found {
		t.Fatalf("FindByID: found=%v err=%v", found, err)
	}
	if got.PBPath != older {
		t.Errorf("internal id shadowed the exact filename match: got %s want %s", got.PBPath, older)
	}

	// The internal id still resolves when nothing matches by filename.
	got, found, err = FindByID(root, "33333333-3333-4333-8333-333333333333")
	if err != nil || found {
		t.Fatalf("unknown id should not resolve: found=%v err=%v (%s)", found, err, got.PBPath)
	}
}

// The WAL magic's low bit selects the byte order the checksum input words are
// read in, matching SQLite's wal.c (bigEndCksum = magic & 1). The pairing is
// spelled out here rather than derived, so a reader that reverses it fails
// these tests instead of agreeing with a fixture that made the same mistake.
var walVariants = []struct {
	name  string
	magic uint32
	order binary.ByteOrder
}{
	{"little-endian checksums", 0x377f0682, binary.LittleEndian},
	{"big-endian checksums", 0x377f0683, binary.BigEndian},
}

// walChecksumFixture mirrors SQLite's rolling WAL checksum so tests can build
// logs the reader will accept as intact.
func walChecksumFixture(order binary.ByteOrder, s0, s1 uint32, data []byte) (uint32, uint32) {
	for i := 0; i+8 <= len(data); i += 8 {
		s0 += order.Uint32(data[i:i+4]) + s1
		s1 += order.Uint32(data[i+4:i+8]) + s0
	}
	return s0, s1
}

// walBuilder assembles a write-ahead log the way SQLite does, stamping the
// header checksum and the rolling per-frame checksums, so fixtures exercise
// the reader's integrity checks instead of bypassing them. The stored checksum
// values are always big-endian; order applies to the checksum input only.
type walBuilder struct {
	buf    []byte
	order  binary.ByteOrder
	s0, s1 uint32
}

func newWALBuilder(magic uint32, order binary.ByteOrder, pageSize uint32, salt []byte) *walBuilder {
	hdr := make([]byte, 32)
	binary.BigEndian.PutUint32(hdr[0:4], magic)
	binary.BigEndian.PutUint32(hdr[4:8], 3007000)
	binary.BigEndian.PutUint32(hdr[8:12], pageSize)
	copy(hdr[16:24], salt)
	s0, s1 := walChecksumFixture(order, 0, 0, hdr[0:24])
	binary.BigEndian.PutUint32(hdr[24:28], s0)
	binary.BigEndian.PutUint32(hdr[28:32], s1)
	return &walBuilder{buf: hdr, order: order, s0: s0, s1: s1}
}

// frame appends one frame, chaining its checksum onto the running value.
func (b *walBuilder) frame(pgno, dbSize uint32, frameSalt, page []byte) {
	h := make([]byte, 24)
	binary.BigEndian.PutUint32(h[0:4], pgno)
	binary.BigEndian.PutUint32(h[4:8], dbSize)
	copy(h[8:16], frameSalt)
	s0, s1 := walChecksumFixture(b.order, b.s0, b.s1, h[0:8])
	s0, s1 = walChecksumFixture(b.order, s0, s1, page)
	binary.BigEndian.PutUint32(h[16:20], s0)
	binary.BigEndian.PutUint32(h[20:24], s1)
	b.buf = append(b.buf, h...)
	b.buf = append(b.buf, page...)
	b.s0, b.s1 = s0, s1
}

// A torn or partially overwritten frame keeps the right salt and can carry a
// plausible commit marker, so only the checksum distinguishes it from a real
// commit. The scan must stop there and keep the last frame that verified,
// rather than letting corrupt bytes override a good id.
func TestReadSQLiteCascadeIDStopsAtCorruptWALFrame(t *testing.T) {
	const (
		pageSize    = 1024
		trajID      = "11111111-2222-3333-4444-555555555555"
		checkointed = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
		inWAL       = "bbbbbbbb-cccc-dddd-eeee-ffffffffffff"
		torn        = "cccccccc-dddd-eeee-ffff-000000000000"
	)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session.db")
	if err := os.WriteFile(dbPath, syntheticDB(t, pageSize, trajID, checkointed), 0o644); err != nil {
		t.Fatal(err)
	}
	salt := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	b := newWALBuilder(0x377f0682, binary.LittleEndian, pageSize, salt)
	b.frame(2, 2, salt, syntheticLeafPage(t, pageSize, trajID, inWAL))
	intact := len(b.buf)
	b.frame(2, 2, salt, syntheticLeafPage(t, pageSize, trajID, torn))
	// Corrupt the second frame's page image after its checksum was stamped,
	// exactly as a half-written frame would appear.
	b.buf[intact+24+40] ^= 0xff
	if err := os.WriteFile(dbPath+"-wal", b.buf, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSQLiteCascadeID(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != inWAL {
		t.Errorf("got %q, want the last frame that verified %q", got, inWAL)
	}

	// A WAL whose own header does not verify is unusable end to end: every
	// frame checksum chains off it, so the main database stays authoritative.
	bad := newWALBuilder(0x377f0682, binary.LittleEndian, pageSize, salt)
	bad.frame(2, 2, salt, syntheticLeafPage(t, pageSize, trajID, torn))
	bad.buf[28] ^= 0xff
	if err := os.WriteFile(dbPath+"-wal", bad.buf, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadSQLiteCascadeID(dbPath); err != nil || got != checkointed {
		t.Errorf("corrupt WAL header: got (%q, %v), want the main database %q", got, err, checkointed)
	}
}
