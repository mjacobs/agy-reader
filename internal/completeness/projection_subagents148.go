package completeness

import (
	"encoding/json"
	"errors"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// projectManageSubagents148 covers captured one-child (1.4.8) and two-child
// (1.5.5) list and kill presentations. Other counts need native evidence. State
// stays in the payload; only present step indices enter the metadata JSON.
func projectManageSubagents148(step, generic, result, payload protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported manage_subagents 1.4.8 projection")
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic")) || genericToolName(step) != "manage_subagents" || statusName(step) != "CORTEX_STEP_STATUS_DONE" ||
		!onlyFields(result, fieldSet("result", "payload", "full_output_uri", "step_render_info")) || !argumentsPresented(generic, step) ||
		!onlyFields(payload, fieldSet("action", "conversation_ids", "subagents")) {
		return nil, unsupported
	}
	action, present := genericArg(generic, "Action")
	if !present || tool143Get(payload, "action").String() != action {
		return nil, unsupported
	}
	ids, states := tool143Get(payload, "conversation_ids").List(), tool143Get(payload, "subagents").List()
	// Native 1.4.8 captures have one child; 1.5.5 captures have two.
	if (ids.Len() != 1 && ids.Len() != 2) || states.Len() != ids.Len() {
		return nil, unsupported
	}
	names := make([]string, ids.Len())
	indices := map[string]int64{}
	seen := map[string]bool{}
	for i := range names {
		id, state := ids.Get(i).String(), states.Get(i).Message()
		if id == "" || strings.Contains(id, ",") || seen[id] ||
			!onlyFields(state, fieldSet("conversation_id", "step_index")) || tool143Get(state, "conversation_id").String() != id {
			return nil, unsupported
		}
		seen[id] = true
		names[i] = id
		indexField := state.Descriptor().Fields().ByName("step_index")
		switch action {
		case "list":
			if !genericArgsAllowed(generic, "Action") || !state.Has(indexField) || state.Get(indexField).Int() != -1 {
				return nil, unsupported
			}
			indices[id] = -1
		case "kill":
			if !genericArgsAllowed(generic, "Action", "ConversationIds") || state.Has(indexField) {
				return nil, unsupported
			}
		default:
			return nil, unsupported
		}
	}
	metadata := map[string]string{"action": action, "conversation_ids": strings.Join(names, ",")}
	if action == "list" {
		b, err := json.Marshal(indices)
		if err != nil {
			return nil, err
		}
		metadata["subagent_step_indices"] = string(b)
	} else {
		arg, ok := genericArg(generic, "ConversationIds")
		var requested []string
		if !ok || json.Unmarshal([]byte(arg), &requested) != nil || len(requested) != len(names) {
			return nil, unsupported
		}
		for i := range names {
			if requested[i] != names[i] {
				return nil, unsupported
			}
		}
		b, err := json.Marshal(names)
		if err != nil {
			return nil, err
		}
		metadata["killed_conversation_ids"] = string(b)
	}
	out := proto.Clone(step.Interface())
	target := tool143Mutable(tool143Mutable(out.ProtoReflect(), "generic"), "result")
	values := target.Mutable(target.Descriptor().Fields().ByName("metadata")).Map()
	for k, v := range metadata {
		values.Set(protoreflect.ValueOfString(k).MapKey(), protoreflect.ValueOfString(v))
	}
	return out, nil
}
