package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mjacobs/agy-reader/internal/subagent"
)

// runBackfillParentLinks implements the one-shot historical sidecar migration.
// It never contacts a daemon: the same directory-wide evidence resolver used
// after normal sync operates entirely on existing sibling sidecars.
func runBackfillParentLinks(args []string) error {
	return runBackfillParentLinksTo(args, os.Stderr)
}

func runBackfillParentLinksTo(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("backfill-parent-links", flag.ContinueOnError)
	fs.SetOutput(out)
	var rootFlags rootsFlag
	fs.Var(&rootFlags, "root", "Antigravity session root (repeatable; defaults to discovered roots)")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(out, "usage: agy-reader backfill-parent-links [--root=PATH]...")
		_, _ = fmt.Fprintln(out, "scan existing <root>/conversations/*.trajectory.json sidecars and stamp unambiguous immediate-parent links")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("backfill-parent-links: unexpected arguments: %v", fs.Args())
	}
	roots, err := resolveRoots(rootFlags)
	if err != nil {
		return err
	}
	for _, root := range roots {
		dir := filepath.Join(root, "conversations")
		report, err := subagent.Backfill(dir, out)
		if err != nil {
			return fmt.Errorf("backfill parent links under %s: %w", root, err)
		}
		if _, err := fmt.Fprintf(out,
			"backfill-parent-links: root=%s scanned=%d stamped=%d unchanged=%d unresolved=%d diagnostics=%d\n",
			root, report.Scanned, report.Stamped, report.Unchanged, report.Unresolved, len(report.Diagnostics),
		); err != nil {
			return fmt.Errorf("write backfill summary: %w", err)
		}
	}
	return nil
}
