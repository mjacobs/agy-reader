package completeness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
)

func schedule1510Pair(t *testing.T, name string) (string, string) {
	t.Helper()
	read := func(kind string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("testdata", "schedule1510", name+"."+kind+".json"))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	return read("full"), read("default")
}

func TestSchedule1510AndPDF(t *testing.T) {
	for _, name := range []string{"running", "cancelled", "fired", "pdf"} {
		t.Run(name, func(t *testing.T) {
			raw, shown := schedule1510Pair(t, name)
			path := filepath.Join(t.TempDir(), "source.db")
			db := testfixture.Source(t, path, []byte(raw))
			testfixture.Exec(t, db, "UPDATE steps SET render_info=NULL,permissions=NULL")
			snapshot, err := ReadSnapshot(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, snapshot, raw, shown, Verified)
			offline, err := snapshot.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, snapshot, raw, string(offline), Verified)
		})
	}
}

func TestSchedule1510AndPDFRejectUnqualifiedSource(t *testing.T) {
	for _, test := range []struct {
		name, fixture, before, after string
	}{
		{"running cron", "running", `"mode": "timer"`, `"mode": "cron"`},
		{"missing countdown", "running", ",\n            \"fireTime\": \"2026-01-01T00:03:01.123456789Z\"", ""},
		{"outcome while running", "running", `"mode": "timer"`, `"mode": "timer", "outcome": "cancelled"`},
		{"running without task ID", "running", `"id": "fixture/task-2"`, `"id": ""`},
		{"unqualified completion", "cancelled", `"outcome": "cancelled"`, `"outcome": "other"`},
		{"fired with result text", "fired", `"payload": {`, `"result": "Unexpected", "payload": {`},
		{"changed timer call", "cancelled", `"DurationSeconds": "180"`, `"DurationSeconds": "90"`},
		{"unqualified MIME", "pdf", `application/pdf`, `application/zip`},
		{"wrong result description", "pdf", `Showing binary file of type application/pdf`, `Showing binary file of type image/png`},
		{"wrong byte count", "pdf", `"fileNumBytes": 9`, `"fileNumBytes": 10`},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, shown := schedule1510Pair(t, test.fixture)
			bad := replace1216(t, raw, test.before, test.after)
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
}

func TestSchedule1510AndPDFRequireExactRPC(t *testing.T) {
	for _, test := range []struct {
		fixture, before, after string
	}{
		{"running", "00:03:01.123456789Z", "00:03:02.123456789Z"},
		{"cancelled", "Wait cancelled.", "Changed."},
		{"pdf", "JVBERi0xLjQK", "JVBERi0xLjUK"},
	} {
		t.Run(test.fixture, func(t *testing.T) {
			raw, shown := schedule1510Pair(t, test.fixture)
			snapshot := snapshot1216(t, raw)
			require1216(t, snapshot, replace1216(t, raw, test.before, test.after), shown, Incomplete)
			require1216(t, snapshot, raw, replace1216(t, shown, test.before, test.after), Incomplete)
		})
	}
}
