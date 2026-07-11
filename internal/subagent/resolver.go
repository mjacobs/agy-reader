// Package subagent resolves the parent->child delegation tree among the
// trajectories in one conversations directory. Linkage is child-side: every
// SUBAGENT trajectory names its parent's cascade id inside an agentPath URI
// (see daemon.Trajectory.ParentCascadeID), so we scan the sibling sidecar
// files, read each, and invert the child->parent edges into a
// parent->children index.
//
// Resolution is sidecar-based (read from disk) rather than daemon-backed on
// purpose: it works offline, and — unlike a parent's own sidecar, which can be
// stale/truncated — each child's sidecar is the freshest complete record of
// that child we have. The render package consumes this via its
// SubagentResolver interface.
package subagent

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/daemon"
)

// Resolver holds a parent-cascade-id -> child-trajectories index built from a
// single conversations directory. The zero value is not usable; build one with
// Build. A nil *Resolver is safe to call Children on (returns nil), so callers
// can treat "no resolver" and "resolver with no children" uniformly.
type Resolver struct {
	children map[string][]*daemon.Trajectory
}

// Build scans dir for *.trajectory.json sidecars, reads each, and returns a
// Resolver indexing every trajectory by its parent's cascade id. Sidecars that
// can't be read or parsed are skipped and logged to logw (nil silences the
// log). Children of a given parent are sorted by first-step timestamp, falling
// back to cascade id, so ordering is stable across runs.
func Build(dir string, logw io.Writer) (*Resolver, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.trajectory.json"))
	if err != nil {
		return nil, fmt.Errorf("glob sidecars in %s: %w", dir, err)
	}
	index := map[string][]*daemon.Trajectory{}
	for _, p := range paths {
		traj, err := cache.Read(p)
		if err != nil {
			logf(logw, "subagent: skip unreadable sidecar %s: %v", p, err)
			continue
		}
		parent := traj.ParentCascadeID()
		if parent == "" {
			continue // a root (or an unlinkable built-in-path subagent)
		}
		index[parent] = append(index[parent], traj)
	}
	for parent := range index {
		sortChildren(index[parent])
	}
	return &Resolver{children: index}, nil
}

// Children returns the child trajectories of cascadeID in stable order, or nil
// when it has none. Safe to call on a nil *Resolver.
func (r *Resolver) Children(cascadeID string) []*daemon.Trajectory {
	if r == nil || cascadeID == "" {
		return nil
	}
	return r.children[cascadeID]
}

// sortChildren orders children by first-step timestamp then cascade id. The
// timestamps are raw RFC3339Nano strings, which sort chronologically as text.
func sortChildren(children []*daemon.Trajectory) {
	sort.SliceStable(children, func(i, j int) bool {
		ti, tj := children[i].FirstStepTime(), children[j].FirstStepTime()
		if ti != tj {
			return ti < tj
		}
		return children[i].CascadeID < children[j].CascadeID
	})
}

func logf(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, format+"\n", args...)
}
