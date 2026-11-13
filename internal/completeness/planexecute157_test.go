package completeness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
)

func TestPlanExecute157RetainedAndCompared(t *testing.T) {
	for _, config := range []string{`{}`, `{"executionModel":0}`, `{"executionModel":235}`} {
		for _, metadata := range []string{
			`"generatorMetadata":[{"plannerConfig":{"planExecuteConfig":%s}}]`,
			`"executorMetadatas":[{"cascadeConfig":{"plannerConfig":{"planExecuteConfig":%s}}}]`,
		} {
			raw := `{"cascadeId":"fixture","metadata":{},` + fmt.Sprintf(metadata, config) + `,"steps":[]}`
			path := filepath.Join(t.TempDir(), "source.db")
			testfixture.Source(t, path, []byte(raw))
			s, err := ReadSnapshot(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			if a, r := s.VerifyResponses([]byte(raw), []byte(raw)); r.State != Verified || len(a.SourceOnly) != 0 {
				t.Fatalf("configuration did not compare normally: %+v", r)
			}
			changes := []string{
				strings.Replace(raw, `"planExecuteConfig":`+config, `"planExecuteConfig":{"executionModel":269}`, 1),
				strings.Replace(raw, `{"planExecuteConfig":`+config+`}`, `{}`, 1),
			}
			if config != `{}` {
				changes = append(changes, strings.Replace(raw, config, `{}`, 1))
			}
			for _, changed := range changes {
				if r := s.CompareFullRaw([]byte(changed)); r.State != Incomplete {
					t.Fatalf("changed configuration passed full comparison: %+v", r)
				}
				if r := s.ComparePresentation([]byte(changed)); r.State != Incomplete {
					t.Fatalf("changed configuration passed presentation comparison: %+v", r)
				}
			}
			exported, err := s.OfflineJSON()
			if err != nil || !bytes.Contains(exported, []byte(`"planExecuteConfig"`)) {
				t.Fatalf("offline export lost configuration: %v", err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(exported, &fields); err != nil {
				t.Fatal(err)
			}
			delete(fields, "agyReader")
			offline, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			if r := s.CompareFullRaw(offline); r.State != Verified {
				t.Fatalf("offline export changed configuration or presence: %+v", r)
			}
		}
	}
}

func TestPlanExecute157StrictWire(t *testing.T) {
	for _, value := range []uint64{0, 235, 1669} {
		if _, err := decode("exa.cortex_pb.CascadePlannerConfig", bytesField(60, options145Varint(1, value)...)); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range map[string][]byte{
		"unknown nested field":  bytesField(60, options145Varint(99, 1)...),
		"unknown model":         bytesField(60, options145Varint(1, 999999)...),
		"wrong nested wire":     bytesField(60, bytesField(1, 'x')...),
		"wrong outer wire":      options145Varint(60, 1),
		"unknown planner field": options145Varint(9999, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decode("exa.cortex_pb.CascadePlannerConfig", data); err == nil {
				t.Fatal("unsupported configuration accepted")
			}
		})
	}
}
