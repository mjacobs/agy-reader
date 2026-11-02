package completeness

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// testdata/taskkill1412 holds the full and default RPC responses of a native
// agy 1.4.12 session: a grep moved to the background after WaitMsBeforeAsync
// 5000, then ended by the agent with manage_task kill. The run used a throwaway
// HOME and no hook; the same daemon-fresh sweep verified it. Only scrubbing was
// applied: signatures, prompt texts and command output became length-tagged
// placeholders, the sandbox path became /fixture-home, and the session ID was
// zeroed. The SQLite source is not kept; the snapshot is built from the full
// response, as in the synthetic fixtures. Refusal cases below are derived from
// these bytes by one edit each.

func taskKill1412Native(t *testing.T) (full, shown string) {
	t.Helper()
	read := func(name string) string {
		b, err := os.ReadFile("testdata/taskkill1412/" + name)
		if err != nil {
			t.Fatal(err)
		}
		var d struct {
			Trajectory json.RawMessage `json:"trajectory"`
		}
		if err := json.Unmarshal(b, &d); err != nil || len(d.Trajectory) == 0 {
			t.Fatalf("%s: %v", name, err)
		}
		return string(d.Trajectory)
	}
	return read("full.json"), read("default.json")
}

func TestProjectionTaskKill1412NativeVerifies(t *testing.T) {
	full, shown := taskKill1412Native(t)
	s := snapshot1216(t, full)
	require1216(t, s, full, shown, Verified)
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, full, string(offline), Verified)
}

// Each case is one edit of the native full response. Edits that touch an
// argument change the generic argument and the retained tool call together, so
// the call-matches-arguments check cannot be what refuses them; every case is
// meant to be refused by one named guard.
func TestProjectionTaskKill1412NativeRefusals(t *testing.T) {
	full, shown := taskKill1412Native(t)
	const (
		wait        = `"WaitMsBeforeAsync":"5000"`
		waitCall    = `\"WaitMsBeforeAsync\":5000`
		runArgs     = `"args":{"CommandLine"`
		runCall     = `{\"CommandLine\":\"grep -rl -i zzzzqq /usr\",`
		killArgs    = `"args":{"Action":"kill",`
		killCall    = `{\"Action\":\"kill\",`
		runDone     = `"status":"CORTEX_STEP_STATUS_CANCELED"`
		killDone    = `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE"`
		killPayload = `"action":"kill"`
	)
	type edit struct{ before, after string }
	for _, tc := range []struct {
		name  string
		edits []edit
	}{
		{"wait pin: another wait, argument and call agree", []edit{{wait, `"WaitMsBeforeAsync":"3000"`}, {waitCall, `\"WaitMsBeforeAsync\":3000`}}},
		{"command allow-list: unknown argument, argument and call agree", []edit{{runArgs, `"args":{"Future":"x","CommandLine"`}, {runCall, `{\"Future\":\"x\",\"CommandLine\":\"grep -rl -i zzzzqq /usr\",`}}},
		{"command status: done", []edit{{runDone, `"status":"CORTEX_STEP_STATUS_DONE"`}}},
		{"command error", []edit{{`"generic":{"args":{"CommandLine"`, `"error":{"shortError":"failed"},"generic":{"args":{"CommandLine"`}}},
		{"kill allow-list: unknown argument, argument and call agree", []edit{{killArgs, `"args":{"Future":"x","Action":"kill",`}, {killCall, `{\"Future\":\"x\",\"Action\":\"kill\",`}}},
		{"kill status: error", []edit{{killDone, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR"`}}},
		{"kill error added", []edit{{`"generic":{"args":{"Action":"kill"`, `"error":{"shortError":"failed"},"generic":{"args":{"Action":"kill"`}}},
		{"kill other action in payload", []edit{{killPayload, `"action":"future"`}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := full
			for _, e := range tc.edits {
				bad = replace1216(t, bad, e.before, e.after)
			}
			s := snapshot1216(t, bad)
			require1216(t, s, bad, shown, Unknown)
			if _, sourceProven := s.compareResponses([]byte(bad), []byte(shown)); !sourceProven {
				t.Fatal("unsupported source must produce a source-proven refusal")
			}
			// The same edit in the default response is a mismatch with a
			// supported source, except where only the source form carries it.
			if strings.Contains(shown, tc.edits[0].before) {
				mismatch := shown
				for _, e := range tc.edits {
					mismatch = strings.ReplaceAll(mismatch, e.before, e.after)
				}
				require1216(t, snapshot1216(t, full), full, mismatch, Incomplete)
			}
		})
	}
}

// A persisted field the descriptors do not know is rejected on the command and
// on the kill.
func TestProjectionTaskKill1412NativeUnknownWireFields(t *testing.T) {
	full, _ := taskKill1412Native(t)
	extra := protowire.AppendVarint(protowire.AppendTag(nil, 9999, protowire.VarintType), 1)
	for name, mutate := range map[string]func(steps protoreflect.List){
		"command result": func(steps protoreflect.List) {
			tool143Get(tool143Get(steps.Get(2).Message(), "generic").Message(), "result").Message().SetUnknown(extra)
		},
		"command task details": func(steps protoreflect.List) {
			tool143Get(steps.Get(2).Message(), "task_details").Message().SetUnknown(extra)
		},
		"kill payload": func(steps protoreflect.List) {
			payload := tool143Get(tool143Get(tool143Get(steps.Get(4).Message(), "generic").Message(), "result").Message(), "payload").Message()
			data := append(append([]byte(nil), tool143Get(payload, "value").Bytes()...), extra...)
			tool143Set(payload, "value", protoreflect.ValueOfBytes(data))
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := trajectory1216(t, full).ProtoReflect()
			mutate(tool143Get(m, "steps").List())
			if _, err := finishSnapshot(m.Interface()); err == nil {
				t.Fatal("unknown wire field accepted")
			}
		})
	}
}

// The task_notification system message that agy appends after a kill appeared
// in earlier native sessions of this shape but not in the preserved one, so
// its fixture is synthetic: the shape recorded from those sessions, appended to
// the preserved native steps. It is presented unchanged, so any difference
// between the two responses is a mismatch.
const taskKill1412Notice = `{"type":"CORTEX_STEP_TYPE_SYSTEM_MESSAGE","status":"CORTEX_STEP_STATUS_DONE","metadata":{"source":"CORTEX_STEP_SOURCE_SYSTEM","isInterruptingStep":true},"systemMessage":{"message":"[Message] sender=fixture/task-2 priority=MESSAGE_PRIORITY_LOW content=Task was canceled","renderInfo":{"title":"Search was canceled","hidden":true,"contentBlocks":[{"terminal":{"command":"grep","cwd":"/fixture","output":"scrubbed"}}]},"eventType":"task_notification","agentMessage":{"id":"fixture-message","recipient":"fixture-parent","sender":"fixture/task-2","priority":"MESSAGE_PRIORITY_LOW","hideFromUser":true,"content":"Task was canceled","renderDetails":{"messageTitle":"Search was canceled"}}}}`

func TestProjectionTaskKill1412SyntheticNotice(t *testing.T) {
	full, shown := taskKill1412Native(t)
	withNotice := func(s string) string {
		i := strings.Index(s, `"generatorMetadata"`)
		j := strings.LastIndex(s[:i], `],`)
		return s[:j] + `,` + taskKill1412Notice + s[j:]
	}
	raw, presented := withNotice(full), withNotice(shown)
	require1216(t, snapshot1216(t, raw), raw, presented, Verified)
	for name, change := range map[string][2]string{
		"event type": {`"eventType":"task_notification"`, `"eventType":"other"`},
		"title":      {`"title":"Search was canceled"`, `"title":"Search finished"`},
		"content":    {`"content":"Task was canceled"`, `"content":"Task finished"`},
		"status":     {`"type":"CORTEX_STEP_TYPE_SYSTEM_MESSAGE","status":"CORTEX_STEP_STATUS_DONE"`, `"type":"CORTEX_STEP_TYPE_SYSTEM_MESSAGE","status":"CORTEX_STEP_STATUS_ERROR"`},
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, snapshot1216(t, raw), raw, replace1216(t, presented, change[0], change[1]), Incomplete)
		})
	}
}
