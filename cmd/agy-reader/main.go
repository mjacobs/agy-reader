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
		listFlag          bool
		formatFlag        string
		syncFlag          bool
		watchFlag         bool
		watchIntervalFlag time.Duration
		rootFlag          string
		outFlag           string
	)
	flag.BoolVar(&listFlag, "list", false, "List discovered conversations and exit")
	flag.StringVar(&formatFlag, "format", "md", "Output format: md, json, both")
	flag.BoolVar(&syncFlag, "sync", false, "Fetch and persist sidecar, no rendering to stdout")
	flag.BoolVar(&watchFlag, "watch", false, "Watch for new/updated sessions and sync continuously")
	flag.DurationVar(
		&watchIntervalFlag,
		"watch-interval",
		30*time.Second,
		"Poll interval for --watch (e.g. 15s, 1m)",
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
		base, err := requireDaemonURL()
		if err != nil {
			return err
		}
		return runWatch(root, base, watchIntervalFlag)
	}
	if listFlag {
		return runList(root)
	}

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		return errors.New("missing conversation id (or pass --list)")
	}
	id := strings.TrimSuffix(args[0], ".pb")

	switch formatFlag {
	case "md", "json", "both":
	default:
		return fmt.Errorf("invalid --format %q (want md|json|both)", formatFlag)
	}

	base, err := requireDaemonURL()
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
  ANTIGRAVITY_DAEMON_URL   daemon base URL (REQUIRED — no default; the agy
                           daemon binds a different ephemeral port each
                           session, see README troubleshooting)
  ANTIGRAVITY_CLI_ROOT     CLI session root (default ~/%s)
`, discovery.DefaultRootSubpath)
}

// requireDaemonURL reads ANTIGRAVITY_DAEMON_URL and returns a descriptive
// error if it is missing. v0.1 will replace this with auto-discovery; the
// single named helper keeps that future change local.
func requireDaemonURL() (string, error) {
	v := strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL"))
	if v != "" {
		return v, nil
	}
	return "", errors.New(`ANTIGRAVITY_DAEMON_URL is not set.

The agy daemon binds a different port every session. Find the
current one with:

    ss -tlnp 2>/dev/null | grep agy        # Linux
    lsof -iTCP -sTCP:LISTEN -anP | grep agy  # macOS

The lower-numbered port is the JSON-RPC endpoint. Then:

    export ANTIGRAVITY_DAEMON_URL=http://127.0.0.1:<port>

Auto-discovery is planned for v0.1.`)
}

func runList(root string) error {
	sessions, err := discovery.ListSessions(root)
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

// runWatch polls the session root every interval, fetching trajectories for
// any .pb whose sidecar is missing or older than the .pb's ModTime. Daemon
// errors are non-fatal — they log to stderr and the loop continues.
func runWatch(root, baseURL string, interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("--watch-interval must be positive, got %s", interval)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := log.New(os.Stderr, "", log.LstdFlags)
	client := daemon.NewClient(baseURL)
	logger.Printf("watch: root=%s daemon=%s interval=%s", root, baseURL, interval)

	consecutiveFailures := 0
	tick := func() {
		synced, skipped, upToDate, failed := watchTick(ctx, client, root, logger, &consecutiveFailures)
		logger.Printf("tick: %d synced, %d skipped, %d up-to-date, %d failed", synced, skipped, upToDate, failed)
	}

	tick()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Printf("watch: shutdown signal received")
			return nil
		case <-t.C:
			tick()
		}
	}
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
	sessions, err := discovery.ListSessions(root)
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
