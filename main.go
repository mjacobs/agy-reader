// Command agy-reader extracts decrypted transcripts from Antigravity
// sessions — both the CLI (`agy`) and Antigravity 2.0 (the IDE) — by talking
// to the local language-server daemon each of them runs. A bare invocation
// operates on every default session store that exists on disk; repeatable
// --root flags (or ANTIGRAVITY_CLI_ROOT) pin the roots explicitly instead.
//
// The daemon binds a different ephemeral port each time its host program
// starts, so the URL is auto-discovered from the daemon's log (or supplied
// explicitly via ANTIGRAVITY_DAEMON_URL). Decrypted trajectories are written
// next to the encrypted .pb/.db file as a sidecar named
// <uuid>.trajectory.json — that file is the integration contract with the
// sister project `agentsview`.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/render"
	"github.com/mjacobs/agy-reader/internal/subagent"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "agy-reader: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Subcommands that don't touch roots/daemon are dispatched before the
	// global flag set so they can parse their own flags/args.
	if len(os.Args) > 1 && os.Args[1] == "shape-fingerprint" {
		return runShapeFingerprint(os.Args[2:])
	}

	var (
		listFlag             bool
		includeImplicitFlag  bool
		formatFlag           string
		syncFlag             bool
		watchFlag            bool
		watchIntervalFlag    time.Duration
		watchIdleTimeoutFlag time.Duration
		rootFlags            rootsFlag
		outFlag              string
	)
	flag.BoolVar(&listFlag, "list", false, "List syncable conversation sessions and exit")
	flag.BoolVar(&includeImplicitFlag, "include-implicit", false, "Include unsupported implicit sessions in --list output")
	flag.StringVar(&formatFlag, "format", "md", "Output format: md, json, both")
	flag.BoolVar(&syncFlag, "sync", false, "Fetch and persist sidecar, no rendering to stdout")
	flag.BoolVar(&watchFlag, "watch", false, "Watch for new/updated sessions and sync continuously")
	flag.DurationVar(
		&watchIntervalFlag,
		"watch-interval",
		30*time.Second,
		"Poll interval for --watch (e.g. 15s, 1m)",
	)
	flag.DurationVar(
		&watchIdleTimeoutFlag,
		"watch-idle-timeout",
		0,
		"Exit --watch after the daemon stays unreachable this long (0 = run forever; "+
			"use for path-triggered/event-driven systemd units)",
	)
	flag.Var(
		&rootFlags,
		"root",
		"Antigravity session root (repeatable; defaults to $ANTIGRAVITY_CLI_ROOT or "+
			"~/.gemini/antigravity-cli; pass ~/.gemini/antigravity for Antigravity 2.0 IDE sessions)",
	)
	flag.StringVar(
		&outFlag,
		"out",
		"",
		"Optional file path to write Markdown/JSON to (default stdout for md, sidecar for json)",
	)
	flag.Usage = usage
	flag.Parse()

	roots, err := resolveRoots(rootFlags)
	if err != nil {
		return err
	}
	// Roots named via --root or ANTIGRAVITY_CLI_ROOT are hard requirements
	// (doctor fails when any is unhealthy); discovered roots soft-fail.
	explicitRoots := len(rootFlags) > 0 || os.Getenv("ANTIGRAVITY_CLI_ROOT") != ""

	if watchFlag {
		return runWatch(roots, watchIntervalFlag, watchIdleTimeoutFlag)
	}
	if listFlag {
		return runListTo(os.Stdout, os.Stderr, roots, includeImplicitFlag)
	}

	args := flag.Args()
	if len(args) > 0 && args[0] == "doctor" {
		return runDoctor(roots, explicitRoots)
	}
	if len(args) == 0 {
		flag.Usage()
		return errors.New("missing conversation id (or pass --list)")
	}
	id := strings.TrimSuffix(args[0], ".pb")
	id = strings.TrimSuffix(id, ".db")

	switch formatFlag {
	case "md", "json", "both":
	default:
		return fmt.Errorf("invalid --format %q (want md|json|both)", formatFlag)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	traj, sidecarPath, err := fetchByID(ctx, roots, id)
	if err != nil {
		return err
	}

	if syncFlag {
		if sidecarPath == "" {
			return fmt.Errorf("cannot --sync: no on-disk .pb found for %s (sidecar location unknown)", id)
		}
		if err := cache.Write(sidecarPath, traj); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", sidecarPath)
		return nil
	}

	// Always persist the sidecar when we know where to put it — it's cheap
	// and that's the whole integration story with agentsview.
	if sidecarPath != "" {
		if err := cache.Write(sidecarPath, traj); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write sidecar %s: %v\n", sidecarPath, err)
		}
	}

	// Build a subagent resolver from the sibling sidecars so nested subagent
	// transcripts render inline under the parent's INVOKE_SUBAGENT steps. Only
	// possible when the session is on disk (sidecarPath known); a not-on-disk
	// fetch has no sibling dir to scan, so it renders flat.
	resolver := buildSubagentResolver(sidecarPath)

	switch formatFlag {
	case "md":
		return writeMarkdown(outFlag, traj, resolver)
	case "json":
		return writeJSON(outFlag, traj)
	case "both":
		if err := writeMarkdown(outFlag, traj, resolver); err != nil {
			return err
		}
		// "both" with --out is ambiguous; for now, --out only applies to md.
		// The sidecar already holds the JSON.
		return nil
	}
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, `agy-reader: extract Antigravity-CLI transcripts via the local decryption daemon.

Usage:
  agy-reader [flags] <cascade-id>
  agy-reader --list
  agy-reader --watch [--watch-interval=DURATION]
  agy-reader doctor

Flags:
`)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Env:
  ANTIGRAVITY_DAEMON_URL   daemon base URL (optional, defaults to port
                           auto-discovery from the daemon's log: cli.log inside
                           a CLI root, language_server.log under the IDE's log
                           dir for an Antigravity 2.0 root)
  ANTIGRAVITY_CLI_ROOT     pin a single session root (suppresses store
                           discovery; default: each of ~/%s
                           and ~/%s that exists)
  ANTIGRAVITY_CSRF_TOKEN   CSRF token override for daemons that enforce one
                           (optional; auto-discovered for the Antigravity 2.0
                           daemon)
`, discovery.DefaultRootSubpath, discovery.DefaultIDERootSubpath)
}

// newDaemonClient builds a client for the daemon serving root, attaching the
// CSRF token when that daemon's launch config has one (the IDE daemon) and
// omitting the header otherwise (the CLI daemon). Called at every client
// (re)build so a watch loop that spans an IDE restart picks up the fresh
// token along with the fresh port.
func newDaemonClient(root, baseURL string) *daemon.Client {
	c := daemon.NewClient(baseURL)
	c.CSRFToken = discovery.DiscoverCSRFToken(root)
	return c
}

// requireDaemonURL reads ANTIGRAVITY_DAEMON_URL or attempts auto-discovery
// of the active daemon port, returning a descriptive error if both fail.
func requireDaemonURL(root string) (string, error) {
	v := strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL"))
	if v != "" {
		return v, nil
	}

	// Try auto-discovery
	discovered, err := discovery.DiscoverDaemonURL(root)
	if err == nil {
		return discovered, nil
	}

	return "", fmt.Errorf("ANTIGRAVITY_DAEMON_URL is not set and auto-discovery failed: %w\n\n"+
		"The agy daemon binds a different port every session. Find the\n"+
		"current one with:\n\n"+
		"    ss -tlnp 2>/dev/null | grep agy        # Linux\n"+
		"    lsof -iTCP -sTCP:LISTEN -anP | grep agy  # macOS\n\n"+
		"The lower-numbered port is the JSON-RPC endpoint. Then:\n\n"+
		"    export ANTIGRAVITY_DAEMON_URL=http://127.0.0.1:<port>", err)
}

// listedSession is one --list line: a discovered session plus the surface of
// the root it came from, so multi-root listings can label each line.
type listedSession struct {
	discovery.Session
	surface discovery.Surface
}

// runListTo renders the session listing for roots to out, with human-facing
// notes (empty-store message, sidecar legend) on msg. Sessions aggregate
// across roots newest-first; a surface column is added only when more than
// one root is in play, keeping single-root output byte-identical for
// existing CLI users' scripts.
func runListTo(out, msg io.Writer, roots []string, includeImplicit bool) error {
	sessions := []listedSession{}
	for _, root := range roots {
		found, err := listSessionsForDisplay(root, includeImplicit)
		if err != nil {
			return err
		}
		surface := discovery.DetectSurface(root)
		for _, s := range found {
			sessions = append(sessions, listedSession{Session: s, surface: surface})
		}
	}
	slices.SortStableFunc(sessions, func(a, b listedSession) int {
		return b.ModTime.Compare(a.ModTime)
	})
	if len(sessions) == 0 {
		fmt.Fprintf(msg, "no sessions found under %s\n", strings.Join(roots, ", "))
		return nil
	}
	multi := len(roots) > 1
	for _, s := range sessions {
		marker := " "
		if cache.Exists(s.SidecarPath) {
			marker = "*"
		}
		if multi {
			fmt.Fprintf(out, "%s %s  %s  (%s)  %s\n", marker, s.CascadeID, s.ModTime.Format(time.RFC3339), s.Bucket, s.surface)
		} else {
			fmt.Fprintf(out, "%s %s  %s  (%s)\n", marker, s.CascadeID, s.ModTime.Format(time.RFC3339), s.Bucket)
		}
	}
	fmt.Fprintf(msg, "\n(* = sidecar present)\n")
	return nil
}

func listSessionsForDisplay(root string, includeImplicit bool) ([]discovery.Session, error) {
	if includeImplicit {
		return discovery.ListSessions(root)
	}
	return discovery.ListConversationSessions(root)
}

// fetchByID resolves a cascade id against the roots in order. An id found on
// disk binds the fetch to its owning root — daemon, CSRF config, sidecar
// location and sidecar fallback all follow it, exactly the single-root
// behavior. An id on no root's disk (e.g. passed from another machine, or a
// session the daemon holds only in memory) is probed against each root's
// daemon in order until one serves it; such a session has no sidecar
// location, so sidecarPath is "".
func fetchByID(ctx context.Context, roots []string, id string) (*daemon.Trajectory, string, error) {
	root, session, found, err := findSessionRoot(roots, id)
	if err != nil {
		return nil, "", err
	}
	if found {
		base, err := requireDaemonURL(root)
		if err != nil {
			return nil, "", err
		}
		traj, err := fetchTrajectory(ctx, base, root, session.SidecarPath, id)
		return traj, session.SidecarPath, err
	}

	var errs []error
	for _, r := range roots {
		traj, err := fetchFromRootDaemon(ctx, r, id)
		if err == nil {
			return traj, "", nil
		}
		if len(roots) == 1 {
			// Keep the single-root error UX (the remediation text stands alone).
			return nil, "", err
		}
		errs = append(errs, fmt.Errorf("%s: %w", r, err))
	}
	return nil, "", errors.Join(errs...)
}

// fetchFromRootDaemon resolves root's daemon URL and fetches id from it —
// one probe step of fetchByID's not-on-disk path.
func fetchFromRootDaemon(ctx context.Context, root, id string) (*daemon.Trajectory, error) {
	base, err := requireDaemonURL(root)
	if err != nil {
		return nil, err
	}
	return fetchTrajectory(ctx, base, root, "", id)
}

// fetchTrajectory resolves a cascade id to a Trajectory via root's daemon;
// on connection failure it falls back to an existing sidecar if one happens
// to be on disk. sidecarPath is "" when the id is not present on disk (e.g.
// user passed an id from a different machine).
func fetchTrajectory(ctx context.Context, baseURL, root, sidecarPath, id string) (*daemon.Trajectory, error) {
	client := newDaemonClient(root, baseURL)
	traj, daemonErr := client.FetchTrajectory(ctx, id)
	if daemonErr == nil {
		return traj, nil
	}

	// Daemon failed — try the sidecar as a fallback.
	if sidecarPath != "" {
		if cached, err := cache.Read(sidecarPath); err == nil {
			fmt.Fprintf(os.Stderr, "warning: daemon unreachable (%v); using cached sidecar\n", daemonErr)
			return cached, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "warning: sidecar read failed: %v\n", err)
		}
	}
	return nil, fmt.Errorf("daemon fetch failed and no usable cache: %w", daemonErr)
}

// buildSubagentResolver scans the directory holding sidecarPath for sibling
// *.trajectory.json files and inverts their child->parent links so the
// renderer can nest subagent transcripts. Returns nil (flat rendering) when
// the session isn't on disk or the scan fails — inlining is best-effort.
func buildSubagentResolver(sidecarPath string) render.SubagentResolver {
	if sidecarPath == "" {
		return nil
	}
	res, err := subagent.Build(filepath.Dir(sidecarPath), os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not index subagents: %v\n", err)
		return nil
	}
	return res
}

func writeMarkdown(out string, t *daemon.Trajectory, r render.SubagentResolver) error {
	if out == "" {
		_, err := render.MarkdownTree(os.Stdout, t, time.Time{}, r)
		return err
	}

	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("create markdown output: %w", err)
	}
	_, writeErr := render.MarkdownTree(f, t, time.Time{}, r)
	if err := closeOutput(f, writeErr); err != nil {
		return fmt.Errorf("write markdown output %s: %w", out, err)
	}
	return nil
}

func writeJSON(out string, t *daemon.Trajectory) error {
	if out == "" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(t)
	}

	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("create json output: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	writeErr := enc.Encode(t)
	if err := closeOutput(f, writeErr); err != nil {
		return fmt.Errorf("write json output %s: %w", out, err)
	}
	return nil
}

func closeOutput(f *os.File, writeErr error) error {
	closeErr := f.Close()
	if writeErr != nil && closeErr != nil {
		return errors.Join(writeErr, closeErr)
	}
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

// watcher holds the state of a --watch session that persists across ticks: the
// current daemon base URL (which goes stale whenever agy restarts on a new
// port), the client bound to it, and the consecutive-failure streak used for
// retry/log bookkeeping.
type watcher struct {
	ctx                 context.Context
	root                string
	baseURL             string
	client              *daemon.Client
	urlPinned           bool
	logger              *log.Logger
	consecutiveFailures int

	// interval and idleTimeout drive optional auto-exit. When idleTimeout > 0
	// and the daemon has been unreachable (or never discovered) for that long,
	// tick reports the watcher as idle-expired; the watch loop returns cleanly
	// once EVERY root is. This lets a path-triggered/event-driven systemd unit
	// run only while a daemon is up and relaunch on the next agy/IDE activity.
	// idleTicks counts the current idle streak; idleExpired records whether
	// the streak currently exceeds the timeout (the watcher keeps polling
	// regardless — expiry is never a retirement, the daemon may come back).
	interval    time.Duration
	idleTimeout time.Duration
	idleTicks   int
	idleExpired bool
}

// tick performs one watch iteration: (re)discover the daemon URL when needed,
// then sync stale sidecars. It is safe to call with an empty baseURL — that
// means the daemon hasn't been located yet (e.g. agy not running at startup),
// in which case it logs a pending line and waits for discovery. tick reports
// whether the daemon has now been idle for at least idleTimeout (see
// updateIdle); the caller exits the watch loop once every watcher reports so.
func (w *watcher) tick() (idleExpired bool) {
	if (w.consecutiveFailures > 0 || w.baseURL == "") && !w.urlPinned {
		if next, ok := rediscoverDaemonURL(w.root, w.baseURL, w.logger); ok {
			w.baseURL = next
			w.client = newDaemonClient(w.root, next)
		}
	}
	if w.baseURL == "" {
		// Daemon not located yet — don't fetch (the client has no URL); just wait
		// for a later tick to auto-discover it. consecutiveFailures is left
		// untouched (nothing failed, discovery is merely pending), but a pending
		// tick still counts as idle for auto-exit purposes.
		w.logger.Printf("tick: 0 synced, 0 skipped, 0 up-to-date, 0 failed (agy daemon not found yet; retrying every %s)", w.interval)
		return w.updateIdle(true)
	}
	synced, skipped, upToDate, failed := watchTick(w.ctx, w.client, w.root, w.logger, &w.consecutiveFailures)
	w.logger.Printf("tick: %d synced, %d skipped, %d up-to-date, %d failed", synced, skipped, upToDate, failed)
	// A positive failure streak means the daemon was unreachable this tick.
	return w.updateIdle(w.consecutiveFailures > 0)
}

// updateIdle grows or resets the idle streak after a tick and reports whether
// the streak currently exceeds idleTimeout. A tick is "idle" when the daemon
// is pending or unreachable; any reachable tick resets the streak (and clears
// expiry — an expired watcher is never retired, so a daemon that comes back
// is picked up again). Auto-exit is disabled when idleTimeout <= 0,
// preserving the run-forever default.
func (w *watcher) updateIdle(idle bool) (idleExpired bool) {
	if !idle {
		w.idleTicks = 0
		w.idleExpired = false
		return false
	}
	w.idleTicks++
	if w.idleTimeout <= 0 {
		return false
	}
	if time.Duration(w.idleTicks)*w.interval >= w.idleTimeout {
		if !w.idleExpired {
			w.logger.Printf("watch: daemon unreachable for ~%s; root is idle (still polling; the process exits once every root is idle)", w.idleTimeout)
		}
		w.idleExpired = true
		return true
	}
	return false
}

// runWatch polls every session root each interval, fetching trajectories for
// any conversations/ .pb whose sidecar is missing or older than the .pb's
// ModTime. Daemon errors are non-fatal — they log to stderr and the loop
// continues; a root whose daemon is down merely waits and never takes the
// other roots' loops with it. When idleTimeout > 0 the process exits cleanly
// (nil) only after EVERY root's daemon has been unreachable that long, for
// event-driven/path-triggered units.
func runWatch(roots []string, interval, idleTimeout time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("--watch-interval must be positive, got %s", interval)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return runWatchLoop(ctx, roots, interval, idleTimeout)
}

// watchLogLabels returns the per-root log prefixes for a multi-root watch:
// the surface name when it identifies the root uniquely, the root path when
// two roots share a surface. A single-root watch keeps unprefixed lines.
func watchLogLabels(roots []string) []string {
	if len(roots) == 1 {
		return []string{""}
	}
	surfaces := make([]discovery.Surface, len(roots))
	counts := map[discovery.Surface]int{}
	for i, r := range roots {
		surfaces[i] = discovery.DetectSurface(r)
		counts[surfaces[i]]++
	}
	labels := make([]string, len(roots))
	for i := range roots {
		label := string(surfaces[i])
		if counts[surfaces[i]] > 1 {
			label = roots[i]
		}
		labels[i] = "[" + label + "] "
	}
	return labels
}

// runWatchLoop is the cancellable core of runWatch, split out so tests can
// drive it with their own context (the production path wires ctx to
// SIGINT/SIGTERM). It runs one watcher per root on a shared ticker; each
// watcher (re)discovers its own daemon URL, so an IDE restart or a late agy
// start on any surface heals independently of the others.
func runWatchLoop(ctx context.Context, roots []string, interval, idleTimeout time.Duration) error {
	pinned := strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL")) != ""
	labels := watchLogLabels(roots)
	watchers := make([]*watcher, 0, len(roots))
	for i, root := range roots {
		logger := log.New(os.Stderr, labels[i], log.LstdFlags)
		// A daemon that isn't running yet is a pending start, not an error: the
		// loop auto-discovers its ephemeral port on a later tick. This keeps a
		// boot-time systemd unit from failing when it starts before agy is up,
		// and keeps a closed IDE from failing the whole multi-root watch.
		baseURL := ""
		if base, err := requireDaemonURL(root); err == nil {
			baseURL = base
		} else {
			logger.Printf("watch: daemon not running yet; starting with auto-discovery pending: %v", err)
		}
		logger.Printf("watch: root=%s daemon=%s interval=%s idle-timeout=%s", root, baseURL, interval, idleTimeout)

		// The daemon binds a fresh random port every session, so a URL that was
		// valid at startup goes stale whenever its host program restarts. Unless
		// the URL is pinned via ANTIGRAVITY_DAEMON_URL, re-discover after
		// failures (or when we started before the daemon existed).
		watchers = append(watchers, &watcher{
			ctx:         ctx,
			root:        root,
			baseURL:     baseURL,
			client:      newDaemonClient(root, baseURL),
			urlPinned:   pinned,
			logger:      logger,
			interval:    interval,
			idleTimeout: idleTimeout,
		})
	}

	// Every watcher ticks every round — an idle-expired root keeps polling so
	// its daemon coming back is picked up. The process exits only when every
	// root is currently idle past the timeout.
	tickAll := func() (allIdleExpired bool) {
		allIdleExpired = true
		for _, w := range watchers {
			if !w.tick() {
				allIdleExpired = false
			}
		}
		return allIdleExpired
	}
	processLogger := log.New(os.Stderr, "", log.LstdFlags)
	exitLine := func() {
		processLogger.Printf("watch: all roots idle for ~%s; exiting (a path-triggered unit relaunches on the next agy/IDE activity)", idleTimeout)
	}

	if tickAll() {
		exitLine()
		return nil
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			processLogger.Printf("watch: shutdown signal received")
			return nil
		case <-t.C:
			if tickAll() {
				exitLine()
				return nil
			}
		}
	}
}

// rediscoverDaemonURL re-runs port auto-discovery after a daemon-unreachable
// tick. Returns the new URL and true when discovery finds a different,
// reachable daemon than current.
func rediscoverDaemonURL(root, current string, logger *log.Logger) (string, bool) {
	next, err := discovery.DiscoverDaemonURL(root)
	if err != nil || next == current {
		return "", false
	}
	if current == "" {
		// First time we've located the daemon (e.g. agy started after the watcher
		// did). Report it as a discovery — a "moved  -> URL" line with an empty
		// old URL reads as noise, not as "agy is now connected".
		logger.Printf("watch: agy daemon discovered at %s", next)
	} else {
		logger.Printf("watch: agy daemon moved %s -> %s", current, next)
	}
	return next, true
}

// watchTick performs one pass over the session root. Returns counts for the
// summary log line. Daemon-level failures (e.g. connection refused) are
// reported once per failure streak via consecutiveFailures, with a
// "recovered" log line when a tick succeeds after prior failures.
func watchTick(
	ctx context.Context,
	client *daemon.Client,
	root string,
	logger *log.Logger,
	consecutiveFailures *int,
) (synced, skipped, upToDate, failed int) {
	if ctx.Err() != nil {
		return
	}
	sessions, err := discovery.ListConversationSessions(root)
	if err != nil {
		logger.Printf("watch: discovery error: %v", err)
		return
	}

	tickHadDaemonFailure := false
	tickHadSuccess := false

	for _, s := range sessions {
		if ctx.Err() != nil {
			return
		}
		stale, reason := isStale(s)
		if !stale {
			upToDate++
			continue
		}
		traj, err := client.FetchTrajectory(ctx, s.CascadeID)
		if err != nil {
			failed++
			if isConnRefused(err) {
				tickHadDaemonFailure = true
				if *consecutiveFailures == 0 {
					logger.Printf("watch: daemon unreachable (%v); retrying next tick", err)
				}
				// Stop processing further sessions this tick — daemon is down.
				*consecutiveFailures++
				return
			}
			logger.Printf("watch: skip %s (%v)", s.CascadeID, err)
			skipped++
			continue
		}
		tickHadSuccess = true
		if err := cache.Write(s.SidecarPath, traj); err != nil {
			logger.Printf("watch: write sidecar %s failed: %v", s.SidecarPath, err)
			failed++
			continue
		}
		size := sidecarSize(s.SidecarPath)
		logger.Printf("synced %s (%d steps, %dKB) [%s]", s.CascadeID, len(traj.Steps), size/1024, reason)
		synced++
	}

	if !tickHadDaemonFailure {
		if *consecutiveFailures > 0 && tickHadSuccess {
			logger.Printf("watch: daemon recovered after %d failed tick(s)", *consecutiveFailures)
		}
		*consecutiveFailures = 0
	}
	return
}

// isStale reports whether a session's sidecar is missing or older than its .pb.
func isStale(s discovery.Session) (bool, string) {
	info, err := os.Stat(s.SidecarPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, "missing"
		}
		return true, "stat-error"
	}
	if s.ModTime.After(info.ModTime()) {
		return true, "modtime-advanced"
	}
	return false, ""
}

func sidecarSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// isConnRefused returns true when err looks like the daemon isn't listening.
func isConnRefused(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	s := err.Error()
	return strings.Contains(s, "connection refused") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "connect: connection")
}
