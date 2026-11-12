package completeness

import (
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// skillMetadata150 qualifies the skill name and description agy 1.5.0 adds to a
// modern skill read that starts at line one. It reads the persisted, numbered
// tool result, never the filesystem or incoming RPC (skillFrontmatter150).
func skillMetadata150(historical proto.Message) proto.Message {
	out := proto.Clone(historical)
	steps := tool143Get(out.ProtoReflect(), "steps").List()
	for i := 0; i < steps.Len(); i++ {
		step := steps.Get(i).Message()
		viewField := step.Descriptor().Fields().ByName("view_file")
		if !step.Has(viewField) {
			continue
		}
		view := step.Get(viewField).Message()
		if !tool143Get(view, "is_skill_file").Bool() || tool143Get(view, "start_line").Uint() != 0 || view.Has(view.Descriptor().Fields().ByName("skill_metadata")) {
			continue
		}
		values, ok := skillFrontmatter150(tool143Get(view, "content").String())
		if !ok {
			continue
		}
		metadata := tool143Mutable(view, "skill_metadata")
		for key, value := range values {
			tool143Set(metadata, protoreflect.Name(key), protoreflect.ValueOfString(value))
		}
	}
	return out
}

// skillFrontmatter150 returns the name and description of the frontmatter at
// the top of a numbered read result. The read starts at line one, under the
// header "Showing lines 1-" or, for a read cut at the byte limit, "Partial read
// (lines 1-", and numbered line 1 is "---".
//
// The skill150 form is unchanged: under "Showing lines 1-", exactly two plain
// single-line fields (plainSkillScalar150), name and description in either
// order, then the closing "---".
//
// The frontmatter150 forms accept only the layouts observed in the 1.5.0
// audit's 499 saved responses: line 2 is "name: " and a plain single-line
// scalar, then the description and a closing "---". The description takes one
// of these forms, with the value agy presented for it:
//
//   - a single-line double-quoted scalar without backslashes, inner quotes or
//     white space just inside the quotes, presented without its quotes;
//   - a folded block, ">-", of at least two lines indented by two spaces,
//     without blank lines, presented as its lines joined by single spaces;
//   - a literal block, "|", indented by two spaces and holding at least one
//     blank line, presented as its lines joined by line breaks with the blank
//     lines kept and no final line break.
//
// A read cut at the byte limit was observed only with a folded description.
// Block lines have no trailing white space, and a block neither starts nor
// ends with a blank line. Any other order, key, form, indentation, escape or
// chomping indicator returns false, so the comparison falls back to the
// presentation without metadata.
func skillFrontmatter150(content string) (map[string]string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return nil, false
	}
	// The 1.5.5 capture prepends two timestamp lines to a plain skill read.
	// Keep the original content in the trajectory; strip only for extraction.
	timestamped := strings.HasPrefix(lines[0], "Created At: ")
	if timestamped {
		if len(lines) < 4 {
			return nil, false
		}
		for i, prefix := range []string{"Created At: ", "Completed At: "} {
			value, ok := strings.CutPrefix(lines[i], prefix)
			if !ok {
				return nil, false
			}
			if _, err := time.Parse(time.RFC3339, value); err != nil {
				return nil, false
			}
		}
		lines = lines[2:]
	}
	partial := strings.HasPrefix(lines[0], "Partial read (lines 1-")
	if !partial && !strings.HasPrefix(lines[0], "Showing lines 1-") {
		return nil, false
	}
	var body []string
	closed := false
	for n := 1; n < len(lines) && !closed; n++ {
		text, ok := strings.CutPrefix(lines[n], strconv.Itoa(n)+": ")
		switch {
		case !ok, n == 1 && text != "---":
			return nil, false
		case n > 1 && text == "---":
			closed = true
		case n > 1:
			body = append(body, text)
		}
	}
	if !closed || len(body) < 2 {
		return nil, false
	}
	if values, ok := plainSkillFields150(body); ok && !partial {
		return values, true
	}
	// agy 1.5.6 also prepends these timestamps to the captured folded form.
	// Other timestamped non-plain forms remain unqualified.
	if timestamped && (partial || body[1] != "description: >-") {
		return nil, false
	}
	name, ok := strings.CutPrefix(body[0], "name: ")
	if !ok || !plainSkillScalar150(name) {
		return nil, false
	}
	value, ok := strings.CutPrefix(body[1], "description: ")
	if !ok {
		return nil, false
	}
	var description string
	switch {
	case value == ">-":
		description, ok = blockSkillScalar150(body[2:], true)
	case value == "|" && !partial:
		description, ok = blockSkillScalar150(body[2:], false)
	case len(body) == 2 && !partial:
		description, ok = flowSkillScalar150(value)
	default:
		ok = false
	}
	if !ok {
		return nil, false
	}
	return map[string]string{"name": name, "description": description}, true
}

// plainSkillFields150 reads the skill150 form: exactly two lines, a name and a
// description in either order, each a plain single-line scalar.
func plainSkillFields150(body []string) (map[string]string, bool) {
	if len(body) != 2 {
		return nil, false
	}
	values := make(map[string]string)
	for _, line := range body {
		key, value, ok := strings.Cut(line, ": ")
		if !ok || (key != "name" && key != "description") || values[key] != "" || !plainSkillScalar150(value) {
			return nil, false
		}
		values[key] = value
	}
	return values, len(values) == 2
}

// blockSkillScalar150 returns the presented value of a folded (">-") or
// literal ("|") block; see skillFrontmatter150 for its bounds.
func blockSkillScalar150(block []string, folded bool) (string, bool) {
	if len(block) < 2 || block[0] == "" || block[len(block)-1] == "" {
		return "", false
	}
	texts := make([]string, len(block))
	blank := false
	for i, line := range block {
		if line == "" {
			blank = true
			continue
		}
		text, ok := strings.CutPrefix(line, "  ")
		if !ok || text == "" || text[0] == ' ' || strings.TrimRight(text, " ") != text || strings.ContainsAny(text, "\r\t") {
			return "", false
		}
		texts[i] = text
	}
	if folded {
		if blank {
			return "", false
		}
		return strings.Join(texts, " "), true
	}
	if !blank {
		return "", false
	}
	return strings.Join(texts, "\n"), true
}

// flowSkillScalar150 returns the presented value of a single-line description:
// a plain one, or a double-quoted one without escapes, inner quotes or white
// space just inside the quotes.
func flowSkillScalar150(s string) (string, bool) {
	if plainSkillScalar150(s) {
		return s, true
	}
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", false
	}
	inner := s[1 : len(s)-1]
	if inner == "" || strings.TrimSpace(inner) != inner || strings.ContainsAny(inner, "\"\\\r\t") {
		return "", false
	}
	return inner, true
}

func plainSkillScalar150(s string) bool {
	if s == "" || strings.TrimSpace(s) != s || strings.ContainsAny(s, "\r\t") || strings.Contains(s, ": ") || strings.Contains(s, " #") {
		return false
	}
	if strings.ContainsRune("-?:,[]{}#&*!|>'\"%@`", rune(s[0])) {
		return false
	}
	switch strings.ToLower(s) {
	case "true", "false", "null", "~":
		return false
	}
	return true
}
