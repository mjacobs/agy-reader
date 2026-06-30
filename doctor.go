package main

import (
	_ "embed"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

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
	daemonURL    string // resolved reachable daemon URL; "" when unreachable
	pinnedURL    string // ANTIGRAVITY_DAEMON_URL value, if the user pinned one
	agyVer       string // "" when agy not found
	recordedVer  string
	total        int
	fresh        int
	stale        int
	coverageErr  error // non-nil when sidecar coverage could not be read
	watchRunning bool
	watchKnown   bool
}

// writeDoctorReport prints the human-readable report and returns the number
// of actionable problems (0 = healthy). Stale sidecars and a version skew
// are actionable; an unreachable daemon alone is informational.
func writeDoctorReport(w io.Writer, r doctorReport) int {
	problems := 0

	// daemon
	switch {
	case r.daemonURL != "":
		fmt.Fprintf(w, "  daemon:      reachable (%s)\n", r.daemonURL)
	case r.pinnedURL != "":
		// A pinned ANTIGRAVITY_DAEMON_URL the CLI would use, but nothing is
		// listening on it. Unlike agy simply being closed (which auto-discovery
		// recovers from on restart), a stale pin never self-heals — agy binds a
		// new random port each start — so the CLI/watch paths keep failing.
		// That is an actionable misconfiguration, not informational.
		problems++
		fmt.Fprintf(w, "  daemon:      configured ANTIGRAVITY_DAEMON_URL (%s) unreachable -> fix or unset it\n", r.pinnedURL)
	default:
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
	switch {
	case r.coverageErr != nil:
		// Could not enumerate sessions (e.g. an unreadable conversations dir).
		// Report it as actionable rather than implying a healthy 0/0.
		problems++
		fmt.Fprintf(w, "  sidecars:    unknown (could not read sessions: %v)\n", r.coverageErr)
	default:
		fmt.Fprintf(w, "  sidecars:    %d/%d fresh\n", r.fresh, r.total)
		if r.stale > 0 {
			problems++
			// --watch refreshes every missing/stale sidecar in one pass; bare
			// --sync needs a cascade id, so it cannot batch-fix from here.
			switch {
			case r.daemonURL != "":
				fmt.Fprintf(w, "               %d missing/stale -> run: agy-reader --watch\n", r.stale)
			case r.pinnedURL != "":
				// --watch would reuse the same dead pinned URL, so fixing the
				// env var is the real first step.
				fmt.Fprintf(w, "               %d missing/stale -> fix or unset ANTIGRAVITY_DAEMON_URL, then: agy-reader --watch\n", r.stale)
			default:
				fmt.Fprintf(w, "               %d missing/stale -> start agy, then: agy-reader --watch\n", r.stale)
			}
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
//
// The args are parsed with a FlagSet mirroring run()'s definitions, so the
// boolean value is honored (--watch=false is not a watcher) and an args layout
// the real flag parser would stop at — a flag after a positional, or after a
// "--" terminator — is treated the same way the live process would treat it.
func cmdlineIsWatch(data []byte) bool {
	argv := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	if len(argv) == 0 || filepath.Base(argv[0]) != "agy-reader" {
		return false
	}
	fs := flag.NewFlagSet("agy-reader", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	watch := fs.Bool("watch", false, "")
	// Remaining flags exist only so parsing matches run(): the value-taking
	// ones (string/duration) must be declared or their values would be
	// misread as positionals and stop the scan early.
	fs.Bool("list", false, "")
	fs.Bool("include-implicit", false, "")
	fs.Bool("sync", false, "")
	fs.String("format", "", "")
	fs.String("root", "", "")
	fs.String("out", "", "")
	fs.Duration("watch-interval", 0, "")
	fs.Duration("watch-idle-timeout", 0, "")
	_ = fs.Parse(argv[1:]) // ignore parse errors; *watch holds what parsed
	return *watch
}

// reachableDaemonURL reports a verified-reachable daemon URL, resolving it the
// same way the rest of the CLI does so doctor reports on the daemon the CLI
// would actually use. A pinned ANTIGRAVITY_DAEMON_URL wins outright —
// requireDaemonURL never falls back from it, so if it is set but unreachable
// doctor reports that (not some other auto-discovered port the CLI would never
// talk to). With no override, it auto-discovers from cli.log. Both paths confirm
// reachability with a dial, so a returned URL is safe to report as "reachable".
// Returns "" and an error when the resolved daemon is unreachable.
func reachableDaemonURL(root string) (string, error) {
	if v := strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL")); v != "" {
		if err := daemonReachable(v); err != nil {
			return "", fmt.Errorf("configured ANTIGRAVITY_DAEMON_URL %s unreachable: %w", v, err)
		}
		return v, nil
	}
	return discovery.DiscoverDaemonURL(root)
}

// daemonReachable dials the host:port of a daemon base URL to confirm something
// is listening, matching the reachability check discovery.DiscoverDaemonURL
// makes for an auto-discovered port.
func daemonReachable(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if u.Host == "" {
		return fmt.Errorf("no host in daemon URL %q", rawURL)
	}
	conn, err := net.DialTimeout("tcp", u.Host, 150*time.Millisecond)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// buildDoctorReport gathers daemon reachability, agy-version compatibility, and
// sidecar coverage for root into a doctorReport.
func buildDoctorReport(root string) doctorReport {
	r := doctorReport{
		recordedVer: recordedAgyVersion(),
		agyVer:      agyVersion(),
		pinnedURL:   strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL")),
	}
	if url, err := reachableDaemonURL(root); err == nil {
		r.daemonURL = url
	}
	r.total, r.fresh, r.stale, r.coverageErr = sidecarCoverage(root)
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
