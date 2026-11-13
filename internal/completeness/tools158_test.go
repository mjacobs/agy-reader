package completeness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
)

func tools158Pair(t *testing.T, name string) (string, string) {
	t.Helper()
	read := func(kind string) string {
		b, err := os.ReadFile(filepath.Join("testdata", "tools158", name+"."+kind+".json"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	return read("full"), read("default")
}

func TestTools158CapturedPresentations(t *testing.T) {
	for _, name := range []string{"define", "mcp-approved", "mcp-allowed", "mcp-denied", "missing-read", "missing-lines", "inverted-read", "hook-failure", "canceled-command"} {
		t.Run(name, func(t *testing.T) {
			raw, shown := tools158Pair(t, name)
			path := filepath.Join(t.TempDir(), "source.db")
			db := testfixture.Source(t, path, []byte(raw))
			testfixture.Exec(t, db, "UPDATE steps SET render_info=NULL,permissions=NULL")
			s, err := ReadSnapshot(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, raw, shown, Verified)
			offline, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, raw, string(offline), Verified)
			// Each RPC representation is independently required to match.
			changed := strings.Replace(raw, "call-fixture", "call-changed", 1)
			require1216(t, s, changed, shown, Incomplete)
			changed = strings.Replace(shown, "call-fixture", "call-changed", 1)
			require1216(t, s, raw, changed, Incomplete)
			bad := mutateJSON158(t, raw, func(step map[string]any) {
				step["generic"].(map[string]any)["args"].(map[string]any)["Future"] = "unsupported"
			})
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
}

func TestTools158RejectsUnqualifiedShapes(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"define", `"EnableMCPTools": "false"`, `"EnableMCPTools": "sometimes"`},
		{"define", `"Description": "Greets users"`, `"Description": "Other description"`},
		{"define", `"agentConfig": {`, `"toolNames": ["run_command"], "agentConfig": {`},
		{"define", `"agentConfig": {`, `"agentPath": "/fixture/agent.md", "agentConfig": {`},
		{"define", `"name": "helper"`, `"name": "other"`},
		{"define", `CORTEX_STEP_STATUS_DONE`, `CORTEX_STEP_STATUS_CANCELED`},
		{"mcp-approved", `"allow": true`, `"allow": false`},
		{"mcp-approved", `TYPE_EFFECTIVE_GRANT`, `TYPE_DEFAULT_FALLBACK`},
		{"mcp-approved", `echo/ping`, `echo/other`},
		{"mcp-approved", `"action": "mcp"`, `"action": "command"`},
		{"mcp-approved", `"serverName": "echo"`, `"serverName": "other"`},
		{"mcp-approved", `"resultString": "PONG:asked158"`, `"resultUri": "file:///fixture/result.txt"`},
		{"mcp-allowed", `"resultString":`, `"progress": 1, "resultString":`},
		{"mcp-denied", `Matches user-configured deny rule.`, `Another failure.`},
		{"missing-read", `CORTEX_STEP_STATUS_ERROR`, `CORTEX_STEP_STATUS_CANCELED`},
		{"missing-lines", `"EndLine": "100"`, `"EndLine": "0"`},
		{"inverted-read", `StartLine 121 is greater than EndLine 120`, `StartLine 120 is greater than EndLine 121`},
		{"canceled-command", `"WaitMsBeforeAsync": "10000"`, `"WaitMsBeforeAsync": "20000"`},
		{"hook-failure", `unknown field \"systemMessage\"`, `unknown field \"other\"`},
	} {
		t.Run(tc.name+tc.before, func(t *testing.T) {
			raw, shown := tools158Pair(t, tc.name)
			bad := replace1216(t, raw, tc.before, tc.after)
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
	// Both fields of an MCP approval record are required.
	raw, shown := tools158Pair(t, "mcp-approved")
	for _, field := range []string{"permissions", "completedInteractions"} {
		bad := mutateJSON158(t, raw, func(step map[string]any) { delete(step, field) })
		require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
	}
}

func TestTools158MetadataSize(t *testing.T) {
	raw := `{"cascadeId":"fixture","metadata":{},"generatorMetadata":[{}],"steps":[]}`
	for _, size := range []int{0, 1, -1, 999} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "source.db")
			db := testfixture.Source(t, path, []byte(raw))
			// An empty protobuf record has length zero. Nonzero markers must match.
			testfixture.Exec(t, db, "UPDATE gen_metadata SET size=?", size)
			s, err := ReadSnapshot(t.Context(), path)
			if size != 0 {
				if err == nil {
					t.Fatal("invalid size accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, raw, raw, Verified)
		})
	}
	// Real nonempty metadata accepts either its length or the new zero marker.
	raw = `{"cascadeId":"fixture","metadata":{},"generatorMetadata":[{"plannerConfig":{"planExecuteConfig":{}}}],"steps":[]}`
	path := filepath.Join(t.TempDir(), "source.db")
	db := testfixture.Source(t, path, []byte(raw))
	testfixture.Exec(t, db, "UPDATE gen_metadata SET size=0")
	s, err := ReadSnapshot(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, raw, raw, Verified)
}

func TestTools158InvalidatesPreviousAttestations(t *testing.T) {
	s, _ := fixture(t)
	a, r := s.VerifyResponses([]byte(syntheticRaw), []byte(syntheticPresentation))
	if r.State != Verified {
		t.Fatal(r)
	}
	a.Profile = strings.TrimSuffix(Profile, "-tools158")
	if a.Profile == Profile {
		t.Fatal("profile not advanced")
	}
	if r := s.CheckAttestation([]byte(syntheticPresentation), a); r.State != Unknown {
		t.Fatal(r)
	}
}

func mutateJSON158(t *testing.T, raw string, change func(map[string]any)) string {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	change(data["steps"].([]any)[0].(map[string]any))
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestTools158PopulatedPermissionColumnMustAgree(t *testing.T) {
	raw, shown := tools158Pair(t, "mcp-approved")
	for _, data := range [][]byte{{}, {0x0a, 0x00}, {0x80}} {
		path := filepath.Join(t.TempDir(), "source.db")
		db := testfixture.Source(t, path, []byte(raw))
		testfixture.Exec(t, db, "UPDATE steps SET permissions=?", data)
		if s, err := ReadSnapshot(t.Context(), path); err == nil {
			a, r := s.VerifyResponses([]byte(raw), []byte(shown))
			t.Fatalf("populated disagreeing column accepted: %+v %+v", a, r)
		}
	}
}
