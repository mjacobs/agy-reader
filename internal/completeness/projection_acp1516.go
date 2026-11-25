package completeness

import (
	"errors"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// T3 Code runs the Antigravity 1.5.16 language server as `antigravity acp` and
// answers its permission prompts over ACP. Every observed command, file edit
// and MCP call then keeps both permission records on its generic wrapper.
// Unlike the 1.5.7 strict-mode approvals (modernPermissions157), the request
// names the default fallback (commands, MCP calls) or the default write grant
// (edits) as its trigger. The ordinary RPC keeps both records on the presented
// step unchanged, so they stay on the cloned step and are compared in full.
//
// Only the observed shapes qualify: one allowing grant for exactly the call's
// action and target, one completed request for the same resource answered
// with allow and an empty persist_grants, and the response naming its
// trajectory. A persist suggestion is either SUGGESTED with a pattern,
// BLOCKED without one (commands only), or absent along with its pattern.
// Commands also repeat the payload's shell, and MCP requests carry only their
// resource, the fixed description and the trigger.
const (
	acpEditGrantDetail1516 = "write_file(/)"
	acpSuggestedPersist    = "PERSIST_SUGGESTION_TYPE_SUGGESTED"
	acpBlockedPersist      = "PERSIST_SUGGESTION_TYPE_BLOCKED"
)

// acpApproval1516 checks the records against kind. The wrapper arguments must
// be the presented call, since the projections that use it drop them.
func acpApproval1516(step, generic protoreflect.Message, kind permissionRecordKind) bool {
	f := step.Descriptor().Fields()
	if kind.target == "" || genericToolName(step) != kind.tool || !step.Has(f.ByName("permissions")) ||
		!step.Has(f.ByName("completed_interactions")) || !argumentsPresented(generic, step) {
		return false
	}
	interactions := tool143Get(step, "completed_interactions").List()
	if !approvedGrant(tool143Get(step, "permissions").Message(), kind) || !completedRequest(interactions, kind, true) {
		return false
	}
	done := interactions.Get(0).Message()
	if tool143Get(tool143Get(done, "response").Message(), "trajectory_id").String() == "" {
		return false
	}
	spec := tool143Get(tool143Get(done, "request").Message(), "permission").Message()
	sf := spec.Descriptor().Fields()
	// Observed: a suggestion always names its pattern, a blocked one
	// (commands only) and an absent one name none.
	pattern := tool143Get(spec, "suggested_persist_pattern").String()
	switch enumName(spec, sf.ByName("persist_suggestion_type")) {
	case acpSuggestedPersist:
		if pattern == "" {
			return false
		}
	case acpBlockedPersist:
		if pattern != "" || kind.action != "command" {
			return false
		}
	default:
		if spec.Has(sf.ByName("persist_suggestion_type")) || spec.Has(sf.ByName("suggested_persist_pattern")) {
			return false
		}
	}
	switch kind.action {
	case "command":
		return kind.shell != "" && tool143Get(spec, "shell_name").String() == kind.shell
	case "mcp":
		server, name, _ := strings.Cut(kind.target, "/")
		return onlyFields(spec, fieldSet("resource", "action_description", "trigger_source")) &&
			tool143Get(spec, "action_description").String() == "Run "+name+" from MCP server "+server
	case "write_file":
		return !spec.Has(sf.ByName("shell_name"))
	}
	return false
}

func acpCommandKind1516(command, shell string) permissionRecordKind {
	return permissionRecordKind{tool: "run_command", action: "command", target: command, trigger: "TYPE_DEFAULT_FALLBACK", shell: shell}
}

func acpEditKind1516(tool, path string) permissionRecordKind {
	return permissionRecordKind{tool: tool, action: "write_file", target: path, trigger: "TYPE_DEFAULT_GRANT", detail: acpEditGrantDetail1516}
}

// acpModernPermissions1516 qualifies the records on a completed RunCommand or
// FileEdit payload. The arguments must name what the payload records.
func acpModernPermissions1516(step, generic, payload protoreflect.Message, name string) bool {
	switch name {
	case "exa.cortex.core.tools.RunCommand":
		command, _ := genericArg(generic, "CommandLine")
		target := tool143Get(payload, "command_line").String()
		return command == target && acpApproval1516(step, generic, acpCommandKind1516(target, tool143Get(payload, "shell_name").String()))
	case "exa.cortex.core.tools.FileEdit":
		path, _ := genericArg(generic, "Path")
		target := tool143Get(payload, "path").String()
		tool := genericToolName(step)
		return (tool == "edit_file" || tool == "write_file") && path == target && acpApproval1516(step, generic, acpEditKind1516(tool, target))
	}
	return false
}

// Strict verification uses the three arguments in the captured ACP commands.
// Offline export also accepts an explicit false flag. qualifiedCommandDaemonFlag
// checks that the retained call stores it as a JSON boolean.
func acpCommandArgs1516(step, generic protoreflect.Message, offline bool) bool {
	names := []string{"CommandLine", "WaitMsBeforeAsync", "toolSummary"}
	if flag, _ := genericArg(generic, "IsDaemon"); offline && flag == "false" {
		names = append(names, "IsDaemon")
	}
	return exactGenericArgs(step, generic, names...)
}

// acpCanceledCommand1516 qualifies an approved background command that was
// later canceled: it waited for its prompt, ran, moved to the background with
// a 10000 ms wait, and was canceled. The result keeps only render
// information. The daemon presents it as a command without output or exit
// code, like the other canceled background commands.
func acpCanceledCommand1516(step, generic, result protoreflect.Message) (proto.Message, bool) {
	f := step.Descriptor().Fields()
	metadata := tool143Get(step, "metadata").Message()
	md := metadata.Descriptor().Fields()
	if genericToolName(step) != "run_command" || statusName(step) != "CORTEX_STEP_STATUS_CANCELED" ||
		!onlyFields(step, fieldSet("type", "status", "metadata", "generic", "task_details", "permissions", "completed_interactions")) ||
		!step.Has(f.ByName("task_details")) || !metadata.Has(md.ByName("completed_at")) ||
		!onlyFields(result, fieldSet("step_render_info")) || !result.Has(result.Descriptor().Fields().ByName("step_render_info")) ||
		!hookFreeBackground157(step, generic) ||
		!historicalTransitions(step, "CORTEX_STEP_STATUS_PENDING", "CORTEX_STEP_STATUS_RUNNING", "CORTEX_STEP_STATUS_WAITING", "CORTEX_STEP_STATUS_RUNNING", "CORTEX_STEP_STATUS_CANCELED") {
		return nil, false
	}
	task := tool143Get(step, "task_details").Message()
	if !onlyFields(task, fieldSet("id", "log_uri", "progress", "description", "title", "moved_to_background_at")) ||
		tool143Get(task, "id").String() == "" || !task.Has(task.Descriptor().Fields().ByName("moved_to_background_at")) {
		return nil, false
	}
	command, _ := genericArg(generic, "CommandLine")
	wait, _ := genericArg(generic, "WaitMsBeforeAsync")
	interactions := tool143Get(step, "completed_interactions").List()
	if wait != "10000" || command == "" || interactions.Len() != 1 {
		return nil, false
	}
	// No payload records the shell; the request names it.
	request := tool143Get(tool143Get(interactions.Get(0).Message(), "request").Message(), "permission").Message()
	if !acpApproval1516(step, generic, acpCommandKind1516(command, tool143Get(request, "shell_name").String())) {
		return nil, false
	}
	return commandWithoutOutput(step, command), true
}

// acpFailedEdit1516 qualifies an approved edit_file that failed after its
// grant (a Target not found). It is presented like other failed edits: the
// requested file from Path, with no action result.
func acpFailedEdit1516(step, generic, result protoreflect.Message) (proto.Message, bool) {
	path, ok := genericArg(generic, "Path")
	if genericToolName(step) != "edit_file" || statusName(step) != "CORTEX_STEP_STATUS_ERROR" || !ok || !verbatimURIPath(path) ||
		!onlyFields(step, permissionOuterFields) || step.Has(step.Descriptor().Fields().ByName("task_details")) || !plainFailure(step) ||
		!onlyFields(result, fieldSet("step_render_info")) || !result.Has(result.Descriptor().Fields().ByName("step_render_info")) ||
		!exactGenericArgs(step, generic, "Path", "Target", "Replacement") || !acpApproval1516(step, generic, acpEditKind1516("edit_file", path)) {
		return nil, false
	}
	return editWithoutResult(step, path), true
}

// qualifyMCP1516 qualifies an ACP MCP call (mcp_<server>_<tool>). agy keeps
// these generic: the presented step is the persisted wrapper, permission
// records included. A completed call keeps its timestamped text, output URI
// and render information; a failed one keeps only render information and an
// error that repeats one message. Every observed call carried toolSummary,
// and the retained tool call must repeat all arguments.
func qualifyMCP1516(step, generic protoreflect.Message) error {
	unsupported := errors.New("unsupported ACP MCP call")
	tool := genericToolName(step)
	f := step.Descriptor().Fields()
	gf := generic.Descriptor().Fields()
	if !generic.Has(gf.ByName("result")) || step.Has(f.ByName("task_details")) || !onlyFields(step, permissionOuterFields) {
		return unsupported
	}
	if _, summarized := genericArg(generic, "toolSummary"); !summarized {
		return unsupported
	}
	result := tool143Get(generic, "result").Message()
	rf := result.Descriptor().Fields()
	switch statusName(step) {
	case "CORTEX_STEP_STATUS_DONE":
		if step.Has(f.ByName("error")) || !onlyFields(result, fieldSet("result", "full_output_uri", "step_render_info")) ||
			!result.Has(rf.ByName("result")) || !result.Has(rf.ByName("step_render_info")) {
			return unsupported
		}
		if _, ok := timestampedBody159(tool143Get(result, "result").String()); !ok {
			return unsupported
		}
	case "CORTEX_STEP_STATUS_ERROR":
		if !step.Has(f.ByName("error")) || !plainFailure(step) ||
			!onlyFields(result, fieldSet("step_render_info")) || !result.Has(rf.ByName("step_render_info")) {
			return unsupported
		}
	default:
		return unsupported
	}
	// The grant names server/tool. Its parts must rebuild the tool name, so
	// an underscore cannot move between the server and the tool.
	grants := tool143Get(tool143Get(step, "permissions").Message(), "permission_grants").List()
	if grants.Len() != 1 {
		return unsupported
	}
	target := tool143Get(grants.Get(0).Message(), "target").String()
	server, name, ok := strings.Cut(target, "/")
	if !ok || server == "" || name == "" || strings.Contains(name, "/") || tool != "mcp_"+server+"_"+name {
		return unsupported
	}
	if !acpApproval1516(step, generic, permissionRecordKind{tool: tool, action: "mcp", target: target, trigger: "TYPE_DEFAULT_FALLBACK"}) {
		return unsupported
	}
	return nil
}

// fileEditPayload reports whether a write_file wrapper holds a FileEdit
// result payload, the only write_file shape whose permission records a
// projection qualifies (acpModernPermissions1516).
func fileEditPayload(step protoreflect.Message) bool {
	generic := tool143Get(step, "generic").Message()
	result := tool143Get(generic, "result").Message()
	return tool143Get(tool143Get(result, "payload").Message(), "type_url").String() == "type.googleapis.com/exa.cortex.core.tools.FileEdit"
}
