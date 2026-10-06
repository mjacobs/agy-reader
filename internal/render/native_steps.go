package render

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

const noDetails = "*(No additional details stored for this step type)*\n\n"

// renderGenerateImage renders a native GENERATE_IMAGE step: what was asked for
// (prompt, model, aspect ratio, name) and where the result was stored. Inline
// image bytes are summarised by length and never printed.
func renderGenerateImage(buf *bytes.Buffer, s daemon.Step) {
	g, err := s.GenerateImageDetails()
	if err != nil {
		renderRawPayload(buf, s.GenerateImage)
		return
	}
	var lines []string
	add := func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }
	if g.ImageName != "" {
		add("- **Image Name:** %s\n", inlineCode(g.ImageName))
	}
	if g.ModelName != "" {
		add("- **Model:** %s\n", inlineCode(g.ModelName))
	}
	if g.AspectRatio != "" {
		add("- **Aspect Ratio:** %s\n", inlineCode(g.AspectRatio))
	}
	if m := g.GeneratedMedia; m != nil {
		if m.MimeType != "" {
			add("- **Media Type:** %s\n", inlineCode(m.MimeType))
		}
		if m.URI != "" {
			add("- **Generated Image:** %s\n", clickableLink(m.URI))
		}
		if m.DisplayName != "" {
			add("- **Display Name:** %s\n", oneLine(m.DisplayName))
		}
		if m.Description != "" {
			add("- **Description:** %s\n", oneLine(m.Description))
		}
		if m.InlineData != "" {
			add("- **Inline Data:** embedded, not shown (%d base64 characters)\n", len(m.InlineData))
		}
	}
	if im := g.GeneratedImage; im != nil {
		if im.MimeType != "" && (g.GeneratedMedia == nil || im.MimeType != g.GeneratedMedia.MimeType) {
			add("- **Media Type:** %s\n", inlineCode(im.MimeType))
		}
		if im.URI != "" && (g.GeneratedMedia == nil || im.URI != g.GeneratedMedia.URI) {
			add("- **Generated Image:** %s\n", clickableLink(im.URI))
		}
		if im.Caption != "" {
			add("- **Caption:** %s\n", oneLine(im.Caption))
		}
		if im.Base64Data != "" {
			add("- **Inline Data:** embedded, not shown (%d base64 characters)\n", len(im.Base64Data))
		}
	}
	for _, p := range g.ImagePaths {
		if p != "" {
			add("- **Input Image:** %s\n", clickableLink(p))
		}
	}
	if len(lines) == 0 && g.Prompt == "" {
		buf.WriteString(noDetails)
		return
	}
	buf.WriteString("### Image Generation\n")
	for _, l := range lines {
		buf.WriteString(l)
	}
	if len(lines) > 0 {
		buf.WriteString("\n")
	}
	if g.Prompt != "" {
		buf.WriteString("**Prompt:**\n")
		writeFence(buf, "", g.Prompt)
	}
}

// renderInvokeSubagent renders an INVOKE_SUBAGENT step. Since agy 1.2.16 the
// step stores the requested child agents (type, role, model, prompt) and the
// child conversations it created (id, result-log URI); older daemons stored no
// payload, and only the delegation note is shown. hasChild is true when the
// child transcript is nested directly below.
func renderInvokeSubagent(buf *bytes.Buffer, s daemon.Step, hasChild bool) {
	buf.WriteString("### Subagent Invoked\n")
	if hasChild {
		buf.WriteString("*The agent delegated work to a subagent at this step. " +
			"The subagent's transcript is nested below.*\n\n")
	} else {
		buf.WriteString("*The agent delegated work to a subagent at this step. " +
			"The subagent's activity is recorded in its own separate trajectory.*\n\n")
	}
	inv, err := s.InvokeSubagentDetails()
	if err != nil {
		if len(s.InvokeSubagent) > 0 {
			renderRawPayload(buf, s.InvokeSubagent)
		}
		return
	}

	var head []string
	if inv.SubagentName != "" {
		head = append(head, fmt.Sprintf("- **Subagent Name:** %s\n", inlineCode(inv.SubagentName)))
	}
	if inv.ConversationID != "" {
		head = append(head, fmt.Sprintf("- **Child Conversation ID:** %s\n", inlineCode(inv.ConversationID)))
	}
	if inv.TaskMode {
		head = append(head, "- **Task Mode:** yes\n")
	}
	if len(head) > 0 {
		buf.WriteString(strings.Join(head, ""))
		buf.WriteString("\n")
	}
	if inv.Prompt != "" {
		buf.WriteString("**Prompt:**\n")
		writeFence(buf, "", inv.Prompt)
	}

	for i, sa := range inv.Subagents {
		label := "Requested subagent"
		if len(inv.Subagents) > 1 {
			label = fmt.Sprintf("Requested subagent %d of %d", i+1, len(inv.Subagents))
		}
		fmt.Fprintf(buf, "**%s:**\n", label)
		if sa.TypeName != "" {
			fmt.Fprintf(buf, "- **Agent Type:** %s\n", inlineCode(sa.TypeName))
		}
		if sa.Role != "" {
			fmt.Fprintf(buf, "- **Role:** %s\n", oneLine(sa.Role))
		}
		if sa.Model != "" || sa.ModelTier != "" {
			model := ""
			if sa.Model != "" {
				model = inlineCode(string(sa.Model))
			}
			if sa.ModelTier != "" {
				tier := "tier " + inlineCode(string(sa.ModelTier))
				if model == "" {
					model = tier
				} else {
					model += " (" + tier + ")"
				}
			}
			fmt.Fprintf(buf, "- **Model:** %s\n", model)
		}
		if sa.Inherit {
			buf.WriteString("- **Inherit:** yes\n")
		}
		if sa.Fork {
			buf.WriteString("- **Fork:** yes\n")
		}
		if sa.RunAsTask {
			buf.WriteString("- **Run As Task:** yes\n")
		}
		if sa.Branch != "" {
			fmt.Fprintf(buf, "- **Branch:** %s\n", inlineCode(sa.Branch))
		}
		if sa.WorkspaceURI != "" {
			fmt.Fprintf(buf, "- **Workspace:** %s\n", clickableLink(sa.WorkspaceURI))
		}
		buf.WriteString("\n")
		if sa.InitialPrompt != "" {
			buf.WriteString("**Initial Prompt:**\n")
			writeFence(buf, "", sa.InitialPrompt)
		}
	}

	for i, r := range inv.Results {
		label := "Child conversation"
		if len(inv.Results) > 1 {
			label = fmt.Sprintf("Child conversation %d of %d", i+1, len(inv.Results))
		}
		fmt.Fprintf(buf, "**%s:**\n", label)
		if r.ConversationID != "" {
			fmt.Fprintf(buf, "- **Conversation ID:** %s\n", inlineCode(r.ConversationID))
		}
		if r.LogAbsoluteURI != "" {
			fmt.Fprintf(buf, "- **Result Log:** %s\n", clickableLink(r.LogAbsoluteURI))
		}
		for _, w := range r.WorkspaceURIs {
			if w != "" {
				fmt.Fprintf(buf, "- **Workspace:** %s\n", clickableLink(w))
			}
		}
		buf.WriteString("\n")
	}
}

// renderRawPayload prints a payload this renderer could not decode as
// indented JSON, so nothing stored is silently dropped. Inline media bytes
// (inlineData, base64Data and similar fields, at any depth) are replaced by a
// length summary. A payload that is not valid JSON is not printed at all:
// its bytes cannot be told apart from the rest.
func renderRawPayload(buf *bytes.Buffer, raw json.RawMessage) {
	if len(raw) == 0 {
		buf.WriteString(noDetails)
		return
	}
	v, err := decodeJSONValue(raw)
	if err != nil {
		fmt.Fprintf(buf, "*(Stored payload is not valid JSON; %d bytes not shown)*\n\n", len(raw))
		return
	}
	pretty, err := json.MarshalIndent(redactInlineBytes(v), "", "  ")
	if err != nil {
		fmt.Fprintf(buf, "*(Stored payload could not be re-encoded; %d bytes not shown)*\n\n", len(raw))
		return
	}
	writeFence(buf, "json", string(pretty))
}

// decodeJSONValue decodes raw as exactly one JSON value. Numbers stay
// json.Number, so a valid number that does not fit a float64 (1e400) is not an
// error and every number prints as written.
func decodeJSONValue(raw json.RawMessage) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("trailing data after JSON value")
	}
	return v, nil
}

// redactInlineBytes replaces, in place and at any depth of a decoded JSON
// value, each inline-bytes field's value with a length summary.
func redactInlineBytes(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if isInlineBytesKey(k) {
				if summary, ok := summariseInlineBytes(val); ok {
					t[k] = summary
					continue
				}
			}
			if isURIKey(k) {
				if str, isStr := val.(string); isStr {
					if summary, ok := summariseDataURI(str); ok {
						t[k] = summary
						continue
					}
				}
			}
			t[k] = redactInlineBytes(val)
		}
	case []any:
		for i := range t {
			t[i] = redactInlineBytes(t[i])
		}
	}
	return v
}

// isInlineBytesKey reports whether a JSON field name holds embedded media
// bytes: inlineData (Media) or anything named like base64 (ImageData's
// base64Data), in camelCase or snake_case.
func isInlineBytesKey(k string) bool {
	n := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(k))
	return n == "inlinedata" || strings.Contains(n, "base64")
}

// isURIKey reports whether a JSON field name holds a link (uri, absolutePathURI,
// image_url and similar), whose value may be a data: URI carrying media bytes.
func isURIKey(k string) bool {
	n := strings.ToLower(k)
	return strings.HasSuffix(n, "uri") || strings.HasSuffix(n, "url")
}

// summariseInlineBytes returns the summary that stands in for an inline-bytes
// value. Empty strings, null, booleans, and numbers carry no bytes and are
// kept as they are.
func summariseInlineBytes(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		if t == "" {
			return "", false
		}
		return fmt.Sprintf("embedded, not shown (%d base64 characters)", len(t)), true
	case []any, map[string]any:
		b, err := json.Marshal(t)
		if err != nil {
			return "embedded, not shown", true
		}
		return fmt.Sprintf("embedded, not shown (%d characters of JSON)", len(b)), true
	}
	return "", false
}

// inlineCode wraps s in a backtick span long enough that backticks inside s
// cannot close it early.
func inlineCode(s string) string {
	s = oneLine(s)
	ticks := "`"
	for strings.Contains(s, ticks) {
		ticks += "`"
	}
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		return ticks + " " + s + " " + ticks
	}
	return ticks + s + ticks
}

// oneLine collapses line breaks so a value cannot break out of its list item.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// renderAskQuestion renders a native ASK_QUESTION step: each question agy put
// to the user, the options offered (selected ones marked) and the answer given,
// either the selected option's text or the written-in response.
func renderAskQuestion(buf *bytes.Buffer, s daemon.Step) {
	a, err := s.AskQuestionDetails()
	if err != nil {
		renderRawPayload(buf, s.AskQuestion)
		return
	}
	if len(a.Questions) == 0 {
		buf.WriteString(noDetails)
		return
	}
	buf.WriteString("### Questions Asked\n")
	for i, q := range a.Questions {
		selected := map[string]bool{}
		for _, id := range q.SelectedOptionIDs {
			selected[id] = true
		}
		fmt.Fprintf(buf, "\n**Question %d:** %s\n", i+1, oneLine(q.Question))
		var answers []string
		matched := map[string]bool{}
		for _, o := range q.Options {
			mark := ""
			if selected[o.ID] {
				mark = " (selected)"
				answers = append(answers, oneLine(o.Text))
				matched[o.ID] = true
			}
			fmt.Fprintf(buf, "- %s%s\n", oneLine(o.Text), mark)
		}
		for _, id := range q.SelectedOptionIDs {
			if !matched[id] {
				answers = append(answers, "option "+inlineCode(id))
			}
		}
		if len(q.Options) > 0 {
			buf.WriteString("\n")
		}
		switch {
		case q.WriteInResponse != "" && q.WriteInResponse == oneLine(q.WriteInResponse):
			fmt.Fprintf(buf, "**Answer (written in):** %s\n", q.WriteInResponse)
		case q.WriteInResponse != "":
			// The user typed this answer; its line breaks, indentation and
			// repeated spaces are kept in a fence rather than collapsed.
			buf.WriteString("**Answer (written in):**\n")
			writeFence(buf, "", q.WriteInResponse)
		case len(answers) > 0:
			fmt.Fprintf(buf, "**Answer:** %s\n", strings.Join(answers, ", "))
		case q.Skipped:
			buf.WriteString("**Answer:** *(skipped)*\n")
		default:
			buf.WriteString("**Answer:** *(none stored)*\n")
		}
	}
	buf.WriteString("\n")
}
