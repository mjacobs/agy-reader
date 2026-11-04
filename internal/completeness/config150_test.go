package completeness

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestConfig150RetainsAndComparesBothResponses(t *testing.T) {
	const tools = `{"antigravityBrowser":{"enableBrowserSubagentWebview":false},"invokeSubagent":{"modelOptions":{"options":[{"name":"fast","model":{"modelName":"fixture-model","thinkingLevel":2},"description":"fixture"}],"disableInherit":false}}}`
	for _, location := range []string{"executor", "generator", "static config", "user config"} {
		t.Run(location, func(t *testing.T) {
			raw := options145Raw(location, tools)
			s := options145Snapshot(t, raw)
			a, r := s.VerifyResponses([]byte(raw), []byte(raw))
			if r.State != Verified || len(a.SourceOnly) != 0 {
				t.Fatalf("%+v", r)
			}
			if r := s.CheckAttestation([]byte(raw), a); r.State != Verified {
				t.Fatal(r)
			}
			for _, change := range [][2]string{
				{`"enableBrowserSubagentWebview":false`, ``},
				{`"enableBrowserSubagentWebview":false`, `"enableBrowserSubagentWebview":true`},
				{`"disableInherit":false`, `"disableInherit":true`},
				{`,"disableInherit":false`, ``},
				{`"fixture-model"`, `"different-model"`},
				{`"thinkingLevel":2`, `"thinkingLevel":3`},
				{`"name":"fast"`, `"name":"slow"`},
				{`"description":"fixture"`, `"description":"changed"`},
			} {
				options145RequireBoth(t, s, raw, strings.Replace(raw, change[0], change[1], 1))
			}
		})
	}
}

func TestConfig150RejectsUnknownNestedFields(t *testing.T) {
	for _, name := range []string{"ModelCatalogKey", "SubagentModelOption", "SubagentModelOptions", "ModelTierConfig", "AntigravityBrowserToolConfig", "InvokeSubagentToolConfig"} {
		t.Run(name, func(t *testing.T) {
			m, err := newMessage(protoreflect.FullName("exa.cortex_pb." + name))
			if err != nil {
				t.Fatal(err)
			}
			m.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 999, protowire.VarintType), 1))
			if err := normalize(m.ProtoReflect(), 0); err == nil {
				t.Fatal("unknown field accepted")
			}
		})
	}
}

func TestConfig150ModelTiers(t *testing.T) {
	raw := `{"cascadeId":"tiers","metadata":{},"executorMetadatas":[{"cascadeConfig":{"modelTiers":{"flashLite":{"modelName":"lite"},"flash":{"modelName":"fast","thinkingLevel":2},"pro":{"modelName":"deep"}}}}]}`
	s := options145Snapshot(t, raw)
	a, r := s.VerifyResponses([]byte(raw), []byte(raw))
	if r.State != Verified || len(a.SourceOnly) != 0 {
		t.Fatal(r)
	}
	for _, change := range [][2]string{{`"deep"`, `"other"`}, {`"thinkingLevel":2`, `"thinkingLevel":3`}, {`"modelName":"lite"`, ``}} {
		options145RequireBoth(t, s, raw, strings.Replace(raw, change[0], change[1], 1))
	}
}
