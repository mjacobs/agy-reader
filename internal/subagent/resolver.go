// Package subagent resolves the parent->child delegation tree among the
// trajectories in one conversations directory. It combines reader-owned
// stamps, native parent/invocation metadata, and legacy directional evidence,
// then inverts the resolved child->parent edges into a parent->children index.
//
// Resolution is sidecar-based (read from disk) rather than daemon-backed on
// purpose: it works offline, and — unlike a parent's own sidecar, which can be
// stale/truncated — each child's sidecar is the freshest complete record of
// that child we have. The render package consumes this via its
// SubagentResolver interface.
package subagent

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

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

const (
	// DiagnosticConflict means two independently strong evidence sources name
	// different parents. The child is intentionally left unstamped.
	DiagnosticConflict = "conflicting-evidence"
	// DiagnosticStaleStamp means the authoritative existing stamp disagrees
	// with live evidence. Backfill reports it but never overwrites it silently.
	DiagnosticStaleStamp = "stale-stamp"
	// DiagnosticMissingParent means a valid direct pointer names a parent whose
	// sidecar is not in this corpus. The pointer is still safe to preserve.
	DiagnosticMissingParent = "missing-parent"
	// DiagnosticUnreadable means one sidecar could not be parsed and was
	// skipped without failing the rest of the corpus.
	DiagnosticUnreadable = "unreadable-sidecar"
	// DiagnosticInvalidCascade means the sidecar does not identify itself with
	// a bare cascade UUID.
	DiagnosticInvalidCascade = "invalid-cascade-id"
	// DiagnosticInvalidStamp means agyReader.parentCascadeId is non-empty but
	// not a bare cascade UUID. Strong live evidence may repair it safely.
	DiagnosticInvalidStamp = "invalid-parent-stamp"
	// DiagnosticCycle marks every node whose selected parent edge forms a cycle.
	DiagnosticCycle = "parent-cycle"
)

const (
	sourceAgentPath  = "agentPath"
	sourceInvocation = "invoke_subagent"
	sourceParent     = "parentConversationId"
	sourceResults    = "invokeSubagent.results"
)

var uuidInTextRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// Diagnostic is one deterministic, non-fatal relationship finding.
type Diagnostic struct {
	CascadeID string
	Kind      string
	Message   string
}

// BackfillReport summarizes a corpus resolution/stamping pass.
type BackfillReport struct {
	Scanned     int
	Stamped     int
	Cleared     int
	Unchanged   int
	Unresolved  int
	Diagnostics []Diagnostic
}

type corpusEntry struct {
	path string
	traj *daemon.Trajectory
}

// BackfillOptions controls the explicit historical repair operation.
type BackfillOptions struct {
	// Repair recomputes links without trusting existing stamps. Unsupported,
	// conflicting, and cyclic pointers are removed. Use a complete corpus.
	Repair bool
}

// Backfill resolves immediate parent relationships across all sibling
// sidecars in dir, then atomically stamps only unambiguous child pointers.
// It is also the second pass used by normal sync/watch. Unreadable sidecars,
// missing parents, and relationship conflicts are diagnostic rather than
// corpus-fatal; only directory-level failures are returned as errors.
func Backfill(dir string, logw io.Writer) (BackfillReport, error) {
	return BackfillWithOptions(dir, logw, BackfillOptions{})
}

// BackfillWithOptions also supports recomputing previously stamped links.
// Default operation diagnoses stale stamps without overwriting them.
func BackfillWithOptions(dir string, logw io.Writer, opts BackfillOptions) (BackfillReport, error) {
	paths, err := sidecarPaths(dir)
	if err != nil {
		return BackfillReport{}, err
	}
	report := BackfillReport{Scanned: len(paths)}
	entries := map[string]*corpusEntry{}
	for _, path := range paths {
		traj, err := cache.Read(path)
		if err != nil {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{
				CascadeID: cascadeFromPath(path), Kind: DiagnosticUnreadable,
				Message: fmt.Sprintf("cannot read %s: %v", path, err),
			})
			continue
		}
		cascadeID := daemon.CanonicalCascadeID(traj.CascadeID)
		if cascadeID == "" {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{
				CascadeID: traj.CascadeID, Kind: DiagnosticInvalidCascade,
				Message: fmt.Sprintf("sidecar %s has invalid cascadeId %q", path, traj.CascadeID),
			})
			continue
		}
		if prior := entries[cascadeID]; prior != nil {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{
				CascadeID: cascadeID, Kind: DiagnosticConflict,
				Message: fmt.Sprintf("duplicate cascadeId in %s and %s", prior.path, path),
			})
			continue
		}
		entries[cascadeID] = &corpusEntry{path: path, traj: traj}
	}

	// child -> source -> candidate parent set
	evidence := map[string]map[string]map[string]bool{}
	for child, entry := range entries {
		if entry.traj.AgyReader != nil && entry.traj.AgyReader.ParentCascadeID != "" && entry.traj.StampedParentCascadeID() == "" {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{
				CascadeID: child, Kind: DiagnosticInvalidStamp,
				Message: fmt.Sprintf("invalid agyReader.parentCascadeId %q", entry.traj.AgyReader.ParentCascadeID),
			})
		}
		for _, parent := range entry.traj.AgentPathParentCascadeIDs() {
			addEvidence(evidence, child, sourceAgentPath, parent)
		}
	}
	collectInvocationEvidence(entries, evidence)
	// Native metadata outranks legacy heuristics. Conflicting native sources
	// prevent new stamps; repair clears old stamps rather than breaking ties.
	native := map[string]map[string]map[string]bool{}
	collectNativeEvidence(entries, native)
	for child, sources := range native {
		evidence[child] = sources
	}

	ids := make([]string, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	// Resolve the entire graph before writing anything so scan order cannot
	// allow the final edge of a cycle to be stamped.
	parents := map[string]string{}
	for _, child := range ids {
		entry := entries[child]
		stamped := entry.traj.StampedParentCascadeID()
		candidates := allCandidates(evidence[child])
		if stamped != "" && !opts.Repair {
			parents[child] = stamped
			if len(candidates) > 0 && (len(candidates) != 1 || candidates[0] != stamped) {
				report.Diagnostics = append(report.Diagnostics, Diagnostic{
					CascadeID: child, Kind: DiagnosticStaleStamp,
					Message: fmt.Sprintf("existing parent %s disagrees with %s; use --repair to recompute", stamped, formatEvidence(evidence[child])),
				})
			}
		} else if len(candidates) == 1 {
			parents[child] = candidates[0]
		} else if len(candidates) > 1 {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{
				CascadeID: child, Kind: DiagnosticConflict,
				Message: "candidate parents disagree: " + formatEvidence(evidence[child]),
			})
		}
	}
	cycles := cyclicNodes(parents)
	for _, child := range ids {
		entry := entries[child]
		stamped := entry.traj.StampedParentCascadeID()
		parent := parents[child]
		if cycles[child] {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{
				CascadeID: child, Kind: DiagnosticCycle,
				Message: "parent edge forms a cycle; no new stamp written (use --repair for existing stamps)",
			})
			parent = ""
		}
		if stamped != "" && !opts.Repair {
			report.Unchanged++
			if entries[stamped] == nil {
				report.Diagnostics = append(report.Diagnostics, missingParentDiagnostic(child, stamped))
			}
			continue
		}
		if parent == "" {
			report.Unresolved++
			if opts.Repair {
				changed, err := cache.ClearParentCascadeID(entry.path)
				if err != nil {
					report.Diagnostics = append(report.Diagnostics, Diagnostic{
						CascadeID: child, Kind: DiagnosticUnreadable, Message: fmt.Sprintf("cannot clear parent: %v", err),
					})
				} else if changed {
					report.Cleared++
				}
			}
			continue
		}
		changed, err := cache.StampParentCascadeID(entry.path, parent)
		if err != nil {
			report.Unresolved++
			report.Diagnostics = append(report.Diagnostics, Diagnostic{
				CascadeID: child, Kind: DiagnosticUnreadable,
				Message: fmt.Sprintf("cannot stamp %s: %v", entry.path, err),
			})
			continue
		}
		if changed {
			report.Stamped++
		} else {
			report.Unchanged++
		}
		if entries[parent] == nil {
			report.Diagnostics = append(report.Diagnostics, missingParentDiagnostic(child, parent))
		}
	}

	sort.SliceStable(report.Diagnostics, func(i, j int) bool {
		a, b := report.Diagnostics[i], report.Diagnostics[j]
		if a.CascadeID != b.CascadeID {
			return a.CascadeID < b.CascadeID
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Message < b.Message
	})
	for _, d := range report.Diagnostics {
		logf(logw, "subagent: %s %s: %s", d.CascadeID, d.Kind, d.Message)
	}
	return report, nil
}

func cascadeFromPath(path string) string {
	return daemon.CanonicalCascadeID(strings.TrimSuffix(filepath.Base(path), ".trajectory.json"))
}

func sidecarPaths(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read sidecars in %s: %w", dir, err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".trajectory.json") {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(paths)
	return paths, nil
}

func missingParentDiagnostic(child, parent string) Diagnostic {
	return Diagnostic{
		CascadeID: child, Kind: DiagnosticMissingParent,
		Message: fmt.Sprintf("parent %s has no sibling sidecar", parent),
	}
}

func addEvidence(e map[string]map[string]map[string]bool, child, source, parent string) {
	child = daemon.CanonicalCascadeID(child)
	parent = daemon.CanonicalCascadeID(parent)
	if child == "" || parent == "" {
		return
	}
	if e[child] == nil {
		e[child] = map[string]map[string]bool{}
	}
	if e[child][source] == nil {
		e[child][source] = map[string]bool{}
	}
	e[child][source][parent] = true
}

func allCandidates(bySource map[string]map[string]bool) []string {
	seen := map[string]bool{}
	for _, parents := range bySource {
		for parent := range parents {
			seen[parent] = true
		}
	}
	out := make([]string, 0, len(seen))
	for parent := range seen {
		out = append(out, parent)
	}
	sort.Strings(out)
	return out
}

func formatEvidence(bySource map[string]map[string]bool) string {
	sources := make([]string, 0, len(bySource))
	for source := range bySource {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		parents := make([]string, 0, len(bySource[source]))
		for parent := range bySource[source] {
			parents = append(parents, parent)
		}
		sort.Strings(parents)
		parts = append(parts, source+"=["+strings.Join(parents, ",")+"]")
	}
	return strings.Join(parts, " ")
}

// collectInvocationEvidence accepts a child id from the next parent-side
// planner response only when the child's first user input exactly matches a
// pending invoke_subagent prompt. agy may report completed child ids under a
// later execution id, so execution ids are not a stable boundary for this
// evidence. Consuming pending prompts after one response prevents them from
// matching unrelated cascades later in the session.
func collectInvocationEvidence(entries map[string]*corpusEntry, evidence map[string]map[string]map[string]bool) {
	for parent, entry := range entries {
		pendingPrompts := map[string]bool{}
		for _, step := range entry.traj.Steps {
			if step.PlannerResponse == nil {
				continue
			}

			resultText := step.PlannerResponse.Response
			for _, child := range uuidInTextRe.FindAllString(resultText, -1) {
				child = daemon.CanonicalCascadeID(child)
				childEntry := entries[child]
				if childEntry == nil || child == parent {
					continue
				}
				prompt := firstUserPrompt(childEntry.traj)
				if prompt != "" && pendingPrompts[prompt] {
					addEvidence(evidence, child, sourceInvocation, parent)
				}
			}

			clear(pendingPrompts)
			for _, call := range step.PlannerResponse.ToolCalls {
				if !isInvokeTool(call.Name) {
					continue
				}
				for _, prompt := range invocationPrompts(call.ArgumentsJSON) {
					pendingPrompts[prompt] = true
				}
			}
		}
	}
}

func isInvokeTool(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	return normalized == "invoke_subagent" || normalized == "spawnagents" || normalized == "spawn_agents"
}

func invocationPrompts(argumentsJSON string) []string {
	var args struct {
		Subagents []struct {
			Prompt string `json:"prompt"`
		} `json:"subagents"`
	}
	if json.Unmarshal([]byte(argumentsJSON), &args) != nil {
		return nil
	}
	out := make([]string, 0, len(args.Subagents))
	for _, child := range args.Subagents {
		if prompt := strings.TrimSpace(child.Prompt); prompt != "" {
			out = append(out, prompt)
		}
	}
	return out
}

func firstUserPrompt(t *daemon.Trajectory) string {
	if t == nil {
		return ""
	}
	for _, step := range t.Steps {
		if step.UserInput != nil {
			if prompt := strings.TrimSpace(step.UserInput.UserResponse); prompt != "" {
				return prompt
			}
		}
	}
	return ""
}

// collectNativeEvidence reads daemon-owned directional relationships. Message
// delivery proves communication, never ancestry, and is deliberately ignored.
func collectNativeEvidence(entries map[string]*corpusEntry, evidence map[string]map[string]map[string]bool) {
	for id, entry := range entries {
		var parent string
		if json.Unmarshal(entry.traj.Metadata.ParentConversationID, &parent) == nil {
			addEvidence(evidence, id, sourceParent, parent)
		}
		for _, step := range entry.traj.Steps {
			if step.Type != "CORTEX_STEP_TYPE_INVOKE_SUBAGENT" {
				continue
			}
			var invocation struct {
				Results []struct {
					ConversationID string `json:"conversationId"`
				} `json:"results"`
			}
			if json.Unmarshal(step.InvokeSubagent, &invocation) != nil {
				continue
			}
			for _, result := range invocation.Results {
				addEvidence(evidence, result.ConversationID, sourceResults, id)
			}
		}
	}
}

// cyclicNodes finds cycle members in a child->parent graph in linear time.
// Ancestors missing from the corpus and paths leading into cycles terminate
// safely; only edges in the cycle itself are rejected.
func cyclicNodes(parents map[string]string) map[string]bool {
	cycles, done := map[string]bool{}, map[string]bool{}
	for start := range parents {
		path := []string{}
		position := map[string]int{}
		for id := start; id != "" && !done[id]; id = parents[id] {
			if at, ok := position[id]; ok {
				for _, member := range path[at:] {
					cycles[member] = true
				}
				break
			}
			position[id] = len(path)
			path = append(path, id)
		}
		for _, id := range path {
			done[id] = true
		}
	}
	return cycles
}

// Build scans dir for *.trajectory.json sidecars, reads each, and returns a
// Resolver indexing stamped relationships by parent cascade id. Backfill must
// run first: Build deliberately ignores legacy guesses so a relationship that
// reconciliation rejected as conflicting cannot still leak into rendering.
// Sidecars that
// can't be read or parsed are skipped and logged to logw (nil silences the
// log). Children of a given parent are sorted by first-step timestamp, falling
// back to cascade id, so ordering is stable across runs.
func Build(dir string, logw io.Writer) (*Resolver, error) {
	paths, err := sidecarPaths(dir)
	if err != nil {
		return nil, err
	}
	index := map[string][]*daemon.Trajectory{}
	parents := map[string]string{}
	linked := map[string]*daemon.Trajectory{}
	for _, p := range paths {
		traj, err := cache.Read(p)
		if err != nil {
			logf(logw, "subagent: skip unreadable sidecar %s: %v", p, err)
			continue
		}
		parent := traj.StampedParentCascadeID()
		if parent == "" {
			continue // a root (or an unlinkable built-in-path subagent)
		}
		id := daemon.CanonicalCascadeID(traj.CascadeID)
		if id == "" {
			continue
		}
		parents[id] = parent
		linked[id] = traj
	}
	cycles := cyclicNodes(parents)
	for id, traj := range linked {
		if cycles[id] {
			logf(logw, "subagent: %s parent-cycle: excluded from render tree", id)
			continue
		}
		parent := parents[id]
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
	if r == nil {
		return nil
	}
	return r.children[daemon.CanonicalCascadeID(cascadeID)]
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
	_, _ = fmt.Fprintf(w, format+"\n", args...)
}
