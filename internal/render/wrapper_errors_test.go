package render_test

import "testing"

// These steps follow the default-RPC presentations of three failed generic
// wrapper shapes that the completeness projection qualifies (agy-reader#dgzv):
// a native Step whose inner error repeats the dropped outer error, a resultless
// wrapper presented unchanged with its outer error, and a failed generate_image
// call presented as a native image Step. Values are synthetic.
const failedWrappersJSON = `{"cascadeId":"wrapper-error-fixture","steps":[
  {"type":"CORTEX_STEP_TYPE_VIEW_FILE","status":"CORTEX_STEP_STATUS_ERROR",
   "metadata":{"createdAt":"2026-01-01T00:00:01Z","toolCall":{"name":"view_file"}},
   "error":{"shortError":"ContentOffset 9000 exceeds line range size 1200"},
   "viewFile":{"absolutePathUri":"file:///fixture/notes.md","startLine":19,"endLine":59,"contentOffset":9000,"lineRangeBytes":1200}},
  {"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR",
   "metadata":{"createdAt":"2026-01-01T00:00:02Z","toolCall":{"name":"write_to_file"}},
   "error":{"shortError":"invalid tool call: /fixture/out/notes.md is not a valid artifact path","fullError":"declaring permissions: invalid tool call: /fixture/out/notes.md is not a valid artifact path"},
   "generic":{"args":{"TargetFile":"/fixture/out/notes.md","toolAction":"Writing notes","toolSummary":"Notes"}}},
  {"type":"CORTEX_STEP_TYPE_GENERATE_IMAGE","status":"CORTEX_STEP_STATUS_ERROR",
   "metadata":{"createdAt":"2026-01-01T00:00:03Z","toolCall":{"name":"generate_image"}},
   "error":{"shortError":"failed to generate content: 429 Too Many Requests"},
   "generateImage":{"prompt":"Draw a bird","imageName":"bird","aspectRatio":"1:1"}}
]}`

func TestFailedWrapperStepsRenderTheirError(t *testing.T) {
	md := renderString(t, decodeTrajectory(t, failedWrappersJSON))

	view := stepSection(t, md, 1, "VIEW_FILE")
	assertContainsAll(t, view, "file:///fixture/notes.md", "### Step Error", "> ContentOffset 9000 exceeds line range size 1200")

	generic := stepSection(t, md, 2, "GENERIC")
	assertContainsAll(t, generic, `"TargetFile": "/fixture/out/notes.md"`, "### Step Error",
		"> invalid tool call: /fixture/out/notes.md is not a valid artifact path",
		"<summary>Full error</summary>", "declaring permissions: invalid tool call")

	image := stepSection(t, md, 3, "GENERATE_IMAGE")
	assertContainsAll(t, image, "- **Image Name:** `bird`", "### Step Error", "> failed to generate content: 429 Too Many Requests")
}
