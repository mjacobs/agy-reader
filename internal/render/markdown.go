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
// a heuristic: steps from before agy 1.2.16 carry no child cascade id (newer
// ones do, and render it, but matching does not use it yet), so we rely on the
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

	compactedSet := map[int]bool{}
	if t.CompactionInfo != nil {
		for _, idx := range t.CompactionInfo.CompactedAtStepIndices {
			compactedSet[idx] = true
		}
	}

	for i, step := range t.Steps {
		child := childAt[i]
		renderStep(buf, i+1, step, child != nil, compactedSet[i])
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
// child), which changes only the INVOKE_SUBAGENT marker wording. compacted is
// true when context compaction occurred at this step.
func renderStep(buf *bytes.Buffer, num int, s daemon.Step, hasChild bool, compacted bool) {
	stype := s.Type
	short := strings.TrimPrefix(stype, "CORTEX_STEP_TYPE_")
	fmt.Fprintf(buf, "## Step %d: %s (Status: %s)\n", num, short, defaultStr(s.Status, "UNKNOWN"))
	fmt.Fprintf(buf, "*Timestamp: %s*\n\n", formatTimestamp(s.Metadata.CreatedAt))

	if compacted {
		buf.WriteString("> **Context Compaction:** Conversation context was compacted at this step. Earlier turn history was summarized.\n\n")
	}

	switch stype {
	case "CORTEX_STEP_TYPE_USER_INPUT":
		if s.UserInput != nil {
			buf.WriteString("### User Request\n")
			fmt.Fprintf(buf, "```\n%s\n```\n\n", s.UserInput.UserResponse)
			renderAttachments(buf, "User Attachments", s.UserInput.Media)
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
			renderAttachments(buf, "Agent Attachments", pr.Media)
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
			switch {
			case vf.IsMedia():
				renderMediaView(buf, vf)
			default:
				fmt.Fprintf(buf, "**File:** %s (Lines %d-%d)\n", clickableLink(vf.AbsolutePathURI), vf.StartLine, vf.EndLine)
				if vf.Content != "" || s.Error == nil {
					fmt.Fprintf(buf, "```\n%s\n```\n\n", vf.Content)
				} else {
					buf.WriteString("\n")
				}
			}
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
			renderCheckpoint(buf, s.Checkpoint)
		}
	case "CORTEX_STEP_TYPE_INVOKE_SUBAGENT":
		// When a child transcript is nested just below (hasChild), point at
		// it; otherwise note that the subagent lives in its own trajectory.
		// Stored child identity, when present, is listed either way.
		renderInvokeSubagent(buf, s, hasChild)
	case "CORTEX_STEP_TYPE_GENERATE_IMAGE":
		renderGenerateImage(buf, s)
	case "CORTEX_STEP_TYPE_ASK_QUESTION":
		renderAskQuestion(buf, s)
	case "CORTEX_STEP_TYPE_FINISH":
		if s.Finish != nil {
			renderFinish(buf, s.Finish)
		}
	default:
		// Fallback: dump generic / listDirectory if present, otherwise note.
		var raw json.RawMessage
		switch {
		case len(s.McpTool) > 0:
			raw = s.McpTool
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
			buf.WriteString(noDetails)
		}
	}
	renderStepError(buf, s.Error)
}

func renderAttachments(buf *bytes.Buffer, title string, media []daemon.Media) {
	if len(media) == 0 {
		return
	}
	fmt.Fprintf(buf, "### %s\n\n", title)
	for i, attachment := range media {
		fmt.Fprintf(buf, "**Attachment %d:**\n", i+1)
		renderMedia(buf, attachment)
		buf.WriteString("\n")
	}
}

// renderMediaView renders a VIEW_FILE step that carries binary media (an
// image, for example) instead of text. Such steps still report the tool's
// default line window, so the line range and the text fence are omitted:
// printing them would claim text coverage that never happened.
func renderMediaView(buf *bytes.Buffer, vf *daemon.ViewFile) {
	m := vf.MediaData
	if vf.AbsolutePathURI != "" {
		fmt.Fprintf(buf, "- **File:** %s\n", clickableLink(vf.AbsolutePathURI))
	}
	if m.MimeType != "" {
		fmt.Fprintf(buf, "- **Media Type:** `%s`\n", m.MimeType)
	}
	if m.URI != "" {
		fmt.Fprintf(buf, "- **Stored Media:** %s\n", clickableLink(m.URI))
	}
	if vf.NumBytes > 0 {
		fmt.Fprintf(buf, "- **Size:** %s\n", formatByteSize(vf.NumBytes))
	}
	if m.InlineData != "" {
		fmt.Fprintf(buf, "- **Inline Data:** embedded, not shown (%d base64 characters)\n", len(m.InlineData))
	}
	buf.WriteString("\n*Binary media view: no text lines were read.*\n\n")
	if vf.Content != "" {
		fmt.Fprintf(buf, "```\n%s\n```\n\n", vf.Content)
	}
}

// formatByteSize renders n as an exact byte count, followed by a binary-unit
// approximation once n reaches 1 KiB.
func formatByteSize(n int) string {
	if n == 1 {
		return "1 byte"
	}
	exact := fmt.Sprintf("%d bytes", n)
	const unit = 1024
	if n < unit {
		return exact
	}
	value := float64(n)
	suffix := ""
	for _, s := range []string{"KiB", "MiB", "GiB", "TiB"} {
		value /= unit
		suffix = s
		if value < unit {
			break
		}
	}
	return fmt.Sprintf("%s (%.1f %s)", exact, value, suffix)
}

// renderStepError prints the top-level step error, independent of the
// type-specific payload, so a failed step explains itself without a later
// assistant summary.
func renderStepError(buf *bytes.Buffer, e *daemon.StepError) {
	if e == nil || (e.ShortError == "" && e.FullError == "") {
		return
	}
	buf.WriteString("### Step Error\n")
	buf.WriteString("> [!WARNING]\n")
	short := e.ShortError
	if short == "" {
		short = e.FullError
	}
	for _, line := range strings.Split(short, "\n") {
		fmt.Fprintf(buf, "> %s\n", line)
	}
	buf.WriteString("\n")
	if e.FullError != "" && e.FullError != short {
		buf.WriteString("<details><summary>Full error</summary>\n\n")
		writeFence(buf, "", e.FullError)
		buf.WriteString("</details>\n\n")
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
	if summary, ok := summariseDataURI(uri); ok {
		return summary
	}
	clean := strings.TrimPrefix(uri, "file://")
	// Extract filename from path
	idx := strings.LastIndex(clean, "/")
	if idx != -1 && idx < len(clean)-1 {
		filename := clean[idx+1:]
		return fmt.Sprintf("[%s](%s)", filename, uri)
	}
	return fmt.Sprintf("[%s](%s)", uri, uri)
}

// summariseDataURI returns a one-line summary for a data: URI, which embeds
// the media bytes in the link itself, so printing it would dump them. The
// summary gives the media type and the URI's length in characters. Any other
// string reports false.
func summariseDataURI(uri string) (string, bool) {
	if len(uri) < len("data:") || !strings.EqualFold(uri[:len("data:")], "data:") {
		return "", false
	}
	rest := uri[len("data:"):]
	header := rest
	if i := strings.IndexByte(rest, ','); i >= 0 {
		header = rest[:i]
	}
	mediaType := strings.TrimSpace(strings.SplitN(header, ";", 2)[0])
	if mediaType == "" {
		mediaType = "text/plain"
	}
	return fmt.Sprintf("data URI (`%s`), embedded, not shown (%d characters)", oneLine(mediaType), len(uri)), true
}
