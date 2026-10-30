package completeness

import (
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func subagents148Pair(t *testing.T, action string) (string, string) {
	t.Helper()
	args := map[string]any{"Action": action}
	genericArgs := map[string]string{"Action": action}
	state := map[string]any{"conversationId": "fixture-child"}
	meta := map[string]string{"action": action, "conversation_ids": "fixture-child"}
	if action == "list" {
		state["stepIndex"] = -1
		meta["subagent_step_indices"] = `{"fixture-child":-1}`
	} else {
		args["ConversationIds"] = []string{"fixture-child"}
		genericArgs["ConversationIds"] = `["fixture-child"]`
		meta["killed_conversation_ids"] = `["fixture-child"]`
	}
	call, _ := json.Marshal(args)
	result := map[string]any{"result": "fixture result", "payload": map[string]any{"@type": "type.googleapis.com/exa.cortex.core.tools.ManageSubagents", "action": action, "conversationIds": []string{"fixture-child"}, "subagents": []any{state}}}
	step := map[string]any{"type": "CORTEX_STEP_TYPE_GENERIC", "status": "CORTEX_STEP_STATUS_DONE", "metadata": map[string]any{"toolCall": map[string]any{"name": "manage_subagents", "argumentsJson": string(call)}}, "generic": map[string]any{"args": genericArgs, "result": result}}
	trajectory := map[string]any{"cascadeId": "fixture-parent", "metadata": map[string]any{}, "steps": []any{step}}
	raw, _ := json.Marshal(trajectory)
	result["metadata"] = meta
	shown, _ := json.Marshal(trajectory)
	return string(raw), string(shown)
}

func TestSubagents148(t *testing.T) {
	for _, action := range []string{"list", "kill"} {
		t.Run(action, func(t *testing.T) {
			raw, shown := subagents148Pair(t, action)
			s := snapshot1216(t, raw)
			require1216(t, s, raw, shown, Verified)
			offline, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, raw, string(offline), Verified)
			// Both retained state and synthesized metadata must remain exact.
			bad := strings.Replace(shown, `"conversation_ids":"fixture-child"`, `"conversation_ids":"other"`, 1)
			require1216(t, s, raw, bad, Incomplete)
			for _, change := range [][2]string{
				{`"conversationId":"fixture-child"`, `"conversationId":"other"`},
				{`"CORTEX_STEP_STATUS_DONE"`, `"CORTEX_STEP_STATUS_ERROR"`},
				{`"result":"fixture result"`, `"result":"fixture result","metadata":{"unexpected":"value"}`},
			} {
				bad := replace1216(t, raw, change[0], change[1])
				if _, err := snapshot1216(t, bad).OfflineJSON(); err == nil {
					t.Fatalf("accepted %s", change[0])
				}
			}
		})
	}
	raw, _ := subagents148Pair(t, "list")
	for _, value := range []string{"0", "1", "-2"} {
		bad := replace1216(t, raw, `"stepIndex":-1`, `"stepIndex":`+value)
		if _, err := snapshot1216(t, bad).OfflineJSON(); err == nil {
			t.Fatal("accepted unqualified index", value)
		}
	}
}

func TestSubagentState148PreservesOptionalZero(t *testing.T) {
	for _, wire := range [][]byte{nil, {0x10, 0x00}, {0x10, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}} {
		m, err := decode("exa.cortex.core.tools.ManageSubagents.SubagentState", wire)
		if err != nil {
			t.Fatal(err)
		}
		p := m.ProtoReflect()
		field := p.Descriptor().Fields().ByName("step_index")
		if p.Has(field) != (len(wire) > 0) {
			t.Fatal("lost optional presence")
		}
		if len(wire) == 2 && p.Get(field).Int() != 0 {
			t.Fatal("lost explicit zero")
		}
		if len(wire) > 2 && p.Get(field).Int() != -1 {
			t.Fatal("lost negative index")
		}
	}
}

func TestSubagents148UnknownNestedWire(t *testing.T) {
	// Unknown data in the newly declared nested message must still fail closed.
	nested := protowire.AppendTag(nil, 99, protowire.VarintType)
	nested = protowire.AppendVarint(nested, 1)
	payload := protowire.AppendTag(nil, 3, protowire.BytesType)
	payload = protowire.AppendBytes(payload, nested)
	if _, err := decode("exa.cortex.core.tools.ManageSubagents", payload); err == nil {
		t.Fatal("accepted unknown nested field")
	}
}

func TestRejectedDefinition148(t *testing.T) {
	args := map[string]any{"Name": "test:agent", "Description": "fixture", "SystemPrompt": "fixture", "EnableMCPTools": false, "EnableSubagentTools": false, "EnableWriteTools": false}
	call, _ := json.Marshal(args)
	generic := map[string]string{}
	for k, v := range args {
		if s, ok := v.(string); ok {
			generic[k] = s
		} else {
			generic[k] = "false"
		}
	}
	text := `agent name "test:agent" is invalid: it must start with a letter or digit and contain only letters, digits, '_', '-', and '.'`
	step := map[string]any{"type": "CORTEX_STEP_TYPE_GENERIC", "status": "CORTEX_STEP_STATUS_ERROR", "metadata": map[string]any{"toolCall": map[string]any{"name": "define_subagent", "argumentsJson": string(call)}}, "error": map[string]string{"shortError": text, "fullError": text}, "generic": map[string]any{"args": generic}}
	data, _ := json.Marshal(map[string]any{"cascadeId": "fixture", "metadata": map[string]any{}, "steps": []any{step}})
	raw := string(data)
	s := snapshot1216(t, raw)
	require1216(t, s, raw, raw, Verified)
	if _, err := s.OfflineJSON(); err != nil {
		t.Fatal(err)
	}
	for _, change := range [][2]string{{"test:agent", "valid-agent"}, {"is invalid:", "failed:"}, {"CORTEX_STEP_STATUS_ERROR", "CORTEX_STEP_STATUS_CANCELED"}} {
		bad := replace1216(t, raw, change[0], change[1])
		if _, err := snapshot1216(t, bad).OfflineJSON(); err == nil {
			t.Fatal("accepted unqualified failure")
		}
	}
}

func TestArtifactOverwrite148(t *testing.T) {
	raw, shown := ide225ArtifactPair(true)
	raw = strings.ReplaceAll(raw, `,`+ide225ArtifactArg, "")
	raw = strings.ReplaceAll(raw, `,`+ide225ArtifactMeta, "")
	shown = strings.ReplaceAll(shown, ide225ArtifactMeta+`,`, "")
	// Remove the metadata argument from the retained tool call on both sides.
	argument := `,\"ArtifactMetadata\":{\"RequestFeedback\":false,\"Summary\":\"Fixture plan\",\"UserFacing\":true}`
	raw = strings.ReplaceAll(raw, argument, "")
	shown = strings.ReplaceAll(shown, argument, "")
	s := snapshot1216(t, raw)
	require1216(t, s, raw, shown, Verified)
	if _, err := s.OfflineJSON(); err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(raw, `"Overwrite":"true"`, `"Overwrite":"false"`, 1)
	if bad == raw {
		t.Fatal("mutation missed")
	}
	if _, err := snapshot1216(t, bad).OfflineJSON(); err == nil {
		t.Fatal("accepted unqualified artifact")
	}
}

func TestArtifactScratchCreate(t *testing.T) {
	raw, shown := ide225ArtifactPair(true)
	raw = strings.ReplaceAll(raw, `,`+ide225ArtifactArg, "")
	raw = strings.ReplaceAll(raw, `,`+ide225ArtifactMeta, "")
	shown = strings.ReplaceAll(shown, ide225ArtifactMeta+`,`, "")
	// Remove both ArtifactMetadata and Overwrite arguments.
	metaArg := `,\"ArtifactMetadata\":{\"RequestFeedback\":false,\"Summary\":\"Fixture plan\",\"UserFacing\":true}`
	raw = strings.ReplaceAll(raw, metaArg, "")
	shown = strings.ReplaceAll(shown, metaArg, "")
	raw = strings.ReplaceAll(raw, `,"Overwrite":"true"`, "")
	raw = strings.ReplaceAll(raw, `,"Overwrite":true`, "")
	raw = strings.ReplaceAll(raw, `,\"Overwrite\":true`, "")
	shown = strings.ReplaceAll(shown, `,"Overwrite":true`, "")
	shown = strings.ReplaceAll(shown, `,\"Overwrite\":true`, "")
	s := snapshot1216(t, raw)
	require1216(t, s, raw, shown, Verified)
	if _, err := s.OfflineJSON(); err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(raw, `"Path":"/fixture/empty.txt"`, `"Path":"/fixture/other.txt"`, 1)
	if bad == raw {
		t.Fatal("mutation missed")
	}
	if _, err := snapshot1216(t, bad).OfflineJSON(); err == nil {
		t.Fatal("accepted unqualified scratch artifact")
	}
}
