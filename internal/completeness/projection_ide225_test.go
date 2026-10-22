package completeness

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// These synthetic source/presentation pairs follow the independently captured
// Antigravity IDE 2.25.0 responses. The IDE keeps modern read_file permission
// records verbatim, accepts a zero ByteOffset, and presents FileEdit artifact
// fields in codeAction. Expected presentations are not produced by project().
const (
	ide225ArtifactArg  = `"ArtifactMetadata":"{\"RequestFeedback\":false,\"Summary\":\"Fixture plan\",\"UserFacing\":true}"`
	ide225ArtifactMeta = `"artifactMetadata":{"summary":"Fixture plan","updatedAt":"2026-01-01T00:00:01Z","userFacing":true}`
	ide225Permissions  = `"permissions":{"permissionGrants":[{"resource":{"action":"read_file","target":"/fixture/external/notes.txt"},"allow":true,"scope":"PERMISSION_SCOPE_PROJECT","action":"read_file","target":"/fixture/external/notes.txt"}]}`
	ide225Interactions = `"completedInteractions":[{"request":{"permission":{"resource":{"action":"read_file","target":"/fixture/external/notes.txt"},"actionDescription":"Read fixture notes","triggerSource":{"type":"TYPE_DEFAULT_FALLBACK"}}},"response":{"trajectoryId":"fixture-parent","stepIndex":1,"permission":{"allow":true,"scope":"PERMISSION_SCOPE_PROJECT","persistGrants":{"allow":["read_file(/fixture/external/notes.txt)"]}}}}]`
)

func ide225ReadPair(permission bool) (string, string) {
	f := tool143Fixtures[2]
	f.args += `,"ByteOffset":"0"`
	raw, shown := f.pair()
	call := `"toolCall":{"name":"read_file","argumentsJson":"{\"Path\":\"/fixture/notes.txt\",\"StartLine\":2,\"EndLine\":4,\"ByteOffset\":0}"}`
	raw = strings.Replace(raw, `"toolCall":{"name":"read_file"}`, call, 1)
	shown = strings.Replace(shown, `"toolCall":{"name":"read_file"}`, call, 1)
	if permission {
		// The retained grant can name a different path. The modern mapping
		// retains this whole record rather than synthesizing or reinterpreting it.
		record := ide225Permissions + `,` + ide225Interactions + `,`
		raw = strings.Replace(raw, `"status":`, record+`"status":`, 1)
		shown = strings.Replace(shown, `"status":`, record+`"status":`, 1)
	}
	return raw, shown
}

func ide225ArtifactPair(create bool) (string, string) {
	index := 4
	if create {
		index = 3
	}
	f := tool143Fixtures[index]
	f.payload += `,"isArtifactFile":true`
	f.presented = strings.Replace(f.presented, `"codeAction":{`, `"codeAction":{"isArtifactFile":true,`, 1)
	if create {
		f.payload += `,` + ide225ArtifactMeta
		f.args += `,` + ide225ArtifactArg
		f.presented = strings.Replace(f.presented, `"codeAction":{`, `"codeAction":{`+ide225ArtifactMeta+`,`, 1)
	}
	raw, shown := f.pair()
	if create {
		call := `"toolCall":{"name":"write_file","argumentsJson":"{\"Path\":\"/fixture/empty.txt\",\"Content\":\"\",\"Overwrite\":true,\"ArtifactMetadata\":{\"RequestFeedback\":false,\"Summary\":\"Fixture plan\",\"UserFacing\":true}}"}`
		raw = strings.Replace(raw, `"toolCall":{"name":"write_file"}`, call, 1)
		shown = strings.Replace(shown, `"toolCall":{"name":"write_file"}`, call, 1)
	}
	return raw, shown
}

func ide225Pairs() map[string][2]string {
	readRaw, readShown := ide225ReadPair(false)
	permissionRaw, permissionShown := ide225ReadPair(true)
	createRaw, createShown := ide225ArtifactPair(true)
	editRaw, editShown := ide225ArtifactPair(false)
	return map[string][2]string{
		"read zero offset":        {readRaw, readShown},
		"read project permission": {permissionRaw, permissionShown},
		"read global permission": {
			strings.ReplaceAll(permissionRaw, "PERMISSION_SCOPE_PROJECT", "PERMISSION_SCOPE_GLOBAL"),
			strings.ReplaceAll(permissionShown, "PERMISSION_SCOPE_PROJECT", "PERMISSION_SCOPE_GLOBAL"),
		},
		"artifact create": {createRaw, createShown},
		"artifact edit":   {editRaw, editShown},
	}
}

func TestProjectionIDE225(t *testing.T) {
	for name, pair := range ide225Pairs() {
		t.Run(name, func(t *testing.T) {
			raw, shown := pair[0], pair[1]
			s := snapshot1216(t, raw)
			before := proto.Clone(s.trajectory)
			require1216(t, s, raw, shown, Verified)
			if !proto.Equal(before, s.trajectory) {
				t.Fatal("projection changed persisted source")
			}
			// Wrapper-only text remains bound by the full-source comparison.
			require1216(t, s, strings.Replace(raw, "fixture result", "changed result", 1), shown, Incomplete)
			require1216(t, s, raw, `{"cascadeId":"fixture-parent","metadata":{},"steps":[]}`, Incomplete)
		})
	}
}

func TestProjectionIDE225ReadOffsetBounds(t *testing.T) {
	raw, shown := ide225ReadPair(false)
	for _, value := range []string{"1", "-1", "00", "0.0", ""} {
		t.Run("offset="+value, func(t *testing.T) {
			bad := replace1216(t, raw, `"ByteOffset":"0"`, `"ByteOffset":"`+value+`"`)
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
	for name, change := range map[string][2]string{
		"retained offset differs": {`\"ByteOffset\":0`, `\"ByteOffset\":1`},
		"retained offset missing": {`,\"ByteOffset\":0`, ``},
		"retained path differs":   {`\"Path\":\"/fixture/notes.txt\"`, `\"Path\":\"/fixture/other.txt\"`},
	} {
		t.Run(name, func(t *testing.T) {
			bad := replace1216(t, raw, change[0], change[1])
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
}

func TestProjectionIDE225ArtifactBounds(t *testing.T) {
	raw, shown := ide225ArtifactPair(true)
	for name, change := range map[string][2]string{
		"metadata without flag":        {`"isArtifactFile":true`, `"isArtifactFile":false`},
		"metadata without argument":    {`,` + ide225ArtifactArg, ``},
		"argument without metadata":    {`,` + ide225ArtifactMeta, ``},
		"argument summary differs":     {`\"Summary\":\"Fixture plan\",\"UserFacing\":true}"`, `\"Summary\":\"Other plan\",\"UserFacing\":true}"`},
		"retained summary differs":     {`\"Summary\":\"Fixture plan\",\"UserFacing\":true}}"`, `\"Summary\":\"Other plan\",\"UserFacing\":true}}"`},
		"argument feedback differs":    {`"ArtifactMetadata":"{\"RequestFeedback\":false`, `"ArtifactMetadata":"{\"RequestFeedback\":true`},
		"metadata feedback unobserved": {`"artifactMetadata":{`, `"artifactMetadata":{"requestFeedback":true,`},
		"metadata version unobserved":  {`"artifactMetadata":{`, `"artifactMetadata":{"version":"1",`},
		"metadata created unobserved":  {`"artifactMetadata":{`, `"artifactMetadata":{"createdAt":"2026-01-01T00:00:00Z",`},
	} {
		t.Run(name, func(t *testing.T) {
			bad := replace1216(t, raw, change[0], change[1])
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
	t.Run("edit with metadata", func(t *testing.T) {
		raw, shown := ide225ArtifactPair(false)
		bad := replace1216(t, raw, `"isArtifactFile":true`, `"isArtifactFile":true,`+ide225ArtifactMeta)
		require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
	})
}

func TestProjectionIDE225PermissionBounds(t *testing.T) {
	raw, shown := ide225ReadPair(true)
	for name, change := range map[string][2]string{
		"without permissions":  {ide225Permissions + `,`, ``},
		"without interactions": {ide225Interactions + `,`, ``},
		"empty interactions":   {ide225Interactions, `"completedInteractions":[]`},
		"pending interaction":  {`"status":`, `"requestedInteraction":{"permission":{"reason":"pending"}},"status":`},
	} {
		t.Run(name, func(t *testing.T) {
			bad := replace1216(t, raw, change[0], change[1])
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
	for _, index := range []int{0, 3} {
		f := tool143Fixtures[index]
		t.Run("record on "+f.tool, func(t *testing.T) {
			raw, shown := f.pair()
			record := ide225Permissions + `,` + ide225Interactions + `,`
			raw = strings.Replace(raw, `"status":`, record+`"status":`, 1)
			shown = strings.Replace(shown, `"status":`, record+`"status":`, 1)
			require1216(t, snapshot1216(t, raw), raw, shown, Unknown)
		})
	}
}

func TestProjectionIDE225RefusesUnqualifiedSource(t *testing.T) {
	for name, pair := range ide225Pairs() {
		t.Run(name, func(t *testing.T) {
			raw, shown := pair[0], pair[1]
			for _, status := range []string{"RUNNING", "ERROR", "CANCELED"} {
				t.Run(status, func(t *testing.T) {
					bad := replace1216(t, raw, `"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_`+status+`"`)
					require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
				})
			}
			bad := replace1216(t, raw, `"args":{`, `"args":{"Unexpected":"value",`)
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
			// Unknown protobuf bytes in a known payload must still fail decoding.
			m := trajectory1216(t, raw).ProtoReflect()
			step := tool143Get(m, "steps").List().Get(0).Message()
			payload := tool143Get(tool143Get(tool143Get(step, "generic").Message(), "result").Message(), "payload").Message()
			data := append([]byte(nil), tool143Get(payload, "value").Bytes()...)
			data = protowire.AppendVarint(protowire.AppendTag(data, 9999, protowire.VarintType), 1)
			tool143Set(payload, "value", protoreflect.ValueOfBytes(data))
			if _, err := finishSnapshot(m.Interface()); err == nil {
				t.Fatal("unknown nested payload field accepted")
			}
		})
	}
}

func TestProjectionIDE225PresentedFields(t *testing.T) {
	for _, tc := range []struct {
		name, fixture, before, after string
	}{
		{"grant dropped", "read project permission", ide225Permissions + `,`, ``},
		{"interactions dropped", "read project permission", ide225Interactions + `,`, ``},
		{"grant scope changed", "read project permission", "PERMISSION_SCOPE_PROJECT", "PERMISSION_SCOPE_GLOBAL"},
		{"grant target changed", "read project permission", "/fixture/external/notes.txt", "/fixture/external/other.txt"},
		{"read offset changed", "read zero offset", `\"ByteOffset\":0`, `\"ByteOffset\":1`},
		{"created artifact flag dropped", "artifact create", `"isArtifactFile":true,`, ``},
		{"edited artifact flag dropped", "artifact edit", `"isArtifactFile":true,`, ``},
		{"artifact metadata dropped", "artifact create", ide225ArtifactMeta + `,`, ``},
		{"artifact timestamp changed", "artifact create", "2026-01-01T00:00:01Z", "2026-01-01T00:00:03Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pair := ide225Pairs()[tc.fixture]
			require1216(t, snapshot1216(t, pair[0]), pair[0], replace1216(t, pair[1], tc.before, tc.after), Incomplete)
		})
	}
}

func TestOfflineJSONIDE225(t *testing.T) {
	for name, pair := range ide225Pairs() {
		t.Run(name, func(t *testing.T) {
			s := snapshot1216(t, pair[0])
			before, digest := proto.Clone(s.trajectory), s.Digest
			data, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			var got, want map[string]any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(pair[1]), &want); err != nil {
				t.Fatal(err)
			}
			reader, ok := got["agyReader"].(map[string]any)
			if !ok || reader["offline"] == nil || reader["completeness"] != nil {
				t.Fatal("incorrect offline provenance")
			}
			delete(got, "agyReader")
			if !reflect.DeepEqual(got, want) {
				t.Fatal("offline export differs from independently specified IDE presentation")
			}
			again, err := s.OfflineJSON()
			if err != nil || !bytes.Equal(data, again) {
				t.Fatal("offline export is not repeatable")
			}
			if s.Digest != digest || !proto.Equal(before, s.trajectory) {
				t.Fatal("offline export changed persisted source")
			}
		})
	}
}
