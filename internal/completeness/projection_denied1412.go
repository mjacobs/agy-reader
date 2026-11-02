package completeness

import (
	"errors"
	"strconv"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// preToolHookDenial starts the outer error agy records for a tool call that a
// PreToolUse hook denied; the hook's reason follows it.
const preToolHookDenial = "tool call denied by pre-tool hook: "

// deniedByPreToolHook reports whether a step's metadata records exactly one
// pre-tool hook result, a deny holding only a decision and a reason, and the
// step's short error is preToolHookDenial followed by that reason. agy stores
// every hook-denied call this way, whatever the tool (run_command, read_file
// and manage_task were observed); how the ordinary response presents the step
// depends on the tool and is qualified per tool.
func deniedByPreToolHook(step protoreflect.Message) bool {
	hook, ok := onlyHookResult(step)
	if !ok || !onlyFields(hook, fieldSet("decision", "reason")) || tool143Get(hook, "decision").String() != "deny" {
		return false
	}
	outer := tool143Get(step, "error").Message()
	return tool143Get(outer, "short_error").String() == preToolHookDenial+tool143Get(hook, "reason").String()
}

// onlyHookResult returns the step's pre-tool hook result when it records
// exactly one.
func onlyHookResult(step protoreflect.Message) (protoreflect.Message, bool) {
	hooks := tool143Get(tool143Get(step, "metadata").Message(), "pre_tool_hook_results").List()
	if hooks.Len() != 1 {
		return nil, false
	}
	return hooks.Get(0).Message(), true
}

// plainFailure reports whether the step's outer error holds only short and
// full error text, both equal and nonempty, as on every hook-denied read_file
// and failed manage_task captured with agy 1.4.12.
func plainFailure(step protoreflect.Message) bool {
	outer := tool143Get(step, "error").Message()
	short := tool143Get(outer, "short_error").String()
	return onlyFields(outer, fieldSet("short_error", "full_error")) && short != "" &&
		tool143Get(outer, "full_error").String() == short
}

// exactGenericArgs reports whether the call carries exactly the named
// arguments, and the retained tool call repeats them.
func exactGenericArgs(step, generic protoreflect.Message, names ...string) bool {
	args := tool143Get(generic, "args").Map()
	if args.Len() != len(names) || !stringMapKeysAllowed(args, names...) {
		return false
	}
	return argumentsPresented(generic, step)
}

// projectDeniedRead1412 returns the step the ordinary response presents for a
// read_file call that a PreToolUse hook denied (agy-reader#gs51). Two native
// agy 1.4.12 captures stored a resultless generic wrapper with status ERROR,
// the built-in tool source, one deny hook result, an outer error repeating its
// reason, and the arguments Path, StartLine and EndLine. The daemon presented
// it as a VIEW_FILE step built from the arguments, as for a resultless
// view_file (viewStepFromArgs): StartLine 2 and EndLine 3 became start_line 1
// and end_line 2. Paths outside plainAbsolutePath, SKILL.md paths, a call
// without both lines, other arguments, another hook result or another error
// were not observed and stay unsupported.
func projectDeniedRead1412(step, generic protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported hook-denied read_file")
	metadata := tool143Get(step, "metadata").Message()
	source := metadata.Descriptor().Fields().ByName("tool_source")
	if v := source.Enum().Values().ByNumber(metadata.Get(source).Enum()); v == nil || v.Name() != "CORTEX_COMPONENT_SOURCE_BUILTIN" {
		return nil, unsupported
	}
	if statusName(step) != "CORTEX_STEP_STATUS_ERROR" || !plainFailure(step) || !deniedByPreToolHook(step) ||
		!exactGenericArgs(step, generic, "Path", "StartLine", "EndLine") {
		return nil, unsupported
	}
	path, _ := genericArg(generic, "Path")
	if !plainAbsolutePath(path) || skillFileRead146(path) {
		return nil, unsupported
	}
	return viewStepFromArgs(step, path, false, generic, unsupported)
}

// qualifyFailedManageTask1412 accepts a resultless manage_task wrapper with
// status ERROR, which the ordinary response presents unchanged, so the
// pass-through comparison covers every byte of it. Native agy 1.4.12 captures
// stored it for two causes, alike apart from the hook result and the error:
//
//   - a PreToolUse hook denied the call (agy-reader#gs51): one deny hook
//     result whose reason the error repeats (deniedByPreToolHook); observed
//     with Action list;
//   - a kill that arrived after its task had finished: one allow hook result
//     with no reason and the error `cannot kill task "<TaskId>": task is not
//     running (status: DONE)`, the task ID quoted as Go %q.
//
// Both errors hold equal short and full text only. The arguments are limited
// to Action and TaskId (TaskId required for, and only seen with, kill) and must
// be repeated by the retained tool call. Another hook result, error, status in
// the error, action for the late kill, or argument stays unsupported.
func qualifyFailedManageTask1412(step, generic protoreflect.Message) error {
	unsupported := errors.New("unsupported failed manage_task")
	if statusName(step) != "CORTEX_STEP_STATUS_ERROR" || !plainFailure(step) {
		return unsupported
	}
	action, _ := genericArg(generic, "Action")
	taskID, hasTask := genericArg(generic, "TaskId")
	if hasTask {
		if !exactGenericArgs(step, generic, "Action", "TaskId") {
			return unsupported
		}
	} else if !exactGenericArgs(step, generic, "Action") {
		return unsupported
	}
	if action == "" {
		return unsupported
	}
	if deniedByPreToolHook(step) {
		return nil
	}
	hook, ok := onlyHookResult(step)
	if !ok || !onlyFields(hook, fieldSet("decision")) || tool143Get(hook, "decision").String() != "allow" {
		return unsupported
	}
	short := tool143Get(tool143Get(step, "error").Message(), "short_error").String()
	if action != "kill" || taskID == "" ||
		short != "cannot kill task "+strconv.Quote(taskID)+": task is not running (status: DONE)" {
		return unsupported
	}
	return nil
}
