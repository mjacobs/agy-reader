package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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

// watchRunning best-effort detects a separate `agy-reader --watch` process.
// Returns (running, known); known=false where detection isn't supported.
func watchRunning() (running, known bool) {
	if runtime.GOOS != "linux" {
		return false, false
	}
	return scanProcForWatch()
}

// scanProcForWatch walks /proc looking for another agy-reader process invoked
// with --watch. It skips our own PID and ignores unreadable entries. A failure
// to read /proc itself yields (false, true): detection is supported here but
// nothing was found.
func scanProcForWatch() (running, known bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false, true
	}
	self := os.Getpid()
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue // not a PID dir, or our own process
		}
		data, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil {
			continue // process may have exited, or we lack permission
		}
		if cmdlineIsWatch(data) {
			return true, true
		}
	}
	return false, true
}

// cmdlineIsWatch reports whether a /proc cmdline (NUL-separated argv) is an
// agy-reader process running the --watch loop. It matches on argv structure
// rather than a loose substring scan so that an unrelated process merely
// mentioning these strings (a shell echoing a command, an editor, another
// `agy-reader doctor`) is not misreported as a running watcher.
func cmdlineIsWatch(data []byte) bool {
	argv := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	if len(argv) == 0 || filepath.Base(argv[0]) != "agy-reader" {
		return false
	}
	for _, arg := range argv[1:] {
		// The boolean watch flag, in the forms Go's flag package accepts.
		// Excludes --watch-interval / --watch-idle-timeout, which take values.
		if arg == "--watch" || arg == "-watch" ||
			strings.HasPrefix(arg, "--watch=") || strings.HasPrefix(arg, "-watch=") {
			return true
		}
	}
	return false
}

// buildDoctorReport gathers daemon reachability, agy-version compatibility, and
// sidecar coverage for root into a doctorReport.
func buildDoctorReport(root string) doctorReport {
	r := doctorReport{recordedVer: recordedAgyVersion(), agyVer: agyVersion()}
	if url, err := discovery.DiscoverDaemonURL(root); err == nil {
		r.daemonURL = url
	} else {
		r.daemonErr = err
	}
	r.total, r.fresh, r.stale, _ = sidecarCoverage(root)
	r.watchRunning, r.watchKnown = watchRunning()
	return r
}

// runDoctorTo renders the doctor report to w and returns the actionable-problem
// count (the intended process exit code). It is the testable seam for runDoctor.
func runDoctorTo(w io.Writer, root string) int {
	return writeDoctorReport(w, buildDoctorReport(root))
}

// runDoctor prints the doctor report to stdout and exits non-zero when there is
// something actionable, so callers can gate on `agy-reader doctor`'s exit code.
func runDoctor(root string) error {
	if runDoctorTo(os.Stdout, root) != 0 {
		os.Exit(1)
	}
	return nil
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
