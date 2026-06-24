// Command agy-reader extracts decrypted transcripts from Antigravity-CLI
// sessions by talking to the local language-server daemon.
//
// The daemon binds a different ephemeral port each time `agy` starts, so the
// URL must be supplied explicitly via ANTIGRAVITY_DAEMON_URL. Decrypted
// trajectories are written next to the encrypted .pb file as a sidecar named
// <uuid>.trajectory.json — that file is the integration contract with the
// sister project `agentsview`.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/render"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "agy-reader: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		listFlag             bool
		includeImplicitFlag  bool
		formatFlag           string
		syncFlag             bool
		watchFlag            bool
		watchIntervalFlag    time.Duration
		watchIdleTimeoutFlag time.Duration
		rootFlag             string
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
	flag.StringVar(
		&rootFlag,
		"root",
		"",
		"Override the Antigravity-CLI root (defaults to $ANTIGRAVITY_CLI_ROOT or ~/.gemini/antigravity-cli)",
	)
	flag.StringVar(
		&outFlag,
		"out",
		"",
		"Optional file path to write Markdown/JSON to (default stdout for md, sidecar for json)",
	)
	flag.Usage = usage
	flag.Parse()

	root := rootFlag
	if root == "" {
		var err error
		root, err = discovery.Root()
		if err != nil {
			return err
		}
	}

	if watchFlag {
		base, err := requireDaemonURL(root)
		if err != nil {
			// requireDaemonURL only errors when ANTIGRAVITY_DAEMON_URL is unset and
			// auto-discovery failed (the daemon isn't running yet) -- a pinned URL
			// short-circuits without error. So rather than exit, start the watch
			// loop in a pending state and let it auto-discover the daemon's
			// ephemeral port on a subsequent tick. This keeps a boot-time systemd
			// unit from failing when it starts before agy is up.
			fmt.Fprintf(os.Stderr, "watch: daemon not running yet; starting with auto-discovery pending: %v\n", err)
			return runWatch(root, "", watchIntervalFlag, watchIdleTimeoutFlag)
		}
		return runWatch(root, base, watchIntervalFlag, watchIdleTimeoutFlag)
	}
	if listFlag {
		return runList(root, includeImplicitFlag)
	}

	args := flag.Args()
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

	base, err := requireDaemonURL(root)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	traj, sidecarPath, err := fetchTrajectory(ctx, base, root, id)
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

	switch formatFlag {
	case "md":
		return writeMarkdown(outFlag, traj)
	case "json":
		return writeJSON(outFlag, traj)
	case "both":
		if err := writeMarkdown(outFlag, traj); err != nil {
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

Flags:
`)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Env:
  ANTIGRAVITY_DAEMON_URL   daemon base URL (optional, defaults to port
                           auto-discovery by parsing ~/.gemini/antigravity-cli/cli.log)
  ANTIGRAVITY_CLI_ROOT     CLI session root (default ~/%s)
`, discovery.DefaultRootSubpath)
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

func runList(root string, includeImplicit bool) error {
	sessions, err := listSessionsForDisplay(root, includeImplicit)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		fmt.Fprintf(os.Stderr, "no sessions found under %s\n", root)
		return nil
	}
	for _, s := range sessions {
		marker := " "
		if cache.Exists(s.SidecarPath) {
			marker = "*"
		}
		fmt.Printf("%s %s  %s  (%s)\n", marker, s.CascadeID, s.ModTime.Format(time.RFC3339), s.Bucket)
	}
	fmt.Fprintf(os.Stderr, "\n(* = sidecar present)\n")
	return nil
}

func listSessionsForDisplay(root string, includeImplicit bool) ([]discovery.Session, error) {
	if includeImplicit {
		return discovery.ListSessions(root)
	}
	return discovery.ListConversationSessions(root)
}

// fetchTrajectory resolves a cascade id to a Trajectory. It tries the daemon
// first; on connection failure it falls back to an existing sidecar if one
// happens to be on disk. sidecarPath is "" when the id is not present on
// disk (e.g. user passed an id from a different machine).
func fetchTrajectory(ctx context.Context, baseURL, root, id string) (*daemon.Trajectory, string, error) {
	session, found, err := discovery.FindByID(root, id)
	if err != nil {
		return nil, "", err
	}
	sidecarPath := ""
	if found {
		sidecarPath = session.SidecarPath
	}

	client := daemon.NewClient(baseURL)
	traj, daemonErr := client.FetchTrajectory(ctx, id)
	if daemonErr == nil {
		return traj, sidecarPath, nil
	}

	// Daemon failed — try the sidecar as a fallback.
	if sidecarPath != "" {
		if cached, err := cache.Read(sidecarPath); err == nil {
			fmt.Fprintf(os.Stderr, "warning: daemon unreachable (%v); using cached sidecar\n", daemonErr)
			return cached, sidecarPath, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "warning: sidecar read failed: %v\n", err)
		}
	}
	return nil, sidecarPath, fmt.Errorf("daemon fetch failed and no usable cache: %w", daemonErr)
}

func writeMarkdown(out string, t *daemon.Trajectory) error {
	if out == "" {
		_, err := render.Markdown(os.Stdout, t, time.Time{})
		return err
	}

	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("create markdown output: %w", err)
	}
	_, writeErr := render.Markdown(f, t, time.Time{})
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
	// tick reports stop=true so the watch loop returns cleanly. This lets a
	// path-triggered/event-driven systemd unit run only while agy is up and
	// relaunch on the next agy activity. idleTicks counts the current idle streak.
	interval    time.Duration
	idleTimeout time.Duration
	idleTicks   int
}

// tick performs one watch iteration: (re)discover the daemon URL when needed,
// then sync stale sidecars. It is safe to call with an empty baseURL — that
// means the daemon hasn't been located yet (e.g. agy not running at startup),
// in which case it logs a pending line and waits for discovery. tick returns
// stop=true once the daemon has been idle for at least idleTimeout (see
// updateIdle); the caller should then exit the watch loop.
func (w *watcher) tick() (stop bool) {
	if (w.consecutiveFailures > 0 || w.baseURL == "") && !w.urlPinned {
		if next, ok := rediscoverDaemonURL(w.root, w.baseURL, w.logger); ok {
			w.baseURL = next
			w.client = daemon.NewClient(next)
		}
	}
	if w.baseURL == "" {
		// Daemon not located yet — don't fetch (the client has no URL); just wait
		// for a later tick to auto-discover it. consecutiveFailures is left
		// untouched (nothing failed, discovery is merely pending), but a pending
		// tick still counts as idle for auto-exit purposes.
		w.logger.Printf("tick: 0 synced, 0 skipped, 0 up-to-date, 0 failed (daemon auto-discovery pending)")
		return w.updateIdle(true)
	}
	synced, skipped, upToDate, failed := watchTick(w.ctx, w.client, w.root, w.logger, &w.consecutiveFailures)
	w.logger.Printf("tick: %d synced, %d skipped, %d up-to-date, %d failed", synced, skipped, upToDate, failed)
	// A positive failure streak means the daemon was unreachable this tick.
	return w.updateIdle(w.consecutiveFailures > 0)
}

// updateIdle grows or resets the idle streak after a tick and reports whether
// the watch loop should stop. A tick is "idle" when the daemon is pending or
// unreachable; any reachable tick resets the streak. Auto-exit is disabled when
// idleTimeout <= 0, preserving the run-forever default.
func (w *watcher) updateIdle(idle bool) (stop bool) {
	if !idle {
		w.idleTicks = 0
		return false
	}
	w.idleTicks++
	if w.idleTimeout <= 0 {
		return false
	}
	if time.Duration(w.idleTicks)*w.interval >= w.idleTimeout {
		w.logger.Printf("watch: daemon unreachable for ~%s; exiting (a path-triggered unit relaunches on next agy activity)", w.idleTimeout)
		return true
	}
	return false
}

// runWatch polls the session root every interval, fetching trajectories for
// any conversations/ .pb whose sidecar is missing or older than the .pb's
// ModTime. Daemon errors are non-fatal — they log to stderr and the loop
// continues. When idleTimeout > 0 the loop exits cleanly (nil) after the daemon
// has been unreachable that long, for event-driven/path-triggered units.
func runWatch(root, baseURL string, interval, idleTimeout time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("--watch-interval must be positive, got %s", interval)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return runWatchLoop(ctx, root, baseURL, interval, idleTimeout)
}

// runWatchLoop is the cancellable core of runWatch, split out so tests can drive
// it with their own context (the production path wires ctx to SIGINT/SIGTERM).
func runWatchLoop(ctx context.Context, root, baseURL string, interval, idleTimeout time.Duration) error {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	logger.Printf("watch: root=%s daemon=%s interval=%s idle-timeout=%s", root, baseURL, interval, idleTimeout)

	// The daemon binds a fresh random port every agy session, so a URL that was
	// valid at startup goes stale whenever agy restarts. Unless the URL is pinned
	// via ANTIGRAVITY_DAEMON_URL, re-discover after failures (or when we started
	// before the daemon existed and have no URL yet).
	w := &watcher{
		ctx:         ctx,
		root:        root,
		baseURL:     baseURL,
		client:      daemon.NewClient(baseURL),
		urlPinned:   strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL")) != "",
		logger:      logger,
		interval:    interval,
		idleTimeout: idleTimeout,
	}

	if w.tick() {
		return nil
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Printf("watch: shutdown signal received")
			return nil
		case <-t.C:
			if w.tick() {
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
	logger.Printf("watch: daemon moved %s -> %s", current, next)
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
