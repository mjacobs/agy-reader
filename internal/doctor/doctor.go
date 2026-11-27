package doctor

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/session"
	"github.com/mjacobs/agy-reader/internal/watchlock"
)

//go:embed COMPATIBILITY.md
var compatibilityMD string

// ParseRecordedAgyVersion extracts the version from the
// "- **agy version:** X.Y.Z" line of COMPATIBILITY.md.
func ParseRecordedAgyVersion(md string) string {
	const marker = "**agy version:**"
	for _, line := range strings.Split(md, "\n") {
		if i := strings.Index(line, marker); i >= 0 {
			return strings.TrimSpace(line[i+len(marker):])
		}
	}
	return ""
}

func RecordedAgyVersion() string { return ParseRecordedAgyVersion(compatibilityMD) }

// AgyVersion returns the running `agy` version (first line of `agy --version`),
// or "" if agy is not on PATH or errors.
func AgyVersion() string {
	out, err := exec.Command("agy", "--version").Output()
	if err != nil {
		return ""
	}
	if i := strings.IndexByte(string(out), '\n'); i >= 0 {
		return strings.TrimSpace(string(out)[:i])
	}
	return strings.TrimSpace(string(out))
}

type Report struct {
	Surface                               discovery.Surface // which daemon serves the root; "" renders as cli
	Root                                  string            // session root the report was built for
	DaemonURL                             string            // resolved reachable daemon URL; "" when unreachable
	PinnedURL                             string            // ANTIGRAVITY_DAEMON_URL value, if the user pinned one
	CSRFFound                             bool              // a CSRF token was discovered (or pinned) for the daemon
	PinnedCSRF                            bool              // ANTIGRAVITY_CSRF_TOKEN is explicitly pinned
	AuthRejected                          bool              // reachable CLI endpoint rejected a read-only RPC
	AgyVer                                string            // "" when agy not found
	RecordedVer                           string
	Total                                 int
	Fresh                                 int
	Stale                                 int
	StaleIDs                              []string // up to 5 missing/stale cascade IDs
	SQLite, Verified, Incomplete, Unknown int
	SavedUnverified                       int
	UnreadableSidecars                    int
	Unsupported                           int
	UnsupportedSessions                   []string // up to 5 unsupported sessions ("<cascade-id> (<reason>)")
	Unreadable                            int      // Sources that could not be read now (lock, access)
	UnreadableReason                      string   // the first unreadable source's error
	VerifiedTranscript                    int
	VerifiedSourceOnly                    int   // verified with declared source-only fields; fresh, not in verified
	CoverageErr                           error // non-nil when sidecar coverage could not be read
	WatchRunning                          bool
	WatchKnown                            bool
}

// WriteReport prints the human-readable single-root report (body plus
// exit line) and returns the number of actionable problems (0 = healthy).
func WriteReport(w io.Writer, r Report) int {
	problems := WriteReportBody(w, r)
	WriteExitLine(w, problems > 0, 0)
	return problems
}

// WriteMultiReport renders one report block per root (with a root
// header when more than one root is in play) and a single exit line, and
// returns the process exit code. Exit semantics are the multi-root crux:
// explicitly requested roots are hard requirements, so any unhealthy one
// fails; discovered roots soft-fail — an unhealthy root there is WAITING
// (e.g. its daemon is down while another surface works) and only
// all-roots-unhealthy fails the run.
func WriteMultiReport(w io.Writer, reports []Report, explicit bool) int {
	multi := len(reports) > 1
	unhealthy := 0
	authRejected := false
	for i, r := range reports {
		if multi {
			if i > 0 {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "%s:\n", r.Root)
		}
		if WriteReportBody(w, r) > 0 {
			unhealthy++
		}
		authRejected = authRejected || r.AuthRejected || r.Incomplete > 0 || r.Unknown > 0 || r.SavedUnverified > 0 || r.UnreadableSidecars > 0
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
	WriteExitLine(w, fail, waiting)
	if fail {
		return 1
	}
	return 0
}

// WriteExitLine prints the trailing exit summary. waiting counts
// discovered roots that have findings but do not fail the run (soft-fail);
// it is only ever non-zero on a passing multi-root report.
func WriteExitLine(w io.Writer, fail bool, waiting int) {
	switch {
	case fail:
		fmt.Fprintf(w, "\n  exit 1 — action needed\n")
	case waiting > 0:
		fmt.Fprintf(w, "\n  exit 0 — healthy (%d root(s) waiting, see above)\n", waiting)
	default:
		fmt.Fprintf(w, "\n  exit 0 — healthy\n")
	}
}

// WriteReportBody prints one root's report lines (no exit line) and
// returns the number of actionable problems (0 = healthy). Stale sidecars
// and a version skew are actionable; an unreachable daemon alone is
// informational.
func WriteReportBody(w io.Writer, r Report) int {
	problems := 0

	// surface: which Antigravity product's daemon serves this root. The zero
	// value renders as the CLI so hand-built reports stay valid.
	surface := r.Surface
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
	case r.DaemonURL != "":
		fmt.Fprintf(w, "  daemon:      reachable (%s)\n", r.DaemonURL)
	case r.PinnedURL != "":
		// A pinned ANTIGRAVITY_DAEMON_URL the CLI would use, but nothing is
		// listening on it. Unlike agy simply being closed (which auto-discovery
		// recovers from on restart), a stale pin never self-heals — agy binds a
		// new random port each start — so the CLI/watch paths keep failing.
		// That is an actionable misconfiguration, not informational.
		problems++
		fmt.Fprintf(w, "  daemon:      configured ANTIGRAVITY_DAEMON_URL (%s) unreachable -> fix or unset it\n", r.PinnedURL)
	default:
		fmt.Fprintf(w, "  daemon:      not running (%s)\n", daemonHint)
	}

	hasLiveOrPinnedCSRF := r.CSRFFound && (r.DaemonURL != "" || r.PinnedCSRF)

	if r.AuthRejected {
		problems++
		fmt.Fprintln(w, "  auth:        rejected — sync blocked; check CLI token discovery or ANTIGRAVITY_CSRF_TOKEN")
	} else if surface == discovery.SurfaceCLI && hasLiveOrPinnedCSRF {
		fmt.Fprintln(w, "  csrf:        token found (sent as x-codeium-csrf-token)")
	}

	// Older CLI daemons need no token; IDE roots always report discovery.
	if surface == discovery.SurfaceIDE {
		switch {
		case hasLiveOrPinnedCSRF:
			fmt.Fprintf(w, "  csrf:        token found (sent as x-codeium-csrf-token)\n")
		case r.DaemonURL != "":
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
	case r.AgyVer == "":
		fmt.Fprintf(w, "  agy version: unknown (agy not on PATH)\n")
	case r.RecordedVer != "" && r.AgyVer != r.RecordedVer:
		fmt.Fprintf(w, "  agy version: %s  (recorded %s — re-run agy-format-audit)\n",
			r.AgyVer, r.RecordedVer)
		problems++
	default:
		fmt.Fprintf(w, "  agy version: %s  (compatible)\n", r.AgyVer)
	}

	// sidecars
	if r.UnreadableSidecars > 0 {
		problems++
		fmt.Fprintf(w, "  transcripts: %d unreadable -> check sidecar file access\n", r.UnreadableSidecars)
	}
	if r.SavedUnverified > 0 {
		problems++
		fmt.Fprintf(w, "  transcripts: %d saved unverified (verification needs a supported source and daemon)\n", r.SavedUnverified)
	}
	switch {
	case r.CoverageErr != nil:
		// Could not enumerate sessions (e.g. an unreadable conversations dir).
		// Report it as actionable rather than implying a healthy 0/0.
		problems++
		fmt.Fprintf(w, "  sidecars:    unknown (could not read sessions: %v)\n", r.CoverageErr)
	default:
		sourceOnly := ""
		if r.VerifiedSourceOnly > 0 {
			sourceOnly = fmt.Sprintf(" %d verified with source-only fields,", r.VerifiedSourceOnly)
		}
		if r.VerifiedTranscript > 0 {
			sourceOnly += fmt.Sprintf(" %d verified transcripts,", r.VerifiedTranscript)
		}
		label := "SQLite"
		if r.Total > r.SQLite {
			label = fmt.Sprintf("native (%d SQLite, %d legacy .pb)", r.SQLite, r.Total-r.SQLite)
		}
		fmt.Fprintf(w, "  sidecars:    %s %d verified,%s %d incomplete, %d unknown (supported daemon projection)\n", label, r.Verified, sourceOnly, r.Incomplete, r.Unknown)

		if r.Unsupported > 0 {
			problems++
			fmt.Fprintf(w, "               %d unsupported -> needs an agy-reader compatibility update (see README)\n", r.Unsupported)
			writeReportedList(w, r.UnsupportedSessions, r.Unsupported)
		}
		if r.Unreadable > 0 {
			// Neither watch nor an update is the first step: the source
			// itself could not be read, most often because agy held an
			// exclusive lock while writing. Nothing is known about support.
			problems++
			reason := ""
			if r.UnreadableReason != "" {
				reason = " (" + r.UnreadableReason + ")"
			}
			fmt.Fprintf(w, "               %d unreadable -> could not read the conversation database%s; rerun doctor once agy is idle, or check file access\n", r.Unreadable, reason)
		}
		if r.Stale > 0 {
			problems++
			// --watch refreshes every missing/stale sidecar in one pass; bare
			// --sync needs a cascade id, so it cannot batch-fix from here. A
			// bare `agy-reader --watch` targets the default CLI root, so the
			// IDE suggestion must carry the --root.
			watchCmd := "agy-reader --watch"
			if surface == discovery.SurfaceIDE && r.Root != "" {
				watchCmd = "agy-reader --root " + r.Root + " --watch"
			}
			switch {
			case r.DaemonURL != "":
				fmt.Fprintf(w, "               %d missing/stale -> run: %s\n", r.Stale, watchCmd)
			case r.PinnedURL != "":
				// --watch would reuse the same dead pinned URL, so fixing the
				// env var is the real first step.
				fmt.Fprintf(w, "               %d missing/stale -> fix or unset ANTIGRAVITY_DAEMON_URL, then: %s\n", r.Stale, watchCmd)
			default:
				starter := "agy"
				if surface == discovery.SurfaceIDE {
					starter = "the Antigravity 2.0 IDE"
				}
				fmt.Fprintf(w, "               %d missing/stale -> start %s, then: %s\n", r.Stale, starter, watchCmd)
			}
			writeReportedList(w, r.StaleIDs, r.Stale)
		}
	}

	// watch
	switch {
	case !r.WatchKnown:
		fmt.Fprintf(w, "  watch:       unknown (optional: agy-reader --watch)\n")
	case r.WatchRunning:
		fmt.Fprintf(w, "  watch:       running\n")
	default:
		fmt.Fprintf(w, "  watch:       not running (optional: agy-reader --watch)\n")
	}

	return problems
}

const maxReportedSessions = 5

func writeReportedList(w io.Writer, items []string, total int) {
	if len(items) == 0 {
		return
	}
	shown := items
	if len(shown) > maxReportedSessions {
		shown = shown[:maxReportedSessions]
	}
	for _, item := range shown {
		fmt.Fprintf(w, "                 %s\n", item)
	}
	if total > len(shown) {
		fmt.Fprintf(w, "                 (+%d more)\n", total-len(shown))
	}
}

// watchLockName is the file every watcher takes an exclusive system lock on in
// each store it syncs, and keeps open until it stops syncing that store (see
// internal/watcher/lock.go). Doctor reads it and never creates or locks it. It
// has to stay the same name as the watcher's; the root package's end-to-end
// doctor tests run a real watcher and fail if the two drift apart.
const watchLockName = ".agy-reader-watch.lock"

// WatchRunningForRoot detects a separate Linux watch process polling root.
// It requires a held flock and the polling path recorded by the watcher.
func WatchRunningForRoot(root string) (running, known bool) {
	if runtime.GOOS != "linux" {
		return false, false
	}
	return ScanProcForWatch(root)
}

// WatchCoversRoot reports whether a watcher refreshes root, mirroring run()'s
// own resolution order: explicit --root argv values win (run() ignores the
// env when --root is passed); otherwise an ANTIGRAVITY_CLI_ROOT pin read
// from the watcher's environment covers exactly that root; a truly bare
// watcher covers the default store roots.
func WatchCoversRoot(explicitRoots []string, envRoot, root string, defaultRoots []string) bool {
	covered := defaultRoots
	switch {
	case len(explicitRoots) > 0:
		covered = explicitRoots
	case envRoot != "":
		covered = []string{envRoot}
	}
	for _, r := range covered {
		if sameRootPath(r, root) {
			return true
		}
	}
	return false
}

func sameRootPath(a, b string) bool {
	ca := filepath.Clean(a)
	cb := filepath.Clean(b)
	if ca == cb {
		return true
	}
	if !filepath.IsAbs(ca) || !filepath.IsAbs(cb) {
		return false
	}
	ra, errA := filepath.EvalSymlinks(ca)
	rb, errB := filepath.EvalSymlinks(cb)
	return errA == nil && errB == nil && ra == rb
}

// WatchProcCoverage decides one watch process's coverage of root from its
// argv and its OWN environment — never this process's env or home, which may
// differ from the watcher's. Coverage is indeterminate (not assumed) when
// the environment is needed but unavailable: environ unreadable, or a bare
// watcher whose HOME is absent.
func WatchProcCoverage(explicitRoots []string, environ []byte, environErr error, root string) (covered, indeterminate bool) {
	if len(explicitRoots) > 0 {
		return WatchCoversRoot(explicitRoots, "", root, nil), false
	}
	if environErr != nil {
		return false, true
	}
	if envRoot := envValueFromProcEnviron(environ, "ANTIGRAVITY_CLI_ROOT"); envRoot != "" {
		return WatchCoversRoot(nil, envRoot, root, nil), false
	}
	home := envValueFromProcEnviron(environ, "HOME")
	if home == "" {
		return false, true
	}
	return WatchCoversRoot(nil, "", root, discovery.DefaultStoreRootsUnder(home)), false
}

func envValueFromProcEnviron(data []byte, name string) string {
	prefix := name + "="
	for _, kv := range strings.Split(string(data), "\x00") {
		if strings.HasPrefix(kv, prefix) {
			return kv[len(prefix):]
		}
	}
	return ""
}

// ScanProcForWatch confirms that a watcher holds the store's lock and
// recorded a polling path that still reaches this store. Command line and
// environment can indicate expected coverage, but current default roots can
// differ from the paths the watcher opened. An older watcher without a
// matching record can hold a lock while its coverage remains unknown.
func ScanProcForWatch(root string) (running, known bool) {
	return scanProcForWatch("/proc", os.Getpid(), root, readPollingRoot)
}

func resolveLockPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

type pollingRootReader func(fdDir string, fds []os.DirEntry, lockFD string) string

// scanProcForWatch is ScanProcForWatch over the process tree at procDir, so a
// test can lay out processes without starting them.
func scanProcForWatch(procDir string, self int, root string, readRoot pollingRootReader) (running, known bool) {
	entries, err := os.ReadDir(procDir)
	if err != nil {
		return false, true
	}
	// The lock file is looked up, never created. A store with no lock file has
	// never been locked, so no watcher can hold it.
	lockPath := filepath.Join(root, watchLockName)
	lockInfo, lockErr := os.Stat(lockPath)
	var resolvedLock string
	if lockErr == nil {
		resolvedLock, lockErr = resolveLockPath(lockPath)
	}
	lockUnreadable := lockErr != nil && !errors.Is(lockErr, os.ErrNotExist)
	absRoot := root
	if !filepath.IsAbs(root) {
		if abs, err := filepath.Abs(root); err == nil {
			absRoot = abs
		}
	}
	sawPossibleOwner := false
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue // not a PID dir, or our own process
		}
		data, err := os.ReadFile(filepath.Join(procDir, e.Name(), "cmdline"))
		if err != nil {
			continue // process may have exited, or we lack permission
		}
		explicitRoots, ok := CmdlineWatchRoots(data)
		if !ok {
			continue
		}
		environ, environErr := os.ReadFile(filepath.Join(procDir, e.Name(), "environ"))
		covered, indeterminate := WatchProcCoverage(explicitRoots, environ, environErr, root)
		if !covered && !indeterminate && absRoot != root {
			covered, indeterminate = WatchProcCoverage(explicitRoots, environ, environErr, absRoot)
		}
		// A watcher with a different configured root and inaccessible file
		// descriptors does not make this store's status indeterminate.
		sawPossibleOwner = sawPossibleOwner || covered || indeterminate || lockUnreadable
		if lockErr != nil {
			continue
		}
		ownerRoot, held := watcherStoreLock(procDir, e.Name(), lockInfo, resolvedLock, readRoot)
		if held {
			if ownerRoot != "" && samePollingRoot(ownerRoot, absRoot) {
				return true, true
			}
			sawPossibleOwner = true
		}
	}
	return false, !sawPossibleOwner
}

// watcherStoreLock locates the exclusive flock by file identity and resolved
// path, then looks for the polling path associated with that descriptor.
func watcherStoreLock(procDir, pid string, lock os.FileInfo, resolvedLock string, readRoot pollingRootReader) (ownerRoot string, held bool) {
	fdDir := filepath.Join(procDir, pid, "fd")
	// ReadDir returns the entries read before an error along with the error;
	// a holder found among them is still a holder.
	fds, _ := os.ReadDir(fdDir)
	for _, fd := range fds {
		fdPath := filepath.Join(fdDir, fd.Name())
		target, err := os.Readlink(fdPath)
		if err != nil || !filepath.IsAbs(target) {
			continue // closed since the listing, or not a filesystem path
		}
		info, err := os.Stat(fdPath)
		if err != nil || !os.SameFile(info, lock) {
			continue
		}
		resolvedTarget, err := filepath.EvalSymlinks(target)
		if err != nil {
			continue
		}
		if resolvedTarget != resolvedLock {
			continue // hard link elsewhere
		}
		fdinfo, err := os.ReadFile(filepath.Join(procDir, pid, "fdinfo", fd.Name()))
		if err != nil {
			continue
		}
		if fdinfoHoldsFlock(fdinfo) {
			return readRoot(fdDir, fds, fd.Name()), true
		}
	}
	return "", false
}

// readPollingRoot reads the process-local record for a particular locked
// descriptor. Older watches, or ones without readable evidence, have no record.
func readPollingRoot(fdDir string, fds []os.DirEntry, lockFD string) string {
	name := "/memfd:" + watchlock.OwnerName(lockFD) + " (deleted)"
	for _, entry := range fds {
		path := filepath.Join(fdDir, entry.Name())
		link, err := os.Readlink(path)
		if err != nil || link != name {
			continue
		}
		return watchlock.ReadRoot(path)
	}
	return ""
}

// Resolve both paths now: the watcher still polls the recorded name, which
// might have disappeared or changed its symlink target since the lock was taken.
func samePollingRoot(pollingRoot, root string) bool {
	poll, err := filepath.EvalSymlinks(pollingRoot)
	if err != nil {
		return false
	}
	target, err := filepath.EvalSymlinks(root)
	return err == nil && poll == target
}

// fdinfoHoldsFlock reports whether /proc/<pid>/fdinfo/<fd> contents show an
// exclusive flock held (not waited for) on that open file. The kernel prints
// one line per lock, for example
//
//	lock:	1: FLOCK  ADVISORY  WRITE 4242 00:24:985401497 0 EOF
//
// where a lock still being waited for has "->" before its class.
func fdinfoHoldsFlock(fdinfo []byte) bool {
	for _, line := range strings.Split(string(fdinfo), "\n") {
		rest, ok := strings.CutPrefix(line, "lock:")
		if !ok {
			continue
		}
		f := strings.Fields(rest)
		if len(f) >= 4 && f[1] == "FLOCK" && f[3] == "WRITE" {
			return true
		}
	}
	return false
}

type stringSliceFlag []string

func (s *stringSliceFlag) String() string       { return strings.Join(*s, ", ") }
func (s *stringSliceFlag) Set(val string) error { *s = append(*s, val); return nil }

// CmdlineWatchRoots reports whether a /proc cmdline (NUL-separated argv) is
// an agy-reader process running the --watch loop, and if so which --root
// values it was launched with (empty = a bare watcher on the default roots).
func CmdlineWatchRoots(data []byte) (explicitRoots []string, isWatch bool) {
	argv := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	if len(argv) == 0 || filepath.Base(argv[0]) != "agy-reader" {
		return nil, false
	}
	fs := flag.NewFlagSet("agy-reader", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	watch := fs.Bool("watch", false, "")
	var roots stringSliceFlag
	fs.Var(&roots, "root", "")
	fs.Bool("list", false, "")
	fs.Bool("include-implicit", false, "")
	fs.Bool("sync", false, "")
	fs.String("format", "", "")
	fs.String("out", "", "")
	fs.Duration("watch-interval", 0, "")
	fs.Duration("watch-idle-timeout", 0, "")
	_ = fs.Parse(argv[1:])
	if !*watch {
		return nil, false
	}
	return roots, true
}

// ReachableDaemonConnection reports a verified-reachable daemon endpoint and
// the token discovered alongside it.
func ReachableDaemonConnection(root string) (discovery.Connection, error) {
	if v := strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL")); v != "" {
		if err := daemon.DaemonReachable(v); err != nil {
			return discovery.Connection{}, fmt.Errorf("configured ANTIGRAVITY_DAEMON_URL %s unreachable: %w", v, err)
		}
		return discovery.Connection{BaseURL: v}, nil
	}
	return discovery.DiscoverConnection(root)
}

// BuildReport gathers daemon reachability, agy-version compatibility, and
// sidecar coverage for root into a Report.
func BuildReport(root string) Report {
	pinnedCSRF := strings.TrimSpace(os.Getenv("ANTIGRAVITY_CSRF_TOKEN")) != ""
	r := Report{
		Surface:     discovery.DetectSurface(root),
		Root:        root,
		RecordedVer: RecordedAgyVersion(),
		AgyVer:      AgyVersion(),
		PinnedURL:   strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL")),
		PinnedCSRF:  pinnedCSRF,
	}
	if conn, err := ReachableDaemonConnection(root); err == nil {
		r.DaemonURL = conn.BaseURL
		client := session.NewDaemonClient(root, conn)
		r.CSRFFound = client.CSRFToken != ""
		if r.Surface == discovery.SurfaceCLI {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			r.AuthRejected = errors.Is(client.CheckAuthentication(ctx), daemon.ErrAuthentication)
			cancel()
		}
	} else {
		r.CSRFFound = pinnedCSRF
	}
	PopulateSidecarCoverage(&r, root)
	r.WatchRunning, r.WatchKnown = WatchRunningForRoot(root)
	return r
}

// RunTo renders the doctor report for roots to w and returns the
// process exit code (0 for healthy, 1 for action needed).
func RunTo(w io.Writer, roots []string, explicit bool) int {
	reports := make([]Report, 0, len(roots))
	for _, root := range roots {
		reports = append(reports, BuildReport(root))
	}
	return WriteMultiReport(w, reports, explicit)
}

// Run prints the doctor report to stdout and exits non-zero when there is
// something actionable, so callers can gate on `agy-reader doctor`'s exit code.
func Run(roots []string, explicit bool) error {
	if RunTo(os.Stdout, roots, explicit) != 0 {
		os.Exit(1)
	}
	return nil
}

// SidecarCoverage reports how many conversations/ sessions have a fresh
// sidecar versus a missing/stale one.
func SidecarCoverage(root string) (total, fresh, stale int, err error) {
	r := Report{}
	PopulateSidecarCoverage(&r, root)
	return r.Total, r.Fresh, r.Stale, r.CoverageErr
}

type sidecarCheck struct {
	result completeness.Result
	class  session.SourceClass
}

// PopulateSidecarCoverage gathers sidecar counts for root across a bounded
// worker pool while aggregating results in deterministic session order.
func PopulateSidecarCoverage(r *Report, root string) {
	sessions, err := discovery.ListConversationSessions(root)
	r.CoverageErr = err
	if err != nil {
		return
	}
	dir := filepath.Join(root, "conversations")
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		r.CoverageErr = err
		return
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".trajectory.json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			r.UnreadableSidecars++
			continue
		}
		if !bytes.Contains(raw, []byte(`"unverified"`)) {
			continue
		}
		if a := completeness.ReadAttestation(raw); a != nil && a.State == "unverified" {
			r.SavedUnverified++
		}
	}
	if len(sessions) == 0 {
		return
	}

	checks := make([]sidecarCheck, len(sessions))
	workers := min(len(sessions), max(1, min(runtime.GOMAXPROCS(0), 8)))
	var next atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1) - 1)
				if i >= len(sessions) {
					return
				}
				res, class := session.CompletenessClassified(sessions[i])
				checks[i] = sidecarCheck{result: res, class: class}
			}
		}()
	}
	wg.Wait()

	for i, s := range sessions {
		r.Total++
		fresh := false
		if strings.HasSuffix(s.PBPath, ".db") {
			r.SQLite++
		}
		result, class := checks[i].result, checks[i].class
		switch {
		case session.SourceOnlyVerified(string(result.State), result.Reason):
			r.VerifiedSourceOnly++
			fresh = true
		case result.State == completeness.Verified && result.Reason == completeness.TranscriptReason:
			r.VerifiedTranscript++
			fresh = true
		case result.State == completeness.Verified:
			r.Verified++
			fresh = true
		case result.State == completeness.Incomplete:
			r.Incomplete++
		default:
			r.Unknown++
			switch class {
			case session.SourceUnsupported:
				r.Unsupported++
				if len(r.UnsupportedSessions) < maxReportedSessions {
					reason := result.Reason
					if reason == "" {
						reason = string(result.State)
					}
					r.UnsupportedSessions = append(r.UnsupportedSessions, fmt.Sprintf("%s (%s)", s.CascadeID, reason))
				}
			case session.SourceUnreadable:
				if r.Unreadable == 0 {
					r.UnreadableReason = result.Reason
				}
				r.Unreadable++
			}
		}
		switch {
		case fresh:
			r.Fresh++
		case class == session.SourceRepairable:
			r.Stale++
			if len(r.StaleIDs) < maxReportedSessions {
				r.StaleIDs = append(r.StaleIDs, s.CascadeID)
			}
		}
	}
}
