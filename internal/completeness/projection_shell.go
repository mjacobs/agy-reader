package completeness

import (
	"encoding/json"
	"errors"
	"strconv"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// canceledInner reports whether an embedded run_command status and error
// presence are one of the two observed inner states under an outer CANCELED.
func canceledInner(status protoreflect.Name, hasError bool) bool {
	switch status {
	case "CORTEX_STEP_STATUS_ERROR":
		return hasError
	case "CORTEX_STEP_STATUS_RUNNING":
		return !hasError
	}
	return false
}

// projectDeniedRunCommand returns the step the ordinary response presents for a
// run_command call that a PreToolUse hook denied. The daemon replaces the
// wrapper with a RUN_COMMAND step that keeps the wrapper's status, metadata
// (including the hook result and tool source) and error, and synthesizes the
// command from the arguments: the command line, repeated as the proposed
// command line, the working directory, a blocking call and an empty combined
// output. The metadata must record exactly one hook result, a denial whose
// reason the error repeats, and the call must carry a wait argument, which was
// observed as 5000 with the call blocking (presentedUnrunCommand); any other
// presentation still fails the comparison.
func projectDeniedRunCommand(step, generic protoreflect.Message) (proto.Message, error) {
	f := step.Descriptor().Fields()
	metadata := step.Get(f.ByName("metadata")).Message()
	mf := metadata.Descriptor().Fields()
	hooks := metadata.Get(mf.ByName("pre_tool_hook_results")).List()
	if hooks.Len() != 1 {
		return nil, errors.New("unsupported run_command hook results")
	}
	hook := hooks.Get(0).Message()
	hf := hook.Descriptor().Fields()
	short := step.Get(f.ByName("error")).Message()
	if !onlyFields(hook, fieldSet("decision", "reason")) || hook.Get(hf.ByName("decision")).String() != "deny" ||
		short.Get(short.Descriptor().Fields().ByName("short_error")).String() != "tool call denied by pre-tool hook: "+hook.Get(hf.ByName("reason")).String() {
		return nil, errors.New("unsupported run_command hook results")
	}
	return presentedUnrunCommand(step, generic)
}

// presentedUnrunCommand builds the RUN_COMMAND step the ordinary response
// presents for a run_command wrapper whose command never ran: the wrapper's
// status, metadata, error and any completed interactions, and a command made
// from the arguments (the command line, repeated as the proposed command line,
// the working directory, a blocking call and an empty combined output). The
// call must carry CommandLine, Cwd and WaitMsBeforeAsync; the wait was observed
// as 5000 and 10000, both presented as blocking.
func presentedUnrunCommand(step, generic protoreflect.Message) (proto.Message, error) {
	command, ok := genericArg(generic, "CommandLine")
	cwd, okCwd := genericArg(generic, "Cwd")
	if _, okWait := genericArg(generic, "WaitMsBeforeAsync"); !ok || !okCwd || !okWait {
		return nil, errors.New("unsupported generic arguments")
	}
	native := proto.Clone(step.Interface())
	m := native.ProtoReflect()
	nf := m.Descriptor().Fields()
	m.Set(nf.ByName("type"), protoreflect.ValueOfEnum(nf.ByName("type").Enum().Values().ByName("CORTEX_STEP_TYPE_RUN_COMMAND").Number()))
	m.Clear(nf.ByName("generic"))
	run := m.Mutable(nf.ByName("run_command")).Message()
	rf := run.Descriptor().Fields()
	run.Set(rf.ByName("command_line"), protoreflect.ValueOfString(command))
	run.Set(rf.ByName("proposed_command_line"), protoreflect.ValueOfString(command))
	run.Set(rf.ByName("cwd"), protoreflect.ValueOfString(cwd))
	run.Set(rf.ByName("blocking"), protoreflect.ValueOfBool(true))
	run.Mutable(rf.ByName("combined_output"))
	return native, nil
}

// rejectedCommandAdvice ends the error agy records for a command whose
// permission request was rejected.
const rejectedCommandAdvice = "Do not attempt to circumvent this denial by rephrasing the command, " +
	"using alternative tools/scripts (e.g. python, sh, curl), or accessing the same target resource. " +
	"Proceed without performing this action."

// projectRejectedRunCommand returns the step the ordinary response presents for
// a run_command call whose permission request was rejected (agy-reader#xyfe):
// a user answered "No, cancel" or "No, deny" in agy's review mode, or agy
// soft-denied a request from a headless (-p) subagent that nobody could
// answer. Observed on agy 1.3.1, all three samples alike: the wrapper has
// status ERROR, the arguments, no result and no grant (permissions), metadata
// with exactly one pre-tool hook result {decision: allow} (recorded with no
// hook configured), an outer error whose short and full text are both
// "permission check failed for command <quoted command>: user denied
// permission to run command:", the command and rejectedCommandAdvice on
// separate lines, and the completed request: one request for that command
// from the TYPE_DEFAULT_FALLBACK trigger, answered without allow and with an
// empty persist_grants. The daemon presents it as a RUN_COMMAND step built
// like the hook-denied one (presentedUnrunCommand), keeping the completed
// request, so the transcript shows the rejection and no output.
//
// The dropped arguments must repeat the presented tool call
// (argumentsPresented), and BypassSandbox, seen only on the headless child, may
// only be "false", which the presented step leaves unset. Any other status,
// error, field, trigger, hook result or answer, an allowing answer among them,
// fails.
func projectRejectedRunCommand(step, generic protoreflect.Message) (proto.Message, error) {
	f := step.Descriptor().Fields()
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic", "error", "completed_interactions")) ||
		statusName(step) != "CORTEX_STEP_STATUS_ERROR" {
		return nil, errors.New("unsupported generic wrapper")
	}
	if !genericArgsAllowed(generic, "CommandLine", "Cwd", "WaitMsBeforeAsync", "BypassSandbox") || !argumentsPresented(generic, step) {
		return nil, errors.New("unsupported generic arguments")
	}
	if bypass, ok := genericArg(generic, "BypassSandbox"); ok && bypass != "false" {
		return nil, errors.New("unsupported generic arguments")
	}
	command, ok := genericArg(generic, "CommandLine")
	if !ok {
		return nil, errors.New("unsupported generic arguments")
	}
	outer := step.Get(f.ByName("error")).Message()
	ef := outer.Descriptor().Fields()
	want := "permission check failed for command " + strconv.Quote(command) +
		": user denied permission to run command:\n" + command + "\n" + rejectedCommandAdvice
	if !onlyFields(outer, fieldSet("short_error", "full_error")) ||
		outer.Get(ef.ByName("short_error")).String() != want || outer.Get(ef.ByName("full_error")).String() != want {
		return nil, errors.New("unsupported rejected run_command error")
	}
	if !onlyAllowHookResult(step) {
		return nil, errors.New("unsupported run_command hook results")
	}
	kind := permissionRecordKind{tool: "run_command", action: "command", target: command, trigger: "TYPE_DEFAULT_FALLBACK"}
	if !completedRequest(step.Get(f.ByName("completed_interactions")).List(), kind, false) {
		return nil, errors.New("unsupported permission record")
	}
	return presentedUnrunCommand(step, generic)
}

// onlyAllowHookResult reports whether the step's metadata records exactly one
// pre-tool hook result, {decision: allow} with no reason. agy 1.3.1 writes it
// on a run_command that reached its permission check, with no hook configured.
func onlyAllowHookResult(step protoreflect.Message) bool {
	metadata := step.Get(step.Descriptor().Fields().ByName("metadata")).Message()
	hooks := metadata.Get(metadata.Descriptor().Fields().ByName("pre_tool_hook_results")).List()
	if hooks.Len() != 1 {
		return false
	}
	hook := hooks.Get(0).Message()
	return onlyFields(hook, fieldSet("decision")) && hook.Get(hook.Descriptor().Fields().ByName("decision")).String() == "allow"
}

// canceledPromptTransitions ends the status history of a run_command step
// canceled while its permission prompt was open: the step waited for an
// answer, resumed, and was canceled. A rejected request ends WAITING,
// RUNNING, ERROR the same way.
var canceledPromptTransitions = []protoreflect.Name{
	"CORTEX_STEP_STATUS_WAITING", "CORTEX_STEP_STATUS_RUNNING", "CORTEX_STEP_STATUS_CANCELED",
}

// projectCanceledRunCommand returns the step the ordinary response presents for
// a run_command call that was canceled while its permission prompt was still
// open (agy-reader#3cxt). Observed once, on agy 1.3.1 (report alias conversation-114),
// when agy was stopped with the prompt unanswered: the wrapper has status
// CANCELED, the arguments CommandLine, Cwd and WaitMsBeforeAsync, no result, no
// error, no grant and no interaction record, and its metadata records exactly
// one pre-tool hook result {decision: allow} and a status history ending
// WAITING, RUNNING, CANCELED, with no answer recorded. The daemon presents it as
// a RUN_COMMAND step built from the arguments (presentedUnrunCommand), keeping
// the status and metadata, so the transcript shows a canceled command with no
// exit code and no output.
//
// The dropped arguments must repeat the presented tool call
// (argumentsPresented). Any other field, argument (BypassSandbox among them),
// hook result or status history fails.
func projectCanceledRunCommand(step, generic protoreflect.Message) (proto.Message, error) {
	f := step.Descriptor().Fields()
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic")) || statusName(step) != "CORTEX_STEP_STATUS_CANCELED" {
		return nil, errors.New("unsupported generic wrapper")
	}
	if !genericArgsAllowed(generic, "CommandLine", "Cwd", "WaitMsBeforeAsync") || !argumentsPresented(generic, step) {
		return nil, errors.New("unsupported generic arguments")
	}
	if !onlyAllowHookResult(step) {
		return nil, errors.New("unsupported run_command hook results")
	}
	metadata := step.Get(f.ByName("metadata")).Message()
	internal := metadata.Get(metadata.Descriptor().Fields().ByName("internal_metadata")).Message()
	transitions := internal.Get(internal.Descriptor().Fields().ByName("status_transitions")).List()
	n := transitions.Len() - len(canceledPromptTransitions)
	if n < 0 {
		return nil, errors.New("unsupported run_command status history")
	}
	for k, want := range canceledPromptTransitions {
		transition := transitions.Get(n + k).Message()
		if enumName(transition, transition.Descriptor().Fields().ByName("updated_status")) != want {
			return nil, errors.New("unsupported run_command status history")
		}
	}
	return presentedUnrunCommand(step, generic)
}

// runCommandFlagsPresented lets run_command drop its BypassSandbox, IsDaemon,
// RunPersistent and RequestedTerminalID arguments only when the presented
// native step carries the same values. Observed in agy 1.2.16 sessions: a
// boolean argument is "true" or "false" in the generic arguments and the same
// JSON literal in the native tool-call arguments; the native step records true
// as sandbox_override, is_daemon or run_persistent and leaves false unset. A
// terminal id is the same string in both and in requested_terminal_id, which is
// unset for an empty id. Absent arguments qualify nothing.
func runCommandFlagsPresented(tool string, generic, native protoreflect.Message) bool {
	command := native.Get(native.Descriptor().Fields().ByName("run_command")).Message()
	cf := command.Descriptor().Fields()
	var called map[string]json.RawMessage
	for _, flag := range []struct {
		arg   string
		field protoreflect.Name
	}{{"BypassSandbox", "sandbox_override"}, {"IsDaemon", "is_daemon"}, {"RunPersistent", "run_persistent"}, {"RequestedTerminalID", "requested_terminal_id"}} {
		arg, ok := genericArg(generic, flag.arg)
		if !ok {
			continue
		}
		if tool != "run_command" {
			return false
		}
		if called == nil {
			if called, ok = calledArguments(native); !ok {
				return false
			}
		}
		field := cf.ByName(flag.field)
		if field.Kind() == protoreflect.BoolKind {
			if (arg != "true" && arg != "false") || string(called[flag.arg]) != arg || command.Get(field).Bool() != (arg == "true") {
				return false
			}
			continue
		}
		text, isString := literalString(called[flag.arg])
		if !isString || text != arg || command.Get(field).String() != arg {
			return false
		}
	}
	return true
}
