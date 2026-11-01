package completeness

import (
	"strings"
	"testing"
)

// Synthetic fixtures shaped like the native agy 1.4.12 capture recorded in
// docs/compatibility/agy-1.4.12-file-uri-paths.md. In every ReadFile, FileEdit,
// image read_file, failed read_file and failed edit_file presentation, the
// default RPC shows "file://" + path without escaping or normalizing it.
const (
	paths1410Doc   = `/fixture/docs/Project Notes — Phase 1 & 2.md`
	paths1410Img   = `/fixture/docs/Diagram — Phase 1 & 2.png`
	paths1410Punct = "/fixture/docs/C# 50% done? a%20b, café.md"

	paths1410ReadMeta  = `"metadata":{"toolCall":{"name":"read_file","argumentsJson":"{\"EndLine\":4,\"Path\":\"` + paths1410Doc + `\",\"StartLine\":2}"},"completedAt":"2026-01-01T00:00:02Z"}`
	paths1410ReadRaw   = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE",` + paths1410ReadMeta + `,"generic":{"args":{"EndLine":"4","Path":"` + paths1410Doc + `","StartLine":"2"},"result":{"result":"fixture result","fullOutputUri":"file:///fixture/output.txt","stepRenderInfo":{"title":"fixture title"},"payload":{"@type":"type.googleapis.com/exa.cortex.core.tools.ReadFile","path":"` + paths1410Doc + `","startLine":2,"endLine":4,"fileNumLines":8,"fileNumBytes":80,"lineRangeBytes":30}}}}]}`
	paths1410ReadShown = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_VIEW_FILE","status":"CORTEX_STEP_STATUS_DONE",` + paths1410ReadMeta + `,"viewFile":{"absolutePathUri":"file://` + paths1410Doc + `","startLine":1,"endLine":3,"numLines":8,"numBytes":80,"lineRangeBytes":30,"content":"fixture result"}}]}`

	paths1410EditMeta  = `"metadata":{"toolCall":{"name":"edit_file","argumentsJson":"{\"Path\":\"` + paths1410Doc + `\",\"Replacement\":\"fixture\",\"Target\":\"old\"}"},"completedAt":"2026-01-01T00:00:02Z"}`
	paths1410EditRaw   = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE",` + paths1410EditMeta + `,"generic":{"args":{"Path":"` + paths1410Doc + `","Replacement":"fixture","Target":"old"},"result":{"result":"fixture result","fullOutputUri":"file:///fixture/output.txt","stepRenderInfo":{"title":"fixture title"},"payload":{"@type":"type.googleapis.com/exa.cortex.core.tools.FileEdit","kind":"EDIT","path":"` + paths1410Doc + `","diff":{"unifiedDiff":{"lines":[{"text":"fixture","type":"UNIFIED_DIFF_LINE_TYPE_INSERT"}]}},"diffStats":{"additions":1}}}}}]}`
	paths1410EditShown = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_CODE_ACTION","status":"CORTEX_STEP_STATUS_DONE",` + paths1410EditMeta + `,"codeAction":{"actionSpec":{"command":{"isEdit":true,"file":{"absoluteUri":"file://` + paths1410Doc + `"}}},"actionResult":{"edit":{"absoluteUri":"file://` + paths1410Doc + `","diff":{"unifiedDiff":{"lines":[{"text":"fixture","type":"UNIFIED_DIFF_LINE_TYPE_INSERT"}]}}}},"diffStats":{"additions":1}}}]}`

	paths1410FailedReadErr   = `open ` + paths1410Doc + `: no such file or directory`
	paths1410FailedReadMeta  = `"metadata":{"toolCall":{"name":"read_file","argumentsJson":"{\"Path\":\"` + paths1410Doc + `\"}"},"completedAt":"2026-01-01T00:00:02Z"},"error":{"shortError":"` + paths1410FailedReadErr + `","fullError":"` + paths1410FailedReadErr + `"}`
	paths1410FailedReadRaw   = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_ERROR",` + paths1410FailedReadMeta + `,"generic":{"args":{"Path":"` + paths1410Doc + `"},"result":{"stepRenderInfo":{"titlePrefix":"Reading","icon":"file"}}}}]}`
	paths1410FailedReadShown = `{"cascadeId":"fixture-parent","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_VIEW_FILE","status":"CORTEX_STEP_STATUS_ERROR",` + paths1410FailedReadMeta + `,"viewFile":{"absolutePathUri":"file://` + paths1410Doc + `"}}]}`
)

func TestPaths1410Qualified(t *testing.T) {
	failedEditRaw := strings.ReplaceAll(failedEditRawClean, `/fixture/code.go`, paths1410Doc)
	failedEditShownDoc := strings.ReplaceAll(failedEditShown, `/fixture/code.go`, paths1410Doc)
	imgRawBase, imgShownBase := imageReadPair147("image/png")
	imgRaw := strings.ReplaceAll(imgRawBase, `/fixture/image`, paths1410Img)
	imgShown := strings.ReplaceAll(imgShownBase, `/fixture/image`, paths1410Img)

	punct := func(s string) string { return strings.ReplaceAll(s, paths1410Doc, paths1410Punct) }

	for name, pair := range map[string][2]string{
		"read unicode and spaces":        {paths1410ReadRaw, paths1410ReadShown},
		"edit unicode and spaces":        {paths1410EditRaw, paths1410EditShown},
		"failed read unicode and spaces": {paths1410FailedReadRaw, paths1410FailedReadShown},
		"failed edit unicode and spaces": {failedEditRaw, failedEditShownDoc},
		"image read unicode and spaces":  {imgRaw, imgShown},
		"read # % ? and decomposed":      {punct(paths1410ReadRaw), punct(paths1410ReadShown)},
		"edit # % ? and decomposed":      {punct(paths1410EditRaw), punct(paths1410EditShown)},
		"failed read # % ? decomposed":   {punct(paths1410FailedReadRaw), punct(paths1410FailedReadShown)},
	} {
		t.Run(name, func(t *testing.T) {
			s := snapshot1216(t, pair[0])
			require1216(t, s, pair[0], pair[1], Verified)
			offline, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, pair[0], string(offline), Verified)
		})
	}
}

func TestPaths1410RefusesInvalidPaths(t *testing.T) {
	for _, tc := range []struct {
		name, badPath string
	}{
		{"relative path", `docs/Project Notes.md`},
		{"unclean dotdot", `/fixture/docs/../Project Notes.md`},
		{"newline control", `/fixture/docs/Project\nNotes.md`},
		{"del control", `/fixture/docs/Project\u007fNotes.md`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := strings.ReplaceAll(paths1410ReadRaw, paths1410Doc, tc.badPath)
			if _, err := snapshot1216(t, bad).OfflineJSON(); err == nil {
				t.Fatalf("accepted invalid path %q", tc.badPath)
			}
		})
	}
}

func TestVerbatimURIPath(t *testing.T) {
	for path, want := range map[string]bool{
		"/fixture/plain.md":                       true,
		"/fixture/Project Notes — Phase 1 & 2.md": true,
		"/fixture/Notes, draft.md":                true,
		"/fixture/C# notes.md":                    true,
		"/fixture/50% done.md":                    true,
		"/fixture/what?.md":                       true,
		"/fixture/a%20b.md":                       true,
		"/fixture/café decomposed.md":            true,
		"/":                                       true,
		"fixture/relative.md":                     false,
		"/fixture/../unclean.md":                  false,
		"/fixture/dir/":                           false,
		"/fixture//double.md":                     false,
		"/fixture/back\\slash.md":                 false,
		"/fixture/new\nline.md":                   false,
		"/fixture/del\x7f.md":                     false,
		"/fixture/bad\xff.md":                     false,
	} {
		if got := verbatimURIPath(path); got != want {
			t.Errorf("verbatimURIPath(%q) = %v, want %v", path, got, want)
		}
	}
}

// paths1410With puts jsonPath, a path already escaped for a JSON string, into a
// fixture. The copy inside argumentsJson sits in a JSON string within a JSON
// string, so its backslashes are escaped once more.
func paths1410With(raw, jsonPath string) string {
	inner := strings.ReplaceAll(jsonPath, `\`, `\\`)
	raw = strings.ReplaceAll(raw, `\"Path\":\"`+paths1410Doc, `\"Path\":\"`+inner)
	return strings.ReplaceAll(raw, paths1410Doc, jsonPath)
}

// agy 1.4.12 presents a backslash in a file path as "/" (a failed read of
// back\slash.md showed file:///.../back/slash.md) and cannot open such a file.
// Projecting the path unchanged would claim a presentation the daemon never
// makes, so these steps stay unsupported rather than reading as incomplete.
func TestPaths1410RefusesBackslash(t *testing.T) {
	const jsonPath = `/fixture/docs/back\\slash.md`
	for name, raw := range map[string]string{
		"read":        paths1410ReadRaw,
		"edit":        paths1410EditRaw,
		"failed read": paths1410FailedReadRaw,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := snapshot1216(t, paths1410With(raw, jsonPath)).OfflineJSON(); err == nil {
				t.Fatal("offline export accepted a backslash path")
			}
		})
	}
	t.Run("native failed read presentation", func(t *testing.T) {
		raw := paths1410With(paths1410FailedReadRaw, jsonPath)
		shown := strings.ReplaceAll(paths1410With(paths1410FailedReadShown, jsonPath),
			`"absolutePathUri":"file://`+jsonPath+`"`, `"absolutePathUri":"file:///fixture/docs/back/slash.md"`)
		require1216(t, snapshot1216(t, raw), raw, shown, Unknown)
	})
}

func TestPaths1410InvalidatesPreviousProfile(t *testing.T) {
	previous := strings.SplitN(Profile, "-paths1410", 2)[0]
	if previous == Profile {
		t.Fatal("profile does not name paths1410")
	}
	s := snapshot1216(t, paths1410ReadRaw)
	a, r := s.VerifyResponses([]byte(paths1410ReadRaw), []byte(paths1410ReadShown))
	if r.State != Verified {
		t.Fatal(r)
	}
	a.Profile = previous
	if r := s.CheckAttestation([]byte(paths1410ReadShown), a); r.State != Unknown {
		t.Fatalf("previous profile accepted: %+v", r)
	}
}
