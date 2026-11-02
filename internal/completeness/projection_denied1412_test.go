package completeness

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// testdata/hookdenied1412 and testdata/latekill1412 hold the full and default
// RPC responses of four native agy 1.4.12 sessions (agy-reader#gs51), each run
// with agy -p in a throwaway HOME and fetched from an idle daemon in that HOME
// that had not loaded them:
//
//   - readfile, readlines: a workspace PreToolUse hook denied read_file (lines
//     1-1, then 2-3); step 2 is the denied call.
//   - managetask: the same hook denied manage_task with Action list; step 2.
//   - latekill1412: no hook; a run_command (WaitMsBeforeAsync 5000) moved to
//     the background and finished, then manage_task kill arrived for it; step 7
//     is the failed kill.
//
// Only scrubbing was applied: signatures and long texts became length-tagged
// placeholders and the sandbox directory became /fixture-home. The SQLite
// sources are not kept; the snapshot is built from the full response.

type denied1412Case struct {
	dir  string
	step int
}

var (
	denied1412ReadFile   = denied1412Case{"hookdenied1412/readfile", 2}
	denied1412ReadLines  = denied1412Case{"hookdenied1412/readlines", 2}
	denied1412ManageTask = denied1412Case{"hookdenied1412/managetask", 2}
	denied1412LateKill   = denied1412Case{"latekill1412", 7}
)

func (c denied1412Case) load(t *testing.T) (full, shown string) {
	t.Helper()
	read := func(name string) string {
		b, err := os.ReadFile("testdata/" + c.dir + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		var d struct {
			Trajectory json.RawMessage `json:"trajectory"`
		}
		if err := json.Unmarshal(b, &d); err != nil || len(d.Trajectory) == 0 {
			t.Fatalf("%s/%s: %v", c.dir, name, err)
		}
		return string(d.Trajectory)
	}
	return read("full.json"), read("default.json")
}

// editStep applies edit to step c.step of a trajectory in JSON form.
func (c denied1412Case) editStep(t *testing.T, raw string, edit func(step map[string]any)) string {
	t.Helper()
	d := json.NewDecoder(strings.NewReader(raw))
	d.UseNumber()
	var traj map[string]any
	if err := d.Decode(&traj); err != nil {
		t.Fatal(err)
	}
	edit(traj["steps"].([]any)[c.step].(map[string]any))
	var out bytes.Buffer
	e := json.NewEncoder(&out)
	e.SetEscapeHTML(false)
	if err := e.Encode(traj); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func obj(m map[string]any, path ...string) map[string]any {
	for _, k := range path {
		m = m[k].(map[string]any)
	}
	return m
}

// setArgument changes or adds one argument in the wrapper and in the retained
// tool call together, so that the call-repeats-arguments check is not what
// refuses the edit. raw is the JSON form of the value in the tool call.
func setArgument(step map[string]any, name, value string, raw json.RawMessage) {
	obj(step, "generic", "args")[name] = value
	call := obj(step, "metadata", "toolCall")
	var args map[string]json.RawMessage
	if err := json.Unmarshal([]byte(call["argumentsJson"].(string)), &args); err != nil {
		panic(err)
	}
	args[name] = raw
	b, _ := json.Marshal(args)
	call["argumentsJson"] = string(b)
}

func setError(step map[string]any, text string) {
	step["error"] = map[string]any{"shortError": text, "fullError": text}
}

func TestDenied1412NativeVerifies(t *testing.T) {
	for _, c := range []denied1412Case{denied1412ReadFile, denied1412ReadLines, denied1412ManageTask, denied1412LateKill} {
		t.Run(c.dir, func(t *testing.T) {
			full, shown := c.load(t)
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

// The denied read is presented as a VIEW_FILE built from its arguments, with
// one-based lines made zero-based; the denied manage_task and the late kill are
// presented unchanged.
func TestDenied1412Presentation(t *testing.T) {
	_, shown := denied1412ReadLines.load(t)
	if !strings.Contains(shown, `"viewFile":{"absolutePathUri":"file:///fixture-home/ws5/notes.txt","startLine":1,"endLine":2}`) {
		t.Fatal("native read presentation changed")
	}
	for _, c := range []denied1412Case{denied1412ManageTask, denied1412LateKill} {
		full, shown := c.load(t)
		a, _ := json.Marshal(c.stepOf(t, full))
		b, _ := json.Marshal(c.stepOf(t, shown))
		if string(a) != string(b) {
			t.Fatalf("%s: the failed manage_task is not presented unchanged", c.dir)
		}
	}
}

// Each source edit is refused by the new rule itself (the reason names it), and
// the same edit in the default response alone is a mismatch.
func TestDenied1412Refusals(t *testing.T) {
	const (
		readRule   = "unsupported hook-denied read_file"
		manageRule = "unsupported failed manage_task"
	)
	type refusal struct {
		name   string
		c      denied1412Case
		reason string
		edit   func(step map[string]any)
		args   bool // the edit changes the call's arguments
	}
	var cases []refusal
	for _, c := range []denied1412Case{denied1412ReadFile, denied1412ReadLines, denied1412ManageTask, denied1412LateKill} {
		reason := manageRule
		if strings.Contains(c.dir, "read") {
			reason = readRule
		}
		cases = append(cases,
			refusal{"unknown argument", c, reason, func(s map[string]any) { setArgument(s, "Future", "x", json.RawMessage(`"x"`)) }, true},
			refusal{"extra outer error field", c, reason, func(s map[string]any) { obj(s, "error")["errorId"] = "fixture" }, false},
			refusal{"status canceled", c, "unsupported generic wrapper", func(s map[string]any) { s["status"] = "CORTEX_STEP_STATUS_CANCELED" }, false},
			refusal{"status invalid", c, "unsupported generic wrapper", func(s map[string]any) { s["status"] = "CORTEX_STEP_STATUS_INVALID" }, false},
			refusal{"error changed", c, reason, func(s map[string]any) { setError(s, "tool call failed: fixture") }, false},
			refusal{"full error differs", c, reason, func(s map[string]any) { obj(s, "error")["fullError"] = "other" }, false},
			refusal{"second hook result", c, reason, func(s map[string]any) {
				m := obj(s, "metadata")
				hooks := m["preToolHookResults"].([]any)
				m["preToolHookResults"] = append(hooks, hooks[0])
			}, false},
		)
	}
	for _, c := range []denied1412Case{denied1412ReadFile, denied1412ReadLines, denied1412ManageTask} {
		reason := manageRule
		if strings.Contains(c.dir, "read") {
			reason = readRule
		}
		cases = append(cases,
			refusal{"hook allows", c, reason, func(s map[string]any) {
				obj(s, "metadata")["preToolHookResults"] = []any{map[string]any{"decision": "allow"}}
			}, false},
			refusal{"reason the error does not repeat", c, reason, func(s map[string]any) {
				obj(s, "metadata")["preToolHookResults"].([]any)[0].(map[string]any)["reason"] = "another reason"
			}, false},
		)
	}
	for _, c := range []denied1412Case{denied1412ReadFile, denied1412ReadLines} {
		cases = append(cases,
			refusal{"skill file path", c, readRule, func(s map[string]any) {
				setArgument(s, "Path", "/fixture-home/ws/SKILL.md", json.RawMessage(`"/fixture-home/ws/SKILL.md"`))
			}, true},
			refusal{"path with a space", c, readRule, func(s map[string]any) {
				setArgument(s, "Path", "/fixture-home/ws/a b.txt", json.RawMessage(`"/fixture-home/ws/a b.txt"`))
			}, true},
			refusal{"lines reversed", c, readRule, func(s map[string]any) {
				setArgument(s, "StartLine", "5", json.RawMessage(`5`))
				setArgument(s, "EndLine", "4", json.RawMessage(`4`))
			}, true},
			refusal{"no lines", c, readRule, func(s map[string]any) {
				delete(obj(s, "generic", "args"), "StartLine")
				delete(obj(s, "generic", "args"), "EndLine")
				call := obj(s, "metadata", "toolCall")
				var args map[string]json.RawMessage
				_ = json.Unmarshal([]byte(call["argumentsJson"].(string)), &args)
				delete(args, "StartLine")
				delete(args, "EndLine")
				b, _ := json.Marshal(args)
				call["argumentsJson"] = string(b)
			}, true},
			refusal{"another tool source", c, readRule, func(s map[string]any) { obj(s, "metadata")["toolSource"] = "CORTEX_COMPONENT_SOURCE_CUSTOMIZATION" }, false},
		)
	}
	cases = append(cases,
		refusal{"late kill: another task status", denied1412LateKill, manageRule, func(s map[string]any) {
			setError(s, `cannot kill task "da0b9198-8ad1-4101-b05c-7e2057bfb754/task-2": task is not running (status: ERROR)`)
		}, false},
		refusal{"late kill: error names another task", denied1412LateKill, manageRule, func(s map[string]any) {
			setError(s, `cannot kill task "da0b9198-8ad1-4101-b05c-7e2057bfb754/task-4": task is not running (status: DONE)`)
		}, false},
		refusal{"late kill: another action", denied1412LateKill, manageRule, func(s map[string]any) { setArgument(s, "Action", "status", json.RawMessage(`"status"`)) }, true},
		refusal{"late kill: hook denies", denied1412LateKill, manageRule, func(s map[string]any) {
			obj(s, "metadata")["preToolHookResults"] = []any{map[string]any{"decision": "deny", "reason": "x"}}
		}, false},
		refusal{"late kill: hook with a reason", denied1412LateKill, manageRule, func(s map[string]any) {
			obj(s, "metadata")["preToolHookResults"] = []any{map[string]any{"decision": "allow", "reason": "x"}}
		}, false},
	)
	for _, tc := range cases {
		t.Run(tc.c.dir+": "+tc.name, func(t *testing.T) {
			full, shown := tc.c.load(t)
			bad := tc.c.editStep(t, full, tc.edit)
			s := snapshot1216(t, bad)
			_, result := s.VerifyResponses([]byte(bad), []byte(shown))
			if result.State != Unknown || result.Reason != tc.reason {
				t.Fatalf("got %+v, want unknown: %s", result, tc.reason)
			}
			if _, sourceProven := s.compareResponses([]byte(bad), []byte(shown)); !sourceProven {
				t.Fatal("unsupported source must produce a source-proven refusal")
			}
			// The supported source against a default response carrying the
			// same edit is a mismatch. A denied read's presentation has no
			// generic arguments, so argument edits apply to the source only.
			shownStep := tc.c.editStep(t, shown, func(map[string]any) {})
			if step := tc.c.stepOf(t, shown); step["generic"] != nil || !tc.args {
				mismatch := tc.c.editStep(t, shown, tc.edit)
				if mismatch == shownStep {
					t.Fatal("edit changed nothing in the default response")
				}
				require1216(t, snapshot1216(t, full), full, mismatch, Incomplete)
			}
		})
	}
}

func (c denied1412Case) stepOf(t *testing.T, raw string) map[string]any {
	t.Helper()
	var step map[string]any
	c.editStep(t, raw, func(s map[string]any) { step = s })
	return step
}

// A persisted field the descriptors do not know is rejected on the error and
// the wrapper of each native step.
func TestDenied1412UnknownWireFields(t *testing.T) {
	extra := protowire.AppendVarint(protowire.AppendTag(nil, 9999, protowire.VarintType), 1)
	for _, c := range []denied1412Case{denied1412ReadFile, denied1412ReadLines, denied1412ManageTask, denied1412LateKill} {
		for _, field := range []string{"error", "generic"} {
			t.Run(c.dir+": "+field, func(t *testing.T) {
				full, shown := c.load(t)
				require1216(t, snapshot1216(t, full), full, shown, Verified)
				m := trajectory1216(t, full).ProtoReflect()
				step := tool143Get(m, "steps").List().Get(c.step).Message()
				step.Mutable(step.Descriptor().Fields().ByName(protoreflect.Name(field))).Message().SetUnknown(extra)
				if _, err := finishSnapshot(m.Interface()); err == nil {
					t.Fatal("unknown wire field accepted")
				}
			})
		}
	}
}

func TestDenied1412InvalidatesPreviousProfile(t *testing.T) {
	previous := strings.SplitN(Profile, "-denied1412", 2)[0]
	if previous == Profile {
		t.Fatal("profile does not name denied1412")
	}
	full, shown := denied1412ReadLines.load(t)
	s := snapshot1216(t, full)
	a, r := s.VerifyResponses([]byte(full), []byte(shown))
	if r.State != Verified {
		t.Fatal(r)
	}
	a.Profile = previous
	if r := s.CheckAttestation([]byte(shown), a); r.State != Unknown {
		t.Fatalf("previous profile accepted: %+v", r)
	}
}
