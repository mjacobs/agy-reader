package completeness

import (
	"encoding/json"
	"strings"
	"testing"
)

// These fixtures cover the eight native tool steps that the projection
// unwrapped for the first time in agy-reader#3pq4: list_dir, find_by_name,
// grep_search, call_mcp_tool, search_web, finish, read_url_content and
// ask_question. Frozen source, full RPC and default RPC copies of retained
// agy 1.2.15 and 1.2.16 sessions showed that the default response presents the
// embedded native Step verbatim in place of the wrapper, for succeeded and
// failed calls alike, and drops the wrapper's own arguments, result text,
// metadata and outer error. For ask_question the wrapper and the native Step
// both carry the completed interactions. Values are synthetic; every response
// is spelled out by nativeToolFixture, never derived from the projection.

type nativeToolCase struct {
	name         string
	tool         string
	nativeType   string
	field        string // native Step field holding the tool's own result
	arguments    string // the call's arguments, without the toolAction/toolSummary pair
	body         string // that field's JSON object
	failure      string // short error of a failed call, or ""
	interactions string // completedInteractions on wrapper and native Step
	resultFile   bool   // the wrapper result has a full output file
}

// nativeToolFixture returns the source (and full RPC) trajectory and the default
// RPC trajectory for one call.
func nativeToolFixture(c nativeToolCase) (raw, presentation string) {
	args := c.arguments
	tail := `"toolAction":"Fixture action","toolSummary":"Fixture summary"`
	called := strings.TrimSuffix(args, "}") + "," + tail + "}"
	if args == "{}" {
		called = "{" + tail + "}"
	}
	quoted, _ := json.Marshal(called)
	wrapperArgs := ""
	var decoded map[string]json.RawMessage
	_ = json.Unmarshal([]byte(called), &decoded)
	for _, k := range sortedKeys(decoded) {
		// The wrapper records a JSON string unquoted and every other value,
		// null included, as its JSON text. Unmarshaling null into a string
		// would succeed and leave "".
		value := string(decoded[k])
		if strings.HasPrefix(value, `"`) {
			_ = json.Unmarshal(decoded[k], &value)
		}
		quotedValue, _ := json.Marshal(value)
		wrapperArgs += `"` + k + `":` + string(quotedValue) + `,`
	}
	wrapperArgs = strings.TrimSuffix(wrapperArgs, ",")
	status := "CORTEX_STEP_STATUS_DONE"
	outerError, innerError := "", ""
	if c.failure != "" {
		status = "CORTEX_STEP_STATUS_ERROR"
		outerError = `"error":{"shortError":"` + c.failure + `","fullError":"fixture: ` + c.failure + `"},`
		innerError = `"error":{"shortError":"` + c.failure + `"},`
	}
	interactions := ""
	if c.interactions != "" {
		interactions = `"completedInteractions":` + c.interactions + `,`
	}
	native := `"type":"` + c.nativeType + `","status":"` + status + `","metadata":{"toolCall":{"id":"call-fixture","name":"` + c.tool + `","argumentsJson":` + string(quoted) + `},"completedAt":"2026-01-01T00:00:01Z"},` + innerError + interactions + `"` + c.field + `":` + c.body
	output := ""
	if c.resultFile {
		output = `"fullOutputUri":"file:///fixture/output.txt",`
	}
	raw = `{"cascadeId":"native-tool-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"` + status + `","metadata":{"toolCall":{"id":"call-fixture","name":"` + c.tool + `"},"completedAt":"2026-01-01T00:00:02Z"},` + outerError + interactions + `"generic":{"args":{` + wrapperArgs + `},"result":{"result":"Fixture result text",` + output + `"payload":{"@type":"type.googleapis.com/gemini_coder.Step",` + native + `}}}}]}`
	presentation = `{"cascadeId":"native-tool-fixture","metadata":{},"steps":[{` + native + `}]}`
	return raw, presentation
}

func sortedKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

const (
	fixtureAskInteractions = `[{"request":{"askQuestion":{"questions":[{"question":"Which scope?","options":[{"id":"1","text":"Everything"},{"id":"2","text":"Capture only"}]},{"question":"Anything else?","options":[{"id":"1","text":"No"}]}]}},"response":{"trajectoryId":"fixture-trajectory","stepIndex":4,"askQuestion":{"responses":[{"question":"Which scope?","selectedOptionIds":["2"]},{"question":"Anything else?","writeInResponse":"Mention the fixture"}]}}}]`
	fixtureAskBody         = `{"questions":[{"question":"Which scope?","options":[{"id":"1","text":"Everything"},{"id":"2","text":"Capture only"}],"selectedOptionIds":["2"]},{"question":"Anything else?","options":[{"id":"1","text":"No"}],"writeInResponse":"Mention the fixture"}]}`
	fixtureAskArguments    = `{"questions":[{"is_multi_select":false,"options":["Everything","Capture only"],"question":"Which scope?"},{"is_multi_select":false,"options":["No"],"question":"Anything else?"}]}`
)

// nativeToolCases lists one succeeded fixture per tool, plus failed ones for the
// tools whose failed calls were observed with a native Step.
var nativeToolCases = []nativeToolCase{
	{name: "list_dir", tool: "list_dir", nativeType: "CORTEX_STEP_TYPE_LIST_DIRECTORY", field: "listDirectory", resultFile: true,
		arguments: `{"DirectoryPath":"/fixture/project"}`,
		body:      `{"directoryPathUri":"file:///fixture/project","results":[{"name":"src","isDir":true},{"name":"README.md","sizeBytes":"120"}]}`},
	{name: "find_by_name", tool: "find_by_name", nativeType: "CORTEX_STEP_TYPE_FIND", field: "find", resultFile: true,
		arguments: `{"Pattern":"*.go","SearchDirectory":"/fixture/project"}`,
		body:      `{"searchDirectory":"/fixture/project","pattern":"*.go","truncatedOutput":"main.go\nutil.go","truncatedTotalResults":2,"totalResults":2,"rawOutput":"main.go\nutil.go"}`},
	{name: "find_by_name with every argument", tool: "find_by_name", nativeType: "CORTEX_STEP_TYPE_FIND", field: "find",
		arguments: `{"Excludes":["build/**",".git/**"],"Extensions":["go","md"],"MaxDepth":3,"Pattern":"*","SearchDirectory":"/fixture/project","Type":"directory"}`,
		body:      `{"searchDirectory":"/fixture/project","pattern":"*","truncatedTotalResults":0,"totalResults":0}`},
	{name: "grep_search", tool: "grep_search", nativeType: "CORTEX_STEP_TYPE_GREP_SEARCH", field: "grepSearch", resultFile: true,
		arguments: `{"CaseInsensitive":true,"MatchPerLine":true,"Query":"fixture","SearchPath":"/fixture/project"}`,
		body:      `{"searchPathUri":"file:///fixture/project","query":"fixture","results":[{"relativePath":"main.go","absolutePath":"/fixture/project/main.go"}],"totalResults":1,"rawOutput":"main.go\n","commandRun":"rg -l fixture /fixture/project"}`},
	{name: "grep_search with every argument", tool: "grep_search", nativeType: "CORTEX_STEP_TYPE_GREP_SEARCH", field: "grepSearch",
		arguments: `{"CaseInsensitive":false,"Includes":["*.go","*.md"],"IsRegex":false,"MatchPerLine":true,"Query":"fixture","SearchPath":"/fixture/project"}`,
		body:      `{"searchPathUri":"file:///fixture/project","query":"fixture","totalResults":0}`},
	{name: "failed grep_search", tool: "grep_search", nativeType: "CORTEX_STEP_TYPE_GREP_SEARCH", field: "grepSearch", resultFile: true,
		failure:   "search path file:///fixture/missing does not exist",
		arguments: `{"Query":"fixture","SearchPath":"/fixture/missing"}`,
		body:      `{"searchPathUri":"file:///fixture/missing","query":"fixture"}`},
	{name: "call_mcp_tool", tool: "call_mcp_tool", nativeType: "CORTEX_STEP_TYPE_MCP_TOOL", field: "mcpTool", resultFile: true,
		arguments: `{"Arguments":{"detail":"full","limit":5},"ServerName":"fixture-server","ToolName":"summarize"}`,
		body:      `{"serverName":"fixture-server","toolCall":{"id":"call-fixture","name":"summarize","argumentsJson":"{\"detail\":\"full\",\"limit\":5}"},"resultString":"{\"status\":\"ok\"}"}`},
	{name: "failed call_mcp_tool", tool: "call_mcp_tool", nativeType: "CORTEX_STEP_TYPE_MCP_TOOL", field: "mcpTool", resultFile: true,
		failure:   "Error executing tool summarize: cannot connect to fixture-server",
		arguments: `{"Arguments":{},"ServerName":"fixture-server","ToolName":"summarize"}`,
		body:      `{"serverName":"fixture-server","toolCall":{"id":"call-fixture","name":"summarize","argumentsJson":"{}"}}`},
	{name: "search_web", tool: "search_web", nativeType: "CORTEX_STEP_TYPE_SEARCH_WEB", field: "searchWeb", resultFile: true,
		arguments: `{"query":"fixture \"changelog\" OR notes"}`,
		body:      `{"query":"fixture \"changelog\" OR notes","summary":"A fixture summary.","thirdPartyConfig":{"provider":"THIRD_PARTY_WEB_SEARCH_PROVIDER_GEMINI"},"searchType":"SEARCH_WEB_TYPE_WEB"}`},
	{name: "search_web with a domain", tool: "search_web", nativeType: "CORTEX_STEP_TYPE_SEARCH_WEB", field: "searchWeb", resultFile: true,
		arguments: `{"domain":"example.test","query":"fixture"}`,
		body:      `{"query":"fixture","summary":"A fixture summary.","thirdPartyConfig":{"provider":"THIRD_PARTY_WEB_SEARCH_PROVIDER_GEMINI"},"searchType":"SEARCH_WEB_TYPE_WEB"}`},
	{name: "failed search_web", tool: "search_web", nativeType: "CORTEX_STEP_TYPE_SEARCH_WEB", field: "searchWeb",
		failure:   "failed to generate content: 503 Service Unavailable",
		arguments: `{"query":"fixture"}`,
		body:      `{"query":"fixture","thirdPartyConfig":{"provider":"THIRD_PARTY_WEB_SEARCH_PROVIDER_GEMINI"},"searchType":"SEARCH_WEB_TYPE_WEB"}`},
	{name: "finish", tool: "finish", nativeType: "CORTEX_STEP_TYPE_FINISH", field: "finish", resultFile: true,
		arguments: `{"marker":"FIXTURE_MARKER","sum":16}`,
		body:      `{"output":{"arguments_json":"{\"marker\":\"FIXTURE_MARKER\",\"sum\":16}"},"outputString":"{\"marker\":\"FIXTURE_MARKER\",\"sum\":16}"}`},
	{name: "read_url_content", tool: "read_url_content", nativeType: "CORTEX_STEP_TYPE_READ_URL_CONTENT", field: "readUrlContent", resultFile: true,
		arguments: `{"Url":"https://example.test/page"}`,
		body:      `{"url":"https://example.test/page","webDocument":{"documentId":"https://example.test/page","url":"https://example.test/page","title":"Live Content","chunks":[{"markdownChunk":{"text":"fixture page"}}],"summary":"Fetched live"},"resolvedUrl":"https://example.test/page","latencyMs":120,"contentPath":"/fixture/content.md"}`},
	{name: "failed read_url_content", tool: "read_url_content", nativeType: "CORTEX_STEP_TYPE_READ_URL_CONTENT", field: "readUrlContent", resultFile: true,
		failure:   "Failed to fetch document content at https://example.test/page",
		arguments: `{"Url":"https://example.test/page"}`,
		body:      `{"url":"https://example.test/page","resolvedUrl":"https://example.test/page"}`},
	{name: "ask_question", tool: "ask_question", nativeType: "CORTEX_STEP_TYPE_ASK_QUESTION", field: "askQuestion", resultFile: true,
		interactions: fixtureAskInteractions, arguments: fixtureAskArguments, body: fixtureAskBody},
}

func nativeToolByName(t *testing.T, name string) nativeToolCase {
	t.Helper()
	for _, c := range nativeToolCases {
		if c.name == name {
			return c
		}
	}
	t.Fatalf("no fixture %q", name)
	return nativeToolCase{}
}

func TestProjectionNativeToolsQualifiedShapes(t *testing.T) {
	for _, c := range nativeToolCases {
		t.Run(c.name, func(t *testing.T) {
			raw, presentation := nativeToolFixture(c)
			require1216(t, snapshot1216(t, raw), raw, presentation, Verified)
		})
	}
	// Every unwrapped tool has a fixture, and the list is exactly the eight tools.
	seen := map[string]bool{}
	for _, c := range nativeToolCases {
		seen[c.tool] = true
	}
	for _, tool := range []string{"list_dir", "find_by_name", "grep_search", "call_mcp_tool", "search_web", "finish", "read_url_content", "ask_question"} {
		if !seen[tool] {
			t.Errorf("no fixture for %s", tool)
		}
		delete(seen, tool)
	}
	if len(seen) != 0 {
		t.Errorf("fixtures for tools outside the list: %v", seen)
	}
	if len(presentedNativeTools) != 8 {
		t.Errorf("%d presented native tools, want 8", len(presentedNativeTools))
	}
}

type nativeToolMutation struct {
	name string
	// source edits apply to the persisted trajectory and the full RPC; the
	// presentation follows only where it carries the same text.
	source [][2]string
	// raw and presentation edits change only that response.
	raw, presentation [][2]string
	want              State
}

func applyNativeToolMutation(t *testing.T, c nativeToolCase, m nativeToolMutation) {
	t.Helper()
	raw, presentation := nativeToolFixture(c)
	for _, e := range m.source {
		raw = replace1216(t, raw, e[0], e[1])
		presentation = strings.ReplaceAll(presentation, e[0], e[1])
	}
	s := snapshot1216(t, raw)
	for _, e := range m.raw {
		raw = replace1216(t, raw, e[0], e[1])
	}
	for _, e := range m.presentation {
		presentation = replace1216(t, presentation, e[0], e[1])
	}
	require1216(t, s, raw, presentation, m.want)
}

// Each of the eight tools must fail on: an unknown extra argument (in the
// wrapper alone, or in the wrapper and the presented call), a changed argument
// value, an extra wrapper field, a new tool name, a changed or unequal status,
// and on a presentation or full RPC that differs from the source.
func TestProjectionNativeToolsChanges(t *testing.T) {
	const outerDone = `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE"`
	const outerError = `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR"`
	for _, c := range nativeToolCases {
		if c.failure != "" {
			continue
		}
		wrapperTool := `"toolAction":"Fixture action"`
		innerDone := `"type":"` + c.nativeType + `","status":"CORTEX_STEP_STATUS_DONE"`
		innerError := `"type":"` + c.nativeType + `","status":"CORTEX_STEP_STATUS_ERROR"`
		wrapperCall := `"name":"` + c.tool + `"}`
		nativeCall := `"name":"` + c.tool + `","argumentsJson"`
		mutations := []nativeToolMutation{
			{name: "unknown wrapper argument", source: [][2]string{{`"args":{`, `"args":{"Future":"new",`}}, want: Unknown},
			{name: "wrapper argument missing from presented call", source: [][2]string{{wrapperTool + `,`, ``}}, want: Unknown},
			{name: "presented argument missing from wrapper", source: [][2]string{{`,\"toolSummary\":\"Fixture summary\"`, ``}}, want: Unknown},
			{name: "changed wrapper action argument", source: [][2]string{{wrapperTool, `"toolAction":"Changed action"`}}, want: Unknown},
			{name: "presented action changed", presentation: [][2]string{{`\"toolAction\":\"Fixture action\"`, `\"toolAction\":\"Changed action\"`}}, want: Incomplete},
			{name: "raw action changed", raw: [][2]string{{wrapperTool, `"toolAction":"Changed action"`}}, want: Incomplete},
			{name: "unexpected wrapper field", source: [][2]string{{`"generic":{`, `"permissions":{},"generic":{`}}, want: Unknown},
			{name: "new tool name", source: [][2]string{{`"name":"` + c.tool + `"`, `"name":"` + c.tool + `_next"`}}, want: Unknown},
			{name: "wrapper tool name changed", source: [][2]string{{wrapperCall, `"name":"fixture_tool"}`}}, want: Unknown},
			{name: "presented tool name changed", source: [][2]string{{nativeCall, `"name":"fixture_tool","argumentsJson"`}}, want: Unknown},
			{name: "outer status changed", source: [][2]string{{outerDone, outerError}}, want: Unknown},
			{name: "native status changed", source: [][2]string{{innerDone, innerError}}, want: Unknown},
			{name: "outer canceled over inner done", source: [][2]string{{outerDone, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_CANCELED"`}}, want: Unknown},
			{name: "presented status changed", presentation: [][2]string{{innerDone, innerError}}, want: Incomplete},
			{name: "raw outer status changed", raw: [][2]string{{outerDone, outerError}}, want: Incomplete},
			{name: "raw result text changed", raw: [][2]string{{`"result":"Fixture result text"`, `"result":"Other result text"`}}, want: Incomplete},
			{name: "presented field changed", presentation: [][2]string{{`"` + c.field + `":`, `"` + c.field + `":{"unexpected":true},"previous":`}}, want: Incomplete},
		}
		if c.tool != "finish" { // finish takes whatever arguments its caller defined
			mutations = append(mutations,
				nativeToolMutation{name: "unknown argument in wrapper and presented call", source: [][2]string{
					{`"args":{`, `"args":{"Future":"new",`}, {`argumentsJson":"{`, `argumentsJson":"{\"Future\":\"new\",`}}, want: Unknown},
				nativeToolMutation{name: "unrelated known argument", source: [][2]string{
					{`"args":{`, `"args":{"CommandLine":"echo unsafe",`}, {`argumentsJson":"{`, `argumentsJson":"{\"CommandLine\":\"echo unsafe\",`}}, want: Unknown},
				nativeToolMutation{name: "task details on wrapper and presented step", source: [][2]string{
					{`"generic":{`, `"taskDetails":{"id":"native-tool-fixture/task-1"},"generic":{`}, {`"completedAt":"2026-01-01T00:00:01Z"},`, `"completedAt":"2026-01-01T00:00:01Z"},"taskDetails":{"id":"native-tool-fixture/task-1"},`}}, want: Unknown})
		}
		if c.resultFile {
			mutations = append(mutations,
				nativeToolMutation{name: "raw output file changed", raw: [][2]string{{`file:///fixture/output.txt`, `file:///fixture/other.txt`}}, want: Incomplete})
		}
		t.Run(c.name, func(t *testing.T) {
			for _, m := range mutations {
				t.Run(m.name, func(t *testing.T) { applyNativeToolMutation(t, c, m) })
			}
		})
	}
}

// Per-tool changes: a changed value in each tool's own arguments, and in its
// native result, must fail whether the change reaches the source or only one
// response.
func TestProjectionNativeToolsValues(t *testing.T) {
	for _, tc := range []struct {
		fixture, wrapperValue, changedWrapper, presentedValue, changedPresented, nativeValue, changedNative string
	}{
		{"list_dir", `"DirectoryPath":"/fixture/project"`, `"DirectoryPath":"/fixture/other"`, `\"DirectoryPath\":\"/fixture/project\"`, `\"DirectoryPath\":\"/fixture/other\"`, `"results":[{"name":"src"`, `"results":[{"name":"lib"`},
		{"find_by_name", `"Pattern":"*.go"`, `"Pattern":"*.py"`, `\"Pattern\":\"*.go\"`, `\"Pattern\":\"*.py\"`, `"totalResults":2`, `"totalResults":3`},
		{"find_by_name with every argument", `"MaxDepth":"3"`, `"MaxDepth":"4"`, `\"MaxDepth\":3`, `\"MaxDepth\":4`, `"pattern":"*"`, `"pattern":"**"`},
		{"grep_search", `"Query":"fixture"`, `"Query":"other"`, `\"Query\":\"fixture\"`, `\"Query\":\"other\"`, `"totalResults":1`, `"totalResults":2`},
		{"grep_search with every argument", `"IsRegex":"false"`, `"IsRegex":"true"`, `\"IsRegex\":false`, `\"IsRegex\":true`, `"query":"fixture"`, `"query":"other"`},
		{"call_mcp_tool", `"ToolName":"summarize"`, `"ToolName":"delete"`, `\"ToolName\":\"summarize\"`, `\"ToolName\":\"delete\"`, `"serverName":"fixture-server"`, `"serverName":"other-server"`},
		{"search_web", `"query":"fixture \"changelog\" OR notes"`, `"query":"other"`, `\"query\":\"fixture \\\"changelog\\\" OR notes\"`, `\"query\":\"other\"`, `"summary":"A fixture summary."`, `"summary":"Another summary."`},
		{"search_web with a domain", `"domain":"example.test"`, `"domain":"other.test"`, `\"domain\":\"example.test\"`, `\"domain\":\"other.test\"`, `"searchType":"SEARCH_WEB_TYPE_WEB"`, `"searchType":"SEARCH_WEB_TYPE_UNSPECIFIED"`},
		{"finish", `"sum":"16"`, `"sum":"17"`, `\"sum\":16,\"toolAction\"`, `\"sum\":17,\"toolAction\"`, `"outputString":"{\"marker\"`, `"outputString":"{\"mark\"`},
		{"read_url_content", `"Url":"https://example.test/page"`, `"Url":"https://example.test/other"`, `\"Url\":\"https://example.test/page\"`, `\"Url\":\"https://example.test/other\"`, `"latencyMs":120`, `"latencyMs":121`},
		{"ask_question", `"questions":"[{`, `"questions":"[{\"extra\":1},{`, `"questions\":[{`, `"questions\":[{\"extra\":1},{`, `"selectedOptionIds":["2"]}`, `"selectedOptionIds":["1"]}`},
	} {
		c := nativeToolByName(t, tc.fixture)
		t.Run(tc.fixture, func(t *testing.T) {
			for _, m := range []nativeToolMutation{
				{name: "changed wrapper argument", source: [][2]string{{tc.wrapperValue, tc.changedWrapper}}, want: Unknown},
				{name: "changed argument in the presented call only", source: [][2]string{{tc.presentedValue, tc.changedPresented}}, want: Unknown},
				{name: "presented argument changed", presentation: [][2]string{{tc.presentedValue, tc.changedPresented}}, want: Incomplete},
				{name: "raw argument changed", raw: [][2]string{{tc.wrapperValue, tc.changedWrapper}}, want: Incomplete},
				{name: "presented result changed", presentation: [][2]string{{tc.nativeValue, tc.changedNative}}, want: Incomplete},
				{name: "raw result changed", raw: [][2]string{{tc.nativeValue, tc.changedNative}}, want: Incomplete},
			} {
				t.Run(m.name, func(t *testing.T) { applyNativeToolMutation(t, c, m) })
			}
		})
	}
}

// Only the observed failures project. grep_search, call_mcp_tool, search_web and
// read_url_content failed with a native Step in retained sessions; list_dir and
// find_by_name failed only before they ran (qualifyResultless), and finish and
// ask_question never failed, so an outer error over their native Steps is
// refused. A failed call's outer short error must repeat the inner one.
func TestProjectionNativeToolsFailures(t *testing.T) {
	for _, name := range []string{"failed grep_search", "failed call_mcp_tool", "failed search_web", "failed read_url_content"} {
		c := nativeToolByName(t, name)
		t.Run(name, func(t *testing.T) {
			for _, m := range []nativeToolMutation{
				{name: "outer error differs from inner", source: [][2]string{{`","fullError"`, ` (retry)","fullError"`}}, want: Unknown},
				{name: "outer error without inner error", source: [][2]string{{`"error":{"shortError":"` + c.failure + `"},`, ``}}, want: Unknown},
				{name: "extra field in outer error", source: [][2]string{{`"error":{"shortError":"` + c.failure + `","fullError"`, `"error":{"errorId":"fixture","shortError":"` + c.failure + `","fullError"`}}, want: Unknown},
				{name: "outer error on a step that did not fail", source: [][2]string{{`"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR"`, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE"`}}, want: Unknown},
				{name: "presented error changed", presentation: [][2]string{{`"shortError":"` + c.failure + `"`, `"shortError":"other failure"`}}, want: Incomplete},
				{name: "presented error dropped", presentation: [][2]string{{`"error":{"shortError":"` + c.failure + `"},`, ``}}, want: Incomplete},
				{name: "raw outer full error changed", raw: [][2]string{{`"fullError":"fixture: `, `"fullError":"other: `}}, want: Incomplete},
			} {
				t.Run(m.name, func(t *testing.T) { applyNativeToolMutation(t, c, m) })
			}
		})
	}
	for _, name := range []string{"list_dir", "find_by_name", "finish", "ask_question"} {
		c := nativeToolByName(t, name)
		c.failure = "fixture failure"
		t.Run("failed "+name+" native step", func(t *testing.T) {
			raw, presentation := nativeToolFixture(c)
			require1216(t, snapshot1216(t, raw), raw, presentation, Unknown)
		})
	}
}

// ask_question wrappers are the only ones that may carry completedInteractions,
// and the wrapper's must equal the native Step's, which the presentation shows.
func TestProjectionNativeToolsInteractions(t *testing.T) {
	c := nativeToolByName(t, "ask_question")
	for _, m := range []nativeToolMutation{
		{name: "answer changed in source and presentation", source: [][2]string{{`"selectedOptionIds":["2"]}`, `"selectedOptionIds":["1"]}`}}, want: Verified},
		{name: "step index changed in source and presentation", source: [][2]string{{`"response":{"trajectoryId":"fixture-trajectory","stepIndex":4`, `"response":{"trajectoryId":"fixture-trajectory","stepIndex":5`}}, want: Verified},
		{name: "presented answer changed", presentation: [][2]string{{`"writeInResponse":"Mention the fixture"`, `"writeInResponse":"Mention another"`}}, want: Incomplete},
		{name: "presented interactions dropped", presentation: [][2]string{{`"completedInteractions":` + fixtureAskInteractions + `,`, ``}}, want: Incomplete},
		{name: "raw interactions changed", raw: [][2]string{{`"writeInResponse":"Mention the fixture"`, `"writeInResponse":"Mention another"`}}, want: Incomplete},
	} {
		t.Run(m.name, func(t *testing.T) {
			applyNativeToolMutation(t, c, m)
		})
	}
	raw, presentation := nativeToolFixture(c)
	other := strings.Replace(raw, `"completedInteractions":`+fixtureAskInteractions+`,"generic"`, `"completedInteractions":`+strings.Replace(fixtureAskInteractions, `"stepIndex":4`, `"stepIndex":5`, 1)+`,"generic"`, 1)
	for name, tc := range map[string]struct{ raw, presentation string }{
		"wrapper interactions differ from the native step's": {other, presentation},
		"wrapper without interactions":                       {strings.Replace(raw, `"completedInteractions":`+fixtureAskInteractions+`,"generic"`, `"generic"`, 1), presentation},
		"native step without interactions": {
			strings.ReplaceAll(strings.Replace(raw, `"completedInteractions":`+fixtureAskInteractions+`,"generic"`, `"completedInteractions":`+fixtureAskInteractions+`,"generic"`, 1), `,"completedInteractions":`+fixtureAskInteractions+`,"askQuestion"`, `,"askQuestion"`),
			strings.ReplaceAll(presentation, `"completedInteractions":`+fixtureAskInteractions+`,`, ``),
		},
	} {
		t.Run(name, func(t *testing.T) {
			if tc.raw == raw && tc.presentation == presentation {
				t.Fatal("fixture unchanged")
			}
			require1216(t, snapshot1216(t, tc.raw), tc.raw, tc.presentation, Unknown)
		})
	}
	// Another tool's wrapper may not carry interactions, even equal to its native Step's.
	list, listPresentation := nativeToolFixture(nativeToolByName(t, "list_dir"))
	inter := `"completedInteractions":` + fixtureAskInteractions + `,`
	withInteractions := strings.Replace(list, `"generic":{`, inter+`"generic":{`, 1)
	withInteractions = strings.Replace(withInteractions, `"listDirectory":`, inter+`"listDirectory":`, 1)
	t.Run("interactions on a list_dir wrapper", func(t *testing.T) {
		require1216(t, snapshot1216(t, withInteractions), withInteractions, strings.Replace(listPresentation, `"listDirectory":`, inter+`"listDirectory":`, 1), Unknown)
	})
	// An ask_question wrapper over another native type is not qualified.
	t.Run("ask_question over a list_dir step", func(t *testing.T) {
		raw := strings.ReplaceAll(list, `"name":"list_dir"`, `"name":"ask_question"`)
		raw = strings.Replace(raw, `"generic":{`, inter+`"generic":{`, 1)
		require1216(t, snapshot1216(t, raw), raw, strings.ReplaceAll(listPresentation, `"name":"list_dir"`, `"name":"ask_question"`), Unknown)
	})
}

// A JSON null argument is not an empty string, and a saved structured argument
// is exactly the presented value, with nothing after it (agy-reader#7256 and
// agy-reader#6f3d). Each case builds its own source, full RPC and presentation.
func TestProjectionNativeToolsArgumentComparison(t *testing.T) {
	finish := nativeToolByName(t, "finish")
	nullSum := [][2]string{{`\"sum\":16`, `\"sum\":null`}}
	for _, m := range []nativeToolMutation{
		// A null result saved as the text null is the same call.
		{name: "finish null output saved as the text null", source: append(append([][2]string{}, nullSum...), [2]string{`"sum":"16"`, `"sum":"null"`}), want: Verified},
		// A null result may not stand for an empty or any other saved text.
		{name: "finish null output saved as an empty string", source: append(append([][2]string{}, nullSum...), [2]string{`"sum":"16"`, `"sum":""`}), want: Unknown},
		{name: "finish null output saved as another text", source: append(append([][2]string{}, nullSum...), [2]string{`"sum":"16"`, `"sum":"0"`}), want: Unknown},
	} {
		t.Run(m.name, func(t *testing.T) { applyNativeToolMutation(t, finish, m) })
	}
	// An empty presented string still equals the empty saved string, and no other.
	emptyFinish := finish
	emptyFinish.arguments = `{"marker":"","sum":16}`
	emptyFinish.body = `{"output":{"arguments_json":"{\"marker\":\"\",\"sum\":16}"},"outputString":"{\"marker\":\"\",\"sum\":16}"}`
	t.Run("finish empty string saved as an empty string", func(t *testing.T) {
		applyNativeToolMutation(t, emptyFinish, nativeToolMutation{name: "same", want: Verified})
	})
	t.Run("finish empty string saved as the text null", func(t *testing.T) {
		applyNativeToolMutation(t, emptyFinish, nativeToolMutation{name: "null text", source: [][2]string{{`"marker":""`, `"marker":"null"`}}, want: Unknown})
	})

	mcp := nativeToolByName(t, "call_mcp_tool")
	saved := `"Arguments":"{\"detail\":\"full\",\"limit\":5}"`
	for _, m := range []nativeToolMutation{
		{name: "structured argument with a trailing bracket", source: [][2]string{{saved, `"Arguments":"{\"detail\":\"full\",\"limit\":5}]"`}}, want: Unknown},
		{name: "structured argument with a trailing brace", source: [][2]string{{saved, `"Arguments":"{\"detail\":\"full\",\"limit\":5}}"`}}, want: Unknown},
		{name: "structured argument with a second value", source: [][2]string{{saved, `"Arguments":"{\"detail\":\"full\",\"limit\":5} {}"`}}, want: Unknown},
		{name: "structured argument repeats a key with conflicting values", source: [][2]string{{saved, `"Arguments":"{\"detail\":\"full\",\"limit\":999,\"limit\":5}"`}}, want: Unknown},
		{name: "structured argument repeats a key with the same value", source: [][2]string{{saved, `"Arguments":"{\"detail\":\"full\",\"limit\":5,\"limit\":5}"`}}, want: Unknown},
		{name: "structured argument repeats a nested key", source: [][2]string{{saved, `"Arguments":"{\"detail\":{\"a\":1,\"a\":2},\"limit\":5}"`}, {`\"detail\":\"full\"`, `\"detail\":{\"a\":2}`}}, want: Unknown},
		{name: "structured argument with nested keys that are unique", source: [][2]string{{`\"detail\":\"full\"`, `\"detail\":{\"a\":2}`}}, want: Verified},
		{name: "structured argument repeats a key inside an array", source: [][2]string{{saved, `"Arguments":"{\"detail\":[{\"a\":1,\"a\":2}],\"limit\":5}"`}, {`\"detail\":\"full\"`, `\"detail\":[{\"a\":2}]`}}, want: Unknown},
		{name: "structured argument with a number beyond float64", source: [][2]string{{`\"limit\":5}`, `\"limit\":1e400}`}}, want: Verified},
		{name: "structured argument reformatted", source: [][2]string{{saved, `"Arguments":"{ \"limit\": 5, \"detail\": \"full\" }"`}}, want: Verified},
	} {
		t.Run(m.name, func(t *testing.T) { applyNativeToolMutation(t, mcp, m) })
	}
	t.Run("structured argument nested far deeper than JSON allows", func(t *testing.T) {
		deep := strings.Repeat("[", 200000) + "0" + strings.Repeat("]", 200000)
		applyNativeToolMutation(t, mcp, nativeToolMutation{name: "deep", source: [][2]string{{saved, `"Arguments":"` + deep + `"`}}, want: Unknown})
	})
}

// The wrapper's completed interactions are dropped only for a native ASK_QUESTION
// step, so the wrapper's tool name must match the native type (agy-reader#txff).
func TestProjectionNativeToolsAskQuestionNameMatchesType(t *testing.T) {
	inter := `"completedInteractions":` + fixtureAskInteractions + `,`
	// The outer wrapper alone is relabelled and given interactions; its embedded
	// step is still the VIEW_FILE step, which has none.
	raw := strings.Replace(currentViewRaw, `"name":"view_file"`, `"name":"ask_question"`, 1)
	raw = strings.Replace(raw, `"generic":{`, inter+`"generic":{`, 1)
	if raw == currentViewRaw {
		t.Fatal("fixture unchanged")
	}
	require1216(t, snapshot1216(t, raw), raw, currentViewPresentation, Unknown)
	// Without the interactions the relabelled wrapper is no better.
	bare := strings.Replace(currentViewRaw, `"name":"view_file"`, `"name":"ask_question"`, 1)
	require1216(t, snapshot1216(t, bare), bare, currentViewPresentation, Unknown)
	// The unlabelled original still verifies.
	require1216(t, snapshot1216(t, currentViewRaw), currentViewRaw, currentViewPresentation, Verified)
}

// Malformed Unicode in a tool argument decodes to U+FFFD, so it must not let a
// saved argument match a different presented one (review finding rv-ta2k.1).
func TestProjectionNativeToolsMalformedUnicodeArguments(t *testing.T) {
	// The saved marker is the text "\ud801", quotation marks and backslash
	// included; the presented call's marker is the malformed JSON string
	// "\ud800". Both decode to U+FFFD as JSON.
	t.Run("finish string argument", func(t *testing.T) {
		raw, presentation := nativeToolFixture(nativeToolByName(t, "finish"))
		raw = replace1216(t, raw, `"marker":"FIXTURE_MARKER"`, `"marker":"\"\\ud801\""`)
		raw = replace1216(t, raw, `\"marker\":\"FIXTURE_MARKER\"`, `\"marker\":\"\\ud800\"`)
		presentation = replace1216(t, presentation, `\"marker\":\"FIXTURE_MARKER\"`, `\"marker\":\"\\ud800\"`)
		require1216(t, snapshot1216(t, raw), raw, presentation, Unknown)
	})

	// The presented call carries a real U+FFFD, which verifies; a saved copy
	// that spells it as a lone surrogate escape does not.
	mcp := nativeToolByName(t, "call_mcp_tool")
	mcp.arguments = strings.ReplaceAll(mcp.arguments, `"detail":"full"`, "\"detail\":\"\ufffd\"")
	mcp.body = strings.ReplaceAll(mcp.body, `\"detail\":\"full\"`, "\\\"detail\\\":\\\"\ufffd\\\"")
	t.Run("call_mcp_tool replacement character", func(t *testing.T) {
		applyNativeToolMutation(t, mcp, nativeToolMutation{name: "same", want: Verified})
	})
	t.Run("call_mcp_tool structured argument saved with a lone surrogate", func(t *testing.T) {
		applyNativeToolMutation(t, mcp, nativeToolMutation{name: "lone surrogate", source: [][2]string{
			{"\"Arguments\":\"{\\\"detail\\\":\\\"\ufffd\\\",\\\"limit\\\":5}\"", `"Arguments":"{\"detail\":\"\\ud800\",\"limit\":5}"`},
		}, want: Unknown})
	})
}

func TestJSONArgumentEqualMalformedUnicode(t *testing.T) {
	for _, tc := range []struct {
		raw, arg string
		want     bool
	}{
		{`"plain"`, `plain`, true},
		{`"\u00e9"`, "\u00e9", true},
		{`"\ud800"`, "\ufffd", false},
		{`"\ud800"`, `"\ud801"`, false},
		{`{"a":"\ud83d\ude00"}`, `{"a":"😀"}`, true},
		{`{"a":"\ufffd"}`, `{"a":"\ud800"}`, false},
		{`{"a":"\ud800"}`, `{"a":"\ufffd"}`, false},
		{`["\ud800"]`, `["\ud801"]`, false},
		{`{"\ud800":1}`, `{"\ud801":1}`, false},
	} {
		if got := jsonArgumentEqual([]byte(tc.raw), tc.arg); got != tc.want {
			t.Errorf("jsonArgumentEqual(%s, %s) = %v, want %v", tc.raw, tc.arg, got, tc.want)
		}
	}
}
