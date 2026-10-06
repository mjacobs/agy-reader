package completeness

import (
	"strings"
	"testing"
)

// These fixtures cover a view_file call that agy rejected before the tool ran
// and whose step records a tool source (agy-reader#2xqw), for example a view
// of a named pipe or /dev/null, which agy refuses with "path is a device,
// pipe, or socket, not a standard file". The source keeps a generic wrapper
// with arguments and an outer error but no result. Frozen source, full RPC and
// default RPC copies (12 steps in 8 retained sessions and 2 in the agy 1.2.17
// audit) showed that the default response presents a VIEW_FILE step instead:
// the wrapper's status, metadata and error, no generic arguments, and a
// view_file holding the path as a file URI and the one-based StartLine and
// EndLine as zero-based start_line and end_line. Values are synthetic; every
// response is written out, never derived from the projection.
const resultlessViewMetadata = `"metadata":{"toolCall":{"id":"call-fifo","name":"view_file","argumentsJson":"{\"AbsolutePath\":\"/fixture/ws/audit.fifo\",\"toolAction\":\"Viewing FIFO\",\"toolSummary\":\"View FIFO\"}"},"completedAt":"2026-01-01T00:00:01Z","toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN"}`
const resultlessViewError = `"error":{"shortError":"invalid tool call: cannot view \"/fixture/ws/audit.fifo\": path is a device, pipe, or socket, not a standard file","fullError":"declaring permissions: invalid tool call: cannot view \"/fixture/ws/audit.fifo\": path is a device, pipe, or socket, not a standard file"}`
const resultlessViewSourceRaw = `{"cascadeId":"resultless-source-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR",` + resultlessViewMetadata + `,` + resultlessViewError + `,"generic":{"args":{"AbsolutePath":"/fixture/ws/audit.fifo","toolAction":"Viewing FIFO","toolSummary":"View FIFO"}}}]}`
const resultlessViewSourcePresentation = `{"cascadeId":"resultless-source-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_VIEW_FILE","status":"CORTEX_STEP_STATUS_ERROR",` + resultlessViewMetadata + `,` + resultlessViewError + `,"viewFile":{"absolutePathUri":"file:///fixture/ws/audit.fifo"}}]}`

// The same failure with a line range: StartLine 2425 and EndLine 2460 are
// presented as start_line 2424 and end_line 2459.
const resultlessViewLinesMetadata = `"metadata":{"toolCall":{"id":"call-lines","name":"view_file","argumentsJson":"{\"AbsolutePath\":\"/fixture/cache/metadata.json\",\"EndLine\":2460,\"StartLine\":2425,\"toolAction\":\"Viewing metadata.json\",\"toolSummary\":\"View metadata.json\"}"},"completedAt":"2026-01-01T00:00:01Z","toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN"}`
const resultlessViewLinesError = `"error":{"shortError":"permission check failed for read_file \"/fixture/cache/metadata.json\"","fullError":"permission check failed for read_file \"/fixture/cache/metadata.json\": denied"}`
const resultlessViewLinesRaw = `{"cascadeId":"resultless-lines-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR",` + resultlessViewLinesMetadata + `,` + resultlessViewLinesError + `,"generic":{"args":{"AbsolutePath":"/fixture/cache/metadata.json","EndLine":"2460","StartLine":"2425","toolAction":"Viewing metadata.json","toolSummary":"View metadata.json"}}}]}`
const resultlessViewLinesPresentation = `{"cascadeId":"resultless-lines-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_VIEW_FILE","status":"CORTEX_STEP_STATUS_ERROR",` + resultlessViewLinesMetadata + `,` + resultlessViewLinesError + `,"viewFile":{"absolutePathUri":"file:///fixture/cache/metadata.json","startLine":2424,"endLine":2459}}]}`

// A StartLine of 1 becomes start_line 0, which the presentation omits.
var resultlessViewFirstLineRaw = strings.NewReplacer(`\"StartLine\":2425`, `\"StartLine\":1`, `"StartLine":"2425"`, `"StartLine":"1"`).Replace(resultlessViewLinesRaw)
var resultlessViewFirstLinePresentation = strings.NewReplacer(`\"StartLine\":2425`, `\"StartLine\":1`, `"startLine":2424,`, ``).Replace(resultlessViewLinesPresentation)

func TestProjectionResultlessViewQualified(t *testing.T) {
	for name, pair := range map[string][2]string{
		"no line range":    {resultlessViewSourceRaw, resultlessViewSourcePresentation},
		"line range":       {resultlessViewLinesRaw, resultlessViewLinesPresentation},
		"range from first": {resultlessViewFirstLineRaw, resultlessViewFirstLinePresentation},
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, snapshot1216(t, pair[0]), pair[0], pair[1], Verified)
		})
	}
}

// A source mutation applies to the persisted trajectory and both RPC copies,
// and to the presentation wherever it carries the same text; a presentation
// or raw mutation changes only that response. Every refused
// source keeps the refusal text the reader gave before this shape was
// qualified.
func TestProjectionResultlessViewChanges(t *testing.T) {
	const pathArg = `"AbsolutePath":"/fixture/ws/audit.fifo"`
	const pathCall = `\"AbsolutePath\":\"/fixture/ws/audit.fifo\"`
	const pathURI = `"absolutePathUri":"file:///fixture/ws/audit.fifo"`
	// path replaces the viewed path in the arguments, the presented tool call
	// and the presented URI alike.
	path := func(p string) [2]string {
		return [2]string{`/fixture/ws/audit.fifo`, p}
	}
	for _, tc := range []struct {
		name, source, presentation string
		swaps                      [][2]string
		where                      string
		want                       State
		reason                     string
	}{
		// The daemon marks skill files; the source cannot tell which.
		{"skill file path", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{path(`/fixture/skills/notes/SKILL.md`)}, "source", Unknown, "unsupported resultless view_file"},
		{"skill in any letter case", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{path(`/fixture/MySkill.md`)}, "source", Unknown, "unsupported resultless view_file"},
		{"provider tool source", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{`CORTEX_COMPONENT_SOURCE_BUILTIN`, `CORTEX_COMPONENT_SOURCE_PROVIDER`}}, "source", Unknown, "unsupported resultless view_file"},
		{"path with a space", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{path(`/fixture/ws/audit fifo`)}, "source", Unknown, "unsupported resultless view_file"},
		{"relative path", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{path(`fixture/ws/audit.fifo`)}, "source", Unknown, "unsupported resultless view_file"},
		{"unclean path", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{path(`/fixture/ws/../ws/audit.fifo`)}, "source", Unknown, "unsupported resultless view_file"},
		{"status invalid", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{`"CORTEX_STEP_STATUS_ERROR"`, `"CORTEX_STEP_STATUS_INVALID"`}}, "source", Unknown, "unsupported generic wrapper"},
		{"extra error field", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{`"error":{`, `"error":{"errorId":"fixture-error",`}}, "source", Unknown, "unsupported resultless view_file"},
		{"extra wrapper field", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{`"generic":{`, `"taskDetails":{"id":"fixture/task-1"},"generic":{`}}, "source", Unknown, "unsupported generic wrapper"},
		{"unknown argument", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{pathArg, pathArg + `,"Future":"new"`}, {pathCall, pathCall + `,\"Future\":\"new\"`}}, "source", Unknown, "unsupported resultless view_file"},
		{"content offset argument", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{pathArg, pathArg + `,"ContentOffset":"10"`}, {pathCall, pathCall + `,\"ContentOffset\":10`}}, "source", Unknown, "unsupported resultless view_file"},
		{"argument differs from the presented call", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{`"toolSummary":"View FIFO"}`, `"toolSummary":"View pipe"}`}}, "source", Unknown, "unsupported resultless view_file"},
		{"argument missing from the presented call", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{`,\"toolSummary\":\"View FIFO\"`, ``}}, "source", Unknown, "unsupported resultless view_file"},
		{"start line only", resultlessViewLinesRaw, resultlessViewLinesPresentation, [][2]string{{`"EndLine":"2460",`, ``}, {`\"EndLine\":2460,`, ``}, {`,"endLine":2459`, ``}}, "source", Unknown, "unsupported resultless view_file"},
		{"start line zero", resultlessViewLinesRaw, resultlessViewLinesPresentation, [][2]string{{`"StartLine":"2425"`, `"StartLine":"0"`}, {`\"StartLine\":2425`, `\"StartLine\":0`}}, "source", Unknown, "unsupported resultless view_file"},
		{"end before start", resultlessViewLinesRaw, resultlessViewLinesPresentation, [][2]string{{`"EndLine":"2460"`, `"EndLine":"2400"`}, {`\"EndLine\":2460`, `\"EndLine\":2400`}, {`"endLine":2459`, `"endLine":2399`}}, "source", Unknown, "unsupported resultless view_file"},
		{"non-canonical line", resultlessViewLinesRaw, resultlessViewLinesPresentation, [][2]string{{`"StartLine":"2425"`, `"StartLine":"02425"`}}, "source", Unknown, "unsupported resultless view_file"},
		// What the presentation shows must be what the source predicts.
		{"presented as a skill file", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{pathURI, pathURI + `,"isSkillFile":true`}}, "presentation", Incomplete, ""},
		{"presented path changed", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{pathURI, `"absolutePathUri":"file:///fixture/ws/other.fifo"`}}, "presentation", Incomplete, ""},
		{"presented start line not converted", resultlessViewLinesRaw, resultlessViewLinesPresentation, [][2]string{{`"startLine":2424`, `"startLine":2425`}}, "presentation", Incomplete, ""},
		{"presented end line not converted", resultlessViewLinesRaw, resultlessViewLinesPresentation, [][2]string{{`"endLine":2459`, `"endLine":2460`}}, "presentation", Incomplete, ""},
		{"presented line range dropped", resultlessViewLinesRaw, resultlessViewLinesPresentation, [][2]string{{`,"startLine":2424,"endLine":2459`, ``}}, "presentation", Incomplete, ""},
		{"presented error changed", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{`"shortError":"invalid tool call`, `"shortError":"rejected tool call`}}, "presentation", Incomplete, ""},
		{"presented status changed", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{`"CORTEX_STEP_STATUS_ERROR"`, `"CORTEX_STEP_STATUS_DONE"`}}, "presentation", Incomplete, ""},
		{"presented tool source dropped", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{`,"toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN"`, ``}}, "presentation", Incomplete, ""},
		{"presented unchanged", resultlessViewSourceRaw, resultlessViewSourceRaw, nil, "presentation", Incomplete, ""},
		{"raw argument changed", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{`"toolAction":"Viewing FIFO"`, `"toolAction":"Viewing pipe"`}}, "raw", Incomplete, ""},
		{"raw full error changed", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{`"fullError":"declaring permissions`, `"fullError":"checking permissions`}}, "raw", Incomplete, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, presentation := tc.source, tc.presentation
			s := snapshot1216(t, raw)
			for _, swap := range tc.swaps {
				switch tc.where {
				case "source":
					if !strings.Contains(raw, swap[0]) && !strings.Contains(presentation, swap[0]) {
						t.Fatalf("invalid fixture mutation: %q", swap[0])
					}
					raw = strings.ReplaceAll(raw, swap[0], swap[1])
					presentation = strings.ReplaceAll(presentation, swap[0], swap[1])
				case "raw":
					raw = replace1216(t, raw, swap[0], swap[1])
				case "presentation":
					presentation = replace1216(t, presentation, swap[0], swap[1])
				}
			}
			if tc.where == "source" {
				s = snapshot1216(t, raw)
			}
			require1216(t, s, raw, presentation, tc.want)
			if tc.reason != "" {
				if _, err := project(s.trajectory); err == nil || err.Error() != tc.reason {
					t.Fatalf("projection error %v, want %q", err, tc.reason)
				}
			}
		})
	}
}

// A resultless view_file without a tool source is still presented unchanged
// and must not be converted.
func TestProjectionResultlessViewWithoutToolSource(t *testing.T) {
	raw := strings.ReplaceAll(resultlessViewSourceRaw, `,"toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN"`, ``)
	require1216(t, snapshot1216(t, raw), raw, raw, Verified)
	presentation := strings.ReplaceAll(resultlessViewSourcePresentation, `,"toolSource":"CORTEX_COMPONENT_SOURCE_BUILTIN"`, ``)
	require1216(t, snapshot1216(t, raw), raw, presentation, Incomplete)
}
