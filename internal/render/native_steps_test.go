package render_test

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/render"
)

// The fixtures below follow the agy 1.2.16 RPC shape of a built-in
// image-generator child and of its parent's invocation step, with sanitized
// ids, paths, and prompts.

const imageChildJSON = `{
  "cascadeId": "child-0000",
  "steps": [
    {"type": "CORTEX_STEP_TYPE_GENERATE_IMAGE", "status": "CORTEX_STEP_STATUS_DONE",
     "metadata": {"createdAt": "2026-10-03T04:20:54Z", "toolCall": {"name": "generate_image"}},
     "generateImage": {
       "prompt": "A friendly blue owl on a plain cream background, no text.",
       "imageName": "blue_owl",
       "aspectRatio": "1:1",
       "modelName": "gemini-3.1-flash-image",
       "generatedMedia": {"mimeType": "image/jpeg", "inlineData": "", "uri": "file:///fixture/brain/parent-0000/blue_owl_1791.jpg"}
     }},
    {"type": "CORTEX_STEP_TYPE_PLANNER_RESPONSE", "status": "CORTEX_STEP_STATUS_DONE",
     "metadata": {"createdAt": "2026-10-03T04:21:07Z"},
     "plannerResponse": {"response": "Saved blue_owl_1791.jpg with gemini-3.1-flash-image at 1:1 (image/jpeg)."}}
  ]
}`

const invokeParentJSON = `{
  "cascadeId": "parent-0000",
  "steps": [
    {"type": "CORTEX_STEP_TYPE_INVOKE_SUBAGENT", "status": "CORTEX_STEP_STATUS_DONE",
     "metadata": {"createdAt": "2026-10-03T04:20:40Z", "toolCall": {"name": "invoke_subagent"}},
     "invokeSubagent": {
       "subagents": [{
         "typeName": "image-generator",
         "role": "Image Generator",
         "initialPrompt": "Generate one image of a blue owl and reply with the saved path.",
         "inherit": true,
         "model": "MODEL_PLACEHOLDER_M322",
         "modelTier": "MODEL_TIER_INHERIT"
       }],
       "results": [{
         "conversationId": "child-0000",
         "logAbsoluteUri": "file:///fixture/brain/child-0000/.system_generated/logs/transcript.jsonl"
       }]
     }},
    {"type": "CORTEX_STEP_TYPE_PLANNER_RESPONSE", "status": "CORTEX_STEP_STATUS_DONE",
     "metadata": {"createdAt": "2026-10-03T04:21:20Z"},
     "plannerResponse": {"response": "The image-generator (Image Generator) child child-0000 finished; see transcript.jsonl."}}
  ]
}`

func decodeTrajectory(t *testing.T, raw string) *daemon.Trajectory {
	t.Helper()
	var traj daemon.Trajectory
	if err := json.Unmarshal([]byte(raw), &traj); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return &traj
}

func renderString(t *testing.T, traj *daemon.Trajectory) string {
	t.Helper()
	var buf bytes.Buffer
	if _, err := render.Markdown(&buf, traj, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	return buf.String()
}

// stepSection returns only the Markdown rendered for step num (1-based): from
// its "## Step N:" heading up to the step separator. Assertions on it cannot
// be satisfied by text from any other step, such as a later assistant summary.
func stepSection(t *testing.T, md string, num int, short string) string {
	t.Helper()
	head := "## Step " + strconv.Itoa(num) + ": " + short + " "
	start := strings.Index(md, head)
	if start < 0 {
		t.Fatalf("no %q heading in:\n%s", head, md)
	}
	rest := md[start:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		t.Fatalf("step %d has no separator:\n%s", num, rest)
	}
	return rest[:end]
}

func assertContainsAll(t *testing.T, section string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(section, want) {
			t.Errorf("step section missing %q:\n%s", want, section)
		}
	}
}

func TestGenerateImageStepRendersNativeDetails(t *testing.T) {
	md := renderString(t, decodeTrajectory(t, imageChildJSON))
	sec := stepSection(t, md, 1, "GENERATE_IMAGE")
	assertContainsAll(t, sec,
		"### Image Generation\n",
		"- **Image Name:** `blue_owl`\n",
		"- **Model:** `gemini-3.1-flash-image`\n",
		"- **Aspect Ratio:** `1:1`\n",
		"- **Media Type:** `image/jpeg`\n",
		"- **Generated Image:** [blue_owl_1791.jpg](file:///fixture/brain/parent-0000/blue_owl_1791.jpg)\n",
		"**Prompt:**\n```\nA friendly blue owl on a plain cream background, no text.\n```\n",
	)
	for _, unwanted := range []string{"No additional details", "Inline Data"} {
		if strings.Contains(sec, unwanted) {
			t.Errorf("step section unexpectedly contains %q:\n%s", unwanted, sec)
		}
	}
}

// The step must explain itself even with no later assistant message at all.
func TestGenerateImageStepAloneStillRendersDetails(t *testing.T) {
	traj := decodeTrajectory(t, imageChildJSON)
	traj.Steps = traj.Steps[:1]
	sec := stepSection(t, renderString(t, traj), 1, "GENERATE_IMAGE")
	assertContainsAll(t, sec, "`gemini-3.1-flash-image`", "`1:1`", "`image/jpeg`",
		"(file:///fixture/brain/parent-0000/blue_owl_1791.jpg)", "A friendly blue owl")
}

func TestGenerateImageInlineDataSummarisedNotDumped(t *testing.T) {
	data := strings.Repeat("QUJD", 300) // 1200 base64 characters
	traj := &daemon.Trajectory{CascadeID: "c", Steps: []daemon.Step{{
		Type: "CORTEX_STEP_TYPE_GENERATE_IMAGE",
		GenerateImage: json.RawMessage(`{"prompt":"p","generatedMedia":{"mimeType":"image/png","inlineData":"` + data + `"},` +
			`"generatedImage":{"mimeType":"image/png","caption":"an owl","base64Data":"` + data + `"}}`),
	}}}
	sec := stepSection(t, renderString(t, traj), 1, "GENERATE_IMAGE")
	assertContainsAll(t, sec,
		"- **Inline Data:** embedded, not shown (1200 base64 characters)\n",
		"- **Caption:** an owl\n",
	)
	if strings.Contains(sec, "QUJD") {
		t.Errorf("inline image bytes leaked into Markdown:\n%s", sec)
	}
	if n := strings.Count(sec, "**Media Type:**"); n != 1 {
		t.Errorf("duplicate media type lines: %d\n%s", n, sec)
	}
}

func TestGenerateImageWithoutPayloadKeepsFallback(t *testing.T) {
	for _, payload := range []json.RawMessage{nil, json.RawMessage(`{}`)} {
		traj := &daemon.Trajectory{Steps: []daemon.Step{{Type: "CORTEX_STEP_TYPE_GENERATE_IMAGE", GenerateImage: payload}}}
		sec := stepSection(t, renderString(t, traj), 1, "GENERATE_IMAGE")
		assertContainsAll(t, sec, "*(No additional details stored for this step type)*")
	}
}

func TestGenerateImageUndecodablePayloadShownRaw(t *testing.T) {
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type:          "CORTEX_STEP_TYPE_GENERATE_IMAGE",
		GenerateImage: json.RawMessage(`{"prompt":["unexpected","shape"]}`),
	}}}
	sec := stepSection(t, renderString(t, traj), 1, "GENERATE_IMAGE")
	assertContainsAll(t, sec, "```json\n", `"unexpected"`)
}

func TestInvokeSubagentStepRendersChildIdentity(t *testing.T) {
	md := renderString(t, decodeTrajectory(t, invokeParentJSON))
	sec := stepSection(t, md, 1, "INVOKE_SUBAGENT")
	assertContainsAll(t, sec,
		"### Subagent Invoked\n",
		"recorded in its own separate trajectory",
		"**Requested subagent:**\n",
		"- **Agent Type:** `image-generator`\n",
		"- **Role:** Image Generator\n",
		"- **Model:** `MODEL_PLACEHOLDER_M322` (tier `MODEL_TIER_INHERIT`)\n",
		"- **Inherit:** yes\n",
		"**Initial Prompt:**\n```\nGenerate one image of a blue owl and reply with the saved path.\n```\n",
		"**Child conversation:**\n",
		"- **Conversation ID:** `child-0000`\n",
		"- **Result Log:** [transcript.jsonl](file:///fixture/brain/child-0000/.system_generated/logs/transcript.jsonl)\n",
	)
}

func TestInvokeSubagentStepAloneStillRendersIdentity(t *testing.T) {
	traj := decodeTrajectory(t, invokeParentJSON)
	traj.Steps = traj.Steps[:1]
	sec := stepSection(t, renderString(t, traj), 1, "INVOKE_SUBAGENT")
	assertContainsAll(t, sec, "`child-0000`", "`image-generator`", "Image Generator",
		"(file:///fixture/brain/child-0000/.system_generated/logs/transcript.jsonl)")
}

// With a resolved child nested below, the identity block still renders and the
// marker points at the nested transcript.
func TestInvokeSubagentIdentityWithNestedChild(t *testing.T) {
	parent := decodeTrajectory(t, invokeParentJSON)
	child := decodeTrajectory(t, imageChildJSON)
	var buf bytes.Buffer
	r := mapResolver{"parent-0000": {child}}
	if _, err := render.MarkdownTree(&buf, parent, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), r); err != nil {
		t.Fatalf("MarkdownTree: %v", err)
	}
	md := buf.String()
	invoke := md[strings.Index(md, "## Step 1: INVOKE_SUBAGENT"):strings.Index(md, "<details>")]
	assertContainsAll(t, invoke, "transcript is nested below", "- **Conversation ID:** `child-0000`",
		"- **Agent Type:** `image-generator`", "- **Result Log:** [transcript.jsonl]")
	if !strings.Contains(md, "- **Model:** `gemini-3.1-flash-image`") {
		t.Errorf("nested child's image details missing:\n%s", md)
	}
}

func TestInvokeSubagentMultipleAndUnknownEnum(t *testing.T) {
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type: "CORTEX_STEP_TYPE_INVOKE_SUBAGENT",
		InvokeSubagent: json.RawMessage(`{"subagents":[{"typeName":"a","model":1234},{"typeName":"b","modelTier":"MODEL_TIER_FAST"}],` +
			`"results":[{"conversationId":"c1"},{"conversationId":"c2","workspaceUris":["file:///fixture/ws"]}]}`),
	}}}
	sec := stepSection(t, renderString(t, traj), 1, "INVOKE_SUBAGENT")
	assertContainsAll(t, sec,
		"**Requested subagent 1 of 2:**\n- **Agent Type:** `a`\n- **Model:** `1234`\n",
		"**Requested subagent 2 of 2:**\n- **Agent Type:** `b`\n- **Model:** tier `MODEL_TIER_FAST`\n",
		"**Child conversation 1 of 2:**\n- **Conversation ID:** `c1`\n",
		"**Child conversation 2 of 2:**\n- **Conversation ID:** `c2`\n- **Workspace:** [ws](file:///fixture/ws)\n",
	)
}

// Pre-1.2.16 invocation steps carry no payload; their output is unchanged.
func TestInvokeSubagentWithoutPayloadUnchanged(t *testing.T) {
	traj := &daemon.Trajectory{Steps: []daemon.Step{{Type: "CORTEX_STEP_TYPE_INVOKE_SUBAGENT"}}}
	sec := stepSection(t, renderString(t, traj), 1, "INVOKE_SUBAGENT")
	want := "### Subagent Invoked\n*The agent delegated work to a subagent at this step. " +
		"The subagent's activity is recorded in its own separate trajectory.*\n"
	if !strings.HasSuffix(sec, want) {
		t.Errorf("legacy invocation output changed:\n%s", sec)
	}
}

func TestInlineCodeEscapesBackticks(t *testing.T) {
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type:          "CORTEX_STEP_TYPE_GENERATE_IMAGE",
		GenerateImage: json.RawMessage("{\"imageName\":\"a`b\",\"modelName\":\"line1\\nline2\"}"),
	}}}
	sec := stepSection(t, renderString(t, traj), 1, "GENERATE_IMAGE")
	assertContainsAll(t, sec, "- **Image Name:** ``a`b``\n", "- **Model:** `line1 line2`\n")
}

// assertNoneOf fails when section contains any of unwanted.
func assertNoneOf(t *testing.T, section string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(section, u) {
			t.Errorf("step section unexpectedly contains %q:\n%s", u, section)
		}
	}
}

// A payload the typed view cannot decode is shown as JSON, but inline image
// bytes in it are still summarised by length, never printed.
func TestGenerateImageUndecodablePayloadRedactsInlineBytes(t *testing.T) {
	const raw = `{"steps":[{"type":"CORTEX_STEP_TYPE_GENERATE_IMAGE","generateImage":{"prompt":["unexpected","shape"],"generatedMedia":{"mimeType":"image/png","inlineData":"QUJDQUJD"},"generatedImage":{"base64Data":"QUJDQUJD"}}}]}`
	sec := stepSection(t, renderString(t, decodeTrajectory(t, raw)), 1, "GENERATE_IMAGE")
	assertNoneOf(t, sec, "QUJDQUJD")
	assertContainsAll(t, sec,
		"```json\n",
		`"unexpected"`,
		`"shape"`,
		`"mimeType": "image/png"`,
		`"inlineData": "embedded, not shown (8 base64 characters)"`,
		`"base64Data": "embedded, not shown (8 base64 characters)"`,
	)
}

// Redaction applies at any depth, to other spellings of the same fields, and
// to byte payloads that are not strings, while other values stay visible.
func TestGenerateImageRawFallbackRedactsNestedInlineBytes(t *testing.T) {
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type: "CORTEX_STEP_TYPE_GENERATE_IMAGE",
		GenerateImage: json.RawMessage(`{"prompt":{"text":"a red fox"},` +
			`"variants":[{"media":{"uri":"file:///fixture/fox.png","inline_data":"QUJDREVG"}},` +
			`{"thumb":{"imageBase64":"R0hJSktM","note":"small"}}],` +
			`"generatedMedia":{"inlineData":[77,78,79]},"generatedImage":{"base64Data":""}}`),
	}}}
	sec := stepSection(t, renderString(t, traj), 1, "GENERATE_IMAGE")
	assertNoneOf(t, sec, "QUJDREVG", "R0hJSktM", "77", "78", "79")
	assertContainsAll(t, sec,
		`"text": "a red fox"`,
		`"uri": "file:///fixture/fox.png"`,
		`"note": "small"`,
		`"inline_data": "embedded, not shown (8 base64 characters)"`,
		`"imageBase64": "embedded, not shown (8 base64 characters)"`,
		`"inlineData": "embedded, not shown (10 characters of JSON)"`,
		`"base64Data": ""`,
	)
}

// The INVOKE_SUBAGENT fallback goes through the same redaction.
func TestInvokeSubagentUndecodablePayloadRedactsInlineBytes(t *testing.T) {
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type: "CORTEX_STEP_TYPE_INVOKE_SUBAGENT",
		InvokeSubagent: json.RawMessage(`{"subagents":"unexpected-shape",` +
			`"results":[{"conversationId":"c1","attachments":[{"media":{"mimeType":"image/png","inlineData":"QUJDQUJD"}}]}]}`),
	}}}
	sec := stepSection(t, renderString(t, traj), 1, "INVOKE_SUBAGENT")
	assertNoneOf(t, sec, "QUJDQUJD")
	assertContainsAll(t, sec,
		"### Subagent Invoked\n",
		"```json\n",
		`"subagents": "unexpected-shape"`,
		`"conversationId": "c1"`,
		`"mimeType": "image/png"`,
		`"inlineData": "embedded, not shown (8 base64 characters)"`,
	)
}

// A payload that is not JSON at all cannot be redacted field by field, so it
// is summarised by length instead of printed.
func TestRawFallbackDoesNotPrintNonJSONPayload(t *testing.T) {
	bad := json.RawMessage(`{"inlineData":"QUJDQUJD"`)
	for _, s := range []daemon.Step{
		{Type: "CORTEX_STEP_TYPE_GENERATE_IMAGE", GenerateImage: bad},
		{Type: "CORTEX_STEP_TYPE_INVOKE_SUBAGENT", InvokeSubagent: bad},
	} {
		short := strings.TrimPrefix(s.Type, "CORTEX_STEP_TYPE_")
		sec := stepSection(t, renderString(t, &daemon.Trajectory{Steps: []daemon.Step{s}}), 1, short)
		assertNoneOf(t, sec, "QUJDQUJD")
		assertContainsAll(t, sec, "*(Stored payload is not valid JSON; "+strconv.Itoa(len(bad))+" bytes not shown)*")
	}
}

// A valid payload whose numbers do not fit a float64 is still shown, with each
// number exactly as written, instead of being mistaken for invalid JSON.
func TestRawFallbackKeepsOutOfRangeNumbers(t *testing.T) {
	const raw = `{"steps":[{"type":"CORTEX_STEP_TYPE_GENERATE_IMAGE","generateImage":{"prompt":[1e400],"imageName":"chart"}}]}`
	sec := stepSection(t, renderString(t, decodeTrajectory(t, raw)), 1, "GENERATE_IMAGE")
	assertNoneOf(t, sec, "not valid JSON", "not shown)*")
	assertContainsAll(t, sec,
		"```json\n",
		`"imageName": "chart"`,
		"\"prompt\": [\n    1e400\n  ]",
	)
}

// Ordinary numbers print as before, and redaction still applies at every depth
// next to numbers that do not fit a float64.
func TestRawFallbackNumbersAndRedactionTogether(t *testing.T) {
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type: "CORTEX_STEP_TYPE_GENERATE_IMAGE",
		GenerateImage: json.RawMessage(`{"prompt":[3,-2,0.5,1234],"seed":123456789012345678901234567890,` +
			`"variants":[{"scale":1e400,"media":{"inlineData":"QUJDQUJD"}}]}`),
	}}}
	sec := stepSection(t, renderString(t, traj), 1, "GENERATE_IMAGE")
	assertNoneOf(t, sec, "QUJDQUJD", "not valid JSON")
	assertContainsAll(t, sec,
		"\"prompt\": [\n    3,\n    -2,\n    0.5,\n    1234\n  ]",
		`"seed": 123456789012345678901234567890`,
		`"scale": 1e400`,
		`"inlineData": "embedded, not shown (8 base64 characters)"`,
	)
}

// JSON followed by anything but whitespace is not one JSON value; it is not
// printed, so bytes in the trailing part cannot leak.
func TestRawFallbackRejectsTrailingData(t *testing.T) {
	for _, bad := range []string{
		`{"imageName":"x"} {"inlineData":"QUJDQUJD"}`,
		`{"imageName":"x"} QUJDQUJD`,
	} {
		traj := &daemon.Trajectory{Steps: []daemon.Step{{
			Type: "CORTEX_STEP_TYPE_GENERATE_IMAGE", GenerateImage: json.RawMessage(bad),
		}}}
		sec := stepSection(t, renderString(t, traj), 1, "GENERATE_IMAGE")
		assertNoneOf(t, sec, "QUJDQUJD")
		assertContainsAll(t, sec, "*(Stored payload is not valid JSON; "+strconv.Itoa(len(bad))+" bytes not shown)*")
	}
}

func TestGenerateImageDataURISummarisedNotDumped(t *testing.T) {
	uri := "data:image/webp;base64," + strings.Repeat("QUJD", 300)
	traj := &daemon.Trajectory{CascadeID: "c", Steps: []daemon.Step{{
		Type: "CORTEX_STEP_TYPE_GENERATE_IMAGE",
		GenerateImage: json.RawMessage(`{"prompt":"p","generatedMedia":{"mimeType":"image/webp","uri":"` + uri + `"},` +
			`"generatedImage":{"uri":"` + uri + `x"}}`),
	}}}
	sec := stepSection(t, renderString(t, traj), 1, "GENERATE_IMAGE")
	assertNoneOf(t, sec, "QUJD")
	assertContainsAll(t, sec,
		"- **Generated Image:** data URI (`image/webp`), embedded, not shown (1223 characters)\n",
		"- **Generated Image:** data URI (`image/webp`), embedded, not shown (1224 characters)\n",
	)
}

// The raw JSON fallback redacts a data: URI in a uri field and leaves file
// and http links alone.
func TestGenerateImageRawFallbackSummarisesDataURI(t *testing.T) {
	traj := &daemon.Trajectory{Steps: []daemon.Step{{
		Type: "CORTEX_STEP_TYPE_GENERATE_IMAGE",
		GenerateImage: json.RawMessage(`{"prompt":["odd","shape"],` +
			`"generatedMedia":{"uri":"data:image/png;base64,QUJDQUJD"},` +
			`"other":[{"image_url":"data:image/gif;base64,R0lGODlh"},{"uri":"file:///fixture/a.png"},{"uri":"https://example.com/a.png"}]}`),
	}}}
	sec := stepSection(t, renderString(t, traj), 1, "GENERATE_IMAGE")
	assertNoneOf(t, sec, "QUJDQUJD", "R0lGODlh")
	assertContainsAll(t, sec,
		`"uri": "data URI (`+"`image/png`"+`), embedded, not shown (30 characters)"`,
		`"image_url": "data URI (`+"`image/gif`"+`), embedded, not shown (30 characters)"`,
		`"uri": "file:///fixture/a.png"`,
		`"uri": "https://example.com/a.png"`,
	)
}

// The ASK_QUESTION fixture follows the ordinary RPC step shape of agy 1.2.17
// (askQuestion.questions[] with question, options[].text and either
// selectedOptionIds or writeInResponse), with the questions and answers of the
// audit's report alias conversation-168 step 12. It was written by hand from that
// shape, not produced by running the reader on the 1.2.17 database, which the
// reader still refuses (agy-reader#8mnx).
const askQuestionJSON = `{
  "cascadeId": "ask-0000",
  "steps": [
    {"type": "CORTEX_STEP_TYPE_ASK_QUESTION", "status": "CORTEX_STEP_STATUS_DONE",
     "metadata": {"createdAt": "2026-10-06T05:38:58Z", "toolCall": {"name": "ask_question"}},
     "askQuestion": {"questions": [
       {"question": "Pick a color",
        "options": [{"id": "1", "text": "Red"}, {"id": "2", "text": "Green"}, {"id": "3", "text": "Blue"}],
        "selectedOptionIds": ["2"]},
       {"question": "Pick a size",
        "options": [{"id": "1", "text": "Small"}, {"id": "2", "text": "Large"}],
        "writeInResponse": "Medium-ish"}
     ]}}
  ]
}`

func TestAskQuestionStepRendersQuestionsOptionsAndAnswers(t *testing.T) {
	sec := stepSection(t, renderString(t, decodeTrajectory(t, askQuestionJSON)), 1, "ASK_QUESTION")
	t.Logf("rendered step:\n%s", sec)
	assertContainsAll(t, sec,
		"### Questions Asked\n",
		"**Question 1:** Pick a color\n- Red\n- Green (selected)\n- Blue\n\n**Answer:** Green\n",
		"**Question 2:** Pick a size\n- Small\n- Large\n\n**Answer (written in):** Medium-ish\n",
	)
	if strings.Contains(sec, "No additional details") {
		t.Errorf("ask_question step still renders the placeholder:\n%s", sec)
	}
}

func TestAskQuestionWrittenInAnswerKeepsItsLayout(t *testing.T) {
	traj := decodeTrajectory(t, `{"steps":[{"type":"CORTEX_STEP_TYPE_ASK_QUESTION","askQuestion":{"questions":[
	  {"question":"Notes?","options":[],"writeInResponse":"line one\n    indented  two\n\nthird"}]}}]}`)
	sec := stepSection(t, renderString(t, traj), 1, "ASK_QUESTION")
	t.Logf("rendered step:\n%s", sec)
	assertContainsAll(t, sec, "**Answer (written in):**\n```\nline one\n    indented  two\n\nthird\n```\n")
}

func TestAskQuestionUnansweredAndEmpty(t *testing.T) {
	traj := decodeTrajectory(t, `{"steps":[
	  {"type":"CORTEX_STEP_TYPE_ASK_QUESTION","askQuestion":{"questions":[{"question":"Go on?","options":[{"id":"1","text":"Yes"}]}]}},
	  {"type":"CORTEX_STEP_TYPE_ASK_QUESTION"}]}`)
	md := renderString(t, traj)
	assertContainsAll(t, stepSection(t, md, 1, "ASK_QUESTION"), "**Answer:** *(none stored)*")
	assertContainsAll(t, stepSection(t, md, 2, "ASK_QUESTION"), "*(No additional details stored for this step type)*")
}

func TestAskQuestionSkipped(t *testing.T) {
	traj := decodeTrajectory(t, `{"steps":[{"type":"CORTEX_STEP_TYPE_ASK_QUESTION","askQuestion":{"questions":[{"question":"Go on?","options":[{"id":"1","text":"Yes"},{"id":"2","text":"No"}],"skipped":true}]}}]}`)
	sec := stepSection(t, renderString(t, traj), 1, "ASK_QUESTION")
	assertContainsAll(t, sec, "**Answer:** *(skipped)*\n")
}
