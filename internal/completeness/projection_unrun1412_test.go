package completeness

import (
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Two native agy 1.4.12 sessions (agy-reader#m2ng), each run with agy -p in a
// throwaway HOME and fetched from an idle daemon in that HOME that had not
// loaded them:
//
//   - canceledretry1412: the prompt asked for a run_command with Cwd and, if
//     that failed, one retry without Cwd. agy rejected the first call as
//     INVALID (step 2, the cwd1412 rule), and headless agy canceled the retry
//     at its permission prompt (step 4): status CANCELED, the arguments
//     CommandLine, WaitMsBeforeAsync and toolSummary, no Cwd.
//   - hookdenied1412/runcommand: a workspace PreToolUse hook denied the only
//     call, a run_command without Cwd (step 2): status ERROR, the hook's
//     denial as the error.
//
// The default response presents both as a RUN_COMMAND step without a working
// directory. Only scrubbing was applied: signatures and long texts became
// length-tagged placeholders and the sandbox directory became /fixture-home.
// The SQLite sources are not kept; the snapshot is built from the full
// response.
var (
	unrun1412Canceled = denied1412Case{"canceledretry1412", 4}
	unrun1412Denied   = denied1412Case{"hookdenied1412/runcommand", 2}
	unrun1412Cases    = []denied1412Case{unrun1412Canceled, unrun1412Denied}
)

func TestUnrun1412NativeVerifies(t *testing.T) {
	for _, c := range unrun1412Cases {
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

// Each call is presented as a RUN_COMMAND step that keeps the wrapper's status
// and error and has no working directory.
func TestUnrun1412Presentation(t *testing.T) {
	for _, c := range []struct {
		denied1412Case
		status, command string
	}{
		{unrun1412Canceled, "CORTEX_STEP_STATUS_CANCELED", "sleep 7; echo M2NG_FIRST"},
		{unrun1412Denied, "CORTEX_STEP_STATUS_ERROR", "echo M2NG_HOOK"},
	} {
		t.Run(c.dir, func(t *testing.T) {
			full, shown := c.load(t)
			stored, presented := c.stepOf(t, full), c.stepOf(t, shown)
			if _, ok := obj(stored, "generic", "args")["Cwd"]; ok || stored["type"] != "CORTEX_STEP_TYPE_GENERIC" {
				t.Fatalf("stored step changed: %v", stored)
			}
			run := obj(presented, "runCommand")
			want := map[string]any{"commandLine": c.command, "proposedCommandLine": c.command, "blocking": true, "combinedOutput": map[string]any{}}
			a, _ := json.Marshal(run)
			b, _ := json.Marshal(want)
			if presented["type"] != "CORTEX_STEP_TYPE_RUN_COMMAND" || presented["status"] != c.status || string(a) != string(b) {
				t.Fatalf("native step changed: %v", presented)
			}
			a, _ = json.Marshal(stored["error"])
			b, _ = json.Marshal(presented["error"])
			if string(a) != string(b) {
				t.Fatal("error not presented as stored")
			}
		})
	}
}

// deleteArgument removes one argument from the wrapper and the retained call.
func deleteArgument(step map[string]any, name string) {
	delete(obj(step, "generic", "args"), name)
	call := obj(step, "metadata", "toolCall")
	var args map[string]json.RawMessage
	if err := json.Unmarshal([]byte(call["argumentsJson"].(string)), &args); err != nil {
		panic(err)
	}
	delete(args, name)
	b, _ := json.Marshal(args)
	call["argumentsJson"] = string(b)
}

// Each source edit of an accepted fixture is refused, with the reason of the
// check that refuses it: an unknown extra field, a changed status and a changed
// argument set.
func TestUnrun1412Refusals(t *testing.T) {
	const args = "unsupported generic arguments"
	type refusal struct {
		name   string
		reason string
		edit   func(step map[string]any)
	}
	shared := []refusal{
		// An unknown extra field.
		{"unknown argument", args, func(s map[string]any) { setArgument(s, "Future", "x", json.RawMessage(`"x"`)) }},
		{"extra wrapper field", "unsupported generic wrapper", func(s map[string]any) {
			s["taskDetails"] = map[string]any{"id": "fixture"}
		}},
		// A changed argument set.
		{"toolAction argument", args, func(s map[string]any) {
			setArgument(s, "toolAction", "Running fixture", json.RawMessage(`"Running fixture"`))
		}},
		{"BypassSandbox argument", args, func(s map[string]any) {
			setArgument(s, "BypassSandbox", "false", json.RawMessage(`false`))
		}},
		{"IsDaemon argument", args, func(s map[string]any) { setArgument(s, "IsDaemon", "false", json.RawMessage(`false`)) }},
		{"no toolSummary", args, func(s map[string]any) { deleteArgument(s, "toolSummary") }},
		{"no wait", args, func(s map[string]any) { deleteArgument(s, "WaitMsBeforeAsync") }},
		{"retained call differs", args, func(s map[string]any) {
			obj(s, "metadata", "toolCall")["argumentsJson"] = `{"CommandLine":"echo other","WaitMsBeforeAsync":5000,"toolSummary":"Other"}`
		}},
	}
	perCase := map[string][]refusal{
		unrun1412Canceled.dir: {
			{"status error", "unsupported generic fields", func(s map[string]any) { s["status"] = "CORTEX_STEP_STATUS_ERROR" }},
			{"status done", "unsupported generic fields", func(s map[string]any) { s["status"] = "CORTEX_STEP_STATUS_DONE" }},
		},
		unrun1412Denied.dir: {
			{"status canceled", "unsupported generic wrapper", func(s map[string]any) { s["status"] = "CORTEX_STEP_STATUS_CANCELED" }},
			{"status invalid", "unsupported invalid run_command", func(s map[string]any) { s["status"] = "CORTEX_STEP_STATUS_INVALID" }},
			{"error differs from the hook reason", "unsupported run_command hook results", func(s map[string]any) {
				setError(s, "tool call denied by pre-tool hook: other")
			}},
		},
	}
	for _, c := range unrun1412Cases {
		for _, tc := range append(append([]refusal{}, shared...), perCase[c.dir]...) {
			t.Run(c.dir+": "+tc.name, func(t *testing.T) {
				full, shown := c.load(t)
				// The refusal is measured against the accepted fixture.
				require1216(t, snapshot1216(t, full), full, shown, Verified)
				bad := c.editStep(t, full, tc.edit)
				s := snapshot1216(t, bad)
				_, result := s.VerifyResponses([]byte(bad), []byte(shown))
				if result.State != Unknown || result.Reason != tc.reason {
					t.Fatalf("got %+v, want unknown: %s", result, tc.reason)
				}
				if _, sourceProven := s.compareResponses([]byte(bad), []byte(shown)); !sourceProven {
					t.Fatal("unsupported source must produce a source-proven refusal")
				}
				if _, err := s.OfflineJSON(); err == nil || !strings.Contains(err.Error(), tc.reason) {
					t.Fatalf("offline export: %v, want %s", err, tc.reason)
				}
			})
		}
	}
}

// The working directory is presented only when the call carries Cwd: a source
// with Cwd, or a presentation with a working directory, is a mismatch.
func TestUnrun1412PresentationMismatch(t *testing.T) {
	for _, c := range unrun1412Cases {
		t.Run(c.dir, func(t *testing.T) {
			full, shown := c.load(t)
			withCwd := c.editStep(t, full, func(s map[string]any) {
				setArgument(s, "Cwd", "/fixture-home/ws", json.RawMessage(`"/fixture-home/ws"`))
			})
			require1216(t, snapshot1216(t, withCwd), withCwd, shown, Incomplete)
			shownCwd := c.editStep(t, shown, func(s map[string]any) { obj(s, "runCommand")["cwd"] = "/fixture-home/ws" })
			require1216(t, snapshot1216(t, full), full, shownCwd, Incomplete)
		})
	}
}

// A rejected permission request was captured only on agy 1.3.1, with Cwd. The
// same record with the 1.4.12 arguments stays unsupported.
func TestUnrun1412RejectedRequestNeedsCwd(t *testing.T) {
	r := strings.NewReplacer(
		`"Cwd":"/fixture/ws",`, ``, `"toolAction":"Running fixture",`, ``,
		`\"Cwd\":\"/fixture/ws\",`, ``, `\"toolAction\":\"Running fixture\",`, ``,
		`"cwd":"/fixture/ws",`, ``,
	)
	raw, shown := r.Replace(rejectedRaw), r.Replace(rejectedPresent)
	if raw == rejectedRaw || shown == rejectedPresent || strings.Contains(raw, "Cwd") || strings.Contains(raw, "toolAction") {
		t.Fatal("fixture unchanged")
	}
	s := snapshot1216(t, raw)
	if _, result := s.VerifyResponses([]byte(raw), []byte(shown)); result.State != Unknown || result.Reason != "unsupported generic arguments" {
		t.Fatalf("got %+v, want unknown: unsupported generic arguments", result)
	}
}

// A persisted field the descriptors do not know is rejected on the wrapper and,
// for the denied call, on its error.
func TestUnrun1412UnknownWireFields(t *testing.T) {
	extra := protowire.AppendVarint(protowire.AppendTag(nil, 9999, protowire.VarintType), 1)
	for _, tc := range []struct {
		c     denied1412Case
		field string
	}{{unrun1412Canceled, "generic"}, {unrun1412Canceled, "metadata"}, {unrun1412Denied, "generic"}, {unrun1412Denied, "error"}} {
		t.Run(tc.c.dir+": "+tc.field, func(t *testing.T) {
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

func TestUnrun1412InvalidatesPreviousProfile(t *testing.T) {
	previous := strings.SplitN(Profile, "-unrun1412", 2)[0]
	if previous == Profile {
		t.Fatal("profile does not name unrun1412")
	}
	for _, c := range unrun1412Cases {
		full, shown := c.load(t)
		s := snapshot1216(t, full)
		a, r := s.VerifyResponses([]byte(full), []byte(shown))
		if r.State != Verified {
			t.Fatal(r)
		}
		a.Profile = previous
		if r := s.CheckAttestation([]byte(shown), a); r.State != Unknown {
			t.Fatalf("%s: previous profile accepted: %+v", c.dir, r)
		}
	}
}
