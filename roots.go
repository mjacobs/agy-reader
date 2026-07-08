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

// findSessionRoot locates the root that holds session id on disk, searching
// roots in order (the first root wins when several hold the id). found=false
// means no root has the id on disk; fetchByID then probes each root's daemon
// instead.
func findSessionRoot(roots []string, id string) (root string, s discovery.Session, found bool, err error) {
	for _, r := range roots {
		s, found, err = discovery.FindByID(r, id)
		if err != nil {
			return "", discovery.Session{}, false, err
		}
		if found {
			return r, s, true, nil
		}
	}
	return "", discovery.Session{}, false, nil
}

// resolveRoots turns explicit --root values into the ordered list of session
// roots this run operates on. Explicit roots win outright and suppress
// discovery; duplicates collapse so a repeated --root is not listed or synced
// twice. With no explicit roots, resolution falls back to ANTIGRAVITY_CLI_ROOT
// or default-store discovery (see discovery.DefaultRoots).
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
	return discovery.DefaultRoots()
}
