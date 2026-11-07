package completeness

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Fixtures from nine retained conversations that the agy 1.5.0 audit fetched
// and rendered but could not verify (agy-reader#t5zw). Each fixture keeps only
// the steps under test of one conversation:
//
//   - default.json is the ordinary (default) RPC trajectory that the audit's
//     sweep saved from an agy 1.5.0 daemon.
//   - full.json is the persisted SQLite source rendered as protobuf JSON. The
//     sweep compared the full RPC with that source before it refused each
//     conversation (CompareFullRaw precedes projection), so the full response
//     equalled it; the response itself was not kept.
//
// Steps were trimmed and both files scrubbed alike: top-level workspace,
// generator and executor metadata dropped, signatures replaced, commands,
// summaries, titles and outputs replaced by placeholders, paths moved under
// /fixture-home, and each word of skill frontmatter replaced by a scrambled
// word of the same length and case (the YAML punctuation, indentation and
// line structure are untouched; the file body after the frontmatter is cut).
//
//   - frontmatter150/quoted-folded: four skill reads from line one, with a plain
//     description (skill150), a double-quoted one and two folded (">-") ones.
//   - frontmatter150/partial-read: a folded description in a read cut at the
//     byte limit, whose header starts "Partial read (lines 1-".
//   - frontmatter150/literal: a literal ("|") description with blank lines.
//   - bgcancel150/background: two background commands with a 10000 ms wait,
//     canceled; the second also carries BypassSandbox "true" and was canceled
//     without a recorded transition or completion time.
//   - bgcancel150/blocking: a command with a 10000 ms wait canceled before it
//     moved to the background.
//   - readlines150: a read_file of lines 1-80 of a missing file.
var (
	frontmatter150QuotedFolded = []denied1412Case{{"frontmatter150/quoted-folded", 0}, {"frontmatter150/quoted-folded", 1}, {"frontmatter150/quoted-folded", 2}, {"frontmatter150/quoted-folded", 3}}
	frontmatter150Partial      = denied1412Case{"frontmatter150/partial-read", 0}
	frontmatter150Literal      = denied1412Case{"frontmatter150/literal", 0}
	frontmatter150Cases        = append(append([]denied1412Case{}, frontmatter150QuotedFolded...), frontmatter150Partial, frontmatter150Literal)

	bgcancel150Wait         = denied1412Case{"bgcancel150/background", 0}
	bgcancel150NoTransition = denied1412Case{"bgcancel150/background", 1}
	bgcancel150Blocking     = denied1412Case{"bgcancel150/blocking", 0}
	bgcancel150Cases        = []denied1412Case{bgcancel150Wait, bgcancel150NoTransition, bgcancel150Blocking}

	readlines150Case = denied1412Case{"readlines150", 0}

	retained150Dirs = []string{"frontmatter150/quoted-folded", "frontmatter150/partial-read", "frontmatter150/literal", "bgcancel150/background", "bgcancel150/blocking", "readlines150"}
)

func TestRetained150NativeVerifies(t *testing.T) {
	for _, dir := range retained150Dirs {
		t.Run(dir, func(t *testing.T) {
			full, shown := denied1412Case{dir, 0}.load(t)
			s := snapshot1216(t, full)
			require1216(t, s, full, shown, Verified)
			offline, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, full, string(offline), Verified)
		})
	}
}

// Each skill read is presented with the metadata its frontmatter gives, in the
// form the fixture comment names.
func TestFrontmatter150Presentation(t *testing.T) {
	forms := map[denied1412Case]string{
		frontmatter150QuotedFolded[0]: "plain",
		frontmatter150QuotedFolded[1]: "quoted",
		frontmatter150QuotedFolded[2]: ">-",
		frontmatter150QuotedFolded[3]: ">-",
		frontmatter150Partial:         ">-",
		frontmatter150Literal:         "|",
	}
	for _, c := range frontmatter150Cases {
		t.Run(fmt.Sprintf("%s#%d", c.dir, c.step), func(t *testing.T) {
			full, shown := c.load(t)
			view := obj(c.stepOf(t, shown), "viewFile")
			content := view["content"].(string)
			stored := obj(c.stepOf(t, full), "generic", "result")["result"].(string)
			if stored != content || view["isSkillFile"] != true || view["startLine"] != nil {
				t.Fatal("fixture changed")
			}
			description := strings.Split(content, "\n")[3]
			form := "plain"
			switch {
			case strings.HasSuffix(description, ": >-"):
				form = ">-"
			case strings.HasSuffix(description, ": |"):
				form = "|"
			case strings.HasPrefix(description, `3: description: "`):
				form = "quoted"
			}
			if form != forms[c] {
				t.Fatalf("form %s, want %s", form, forms[c])
			}
			values, ok := skillFrontmatter150(content)
			if !ok {
				t.Fatal("frontmatter not derived")
			}
			a, _ := json.Marshal(values)
			b, _ := json.Marshal(view["skillMetadata"])
			if string(a) != string(b) {
				t.Fatalf("derived %s, presented %s", a, b)
			}
		})
	}
	// The literal block keeps its blank lines and has no final line break; the
	// folded and quoted values have no line break at all.
	_, shown := frontmatter150Literal.load(t)
	literal := obj(frontmatter150Literal.stepOf(t, shown), "viewFile", "skillMetadata")["description"].(string)
	if !strings.Contains(literal, ".\n\n") || strings.HasSuffix(literal, "\n") {
		t.Fatalf("literal block changed: %q", literal)
	}
}

// editContent applies edit to the numbered lines of the read result in both
// responses, renumbering them, as agy would present the edited file.
func (c denied1412Case) editContent(t *testing.T, full, shown string, edit func(lines []string) []string) (string, string) {
	t.Helper()
	rewrite := func(content string) string {
		lines := strings.Split(content, "\n")
		texts := make([]string, 0, len(lines)-1)
		for n, line := range lines[1:] {
			text, ok := strings.CutPrefix(line, fmt.Sprintf("%d: ", n+1))
			if !ok {
				t.Fatalf("unnumbered line %q", line)
			}
			texts = append(texts, text)
		}
		texts = edit(texts)
		out := []string{lines[0]}
		for n, text := range texts {
			out = append(out, fmt.Sprintf("%d: %s", n+1, text))
		}
		return strings.Join(out, "\n")
	}
	full = c.editStep(t, full, func(s map[string]any) {
		result := obj(s, "generic", "result")
		result["result"] = rewrite(result["result"].(string))
	})
	shown = c.editStep(t, shown, func(s map[string]any) {
		view := obj(s, "viewFile")
		view["content"] = rewrite(view["content"].(string))
	})
	return full, shown
}

// Edits of the frontmatter that leave its form unsupported. Each is applied to
// the saved read result in both responses, and the default response keeps its
// metadata: neither the presentation without metadata nor the derived one
// matches, so the conversation does not verify.
func TestFrontmatter150Refusals(t *testing.T) {
	insertAfter := func(n int, text string) func([]string) []string {
		return func(lines []string) []string {
			return append(append(append([]string{}, lines[:n]...), text), lines[n:]...)
		}
	}
	replace := func(n int, from, to string) func([]string) []string {
		return func(lines []string) []string {
			if !strings.Contains(lines[n], from) {
				panic(fmt.Sprintf("line %d has no %q", n, from))
			}
			lines[n] = strings.Replace(lines[n], from, to, 1)
			return lines
		}
	}
	type refusal struct {
		name   string
		c      denied1412Case
		edit   func([]string) []string
		header bool // also replace "Showing lines 1-" with "Partial read (lines 1-"
	}
	plain, quoted, folded := frontmatter150QuotedFolded[0], frontmatter150QuotedFolded[1], frontmatter150QuotedFolded[2]
	// blockLines applies change to each nonblank block line (lines 3 up to the
	// closing line).
	blockLines := func(change func(string) string) func([]string) []string {
		return func(lines []string) []string {
			for i := 3; i < len(lines)-1; i++ {
				if lines[i] != "" {
					lines[i] = change(lines[i])
				}
			}
			return lines
		}
	}
	cases := []refusal{
		// Layouts never observed in the 499 saved responses: in the new forms
		// the name must be plain and first, blocks indented by two spaces, a
		// folded block at least two lines, a literal block with a blank line,
		// and a read cut at the byte limit only with a folded description. Two
		// plain fields in either order stay accepted as under skill150
		// (TestSkill150DescriptionFirst).
		{"description before name, folded", folded, func(lines []string) []string {
			name := lines[1]
			out := append(append([]string{lines[0]}, lines[2:len(lines)-1]...), name, lines[len(lines)-1])
			return out
		}, false},
		{"quoted name", quoted, func(lines []string) []string {
			lines[1] = `name: "` + strings.TrimPrefix(lines[1], "name: ") + `"`
			return lines
		}, false},
		{"folded name block", quoted, func(lines []string) []string {
			first, rest, _ := strings.Cut(strings.TrimPrefix(lines[1], "name: "), "-")
			return append([]string{lines[0], "name: >-", "  " + first, "  " + rest}, lines[2:]...)
		}, false},
		{"folded indentation 4", folded, blockLines(func(l string) string { return "  " + l }), false},
		{"folded indentation 1", folded, blockLines(func(l string) string { return l[1:] }), false},
		{"literal indentation 4", frontmatter150Literal, blockLines(func(l string) string { return "  " + l }), false},
		{"literal without blank lines", frontmatter150Literal, func(lines []string) []string {
			var out []string
			for _, l := range lines {
				if l != "" {
					out = append(out, l)
				}
			}
			return out
		}, false},
		{"single-line folded block", folded, func(lines []string) []string {
			return append(append([]string{}, lines[:4]...), lines[len(lines)-1])
		}, false},
		{"partial read with literal", frontmatter150Literal, nil, true},
		{"partial read with plain", plain, nil, true},
		{"partial read with quoted", quoted, nil, true},
		// An unknown extra field: another frontmatter key.
		{"extra key before the closing line", folded, insertAfter(len(strings.Split(loadContent(t, folded), "\n"))-2, "license: MIT"), false},
		{"extra key after name", quoted, insertAfter(2, "license: MIT"), false},
		{"duplicate description", quoted, insertAfter(2, "description: other"), false},
		{"another key instead of name", quoted, replace(1, "name: ", "title: "), false},
		// A changed status: another block style or chomping indicator.
		{"folded with clip", folded, replace(2, ": >-", ": >"), false},
		{"folded with keep", folded, replace(2, ": >-", ": >+"), false},
		{"literal with strip", frontmatter150Literal, replace(2, ": |", ": |-"), false},
		{"literal with keep", frontmatter150Literal, replace(2, ": |", ": |+"), false},
		{"explicit indentation", folded, replace(2, ": >-", ": >2-"), false},
		{"single-quoted", quoted, func(lines []string) []string {
			lines[2] = "description: '" + strings.Trim(strings.TrimPrefix(lines[2], "description: "), `"`) + "'"
			return lines
		}, false},
		{"escape in quotes", quoted, replace(2, `description: "`, `description: "\t`), false},
		{"inner quote", quoted, replace(2, `description: "`, `description: "a " b `), false},
		{"space inside quotes", quoted, replace(2, `description: "`, `description: " `), false},
		// A changed argument set: the read does not start at line one, or the
		// block lines are no longer uniform.
		{"more-indented folded line", folded, replace(4, "  ", "    "), false},
		{"blank line in folded block", folded, insertAfter(4, ""), false},
		{"trailing space in block", folded, func(lines []string) []string { lines[3] += " "; return lines }, false},
		{"tab in block", frontmatter150Literal, replace(3, "  ", "\t"), false},
		{"blank last block line", frontmatter150Literal, func(lines []string) []string {
			end := len(lines) - 1
			return append(append(append([]string{}, lines[:end]...), ""), lines[end:]...)
		}, false},
		{"no closing line", folded, func(lines []string) []string { return lines[:len(lines)-1] }, false},
		{"no opening line", folded, func(lines []string) []string { return lines[1:] }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			full, shown := tc.c.load(t)
			require1216(t, snapshot1216(t, full), full, shown, Verified)
			badFull, badShown := full, shown
			if tc.edit != nil {
				badFull, badShown = tc.c.editContent(t, full, shown, tc.edit)
			}
			if tc.header {
				badFull = tc.c.editStep(t, badFull, func(s map[string]any) {
					r := obj(s, "generic", "result")
					r["result"] = strings.Replace(r["result"].(string), "Showing lines 1-", "Partial read (lines 1-", 1)
				})
				badShown = tc.c.editStep(t, badShown, func(s map[string]any) {
					v := obj(s, "viewFile")
					v["content"] = strings.Replace(v["content"].(string), "Showing lines 1-", "Partial read (lines 1-", 1)
				})
			}
			view := obj(tc.c.stepOf(t, badShown), "viewFile")
			if _, ok := skillFrontmatter150(view["content"].(string)); ok {
				t.Fatal("edited frontmatter still derives metadata")
			}
			require1216(t, snapshot1216(t, badFull), badFull, badShown, Incomplete)
		})
	}
	// A read that does not start at line one carries no derived metadata.
	t.Run("read from line two", func(t *testing.T) {
		full, shown := folded.load(t)
		full = folded.editStep(t, full, func(s map[string]any) {
			r := obj(s, "generic", "result")
			r["result"] = strings.Replace(r["result"].(string), "Showing lines 1-", "Showing lines 2-", 1)
		})
		shown = folded.editStep(t, shown, func(s map[string]any) {
			v := obj(s, "viewFile")
			v["content"] = strings.Replace(v["content"].(string), "Showing lines 1-", "Showing lines 2-", 1)
		})
		require1216(t, snapshot1216(t, full), full, shown, Incomplete)
	})
}

func loadContent(t *testing.T, c denied1412Case) string {
	t.Helper()
	_, shown := c.load(t)
	return obj(c.stepOf(t, shown), "viewFile")["content"].(string)
}

// A presented value that differs from the frontmatter, or carries an extra
// field, does not verify.
func TestFrontmatter150PresentationMismatch(t *testing.T) {
	for _, c := range frontmatter150Cases {
		for name, edit := range map[string]func(map[string]any){
			"description differs": func(m map[string]any) { m["description"] = m["description"].(string) + "." },
			"name differs":        func(m map[string]any) { m["name"] = "other" },
			"extra field":         func(m map[string]any) { m["publisher"] = "untrusted" },
			"no metadata":         nil,
		} {
			t.Run(fmt.Sprintf("%s#%d: %s", c.dir, c.step, name), func(t *testing.T) {
				full, shown := c.load(t)
				bad := c.editStep(t, shown, func(s map[string]any) {
					view := obj(s, "viewFile")
					if edit == nil {
						delete(view, "skillMetadata")
						return
					}
					edit(obj(view, "skillMetadata"))
				})
				want := Incomplete
				// Removing the metadata from the only skill read of a
				// conversation leaves the presentation without metadata, which
				// stays supported.
				if edit == nil && c.dir != "frontmatter150/quoted-folded" {
					want = Verified
				}
				s := snapshot1216(t, full)
				if _, r := s.VerifyResponses([]byte(full), []byte(bad)); r.State != want {
					t.Fatalf("got %+v, want %s", r, want)
				}
			})
		}
	}
}

// Each source edit of an accepted canceled command is refused as an
// unsupported canceled background command: an unknown extra field, a changed
// status and a changed argument set or history.
func TestBgCancel150Refusals(t *testing.T) {
	const reason = "unsupported canceled background command projection"
	type refusal struct {
		name string
		edit func(step map[string]any)
	}
	transitions := func(s map[string]any) []any {
		return obj(s, "metadata", "internalMetadata")["statusTransitions"].([]any)
	}
	shared := []refusal{
		// An unknown extra field.
		{"unknown argument", func(s map[string]any) { setArgument(s, "Future", "x", json.RawMessage(`"x"`)) }},
		{"result text", func(s map[string]any) { obj(s, "generic", "result")["result"] = "fixture" }},
		{"deny hook", func(s map[string]any) {
			obj(s, "metadata")["preToolHookResults"] = []any{map[string]any{"decision": "deny"}}
		}},
		// A changed status.
		{"status done", func(s map[string]any) { s["status"] = "CORTEX_STEP_STATUS_DONE" }},
		{"status error", func(s map[string]any) { s["status"] = "CORTEX_STEP_STATUS_ERROR" }},
		// A changed argument set.
		{"wait 3000", func(s map[string]any) {
			setArgument(s, "WaitMsBeforeAsync", "3000", json.RawMessage(`3000`))
		}},
		{"wait 0", func(s map[string]any) { setArgument(s, "WaitMsBeforeAsync", "0", json.RawMessage(`0`)) }},
		{"no wait", func(s map[string]any) { deleteArgument(s, "WaitMsBeforeAsync") }},
		{"BypassSandbox false", func(s map[string]any) {
			setArgument(s, "BypassSandbox", "false", json.RawMessage(`false`))
		}},
		{"IsDaemon argument", func(s map[string]any) { setArgument(s, "IsDaemon", "false", json.RawMessage(`false`)) }},
		{"Cwd argument", func(s map[string]any) {
			setArgument(s, "Cwd", "/fixture-home/ws", json.RawMessage(`"/fixture-home/ws"`))
		}},
		{"retained call differs", func(s map[string]any) {
			call := obj(s, "metadata", "toolCall")
			call["argumentsJson"] = strings.Replace(call["argumentsJson"].(string), "fixture command", "other command", 1)
		}},
		// A changed history.
		{"history ends done", func(s map[string]any) {
			tr := transitions(s)
			tr[len(tr)-1].(map[string]any)["updatedStatus"] = "CORTEX_STEP_STATUS_DONE"
		}},
		{"history pending only", func(s map[string]any) {
			obj(s, "metadata", "internalMetadata")["statusTransitions"] = transitions(s)[:1]
		}},
	}
	// Each shape is accepted only with the wait and BypassSandbox argument it
	// was observed with.
	wait5000 := refusal{"wait 5000", func(s map[string]any) {
		setArgument(s, "WaitMsBeforeAsync", "5000", json.RawMessage(`5000`))
	}}
	bypassTrue := refusal{"BypassSandbox true", func(s map[string]any) {
		setArgument(s, "BypassSandbox", "true", json.RawMessage(`true`))
	}}
	perCase := map[denied1412Case][]refusal{
		bgcancel150Wait: {
			bypassTrue,
			{"no transition but completed", func(s map[string]any) {
				obj(s, "metadata", "internalMetadata")["statusTransitions"] = transitions(s)[:2]
			}},
			{"no background timestamp", func(s map[string]any) { delete(obj(s, "taskDetails"), "movedToBackgroundAt") }},
		},
		bgcancel150NoTransition: {
			wait5000,
			{"completion time", func(s map[string]any) { obj(s, "metadata")["completedAt"] = "2026-11-03T21:30:00Z" }},
			{"no task details", func(s map[string]any) { delete(s, "taskDetails") }},
		},
		bgcancel150Blocking: {
			wait5000,
			bypassTrue,
			{"no canceled transition", func(s map[string]any) {
				obj(s, "metadata", "internalMetadata")["statusTransitions"] = transitions(s)[:2]
				delete(obj(s, "metadata"), "completedAt")
			}},
			{"task details without background timestamp", func(s map[string]any) {
				s["taskDetails"] = map[string]any{"id": "fixture/task-1"}
			}},
		},
	}
	for _, c := range bgcancel150Cases {
		for _, tc := range append(append([]refusal{}, shared...), perCase[c]...) {
			t.Run(fmt.Sprintf("%s#%d: %s", c.dir, c.step, tc.name), func(t *testing.T) {
				full, shown := c.load(t)
				require1216(t, snapshot1216(t, full), full, shown, Verified)
				bad := c.editStep(t, full, tc.edit)
				s := snapshot1216(t, bad)
				_, result := s.VerifyResponses([]byte(bad), []byte(shown))
				if result.State != Unknown || result.Reason != reason {
					t.Fatalf("got %+v, want unknown: %s", result, reason)
				}
				if _, err := s.OfflineJSON(); err == nil || !strings.Contains(err.Error(), reason) {
					t.Fatalf("offline export: %v, want %s", err, reason)
				}
			})
		}
	}
}

// A command canceled before it moved to the background is presented blocking,
// and a background command is not; the other presentation does not verify.
func TestBgCancel150Presentation(t *testing.T) {
	for _, c := range bgcancel150Cases {
		t.Run(fmt.Sprintf("%s#%d", c.dir, c.step), func(t *testing.T) {
			full, shown := c.load(t)
			run := obj(c.stepOf(t, shown), "runCommand")
			_, background := c.stepOf(t, full)["taskDetails"]
			if blocking, _ := run["blocking"].(bool); blocking == background {
				t.Fatalf("blocking %v with task details %v", run["blocking"], background)
			}
			flipped := c.editStep(t, shown, func(s map[string]any) {
				r := obj(s, "runCommand")
				if background {
					r["blocking"] = true
				} else {
					delete(r, "blocking")
				}
			})
			require1216(t, snapshot1216(t, full), full, flipped, Incomplete)
		})
	}
}

// Each source edit of the accepted failed read is refused: an unknown extra
// field, a changed status and a changed argument set.
func TestReadLines150Refusals(t *testing.T) {
	const reason = "unsupported failed read_file projection"
	c := readlines150Case
	for _, tc := range []struct {
		name string
		edit func(step map[string]any)
	}{
		// An unknown extra field.
		{"unknown argument", func(s map[string]any) { setArgument(s, "Future", "x", json.RawMessage(`"x"`)) }},
		{"ByteOffset argument", func(s map[string]any) { setArgument(s, "ByteOffset", "0", json.RawMessage(`0`)) }},
		{"result text", func(s map[string]any) { obj(s, "generic", "result")["result"] = "fixture" }},
		{"error with other full text", func(s map[string]any) {
			s["error"] = map[string]any{"shortError": "file does not exist", "fullError": "other"}
		}},
		// A changed status.
		{"status canceled", func(s map[string]any) { s["status"] = "CORTEX_STEP_STATUS_CANCELED" }},
		// A changed argument set.
		{"no EndLine", func(s map[string]any) { deleteArgument(s, "EndLine") }},
		{"no StartLine", func(s map[string]any) { deleteArgument(s, "StartLine") }},
		{"StartLine 0", func(s map[string]any) { setArgument(s, "StartLine", "0", json.RawMessage(`0`)) }},
		{"EndLine before StartLine", func(s map[string]any) {
			setArgument(s, "StartLine", "90", json.RawMessage(`90`))
		}},
		{"non-canonical line", func(s map[string]any) { setArgument(s, "EndLine", "080", json.RawMessage(`80`)) }},
		// Line ranges and errors never observed with this shape.
		{"StartLine 2", func(s map[string]any) { setArgument(s, "StartLine", "2", json.RawMessage(`2`)) }},
		{"lines 5-5", func(s map[string]any) {
			setArgument(s, "StartLine", "5", json.RawMessage(`5`))
			setArgument(s, "EndLine", "5", json.RawMessage(`5`))
		}},
		{"lines 100-200", func(s map[string]any) {
			setArgument(s, "StartLine", "100", json.RawMessage(`100`))
			setArgument(s, "EndLine", "200", json.RawMessage(`200`))
		}},
		{"lines 1-1", func(s map[string]any) { setArgument(s, "EndLine", "1", json.RawMessage(`1`)) }},
		{"another error", func(s map[string]any) { setError(s, "permission denied") }},
		{"retained call differs", func(s map[string]any) {
			call := obj(s, "metadata", "toolCall")
			call["argumentsJson"] = strings.Replace(call["argumentsJson"].(string), `"EndLine":80`, `"EndLine":81`, 1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			full, shown := c.load(t)
			require1216(t, snapshot1216(t, full), full, shown, Verified)
			bad := c.editStep(t, full, tc.edit)
			s := snapshot1216(t, bad)
			_, result := s.VerifyResponses([]byte(bad), []byte(shown))
			if result.State != Unknown || result.Reason != reason {
				t.Fatalf("got %+v, want unknown: %s", result, reason)
			}
			if _, err := s.OfflineJSON(); err == nil || !strings.Contains(err.Error(), reason) {
				t.Fatalf("offline export: %v, want %s", err, reason)
			}
		})
	}
}

// The lines are presented zero-based: end line 80 as 79 and start line 1 as
// absent. Another presented range does not verify, and a call from line 2,
// never observed with this shape, is refused even when presented as the
// hook-denied read's mapping would present it.
func TestReadLines150Presentation(t *testing.T) {
	c := readlines150Case
	full, shown := c.load(t)
	view := obj(c.stepOf(t, shown), "viewFile")
	if view["endLine"] != json.Number("79") || view["startLine"] != nil {
		t.Fatalf("fixture changed: %v", view)
	}
	for name, edit := range map[string]func(map[string]any){
		"end line not converted": func(v map[string]any) { v["endLine"] = 80 },
		"start line presented":   func(v map[string]any) { v["startLine"] = 1 },
		"no end line":            func(v map[string]any) { delete(v, "endLine") },
	} {
		t.Run(name, func(t *testing.T) {
			bad := c.editStep(t, shown, func(s map[string]any) { edit(obj(s, "viewFile")) })
			require1216(t, snapshot1216(t, full), full, bad, Incomplete)
		})
	}
	t.Run("start line 2", func(t *testing.T) {
		fromTwo := c.editStep(t, full, func(s map[string]any) {
			setArgument(s, "StartLine", "2", json.RawMessage(`2`))
		})
		shownFromTwo := c.editStep(t, shown, func(s map[string]any) {
			obj(s, "viewFile")["startLine"] = 1
			call := obj(s, "metadata", "toolCall")
			var args map[string]json.RawMessage
			if err := json.Unmarshal([]byte(call["argumentsJson"].(string)), &args); err != nil {
				t.Fatal(err)
			}
			args["StartLine"] = json.RawMessage(`2`)
			b, _ := json.Marshal(args)
			call["argumentsJson"] = string(b)
		})
		s := snapshot1216(t, fromTwo)
		if _, r := s.VerifyResponses([]byte(fromTwo), []byte(shownFromTwo)); r.State != Unknown || r.Reason != "unsupported failed read_file projection" {
			t.Fatalf("got %+v, want unknown: unsupported failed read_file projection", r)
		}
	})
}

// A persisted field the descriptors do not know is rejected on the wrapper,
// the metadata and, for the failed read, the error.
func TestRetained150UnknownWireFields(t *testing.T) {
	extra := protowire.AppendVarint(protowire.AppendTag(nil, 9999, protowire.VarintType), 1)
	for _, tc := range []struct {
		c     denied1412Case
		field string
	}{
		{bgcancel150Wait, "generic"}, {bgcancel150NoTransition, "metadata"}, {bgcancel150Blocking, "generic"},
		{readlines150Case, "generic"}, {readlines150Case, "error"},
		{frontmatter150Literal, "generic"},
	} {
		t.Run(fmt.Sprintf("%s#%d: %s", tc.c.dir, tc.c.step, tc.field), func(t *testing.T) {
			full, shown := tc.c.load(t)
			require1216(t, snapshot1216(t, full), full, shown, Verified)
			m := trajectory1216(t, full).ProtoReflect()
			step := tool143Get(m, "steps").List().Get(tc.c.step).Message()
			step.Mutable(step.Descriptor().Fields().ByName(protoreflect.Name(tc.field))).Message().SetUnknown(extra)
			if _, err := finishSnapshot(m.Interface()); err == nil {
				t.Fatal("unknown wire field accepted")
			}
		})
	}
}

func TestRetained150InvalidatesPreviousProfile(t *testing.T) {
	for _, suffix := range []string{"-frontmatter150", "-bgcancel150", "-readlines150"} {
		if !strings.Contains(Profile, suffix) {
			t.Fatalf("profile does not name %s", suffix)
		}
	}
	previous := strings.SplitN(Profile, "-frontmatter150", 2)[0]
	for _, dir := range retained150Dirs {
		full, shown := denied1412Case{dir, 0}.load(t)
		s := snapshot1216(t, full)
		a, r := s.VerifyResponses([]byte(full), []byte(shown))
		if r.State != Verified {
			t.Fatal(r)
		}
		a.Profile = previous
		if r := s.CheckAttestation([]byte(shown), a); r.State != Unknown {
			t.Fatalf("%s: previous profile accepted: %+v", dir, r)
		}
	}
}
