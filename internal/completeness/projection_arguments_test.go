package completeness

import (
	"strings"
	"testing"
)

// These fixtures cover the generic arguments that agy 1.2.16 sessions carry
// outside the earlier allowlists (agy-reader#q4q5): send_message Priority,
// run_command BypassSandbox, IsDaemon, RunPersistent and RequestedTerminalID,
// and write_to_file ArtifactMetadata. Frozen source, full RPC and default RPC
// copies of ordinary sessions showed the default response presenting:
//
//   - send_message unchanged, Priority included;
//   - each run_command and write_to_file wrapper as its embedded native Step,
//     whose tool-call arguments repeat the dropped arguments and whose own fields
//     record them: sandbox_override, is_daemon, run_persistent and
//     requested_terminal_id for the flags (false is left unset), and
//     is_artifact_file and artifact_metadata for the artifact object.
//
// Values are synthetic and the expected presentations are written out
// explicitly. The source is the same trajectory as the full RPC copy, as in the
// other projection fixtures.

const argumentsSendRaw = `{"cascadeId":"priority-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-send","name":"send_message"}},"generic":{"args":{"Message":"Done","Priority":"high","Recipient":"parent-fixture"},"result":{"result":"Message sent to \"parent-fixture\".","stepRenderInfo":{"title":"Parent Agent","titlePrefix":"Messaged","markdown":"> Done","icon":"send"}}}}]}`

// runFlagsFixture builds the raw and presented forms of a finished
// run_command with the given flags. Native fields mirror the arguments the way
// the daemon wrote them: true is set, false is left out.
func runFlagsFixture(bypass, daemon, persistent bool, terminal string) (raw, presentation string) {
	flag := func(on bool) string {
		if on {
			return "true"
		}
		return "false"
	}
	arguments := `{\"BypassSandbox\":` + flag(bypass) + `,\"CommandLine\":\"echo fixture\",\"Cwd\":\"/fixture\",\"IsDaemon\":` + flag(daemon) +
		`,\"RequestedTerminalID\":\"` + terminal + `\",\"RunPersistent\":` + flag(persistent) + `,\"WaitMsBeforeAsync\":5000,\"toolAction\":\"Running\",\"toolSummary\":\"Run fixture\"}`
	nativeFlags := ""
	if bypass {
		nativeFlags += `"sandboxOverride":true,`
	}
	if daemon {
		nativeFlags += `"isDaemon":true,`
	}
	if persistent {
		nativeFlags += `"runPersistent":true,`
	}
	if terminal != "" {
		nativeFlags += `"requestedTerminalId":"` + terminal + `","terminalId":"` + terminal + `",`
	}
	native := `"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-flags","name":"run_command","argumentsJson":"` + arguments +
		`"},"completedAt":"2026-01-01T00:00:01Z"},"runCommand":{"commandLine":"echo fixture","proposedCommandLine":"echo fixture","cwd":"/fixture","waitMsBeforeAsync":"5000",` + nativeFlags +
		`"blocking":true,"exitCode":0,"shellName":"bash"}`
	raw = `{"cascadeId":"flags-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-flags","name":"run_command"},"completedAt":"2026-01-01T00:00:02Z"},"generic":{"args":{"BypassSandbox":"` +
		flag(bypass) + `","CommandLine":"echo fixture","Cwd":"/fixture","IsDaemon":"` + flag(daemon) + `","RequestedTerminalID":"` + terminal + `","RunPersistent":"` + flag(persistent) +
		`","WaitMsBeforeAsync":"5000","toolAction":"Running","toolSummary":"Run fixture"},"result":{"result":"\nThe command exited with code 0.\n","fullOutputUri":"file:///fixture/steps/1/output.txt","payload":{"@type":"type.googleapis.com/gemini_coder.Step",` + native + `}}}}]}`
	presentation = `{"cascadeId":"flags-fixture","metadata":{},"steps":[{` + native + `}]}`
	return raw, presentation
}

const argumentsArtifactArguments = `{\"ArtifactMetadata\":{\"RequestFeedback\":true,\"Summary\":\"Fixture plan\",\"UserFacing\":true},\"CodeContent\":\"hello\\n\",\"Description\":\"Write plan\",\"Overwrite\":true,\"TargetFile\":\"/fixture/brain/plan.md\",\"toolAction\":\"Writing plan\",\"toolSummary\":\"Create plan\"}`
const argumentsArtifactNative = `"type":"CORTEX_STEP_TYPE_CODE_ACTION","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-plan","name":"write_to_file","argumentsJson":"` + argumentsArtifactArguments +
	`"},"completedAt":"2026-01-01T00:00:01Z"},"codeAction":{"actionSpec":{"createFile":{"instruction":"hello\n","path":{"absoluteUri":"file:///fixture/brain/plan.md"},"overwrite":true}},"useFastApply":true,"isArtifactFile":true,"artifactMetadata":{"summary":"Fixture plan","updatedAt":"2026-01-01T00:00:00Z","requestFeedback":true,"userFacing":true},"description":"Write plan"}`
const argumentsArtifactRaw = `{"cascadeId":"artifact-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-plan","name":"write_to_file"},"completedAt":"2026-01-01T00:00:02Z"},"generic":{"args":{"ArtifactMetadata":"{\"RequestFeedback\":true,\"Summary\":\"Fixture plan\",\"UserFacing\":true}","CodeContent":"hello\n","Description":"Write plan","Overwrite":"true","TargetFile":"/fixture/brain/plan.md","toolAction":"Writing plan","toolSummary":"Create plan"},"result":{"result":"Created file file:///fixture/brain/plan.md with requested content.","fullOutputUri":"file:///fixture/steps/1/output.txt","payload":{"@type":"type.googleapis.com/gemini_coder.Step",` + argumentsArtifactNative + `}}}}]}`
const argumentsArtifactPresentation = `{"cascadeId":"artifact-fixture","metadata":{},"steps":[{` + argumentsArtifactNative + `}]}`

func TestProjectionArgumentsQualifiedShapes(t *testing.T) {
	allRaw, allPresentation := runFlagsFixture(true, true, true, "term-fixture")
	noneRaw, nonePresentation := runFlagsFixture(false, false, false, "")
	bypassRaw, bypassPresentation := runFlagsFixture(true, false, false, "")
	persistentRaw, persistentPresentation := runFlagsFixture(false, false, true, "term-fixture")
	for name, pair := range map[string][2]string{
		"send_message priority":              {argumentsSendRaw, argumentsSendRaw},
		"run_command all flags set":          {allRaw, allPresentation},
		"run_command all flags false":        {noneRaw, nonePresentation},
		"run_command bypass sandbox":         {bypassRaw, bypassPresentation},
		"run_command persistent in terminal": {persistentRaw, persistentPresentation},
		"write_to_file artifact metadata":    {argumentsArtifactRaw, argumentsArtifactPresentation},
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, snapshot1216(t, pair[0]), pair[0], pair[1], Verified)
		})
	}
	// A flag the call did not carry qualifies nothing, so each is optional.
	t.Run("run_command without flags", func(t *testing.T) {
		raw := allRaw
		for _, arg := range []string{`"BypassSandbox":"true",`, `"IsDaemon":"true",`, `"RequestedTerminalID":"term-fixture",`, `"RunPersistent":"true",`} {
			raw = replace1216(t, raw, arg, ``)
		}
		require1216(t, snapshot1216(t, raw), raw, allPresentation, Verified)
	})
	t.Run("write_to_file without artifact metadata", func(t *testing.T) {
		raw := replace1216(t, argumentsArtifactRaw, `"ArtifactMetadata":"{\"RequestFeedback\":true,\"Summary\":\"Fixture plan\",\"UserFacing\":true}",`, ``)
		require1216(t, snapshot1216(t, raw), raw, argumentsArtifactPresentation, Verified)
	})
	t.Run("send_message without priority", func(t *testing.T) {
		raw := replace1216(t, argumentsSendRaw, `"Priority":"high",`, ``)
		require1216(t, snapshot1216(t, raw), raw, raw, Verified)
	})
}

type argumentsChange struct {
	name, before, after, where string
	want                       State
}

// runArgumentsChanges applies each change to a copy of the fixture. A source
// change applies to the persisted trajectory, the full RPC and, where it
// carries the same content, the presentation; "raw" changes only the full RPC
// and "presentation" only the default response.
func runArgumentsChanges(t *testing.T, raw, presentation string, changes []argumentsChange) {
	t.Helper()
	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			r, p := raw, presentation
			s := snapshot1216(t, r)
			switch tc.where {
			case "source":
				r = replace1216(t, r, tc.before, tc.after)
				p = strings.ReplaceAll(p, tc.before, tc.after)
				s = snapshot1216(t, r)
			case "raw":
				r = replace1216(t, r, tc.before, tc.after)
			case "presentation":
				p = replace1216(t, p, tc.before, tc.after)
			}
			require1216(t, s, r, p, tc.want)
		})
	}
}

func TestProjectionArgumentsSendMessageChanges(t *testing.T) {
	runArgumentsChanges(t, argumentsSendRaw, argumentsSendRaw, []argumentsChange{
		{"unknown extra argument", `"args":{`, `"args":{"Future":"new",`, "source", Unknown},
		{"flag argument from another tool", `"args":{`, `"args":{"BypassSandbox":"true",`, "source", Unknown},
		{"new tool name", `"name":"send_message"`, `"name":"future_tool"`, "source", Unknown},
		{"presented as the daemon stored it", `"Priority":"high"`, `"Priority":"low"`, "source", Verified},
		{"raw priority changed", `"Priority":"high"`, `"Priority":"low"`, "raw", Incomplete},
		{"presented priority changed", `"Priority":"high"`, `"Priority":"low"`, "presentation", Incomplete},
		{"presented priority dropped", `"Priority":"high",`, ``, "presentation", Incomplete},
		{"presented status changed", `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_ERROR"`, "presentation", Incomplete},
		{"unequal status pair", `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_ERROR"`, "raw", Incomplete},
		{"error on a step that was not failed", `"status":"CORTEX_STEP_STATUS_DONE","metadata"`, `"status":"CORTEX_STEP_STATUS_ERROR","error":{"shortError":"failed"},"metadata"`, "source", Unknown},
	})
	t.Run("priority on another payload-free tool", func(t *testing.T) {
		raw := replace1216(t, syntheticManageRaw, `"args":{`, `"args":{"Priority":"high",`)
		require1216(t, snapshot1216(t, raw), raw, raw, Unknown)
	})
}

func TestProjectionArgumentsRunCommandChanges(t *testing.T) {
	raw, presentation := runFlagsFixture(true, true, true, "term-fixture")
	runArgumentsChanges(t, raw, presentation, []argumentsChange{
		{"unknown extra argument", `"args":{`, `"args":{"Future":"new",`, "source", Unknown},
		{"priority argument from another tool", `"args":{`, `"args":{"Priority":"high",`, "source", Unknown},
		{"new tool name", `"name":"run_command"`, `"name":"future_tool"`, "source", Unknown},
		{"bypass argument changed", `"BypassSandbox":"true"`, `"BypassSandbox":"false"`, "source", Unknown},
		{"daemon argument changed", `"IsDaemon":"true"`, `"IsDaemon":"false"`, "source", Unknown},
		{"persistent argument changed", `"RunPersistent":"true"`, `"RunPersistent":"false"`, "source", Unknown},
		{"terminal argument changed", `"RequestedTerminalID":"term-fixture"`, `"RequestedTerminalID":"other-terminal"`, "source", Unknown},
		{"non-canonical boolean argument", `"IsDaemon":"true"`, `"IsDaemon":"True"`, "source", Unknown},
		{"numeric boolean argument", `"RunPersistent":"true"`, `"RunPersistent":"1"`, "source", Unknown},
		{"flag changed in presented call", `\"IsDaemon\":true`, `\"IsDaemon\":false`, "source", Unknown},
		{"terminal changed in presented call", `\"RequestedTerminalID\":\"term-fixture\"`, `\"RequestedTerminalID\":\"other-terminal\"`, "source", Unknown},
		{"terminal null in presented call", `\"RequestedTerminalID\":\"term-fixture\"`, `\"RequestedTerminalID\":null`, "source", Unknown},
		{"flag missing from presented call", `\"RunPersistent\":true,`, ``, "source", Unknown},
		{"native bypass unset", `"sandboxOverride":true,`, ``, "source", Unknown},
		{"native daemon unset", `"isDaemon":true,`, ``, "source", Unknown},
		{"native persistent unset", `"runPersistent":true,`, ``, "source", Unknown},
		{"native terminal changed", `"requestedTerminalId":"term-fixture"`, `"requestedTerminalId":"other-terminal"`, "source", Unknown},
		{"unequal status pair", `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE"`, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR"`, "source", Unknown},
		{"raw flag argument changed", `"IsDaemon":"true"`, `"IsDaemon":"false"`, "raw", Incomplete},
		{"raw terminal argument changed", `"RequestedTerminalID":"term-fixture"`, `"RequestedTerminalID":"other-terminal"`, "raw", Incomplete},
		{"presented flag changed", `\"IsDaemon\":true`, `\"IsDaemon\":false`, "presentation", Incomplete},
		{"presented native flag changed", `"sandboxOverride":true,`, ``, "presentation", Incomplete},
		{"presented terminal changed", `"requestedTerminalId":"term-fixture"`, `"requestedTerminalId":"other-terminal"`, "presentation", Incomplete},
		{"presented status changed", `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_ERROR"`, "presentation", Incomplete},
	})
	t.Run("empty terminal argument with null in the presented call", func(t *testing.T) {
		raw, presentation := runFlagsFixture(false, false, false, "")
		raw = replace1216(t, raw, `\"RequestedTerminalID\":\"\"`, `\"RequestedTerminalID\":null`)
		presentation = strings.ReplaceAll(presentation, `\"RequestedTerminalID\":\"\"`, `\"RequestedTerminalID\":null`)
		require1216(t, snapshot1216(t, raw), raw, presentation, Unknown)
	})
	t.Run("flag on a native step of another tool", func(t *testing.T) {
		// A VIEW_FILE step cannot account for run_command flags.
		raw := replace1216(t, currentViewRaw, `"args":{`, `"args":{"BypassSandbox":"true",`)
		require1216(t, snapshot1216(t, raw), raw, currentViewPresentation, Unknown)
	})
}

func TestProjectionArgumentsRunCommandCanceled(t *testing.T) {
	// A command canceled while running keeps its flags; the daemon presents the
	// outer CANCELED status with the inner step's other content.
	arguments := `{\"BypassSandbox\":true,\"CommandLine\":\"sleep 60\",\"Cwd\":\"/fixture\",\"WaitMsBeforeAsync\":5000}`
	metadata := `"metadata":{"toolCall":{"id":"call-run","name":"run_command","argumentsJson":"` + arguments + `"},"completedAt":"2026-01-01T00:00:01Z"}`
	command := `"error":{"shortError":"context canceled"},"runCommand":{"commandLine":"sleep 60","proposedCommandLine":"sleep 60","cwd":"/fixture","waitMsBeforeAsync":"5000","sandboxOverride":true,"shellName":"bash"}`
	raw := `{"cascadeId":"canceled-flags-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_CANCELED","metadata":{"toolCall":{"id":"call-run","name":"run_command"},"completedAt":"2026-01-01T00:00:02Z"},"generic":{"args":{"BypassSandbox":"true","CommandLine":"sleep 60","Cwd":"/fixture","WaitMsBeforeAsync":"5000"},"result":{"result":"\nEncountered error in step execution: context canceled\n","fullOutputUri":"file:///fixture/steps/1/output.txt","payload":{"@type":"type.googleapis.com/gemini_coder.Step","type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_ERROR",` + metadata + `,` + command + `}}}}]}`
	presentation := `{"cascadeId":"canceled-flags-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_CANCELED",` + metadata + `,` + command + `}]}`
	require1216(t, snapshot1216(t, raw), raw, presentation, Verified)
	runArgumentsChanges(t, raw, presentation, []argumentsChange{
		{"unknown extra argument", `"args":{`, `"args":{"Future":"new",`, "source", Unknown},
		{"bypass argument changed", `"BypassSandbox":"true"`, `"BypassSandbox":"false"`, "source", Unknown},
		{"native bypass unset", `"sandboxOverride":true,`, ``, "source", Unknown},
		{"inner status done", `"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_DONE"`, "source", Unknown},
		{"presented status changed", `"status":"CORTEX_STEP_STATUS_CANCELED"`, `"status":"CORTEX_STEP_STATUS_ERROR"`, "presentation", Incomplete},
	})
}

func TestProjectionArgumentsArtifactMetadataKeyOrder(t *testing.T) {
	// The wrapper serializes the same object with its keys in another order;
	// that is the same metadata and must still qualify.
	raw := replace1216(t, argumentsArtifactRaw, `"ArtifactMetadata":"{\"RequestFeedback\":true,\"Summary\":\"Fixture plan\",\"UserFacing\":true}"`, `"ArtifactMetadata":"{\"Summary\":\"Fixture plan\",\"UserFacing\":true,\"RequestFeedback\":true}"`)
	require1216(t, snapshot1216(t, raw), raw, argumentsArtifactPresentation, Verified)
}

func TestProjectionArgumentsArtifactMetadataPresence(t *testing.T) {
	// The wrapper spells out RequestFeedback:false while the presented call and
	// the native step omit it; the decoded values agree, the present keys do not.
	raw := replace1216(t, argumentsArtifactRaw, `"ArtifactMetadata":"{\"RequestFeedback\":true,`, `"ArtifactMetadata":"{\"RequestFeedback\":false,`)
	raw = replace1216(t, raw, `\"ArtifactMetadata\":{\"RequestFeedback\":true,`, `\"ArtifactMetadata\":{`)
	raw = replace1216(t, raw, `"requestFeedback":true,`, ``)
	presentation := strings.ReplaceAll(argumentsArtifactPresentation, `\"ArtifactMetadata\":{\"RequestFeedback\":true,`, `\"ArtifactMetadata\":{`)
	presentation = strings.ReplaceAll(presentation, `"requestFeedback":true,`, ``)
	require1216(t, snapshot1216(t, raw), raw, presentation, Unknown)
}

func TestProjectionArgumentsArtifactMetadataNullAgainstZeroValue(t *testing.T) {
	// A null member would decode to false or "" and match a native value that
	// is false or empty, so these cases start from metadata where every side
	// holds those values. Only the refusal of null can then return Unknown.
	zero := func(s string) string {
		s = strings.ReplaceAll(s, `"ArtifactMetadata":"{\"RequestFeedback\":true,`, `"ArtifactMetadata":"{\"RequestFeedback\":false,`)
		s = strings.ReplaceAll(s, `\"ArtifactMetadata\":{\"RequestFeedback\":true,`, `\"ArtifactMetadata\":{\"RequestFeedback\":false,`)
		s = strings.ReplaceAll(s, `"requestFeedback":true,`, ``)
		s = strings.ReplaceAll(s, `\"Summary\":\"Fixture plan\"`, `\"Summary\":\"\"`)
		return strings.ReplaceAll(s, `"summary":"Fixture plan",`, ``)
	}
	raw, presentation := zero(argumentsArtifactRaw), zero(argumentsArtifactPresentation)
	t.Run("false and empty values on every side", func(t *testing.T) {
		require1216(t, snapshot1216(t, raw), raw, presentation, Verified)
	})
	t.Run("feedback argument null", func(t *testing.T) {
		r := replace1216(t, raw, `"ArtifactMetadata":"{\"RequestFeedback\":false,`, `"ArtifactMetadata":"{\"RequestFeedback\":null,`)
		require1216(t, snapshot1216(t, r), r, presentation, Unknown)
	})
	t.Run("summary argument null", func(t *testing.T) {
		r := replace1216(t, raw, `"ArtifactMetadata":"{\"RequestFeedback\":false,\"Summary\":\"\"`, `"ArtifactMetadata":"{\"RequestFeedback\":false,\"Summary\":null`)
		require1216(t, snapshot1216(t, r), r, presentation, Unknown)
	})
	t.Run("metadata argument null against empty native metadata", func(t *testing.T) {
		const object = `{\"RequestFeedback\":true,\"Summary\":\"Fixture plan\",\"UserFacing\":true}`
		const native = `"artifactMetadata":{"summary":"Fixture plan","updatedAt":"2026-01-01T00:00:00Z","requestFeedback":true,"userFacing":true}`
		const empty = `"artifactMetadata":{"updatedAt":"2026-01-01T00:00:00Z"}`
		r := replace1216(t, argumentsArtifactRaw, `"ArtifactMetadata":"`+object+`"`, `"ArtifactMetadata":"null"`)
		r = replace1216(t, r, `\"ArtifactMetadata\":`+object, `\"ArtifactMetadata\":null`)
		r = replace1216(t, r, native, empty)
		p := strings.ReplaceAll(argumentsArtifactPresentation, `\"ArtifactMetadata\":`+object, `\"ArtifactMetadata\":null`)
		p = strings.ReplaceAll(p, native, empty)
		require1216(t, snapshot1216(t, r), r, p, Unknown)
	})
}

func TestProjectionArgumentsArtifactMetadataChanges(t *testing.T) {
	runArgumentsChanges(t, argumentsArtifactRaw, argumentsArtifactPresentation, []argumentsChange{
		{"unknown extra argument", `"args":{`, `"args":{"Future":"new",`, "source", Unknown},
		{"new tool name", `"name":"write_to_file"`, `"name":"replace_file_content"`, "source", Unknown},
		{"unknown artifact key", `{\"RequestFeedback\":true,`, `{\"Future\":1,\"RequestFeedback\":true,`, "source", Unknown},
		{"feedback argument changed", `"ArtifactMetadata":"{\"RequestFeedback\":true`, `"ArtifactMetadata":"{\"RequestFeedback\":false`, "source", Unknown},
		{"summary argument changed", `"ArtifactMetadata":"{\"RequestFeedback\":true,\"Summary\":\"Fixture plan\"`, `"ArtifactMetadata":"{\"RequestFeedback\":true,\"Summary\":\"Other plan\"`, "source", Unknown},
		{"user facing argument changed", `\"UserFacing\":true}"`, `\"UserFacing\":false}"`, "source", Unknown},
		{"trailing bracket after the metadata object", `\"UserFacing\":true}"`, `\"UserFacing\":true}]"`, "source", Unknown},
		{"second object after the metadata object", `\"UserFacing\":true}"`, `\"UserFacing\":true}{}"`, "source", Unknown},
		{"summary key spelled twice with different case", `"ArtifactMetadata":"{\"RequestFeedback\":true,\"Summary\":\"Fixture plan\"`, `"ArtifactMetadata":"{\"RequestFeedback\":true,\"Summary\":\"Other plan\",\"summary\":\"Fixture plan\"`, "source", Unknown},
		{"summary key repeated", `"ArtifactMetadata":"{\"RequestFeedback\":true,\"Summary\":\"Fixture plan\"`, `"ArtifactMetadata":"{\"RequestFeedback\":true,\"Summary\":\"Other plan\",\"Summary\":\"Fixture plan\"`, "source", Unknown},
		{"metadata key in the wrong case", `\"UserFacing\":true}"`, `\"userFacing\":true}"`, "source", Unknown},
		{"feedback argument null", `"ArtifactMetadata":"{\"RequestFeedback\":true`, `"ArtifactMetadata":"{\"RequestFeedback\":null`, "source", Unknown},
		{"summary argument null", `"ArtifactMetadata":"{\"RequestFeedback\":true,\"Summary\":\"Fixture plan\"`, `"ArtifactMetadata":"{\"RequestFeedback\":true,\"Summary\":null`, "source", Unknown},
		{"user facing argument null", `\"UserFacing\":true}"`, `\"UserFacing\":null}"`, "source", Unknown},
		{"presented call changed", `\"ArtifactMetadata\":{\"RequestFeedback\":true`, `\"ArtifactMetadata\":{\"RequestFeedback\":false`, "source", Unknown},
		{"presented call lacks the object", `\"ArtifactMetadata\":{\"RequestFeedback\":true,\"Summary\":\"Fixture plan\",\"UserFacing\":true},`, ``, "source", Unknown},
		{"native summary changed", `"summary":"Fixture plan"`, `"summary":"Other plan"`, "source", Unknown},
		{"native feedback unset", `"requestFeedback":true,`, ``, "source", Unknown},
		{"native user facing unset", `,"userFacing":true}`, `}`, "source", Unknown},
		{"native step not an artifact file", `"isArtifactFile":true,`, ``, "source", Unknown},
		{"unequal status pair", `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE"`, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR"`, "source", Unknown},
		{"raw argument changed", `"ArtifactMetadata":"{\"RequestFeedback\":true`, `"ArtifactMetadata":"{\"RequestFeedback\":false`, "raw", Incomplete},
		{"presented summary changed", `"summary":"Fixture plan"`, `"summary":"Other plan"`, "presentation", Incomplete},
		{"presented timestamp changed", `"updatedAt":"2026-01-01T00:00:00Z"`, `"updatedAt":"2026-01-01T00:00:09Z"`, "presentation", Incomplete},
		{"presented call changed", `\"ArtifactMetadata\":{\"RequestFeedback\":true`, `\"ArtifactMetadata\":{\"RequestFeedback\":false`, "presentation", Incomplete},
	})
}

// TestProjectionArgumentsArtifactMetadataMalformedSummary is review finding
// rv-1de6.1. The saved wrapper and the presented tool call spell the summary
// with two different unpaired surrogate escapes, \ud800 and \ud801, and the
// native step presents U+FFFD. encoding/json decodes both escapes to U+FFFD, so
// without a check of the escapes the three summaries compared equal and the
// session verified.
func TestProjectionArgumentsArtifactMetadataMalformedSummary(t *testing.T) {
	malformed := func(text string) string {
		text = strings.ReplaceAll(text, `\"Summary\":\"Fixture plan\",\"UserFacing\":true}"`, `\"Summary\":\"\\ud800\",\"UserFacing\":true}"`)
		text = strings.ReplaceAll(text, `\"Summary\":\"Fixture plan\",\"UserFacing\":true},`, `\"Summary\":\"\\ud801\",\"UserFacing\":true},`)
		return strings.ReplaceAll(text, `"summary":"Fixture plan"`, `"summary":"\ufffd"`)
	}
	raw, presentation := malformed(argumentsArtifactRaw), malformed(argumentsArtifactPresentation)
	for _, want := range []string{`\"Summary\":\"\\ud800\"`, `\"Summary\":\"\\ud801\"`, `"summary":"\ufffd"`} {
		if !strings.Contains(raw, want) {
			t.Fatalf("raw fixture lacks %s", want)
		}
	}
	if strings.Contains(presentation, `\\ud800`) || !strings.Contains(presentation, `\"Summary\":\"\\ud801\"`) || !strings.Contains(presentation, `"summary":"\ufffd"`) {
		t.Fatal("presentation fixture not rewritten")
	}
	require1216(t, snapshot1216(t, raw), raw, presentation, Unknown)

	// A valid surrogate pair escape still verifies against the same character
	// written out, as an ordinary summary does.
	pair := strings.ReplaceAll(raw, `\\ud800`, `\\ud83d\\ude00 caf\\u00e9`)
	pair = strings.ReplaceAll(pair, `\\ud801`, `😀 café`)
	pair = strings.ReplaceAll(pair, `"summary":"\ufffd"`, `"summary":"😀 café"`)
	pairPresentation := strings.ReplaceAll(presentation, `\\ud801`, `😀 café`)
	pairPresentation = strings.ReplaceAll(pairPresentation, `"summary":"\ufffd"`, `"summary":"😀 café"`)
	require1216(t, snapshot1216(t, pair), pair, pairPresentation, Verified)

	// The same malformed escape on every side is still not a summary the
	// source presented, so it does not verify either.
	same := strings.ReplaceAll(raw, `\\ud801`, `\\ud800`)
	samePresentation := strings.ReplaceAll(presentation, `\\ud801`, `\\ud800`)
	require1216(t, snapshot1216(t, same), same, samePresentation, Unknown)
}

func TestLiteralStringRejectsMalformedUnicode(t *testing.T) {
	for raw, want := range map[string]bool{
		`"Fixture plan"`:    true,
		`"café é"`:          true,
		`"😀"`:               true,
		`"😀 tail"`:          true,
		`"\\ud800"`:         true, // an escaped backslash, then plain text
		`"\\😀"`:             true,
		`"�"`:               true,
		`"\ud800"`:          false,
		`"\udc00"`:          false,
		`"\ud800x"`:         false,
		`"\ud800\ud800"`:    false,
		`"\ud800A"`:         false,
		`"\ude00\ud83d"`:    false,
		`"\\\ud800"`:        false,
		"\"bad \xff byte\"": false,
		`"a😀\udc00b"`:       false,
	} {
		if _, ok := literalString([]byte(raw)); ok != want {
			t.Errorf("literalString(%s) accepted %v, want %v", raw, ok, want)
		}
	}
}
