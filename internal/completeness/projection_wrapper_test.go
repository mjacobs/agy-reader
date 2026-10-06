package completeness

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// These fixtures reproduce three failed generic wrapper shapes found in
// retained historical sessions (agy-reader#dgzv). Frozen source, full RPC and
// default RPC copies showed what the default response presents:
//
//   - A wrapper with an outer error over an embedded view_file, run_command or
//     code-action Step presents the native Step unchanged. Its inner error
//     repeats the outer short error; the outer error, metadata and status are
//     dropped with the wrapper.
//   - A wrapper that failed before its tool ran has arguments and an outer
//     error but no result, and is presented unchanged.
//   - A failed generate_image call embeds a native GENERATE_IMAGE Step, not a
//     GenerateImage payload, and is presented like the first shape.
//
// Values are synthetic; the expected presentations are written out explicitly.
const wrapperErrorArguments = `{\"AbsolutePath\":\"/fixture/notes.md\",\"ContentOffset\":9000,\"EndLine\":60,\"StartLine\":20,\"toolAction\":\"Viewing notes\",\"toolSummary\":\"View notes\"}`
const wrapperErrorNative = `"type":"CORTEX_STEP_TYPE_VIEW_FILE","status":"CORTEX_STEP_STATUS_ERROR","metadata":{"toolCall":{"id":"call-offset","name":"view_file","argumentsJson":"` + wrapperErrorArguments + `"},"completedAt":"2026-01-01T00:00:01Z"},"error":{"shortError":"ContentOffset 9000 exceeds line range size 1200"},"viewFile":{"absolutePathUri":"file:///fixture/notes.md","startLine":19,"endLine":59,"contentOffset":9000,"lineRangeBytes":1200}`
const wrapperErrorRaw = `{"cascadeId":"wrapper-error-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR","metadata":{"toolCall":{"id":"call-offset","name":"view_file","argumentsJson":"` + wrapperErrorArguments + `"},"completedAt":"2026-01-01T00:00:02Z"},"error":{"shortError":"ContentOffset 9000 exceeds line range size 1200","fullError":"view file: ContentOffset 9000 exceeds line range size 1200"},"generic":{"args":{"AbsolutePath":"/fixture/notes.md","ContentOffset":"9000","EndLine":"60","StartLine":"20","toolAction":"Viewing notes","toolSummary":"View notes"},"result":{"result":"Encountered error in step execution: ContentOffset 9000 exceeds line range size 1200","payload":{"@type":"type.googleapis.com/gemini_coder.Step",` + wrapperErrorNative + `}}}}]}`
const wrapperErrorPresentation = `{"cascadeId":"wrapper-error-fixture","metadata":{},"steps":[{` + wrapperErrorNative + `}]}`

const wrapperResultlessRaw = `{"cascadeId":"resultless-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR","metadata":{"toolCall":{"id":"call-write","name":"write_to_file","argumentsJson":"{\"ArtifactMetadata\":{\"RequestFeedback\":false,\"Summary\":\"Fixture notes\",\"UserFacing\":true},\"CodeContent\":\"hello\\n\",\"Description\":\"Write notes\",\"Overwrite\":true,\"TargetFile\":\"/fixture/out/notes.md\",\"toolAction\":\"Writing notes\",\"toolSummary\":\"Notes\"}"},"completedAt":"2026-01-01T00:00:01Z"},"error":{"shortError":"invalid tool call: /fixture/out/notes.md is not a valid artifact path","fullError":"invalid tool call: /fixture/out/notes.md is not a valid artifact path"},"generic":{"args":{"ArtifactMetadata":"{\"RequestFeedback\":false,\"Summary\":\"Fixture notes\",\"UserFacing\":true}","CodeContent":"hello\n","Description":"Write notes","Overwrite":"true","TargetFile":"/fixture/out/notes.md","toolAction":"Writing notes","toolSummary":"Notes"}}}]}`
const wrapperResultlessViewRaw = `{"cascadeId":"resultless-view-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR","metadata":{"toolCall":{"id":"call-view","name":"view_file","argumentsJson":"{\"AbsolutePath\":\"/fixture/missing.go\",\"EndLine\":60,\"StartLine\":1,\"toolAction\":\"Viewing missing.go\",\"toolSummary\":\"View missing.go\"}"},"completedAt":"2026-01-01T00:00:01Z"},"error":{"shortError":"failed to read file: open /fixture/missing.go: no such file or directory","fullError":"invalid tool call: failed to read file: open /fixture/missing.go: no such file or directory"},"generic":{"args":{"AbsolutePath":"/fixture/missing.go","EndLine":"60","StartLine":"1","toolAction":"Viewing missing.go","toolSummary":"View missing.go"}}}]}`

const wrapperImageArguments = `{\"AspectRatio\":\"1:1\",\"ImageName\":\"bird\",\"Prompt\":\"Draw a bird\",\"toolAction\":\"Drawing bird\",\"toolSummary\":\"Bird illustration\"}`
const wrapperImageNative = `"type":"CORTEX_STEP_TYPE_GENERATE_IMAGE","status":"CORTEX_STEP_STATUS_ERROR","metadata":{"toolCall":{"id":"call-image","name":"generate_image","argumentsJson":"` + wrapperImageArguments + `"},"completedAt":"2026-01-01T00:00:01Z","retryInfos":[{"error":"429 Too Many Requests","retryReason":"RETRY_REASON_NON_RECOVERABLE"}]},"error":{"shortError":"failed to generate content: 429 Too Many Requests"},"generateImage":{"prompt":"Draw a bird","imageName":"bird","aspectRatio":"1:1"}`
const wrapperImageRaw = `{"cascadeId":"image-error-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR","metadata":{"toolCall":{"id":"call-image","name":"generate_image","argumentsJson":"` + wrapperImageArguments + `"},"completedAt":"2026-01-01T00:00:02Z"},"error":{"shortError":"failed to generate content: 429 Too Many Requests","fullError":"generate image: failed to generate content: 429 Too Many Requests, quota exhausted"},"generic":{"args":{"AspectRatio":"1:1","ImageName":"bird","Prompt":"Draw a bird","toolAction":"Drawing bird","toolSummary":"Bird illustration"},"result":{"result":"Encountered error in step execution: failed to generate content: 429 Too Many Requests","fullOutputUri":"file:///fixture/steps/1/output.txt","payload":{"@type":"type.googleapis.com/gemini_coder.Step",` + wrapperImageNative + `}}}}]}`
const wrapperImagePresentation = `{"cascadeId":"image-error-fixture","metadata":{},"steps":[{` + wrapperImageNative + `}]}`

func TestProjectionWrapperQualifiedShapes(t *testing.T) {
	for name, pair := range map[string][2]string{
		"outer error over native step":  {wrapperErrorRaw, wrapperErrorPresentation},
		"resultless write_to_file":      {wrapperResultlessRaw, wrapperResultlessRaw},
		"resultless view_file":          {wrapperResultlessViewRaw, wrapperResultlessViewRaw},
		"failed generate_image":         {wrapperImageRaw, wrapperImagePresentation},
		"invalid resultless wrapper":    {strings.ReplaceAll(wrapperResultlessRaw, `"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_INVALID"`)},
		"resultless wrapper, list_dir":  {resultlessTool(t, "list_dir", `"DirectoryPath":"/fixture/private"`)},
		"resultless wrapper, schedule":  {resultlessTool(t, "schedule", `"DurationSeconds":"60","Prompt":"check again","TimerCondition":"none"`)},
		"resultless wrapper, find_name": {resultlessTool(t, "find_by_name", `"Excludes":"[]","Extensions":"[\"go\"]","SearchDirectory":"/fixture"`)},
	} {
		t.Run(name, func(t *testing.T) {
			raw, presentation := pair[0], pair[1]
			if presentation == "" {
				presentation = raw
			}
			require1216(t, snapshot1216(t, raw), raw, presentation, Verified)
		})
	}
}

// resultlessTool rewrites the resultless write_to_file fixture as another
// qualified tool with the given generic arguments.
func resultlessTool(t *testing.T, tool, args string) string {
	t.Helper()
	raw := replace1216(t, wrapperResultlessRaw, `"name":"write_to_file"`, `"name":"`+tool+`"`)
	start := strings.Index(raw, `"args":{`) + len(`"args":{`)
	end := strings.Index(raw[start:], `,"toolAction"`)
	return raw[:start] + args + raw[start+end:]
}

// A source mutation applies to the persisted trajectory and both RPC copies; the
// presentation follows only where it carries the same content. A presentation
// or raw mutation changes only that response.
func TestProjectionWrapperChanges(t *testing.T) {
	const outerStatus = `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR"`
	const outerDone = `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE"`
	for _, tc := range []struct {
		name, source, presentation, before, after string
		where                                     string
		want                                      State
	}{
		// outer error over a native Step
		{"outer error without inner error", wrapperErrorRaw, wrapperErrorPresentation, `"error":{"shortError":"ContentOffset 9000 exceeds line range size 1200"},`, ``, "source", Unknown},
		{"outer error differs from inner", wrapperErrorRaw, wrapperErrorPresentation, `1200","fullError"`, `1300","fullError"`, "source", Unknown},
		{"extra field in outer error", wrapperErrorRaw, wrapperErrorPresentation, `"error":{"shortError":"ContentOffset 9000 exceeds line range size 1200","fullError"`, `"error":{"errorId":"fixture-error","shortError":"ContentOffset 9000 exceeds line range size 1200","fullError"`, "source", Unknown},
		{"unexpected extra wrapper field", wrapperErrorRaw, wrapperErrorPresentation, `"generic":{`, `"permissions":{},"generic":{`, "source", Unknown},
		{"outer error on a step that did not fail", wrapperErrorRaw, wrapperErrorPresentation, outerStatus, outerDone, "source", Unknown},
		{"outer error over a native step that did not fail", wrapperErrorRaw, wrapperErrorPresentation, `"type":"CORTEX_STEP_TYPE_VIEW_FILE","status":"CORTEX_STEP_STATUS_ERROR"`, `"type":"CORTEX_STEP_TYPE_VIEW_FILE","status":"CORTEX_STEP_STATUS_DONE"`, "source", Unknown},
		{"outer error with unknown argument", wrapperErrorRaw, wrapperErrorPresentation, `"args":{`, `"args":{"Future":"new",`, "source", Unknown},
		{"outer error with changed offset argument", wrapperErrorRaw, wrapperErrorPresentation, `"ContentOffset":"9000"`, `"ContentOffset":"9001"`, "source", Unknown},
		{"presented error changed", wrapperErrorRaw, wrapperErrorPresentation, `size 1200"}`, `size 1300"}`, "presentation", Incomplete},
		{"presented error dropped", wrapperErrorRaw, wrapperErrorPresentation, `"error":{"shortError":"ContentOffset 9000 exceeds line range size 1200"},`, ``, "presentation", Incomplete},
		{"presented outer full error", wrapperErrorRaw, wrapperErrorPresentation, `size 1200"}`, `size 1200","fullError":"view file: ContentOffset 9000 exceeds line range size 1200"}`, "presentation", Incomplete},
		{"presented status changed", wrapperErrorRaw, wrapperErrorPresentation, `"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_DONE"`, "presentation", Incomplete},
		{"raw outer full error changed", wrapperErrorRaw, wrapperErrorPresentation, `"fullError":"view file:`, `"fullError":"read file:`, "raw", Incomplete},
		// resultless wrapper
		{"resultless wrapper without error", wrapperResultlessRaw, wrapperResultlessRaw, `"error":{"shortError":"invalid tool call: /fixture/out/notes.md is not a valid artifact path","fullError":"invalid tool call: /fixture/out/notes.md is not a valid artifact path"},`, ``, "source", Unknown},
		{"resultless wrapper that did not fail", wrapperResultlessRaw, wrapperResultlessRaw, outerStatus, outerDone, "source", Unknown},
		{"resultless wrapper canceled", wrapperResultlessRaw, wrapperResultlessRaw, outerStatus, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_CANCELED"`, "source", Unknown},
		{"resultless list_dir invalid", resultlessTool(t, "list_dir", `"DirectoryPath":"/fixture/private"`), resultlessTool(t, "list_dir", `"DirectoryPath":"/fixture/private"`), outerStatus, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_INVALID"`, "source", Unknown},
		{"resultless unknown argument", wrapperResultlessRaw, wrapperResultlessRaw, `"args":{`, `"args":{"Future":"new",`, "source", Unknown},
		{"resultless unrelated known argument", wrapperResultlessRaw, wrapperResultlessRaw, `"args":{`, `"args":{"CommandLine":"echo unsafe",`, "source", Unknown},
		{"resultless unexpected wrapper field", wrapperResultlessRaw, wrapperResultlessRaw, `"generic":{`, `"taskDetails":{"id":"resultless-fixture/task-1"},"generic":{`, "source", Unknown},
		{"resultless unqualified tool", wrapperResultlessRaw, wrapperResultlessRaw, `"name":"write_to_file"`, `"name":"grep_search"`, "source", Unknown},
		{"resultless view_file with tool source", wrapperResultlessViewRaw, wrapperResultlessViewRaw, `"completedAt"`, `"toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN","completedAt"`, "source", Unknown},
		{"resultless presented error changed", wrapperResultlessRaw, wrapperResultlessRaw, `"shortError":"invalid tool call`, `"shortError":"rejected tool call`, "presentation", Incomplete},
		{"resultless presented argument changed", wrapperResultlessRaw, wrapperResultlessRaw, `"CodeContent":"hello\n"`, `"CodeContent":"goodbye\n"`, "presentation", Incomplete},
		{"resultless presented status changed", wrapperResultlessRaw, wrapperResultlessRaw, outerStatus, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_INVALID"`, "presentation", Incomplete},
		{"resultless raw error changed", wrapperResultlessRaw, wrapperResultlessRaw, `"fullError":"invalid tool call`, `"fullError":"rejected tool call`, "raw", Incomplete},
		// failed generate_image
		{"image step that did not fail", wrapperImageRaw, wrapperImagePresentation, `"status":"CORTEX_STEP_STATUS_ERROR"`, `"status":"CORTEX_STEP_STATUS_DONE"`, "source", Unknown},
		{"image step without outer error", wrapperImageRaw, wrapperImagePresentation, `"error":{"shortError":"failed to generate content: 429 Too Many Requests","fullError":"generate image: failed to generate content: 429 Too Many Requests, quota exhausted"},`, ``, "source", Unknown},
		{"image step on another tool", wrapperImageRaw, wrapperImagePresentation, `"name":"generate_image"`, `"name":"edit_image"`, "source", Unknown},
		{"image step with unknown argument", wrapperImageRaw, wrapperImagePresentation, `"args":{`, `"args":{"Future":"new",`, "source", Unknown},
		{"image outer error differs from inner", wrapperImageRaw, wrapperImagePresentation, `Requests","fullError"`, `Requests (retry)","fullError"`, "source", Unknown},
		{"image inner status done", wrapperImageRaw, wrapperImagePresentation, `"type":"CORTEX_STEP_TYPE_GENERATE_IMAGE","status":"CORTEX_STEP_STATUS_ERROR"`, `"type":"CORTEX_STEP_TYPE_GENERATE_IMAGE","status":"CORTEX_STEP_STATUS_DONE"`, "source", Unknown},
		{"image presented error changed", wrapperImageRaw, wrapperImagePresentation, `429 Too Many Requests"}`, `500 Internal Server Error"}`, "presentation", Incomplete},
		{"image presented prompt changed", wrapperImageRaw, wrapperImagePresentation, `"prompt":"Draw a bird"`, `"prompt":"Draw a fish"`, "presentation", Incomplete},
		{"image raw outer error changed", wrapperImageRaw, wrapperImagePresentation, `quota exhausted`, `quota reset`, "raw", Incomplete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, presentation := tc.source, tc.presentation
			s := snapshot1216(t, raw)
			switch tc.where {
			case "source":
				raw = replace1216(t, raw, tc.before, tc.after)
				presentation = strings.ReplaceAll(presentation, tc.before, tc.after)
				s = snapshot1216(t, raw)
			case "raw":
				raw = replace1216(t, raw, tc.before, tc.after)
			case "presentation":
				presentation = replace1216(t, presentation, tc.before, tc.after)
			}
			require1216(t, s, raw, presentation, tc.want)
		})
	}
}

// An outer error over a native Step is accepted only for the types observed
// with one in retained sessions: view_file, run_command, code-action,
// generate_image, grep_search, call_mcp_tool, search_web and read_url_content.
// invoke_subagent is qualified when it succeeds, but a failed
// invoke_subagent wrapper with an outer error was never observed, so the same
// matching outer error over it still fails, as does every other step type.
func TestProjectionWrapperOuterErrorTypes(t *testing.T) {
	t.Run("invoke_subagent with matching outer error", func(t *testing.T) {
		const innerDone = `"type":"CORTEX_STEP_TYPE_INVOKE_SUBAGENT","status":"CORTEX_STEP_STATUS_DONE"`
		const innerError = `"type":"CORTEX_STEP_TYPE_INVOKE_SUBAGENT","status":"CORTEX_STEP_STATUS_ERROR"`
		raw := replace1216(t, syntheticInvokeRaw, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE"`, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR"`)
		raw = replace1216(t, raw, `"generic":{`, `"error":{"shortError":"boom","fullError":"boom full"},"generic":{`)
		raw = replace1216(t, replace1216(t, raw, innerDone, innerError), `"invokeSubagent":`, `"error":{"shortError":"boom"},"invokeSubagent":`)
		presentation := replace1216(t, replace1216(t, syntheticInvokePresentation, innerDone, innerError), `"invokeSubagent":`, `"error":{"shortError":"boom"},"invokeSubagent":`)
		require1216(t, snapshot1216(t, raw), raw, presentation, Unknown)
	})

	// The same minimal wrapper is built over every step type. Only the
	// observed types project, so the type list alone decides; a new type is
	// added there only with an observed source, full RPC and default RPC triplet.
	observed := map[protoreflect.Name]bool{
		"CORTEX_STEP_TYPE_CODE_ACTION":      true,
		"CORTEX_STEP_TYPE_GENERATE_IMAGE":   true,
		"CORTEX_STEP_TYPE_GREP_SEARCH":      true,
		"CORTEX_STEP_TYPE_MCP_TOOL":         true,
		"CORTEX_STEP_TYPE_READ_URL_CONTENT": true,
		"CORTEX_STEP_TYPE_RUN_COMMAND":      true,
		"CORTEX_STEP_TYPE_SEARCH_WEB":       true,
		"CORTEX_STEP_TYPE_VIEW_FILE":        true,
	}
	tools := map[protoreflect.Name]string{
		"CORTEX_STEP_TYPE_CODE_ACTION":      "replace_file_content",
		"CORTEX_STEP_TYPE_GENERATE_IMAGE":   "generate_image",
		"CORTEX_STEP_TYPE_INVOKE_SUBAGENT":  "invoke_subagent",
		"CORTEX_STEP_TYPE_RUN_COMMAND":      "run_command",
		"CORTEX_STEP_TYPE_VIEW_FILE":        "view_file",
		"CORTEX_STEP_TYPE_GREP_SEARCH":      "grep_search",
		"CORTEX_STEP_TYPE_MCP_TOOL":         "call_mcp_tool",
		"CORTEX_STEP_TYPE_READ_URL_CONTENT": "read_url_content",
		"CORTEX_STEP_TYPE_SEARCH_WEB":       "search_web",
		"CORTEX_STEP_TYPE_LIST_DIRECTORY":   "list_dir",
		"CORTEX_STEP_TYPE_FIND":             "find_by_name",
		"CORTEX_STEP_TYPE_FINISH":           "finish",
		"CORTEX_STEP_TYPE_ASK_QUESTION":     "ask_question",
	}
	m, err := newMessage("gemini_coder.Step")
	if err != nil {
		t.Fatal(err)
	}
	types := m.ProtoReflect().Descriptor().Fields().ByName("type").Enum().Values()
	for i := 0; i < types.Len(); i++ {
		typ := types.Get(i).Name()
		tool := tools[typ]
		if tool == "" {
			tool = "fixture_tool"
		}
		raw := `{"cascadeId":"outer-error-type-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR","metadata":{"toolCall":{"name":"` + tool + `"}},"error":{"shortError":"boom","fullError":"boom full"},"generic":{"args":{"toolAction":"Acting","toolSummary":"Action"},"result":{"result":"Encountered error in step execution: boom","payload":{"@type":"type.googleapis.com/gemini_coder.Step","type":"` + string(typ) + `","status":"CORTEX_STEP_STATUS_ERROR","metadata":{"toolCall":{"name":"` + tool + `","argumentsJson":"{\"toolAction\":\"Acting\",\"toolSummary\":\"Action\"}"}},"error":{"shortError":"boom"}}}}}]}`
		if _, err := project(snapshot1216(t, raw).trajectory); (err == nil) != observed[typ] {
			t.Errorf("%s with a matching outer error: projection error %v, want accepted=%v", typ, err, observed[typ])
		}
	}
}
