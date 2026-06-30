package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func writeFileT(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func mustChtimes(t *testing.T, p string, ts time.Time) {
	t.Helper()
	if err := os.Chtimes(p, ts, ts); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

func TestSidecarCoverage(t *testing.T) {
	root := t.TempDir()
	conv := filepath.Join(root, "conversations")

	// fresh: sidecar newer than .db
	writeFileT(t, filepath.Join(conv, "a.db"), []byte("x"))
	writeFileT(t, filepath.Join(conv, "a.trajectory.json"), []byte("{}"))
	old := time.Unix(1779000000, 0)
	newer := time.Unix(1779000300, 0)
	mustChtimes(t, filepath.Join(conv, "a.db"), old)
	mustChtimes(t, filepath.Join(conv, "a.trajectory.json"), newer)

	// stale: .db newer than sidecar
	writeFileT(t, filepath.Join(conv, "b.db"), []byte("x"))
	writeFileT(t, filepath.Join(conv, "b.trajectory.json"), []byte("{}"))
	mustChtimes(t, filepath.Join(conv, "b.trajectory.json"), old)
	mustChtimes(t, filepath.Join(conv, "b.db"), newer)

	// missing: no sidecar
	writeFileT(t, filepath.Join(conv, "c.db"), []byte("x"))

	total, fresh, stale, err := sidecarCoverage(root)
	if err != nil {
		t.Fatalf("sidecarCoverage: %v", err)
	}
	if total != 3 || fresh != 1 || stale != 2 {
		t.Fatalf("got total=%d fresh=%d stale=%d, want 3/1/2", total, fresh, stale)
	}
}

func TestRecordedAgyVersionParsesEmbed(t *testing.T) {
	v := recordedAgyVersion()
	if v == "" {
		t.Fatal("expected a recorded agy version from embedded COMPATIBILITY.md")
	}
	// COMPATIBILITY.md is machine-generated as "1.0.13"-style; assert shape.
	if !regexp.MustCompile(`^\d+\.\d+\.\d+`).MatchString(v) {
		t.Fatalf("recorded version %q is not semver-shaped", v)
	}
}

func TestParseRecordedVersionLine(t *testing.T) {
	got := parseRecordedAgyVersion(
		"- **agy version:** 1.0.13\n- **Verified on:** 2026-06-27\n")
	if got != "1.0.13" {
		t.Fatalf("got %q want 1.0.13", got)
	}
}

func TestWriteDoctorReportHealthy(t *testing.T) {
	var buf bytes.Buffer
	code := writeDoctorReport(&buf, doctorReport{
		daemonURL: "http://127.0.0.1:51847",
		agyVer:    "1.0.13", recordedVer: "1.0.13",
		total: 23, fresh: 23, stale: 0,
		watchKnown: true, watchRunning: true,
	})
	if code != 0 {
		t.Fatalf("healthy report should exit 0, got %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "reachable") || !strings.Contains(out, "23/23") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestWriteDoctorReportStaleNeedsAction(t *testing.T) {
	var buf bytes.Buffer
	code := writeDoctorReport(&buf, doctorReport{
		daemonURL: "http://127.0.0.1:51847",
		agyVer:    "1.0.13", recordedVer: "1.0.13",
		total: 23, fresh: 18, stale: 5,
	})
	if code == 0 {
		t.Fatal("stale sidecars should be actionable (non-zero exit)")
	}
	if !strings.Contains(buf.String(), "agy-reader --sync") {
		t.Fatalf("should suggest --sync:\n%s", buf.String())
	}
}

func TestWriteDoctorReportVersionSkew(t *testing.T) {
	var buf bytes.Buffer
	code := writeDoctorReport(&buf, doctorReport{
		daemonURL: "http://127.0.0.1:51847",
		agyVer:    "1.0.20", recordedVer: "1.0.13",
		total: 1, fresh: 1, stale: 0,
	})
	if code == 0 {
		t.Fatal("version skew should be actionable")
	}
	if !strings.Contains(buf.String(), "1.0.20") {
		t.Fatalf("should show running version:\n%s", buf.String())
	}
}
