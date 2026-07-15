// Package subagent resolves the parent->child delegation tree among the
// trajectories in one conversations directory. It combines reader-owned
// stamps, legacy child-side agentPath metadata, invocation results, and
// corroborated two-way message evidence, then inverts the resolved
// child->parent edges into a parent->children index.
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
)

const (
	sourceAgentPath  = "agentPath"
	sourceInvocation = "invoke_subagent"
	sourceMessage    = "corroborated-message"
)

var messageSenderRe = regexp.MustCompile(`(?:^|\s)sender=([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})(?:\s|$)`)
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
	Unchanged   int
	Unresolved  int
	Diagnostics []Diagnostic
}

type corpusEntry struct {
	path string
	traj *daemon.Trajectory
}

// Backfill resolves immediate parent relationships across all sibling
// sidecars in dir, then atomically stamps only unambiguous child pointers.
// It is also the second pass used by normal sync/watch. Unreadable sidecars,
// missing parents, and relationship conflicts are diagnostic rather than
// corpus-fatal; only directory-level failures are returned as errors.
func Backfill(dir string, logw io.Writer) (BackfillReport, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.trajectory.json"))
	if err != nil {
		return BackfillReport{}, fmt.Errorf("glob sidecars in %s: %w", dir, err)
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
		if !daemon.IsCascadeID(traj.CascadeID) {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{
				CascadeID: traj.CascadeID, Kind: DiagnosticInvalidCascade,
				Message: fmt.Sprintf("sidecar %s has invalid cascadeId %q", path, traj.CascadeID),
			})
			continue
		}
		if prior := entries[traj.CascadeID]; prior != nil {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{
				CascadeID: traj.CascadeID, Kind: DiagnosticConflict,
				Message: fmt.Sprintf("duplicate cascadeId in %s and %s", prior.path, path),
			})
			continue
		}
		entries[traj.CascadeID] = &corpusEntry{path: path, traj: traj}
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
	collectMessageEvidence(entries, evidence)

	ids := make([]string, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, child := range ids {
		entry := entries[child]
		stamped := entry.traj.StampedParentCascadeID()
		candidates := allCandidates(evidence[child])

		if stamped != "" {
			report.Unchanged++
			if len(candidates) > 0 && (len(candidates) != 1 || candidates[0] != stamped) {
				report.Diagnostics = append(report.Diagnostics, Diagnostic{
					CascadeID: child, Kind: DiagnosticStaleStamp,
					Message: fmt.Sprintf("existing parent %s disagrees with %s", stamped, formatEvidence(evidence[child])),
				})
			}
			if entries[stamped] == nil {
				report.Diagnostics = append(report.Diagnostics, missingParentDiagnostic(child, stamped))
			}
			continue
		}

		switch {
		case len(candidates) == 0:
			report.Unresolved++
			continue
		case len(candidates) > 1:
			report.Unresolved++
			report.Diagnostics = append(report.Diagnostics, Diagnostic{
				CascadeID: child, Kind: DiagnosticConflict,
				Message: "candidate parents disagree: " + formatEvidence(evidence[child]),
			})
			continue
		}

		parent := candidates[0]
		if parent == child {
			report.Unresolved++
			report.Diagnostics = append(report.Diagnostics, Diagnostic{
				CascadeID: child, Kind: DiagnosticConflict,
				Message: "relationship evidence points the cascade to itself",
			})
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
	return strings.TrimSuffix(filepath.Base(path), ".trajectory.json")
}

func missingParentDiagnostic(child, parent string) Diagnostic {
	return Diagnostic{
		CascadeID: child, Kind: DiagnosticMissingParent,
		Message: fmt.Sprintf("parent %s has no sibling sidecar", parent),
	}
}

func addEvidence(e map[string]map[string]map[string]bool, child, source, parent string) {
	if !daemon.IsCascadeID(child) || !daemon.IsCascadeID(parent) {
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

type invocationContext struct {
	firstIndex int
	prompts    map[string]bool
}

// collectInvocationEvidence accepts a child id from parent-side result text
// only when it is tied to the same execution as an invoke_subagent call and
// the child's first user input exactly matches one of that call's prompts.
// This is deliberately stricter than UUID/text adjacency.
func collectInvocationEvidence(entries map[string]*corpusEntry, evidence map[string]map[string]map[string]bool) {
	for parent, entry := range entries {
		contexts := map[string]*invocationContext{}
		for i, step := range entry.traj.Steps {
			execID := step.Metadata.ExecutionID
			if execID == "" {
				continue
			}
			if step.Type == "CORTEX_STEP_TYPE_INVOKE_SUBAGENT" {
				ensureInvocationContext(contexts, execID, i)
			}
			if step.PlannerResponse == nil {
				continue
			}
			for _, call := range step.PlannerResponse.ToolCalls {
				if !isInvokeTool(call.Name) {
					continue
				}
				ctx := ensureInvocationContext(contexts, execID, i)
				for _, prompt := range invocationPrompts(call.ArgumentsJSON) {
					ctx.prompts[prompt] = true
				}
			}
		}

		for i, step := range entry.traj.Steps {
			ctx := contexts[step.Metadata.ExecutionID]
			if ctx == nil || i <= ctx.firstIndex || step.PlannerResponse == nil {
				continue
			}
			resultText := step.PlannerResponse.Response
			for _, child := range uuidInTextRe.FindAllString(resultText, -1) {
				childEntry := entries[child]
				if childEntry == nil || child == parent {
					continue
				}
				prompt := firstUserPrompt(childEntry.traj)
				if prompt != "" && ctx.prompts[prompt] {
					addEvidence(evidence, child, sourceInvocation, parent)
				}
			}
		}
	}
}

func ensureInvocationContext(contexts map[string]*invocationContext, executionID string, index int) *invocationContext {
	ctx := contexts[executionID]
	if ctx == nil {
		ctx = &invocationContext{firstIndex: index, prompts: map[string]bool{}}
		contexts[executionID] = ctx
	} else if index < ctx.firstIndex {
		ctx.firstIndex = index
	}
	return ctx
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

// collectMessageEvidence requires both halves of the delivery: a child's
// send_message Recipient=<parent> generic step and the parent's inbound
// agent_message with sender=<child>. Either half alone is ambiguous.
func collectMessageEvidence(entries map[string]*corpusEntry, evidence map[string]map[string]map[string]bool) {
	outbound := map[string]map[string]bool{}
	inbound := map[string]map[string]bool{}
	for id, entry := range entries {
		for _, step := range entry.traj.Steps {
			if recipient := genericRecipient(step.Generic); daemon.IsCascadeID(recipient) {
				if outbound[id] == nil {
					outbound[id] = map[string]bool{}
				}
				outbound[id][recipient] = true
			}
			if step.SystemMessage == nil || !strings.EqualFold(step.SystemMessage.EventType, "agent_message") {
				continue
			}
			if match := messageSenderRe.FindStringSubmatch(step.SystemMessage.Message); match != nil {
				if inbound[id] == nil {
					inbound[id] = map[string]bool{}
				}
				inbound[id][match[1]] = true
			}
		}
	}
	for child, parents := range outbound {
		for parent := range parents {
			if inbound[parent][child] {
				addEvidence(evidence, child, sourceMessage, parent)
			}
		}
	}
}

func genericRecipient(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var generic struct {
		Args map[string]json.RawMessage `json:"args"`
	}
	if json.Unmarshal(raw, &generic) != nil {
		return ""
	}
	for key, value := range generic.Args {
		if !strings.EqualFold(key, "recipient") {
			continue
		}
		var recipient string
		if json.Unmarshal(value, &recipient) == nil {
			return recipient
		}
	}
	return ""
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
	_, _ = fmt.Fprintf(w, format+"\n", args...)
}
