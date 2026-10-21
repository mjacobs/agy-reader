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
// or raw mutation changes only that response. Refusal checks keep the
// unsupported shape distinct from a changed RPC response.
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
		// A path name alone does not establish a skill registration.
		{"skill file path", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{path(`/fixture/skills/notes/SKILL.md`)}, "source", Verified, ""},
		{"skill in any letter case", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{path(`/fixture/MySkill.md`)}, "source", Verified, ""},
		{"provider tool source", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{`CORTEX_COMPONENT_SOURCE_BUILTIN`, `CORTEX_COMPONENT_SOURCE_PROVIDER`}}, "source", Unknown, "unsupported resultless view_file"},
		{"path with a space", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{path(`/fixture/ws/audit fifo`)}, "source", Unknown, "unsupported resultless view_file"},
		{"relative path", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{path(`fixture/ws/audit.fifo`)}, "source", Unknown, "unsupported resultless view_file"},
		{"unclean path", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{path(`/fixture/ws/../ws/audit.fifo`)}, "source", Unknown, "unsupported resultless view_file"},
		{"status invalid", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{{`"CORTEX_STEP_STATUS_ERROR"`, `"CORTEX_STEP_STATUS_INVALID"`}}, "source", Unknown, "unsupported invalid generic wrapper"},
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

func legacyViewExecutor(id, end, entries string) string {
	return `{"executionId":"` + id + `","lastStepIdx":` + end + `,"loadedCustomizations":{"customizations":[` + entries + `]}}`
}

func legacyViewSkillPair(executors, executionID, path string, marked bool) (string, string) {
	apply := func(s string) string {
		s = strings.ReplaceAll(s, "/fixture/ws/audit.fifo", path)
		s = strings.Replace(s, `"steps":[`, `"trajectoryId":"fixture-trajectory","executorMetadatas":[`+executors+`],"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{}},`, 1)
		s = strings.Replace(s, `"metadata":{"toolCall":`, `"metadata":{"executionId":"`+executionID+`","sourceTrajectoryStepInfo":{"cascadeId":"resultless-source-fixture","trajectoryId":"fixture-trajectory","stepIndex":1},"toolCall":`, 1)
		return s
	}
	raw, shown := apply(resultlessViewSourceRaw), apply(resultlessViewSourcePresentation)
	if marked {
		shown = strings.Replace(shown, `"viewFile":{`, `"viewFile":{"isSkillFile":true,`, 1)
	}
	return raw, shown
}

func TestProjectionResultlessViewSkillRegistry(t *testing.T) {
	prior := legacyViewExecutor("prior-exec", "0", skill144Entry)
	current := legacyViewExecutor("current-exec", "2", "")
	currentSkill := legacyViewExecutor("current-exec", "2", skill144Entry)
	unrelated := legacyViewExecutor("unrelated-exec", "0", "")
	disabled := strings.Replace(skill144Entry, "STATUS_ENABLED", "STATUS_DISABLED", 1)
	rule := strings.Replace(skill144Entry, "REFRESH_CUSTOMIZATION_TYPE_SKILL", "REFRESH_CUSTOMIZATION_TYPE_RULE", 1)
	for _, tc := range []struct {
		name, executors, executionID, path string
		marked                             bool
	}{
		{"earlier registered skill", prior + "," + current, "current-exec", skill144Path, true},
		{"current registered skill", legacyViewExecutor("current-exec", "2", skill144Entry), "current-exec", skill144Path, true},
		{"future-only registration", current + "," + legacyViewExecutor("future-exec", "3", skill144Entry), "current-exec", skill144Path, false},
		{"path absent from registry", prior + "," + current, "current-exec", "/fixture/skills/other/SKILL.md", false},
		{"case differs", prior + "," + current, "current-exec", "/fixture/skills/example/skill.md", false},
		{"no registration", current, "current-exec", skill144Path, false},
		{"current executor missing", prior, "missing-exec", skill144Path, false},
		{"no execution identity", prior + "," + current, "", skill144Path, false},
		{"registered plain name", strings.ReplaceAll(prior, skill144Path, "/fixture/entry.md") + "," + current, "current-exec", "/fixture/entry.md", true},
		{"disabled prior registration", strings.Replace(prior, "STATUS_ENABLED", "STATUS_DISABLED", 1) + "," + current, "current-exec", skill144Path, false},
		{"non-skill prior registration", strings.Replace(prior, "REFRESH_CUSTOMIZATION_TYPE_SKILL", "REFRESH_CUSTOMIZATION_TYPE_RULE", 1) + "," + current, "current-exec", skill144Path, false},
		{"disabled current plain path", strings.ReplaceAll(strings.Replace(currentSkill, "STATUS_ENABLED", "STATUS_DISABLED", 1), skill144Path, "/fixture/notes.md"), "current-exec", "/fixture/notes.md", false},
		{"non-skill current plain path", strings.ReplaceAll(strings.Replace(currentSkill, "REFRESH_CUSTOMIZATION_TYPE_SKILL", "REFRESH_CUSTOMIZATION_TYPE_RULE", 1), skill144Path, "/fixture/notes.md"), "current-exec", "/fixture/notes.md", false},
		{"unmarked unrelated duplicate prior IDs", unrelated + "," + unrelated + "," + current, "current-exec", "/fixture/notes.md", false},
		{"unmarked unrelated duplicate future IDs", current + "," + unrelated + "," + unrelated, "current-exec", "/fixture/notes.md", false},
		{"unmarked duplicate current IDs without registration", current + "," + current, "current-exec", "/fixture/notes.md", false},
		{"unmarked duplicate current IDs with non-skill registrations", strings.ReplaceAll(legacyViewExecutor("current-exec", "0", rule)+","+legacyViewExecutor("current-exec", "2", rule), skill144Path, "/fixture/notes.md"), "current-exec", "/fixture/notes.md", false},
		{"current skill ignores duplicate prior IDs", unrelated + "," + unrelated + "," + currentSkill, "current-exec", skill144Path, true},
		{"current skill ignores duplicate future IDs", currentSkill + "," + unrelated + "," + unrelated, "current-exec", skill144Path, true},
		{"prior skill ignores duplicate future IDs", prior + "," + current + "," + unrelated + "," + unrelated, "current-exec", skill144Path, true},
		{"disabled prior skips unrelated identity and order checks", strings.Replace(prior, "STATUS_ENABLED", "STATUS_DISABLED", 1) + "," + unrelated + "," + unrelated + "," + current, "current-exec", skill144Path, false},
		{"disabled prior invalid bounds", legacyViewExecutor("prior-exec", "3", disabled) + "," + current, "current-exec", skill144Path, false},
		{"non-skill prior invalid bounds", legacyViewExecutor("prior-exec", "3", rule) + "," + current, "current-exec", skill144Path, false},
		{"duplicate disabled registrations", legacyViewExecutor("prior-exec", "0", disabled+","+disabled) + "," + current, "current-exec", skill144Path, false},
		{"repeated non-skill registrations", legacyViewExecutor("prior-exec", "0", rule) + "," + legacyViewExecutor("current-exec", "2", rule), "current-exec", skill144Path, false},
		{"disabled and non-skill registrations", legacyViewExecutor("prior-exec", "0", rule) + "," + legacyViewExecutor("current-exec", "2", disabled), "current-exec", skill144Path, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, shown := legacyViewSkillPair(tc.executors, tc.executionID, tc.path, tc.marked)
			s := snapshot1216(t, raw)
			require1216(t, s, raw, shown, Verified)
			wrong := strings.Replace(shown, `"isSkillFile":true,`, "", 1)
			if !tc.marked {
				wrong = strings.Replace(shown, `"viewFile":{`, `"viewFile":{"isSkillFile":true,`, 1)
			}
			require1216(t, s, raw, wrong, Incomplete)
		})
	}
	for name, executors := range map[string]string{
		"duplicate current identity":             prior + "," + current + "," + current,
		"ambiguous current enabled registration": legacyViewExecutor("current-exec", "0", "") + "," + currentSkill,
		"duplicate prior identity":               prior + "," + prior + "," + current,
		"missing prior identity":                 legacyViewExecutor("", "0", skill144Entry) + "," + current,
		"duplicate registration":                 legacyViewExecutor("prior-exec", "0", skill144Entry+","+skill144Entry) + "," + current,
		"current repeats registration":           prior + "," + legacyViewExecutor("current-exec", "2", skill144Entry),
		"conflicting registration":               prior + "," + legacyViewExecutor("current-exec", "2", strings.Replace(skill144Entry, "STATUS_ENABLED", "STATUS_DISABLED", 1)),
		"mixed skill and non-skill registration": prior + "," + legacyViewExecutor("current-exec", "2", rule),
		"prior ends at read":                     legacyViewExecutor("prior-exec", "1", skill144Entry) + "," + current,
		"prior ends after read":                  legacyViewExecutor("prior-exec", "3", skill144Entry) + "," + current,
		"current ends before read":               prior + "," + legacyViewExecutor("current-exec", "0", ""),
		"unordered prior executors":              prior + "," + legacyViewExecutor("other-exec", "0", "") + "," + current,
	} {
		t.Run(name, func(t *testing.T) {
			raw, shown := legacyViewSkillPair(executors, "current-exec", skill144Path, true)
			require1216(t, snapshot1216(t, raw), raw, shown, Unknown)
		})
	}
	for name, entry := range map[string]string{"disabled": disabled, "non-skill": rule} {
		t.Run(name+" prior does not need source identity", func(t *testing.T) {
			raw, shown := legacyViewSkillPair(legacyViewExecutor("", "3", entry)+","+current, "current-exec", skill144Path, false)
			remove := `"sourceTrajectoryStepInfo":{"cascadeId":"resultless-source-fixture","trajectoryId":"fixture-trajectory","stepIndex":1},`
			raw, shown = strings.Replace(raw, remove, "", 1), strings.Replace(shown, remove, "", 1)
			s := snapshot1216(t, raw)
			require1216(t, s, raw, shown, Verified)
			require1216(t, s, raw, strings.Replace(shown, `"viewFile":{`, `"viewFile":{"isSkillFile":true,`, 1), Incomplete)
		})
	}
	// Duplicate prior identities remain ambiguous when a prior enabled skill is
	// the evidence for a true marker, even if their end indices are ordered and
	// only one of those executors contains the matching path.
	executors := legacyViewExecutor("prior-exec", "0", skill144Entry) + "," +
		legacyViewExecutor("prior-exec", "1", "") + "," + legacyViewExecutor("current-exec", "2", "")
	badRaw, badShown := legacyViewSkillPair(executors, "current-exec", skill144Path, true)
	addStep := func(s string) string {
		s = strings.Replace(s, `"steps":[`, `"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{}},`, 1)
		return strings.Replace(s, `"stepIndex":1`, `"stepIndex":2`, 1)
	}
	badRaw, badShown = addStep(badRaw), addStep(badShown)
	require1216(t, snapshot1216(t, badRaw), badRaw, badShown, Unknown)
	raw, shown := legacyViewSkillPair(prior+","+current, "current-exec", skill144Path, true)
	for name, change := range map[string][2]string{
		"source step index mismatch": {`"stepIndex":1`, `"stepIndex":0`},
		"source trajectory mismatch": {`"sourceTrajectoryStepInfo":{"cascadeId":"resultless-source-fixture","trajectoryId":"fixture-trajectory"`, `"sourceTrajectoryStepInfo":{"cascadeId":"resultless-source-fixture","trajectoryId":"other-trajectory"`},
		"source cascade mismatch":    {`"sourceTrajectoryStepInfo":{"cascadeId":"resultless-source-fixture"`, `"sourceTrajectoryStepInfo":{"cascadeId":"other-cascade"`},
	} {
		t.Run(name, func(t *testing.T) {
			bad := replace1216(t, raw, change[0], change[1])
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
	for name, change := range map[string][2]string{
		"registry status": {`STATUS_ENABLED`, `STATUS_DISABLED`},
		"registry path":   {`"path":"` + skill144Path + `"`, `"path":"/fixture/other.md"`},
		"registry bounds": {`"lastStepIdx":0`, `"lastStepIdx":1`},
	} {
		t.Run("response "+name, func(t *testing.T) {
			s := snapshot1216(t, raw)
			require1216(t, s, replace1216(t, raw, change[0], change[1]), shown, Incomplete)
			require1216(t, s, raw, replace1216(t, shown, change[0], change[1]), Incomplete)
		})
	}
}
