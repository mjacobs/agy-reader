package completeness

import (
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Synthetic values in the shape independently observed in SQLite, full and
// ordinary RPCs for four completed ACP searches. The native presentation
// has no generic tool-output wrapper.
const search1518Metadata = `"status":"CORTEX_STEP_STATUS_DONE",
	"metadata":{"toolCall":{"name":"search_web","argumentsJson":"{\"query\":\"fixture query\"}"}}`
const search1518Fields = `"query":"fixture query","summary":"fixture summary",
	"webDocuments":[{"documentId":"1","title":"First","url":"https://example.org/1"},
	                {"documentId":"2","title":"Second","url":"https://example.org/2"}],
	"thirdPartyConfig":{"provider":"THIRD_PARTY_WEB_SEARCH_PROVIDER_GEMINI"}`
const search1518Full = `{"cascadeId":"fixture","metadata":{},"steps":[{
	"type":"CORTEX_STEP_TYPE_GENERIC",` + search1518Metadata + `,
	"generic":{"args":{"query":"fixture query"},"result":{
		"result":"Created At: 2026-01-01T00:00:00Z\nCompleted At: 2026-01-01T00:00:01Z\nThe search for \"fixture query\" returned the following summary:\nfixture summary\n\nNote: Source URLs are HTTP redirects; check where they resolve if you need the direct link.",
		"fullOutputUri":"file:///fixture/output.txt",
		"payload":{"@type":"type.googleapis.com/exa.cortex.core.tools.SearchWeb",` + search1518Fields + `},
		"stepRenderInfo":{"titlePrefix":"Searched","icon":"search","aggregationKey":"web","titleParts":[{"text":"fixture query"}]}
	}}}]}`
const search1518Shown = `{"cascadeId":"fixture","metadata":{},"steps":[{
	"type":"CORTEX_STEP_TYPE_SEARCH_WEB",` + search1518Metadata + `,
	"searchWeb":{"searchType":"SEARCH_WEB_TYPE_WEB",` + search1518Fields + `}}]}`

func TestTypedSearchWeb1518(t *testing.T) {
	source := snapshot1216(t, search1518Full)
	require1216(t, source, search1518Full, search1518Shown, Verified)
	offline, err := source.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, source, search1518Full, string(offline), Verified)
	for _, changed := range [][2]string{
		{`"title":"Second"`, `"title":"First"`},
		{`"documentId":"1"`, `"documentId":"different"`},
		{`"url":"https://example.org/1"`, `"url":"https://example.org/x"`},
		{`"summary":"fixture summary"`, `"summary":"other"`},
	} {
		require1216(t, source, replace1216(t, search1518Full, changed[0], changed[1]), search1518Shown, Incomplete)
		require1216(t, source, search1518Full, replace1216(t, search1518Shown, changed[0], changed[1]), Incomplete)
	}
	require1216(t, source, search1518Full, replace1216(t, search1518Shown, `"searchType":"SEARCH_WEB_TYPE_WEB",`, ""), Incomplete)
	require1216(t, source, search1518Full, replace1216(t, search1518Shown, "SEARCH_WEB_TYPE_WEB", "SEARCH_WEB_TYPE_FUTURE"), Unknown)
}

func TestTypedSearchWeb1518RefusesUnqualifiedWrappers(t *testing.T) {
	for name, changed := range map[string][2]string{
		"wrong tool":               {`"name":"search_web"`, `"name":"different"`},
		"active":                   {"CORTEX_STEP_STATUS_DONE", "CORTEX_STEP_STATUS_RUNNING"},
		"domain filter":            {`"summary":"fixture summary"`, `"domain":"example.org","summary":"fixture summary"`},
		"wrong query":              {`"args":{"query":"fixture query"}`, `"args":{"query":"other"}`},
		"wrong summary":            {`"summary":"fixture summary"`, `"summary":"other"`},
		"unexpected metadata":      {`"fullOutputUri":`, `"metadata":{"future":"x"},"fullOutputUri":`},
		"unobserved document":      {`"title":"First"`, `"title":"First","text":"new content"`},
		"unobserved provider":      {`"thirdPartyConfig":{"provider":"THIRD_PARTY_WEB_SEARCH_PROVIDER_GEMINI"}`, `"thirdPartyConfig":{}`},
		"rendered query disagrees": {`"text":"fixture query"`, `"text":"other"`},
	} {
		t.Run(name, func(t *testing.T) {
			full := replace1216(t, search1518Full, changed[0], changed[1])
			source := snapshot1216(t, full)
			require1216(t, source, full, search1518Shown, Unknown)
			if _, err := source.OfflineJSON(); err == nil {
				t.Fatal("unqualified search wrapper accepted for offline export")
			}
		})
	}
}

func TestTypedSearchWeb1518RejectsUnknownPersistedField(t *testing.T) {
	trajectory := trajectory1216(t, search1518Full)
	step := tool143Get(trajectory.ProtoReflect(), "steps").List().Get(0).Message()
	result := tool143Get(tool143Get(step, "generic").Message(), "result").Message()
	payload := tool143Get(result, "payload").Message()
	data := append([]byte(nil), tool143Get(payload, "value").Bytes()...)
	data = protowire.AppendVarint(protowire.AppendTag(data, 999, protowire.VarintType), 1)
	tool143Set(payload, "value", protoreflect.ValueOfBytes(data))
	if _, err := finishSnapshot(trajectory); err == nil {
		t.Fatal("unknown SearchWeb wire field accepted")
	}
}
