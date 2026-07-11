package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mjacobs/agy-reader/internal/shapefp"
)

// runShapeFingerprint implements the `shape-fingerprint` subcommand: it
// computes the canonical trajectory shape fingerprint (see internal/shapefp)
// over the union of the given sidecars. Arguments are sidecar files and/or
// directories (a directory contributes its *.trajectory.json entries,
// non-recursively — sidecars live flat in conversations/). It powers the
// sidecar-shape check in the agy-format-audit skill; agentsview can reproduce
// the same digest with the algorithm documented in that package.
//
// Output: the fingerprint line "sha256:<hex>" on stdout. With --paths, the
// canonical "path<TAB>typeUnion" lines are printed instead (for diffing what
// changed on a DRIFT). Exit 3 when no sidecars are found (so the audit script
// can report "not computed" rather than treating it as drift/failure).
func runShapeFingerprint(argv []string) error {
	fs := flag.NewFlagSet("shape-fingerprint", flag.ContinueOnError)
	var showPaths bool
	fs.BoolVar(&showPaths, "paths", false, "Print the canonical path/type lines instead of the digest")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: agy-reader shape-fingerprint [--paths] <sidecar-or-dir>...")
		fmt.Fprintln(os.Stderr, "  Computes the canonical trajectory-JSON shape fingerprint over the")
		fmt.Fprintln(os.Stderr, "  union of the given *.trajectory.json sidecars (files and/or dirs).")
	}
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("shape-fingerprint: no sidecar files or directories given")
	}

	files, err := collectSidecars(fs.Args())
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "shape-fingerprint: no *.trajectory.json sidecars found")
		os.Exit(3)
	}

	docs := make([][]byte, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("shape-fingerprint: read %s: %w", f, err)
		}
		docs = append(docs, data)
	}

	fp, lines, err := shapefp.FingerprintDocs(docs)
	if err != nil {
		return err
	}

	if showPaths {
		for _, l := range lines {
			fmt.Println(l)
		}
		return nil
	}
	fmt.Println(fp)
	return nil
}

// collectSidecars expands the given file/dir arguments into a deduplicated,
// sorted list of sidecar file paths. Directories contribute their direct
// *.trajectory.json children; a file argument is taken as-is.
func collectSidecars(args []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, a := range args {
		info, err := os.Stat(a)
		if err != nil {
			return nil, fmt.Errorf("shape-fingerprint: %w", err)
		}
		if info.IsDir() {
			matches, err := filepath.Glob(filepath.Join(a, "*.trajectory.json"))
			if err != nil {
				return nil, fmt.Errorf("shape-fingerprint: glob %s: %w", a, err)
			}
			for _, m := range matches {
				add(m)
			}
			continue
		}
		add(a)
	}
	return out, nil
}
