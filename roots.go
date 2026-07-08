package main

import (
	"path/filepath"
	"strings"

	"github.com/mjacobs/agy-reader/internal/discovery"
)

// rootsFlag collects repeated --root values in the order they appear.
type rootsFlag []string

func (f *rootsFlag) String() string { return strings.Join(*f, ",") }

func (f *rootsFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}

// resolveRoots turns explicit --root values into the ordered list of session
// roots this run operates on. Explicit roots win outright and suppress
// discovery; duplicates collapse so a repeated --root is not listed or synced
// twice. With no explicit roots, resolution falls back to the configured or
// default root.
func resolveRoots(explicit []string) ([]string, error) {
	if len(explicit) > 0 {
		seen := map[string]bool{}
		roots := []string{}
		for _, r := range explicit {
			r = filepath.Clean(r)
			if seen[r] {
				continue
			}
			seen[r] = true
			roots = append(roots, r)
		}
		return roots, nil
	}
	root, err := discovery.Root()
	if err != nil {
		return nil, err
	}
	return []string{root}, nil
}
