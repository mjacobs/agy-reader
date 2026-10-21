package completeness

import (
	"errors"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// qualifyInvalidMissingSummary covers two retained calls rejected before tool
// execution: run_command and view_file omitted the required toolSummary. The
// daemon keeps the entire INVALID wrapper, including arguments and error. In
// particular these are not hook-denied commands or failed file reads, whose
// presentations synthesize native Steps.
func qualifyInvalidMissingSummary(step, generic protoreflect.Message) error {
	if !onlyFields(step, fieldSet("type", "status", "metadata", "error", "generic")) ||
		statusName(step) != "CORTEX_STEP_STATUS_INVALID" || !onlyFields(generic, fieldSet("args")) ||
		!exactHistoricalError(step, "invalid arguments:\n- missing property 'toolSummary'") ||
		!argumentsPresented(generic, step) || !historicalToolSource(step, "CORTEX_COMPONENT_SOURCE_BUILTIN") ||
		!historicalTransitions(step, "CORTEX_STEP_STATUS_PENDING", "CORTEX_STEP_STATUS_INVALID") {
		return errors.New("unsupported invalid generic wrapper")
	}
	var args []string
	switch genericToolName(step) {
	case "run_command":
		args = []string{"CommandLine", "Cwd", "IsDaemon", "RunPersistent", "WaitMsBeforeAsync", "toolAction"}
		for _, name := range []string{"IsDaemon", "RunPersistent"} {
			if value, ok := genericArg(generic, name); !ok || value != "false" {
				return errors.New("unsupported invalid command flags")
			}
		}
	case "view_file":
		args = []string{"AbsolutePath", "StartLine", "EndLine", "toolAction"}
	default:
		return errors.New("unsupported invalid generic wrapper")
	}
	if !exactHistoricalArgs(generic, args...) {
		return errors.New("unsupported invalid generic arguments")
	}
	return nil
}

// qualifyFailedListResources admits only the observed failed list_resources
// bridge. Its embedded Step holds the server and short error; the wrapper's
// repeated error, result text and full-output URI are covered by full equality.
func qualifyFailedListResources(step, generic, native protoreflect.Message) error {
	if genericToolName(step) != "list_resources" || nativeToolName(native) != "list_resources" ||
		!outerErrorRepeated(step, native) || !argumentsPresented(generic, native) ||
		!onlyFields(step, fieldSet("type", "status", "metadata", "error", "generic")) ||
		!onlyFields(native, fieldSet("type", "status", "metadata", "error", "list_resources")) {
		return errors.New("unsupported failed list_resources")
	}
	result := generic.Get(generic.Descriptor().Fields().ByName("result")).Message()
	list := native.Get(native.Descriptor().Fields().ByName("list_resources")).Message()
	server, ok := genericArg(generic, "ServerName")
	if !ok || server == "" || !onlyFields(result, fieldSet("result", "payload", "full_output_uri")) ||
		!onlyFields(list, fieldSet("server_name")) || list.Get(list.Descriptor().Fields().ByName("server_name")).String() != server {
		return errors.New("unsupported failed list_resources")
	}
	return nil
}

const commandRenderErrorPrefix = "bridged adapter run_command: to string: template execution error: "

// qualifyRenderFailedCommand covers a completed command whose bridge failed
// while formatting its result. The sole retained example stores an ERROR
// wrapper around a DONE command with exit code zero and no native error. The
// default response keeps the entire native Step except its status, which is
// ERROR, and omits the wrapper's formatting error. Only this source-derived
// shape is admitted; full equality still checks every omitted byte.
func qualifyRenderFailedCommand(step, generic, native protoreflect.Message) error {
	f, nf := step.Descriptor().Fields(), native.Descriptor().Fields()
	if genericToolName(step) != "run_command" || nativeToolName(native) != "run_command" ||
		statusName(step) != "CORTEX_STEP_STATUS_ERROR" || statusName(native) != "CORTEX_STEP_STATUS_DONE" ||
		!onlyFields(step, fieldSet("type", "status", "metadata", "error", "generic")) ||
		!onlyFields(native, fieldSet("type", "status", "metadata", "run_command")) ||
		!argumentsPresented(generic, step) || !argumentsPresented(generic, native) ||
		!exactHistoricalArgs(generic, "CommandLine", "Cwd", "WaitMsBeforeAsync", "toolAction", "toolSummary") ||
		!onlyAllowHookResult(step) || !onlyAllowHookResult(native) ||
		!historicalToolSource(step, "CORTEX_COMPONENT_SOURCE_BUILTIN") || !historicalToolSource(native, "CORTEX_COMPONENT_SOURCE_BUILTIN") ||
		!historicalTransitions(step, "CORTEX_STEP_STATUS_PENDING", "CORTEX_STEP_STATUS_RUNNING", "CORTEX_STEP_STATUS_ERROR") ||
		!historicalTransitions(native, "CORTEX_STEP_STATUS_PENDING", "CORTEX_STEP_STATUS_RUNNING") {
		return errors.New("unsupported command render failure")
	}
	outer := step.Get(f.ByName("error")).Message()
	short := outer.Get(outer.Descriptor().Fields().ByName("short_error")).String()
	result := generic.Get(generic.Descriptor().Fields().ByName("result")).Message()
	if !strings.HasPrefix(short, commandRenderErrorPrefix) || len(short) == len(commandRenderErrorPrefix) ||
		!exactHistoricalError(step, short) || !onlyFields(result, fieldSet("payload")) {
		return errors.New("unsupported command render failure")
	}
	run := native.Get(nf.ByName("run_command")).Message()
	rf := run.Descriptor().Fields()
	command, _ := genericArg(generic, "CommandLine")
	cwd, _ := genericArg(generic, "Cwd")
	wait, _ := genericArg(generic, "WaitMsBeforeAsync")
	if !onlyFields(run, fieldSet("command_line", "proposed_command_line", "cwd", "exit_code", "blocking", "wait_ms_before_async", "combined_output", "shell_name", "ran_in_sandbox")) ||
		command == "" || cwd == "" || run.Get(rf.ByName("command_line")).String() != command || run.Get(rf.ByName("proposed_command_line")).String() != command ||
		run.Get(rf.ByName("cwd")).String() != cwd || wait != "5000" || run.Get(rf.ByName("wait_ms_before_async")).Uint() != 5000 ||
		!run.Has(rf.ByName("exit_code")) || run.Get(rf.ByName("exit_code")).Int() != 0 || !run.Get(rf.ByName("blocking")).Bool() ||
		!run.Get(rf.ByName("ran_in_sandbox")).Bool() || run.Get(rf.ByName("shell_name")).String() != "bash" ||
		!run.Has(rf.ByName("combined_output")) || !onlyFields(run.Get(rf.ByName("combined_output")).Message(), fieldSet("full")) {
		return errors.New("unsupported command render failure")
	}
	native.Set(nf.ByName("status"), step.Get(f.ByName("status")))
	return nil
}

func exactHistoricalArgs(generic protoreflect.Message, names ...string) bool {
	args := generic.Get(generic.Descriptor().Fields().ByName("args")).Map()
	if args.Len() != len(names) {
		return false
	}
	for _, name := range names {
		if _, ok := genericArg(generic, name); !ok {
			return false
		}
	}
	return true
}

func exactHistoricalError(step protoreflect.Message, want string) bool {
	outer := step.Get(step.Descriptor().Fields().ByName("error")).Message()
	f := outer.Descriptor().Fields()
	return onlyFields(outer, fieldSet("short_error", "full_error")) && outer.Get(f.ByName("short_error")).String() == want && outer.Get(f.ByName("full_error")).String() == want
}

func historicalToolSource(step protoreflect.Message, want protoreflect.Name) bool {
	metadata := step.Get(step.Descriptor().Fields().ByName("metadata")).Message()
	return enumName(metadata, metadata.Descriptor().Fields().ByName("tool_source")) == want
}

func historicalTransitions(step protoreflect.Message, want ...protoreflect.Name) bool {
	metadata := step.Get(step.Descriptor().Fields().ByName("metadata")).Message()
	internal := metadata.Get(metadata.Descriptor().Fields().ByName("internal_metadata")).Message()
	transitions := internal.Get(internal.Descriptor().Fields().ByName("status_transitions")).List()
	if transitions.Len() != len(want) {
		return false
	}
	for i, status := range want {
		transition := transitions.Get(i).Message()
		if enumName(transition, transition.Descriptor().Fields().ByName("updated_status")) != status {
			return false
		}
	}
	return true
}
