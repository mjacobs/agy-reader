package completeness

import (
	"errors"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// cwdRejection1412 is the error agy 1.4.12 records, as both short and full
// text, for a run_command call that passes Cwd: the tool's argument schema no
// longer has that property.
const cwdRejection1412 = "invalid arguments:\n- additional properties 'Cwd' not allowed"

// qualifyInvalidCwd1412 accepts a run_command call that agy 1.4.12 rejected
// for its Cwd argument before running the tool or any hook (agy-reader#qw3t).
// Two native captures stored a resultless generic wrapper with status INVALID,
// the history PENDING, INVALID, the built-in tool source, no hook result,
// exactly cwdRejection1412 as the outer error, and the arguments CommandLine,
// Cwd, WaitMsBeforeAsync and toolSummary repeated by the retained tool call.
// The ordinary response presents the wrapper unchanged, so the pass-through
// comparison covers every byte of it; this is the same treatment as the
// missing-toolSummary rejection (qualifyInvalidMissingSummary) and the
// rejected send_message Priority (qualifyInvalidSendMessage149). A different
// or additional rejected property, another argument, status, history, tool
// source, hook result or wrapper field stays unsupported.
func qualifyInvalidCwd1412(step, generic protoreflect.Message) error {
	unsupported := errors.New("unsupported invalid run_command")
	if !onlyFields(step, fieldSet("type", "status", "metadata", "error", "generic")) ||
		statusName(step) != "CORTEX_STEP_STATUS_INVALID" || !onlyFields(generic, fieldSet("args")) ||
		!exactHistoricalError(step, cwdRejection1412) ||
		!historicalToolSource(step, "CORTEX_COMPONENT_SOURCE_BUILTIN") ||
		!historicalTransitions(step, "CORTEX_STEP_STATUS_PENDING", "CORTEX_STEP_STATUS_INVALID") ||
		tool143Get(tool143Get(step, "metadata").Message(), "pre_tool_hook_results").List().Len() != 0 ||
		!exactGenericArgs(step, generic, "CommandLine", "Cwd", "WaitMsBeforeAsync", "toolSummary") {
		return unsupported
	}
	return nil
}
