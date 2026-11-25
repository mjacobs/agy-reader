package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/render"
)

const offlineMarkdownNotice = "> Offline SQLite reconstruction; not daemon-verified.\n\n"

var offlineUUIDInTextRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// RunExportOffline reads one native session without discovery, RPCs, or sidecars.
func RunExportOffline(ctx context.Context, argv []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("export-offline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "md", "Output format: md or json")
	out := fs.String("out", "", "New output file (default stdout; existing paths refused)")
	withChildren := fs.Bool("with-children", false, "Nest resolved child transcripts from sibling session files (md only)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: agy-reader export-offline [--format=md|json] [--out=NEW_FILE] [--with-children] SESSION.db|SESSION.pb")
		fs.PrintDefaults()
	}
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("export-offline requires one session source path")
	}
	if *format != "md" && *format != "json" {
		return errors.New("export-offline format must be md or json")
	}
	if *withChildren && *format == "json" {
		return errors.New("export-offline --with-children is only supported with --format=md")
	}
	path := fs.Arg(0)
	if !offlineSourceName(path) {
		return errors.New("export-offline requires a .db or .pb session")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("export-offline source must be a regular file")
	}
	snapshot, err := completeness.ReadOfflineSnapshot(ctx, path)
	if err != nil {
		return fmt.Errorf("read offline source: %w", err)
	}
	data, err := snapshot.OfflineJSON()
	if err != nil {
		return err
	}
	if *format == "md" {
		var trajectory daemon.Trajectory
		if err = json.Unmarshal(data, &trajectory); err != nil {
			return fmt.Errorf("render offline trajectory: %w", err)
		}
		var resolver render.SubagentResolver
		if *withChildren {
			resolver, err = buildOfflineSubagentResolver(ctx, path, info, &trajectory, stderr)
			if err != nil {
				return err
			}
		}
		var buf bytes.Buffer
		if snapshot.Profile == completeness.ImplicitProfile {
			buf.WriteString("> Offline implicit background trajectory; not daemon-verified.\n\n")
		} else if filepath.Ext(path) == ".pb" {
			buf.WriteString("> Offline protobuf reconstruction; not daemon-verified.\n\n")
		} else {
			buf.WriteString(offlineMarkdownNotice)
		}
		if snapshot.OmittedExecutors > 0 {
			fmt.Fprintf(&buf, "> Transcript only: %d executor metadata records omitted (source size limit).\n\n", snapshot.OmittedExecutors)
		}
		if _, err = render.MarkdownTree(&buf, &trajectory, time.Time{}, resolver); err != nil {
			return err
		}
		data = buf.Bytes()
	} else {
		data = append(data, '\n')
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if *out != "" {
		return writeNewExport(*out, bytes.NewReader(data))
	}
	_, err = io.Copy(stdout, bytes.NewReader(data))
	return err
}

func offlineSourceName(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".db") || filepath.Ext(path) == ".pb"
}

func readOfflineJSON(ctx context.Context, path string) ([]byte, error) {
	snapshot, err := completeness.ReadOfflineSnapshot(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("read offline source: %w", err)
	}
	return snapshot.OfflineJSON()
}

type offlineSubagentResolver struct {
	children map[string][]*daemon.Trajectory
}

func (r *offlineSubagentResolver) Children(cascadeID string) []*daemon.Trajectory {
	if r == nil {
		return nil
	}
	return r.children[daemon.CanonicalCascadeID(cascadeID)]
}

func (*offlineSubagentResolver) ChildPlacementEligible(parentCascadeID string, step daemon.Step) bool {
	return !step.IsInheritedFromAnotherCascade(parentCascadeID)
}

func buildOfflineSubagentResolver(ctx context.Context, primaryPath string, primaryInfo os.FileInfo, primary *daemon.Trajectory, stderr io.Writer) (render.SubagentResolver, error) {
	dir := filepath.Dir(primaryPath)
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read sibling sessions in %s: %w", dir, err)
	}
	entries := map[string]*daemon.Trajectory{}
	entryPaths := map[string]string{}
	if primaryID := daemon.CanonicalCascadeID(primary.CascadeID); primaryID != "" {
		entries[primaryID] = primary
		entryPaths[primaryID] = primaryPath
	}
	// ReadDir sorts names, so a migrated SQLite sibling sorts before its
	// same-stem .pb archive. Keep the source the caller selected for the root.
	primaryName := filepath.Base(primaryPath)
	primaryExt := filepath.Ext(primaryName)
	selectedFormat := map[string]string{strings.TrimSuffix(primaryName, primaryExt): strings.ToLower(primaryExt)}
	for _, entry := range dirEntries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry.IsDir() || !offlineSourceName(entry.Name()) {
			continue
		}
		ext := filepath.Ext(entry.Name())
		stem := strings.TrimSuffix(entry.Name(), ext)
		format := strings.ToLower(ext)
		if selected, ok := selectedFormat[stem]; ok && selected != format {
			continue
		}
		siblingPath := filepath.Join(dir, entry.Name())
		siblingInfo, err := os.Stat(siblingPath)
		if err != nil {
			warnOffline(stderr, "skipping sibling %s: %v", siblingPath, err)
			continue
		}
		if os.SameFile(primaryInfo, siblingInfo) {
			continue
		}
		if !siblingInfo.Mode().IsRegular() {
			warnOffline(stderr, "skipping sibling %s: not a regular file", siblingPath)
			continue
		}
		// A refused migrated database still supersedes the older archive.
		selectedFormat[stem] = format
		data, err := readOfflineJSON(ctx, siblingPath)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			warnOffline(stderr, "skipping sibling %s: %v", siblingPath, err)
			continue
		}
		var traj daemon.Trajectory
		if err = json.Unmarshal(data, &traj); err != nil {
			warnOffline(stderr, "skipping sibling %s: decode offline trajectory: %v", siblingPath, err)
			continue
		}
		id := daemon.CanonicalCascadeID(traj.CascadeID)
		if id == "" {
			warnOffline(stderr, "skipping sibling %s: invalid cascadeId %q", siblingPath, traj.CascadeID)
			continue
		}
		if priorPath, exists := entryPaths[id]; exists {
			warnOffline(stderr, "skipping sibling %s: duplicate cascadeId %s (already loaded from %s)", siblingPath, id, priorPath)
			continue
		}
		entries[id] = &traj
		entryPaths[id] = siblingPath
	}
	return resolveOfflineChildren(entries, stderr), nil
}

func resolveOfflineChildren(entries map[string]*daemon.Trajectory, stderr io.Writer) *offlineSubagentResolver {
	evidence := map[string]map[string]map[string]bool{}
	for child, traj := range entries {
		for _, parent := range traj.AgentPathParentCascadeIDs() {
			addOfflineEvidence(evidence, child, "agentPath", parent)
		}
	}
	collectOfflineInvocationEvidence(entries, evidence)

	native := map[string]map[string]map[string]bool{}
	collectOfflineNativeEvidence(entries, native)
	for child, sources := range native {
		evidence[child] = sources
	}

	ids := make([]string, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	parents := map[string]string{}
	for _, child := range ids {
		candidates := offlineCandidates(evidence[child])
		if len(candidates) == 1 {
			parents[child] = candidates[0]
		} else if len(candidates) > 1 {
			warnOffline(stderr, "excluding %s from child tree: conflicting parent evidence", child)
		}
	}

	cycles := offlineCyclicNodes(parents)
	index := map[string][]*daemon.Trajectory{}
	for _, id := range ids {
		parent := parents[id]
		if parent == "" {
			continue
		}
		if cycles[id] {
			warnOffline(stderr, "excluding %s from child tree: parent link forms a cycle", id)
			continue
		}
		index[parent] = append(index[parent], entries[id])
	}
	for parent := range index {
		sortOfflineChildren(index[parent])
	}
	return &offlineSubagentResolver{children: index}
}

func addOfflineEvidence(e map[string]map[string]map[string]bool, child, source, parent string) {
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

func offlineCandidates(bySource map[string]map[string]bool) []string {
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

func collectOfflineInvocationEvidence(entries map[string]*daemon.Trajectory, evidence map[string]map[string]map[string]bool) {
	for parent, traj := range entries {
		pendingPrompts := map[string]bool{}
		for _, step := range traj.Steps {
			if step.PlannerResponse == nil || step.IsInheritedFromAnotherCascade(parent) {
				continue
			}
			for _, child := range offlineUUIDInTextRe.FindAllString(step.PlannerResponse.ReplyText(), -1) {
				child = daemon.CanonicalCascadeID(child)
				childTraj := entries[child]
				if childTraj == nil || child == parent {
					continue
				}
				prompt := offlineFirstUserPrompt(childTraj)
				if prompt != "" && pendingPrompts[prompt] {
					addOfflineEvidence(evidence, child, "invoke_subagent", parent)
				}
			}
			clear(pendingPrompts)
			for _, call := range step.PlannerResponse.ToolCalls {
				if !isOfflineInvokeTool(call.Name) {
					continue
				}
				for _, prompt := range offlineInvocationPrompts(call.ArgumentsJSON) {
					pendingPrompts[prompt] = true
				}
			}
		}
	}
}

func isOfflineInvokeTool(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	return normalized == "invoke_subagent" || normalized == "spawnagents" || normalized == "spawn_agents"
}

func offlineInvocationPrompts(argumentsJSON string) []string {
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

func offlineFirstUserPrompt(t *daemon.Trajectory) string {
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

func collectOfflineNativeEvidence(entries map[string]*daemon.Trajectory, evidence map[string]map[string]map[string]bool) {
	for id, traj := range entries {
		if parent := entries[traj.WorkflowParentCascadeID()]; parent != nil {
			if index, ok := traj.WorkflowStep(parent); ok && !parent.Steps[index].IsInheritedFromAnotherCascade(parent.CascadeID) {
				addOfflineEvidence(evidence, id, "sourceMetadata.tool", parent.CascadeID)
			}
		}
		var parent string
		if json.Unmarshal(traj.Metadata.ParentConversationID, &parent) == nil {
			addOfflineEvidence(evidence, id, "parentConversationId", parent)
		}
		for _, step := range traj.Steps {
			if step.Type != "CORTEX_STEP_TYPE_INVOKE_SUBAGENT" || step.IsInheritedFromAnotherCascade(id) {
				continue
			}
			details, err := step.InvokeSubagentDetails()
			if err != nil {
				continue
			}
			for _, result := range details.Results {
				addOfflineEvidence(evidence, result.ConversationID, "invokeSubagent.results", id)
			}
		}
	}
}

func offlineCyclicNodes(parents map[string]string) map[string]bool {
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

func sortOfflineChildren(children []*daemon.Trajectory) {
	sort.SliceStable(children, func(i, j int) bool {
		ti, tj := children[i].FirstStepTime(), children[j].FirstStepTime()
		pi, errI := time.Parse(time.RFC3339Nano, ti)
		pj, errJ := time.Parse(time.RFC3339Nano, tj)
		validI, validJ := errI == nil, errJ == nil
		if validI != validJ {
			return validI
		}
		if validI && !pi.Equal(pj) {
			return pi.Before(pj)
		}
		if !validI && ti != tj {
			return ti < tj
		}
		ci, cj := daemon.CanonicalCascadeID(children[i].CascadeID), daemon.CanonicalCascadeID(children[j].CascadeID)
		if ci != cj {
			return ci < cj
		}
		return children[i].CascadeID < children[j].CascadeID
	})
}

func warnOffline(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "warning: "+format+"\n", args...)
}

// Create a new file without replacing any existing directory entry, including
// dangling symlinks. Encode and validate the transcript before calling this.
// Remove the new file on a reported write/close failure; process termination
// during a write can still leave a partial file.
func writeNewExport(path string, data io.Reader) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create new export: %w", err)
	}
	_, err = io.Copy(f, data)
	if err = CloseOutput(f, err); err != nil {
		return errors.Join(err, os.Remove(path))
	}
	return nil
}
