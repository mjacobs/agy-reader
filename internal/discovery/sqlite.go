package discovery

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
)

// SQLite page sizes are powers of two between 512 bytes and 64 KiB. A header
// advertising anything else describes a file this reader cannot parse, and
// trusting it would index past the end of a short page.
const (
	minPageSize = 512
	maxPageSize = 65536
)

// WAL header magic. Both values describe the same frame layout; the low bit
// selects the byte order the checksum *input* words are read in, matching
// SQLite's own wal.c (bigEndCksum = magic & 1). The stored checksum values
// themselves are always big-endian regardless.
const (
	walMagicLECksum = 0x377f0682
	walMagicBECksum = 0x377f0683
	walHdrSize      = 32
	frameHdr        = 24
)

// walChecksum runs SQLite's rolling WAL checksum over data, continuing from
// (s0, s1). data is always a multiple of 8 bytes here: the 8-byte prefix of a
// frame header, or a page whose size is a power of two of at least 512.
func walChecksum(order binary.ByteOrder, s0, s1 uint32, data []byte) (uint32, uint32) {
	for i := 0; i+8 <= len(data); i += 8 {
		s0 += order.Uint32(data[i:i+4]) + s1
		s1 += order.Uint32(data[i+4:i+8]) + s0
	}
	return s0, s1
}

// ReadSQLiteCascadeID extracts the cascade_id recorded in trajectory_meta
// from an Antigravity SQLite .db session file. Returns ("", nil) if the file
// is not an SQLite database, has no trajectory_meta table, or has no rows.
//
// A session the daemon still has open keeps its newest rows in the write-ahead
// log until a checkpoint folds them into the database, so the WAL is consulted
// first: reading only the main file leaves active sessions without an internal
// id. Malformed input never panics — every offset taken from the file is
// bounds-checked before it is used.
func ReadSQLiteCascadeID(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var header [100]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return "", nil
	}
	if string(header[:16]) != "SQLite format 3\x00" {
		return "", nil
	}

	pageSize := int(binary.BigEndian.Uint16(header[16:18]))
	if pageSize == 1 {
		pageSize = maxPageSize
	}
	if !validPageSize(pageSize) {
		return "", nil
	}

	// trajectory_meta is rootpage 2 in Antigravity conversation databases.
	if page, ok := walPage(path+"-wal", 2, pageSize); ok {
		return cascadeIDFromLeafPage(page), nil
	}
	page := make([]byte, pageSize)
	if _, err := f.Seek(int64(pageSize), io.SeekStart); err != nil {
		return "", nil
	}
	if _, err := io.ReadFull(f, page); err != nil {
		return "", nil
	}
	return cascadeIDFromLeafPage(page), nil
}

func validPageSize(size int) bool {
	return size >= minPageSize && size <= maxPageSize && size&(size-1) == 0
}

// walPage returns the newest committed image of pgno in a write-ahead log.
// Frames carrying a different salt pair belong to an earlier WAL generation
// that a checkpoint has already recycled, and frames written after the last
// commit frame are part of a transaction that may never land, so neither is
// used.
//
// Salt and commit markers alone cannot tell a torn or partially overwritten
// frame from a good one — a writer crash or a concurrent write can leave a
// frame with the right salt and a plausible commit marker but garbage content.
// So the header checksum is verified before any frame is trusted, and the
// rolling frame checksums are verified in order: the scan stops at the first
// frame that does not verify, keeping only the last valid committed image.
// Reading a corrupt frame would otherwise override a perfectly good id from
// the main database.
func walPage(walPath string, pgno uint32, dbPageSize int) ([]byte, bool) {
	f, err := os.Open(walPath)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	var hdr [walHdrSize]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return nil, false
	}
	var order binary.ByteOrder
	switch binary.BigEndian.Uint32(hdr[0:4]) {
	case walMagicLECksum:
		order = binary.LittleEndian
	case walMagicBECksum:
		order = binary.BigEndian
	default:
		return nil, false
	}
	if int(binary.BigEndian.Uint32(hdr[8:12])) != dbPageSize {
		return nil, false
	}
	// The header checksum covers its own first 24 bytes and seeds the rolling
	// frame checksums. A header that does not verify makes every later frame
	// unverifiable, so the whole WAL is ignored.
	s0, s1 := walChecksum(order, 0, 0, hdr[0:24])
	if s0 != binary.BigEndian.Uint32(hdr[24:28]) || s1 != binary.BigEndian.Uint32(hdr[28:32]) {
		return nil, false
	}
	salt := hdr[16:24]

	var committed, pending []byte
	frame := make([]byte, frameHdr+dbPageSize)
	for {
		if _, err := io.ReadFull(f, frame); err != nil {
			break
		}
		if !bytes.Equal(frame[8:16], salt) {
			break
		}
		// Each frame's checksum continues the running value over the frame's
		// first 8 header bytes and then its page image.
		f0, f1 := walChecksum(order, s0, s1, frame[0:8])
		f0, f1 = walChecksum(order, f0, f1, frame[frameHdr:])
		if f0 != binary.BigEndian.Uint32(frame[16:20]) || f1 != binary.BigEndian.Uint32(frame[20:24]) {
			break
		}
		s0, s1 = f0, f1
		if binary.BigEndian.Uint32(frame[0:4]) == pgno {
			pending = append([]byte(nil), frame[frameHdr:]...)
		}
		// A nonzero database size marks the commit frame of a transaction.
		if binary.BigEndian.Uint32(frame[4:8]) != 0 && pending != nil {
			committed, pending = pending, nil
		}
	}
	return committed, committed != nil
}

// cascadeIDFromLeafPage reads the cascade_id column out of the first cell of a
// b-tree leaf table page. Returns "" for any page that does not hold the
// expected two leading text columns.
func cascadeIDFromLeafPage(page []byte) string {
	// 0x0D is a b-tree leaf table page.
	if len(page) < 10 || page[0] != 0x0d {
		return ""
	}
	if binary.BigEndian.Uint16(page[3:5]) == 0 {
		return ""
	}
	// First cell pointer is at offset 8.
	cellOffset := int(binary.BigEndian.Uint16(page[8:10]))
	if cellOffset >= len(page) {
		return ""
	}

	cell := page[cellOffset:]
	pos := 0

	// Varint: payload size
	payloadSize, n := readVarint(cell[pos:])
	if n == 0 {
		return ""
	}
	pos += n

	// Varint: rowid
	_, n = readVarint(cell[pos:])
	if n == 0 {
		return ""
	}
	pos += n

	avail := len(cell) - pos
	if avail <= 0 {
		return ""
	}
	// An overlong payload size describes bytes spilled to an overflow page,
	// which this reader does not follow; take what is on this page.
	if payloadSize > uint64(avail) {
		payloadSize = uint64(avail)
	}
	record := cell[pos : pos+int(payloadSize)]

	// Parse record header
	hdrLen, n := readVarint(record)
	if n == 0 || hdrLen > uint64(len(record)) || int(hdrLen) < n {
		return ""
	}

	pos2 := n
	var serialTypes []uint64
	for pos2 < int(hdrLen) {
		st, n := readVarint(record[pos2:int(hdrLen)])
		if n == 0 {
			break
		}
		pos2 += n
		serialTypes = append(serialTypes, st)
	}

	if len(serialTypes) < 2 {
		return ""
	}

	// Column 0: trajectory_id (text)
	// Column 1: cascade_id (text)
	st0 := serialTypes[0]
	st1 := serialTypes[1]
	if st0 < 13 || st0%2 == 0 || st1 < 13 || st1%2 == 0 {
		return ""
	}

	body := record[hdrLen:]
	len0 := (st0 - 13) / 2
	len1 := (st1 - 13) / 2
	if len0 > uint64(len(body)) || len1 > uint64(len(body))-len0 {
		return ""
	}
	return string(body[len0 : len0+len1])
}

func readVarint(buf []byte) (uint64, int) {
	var val uint64
	for i, b := range buf {
		if i == 8 {
			return (val << 8) | uint64(b), 9
		}
		val = (val << 7) | uint64(b&0x7f)
		if b&0x80 == 0 {
			return val, i + 1
		}
	}
	return 0, 0
}
