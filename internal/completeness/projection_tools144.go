package completeness

import (
	"errors"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// skillFileRead146 mirrors how agy presents a modern ReadFile payload. The
// payload has no marker; when agy 1.4.6 rebuilds the VIEW_FILE step for the
// ordinary RPC (rehydrateReadFile), it sets is_skill_file from the stored path
// alone: after turning backslashes into slashes, the path ends in "/SKILL.md"
// or is exactly "SKILL.md". The comparison is case-sensitive. It does not
// consult the executor's skill registry, the filesystem or the file content, so
// neither does this rule. A daemon answer with a different marker still fails
// exact comparison.
func skillFileRead146(path string) bool {
	path = strings.ReplaceAll(path, `\`, "/")
	return strings.HasSuffix(path, "/SKILL.md") || path == "SKILL.md"
}

// projectCanceledBackgroundCommand144 covers canceled run_command steps whose
// result contains render information only. Default RPC presents the command
// from its arguments with empty combined output. It preserves metadata and task
// details, but does not claim an exit code or cwd. Full-source equality still
// checks the dropped render data, and the retained tool call must repeat the
// dropped arguments.
//
// Each accepted shape is a combination observed natively (canceledCommandShapes):
// its history, whether it has task details, its wait and its BypassSandbox
// argument. Any other combination stays unsupported.
func projectCanceledBackgroundCommand144(step, generic, result protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported canceled background command projection")
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic", "task_details")) ||
		statusName(step) != "CORTEX_STEP_STATUS_CANCELED" ||
		!onlyFields(result, fieldSet("step_render_info")) ||
		!result.Has(result.Descriptor().Fields().ByName("step_render_info")) ||
		!genericArgsAllowed(generic, "CommandLine", "WaitMsBeforeAsync", "BypassSandbox", "IsDaemon") ||
		!argumentsPresented(generic, step) {
		return nil, unsupported
	}
	command, hasCommand := genericArg(generic, "CommandLine")
	wait, _ := genericArg(generic, "WaitMsBeforeAsync")
	bypass, hasBypass := genericArg(generic, "BypassSandbox")
	daemon, hasDaemon := genericArg(generic, "IsDaemon")
	if !hasCommand {
		return nil, unsupported
	}
	backgrounded := step.Has(step.Descriptor().Fields().ByName("task_details"))
	taskDaemon := false
	if backgrounded {
		task := tool143Get(step, "task_details").Message()
		if !onlyFields(task, fieldSet("id", "log_uri", "progress", "description", "title", "moved_to_background_at", "is_daemon")) ||
			tool143Get(task, "id").String() == "" || !task.Has(task.Descriptor().Fields().ByName("moved_to_background_at")) {
			return nil, unsupported
		}
		taskDaemon = tool143Get(task, "is_daemon").Bool()
	}
	metadata := tool143Get(step, "metadata").Message()
	completed := metadata.Has(metadata.Descriptor().Fields().ByName("completed_at"))
	transitions := tool143Get(tool143Get(metadata, "internal_metadata").Message(), "status_transitions").List()
	history := make([]protoreflect.Name, transitions.Len())
	for i := range history {
		transition := transitions.Get(i).Message()
		history[i] = enumName(transition, transition.Descriptor().Fields().ByName("updated_status"))
	}
	modernUncompleted := hookFreeBackground157(step, generic) && backgrounded && !completed && !hasBypass && !hasDaemon && !taskDaemon && slices.Equal(history, canceledHistory[:2])
	if !onlyAllowHookResult(step) && !modernUncompleted {
		// agy 1.5.6 also cancels Auto-mode background commands without any
		// hook result. Qualify only the captured combination, rather than
		// relaxing hook checks for every historical cancellation shape.
		hooks := tool143Get(metadata, "pre_tool_hook_results").List()
		// The 1.5.8 audit transcript repeats the completed hook-free shape
		// with a 10000 ms wait and the retained builtin tool identity.
		waitOK := wait == "5000" || (wait == "10000" && hookFreeBackground157(step, generic))
		if hooks.Len() != 0 || !backgrounded || !completed || !waitOK || hasBypass || !slices.Equal(history, canceledHistory) {
			return nil, unsupported
		}
	}
	called, _ := calledArguments(step)
	if hasDaemon && (daemon != "true" || string(called["IsDaemon"]) != "true") {
		return nil, unsupported
	}
	matched := modernUncompleted
	for _, shape := range canceledCommandShapes {
		if shape.daemon != hasDaemon || shape.daemon != taskDaemon {
			continue
		}
		if shape.backgrounded == backgrounded && (!shape.uncompleted || !completed) && (!shape.completed || completed) && slices.Equal(shape.history, history) &&
			slices.Contains(shape.waits, wait) && hasBypass == (shape.bypass != "") && bypass == shape.bypass {
			matched = true
			break
		}
	}
	if !matched {
		return nil, unsupported
	}
	out := proto.Clone(step.Interface())
	run := tool143Native(out.ProtoReflect(), "RUN_COMMAND", "run_command")
	tool143Set(run, "command_line", protoreflect.ValueOfString(command))
	tool143Set(run, "proposed_command_line", protoreflect.ValueOfString(command))
	if !backgrounded {
		tool143Set(run, "blocking", protoreflect.ValueOfBool(true))
	}
	tool143Mutable(run, "combined_output")
	return out, nil
}

// canceledCommandShape is one natively observed combination for
// projectCanceledBackgroundCommand144. A command without task details never
// moved to the background and is presented blocking.
type canceledCommandShape struct {
	backgrounded bool                // task details with a background timestamp
	daemon       bool                // IsDaemon=true in the call and task
	completed    bool                // this shape requires a completion time
	uncompleted  bool                // the metadata must not record a completion time
	history      []protoreflect.Name // the status transitions, in order
	waits        []string            // accepted WaitMsBeforeAsync values
	bypass       string              // the required BypassSandbox value; "" means absent
}

var canceledHistory = []protoreflect.Name{"CORTEX_STEP_STATUS_PENDING", "CORTEX_STEP_STATUS_RUNNING", "CORTEX_STEP_STATUS_CANCELED"}

// canceledCommandShapes lists the observed combinations:
//
//   - tools144 (agy 1.4.4, and agy-reader#z0wh on 1.4.12): moved to the
//     background, then canceled, with a 5000 ms wait. bgcancel150 (retained
//     conversations rechecked with agy 1.5.0, agy-reader#t5zw) adds the
//     10000 ms wait seen on two such steps.
//   - bgcancel150: canceled before it moved to the background (no task
//     details), seen once with a 10000 ms wait and no BypassSandbox.
//   - bgcancel150: a background command canceled without a recorded
//     transition: the history ends pending, running and there is no completion
//     time. Seen once, with a 10000 ms wait and BypassSandbox "true", which the
//     presentation leaves to the retained tool call.
//   - tools153: the same short history and wait without a bypass argument.
//   - tools154: canceled before backgrounding, with completion, 5000 ms wait
//     and BypassSandbox true retained in the tool call.
//   - retained156: daemon command moved to the background, completed cancellation
//     history, 1000 ms wait and no bypass argument.
var canceledCommandShapes = []canceledCommandShape{
	{backgrounded: true, daemon: true, completed: true, history: canceledHistory, waits: []string{"1000"}},
	{backgrounded: false, completed: true, history: canceledHistory, waits: []string{"5000"}, bypass: "true"},
	{backgrounded: true, history: canceledHistory, waits: []string{"5000", "10000"}},
	{backgrounded: false, history: canceledHistory, waits: []string{"10000"}},
	{backgrounded: true, uncompleted: true, history: canceledHistory[:2], waits: []string{"10000"}, bypass: "true"},
	{backgrounded: true, uncompleted: true, history: canceledHistory[:2], waits: []string{"10000"}},
}

// projectManageSubagents144 covers the observed completed list calls naming two
// conversations, and the empty list captured on agy 1.4.9. The daemon preserves
// the wrapper and adds action and, when nonempty, the comma-joined conversation
// IDs to result metadata. Other actions, counts and permission-bearing shapes
// need their own native evidence.
func projectManageSubagents144(step, generic, result, payload protoreflect.Message) (proto.Message, error) {
	decoded, err := decode("exa.cortex.core.tools.ManageSubagents", tool143Get(payload, "value").Bytes())
	if err != nil {
		return nil, err
	}
	if tool143Get(decoded.ProtoReflect(), "subagents").List().Len() > 0 {
		return projectManageSubagents148(step, generic, result, decoded.ProtoReflect())
	}
	unsupported := errors.New("unsupported manage_subagents projection")
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic")) ||
		genericToolName(step) != "manage_subagents" || statusName(step) != "CORTEX_STEP_STATUS_DONE" ||
		!onlyFields(result, fieldSet("result", "payload", "full_output_uri", "step_render_info")) ||
		!genericArgsAllowed(generic, "Action") || !argumentsPresented(generic, step) {
		return nil, unsupported
	}
	action, present := genericArg(generic, "Action")
	if !present || action != "list" {
		return nil, unsupported
	}
	p := decoded.ProtoReflect()
	if !onlyFields(p, fieldSet("action", "conversation_ids")) || tool143Get(p, "action").String() != action {
		return nil, unsupported
	}
	ids := tool143Get(p, "conversation_ids").List()
	if ids.Len() != 0 && ids.Len() != 2 {
		return nil, unsupported
	}
	names := make([]string, ids.Len())
	for i := range names {
		names[i] = ids.Get(i).String()
		if names[i] == "" || strings.Contains(names[i], ",") {
			return nil, unsupported
		}
	}
	out := proto.Clone(step.Interface())
	clonedResult := tool143Mutable(tool143Mutable(out.ProtoReflect(), "generic"), "result")
	metadata := clonedResult.Mutable(clonedResult.Descriptor().Fields().ByName("metadata")).Map()
	metadata.Set(protoreflect.ValueOfString("action").MapKey(), protoreflect.ValueOfString(action))
	if len(names) > 0 {
		metadata.Set(protoreflect.ValueOfString("conversation_ids").MapKey(), protoreflect.ValueOfString(strings.Join(names, ",")))
	}
	return out, nil
}
