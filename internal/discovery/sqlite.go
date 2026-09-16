package discovery

import (
	"encoding/binary"
	"io"
	"os"
)

// ReadSQLiteCascadeID extracts the cascade_id recorded in trajectory_meta
// from an Antigravity SQLite .db session file. Returns ("", nil) if the file
// is not an SQLite database, has no trajectory_meta table, or has no rows.
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
		pageSize = 65536
	}

	// trajectory_meta is rootpage 2 in Antigravity conversation databases.
	page := make([]byte, pageSize)
	if _, err := f.Seek(int64(pageSize), io.SeekStart); err != nil {
		return "", nil
	}
	if _, err := io.ReadFull(f, page); err != nil {
		return "", nil
	}

	// 0x0D is a b-tree leaf table page.
	if page[0] != 0x0d {
		return "", nil
	}

	cellCount := binary.BigEndian.Uint16(page[3:5])
	if cellCount == 0 {
		return "", nil
	}

	// First cell pointer is at offset 8.
	if len(page) < 10 {
		return "", nil
	}
	cellOffset := binary.BigEndian.Uint16(page[8:10])
	if int(cellOffset) >= len(page) {
		return "", nil
	}

	cell := page[cellOffset:]
	pos := 0

	// Varint: payload size
	payloadSize, n := readVarint(cell[pos:])
	if n == 0 {
		return "", nil
	}
	pos += n

	// Varint: rowid
	_, n = readVarint(cell[pos:])
	if n == 0 {
		return "", nil
	}
	pos += n

	if pos+int(payloadSize) > len(cell) {
		payloadSize = uint64(len(cell) - pos)
	}
	record := cell[pos : pos+int(payloadSize)]

	// Parse record header
	hdrLen, n := readVarint(record)
	if n == 0 || int(hdrLen) > len(record) {
		return "", nil
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
		return "", nil
	}

	// Column 0: trajectory_id (text)
	// Column 1: cascade_id (text)
	st0 := serialTypes[0]
	st1 := serialTypes[1]
	if st0 < 13 || st0%2 == 0 || st1 < 13 || st1%2 == 0 {
		return "", nil
	}

	len0 := int((st0 - 13) / 2)
	len1 := int((st1 - 13) / 2)

	body := record[hdrLen:]
	if len(body) < len0+len1 {
		return "", nil
	}

	cascadeID := string(body[len0 : len0+len1])
	return cascadeID, nil
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
