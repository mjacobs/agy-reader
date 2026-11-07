package completeness

import (
	"strings"
	"testing"
)

func TestConfig152ChoicesMatchBothResponses(t *testing.T) {
	const tools = `{"invokeSubagent":{"modelOptions":{"options":[{"inherit":{},"description":"default"},{"namedModel":{"name":"fast","model":{"modelName":"fixture","thinkingLevel":2}}}]}}}`
	for _, location := range []string{"executor", "generator", "static config", "user config"} {
		t.Run(location, func(t *testing.T) {
			raw := options145Raw(location, tools)
			s := options145Snapshot(t, raw)
			a, r := s.VerifyResponses([]byte(raw), []byte(raw))
			if r.State != Verified || len(a.SourceOnly) != 0 {
				t.Fatalf("%+v", r)
			}
			for _, change := range [][2]string{{`"inherit":{},`, ``}, {`"fixture"`, `"changed"`}, {`"thinkingLevel":2`, `"thinkingLevel":3`}} {
				options145RequireBoth(t, s, raw, strings.Replace(raw, change[0], change[1], 1))
			}
		})
	}
}
