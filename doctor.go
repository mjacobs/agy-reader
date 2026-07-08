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
	surface      discovery.Surface // which daemon serves the root; "" renders as cli
	root         string            // session root the report was built for
	daemonURL    string            // resolved reachable daemon URL; "" when unreachable
	pinnedURL    string            // ANTIGRAVITY_DAEMON_URL value, if the user pinned one
	csrfFound    bool              // a CSRF token was discovered (or pinned) for the daemon
	agyVer       string            // "" when agy not found
	recordedVer  string
	total        int
	fresh        int
	stale        int
	coverageErr  error // non-nil when sidecar coverage could not be read
	watchRunning bool
	watchKnown   bool
}

// writeDoctorReport prints the human-readable single-root report (body plus
// exit line) and returns the number of actionable problems (0 = healthy).
func writeDoctorReport(w io.Writer, r doctorReport) int {
	problems := writeDoctorReportBody(w, r)
	writeDoctorExitLine(w, problems > 0, 0)
	return problems
}

// writeMultiDoctorReport renders one report block per root (with a root
// header when more than one root is in play) and a single exit line, and
// returns the process exit code. Exit semantics are the multi-root crux:
// explicitly requested roots are hard requirements, so any unhealthy one
// fails; discovered roots soft-fail — an unhealthy root there is WAITING
// (e.g. its daemon is down while another surface works) and only
// all-roots-unhealthy fails the run.
func writeMultiDoctorReport(w io.Writer, reports []doctorReport, explicit bool) int {
	multi := len(reports) > 1
	unhealthy := 0
	for i, r := range reports {
		if multi {
			if i > 0 {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "%s:\n", r.root)
		}
		if writeDoctorReportBody(w, r) > 0 {
			unhealthy++
		}
	}
	fail := unhealthy == len(reports)
	if explicit {
		fail = unhealthy > 0
	}
	waiting := 0
	if !fail {
		waiting = unhealthy
	}
	writeDoctorExitLine(w, fail, waiting)
	if fail {
		return 1
	}
	return 0
}

// writeDoctorExitLine prints the trailing exit summary. waiting counts
// discovered roots that have findings but do not fail the run (soft-fail);
// it is only ever non-zero on a passing multi-root report.
func writeDoctorExitLine(w io.Writer, fail bool, waiting int) {
	switch {
	case fail:
		fmt.Fprintf(w, "\n  exit 1 — action needed\n")
	case waiting > 0:
		fmt.Fprintf(w, "\n  exit 0 — healthy (%d root(s) waiting, see above)\n", waiting)
	default:
		fmt.Fprintf(w, "\n  exit 0 — healthy\n")
	}
}

// writeDoctorReportBody prints one root's report lines (no exit line) and
// returns the number of actionable problems (0 = healthy). Stale sidecars
// and a version skew are actionable; an unreachable daemon alone is
// informational.
func writeDoctorReportBody(w io.Writer, r doctorReport) int {
	problems := 0

	// surface: which Antigravity product's daemon serves this root. The zero
	// value renders as the CLI so hand-built reports stay valid.
	surface := r.surface
	if surface == "" {
		surface = discovery.SurfaceCLI
	}
	fmt.Fprintf(w, "  surface:     %s\n", surface)

	// daemon
	daemonHint := "start `agy` to refresh sidecars"
	if surface == discovery.SurfaceIDE {
		daemonHint = "start the Antigravity 2.0 IDE to refresh sidecars"
	}
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
		fmt.Fprintf(w, "  daemon:      not running (%s)\n", daemonHint)
	}

	// csrf — only meaningful for the IDE daemon, which is launched with
	// --csrf_token and rejects RPCs missing the header. The CLI daemon takes
	// no token, so the line would be noise there.
	if surface == discovery.SurfaceIDE {
		switch {
		case r.csrfFound:
			fmt.Fprintf(w, "  csrf:        token found (sent as x-codeium-csrf-token)\n")
		case r.daemonURL != "":
			// Daemon reachable but no token: every RPC will be rejected, and
			// that never self-heals without the token, so it is actionable.
			problems++
			fmt.Fprintf(w, "  csrf:        no token found -> IDE daemon rejects RPCs; set ANTIGRAVITY_CSRF_TOKEN\n")
		default:
			// No daemon and no token is just "the IDE is closed": discovery
			// reads the token from the IDE's main.log once it runs again.
			fmt.Fprintf(w, "  csrf:        no token found (discovered from the IDE's main.log when it runs)\n")
		}
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
			// --sync needs a cascade id, so it cannot batch-fix from here. A
			// bare `agy-reader --watch` targets the default CLI root, so the
			// IDE suggestion must carry the --root.
			watchCmd := "agy-reader --watch"
			if surface == discovery.SurfaceIDE && r.root != "" {
				watchCmd = "agy-reader --root " + r.root + " --watch"
			}
			switch {
			case r.daemonURL != "":
				fmt.Fprintf(w, "               %d missing/stale -> run: %s\n", r.stale, watchCmd)
			case r.pinnedURL != "":
				// --watch would reuse the same dead pinned URL, so fixing the
				// env var is the real first step.
				fmt.Fprintf(w, "               %d missing/stale -> fix or unset ANTIGRAVITY_DAEMON_URL, then: %s\n", r.stale, watchCmd)
			default:
				starter := "agy"
				if surface == discovery.SurfaceIDE {
					starter = "the Antigravity 2.0 IDE"
				}
				fmt.Fprintf(w, "               %d missing/stale -> start %s, then: %s\n", r.stale, starter, watchCmd)
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

	return problems
}

// watchRunningForRoot best-effort detects a separate `agy-reader --watch`
// process that covers root: one launched with that --root explicitly, or a
// bare watcher (which operates on the discovered default roots). Returns
// (running, known); known=false where detection isn't supported.
func watchRunningForRoot(root string) (running, known bool) {
	if runtime.GOOS != "linux" {
		return false, false
	}
	// A bare watcher's coverage is what ITS bare invocation would resolve —
	// the literal default stores — not what this process's ANTIGRAVITY_CLI_ROOT
	// pin would resolve to.
	defaults, err := discovery.DefaultStoreRoots()
	if err != nil {
		defaults = nil
	}
	return scanProcForWatch(func(explicitRoots []string) bool {
		return watchCoversRoot(explicitRoots, root, defaults)
	})
}

// watchCoversRoot reports whether a watcher launched with explicitRoots (its
// --root argv values) refreshes root. A watcher with no --root operates on
// the discovered default roots. Detection is argv-based: a watcher pointed
// elsewhere via an ANTIGRAVITY_CLI_ROOT env var is not visible here and is
// treated as a bare watcher.
func watchCoversRoot(explicitRoots []string, root string, defaultRoots []string) bool {
	root = filepath.Clean(root)
	covered := defaultRoots
	if len(explicitRoots) > 0 {
		covered = explicitRoots
	}
	for _, r := range covered {
		if filepath.Clean(r) == root {
			return true
		}
	}
	return false
}

// scanProcForWatch walks /proc looking for another agy-reader process invoked
// with --watch whose roots satisfy covers. It skips our own PID and ignores
// unreadable entries. A failure to read /proc itself yields (false, true):
// detection is supported here but nothing was found.
func scanProcForWatch(covers func(explicitRoots []string) bool) (running, known bool) {
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
		if explicitRoots, ok := cmdlineWatchRoots(data); ok && covers(explicitRoots) {
			return true, true
		}
	}
	return false, true
}

// cmdlineIsWatch reports whether a /proc cmdline (NUL-separated argv) is an
// agy-reader process running the --watch loop.
func cmdlineIsWatch(data []byte) bool {
	_, ok := cmdlineWatchRoots(data)
	return ok
}

// cmdlineWatchRoots reports whether a /proc cmdline (NUL-separated argv) is
// an agy-reader process running the --watch loop, and if so which --root
// values it was launched with (empty = a bare watcher on the default roots).
// It matches on argv structure rather than a loose substring scan so that an
// unrelated process merely mentioning these strings (a shell echoing a
// command, an editor, another `agy-reader doctor`) is not misreported as a
// running watcher.
//
// The args are parsed with a FlagSet mirroring run()'s definitions, so the
// boolean value is honored (--watch=false is not a watcher) and an args layout
// the real flag parser would stop at — a flag after a positional, or after a
// "--" terminator — is treated the same way the live process would treat it.
func cmdlineWatchRoots(data []byte) (explicitRoots []string, isWatch bool) {
	argv := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	if len(argv) == 0 || filepath.Base(argv[0]) != "agy-reader" {
		return nil, false
	}
	fs := flag.NewFlagSet("agy-reader", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	watch := fs.Bool("watch", false, "")
	var roots rootsFlag
	fs.Var(&roots, "root", "")
	// Remaining flags exist only so parsing matches run(): the value-taking
	// ones (string/duration) must be declared or their values would be
	// misread as positionals and stop the scan early.
	fs.Bool("list", false, "")
	fs.Bool("include-implicit", false, "")
	fs.Bool("sync", false, "")
	fs.String("format", "", "")
	fs.String("out", "", "")
	fs.Duration("watch-interval", 0, "")
	fs.Duration("watch-idle-timeout", 0, "")
	_ = fs.Parse(argv[1:]) // ignore parse errors; the flags hold what parsed
	if !*watch {
		return nil, false
	}
	return roots, true
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
		surface:     discovery.DetectSurface(root),
		root:        root,
		recordedVer: recordedAgyVersion(),
		agyVer:      agyVersion(),
		pinnedURL:   strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL")),
		csrfFound:   discovery.DiscoverCSRFToken(root) != "",
	}
	if url, err := reachableDaemonURL(root); err == nil {
		r.daemonURL = url
	}
	r.total, r.fresh, r.stale, r.coverageErr = sidecarCoverage(root)
	r.watchRunning, r.watchKnown = watchRunningForRoot(root)
	return r
}

// runDoctorTo renders the doctor report for roots to w and returns the
// intended process exit code. explicit says whether the roots were requested
// via --root/env (hard requirements) or discovered (soft-fail). It is the
// testable seam for runDoctor.
func runDoctorTo(w io.Writer, roots []string, explicit bool) int {
	reports := make([]doctorReport, 0, len(roots))
	for _, root := range roots {
		reports = append(reports, buildDoctorReport(root))
	}
	return writeMultiDoctorReport(w, reports, explicit)
}

// runDoctor prints the doctor report to stdout and exits non-zero when there is
// something actionable, so callers can gate on `agy-reader doctor`'s exit code.
func runDoctor(roots []string, explicit bool) error {
	if runDoctorTo(os.Stdout, roots, explicit) != 0 {
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
