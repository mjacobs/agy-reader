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
		// failedread146: a SKILL.md path is presented marked, so an unmarked
		// presentation of one is incomplete; other names stay unmarked.
		{"skill file path presented unmarked", resultlessViewSourceRaw, resultlessViewSourcePresentation, [][2]string{path(`/fixture/skills/notes/SKILL.md`)}, "source", Incomplete, ""},
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

// TestProjectionFailedRead146Skill follows agy 1.4.6 viewFileFromArgs, which
// rebuilds a failed legacy view_file and sets is_skill_file with the same
// path-only isSkillFile check as rehydrateReadFile (agy-reader#2f8f). The
// executor registry, its identities and its ordering never change the marker:
// each case's expected marker is the daemon's rule written out, and the
// opposite marker must be incomplete.
func TestProjectionFailedRead146Skill(t *testing.T) {
	prior := legacyViewExecutor("prior-exec", "0", skill144Entry)
	current := legacyViewExecutor("current-exec", "2", "")
	currentSkill := legacyViewExecutor("current-exec", "2", skill144Entry)
	unrelated := legacyViewExecutor("unrelated-exec", "0", "")
	disabled := strings.Replace(skill144Entry, "STATUS_ENABLED", "STATUS_DISABLED", 1)
	rule := strings.Replace(skill144Entry, "REFRESH_CUSTOMIZATION_TYPE_SKILL", "REFRESH_CUSTOMIZATION_TYPE_RULE", 1)
	plain := func(executors string) string { return strings.ReplaceAll(executors, skill144Path, "/fixture/entry.md") }
	for _, tc := range []struct {
		name, executors, executionID, path string
		marked                             bool
	}{
		// The two cases the registry rule got wrong.
		{"unregistered SKILL.md", current, "current-exec", skill144Path, true},
		{"registered non-SKILL.md in current executor", plain(currentSkill), "current-exec", "/fixture/entry.md", false},
		{"registered non-SKILL.md in earlier executor", plain(prior) + "," + current, "current-exec", "/fixture/entry.md", false},
		// Registered SKILL.md reads stay marked, as conversation-098 step 62 is.
		{"earlier registered SKILL.md", prior + "," + current, "current-exec", skill144Path, true},
		{"current registered SKILL.md", currentSkill, "current-exec", skill144Path, true},
		// No registry evidence is needed for a SKILL.md path.
		{"registry names another SKILL.md", prior + "," + current, "current-exec", "/fixture/skills/other/SKILL.md", true},
		{"future-only registration", current + "," + legacyViewExecutor("future-exec", "3", skill144Entry), "current-exec", skill144Path, true},
		{"current executor missing", prior, "missing-exec", skill144Path, true},
		{"no execution identity", prior + "," + current, "", skill144Path, true},
		{"no executors", "", "current-exec", skill144Path, true},
		{"disabled registration", legacyViewExecutor("prior-exec", "0", disabled) + "," + current, "current-exec", skill144Path, true},
		{"non-skill registration", legacyViewExecutor("prior-exec", "0", rule) + "," + current, "current-exec", skill144Path, true},
		{"reference named SKILL.md", prior + "," + current, "current-exec", "/fixture/skills/example/references/SKILL.md", true},
		{"plain SKILL.md", current, "current-exec", "/SKILL.md", true},
		// Registry shapes the history2 rule refused no longer matter.
		{"duplicate current identity", prior + "," + current + "," + current, "current-exec", skill144Path, true},
		{"duplicate registration", legacyViewExecutor("prior-exec", "0", skill144Entry+","+skill144Entry) + "," + current, "current-exec", skill144Path, true},
		{"prior ends after read", legacyViewExecutor("prior-exec", "3", skill144Entry) + "," + current, "current-exec", skill144Path, true},
		{"unordered prior executors", prior + "," + legacyViewExecutor("other-exec", "0", "") + "," + current, "current-exec", skill144Path, true},
		{"duplicate unrelated identities", unrelated + "," + unrelated + "," + currentSkill, "current-exec", "/fixture/entry.md", false},
		// Only the exact, case-sensitive basename SKILL.md is marked.
		{"case differs", prior + "," + current, "current-exec", "/fixture/skills/example/skill.md", false},
		{"name only ends in SKILL.md", current, "current-exec", "/fixture/skills/example/MYSKILL.md", false},
		{"longer extension", current, "current-exec", "/fixture/skills/example/SKILL.md.bak", false},
		{"ordinary file", current, "current-exec", "/fixture/notes.md", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := skillFileRead146(tc.path); got != tc.marked {
				t.Fatalf("fixture expects marker %v, path rule gives %v", tc.marked, got)
			}
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
	// The saved source step identity was needed only for an earlier
	// registration; it no longer changes the marker.
	raw, shown := legacyViewSkillPair(prior+","+current, "current-exec", skill144Path, true)
	for name, change := range map[string][2]string{
		"source step index mismatch": {`"stepIndex":1`, `"stepIndex":0`},
		"source trajectory mismatch": {`"sourceTrajectoryStepInfo":{"cascadeId":"resultless-source-fixture","trajectoryId":"fixture-trajectory"`, `"sourceTrajectoryStepInfo":{"cascadeId":"resultless-source-fixture","trajectoryId":"other-trajectory"`},
		"source cascade mismatch":    {`"sourceTrajectoryStepInfo":{"cascadeId":"resultless-source-fixture"`, `"sourceTrajectoryStepInfo":{"cascadeId":"other-cascade"`},
	} {
		t.Run(name, func(t *testing.T) {
			bad := replace1216(t, raw, change[0], change[1])
			badShown := replace1216(t, shown, change[0], change[1])
			require1216(t, snapshot1216(t, bad), bad, badShown, Verified)
		})
	}
	// The registry is still part of the full-source comparison: a response
	// whose registry differs from the source is incomplete.
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

func TestFailedRead146InvalidatesPreviousProfile(t *testing.T) {
	const previous = "sqlite-v1-format0-agy-1.3.4-5b93565c5f04-planner50-permission2-prompt23-tools143-permission1-tools144-skill1-history2-ide225-options145-compaction146-skill146-finish147-replay147"
	if Profile == previous {
		t.Fatal("failed-read skill rule did not change the profile")
	}
	raw, shown := legacyViewSkillPair(legacyViewExecutor("current-exec", "2", ""), "current-exec", skill144Path, true)
	s := snapshot1216(t, raw)
	a, r := s.VerifyResponses([]byte(raw), []byte(shown))
	if r.State != Verified {
		t.Fatal(r)
	}
	a.Profile = previous
	if r := s.CheckAttestation([]byte(shown), a); r.State != Unknown {
		t.Fatalf("previous profile accepted: %+v", r)
	}
}
