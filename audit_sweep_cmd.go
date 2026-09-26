package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/render"
)

type sweepSession struct {
	FileID    string `json:"fileId"`
	CascadeID string `json:"cascadeId"`
	Steps     int    `json:"steps"`
}

type sweepManifest struct {
	Version    int            `json:"manifestVersion"`
	Complete   bool           `json:"complete"`
	Root       string         `json:"root"`
	Endpoint   string         `json:"endpoint"`
	AgyVersion string         `json:"installedAgyVersion"`
	Started    time.Time      `json:"started"`
	Finished   time.Time      `json:"finished"`
	Selected   int            `json:"selected"`
	FailedID   string         `json:"failedId,omitempty"`
	Sessions   []sweepSession `json:"sessions"`
}

func runAuditSweep(args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return runAuditSweepTo(ctx, args, os.Stderr)
}

func runAuditSweepTo(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("audit-sweep", flag.ContinueOnError)
	fs.SetOutput(out)
	root := fs.String("root", "", "one session store (default: ANTIGRAVITY_CLI_ROOT or CLI store)")
	dest := fs.String("out", "", "new private directory for corpus/ and manifest.json; must not exist")
	timeout := fs.Duration("timeout", 30*time.Second, "timeout per conversation fetch")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *dest == "" || *timeout <= 0 {
		return fmt.Errorf("usage: agy-reader audit-sweep --out=NEW_DIRECTORY [--root=PATH] [--timeout=30s]")
	}
	if *root == "" {
		var err error
		*root, err = discovery.Root()
		if err != nil {
			return err
		}
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	sessions, err := discovery.ListConversationSessions(absRoot)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return fmt.Errorf("audit-sweep: no syncable conversations under %s", absRoot)
	}
	// A .pb and .db can name the same session. A corpus file must correspond
	// to exactly one fetch, and filenames must remain inside the output dir.
	seen := map[string]bool{}
	for _, s := range sessions {
		if daemon.CanonicalCascadeID(s.CascadeID) == "" || seen[s.CascadeID] {
			return fmt.Errorf("audit-sweep: invalid or duplicate session ID %q", s.CascadeID)
		}
		seen[s.CascadeID] = true
	}
	conn, err := requireDaemonConnection(absRoot)
	if err != nil {
		return err
	}
	if err := os.Mkdir(*dest, 0o700); err != nil {
		return fmt.Errorf("audit-sweep: create new output directory: %w", err)
	}
	corpus := filepath.Join(*dest, "corpus")
	if err := os.Mkdir(corpus, 0o700); err != nil {
		return err
	}
	m := sweepManifest{
		Version: 1, Root: absRoot, Endpoint: conn.BaseURL, AgyVersion: agyVersion(),
		Started: time.Now().UTC(), Selected: len(sessions), Sessions: []sweepSession{},
	}
	if err := writeSweepManifest(*dest, m); err != nil {
		return err
	}
	client := newDaemonClient(absRoot, conn)
	client.HTTP.Timeout = *timeout
	for _, s := range sessions {
		fetchCtx, cancel := context.WithTimeout(ctx, *timeout)
		// Deliberately bypass fetchTrajectory: audit evidence must never fall
		// back to an existing sidecar when the live daemon fails.
		t, fetchErr := client.FetchTrajectoryWithFallback(fetchCtx, s.CascadeID, s.InternalCascadeID)
		cancel()
		if fetchErr == nil && daemon.CanonicalCascadeID(t.CascadeID) == "" {
			fetchErr = fmt.Errorf("daemon returned no valid trajectory identity")
		}
		if fetchErr == nil {
			_, fetchErr = render.Markdown(io.Discard, t, time.Now())
		}
		if fetchErr == nil {
			fetchErr = os.WriteFile(filepath.Join(corpus, s.CascadeID+".trajectory.json"), t.RawJSON, 0o600)
		}
		if fetchErr != nil {
			m.FailedID = s.CascadeID
			m.Finished = time.Now().UTC()
			if err := writeSweepManifest(*dest, m); err != nil {
				return fmt.Errorf("audit-sweep: fetch failed (%v); write incomplete manifest: %w", fetchErr, err)
			}
			return fmt.Errorf("audit-sweep: %s: %w; incomplete corpus retained at %s (do not use --corpus-swept)", s.CascadeID, fetchErr, *dest)
		}
		m.Sessions = append(m.Sessions, sweepSession{s.CascadeID, t.CascadeID, len(t.Steps)})
		if len(m.Sessions)%40 == 0 {
			if _, err := fmt.Fprintf(out, "audit-sweep: fetched and rendered %d/%d\n", len(m.Sessions), m.Selected); err != nil {
				return err
			}
		}
	}
	m.Complete = true
	m.Finished = time.Now().UTC()
	if err := writeSweepManifest(*dest, m); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "audit-sweep: selected=%d fetched=%d rendered=%d failures=0 corpus=%s\n", m.Selected, len(m.Sessions), len(m.Sessions), corpus)
	return err
}

func writeSweepManifest(dest string, m sweepManifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	// Publish completeness atomically; an interrupted run retains an
	// incomplete manifest rather than a truncated or falsely complete one.
	tmp := filepath.Join(dest, "manifest.json.tmp")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dest, "manifest.json"))
}
