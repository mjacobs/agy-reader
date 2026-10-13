package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
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
	surface                                            discovery.Surface // which daemon serves the root; "" renders as cli
	root                                               string            // session root the report was built for
	daemonURL                                          string            // resolved reachable daemon URL; "" when unreachable
	pinnedURL                                          string            // ANTIGRAVITY_DAEMON_URL value, if the user pinned one
	csrfFound                                          bool              // a CSRF token was discovered (or pinned) for the daemon
	authRejected                                       bool              // reachable CLI endpoint rejected a read-only RPC
	agyVer                                             string            // "" when agy not found
	recordedVer                                        string
	total                                              int
	fresh                                              int
	stale                                              int
	sqlite, verified, incomplete, unknown, legacyFresh int
	unsupported                                        int
	unreadable                                         int    // SQLite sources that could not be read now (lock, access)
	unreadableReason                                   string // the first unreadable source's error
	verifiedSourceOnly                                 int    // verified with declared source-only fields; fresh, not in verified
	coverageErr                                        error  // non-nil when sidecar coverage could not be read
	watchRunning                                       bool
	watchKnown                                         bool
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
	authRejected := false
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
		authRejected = authRejected || r.authRejected || r.incomplete > 0 || r.unknown > 0
	}
	fail := unhealthy == len(reports)
	if explicit {
		fail = unhealthy > 0
	}
	// A reachable daemon rejecting credentials is blocked, not waiting for
	// the user to open another surface. A healthy IDE must not hide it.
	fail = fail || authRejected
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

	if r.authRejected {
		problems++
		fmt.Fprintln(w, "  auth:        rejected — sync blocked; check CLI token discovery or ANTIGRAVITY_CSRF_TOKEN")
	} else if surface == discovery.SurfaceCLI && r.csrfFound {
		fmt.Fprintln(w, "  csrf:        token found (sent as x-codeium-csrf-token)")
	}

	// Older CLI daemons need no token; IDE roots always report discovery.
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
		if r.sqlite > 0 {
			// Sources verified with declared source-only fields are counted on
			// their own; the count appears only when there is one.
			sourceOnly := ""
			if r.verifiedSourceOnly > 0 {
				sourceOnly = fmt.Sprintf(" %d verified with source-only fields,", r.verifiedSourceOnly)
			}
			fmt.Fprintf(w, "  sidecars:    SQLite %d verified,%s %d incomplete, %d unknown (supported daemon projection)\n", r.verified, sourceOnly, r.incomplete, r.unknown)
			fmt.Fprintf(w, "               legacy .pb: %d/%d fresh by mtime (completeness unknown)\n", r.legacyFresh, r.total-r.sqlite)
		} else {
			fmt.Fprintf(w, "  sidecars:    %d/%d fresh (legacy .pb mtime; completeness unknown)\n", r.fresh, r.total)
		}
		if r.unsupported > 0 {
			problems++
			fmt.Fprintf(w, "               %d unsupported -> needs an agy-reader compatibility update (see README)\n", r.unsupported)
		}
		if r.unreadable > 0 {
			// Neither watch nor an update is the first step: the source
			// itself could not be read, most often because agy held an
			// exclusive lock while writing. Nothing is known about support.
			problems++
			reason := ""
			if r.unreadableReason != "" {
				reason = " (" + r.unreadableReason + ")"
			}
			fmt.Fprintf(w, "               %d unreadable -> could not read the conversation database%s; rerun doctor once agy is idle, or check file access\n", r.unreadable, reason)
		}
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
// process that covers root, resolved the way that watcher itself would (see
// watchProcCoverage). Returns (running, known); known=false where detection
// isn't supported, or when the only candidate watchers had indeterminate
// coverage (their environment could not be read).
func watchRunningForRoot(root string) (running, known bool) {
	if runtime.GOOS != "linux" {
		return false, false
	}
	return scanProcForWatch(root)
}

// watchCoversRoot reports whether a watcher refreshes root, mirroring run()'s
// own resolution order: explicit --root argv values win (run() ignores the
// env when --root is passed); otherwise an ANTIGRAVITY_CLI_ROOT pin read
// from the watcher's environment covers exactly that root; a truly bare
// watcher covers the default store roots.
func watchCoversRoot(explicitRoots []string, envRoot, root string, defaultRoots []string) bool {
	root = filepath.Clean(root)
	covered := defaultRoots
	switch {
	case len(explicitRoots) > 0:
		covered = explicitRoots
	case envRoot != "":
		covered = []string{envRoot}
	}
	for _, r := range covered {
		if filepath.Clean(r) == root {
			return true
		}
	}
	return false
}

// watchProcCoverage decides one watch process's coverage of root from its
// argv and its OWN environment — never this process's env or home, which may
// differ from the watcher's. Coverage is indeterminate (not assumed) when
// the environment is needed but unavailable: environ unreadable, or a bare
// watcher whose HOME is absent.
func watchProcCoverage(explicitRoots []string, environ []byte, environErr error, root string) (covered, indeterminate bool) {
	if len(explicitRoots) > 0 {
		return watchCoversRoot(explicitRoots, "", root, nil), false
	}
	if environErr != nil {
		return false, true
	}
	if envRoot := envValueFromProcEnviron(environ, "ANTIGRAVITY_CLI_ROOT"); envRoot != "" {
		return watchCoversRoot(nil, envRoot, root, nil), false
	}
	home := envValueFromProcEnviron(environ, "HOME")
	if home == "" {
		return false, true
	}
	return watchCoversRoot(nil, "", root, discovery.DefaultStoreRootsUnder(home)), false
}

// envValueFromProcEnviron extracts one variable's value from a NUL-separated
// /proc/<pid>/environ blob, or "" when unset.
func envValueFromProcEnviron(data []byte, name string) string {
	prefix := name + "="
	for _, kv := range strings.Split(string(data), "\x00") {
		if strings.HasPrefix(kv, prefix) {
			return kv[len(prefix):]
		}
	}
	return ""
}

// scanProcForWatch walks /proc looking for another agy-reader process invoked
// with --watch that covers root (see watchProcCoverage). It skips our own PID
// and ignores unreadable entries. A failure to read /proc itself yields
// (false, true): detection is supported here but nothing was found. When no
// watcher covers root but at least one had indeterminate coverage, the
// result is (false, false) — "unknown" is more honest than a guess.
func scanProcForWatch(root string) (running, known bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false, true
	}
	self := os.Getpid()
	sawIndeterminate := false
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue // not a PID dir, or our own process
		}
		data, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil {
			continue // process may have exited, or we lack permission
		}
		explicitRoots, ok := cmdlineWatchRoots(data)
		if !ok {
			continue
		}
		// environ is readable for same-uid watchers; a different-uid watcher
		// yields an error and indeterminate coverage.
		environ, environErr := os.ReadFile(filepath.Join("/proc", e.Name(), "environ"))
		covered, indeterminate := watchProcCoverage(explicitRoots, environ, environErr, root)
		if covered {
			return true, true
		}
		if indeterminate {
			sawIndeterminate = true
		}
	}
	return false, !sawIndeterminate
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

// reachableDaemonConnection reports a verified-reachable daemon endpoint and
// the token discovered alongside it, resolving both the same way the rest of
// the CLI does so doctor reports on the daemon the CLI would actually use. A
// pinned ANTIGRAVITY_DAEMON_URL wins outright — requireDaemonConnection never
// falls back from it, so if it is set but unreachable doctor reports that (not
// some other auto-discovered port the CLI would never talk to). With no
// override, it auto-discovers from cli.log. Both paths confirm reachability
// with a dial, so a returned URL is safe to report as "reachable".
//
// The token travels with the endpoint because CLI credentials live in
// short-lived tool processes: rescanning for a token after the endpoint scan
// can miss a process that has since exited, which would make doctor report a
// missing or rejected credential that the CLI itself would have found.
// Returns a zero Connection and an error when the resolved daemon is unreachable.
func reachableDaemonConnection(root string) (discovery.Connection, error) {
	if v := strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL")); v != "" {
		if err := daemonReachable(v); err != nil {
			return discovery.Connection{}, fmt.Errorf("configured ANTIGRAVITY_DAEMON_URL %s unreachable: %w", v, err)
		}
		return discovery.Connection{BaseURL: v}, nil
	}
	return discovery.DiscoverConnection(root)
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
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("daemon URL %q is not http or https", rawURL)
	}
	conn, err := net.DialTimeout("tcp", daemonDialAddr(u), 150*time.Millisecond)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// daemonDialAddr is the host:port to probe for a daemon URL. A URL without an
// explicit port uses its scheme's default, so the probe does not fail on a
// reachable daemon with "missing port in address".
func daemonDialAddr(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}
	port := "80"
	if u.Scheme == "https" {
		port = "443"
	}
	return net.JoinHostPort(u.Hostname(), port)
}

// clientReachesDaemon checks the daemon the way client's RPCs reach it. A
// direct connection gets the plain dial of daemonReachable. When the client's
// transport sends the daemon URL through a proxy (Go's default transport
// honours HTTP_PROXY), a direct dial proves nothing either way. A status code
// proves nothing either: a proxy can answer on its own with anything, a login
// page served as 200 included, and a 401 or 403 of its own looks the same as
// the daemon refusing the token. So the probe is a real RPC that only the
// daemon can answer, sent through the client with its CSRF token, and it
// counts as reachable only when it decodes as that RPC's response (see
// daemon.Client.CheckResponds). An authentication refusal is a failure like
// any other, though the error still wraps daemon.ErrAuthentication. That RPC
// lists conversations without loading any, so a held source is still not
// fetched again. Redirects are not followed, so a proxy cannot send the probe
// and its token to a page of its own.
func clientReachesDaemon(ctx context.Context, client *daemon.Client) error {
	u, err := url.Parse(client.BaseURL)
	if err != nil || !daemonProxied(client.HTTP, u) {
		return daemonReachable(client.BaseURL)
	}
	if err := clientRPCResponds(ctx, client); err != nil {
		return fmt.Errorf("daemon %s not reached through its proxy: %w", client.BaseURL, err)
	}
	return nil
}

// clientRPCResponds sends the daemon.Client.CheckResponds probe through
// client, with a short timeout and without following redirects, so a proxy
// cannot send the probe and its token to a page of its own. It passes only
// when the daemon itself answers its listing RPC.
func clientRPCResponds(ctx context.Context, client *daemon.Client) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	httpClient := http.Client{}
	if client.HTTP != nil {
		httpClient = *client.HTTP
	}
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	probe := *client
	probe.HTTP = &httpClient
	return probe.CheckResponds(ctx)
}

// clientProxied reports whether client's RPCs go through a proxy.
func clientProxied(client *daemon.Client) bool {
	u, err := url.Parse(client.BaseURL)
	return err == nil && daemonProxied(client.HTTP, u)
}

// daemonProxied reports whether httpClient's transport would send a request
// for u through a proxy. Only *http.Transport exposes that decision; any other
// round tripper is treated as a direct connection. A failing proxy lookup
// counts as proxied, so the probe fails the same way the RPC would.
func daemonProxied(httpClient *http.Client, u *url.URL) bool {
	rt := http.DefaultTransport
	if httpClient != nil && httpClient.Transport != nil {
		rt = httpClient.Transport
	}
	t, ok := rt.(*http.Transport)
	if !ok || t.Proxy == nil {
		return false
	}
	proxyURL, err := t.Proxy(&http.Request{URL: u})
	return err != nil || proxyURL != nil
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
	}
	var conn discovery.Connection
	if discovered, err := reachableDaemonConnection(root); err == nil {
		conn = discovered
		r.daemonURL = conn.BaseURL
	}
	client := newDaemonClient(root, conn)
	r.csrfFound = client.CSRFToken != ""
	if r.surface == discovery.SurfaceCLI && r.daemonURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		r.authRejected = errors.Is(client.CheckAuthentication(ctx), daemon.ErrAuthentication)
		cancel()
	}
	populateSidecarCoverage(&r, root)
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
	r := doctorReport{}
	populateSidecarCoverage(&r, root)
	return r.total, r.fresh, r.stale, r.coverageErr
}

// Use one session enumeration and one content check per source so counters
// always describe the same observation, even while a conversation is active.
func populateSidecarCoverage(r *doctorReport, root string) {
	sessions, err := discovery.ListConversationSessions(root)
	r.coverageErr = err
	if err != nil {
		return
	}
	for _, s := range sessions {
		r.total++
		fresh := false
		var result completeness.Result
		class := sourceRepairable
		if strings.HasSuffix(s.PBPath, ".db") {
			r.sqlite++
			result, class = sessionCompletenessClassified(s)
			switch {
			case sourceOnlyVerified(string(result.State), result.Reason):
				r.verifiedSourceOnly++
				fresh = true
			case result.State == completeness.Verified:
				r.verified++
				fresh = true
			case result.State == completeness.Incomplete:
				r.incomplete++
			default:
				r.unknown++
				switch class {
				case sourceUnsupported:
					r.unsupported++
				case sourceUnreadable:
					if r.unreadable == 0 {
						r.unreadableReason = result.Reason
					}
					r.unreadable++
				}
			}
		} else {
			stale, _ := isStale(s)
			fresh = !stale
			if fresh {
				r.legacyFresh++
			}
		}
		switch {
		case fresh:
			r.fresh++
		case class == sourceRepairable:
			r.stale++
		}
	}
}
