package completeness

import (
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestFailedRead149PreservesError(t *testing.T) {
	for _, message := range []string{
		`file "/fixture/corrupt.png" (image/png, 146 bytes) cannot be read: invalid image/png: header found at byte offset 77 after leading non-image bytes`,
		"permission denied",
	} {
		t.Run(message, func(t *testing.T) {
			errorJSON, err := json.Marshal(map[string]string{"shortError": message, "fullError": message})
			if err != nil {
				t.Fatal(err)
			}
			replace := func(s string) string {
				s = strings.Replace(s, missingRead147Error, `"error":`+string(errorJSON), 1)
				s = strings.Replace(s, `"stepRenderInfo":{"titlePrefix":"Analyzing","icon":"description"}`, `"stepRenderInfo":{"titlePrefix":"Analyzing","icon":"description","aggregationKey":"files","titleParts":[{"file":{"uri":"file:///fixture/corrupt.png"}}]}`, 1)
				return strings.ReplaceAll(s, "/fixture/missing.md", "/fixture/corrupt.png")
			}
			raw, shown := replace(missingRead147Raw), replace(missingRead147Shown)
			s := snapshot1216(t, raw)
			require1216(t, s, raw, shown, Verified)
			offline, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, raw, string(offline), Verified)
			quoted, _ := json.Marshal(message)
			for name, change := range map[string][2]string{
				"short error": {`"shortError":` + string(quoted), `"shortError":"changed"`},
				"full error":  {`"fullError":` + string(quoted), `"fullError":"changed"`},
				"path":        {`"absolutePathUri":"file:///fixture/corrupt.png"`, `"absolutePathUri":"file:///fixture/other.png"`},
			} {
				t.Run(name, func(t *testing.T) {
					bad := replace1216(t, shown, change[0], change[1])
					require1216(t, s, raw, bad, Incomplete)
				})
			}
			badRaw := replace1216(t, raw, `"titlePrefix":"Analyzing"`, `"titlePrefix":"Changed"`)
			require1216(t, s, badRaw, shown, Incomplete)
		})
	}
}

// Synthetic values follow the agy 1.4.9 empty-list source/default capture.
// An empty list has neither conversation_ids nor subagent_step_indices in
// metadata; it retains the payload with its action and the result text.
const emptySubagents149Result = `"result":"You have 0 active subagent(s):\n[]","fullOutputUri":"file:///fixture/steps/2/output.txt","payload":{"@type":"type.googleapis.com/exa.cortex.core.tools.ManageSubagents","action":"list"},"stepRenderInfo":{"titlePrefix":"Found","icon":"robot_2","aggregationKey":"actions","titleParts":[{"text":"subagents"}]}`
const emptySubagents149Raw = `{"cascadeId":"fixture-parent","steps":[{` + manage144Common + `,"generic":{"args":{"Action":"list"},"result":{` + emptySubagents149Result + `}}}]}`
const emptySubagents149Shown = `{"cascadeId":"fixture-parent","steps":[{` + manage144Common + `,"generic":{"args":{"Action":"list"},"result":{` + emptySubagents149Result + `,"metadata":{"action":"list"}}}}]}`

func TestEmptySubagents149(t *testing.T) {
	s := snapshot1216(t, emptySubagents149Raw)
	require1216(t, s, emptySubagents149Raw, emptySubagents149Shown, Verified)
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, emptySubagents149Raw, string(offline), Verified)
	m := trajectory1216(t, emptySubagents149Raw)
	before := proto.Clone(m)
	if _, err := project(m); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(m, before) {
		t.Fatal("projection mutated source")
	}
	for name, change := range map[string][2]string{
		"missing action metadata": {`,"metadata":{"action":"list"}`, ""},
		"spurious child":          {`"metadata":{"action":"list"}`, `"metadata":{"action":"list","conversation_ids":"fixture-child"}`},
		"spurious state":          {`"metadata":{"action":"list"}`, `"metadata":{"action":"list","subagent_step_indices":"{}"}`},
		"changed result":          {`0 active`, `1 active`},
	} {
		t.Run(name, func(t *testing.T) {
			bad := replace1216(t, emptySubagents149Shown, change[0], change[1])
			require1216(t, s, emptySubagents149Raw, bad, Incomplete)
		})
	}
	for name, change := range map[string][2]string{
		"different action":  {`"action":"list"`, `"action":"kill"`},
		"unpaired state":    {`"action":"list"}`, `"action":"list","subagents":[{"conversationId":"fixture-child","stepIndex":-1}]}`},
		"extra argument":    {`"args":{`, `"args":{"ConversationIds":"[]",`},
		"existing metadata": {`"result":{"result":`, `"result":{"metadata":{"action":"list"},"result":`},
	} {
		t.Run(name, func(t *testing.T) {
			bad := strings.ReplaceAll(emptySubagents149Raw, change[0], change[1])
			if bad == emptySubagents149Raw {
				t.Fatal("mutation missed")
			}
			if _, err := snapshot1216(t, bad).OfflineJSON(); err == nil {
				t.Fatal("accepted unqualified empty list")
			}
		})
	}
}
