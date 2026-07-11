// Package render turns a decrypted Trajectory into Markdown. The output
// shape is a near-direct port of the renderer in
// ~/.gemini/antigravity-cli/scratch/read_trajectory.py so transcripts
// produced by either tool look similar.
package render

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

// SubagentResolver supplies the child (subagent) trajectories nested under a
// parent's INVOKE_SUBAGENT steps. Implementations live outside render (which
// stays pure — no filesystem walking): Children returns the already-fetched
// child trajectories of the given cascade id, in stable order, or nil for a
// leaf. See internal/subagent.Resolver.
type SubagentResolver interface {
	Children(cascadeID string) []*daemon.Trajectory
}

// maxSubagentDepth caps how deep the delegation tree renders, a backstop
// against pathological trees on top of the per-path cycle guard.
const maxSubagentDepth = 8

// Markdown renders t to w as Markdown. Returns the number of bytes written
// and any I/O error. now is used for the "Generated on" header so tests can
// pin it; pass time.Time{} to use time.Now().
func Markdown(w io.Writer, t *daemon.Trajectory, now time.Time) (int, error) {
	return MarkdownTree(w, t, now, nil)
}

// MarkdownTree renders t like Markdown, additionally nesting each child
// subagent transcript (resolved via r) inline under the parent's
// INVOKE_SUBAGENT steps, recursing into grandchildren. A nil resolver — or one
// that reports no children — produces output byte-identical to plain Markdown.
func MarkdownTree(w io.Writer, t *daemon.Trajectory, now time.Time, r SubagentResolver) (int, error) {
	if t == nil {
		return 0, errors.New("render markdown: nil trajectory")
	}
	if now.IsZero() {
		now = time.Now()
	}
	var buf bytes.Buffer
	cascadeID := t.CascadeID
	if cascadeID == "" {
		cascadeID = "Unknown"
	}
	fmt.Fprintf(&buf, "# Conversation Decrypted Transcript\n")
	fmt.Fprintf(&buf, "- **Cascade/Conversation ID:** `%s`\n", cascadeID)
	fmt.Fprintf(&buf, "- **Total Steps:** %d\n", len(t.Steps))
	fmt.Fprintf(&buf, "- **Generated on:** %s\n", now.Format("2006-01-02 15:04:05"))
	buf.WriteString("\n---\n\n")

	// seen guards against cycles: a cascade already on the current render path
	// is never re-expanded. The root seeds the set.
	seen := map[string]bool{t.CascadeID: true}
	writeBody(&buf, t, r, 0, seen)

	n, err := w.Write(buf.Bytes())
	return n, err
}

// writeBody renders t's steps and nests its resolved children. depth is t's own
// nesting level (root = 0). Children are matched to INVOKE_SUBAGENT steps in
// chronological order — 1st invoke step gets the 1st child, and so on. This is
// a heuristic: the step payload carries no child cascade id, so we rely on the
// invoke steps and the children both being time-ordered. Any children past the
// number of invoke steps — and ALL children when the parent has zero invoke
// steps (the stale/truncated-sidecar case, where the parent's trajectory is
// missing its INVOKE_SUBAGENT steps entirely) — fall into a trailing
// "Subagent Transcripts" section.
func writeBody(buf *bytes.Buffer, t *daemon.Trajectory, r SubagentResolver, depth int, seen map[string]bool) {
	var children []*daemon.Trajectory
	if r != nil {
		children = r.Children(t.CascadeID)
	}

	// Map each INVOKE_SUBAGENT step index to a child, in order.
	childAt := map[int]*daemon.Trajectory{}
	next := 0
	for i, step := range t.Steps {
		if step.Type == "CORTEX_STEP_TYPE_INVOKE_SUBAGENT" && next < len(children) {
			childAt[i] = children[next]
			next++
		}
	}
	leftover := children[next:]

	for i, step := range t.Steps {
		child := childAt[i]
		renderStep(buf, i+1, step, child != nil)
		if child != nil {
			renderChild(buf, child, r, depth+1, seen)
		}
		buf.WriteString("---\n\n")
	}

	if len(leftover) > 0 {
		buf.WriteString("## Subagent Transcripts\n\n")
		buf.WriteString("*Subagent transcripts linked to this conversation that are not " +
			"attached to a specific step above (the parent's stored trajectory has no " +
			"matching invoke step).*\n\n")
		for _, child := range leftover {
			renderChild(buf, child, r, depth+1, seen)
		}
	}
}

// renderChild nests one child transcript inside a <details> block at the given
// depth (root children = depth 1). The blank line after </summary>... the
// <summary> lets the inner Markdown render. Recurses into the child's own
// subagents via writeBody. depth and the seen-set bound the recursion.
func renderChild(buf *bytes.Buffer, child *daemon.Trajectory, r SubagentResolver, depth int, seen map[string]bool) {
	if child == nil {
		return
	}
	id := child.CascadeID
	if id == "" {
		id = "Unknown"
	}
	if depth > maxSubagentDepth {
		fmt.Fprintf(buf, "> *Subagent `%s` not expanded: max nesting depth %d reached.*\n\n", id, maxSubagentDepth)
		return
	}
	if child.CascadeID != "" && seen[child.CascadeID] {
		fmt.Fprintf(buf, "> *Subagent `%s` already shown above (cycle guard).*\n\n", id)
		return
	}
	if child.CascadeID != "" {
		seen[child.CascadeID] = true
	}

	fmt.Fprintf(buf, "<details><summary>Subagent transcript: <code>%s</code> (depth %d, %d steps)</summary>\n\n", id, depth, len(child.Steps))
	fmt.Fprintf(buf, "- **Cascade/Conversation ID:** `%s`\n", id)
	fmt.Fprintf(buf, "- **Total Steps:** %d\n\n", len(child.Steps))
	writeBody(buf, child, r, depth, seen)
	buf.WriteString("</details>\n\n")
}

// renderStep renders one step. hasChild is true when a subagent transcript is
// nested immediately after this step (an INVOKE_SUBAGENT step with a resolved
// child), which changes only the INVOKE_SUBAGENT marker wording.
func renderStep(buf *bytes.Buffer, num int, s daemon.Step, hasChild bool) {
	stype := s.Type
	short := strings.TrimPrefix(stype, "CORTEX_STEP_TYPE_")
	fmt.Fprintf(buf, "## Step %d: %s (Status: %s)\n", num, short, defaultStr(s.Status, "UNKNOWN"))
	fmt.Fprintf(buf, "*Timestamp: %s*\n\n", formatTimestamp(s.Metadata.CreatedAt))

	switch stype {
	case "CORTEX_STEP_TYPE_USER_INPUT":
		if s.UserInput != nil {
			buf.WriteString("### User Request\n")
			fmt.Fprintf(buf, "```\n%s\n```\n\n", s.UserInput.UserResponse)
		}
	case "CORTEX_STEP_TYPE_PLANNER_RESPONSE":
		if s.PlannerResponse != nil {
			pr := s.PlannerResponse
			if pr.Thinking != "" {
				buf.WriteString("### Agent Internal Thought\n")
				fmt.Fprintf(buf, "%s\n\n", pr.Thinking)
			}
			if pr.Response != "" {
				buf.WriteString("### Agent Chat Response\n")
				fmt.Fprintf(buf, "%s\n\n", pr.Response)
			}
			if len(pr.ToolCalls) > 0 {
				buf.WriteString("### Tool Calls Proposed\n")
				for _, tc := range pr.ToolCalls {
					name := defaultStr(tc.Name, "unknown_tool")
					args := defaultStr(tc.ArgumentsJSON, "{}")
					formatted := args
					var anyVal any
					if err := json.Unmarshal([]byte(args), &anyVal); err == nil {
						if pretty, err := json.MarshalIndent(anyVal, "  ", "  "); err == nil {
							formatted = string(pretty)
						}
					}
					fmt.Fprintf(buf, "- **Tool:** `%s`\n", name)
					fmt.Fprintf(buf, "  **Arguments:**\n  ```json\n  %s\n  ```\n\n", formatted)
				}
			}
		}
	case "CORTEX_STEP_TYPE_RUN_COMMAND":
		if s.RunCommand != nil {
			rc := s.RunCommand
			cmd := rc.CommandLine
			if cmd == "" {
				cmd = rc.ProposedCommandLine
			}
			buf.WriteString("### Command Execution\n")
			fmt.Fprintf(buf, "**Directory:** `%s`\n", rc.Cwd)
			fmt.Fprintf(buf, "**Command:** `%s`\n", cmd)
			exit := "unknown"
			if rc.ExitCode != nil {
				exit = fmt.Sprintf("%d", *rc.ExitCode)
			}
			fmt.Fprintf(buf, "**Exit Code:** `%s`\n", exit)
			if out := rc.CombinedOutputString(); out != "" {
				buf.WriteString("**Output:**\n")
				fmt.Fprintf(buf, "```\n%s\n```\n\n", out)
			} else {
				buf.WriteString("**Output:** *(No output)*\n\n")
			}
		}
	case "CORTEX_STEP_TYPE_VIEW_FILE":
		if s.ViewFile != nil {
			vf := s.ViewFile
			buf.WriteString("### File Viewed\n")
			fmt.Fprintf(buf, "**File:** %s (Lines %d-%d)\n", clickableLink(vf.AbsolutePathURI), vf.StartLine, vf.EndLine)
			fmt.Fprintf(buf, "```\n%s\n```\n\n", vf.Content)
		}
	case "CORTEX_STEP_TYPE_CODE_ACTION":
		if s.CodeAction != nil {
			ca := s.CodeAction
			buf.WriteString("### Code Action (File Edit/Create)\n")
			if ca.Description != "" {
				fmt.Fprintf(buf, "**Description:** %s\n", ca.Description)
			}
			if spec, err := ca.GetSpec(); err == nil && spec != nil {
				if filePath := spec.FilePath(); filePath != "" {
					var fileURI string
					if spec.File != nil {
						fileURI = spec.File.AbsoluteURI
					}
					if fileURI != "" {
						fmt.Fprintf(buf, "**File:** [%s](%s)\n", filePath, fileURI)
					} else {
						fmt.Fprintf(buf, "**File:** `%s`\n", filePath)
					}
				}
				if spec.Instruction != "" {
					fmt.Fprintf(buf, "**Instruction:** %s\n", spec.Instruction)
				}
			}
			if diff := ca.FormattedDiff(); diff != "" {
				buf.WriteString("**Changes:**\n" + diff + "\n\n")
			} else if r := ca.ActionResultString(); r != "" {
				if strings.ContainsAny(r, "\n{") {
					fmt.Fprintf(buf, "**Result:**\n```\n%s\n```\n\n", r)
				} else {
					fmt.Fprintf(buf, "**Result:** %s\n\n", r)
				}
			} else {
				buf.WriteString("**Result:** *(none)*\n\n")
			}
		}
	case "CORTEX_STEP_TYPE_GREP_SEARCH":
		if s.GrepSearch != nil {
			gs := s.GrepSearch
			buf.WriteString("### Ripgrep Search\n")
			fmt.Fprintf(buf, "- **Query:** `%s`\n", gs.Query)
			fmt.Fprintf(buf, "- **Path:** %s\n\n", clickableLink(gs.SearchPathURI))
		}
	case "CORTEX_STEP_TYPE_ERROR_MESSAGE":
		if s.ErrorMessage != nil {
			em := s.ErrorMessage
			buf.WriteString("### Tool Execution Error\n")
			buf.WriteString("> [!WARNING]\n")
			if em.Error.UserErrorMessage == "" && em.Error.ModelErrorMessage == "" {
				buf.WriteString("> (error details unavailable)\n\n")
			} else {
				if em.Error.UserErrorMessage != "" {
					for _, line := range strings.Split(em.Error.UserErrorMessage, "\n") {
						fmt.Fprintf(buf, "> %s\n", line)
					}
					buf.WriteString(">\n")
				}
				if em.Error.ModelErrorMessage != "" {
					buf.WriteString("\n<details><summary>Model error detail</summary>\n\n")
					fmt.Fprintf(buf, "```\n%s\n```\n\n", em.Error.ModelErrorMessage)
					buf.WriteString("</details>\n\n")
				} else {
					buf.WriteString("\n")
				}
			}
		}
	case "CORTEX_STEP_TYPE_SYSTEM_MESSAGE":
		if s.SystemMessage != nil {
			buf.WriteString("### System Message\n")
			fmt.Fprintf(buf, "> %s\n\n", s.SystemMessage.Message)
		}
	case "CORTEX_STEP_TYPE_CHECKPOINT":
		if s.Checkpoint != nil {
			cp := s.Checkpoint
			buf.WriteString("### Checkpoint / Compaction State\n")
			if len(cp.UserRequests) > 0 {
				fmt.Fprintf(buf, "**User Requests:** %s\n", strings.Join(cp.UserRequests, ", "))
			}
			if cp.SessionSummary != "" {
				fmt.Fprintf(buf, "\n%s\n\n", cp.SessionSummary)
			}
		}
	case "CORTEX_STEP_TYPE_INVOKE_SUBAGENT":
		// Payload-less marker: the step carries no child-cascade id. When a
		// child transcript is nested just below (hasChild), point at it;
		// otherwise note that the subagent lives in its own trajectory.
		buf.WriteString("### Subagent Invoked\n")
		if hasChild {
			buf.WriteString("*The agent delegated work to a subagent at this step. " +
				"The subagent's transcript is nested below.*\n\n")
		} else {
			buf.WriteString("*The agent delegated work to a subagent at this step. " +
				"The subagent's activity is recorded in its own separate trajectory.*\n\n")
		}
	default:
		// Fallback: dump generic / listDirectory if present, otherwise note.
		var raw json.RawMessage
		switch {
		case len(s.Generic) > 0:
			raw = s.Generic
		case s.ListDirectory != nil:
			b, _ := json.Marshal(s.ListDirectory)
			raw = b
		case len(s.ConversationHistory) > 0:
			raw = s.ConversationHistory
		}
		if len(raw) > 0 {
			pretty, err := indentJSON(raw)
			if err != nil {
				pretty = string(raw)
			}
			fmt.Fprintf(buf, "```json\n%s\n```\n\n", pretty)
		} else {
			buf.WriteString("*(No additional details stored for this step type)*\n\n")
		}
	}
}

func indentJSON(raw json.RawMessage) (string, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func formatTimestamp(ts string) string {
	if ts == "" {
		return "Unknown Time"
	}
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t.UTC().Format("2006-01-02 15:04:05 UTC")
	}
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.UTC().Format("2006-01-02 15:04:05 UTC")
	}
	return ts
}

func defaultStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func clickableLink(uri string) string {
	if uri == "" {
		return ""
	}
	clean := uri
	if strings.HasPrefix(clean, "file://") {
		clean = strings.TrimPrefix(clean, "file://")
	}
	// Extract filename from path
	idx := strings.LastIndex(clean, "/")
	if idx != -1 && idx < len(clean)-1 {
		filename := clean[idx+1:]
		return fmt.Sprintf("[%s](%s)", filename, uri)
	}
	return fmt.Sprintf("[%s](%s)", uri, uri)
}
