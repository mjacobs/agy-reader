package completeness

import (
	"encoding/json"
	"errors"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// projectManageSubagents148 covers the captured single-child list and kill
// presentations. Other counts and actions still need native evidence. State
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
	if ids.Len() != 1 || states.Len() != 1 {
		return nil, unsupported
	}
	id, state := ids.Get(0).String(), states.Get(0).Message()
	if id == "" || strings.Contains(id, ",") || !onlyFields(state, fieldSet("conversation_id", "step_index")) || tool143Get(state, "conversation_id").String() != id {
		return nil, unsupported
	}
	metadata := map[string]string{"action": action, "conversation_ids": id}
	indexField := state.Descriptor().Fields().ByName("step_index")
	switch action {
	case "list":
		if !genericArgsAllowed(generic, "Action") || !state.Has(indexField) || state.Get(indexField).Int() != -1 {
			return nil, unsupported
		}
		b, err := json.Marshal(map[string]int64{id: -1})
		if err != nil {
			return nil, err
		}
		metadata["subagent_step_indices"] = string(b)
	case "kill":
		if !genericArgsAllowed(generic, "Action", "ConversationIds") || state.Has(indexField) {
			return nil, unsupported
		}
		arg, ok := genericArg(generic, "ConversationIds")
		var requested []string
		if !ok || json.Unmarshal([]byte(arg), &requested) != nil || len(requested) != 1 || requested[0] != id {
			return nil, unsupported
		}
		b, err := json.Marshal([]string{id})
		if err != nil {
			return nil, err
		}
		metadata["killed_conversation_ids"] = string(b)
	default:
		return nil, unsupported
	}
	out := proto.Clone(step.Interface())
	target := tool143Mutable(tool143Mutable(out.ProtoReflect(), "generic"), "result")
	values := target.Mutable(target.Descriptor().Fields().ByName("metadata")).Map()
	for k, v := range metadata {
		values.Set(protoreflect.ValueOfString(k).MapKey(), protoreflect.ValueOfString(v))
	}
	return out, nil
}
