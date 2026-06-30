package main

import (
	_ "embed"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/mjacobs/agy-reader/internal/discovery"
)

//go:embed COMPATIBILITY.md
var compatibilityMD string

// parseRecordedAgyVersion extracts the version from the
// "- **agy version:** X.Y.Z" line of COMPATIBILITY.md.
func parseRecordedAgyVersion(md string) string {
	const marker = "**agy version:**"
	for _, line := range strings.Split(md, "\n") {
		if i := strings.Index(line, marker); i >= 0 {
			return strings.TrimSpace(line[i+len(marker):])
		}
	}
	return ""
}

func recordedAgyVersion() string { return parseRecordedAgyVersion(compatibilityMD) }

// agyVersion returns the running `agy` version (first line of `agy --version`),
// or "" if agy is not on PATH or errors.
func agyVersion() string {
	out, err := exec.Command("agy", "--version").Output()
	if err != nil {
		return ""
	}
	if i := strings.IndexByte(string(out), '\n'); i >= 0 {
		return strings.TrimSpace(string(out)[:i])
	}
	return strings.TrimSpace(string(out))
}

type doctorReport struct {
	daemonURL    string // "" when unreachable
	daemonErr    error
	agyVer       string // "" when agy not found
	recordedVer  string
	total        int
	fresh        int
	stale        int
	watchRunning bool
	watchKnown   bool
}

// writeDoctorReport prints the human-readable report and returns the number
// of actionable problems (0 = healthy). Stale sidecars and a version skew
// are actionable; an unreachable daemon alone is informational.
func writeDoctorReport(w io.Writer, r doctorReport) int {
	problems := 0

	// daemon
	if r.daemonURL != "" {
		fmt.Fprintf(w, "  daemon:      reachable (%s)\n", r.daemonURL)
	} else {
		fmt.Fprintf(w, "  daemon:      not running (start `agy` to refresh sidecars)\n")
	}

	// agy version
	switch {
	case r.agyVer == "":
		fmt.Fprintf(w, "  agy version: unknown (agy not on PATH)\n")
	case r.recordedVer != "" && r.agyVer != r.recordedVer:
		fmt.Fprintf(w, "  agy version: %s  (recorded %s — re-run agy-format-audit)\n",
			r.agyVer, r.recordedVer)
		problems++
	default:
		fmt.Fprintf(w, "  agy version: %s  (compatible)\n", r.agyVer)
	}

	// sidecars
	fmt.Fprintf(w, "  sidecars:    %d/%d fresh\n", r.fresh, r.total)
	if r.stale > 0 {
		problems++
		if r.daemonURL != "" {
			fmt.Fprintf(w, "               %d missing/stale -> run: agy-reader --sync\n", r.stale)
		} else {
			fmt.Fprintf(w, "               %d missing/stale -> start agy, then: agy-reader --sync\n", r.stale)
		}
	}

	// watch
	switch {
	case !r.watchKnown:
		fmt.Fprintf(w, "  watch:       unknown (optional: agy-reader --watch)\n")
	case r.watchRunning:
		fmt.Fprintf(w, "  watch:       running\n")
	default:
		fmt.Fprintf(w, "  watch:       not running (optional: agy-reader --watch)\n")
	}

	if problems == 0 {
		fmt.Fprintf(w, "\n  exit 0 — healthy\n")
	} else {
		fmt.Fprintf(w, "\n  exit 1 — action needed\n")
	}
	return problems
}

// sidecarCoverage reports how many conversations/ sessions have a fresh
// sidecar versus a missing/stale one, reusing the same staleness rule the
// watcher uses (isStale).
func sidecarCoverage(root string) (total, fresh, stale int, err error) {
	sessions, err := discovery.ListConversationSessions(root)
	if err != nil {
		return 0, 0, 0, err
	}
	for _, s := range sessions {
		total++
		if isStaleNow, _ := isStale(s); isStaleNow {
			stale++
		} else {
			fresh++
		}
	}
	return total, fresh, stale, nil
}
