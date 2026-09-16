package discovery

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
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

func TestReadSQLiteCascadeIDLiveSessions(t *testing.T) {
	// If the user's sessions exist, test the 5 target sessions with known internal IDs.
	cases := []struct {
		filenameID string
		internalID string
	}{
		{"fcf78168-0559-4217-8f8b-f6712cfa854d", "62fe6277-5b6e-4428-b446-631a238b13f8"},
		{"0806de5a-10f1-411d-9124-6f4c2aac3d96", "13c4ae7c-189d-4255-ab1f-f9a7c74f6110"},
		{"6c7be1e4-7e1e-4b37-9f16-0a09feade911", "41cebfa0-b750-4d95-9fa8-1db2e24376a4"},
		{"c0d48394-351d-4fd7-96c0-8dc73ff397e4", "09d875da-c39d-4cef-b10e-dbf71bbd9f84"},
		{"3d094635-6ec4-4770-8b3e-b62de000818c", "6190be03-3f9e-4549-80e5-cbbcd7dacaee"},
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	cliDir := filepath.Join(home, ".gemini/antigravity-cli/conversations")

	for _, tc := range cases {
		dbPath := filepath.Join(cliDir, tc.filenameID+".db")
		if _, err := os.Stat(dbPath); err != nil {
			continue
		}
		got, err := ReadSQLiteCascadeID(dbPath)
		if err != nil {
			t.Errorf("file %s: unexpected error: %v", tc.filenameID, err)
		} else if got != tc.internalID {
			t.Errorf("file %s: got internal cascade ID %q, want %q", tc.filenameID, got, tc.internalID)
		}
	}
}
