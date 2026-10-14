package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mjacobs/agy-reader/internal/shapefp"
)

func RunShapeFingerprint(argv []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("shape-fingerprint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var showPaths bool
	fs.BoolVar(&showPaths, "paths", false, "Print the canonical path/type lines instead of the digest")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: agy-reader shape-fingerprint [--paths] <sidecar-or-dir>...")
		fmt.Fprintln(stderr, "  Computes the canonical trajectory-JSON shape fingerprint over the")
		fmt.Fprintln(stderr, "  union of the given *.trajectory.json sidecars (files and/or dirs).")
	}
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("shape-fingerprint: no sidecar files or directories given")
	}

	files, err := CollectSidecars(fs.Args())
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Fprintln(stderr, "shape-fingerprint: no *.trajectory.json sidecars found")
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
			fmt.Fprintln(stdout, l)
		}
		return nil
	}
	fmt.Fprintln(stdout, fp)
	return nil
}

func CollectSidecars(args []string) ([]string, error) {
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
			entries, err := os.ReadDir(a)
			if err != nil {
				return nil, fmt.Errorf("shape-fingerprint: read dir %s: %w", a, err)
			}
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".trajectory.json") {
					continue
				}
				add(filepath.Join(a, entry.Name()))
			}
			continue
		}
		add(a)
	}
	sort.Strings(out)
	return out, nil
}
