package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/discovery"
)

func healthyReport(root string, surface discovery.Surface) doctorReport {
	return doctorReport{
		surface:   surface,
		root:      root,
		daemonURL: "http://127.0.0.1:51847",
		csrfFound: surface == discovery.SurfaceIDE,
		agyVer:    "1.0.16", recordedVer: "1.0.16",
		total: 3, fresh: 3,
		watchKnown: true, watchRunning: true,
	}
}

func staleReport(root string, surface discovery.Surface) doctorReport {
	r := healthyReport(root, surface)
	r.fresh = 1
	r.stale = 2
	return r
}

// A multi-root doctor run renders one block per root, each naming its root
// and surface, with a single trailing exit line.
func TestWriteMultiDoctorReportRendersPerRootBlocks(t *testing.T) {
	var buf bytes.Buffer
	code := writeMultiDoctorReport(&buf, []doctorReport{
		healthyReport("/home/x/.gemini/antigravity-cli", discovery.SurfaceCLI),
		healthyReport("/home/x/.gemini/antigravity", discovery.SurfaceIDE),
	}, false)
	if code != 0 {
		t.Fatalf("two healthy roots should exit 0, got %d:\n%s", code, buf.String())
	}
	out := buf.String()
	for _, want := range []string{
		"/home/x/.gemini/antigravity-cli:",
		"/home/x/.gemini/antigravity:",
		"surface:     cli",
		"surface:     ide",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in multi-root report:\n%s", want, out)
		}
	}
	if got := strings.Count(out, "exit 0"); got != 1 {
		t.Errorf("want exactly one exit line, got %d:\n%s", got, out)
	}
}

// Single-root doctor output is a live contract: no root header line appears,
// matching the pre-multi-root report shape.
func TestWriteMultiDoctorReportSingleRootHasNoHeader(t *testing.T) {
	var buf bytes.Buffer
	writeMultiDoctorReport(&buf, []doctorReport{
		healthyReport("/home/x/.gemini/antigravity-cli", discovery.SurfaceCLI),
	}, true)
	if strings.Contains(buf.String(), "antigravity-cli:") {
		t.Fatalf("single-root report must not grow a root header:\n%s", buf.String())
	}
}

// Explicitly requested roots are hard requirements: any unhealthy one fails
// the run, even when another root is fine.
func TestWriteMultiDoctorReportExplicitUnhealthyFails(t *testing.T) {
	var buf bytes.Buffer
	code := writeMultiDoctorReport(&buf, []doctorReport{
		healthyReport("/a", discovery.SurfaceCLI),
		staleReport("/b", discovery.SurfaceIDE),
	}, true)
	if code == 0 {
		t.Fatalf("an unhealthy explicitly requested root must fail doctor:\n%s", buf.String())
	}
}

// The phase-2 crux: with discovered (non-explicit) roots, one unhealthy root
// is WAITING, not failing — doctor reports it per-surface but exits 0 while
// any root is healthy. Only all-roots-unhealthy fails.
func TestWriteMultiDoctorReportDiscoveredSoftFails(t *testing.T) {
	var buf bytes.Buffer
	code := writeMultiDoctorReport(&buf, []doctorReport{
		healthyReport("/a", discovery.SurfaceCLI),
		staleReport("/b", discovery.SurfaceIDE),
	}, false)
	if code != 0 {
		t.Fatalf("a discovered unhealthy root must not fail doctor while another is healthy:\n%s", buf.String())
	}

	buf.Reset()
	code = writeMultiDoctorReport(&buf, []doctorReport{
		staleReport("/a", discovery.SurfaceCLI),
		staleReport("/b", discovery.SurfaceIDE),
	}, false)
	if code == 0 {
		t.Fatalf("all discovered roots unhealthy must fail doctor:\n%s", buf.String())
	}
}
