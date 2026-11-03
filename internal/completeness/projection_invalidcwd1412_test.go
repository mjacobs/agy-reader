package completeness

import (
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// testdata/invalidcwd1412 holds the full and default RPC responses of one
// native agy 1.4.12 session (agy-reader#qw3t), run with agy -p in a throwaway
// HOME and fetched from an idle daemon in that HOME that had not loaded it.
// The prompt asked for one run_command call with a Cwd argument; agy rejected
// it as INVALID before running it (step 2), and the model then replied with
// the error. Only scrubbing was applied: signatures and long texts became
// length-tagged placeholders and the sandbox directory became /fixture-home.
// The SQLite source is not kept; the snapshot is built from the full response.
var invalidCwd1412 = denied1412Case{"invalidcwd1412", 2}

func TestInvalidCwd1412NativeVerifies(t *testing.T) {
	full, shown := invalidCwd1412.load(t)
	s := snapshot1216(t, full)
	require1216(t, s, full, shown, Verified)
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, full, string(offline), Verified)
}

// The rejected call is presented unchanged: the wrapper, its arguments and
// its error.
func TestInvalidCwd1412Presentation(t *testing.T) {
	full, shown := invalidCwd1412.load(t)
	stored, presented := invalidCwd1412.stepOf(t, full), invalidCwd1412.stepOf(t, shown)
	a, _ := json.Marshal(stored)
	b, _ := json.Marshal(presented)
	if string(a) != string(b) {
		t.Fatal("the rejected run_command is not presented unchanged")
	}
	if presented["status"] != "CORTEX_STEP_STATUS_INVALID" || obj(presented, "error")["shortError"] != cwdRejection1412 ||
		obj(presented, "generic", "args")["Cwd"] != "/fixture-home/ws" {
		t.Fatalf("native step changed: %v", presented)
	}
}

// Each source edit is refused (the reason names the rule that refused it),
// and the same edit in the default response alone is a mismatch.
func TestInvalidCwd1412Refusals(t *testing.T) {
	const rule = "unsupported invalid run_command"
	c := invalidCwd1412
	cases := []struct {
		name   string
		reason string
		edit   func(step map[string]any)
	}{
		{"unknown argument", rule, func(s map[string]any) { setArgument(s, "Future", "x", json.RawMessage(`"x"`)) }},
		{"IsDaemon argument", rule, func(s map[string]any) { setArgument(s, "IsDaemon", "false", json.RawMessage(`false`)) }},
		{"no Cwd argument", rule, func(s map[string]any) {
			delete(obj(s, "generic", "args"), "Cwd")
			call := obj(s, "metadata", "toolCall")
			var args map[string]json.RawMessage
			_ = json.Unmarshal([]byte(call["argumentsJson"].(string)), &args)
			delete(args, "Cwd")
			b, _ := json.Marshal(args)
			call["argumentsJson"] = string(b)
		}},
		{"retained call differs", rule, func(s map[string]any) {
			obj(s, "metadata", "toolCall")["argumentsJson"] = `{"CommandLine":"echo other","Cwd":"/fixture-home/ws","WaitMsBeforeAsync":5000,"toolSummary":"Echo test string"}`
		}},
		{"extra outer error field", rule, func(s map[string]any) { obj(s, "error")["errorId"] = "fixture" }},
		{"extra wrapper field", rule, func(s map[string]any) {
			s["taskDetails"] = map[string]any{"id": "fixture"}
		}},
		{"hook result", rule, func(s map[string]any) {
			obj(s, "metadata")["preToolHookResults"] = []any{map[string]any{"decision": "allow"}}
		}},
		{"another tool source", rule, func(s map[string]any) { obj(s, "metadata")["toolSource"] = "CORTEX_COMPONENT_SOURCE_CUSTOMIZATION" }},
		{"another history", rule, func(s map[string]any) {
			internal := obj(s, "metadata", "internalMetadata")
			transitions := internal["statusTransitions"].([]any)
			internal["statusTransitions"] = []any{transitions[1]}
		}},
		{"status error", "unsupported generic wrapper", func(s map[string]any) { s["status"] = "CORTEX_STEP_STATUS_ERROR" }},
		{"status canceled", "unsupported generic wrapper", func(s map[string]any) { s["status"] = "CORTEX_STEP_STATUS_CANCELED" }},
		{"error names another property", rule, func(s map[string]any) {
			setError(s, "invalid arguments:\n- additional properties 'Directory' not allowed")
		}},
		{"error names a second property", rule, func(s map[string]any) {
			setError(s, cwdRejection1412+"\n- additional properties 'Future' not allowed")
		}},
		{"missing-summary error", rule, func(s map[string]any) { setError(s, "invalid arguments:\n- missing property 'toolSummary'") }},
		{"full error differs", rule, func(s map[string]any) { obj(s, "error")["fullError"] = "other" }},
		{"short error only", rule, func(s map[string]any) { delete(obj(s, "error"), "fullError") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			full, shown := c.load(t)
			bad := c.editStep(t, full, tc.edit)
			s := snapshot1216(t, bad)
			_, result := s.VerifyResponses([]byte(bad), []byte(shown))
			if result.State != Unknown || result.Reason != tc.reason {
				t.Fatalf("got %+v, want unknown: %s", result, tc.reason)
			}
			if _, sourceProven := s.compareResponses([]byte(bad), []byte(shown)); !sourceProven {
				t.Fatal("unsupported source must produce a source-proven refusal")
			}
			mismatch := c.editStep(t, shown, tc.edit)
			if mismatch == c.editStep(t, shown, func(map[string]any) {}) {
				t.Fatal("edit changed nothing in the default response")
			}
			require1216(t, snapshot1216(t, full), full, mismatch, Incomplete)
		})
	}
}

// A persisted field the descriptors do not know is rejected on the error and
// the wrapper of the native step.
func TestInvalidCwd1412UnknownWireFields(t *testing.T) {
	extra := protowire.AppendVarint(protowire.AppendTag(nil, 9999, protowire.VarintType), 1)
	for _, field := range []string{"error", "generic"} {
		t.Run(field, func(t *testing.T) {
			full, shown := invalidCwd1412.load(t)
			require1216(t, snapshot1216(t, full), full, shown, Verified)
			m := trajectory1216(t, full).ProtoReflect()
			step := tool143Get(m, "steps").List().Get(invalidCwd1412.step).Message()
			step.Mutable(step.Descriptor().Fields().ByName(protoreflect.Name(field))).Message().SetUnknown(extra)
			if _, err := finishSnapshot(m.Interface()); err == nil {
				t.Fatal("unknown wire field accepted")
			}
		})
	}
}

func TestInvalidCwd1412InvalidatesPreviousProfile(t *testing.T) {
	previous := strings.SplitN(Profile, "-cwd1412", 2)[0]
	if previous == Profile {
		t.Fatal("profile does not name cwd1412")
	}
	full, shown := invalidCwd1412.load(t)
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
