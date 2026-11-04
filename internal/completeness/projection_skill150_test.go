package completeness

import (
	"encoding/json"
	"strings"
	"testing"
)

func skill150Pair(content string) (string, string) {
	raw, shown := skill144Pair("", "", skill144Path, true)
	raw = strings.ReplaceAll(raw, `"startLine":2`, `"startLine":1`)
	raw = strings.ReplaceAll(raw, `"StartLine":"2"`, `"StartLine":"1"`)
	shown = strings.ReplaceAll(shown, `"startLine":1,`, ``)
	b, _ := json.Marshal(content)
	raw = strings.ReplaceAll(raw, `"fixture result"`, string(b))
	shown = strings.ReplaceAll(shown, `"fixture result"`, string(b))
	return raw, shown
}

func TestSkill150SourceDerivedMetadata(t *testing.T) {
	raw, old := skill150Pair("Showing lines 1-4 of 8\n1: ---\n2: name: sample\n3: description: A sample skill.\n4: ---")
	current := strings.Replace(old, `"isSkillFile":true,`, `"isSkillFile":true,"skillMetadata":{"name":"sample","description":"A sample skill."},`, 1)
	s := snapshot1216(t, raw)
	require1216(t, s, raw, old, Verified)
	require1216(t, s, raw, current, Verified)
	for _, change := range [][2]string{
		{`"name":"sample"`, `"name":"other"`},
		{`"description":"A sample skill."`, `"description":"different"`},
		{`"name":"sample",`, ``},
		{`"name":"sample"`, `"name":"sample","publisher":"untrusted"`},
	} {
		require1216(t, s, raw, strings.Replace(current, change[0], change[1], 1), Incomplete)
	}
	// Changing the saved response content in full RPC remains a failure even
	// when default RPC still carries the expected derived metadata.
	require1216(t, s, strings.Replace(raw, "A sample skill.", "Changed source.", 1), current, Incomplete)
}

func TestSkill150UnsupportedFrontmatterCannotSupplyMetadata(t *testing.T) {
	for _, content := range []string{
		"Showing lines 2-4 of 8\n2: name: sample\n3: description: text\n4: ---",
		"Showing lines 1-4 of 8\n1: ---\n2: name: sample\n3: name: duplicate\n4: ---",
		"Showing lines 1-4 of 8\n1: ---\n2: name: sample\n3: description: >\n4: ---",
		"Showing lines 1-4 of 8\n1: ---\n2: name: sample\n3: description: text # comment\n4: ---",
		"Showing lines 1-4 of 8\n1: ---\n2: name: sample\n3: description: text\n4: missing terminator",
	} {
		raw, old := skill150Pair(content)
		current := strings.Replace(old, `"isSkillFile":true,`, `"isSkillFile":true,"skillMetadata":{"name":"sample","description":"text"},`, 1)
		require1216(t, snapshot1216(t, raw), raw, current, Incomplete)
	}
}

func TestSkill150RejectsMixedTrajectoryMetadata(t *testing.T) {
	raw, old := skill150Pair("Showing lines 1-4 of 8\n1: ---\n2: name: sample\n3: description: A sample skill.\n4: ---")
	double := func(s string) string {
		var x map[string]json.RawMessage
		if err := json.Unmarshal([]byte(s), &x); err != nil {
			t.Fatal(err)
		}
		var steps []json.RawMessage
		if err := json.Unmarshal(x["steps"], &steps); err != nil {
			t.Fatal(err)
		}
		x["steps"], _ = json.Marshal(append(steps, steps[0]))
		b, _ := json.Marshal(x)
		return string(b)
	}
	raw, old = double(raw), double(old)
	mixed := strings.Replace(old, `"isSkillFile":true,`, `"isSkillFile":true,"skillMetadata":{"name":"sample","description":"A sample skill."},`, 1)
	require1216(t, snapshot1216(t, raw), raw, mixed, Incomplete)
}
