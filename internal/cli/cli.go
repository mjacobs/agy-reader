package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
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
	"github.com/mjacobs/agy-reader/internal/doctor"
	"github.com/mjacobs/agy-reader/internal/render"
	"github.com/mjacobs/agy-reader/internal/session"
	"github.com/mjacobs/agy-reader/internal/subagent"
	"github.com/mjacobs/agy-reader/internal/watcher"
)

// Run executes the agy-reader CLI with the given arguments and I/O streams.
func Run(argv []string, stdout, stderr io.Writer) error {
	// Subcommands with their own flags are dispatched before the global flag
	// set so they can parse their own arguments.
	if len(argv) > 0 {
		switch argv[0] {
		case "shape-fingerprint":
			return RunShapeFingerprint(argv[1:], stdout, stderr)
		case "backfill-parent-links":
			return RunBackfillParentLinksTo(argv[1:], stderr)
		case "audit-sweep":
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return RunAuditSweepTo(ctx, argv[1:], stderr)
		case "audit-qualify":
			return RunAuditQualifyTo(argv[1:], stdout)
		case "verify-contract":
			return RunVerifyContract(argv[1:], stdout, stderr)
		case "doctor":
			return runDoctorSubcommand(argv[1:], stdout, stderr)
		}
	}

	fs := flag.NewFlagSet("agy-reader", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		listFlag             bool
		includeImplicitFlag  bool
		formatFlag           string
		syncFlag             bool
		watchFlag            bool
		watchIntervalFlag    time.Duration
		watchIdleTimeoutFlag time.Duration
		rootFlags            RootsFlag
		outFlag              string
	)
	fs.BoolVar(&listFlag, "list", false, "List syncable conversation sessions and exit")
	fs.BoolVar(&includeImplicitFlag, "include-implicit", false, "Include unsupported implicit sessions in --list output")
	fs.StringVar(&formatFlag, "format", "md", "Output format: md, json, both")
	fs.BoolVar(&syncFlag, "sync", false, "Fetch and persist sidecar, no rendering to stdout")
	fs.BoolVar(&watchFlag, "watch", false, "Watch for new/updated sessions and sync continuously")
	fs.DurationVar(
		&watchIntervalFlag,
		"watch-interval",
		30*time.Second,
		"Poll interval for --watch (e.g. 15s, 1m)",
	)
	fs.DurationVar(
		&watchIdleTimeoutFlag,
		"watch-idle-timeout",
		0,
		"Exit --watch after the daemon stays unreachable this long (0 = run forever; "+
			"use for path-triggered/event-driven systemd units)",
	)
	fs.Var(
		&rootFlags,
		"root",
		"Antigravity session root (repeatable; defaults to $ANTIGRAVITY_CLI_ROOT or "+
			"~/.gemini/antigravity-cli; pass ~/.gemini/antigravity for Antigravity 2.0 IDE sessions)",
	)
	fs.StringVar(
		&outFlag,
		"out",
		"",
		"Optional file path to write Markdown/JSON to (default stdout for md, sidecar for json)",
	)
	fs.Usage = func() { Usage(stderr, fs) }
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	roots, err := ResolveRoots(rootFlags)
	if err != nil {
		return err
	}
	explicitRoots := len(rootFlags) > 0 || os.Getenv("ANTIGRAVITY_CLI_ROOT") != ""

	if watchFlag {
		return watcher.Run(roots, watchIntervalFlag, watchIdleTimeoutFlag)
	}
	if listFlag {
		return RunListTo(stdout, stderr, roots, includeImplicitFlag)
	}

	args := fs.Args()
	if len(args) > 0 && args[0] == "doctor" {
		if doctor.RunTo(stdout, roots, explicitRoots) != 0 {
			os.Exit(1)
		}
		return nil
	}
	if len(args) == 0 {
		fs.Usage()
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

	traj, sidecarPath, err := FetchByID(ctx, roots, id)
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
		report, err := watcher.ReconcileParentLinks(filepath.Dir(sidecarPath), stderr)
		if err != nil {
			return err
		}
		if report.Stamped > 0 {
			fmt.Fprintf(stderr, "stamped %d parent link(s)\n", report.Stamped)
		}
		fmt.Fprintf(stderr, "wrote %s%s\n", sidecarPath, session.VerifiedNote(traj))
		return nil
	}

	if sidecarPath != "" && !traj.FromCache {
		if err := cache.Write(sidecarPath, traj); err != nil {
			fmt.Fprintf(stderr, "warning: could not write sidecar %s: %v\n", sidecarPath, err)
		} else if report, err := watcher.ReconcileParentLinks(filepath.Dir(sidecarPath), stderr); err != nil {
			fmt.Fprintf(stderr, "warning: could not resolve parent links: %v\n", err)
		} else if refreshed, err := cache.Read(sidecarPath); err == nil {
			traj = refreshed
			if report.Stamped > 0 {
				fmt.Fprintf(stderr, "stamped %d parent link(s)\n", report.Stamped)
			}
		}
	}

	resolver := BuildSubagentResolver(sidecarPath, stderr)

	switch formatFlag {
	case "md":
		return WriteMarkdown(outFlag, traj, resolver, stdout)
	case "json":
		return WriteJSON(outFlag, traj, stdout)
	case "both":
		if err := WriteMarkdown(outFlag, traj, resolver, stdout); err != nil {
			return err
		}
		return nil
	}
	return nil
}

func runDoctorSubcommand(argv []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var rootFlags RootsFlag
	fs.Var(&rootFlags, "root", "Antigravity session root (repeatable)")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	roots, err := ResolveRoots(rootFlags)
	if err != nil {
		return err
	}
	explicitRoots := len(rootFlags) > 0 || os.Getenv("ANTIGRAVITY_CLI_ROOT") != ""
	if doctor.RunTo(stdout, roots, explicitRoots) != 0 {
		os.Exit(1)
	}
	return nil
}

func Usage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintf(w, `agy-reader: extract Antigravity transcripts via the local language-server daemon.

Usage:
  agy-reader [flags] <cascade-id>
  agy-reader --list
  agy-reader --watch [--watch-interval=DURATION]
  agy-reader backfill-parent-links [--repair] [--root=PATH]...
  agy-reader audit-sweep --out=NEW_DIRECTORY [--root=PATH] [--timeout=30s]
  agy-reader audit-qualify --root=PATH [--manifest=PATH] CONVERSATION_ID...
  agy-reader verify-contract [--schema] <FILE_OR_DIR...>
  agy-reader doctor

Flags:
`)
	if fs != nil {
		fs.PrintDefaults()
	} else {
		flag.PrintDefaults()
	}
	fmt.Fprintf(w, `
Env:
  ANTIGRAVITY_DAEMON_URL   daemon base URL (optional, defaults to port
                           discovery from CLI processes on Linux, else cli.log;
                           language_server.log for an Antigravity 2.0 root)
  ANTIGRAVITY_CLI_ROOT     pin a single session root (suppresses store
                           discovery; default: each of ~/%s
                           and ~/%s that exists)
  ANTIGRAVITY_CSRF_TOKEN   CSRF token override for daemons that enforce one
                           (optional; auto-discovered for Linux CLI tool
                           processes and the Antigravity 2.0 daemon)
`, discovery.DefaultRootSubpath, discovery.DefaultIDERootSubpath)
}

func RequireDaemonConnection(root string) (discovery.Connection, error) {
	v := strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL"))
	if v != "" {
		return discovery.Connection{BaseURL: v}, nil
	}
	discovered, err := discovery.DiscoverConnection(root)
	if err == nil {
		return discovered, nil
	}
	return discovery.Connection{}, fmt.Errorf("ANTIGRAVITY_DAEMON_URL is not set and auto-discovery failed: %w\n\n"+
		"The agy daemon binds a different port every session. Find the\n"+
		"current one with:\n\n"+
		"    ss -tlnp 2>/dev/null | grep agy        # Linux\n"+
		"    lsof -iTCP -sTCP:LISTEN -anP | grep agy  # macOS\n\n"+
		"Use the port logged as 'for HTTP', not HTTPS/gRPC. Then:\n\n"+
		"    export ANTIGRAVITY_DAEMON_URL=http://127.0.0.1:<port>\n\n"+
		"If the daemon requires CSRF, also set ANTIGRAVITY_CSRF_TOKEN\n"+
		"to the matching token exported to a CLI tool process.", err)
}

func RequireDaemonURL(root string) (string, error) {
	conn, err := RequireDaemonConnection(root)
	return conn.BaseURL, err
}

type ListedSession struct {
	discovery.Session
	Surface discovery.Surface
}

func RunListTo(out, msg io.Writer, roots []string, includeImplicit bool) error {
	sessions := []ListedSession{}
	for _, root := range roots {
		found, err := ListSessionsForDisplay(root, includeImplicit)
		if err != nil {
			return err
		}
		surface := discovery.DetectSurface(root)
		for _, s := range found {
			sessions = append(sessions, ListedSession{Session: s, Surface: surface})
		}
	}
	slices.SortStableFunc(sessions, func(a, b ListedSession) int {
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
			fmt.Fprintf(out, "%s %s  %s  (%s)  %s\n", marker, s.CascadeID, s.ModTime.Format(time.RFC3339), s.Bucket, s.Surface)
		} else {
			fmt.Fprintf(out, "%s %s  %s  (%s)\n", marker, s.CascadeID, s.ModTime.Format(time.RFC3339), s.Bucket)
		}
	}
	fmt.Fprintf(msg, "\n(* = sidecar present)\n")
	return nil
}

func ListSessionsForDisplay(root string, includeImplicit bool) ([]discovery.Session, error) {
	if includeImplicit {
		return discovery.ListSessions(root)
	}
	return discovery.ListConversationSessions(root)
}

func FetchByID(ctx context.Context, roots []string, id string) (*daemon.Trajectory, string, error) {
	root, sess, found, err := FindSessionRoot(roots, id)
	if err != nil {
		return nil, "", err
	}
	if found {
		conn, err := RequireDaemonConnection(root)
		if err != nil {
			return nil, "", err
		}
		traj, err := session.FetchDisk(ctx, conn, root, sess)
		return traj, sess.SidecarPath, err
	}

	var errs []error
	for _, r := range roots {
		traj, err := FetchFromRootDaemon(ctx, r, id)
		if err == nil {
			return traj, "", nil
		}
		if len(roots) == 1 {
			return nil, "", err
		}
		errs = append(errs, fmt.Errorf("%s: %w", r, err))
	}
	return nil, "", errors.Join(errs...)
}

func FetchFromRootDaemon(ctx context.Context, root, id string) (*daemon.Trajectory, error) {
	conn, err := RequireDaemonConnection(root)
	if err != nil {
		return nil, err
	}
	return FetchTrajectory(ctx, conn, root, "", id, "")
}

func FetchTrajectory(ctx context.Context, conn discovery.Connection, root, sidecarPath, id, fallbackID string) (*daemon.Trajectory, error) {
	client := session.NewDaemonClient(root, conn)
	traj, daemonErr := client.FetchTrajectoryWithFallback(ctx, id, fallbackID)
	if daemonErr == nil {
		return traj, nil
	}

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

func BuildSubagentResolver(sidecarPath string, errw io.Writer) render.SubagentResolver {
	if sidecarPath == "" {
		return nil
	}
	res, err := subagent.Build(filepath.Dir(sidecarPath), errw)
	if err != nil {
		fmt.Fprintf(errw, "warning: could not index subagents: %v\n", err)
		return nil
	}
	return res
}

func WriteMarkdown(out string, t *daemon.Trajectory, r render.SubagentResolver, stdout io.Writer) error {
	if out == "" {
		_, err := render.MarkdownTree(stdout, t, time.Time{}, r)
		return err
	}

	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("create markdown output: %w", err)
	}
	_, writeErr := render.MarkdownTree(f, t, time.Time{}, r)
	if err := CloseOutput(f, writeErr); err != nil {
		return fmt.Errorf("write markdown output %s: %w", out, err)
	}
	return nil
}

func WriteJSON(out string, t *daemon.Trajectory, stdout io.Writer) error {
	if out == "" {
		return EncodeTrajectoryJSON(stdout, t)
	}

	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("create json output: %w", err)
	}
	writeErr := EncodeTrajectoryJSON(f, t)
	if err := CloseOutput(f, writeErr); err != nil {
		return fmt.Errorf("write json output %s: %w", out, err)
	}
	return nil
}

func EncodeTrajectoryJSON(w io.Writer, t *daemon.Trajectory) error {
	if t != nil && len(t.RawJSON) > 0 {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, bytes.TrimSpace(t.RawJSON), "", "  "); err != nil {
			return fmt.Errorf("format raw trajectory JSON: %w", err)
		}
		pretty.WriteByte('\n')
		_, err := w.Write(pretty.Bytes())
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(t)
}

func CloseOutput(f *os.File, writeErr error) error {
	closeErr := f.Close()
	if writeErr != nil && closeErr != nil {
		return errors.Join(writeErr, closeErr)
	}
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
