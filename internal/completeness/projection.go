package completeness

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	pathpkg "path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// CompareResponses requires both the full unhydrated source representation and
// the default presentation to match this snapshot. The two responses must come
// from the same endpoint/trajectory; callers bracket them with source snapshots.
// Checking only the projection would miss changes to omitted wrapper content.
func (s *Snapshot) CompareResponses(fullRaw, presentation []byte) Result {
	r, _ := s.compareResponses(fullRaw, presentation)
	return r
}

// compareResponses is CompareResponses that also reports whether a refusal is
// source-proven: fixed by the snapshot and Profile alone, so that every daemon
// answer for this snapshot is refused.
//
// Only an unsupported projection of the snapshot is source-proven. It is
// reached only after the full representation equals the snapshot, and project
// reads nothing but the snapshot, so no presentation the daemon sends can ever
// verify it. Refusals found while validating either response (malformed JSON,
// an RPC enum name or Any type outside the profile, size or nesting limits)
// and differing content depend on what the daemon sent.
func (s *Snapshot) compareResponses(fullRaw, presentation []byte) (Result, bool) {
	if r := s.CompareFullRaw(fullRaw); r.State != Verified {
		return r, false
	}
	projected, err := project(s.trajectory)
	if err != nil {
		return Result{Unknown, err.Error()}, true
	}
	return compareProjection(projected, presentation), false
}

// ComparePresentation checks the supported daemon projection, not a lossless
// archive. Callers may use it alone only to recheck an existing attestation whose
// source digest also matches; it cannot mint a completeness attestation.
func (s *Snapshot) ComparePresentation(raw []byte) Result {
	if s == nil || s.trajectory == nil {
		return Result{Unknown, "source snapshot unavailable"}
	}
	projected, err := project(s.trajectory)
	if err != nil {
		return Result{Unknown, err.Error()}
	}
	return compareProjection(projected, raw)
}

func compareProjection(projected proto.Message, raw []byte) Result {
	incoming, err := rpcMessage(raw)
	if err != nil {
		return Result{Unknown, err.Error()}
	}
	if !proto.Equal(projected, incoming) {
		return Result{Incomplete, "daemon presentation differs from supported source projection"}
	}
	return Result{Verified, ""}
}

func rpcMessage(raw []byte) (proto.Message, error) {
	if len(raw) > maxBytes {
		return nil, errors.New("RPC size limit")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil || top == nil {
		return nil, errors.New("invalid trajectory JSON")
	}
	delete(top, "agyReader")
	data, err := json.Marshal(top)
	if err != nil {
		return nil, errors.New("invalid trajectory JSON")
	}
	m, err := newMessage("gemini_coder.Trajectory")
	if err != nil {
		return nil, err
	}
	if err = validateRPCEnums(data, m.ProtoReflect().Descriptor()); err != nil {
		return nil, err
	}
	if err = (protojson.UnmarshalOptions{DiscardUnknown: true, Resolver: schemaTypes, RecursionLimit: maxDepth}).Unmarshal(data, m); err != nil {
		return nil, errors.New("unsupported RPC protobuf JSON")
	}
	if err = normalize(m.ProtoReflect(), 0); err != nil {
		return nil, err
	}
	return m, nil
}

func onlyFields(m protoreflect.Message, allowed map[protoreflect.Name]bool) bool {
	ok := true
	m.Range(func(f protoreflect.FieldDescriptor, _ protoreflect.Value) bool { ok = allowed[f.Name()]; return ok })
	return ok
}

// project implements only qualified daemon presentation behaviors. Dropped
// wrapper content is still required to match in CompareFullRaw before a new
// attestation can be issued.
func project(source proto.Message) (proto.Message, error) {
	out := proto.Clone(source)
	m := out.ProtoReflect()
	steps := m.Mutable(m.Descriptor().Fields().ByName("steps")).List()
	// An outer error is accepted only on a failed payload-free manage_task or
	// schedule step or a failed resultless wrapper, all presented unchanged,
	// and on a failed native Step of an observed type whose own error it
	// repeats (see outerErrorTypes).
	baseOuterFields := fieldSet("type", "status", "metadata", "generic", "task_details", "error")
	askOuterFields := fieldSet("type", "status", "metadata", "generic", "task_details", "error", "completed_interactions")
	commandOuterFields := fieldSet("type", "status", "metadata", "generic", "task_details", "error", "permissions", "completed_interactions")
	for i := 0; i < steps.Len(); i++ {
		step := steps.Get(i).Message()
		f := step.Descriptor().Fields()
		typ := f.ByName("type").Enum().Values().ByNumber(step.Get(f.ByName("type")).Enum())
		if typ == nil {
			return nil, errors.New("unsupported step type")
		}
		if typ.Name() != "CORTEX_STEP_TYPE_GENERIC" {
			continue
		}
		// Only an ask_question wrapper may carry the interactions it completed,
		// and only a run_command wrapper the permission request it completed;
		// qualifyNativeStep requires them to equal the presented native Step's.
		outerFields := baseOuterFields
		if step.Has(f.ByName("generic")) {
			switch genericToolName(step) {
			case "ask_question":
				outerFields = askOuterFields
			case "run_command":
				outerFields = commandOuterFields
			}
		}
		if !onlyFields(step, outerFields) || !step.Has(f.ByName("generic")) {
			return nil, errors.New("unsupported generic wrapper")
		}
		failed := step.Has(f.ByName("error"))
		generic := step.Get(f.ByName("generic")).Message()
		gf := generic.Descriptor().Fields()
		if !onlyFields(generic, fieldSet("args", "result")) || (!generic.Has(gf.ByName("result")) && !failed) {
			return nil, errors.New("unsupported generic fields")
		}
		if !generic.Has(gf.ByName("result")) {
			converted, err := qualifyResultless(step, generic)
			if err != nil {
				return nil, err
			}
			if converted != nil {
				steps.Set(i, protoreflect.ValueOfMessage(converted.ProtoReflect()))
			} else if genericToolName(step) == "run_command" {
				native, err := projectDeniedRunCommand(step, generic)
				if err != nil {
					return nil, err
				}
				steps.Set(i, protoreflect.ValueOfMessage(native.ProtoReflect()))
			}
			continue
		}
		result := generic.Get(gf.ByName("result")).Message()
		rf := result.Descriptor().Fields()
		tool := genericToolName(step)
		if !result.Has(rf.ByName("payload")) {
			// These tools remain generic, including all their text, render
			// information and metadata. Missing payloads on other tools fail closed.
			var args []string
			fields := fieldSet("result", "full_output_uri", "step_render_info")
			switch tool {
			case "manage_task":
				args = []string{"Action", "TaskId"}
				fields["metadata"] = true
			case "send_message":
				args = []string{"Message", "Priority", "Recipient"}
			case "schedule", "manage_subagents", "define_subagent":
				if err := qualifyPayloadFree(step, generic, result, tool, failed); err != nil {
					return nil, err
				}
				continue
			default:
				return nil, errors.New("unsupported generic result")
			}
			if !onlyFields(result, fields) {
				return nil, errors.New("unsupported generic result")
			}
			if tool == "manage_task" && !stringMapKeysAllowed(result.Get(rf.ByName("metadata")).Map(), "task_description", "task_id", "tool_name") {
				return nil, errors.New("unsupported generic result metadata")
			}
			if !genericArgsAllowed(generic, args...) {
				return nil, errors.New("unsupported generic arguments")
			}
			if failed && (tool != "manage_task" || statusName(step) != "CORTEX_STEP_STATUS_ERROR") {
				return nil, errors.New("unsupported generic wrapper")
			}
			continue
		}
		payload := result.Get(rf.ByName("payload")).Message()
		pf := payload.Descriptor().Fields()
		var native proto.Message
		var err error
		switch payload.Get(pf.ByName("type_url")).String() {
		case "type.googleapis.com/gemini_coder.Step":
			if !onlyFields(result, fieldSet("result", "payload", "full_output_uri", "media")) {
				return nil, errors.New("unsupported generic result")
			}
			native, err = decode("gemini_coder.Step", payload.Get(pf.ByName("value")).Bytes())
			if err == nil {
				err = qualifyNativeStep(step, generic, native.ProtoReflect(), tool)
			}
		case "type.googleapis.com/exa.cortex.core.tools.GenerateImage":
			if failed {
				return nil, errors.New("unsupported generic wrapper")
			}
			if tool != "generate_image" || !onlyFields(result, fieldSet("result", "payload", "full_output_uri", "media", "step_render_info")) {
				return nil, errors.New("unsupported image tool projection")
			}
			if !genericArgsAllowed(generic, "AspectRatio", "ImageName", "Prompt") {
				return nil, errors.New("unsupported generic arguments")
			}
			native, err = projectImage(step, result, payload.Get(pf.ByName("value")).Bytes())
		default:
			return nil, errors.New("unsupported generic payload type")
		}
		if err != nil {
			return nil, err
		}
		steps.Set(i, protoreflect.ValueOfMessage(native.ProtoReflect()))
	}
	return out, nil
}

func fieldSet(names ...protoreflect.Name) map[protoreflect.Name]bool {
	fields := make(map[protoreflect.Name]bool, len(names))
	for _, name := range names {
		fields[name] = true
	}
	return fields
}

func genericToolName(step protoreflect.Message) string {
	metadata := step.Get(step.Descriptor().Fields().ByName("metadata")).Message()
	call := metadata.Get(metadata.Descriptor().Fields().ByName("tool_call")).Message()
	return call.Get(call.Descriptor().Fields().ByName("name")).String()
}

func genericArgsAllowed(generic protoreflect.Message, names ...string) bool {
	allowed := map[string]bool{"toolAction": true, "toolSummary": true}
	for _, name := range names {
		allowed[name] = true
	}
	return mapKeysAllowed(generic.Get(generic.Descriptor().Fields().ByName("args")).Map(), allowed)
}

func stringMapKeysAllowed(m protoreflect.Map, names ...string) bool {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	return mapKeysAllowed(m, allowed)
}

func mapKeysAllowed(m protoreflect.Map, allowed map[string]bool) bool {
	ok := true
	m.Range(func(k protoreflect.MapKey, _ protoreflect.Value) bool {
		ok = allowed[k.String()]
		return ok
	})
	return ok
}

// qualifyPayloadFree bounds the payload-free schedule, manage_subagents and
// define_subagent steps that the daemon presents unchanged, so the pass-through
// comparison covers everything shown. Statuses, outer fields, arguments and
// result fields are limited to the observed shapes: a schedule step may carry
// task details, or fail with status ERROR and an outer short/full error; a
// manage_subagents result may name killed conversations; define_subagent
// arguments are either the single definition or the separate fields.
func qualifyPayloadFree(step, generic, result protoreflect.Message, tool string, failed bool) error {
	f := step.Descriptor().Fields()
	fields := fieldSet("result", "full_output_uri", "step_render_info")
	var argsOK bool
	switch tool {
	case "schedule":
		argsOK = genericArgsAllowed(generic, "DurationSeconds", "Prompt", "TimerCondition")
	case "manage_subagents":
		argsOK = genericArgsAllowed(generic, "Action")
		fields["metadata"] = true
	case "define_subagent":
		argsOK = genericArgsAllowed(generic, "definition") ||
			genericArgsAllowed(generic, "description", "enable_mcp_tools", "enable_subagent_tools", "enable_write_tools", "name", "system_prompt")
	default:
		return errors.New("unsupported generic result")
	}
	if !onlyFields(result, fields) {
		return errors.New("unsupported generic result")
	}
	if tool == "manage_subagents" && !stringMapKeysAllowed(result.Get(result.Descriptor().Fields().ByName("metadata")).Map(), "killed_conversation_ids") {
		return errors.New("unsupported generic result metadata")
	}
	if !argsOK {
		return errors.New("unsupported generic arguments")
	}
	if tool != "schedule" && step.Has(f.ByName("task_details")) {
		return errors.New("unsupported generic wrapper")
	}
	want := protoreflect.Name("CORTEX_STEP_STATUS_DONE")
	if failed {
		if tool != "schedule" || !onlyFields(step.Get(f.ByName("error")).Message(), fieldSet("short_error", "full_error")) {
			return errors.New("unsupported generic wrapper")
		}
		want = "CORTEX_STEP_STATUS_ERROR"
	}
	if statusName(step) != want {
		return errors.New("unsupported generic wrapper")
	}
	return nil
}

func qualifyNativeStep(step, generic, native protoreflect.Message, tool string) error {
	f, nf := step.Descriptor().Fields(), native.Descriptor().Fields()
	typ := nf.ByName("type").Enum().Values().ByNumber(native.Get(nf.ByName("type")).Enum())
	if typ == nil {
		return errors.New("unsupported native tool projection")
	}
	if step.Has(f.ByName("error")) && (!outerErrorTypes[typ.Name()] || !outerErrorRepeated(step, native)) {
		return errors.New("unsupported generic wrapper")
	}
	var args []string
	anyArgs := false
	switch typ.Name() {
	case "CORTEX_STEP_TYPE_RUN_COMMAND":
		args = []string{"CommandLine", "Cwd", "WaitMsBeforeAsync", "BypassSandbox", "IsDaemon", "RunPersistent", "RequestedTerminalID"}
		if !runCommandFlagsPresented(tool, generic, native) {
			return errors.New("unsupported run_command flags")
		}
	case "CORTEX_STEP_TYPE_CODE_ACTION":
		args = []string{"CodeContent", "Overwrite", "TargetFile", "Description", "EndLine", "StartLine", "TargetContent", "AllowMultiple", "Instruction", "ReplacementContent", "ArtifactMetadata"}
		if !artifactMetadataPresented(tool, generic, native) {
			return errors.New("unsupported write_to_file artifact metadata")
		}
	case "CORTEX_STEP_TYPE_VIEW_FILE":
		args = []string{"AbsolutePath", "StartLine", "EndLine", "ContentOffset"}
		if !contentOffsetPresented(generic, native) {
			return errors.New("unsupported view_file content offset")
		}
	case "CORTEX_STEP_TYPE_INVOKE_SUBAGENT":
		if tool != "invoke_subagent" {
			return errors.New("unsupported native tool projection")
		}
		args = []string{"Subagents"}
	case "CORTEX_STEP_TYPE_GENERATE_IMAGE":
		// A successful call stores a GenerateImage payload (projectImage); an
		// embedded image Step was observed only for a failed call.
		if tool != "generate_image" || !step.Has(f.ByName("error")) {
			return errors.New("unsupported native tool projection")
		}
		args = []string{"AspectRatio", "ImageName", "Prompt"}
	default:
		spec, ok := presentedNativeTools[typ.Name()]
		if !ok || tool != spec.tool || nativeToolName(native) != spec.tool || !argumentsPresented(generic, native) {
			return errors.New("unsupported native tool projection")
		}
		args = spec.args
		anyArgs = spec.anyArgs
	}
	if !anyArgs && !genericArgsAllowed(generic, args...) {
		return errors.New("unsupported generic arguments")
	}
	// An ask_question wrapper's completed interactions are dropped by the
	// presentation, which shows the native Step's own. Both must be present and
	// equal. No other wrapper may carry them (project's outer field check), and
	// the wrapper's tool name and the native type must agree: the outer check
	// admits the interactions on the name alone.
	if (tool == "ask_question") != (typ.Name() == "CORTEX_STEP_TYPE_ASK_QUESTION") {
		return errors.New("unsupported ask_question wrapper")
	}
	if typ.Name() == "CORTEX_STEP_TYPE_ASK_QUESTION" {
		wi, ni := f.ByName("completed_interactions"), nf.ByName("completed_interactions")
		if !step.Has(wi) || !native.Has(ni) || !listsEqual(step.Get(wi).List(), native.Get(ni).List()) {
			return errors.New("unsupported wrapper/native completed_interactions difference")
		}
	}
	if err := qualifyPermissionRecord(step, native, typ.Name(), tool); err != nil {
		return err
	}
	if step.Get(f.ByName("status")).Enum() != native.Get(nf.ByName("status")).Enum() {
		// A command canceled while running is stored with the outer CANCELED
		// status over one of two inner states, and the daemon presents the outer
		// CANCELED status instead: an inner ERROR with error details (agy 1.2.16
		// and later), or an inner step still RUNNING with no error (agy stopped
		// while a foreground command ran, observed on 1.2.17). Full raw equality
		// still covers the dropped inner status. Every other difference, such as
		// an inner DONE, or a RUNNING step that also carries an error, fails closed.
		if typ.Name() != "CORTEX_STEP_TYPE_RUN_COMMAND" || statusName(step) != "CORTEX_STEP_STATUS_CANCELED" ||
			!canceledInner(statusName(native), native.Has(nf.ByName("error"))) {
			return errors.New("unsupported wrapper/native status difference")
		}
		native.Set(nf.ByName("status"), step.Get(f.ByName("status")))
	}
	if _, observed := presentedNativeTools[typ.Name()]; observed && (step.Has(f.ByName("task_details")) || native.Has(nf.ByName("task_details"))) {
		return errors.New("unsupported native tool task_details")
	}
	if step.Has(f.ByName("task_details")) {
		if native.Has(nf.ByName("task_details")) && !proto.Equal(step.Get(f.ByName("task_details")).Message().Interface(), native.Get(nf.ByName("task_details")).Message().Interface()) {
			return errors.New("unsupported wrapper/native task_details difference")
		}
		native.Set(nf.ByName("task_details"), step.Get(f.ByName("task_details")))
	} else if native.Has(nf.ByName("task_details")) {
		return errors.New("unsupported wrapper/native task_details difference")
	}
	return nil
}

// qualifyPermissionRecord bounds the record of an approved permission request
// on a run_command step (agy-reader#pzg7). When agy asks before running a
// command, here because a workspace PreToolUse hook answered force_ask, and the
// request is approved, the wrapper and its embedded RUN_COMMAND Step both store
// the grant (permissions) and the completed request and answer
// (completed_interactions). The ordinary response presents the native Step,
// with its own copy and its metadata's pre-tool hook results, and drops the
// wrapper's. So both copies must be present and equal, and the record must be
// the observed approval: one command grant for the command that ran, and one
// permission request raised by a force_ask hook and answered with allow,
// persisting nothing. A rejected request and one raised by a subagent that
// cannot ask were never observed. A rejected answer, another trigger or any
// other field fails here.
//
// A native RUN_COMMAND that carries a record under a wrapper without one, or
// the reverse, fails; a record on any other wrapper is refused by project's
// outer field check, or here when the wrapper is named run_command over
// another native type.
func qualifyPermissionRecord(step, native protoreflect.Message, typ protoreflect.Name, tool string) error {
	f, nf := step.Descriptor().Fields(), native.Descriptor().Fields()
	wp, wi := f.ByName("permissions"), f.ByName("completed_interactions")
	np, ni := nf.ByName("permissions"), nf.ByName("completed_interactions")
	outer := step.Has(wp) || (step.Has(wi) && typ != "CORTEX_STEP_TYPE_ASK_QUESTION")
	if typ != "CORTEX_STEP_TYPE_RUN_COMMAND" {
		if outer {
			return errors.New("unsupported permission record")
		}
		return nil
	}
	inner := native.Has(np) || native.Has(ni)
	if !outer && !inner {
		return nil
	}
	if tool != "run_command" || !step.Has(wp) || !step.Has(wi) || !native.Has(np) || !native.Has(ni) {
		return errors.New("unsupported wrapper/native permission record difference")
	}
	if !proto.Equal(step.Get(wp).Message().Interface(), native.Get(np).Message().Interface()) ||
		!listsEqual(step.Get(wi).List(), native.Get(ni).List()) {
		return errors.New("unsupported wrapper/native permission record difference")
	}
	if statusName(step) != "CORTEX_STEP_STATUS_DONE" || statusName(native) != "CORTEX_STEP_STATUS_DONE" {
		return errors.New("unsupported permission record status")
	}
	command := native.Get(nf.ByName("run_command")).Message()
	commandLine := command.Get(command.Descriptor().Fields().ByName("command_line")).String()
	if !approvedCommandGrant(native.Get(np).Message(), commandLine) || !approvedCommandRequest(native.Get(ni).List(), commandLine) {
		return errors.New("unsupported permission record")
	}
	return nil
}

// commandResource reports whether a PermissionResource names exactly the
// command that ran.
func commandResource(resource protoreflect.Message, commandLine string) bool {
	rf := resource.Descriptor().Fields()
	return onlyFields(resource, fieldSet("action", "target")) &&
		resource.Get(rf.ByName("action")).String() == "command" &&
		resource.Get(rf.ByName("target")).String() == commandLine
}

// approvedCommandGrant accepts the observed permissions: a single allowing
// grant for the command, its action and target repeated in its resource.
func approvedCommandGrant(permissions protoreflect.Message, commandLine string) bool {
	pf := permissions.Descriptor().Fields()
	if !onlyFields(permissions, fieldSet("permission_grants")) {
		return false
	}
	grants := permissions.Get(pf.ByName("permission_grants")).List()
	if grants.Len() != 1 {
		return false
	}
	grant := grants.Get(0).Message()
	gf := grant.Descriptor().Fields()
	return onlyFields(grant, fieldSet("allow", "action", "target", "resource")) &&
		grant.Get(gf.ByName("allow")).Bool() &&
		grant.Get(gf.ByName("action")).String() == "command" &&
		grant.Get(gf.ByName("target")).String() == commandLine &&
		grant.Has(gf.ByName("resource")) && commandResource(grant.Get(gf.ByName("resource")).Message(), commandLine)
}

// approvedCommandRequest accepts the observed completed interactions: a single
// permission request for the command, raised by a force_ask hook, answered
// with allow and nothing persisted.
func approvedCommandRequest(interactions protoreflect.List, commandLine string) bool {
	if interactions.Len() != 1 {
		return false
	}
	done := interactions.Get(0).Message()
	df := done.Descriptor().Fields()
	if !onlyFields(done, fieldSet("request", "response")) || !done.Has(df.ByName("request")) || !done.Has(df.ByName("response")) {
		return false
	}
	request := done.Get(df.ByName("request")).Message()
	rf := request.Descriptor().Fields()
	if !onlyFields(request, fieldSet("permission")) || !request.Has(rf.ByName("permission")) {
		return false
	}
	spec := request.Get(rf.ByName("permission")).Message()
	sf := spec.Descriptor().Fields()
	if !onlyFields(spec, fieldSet("resource", "persist_suggestion_type", "suggested_persist_pattern", "reason", "action_description", "trigger_source", "shell_name")) ||
		!spec.Has(sf.ByName("resource")) || !commandResource(spec.Get(sf.ByName("resource")).Message(), commandLine) || !spec.Has(sf.ByName("trigger_source")) {
		return false
	}
	trigger := spec.Get(sf.ByName("trigger_source")).Message()
	tf := trigger.Descriptor().Fields()
	if !onlyFields(trigger, fieldSet("type", "detail")) || enumName(trigger, tf.ByName("type")) != "TYPE_FORCE_ASK_HOOK" {
		return false
	}
	response := done.Get(df.ByName("response")).Message()
	of := response.Descriptor().Fields()
	if !onlyFields(response, fieldSet("trajectory_id", "step_index", "permission")) || !response.Has(of.ByName("permission")) {
		return false
	}
	answer := response.Get(of.ByName("permission")).Message()
	af := answer.Descriptor().Fields()
	if !onlyFields(answer, fieldSet("allow", "persist_grants")) || !answer.Get(af.ByName("allow")).Bool() {
		return false
	}
	// An empty persist_grants message was observed; a persisted rule was not.
	persist := answer.Get(af.ByName("persist_grants")).Message()
	return onlyFields(persist, fieldSet())
}

// enumName returns the name of an enum field's value, or "" for an undeclared
// value.
func enumName(m protoreflect.Message, fd protoreflect.FieldDescriptor) protoreflect.Name {
	if v := fd.Enum().Values().ByNumber(m.Get(fd).Enum()); v != nil {
		return v.Name()
	}
	return ""
}

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

// presentedNativeTools lists the native tool Steps whose embedded Step the
// daemon presents verbatim in place of the wrapper, in a default response that
// matched a full response and frozen source (agy-reader#3pq4). Each is tied to
// one tool name and bounded to the observed arguments. finish takes whatever
// output arguments its caller defined, so it has no fixed list; the wrapper
// arguments must still equal the presented tool call (argumentsPresented).
var presentedNativeTools = map[protoreflect.Name]struct {
	tool    string
	args    []string
	anyArgs bool
}{
	"CORTEX_STEP_TYPE_LIST_DIRECTORY":   {tool: "list_dir", args: []string{"DirectoryPath"}},
	"CORTEX_STEP_TYPE_FIND":             {tool: "find_by_name", args: []string{"Excludes", "Extensions", "MaxDepth", "Pattern", "SearchDirectory", "Type"}},
	"CORTEX_STEP_TYPE_GREP_SEARCH":      {tool: "grep_search", args: []string{"CaseInsensitive", "Includes", "IsRegex", "MatchPerLine", "Query", "SearchPath"}},
	"CORTEX_STEP_TYPE_MCP_TOOL":         {tool: "call_mcp_tool", args: []string{"Arguments", "ServerName", "ToolName"}},
	"CORTEX_STEP_TYPE_SEARCH_WEB":       {tool: "search_web", args: []string{"domain", "query"}},
	"CORTEX_STEP_TYPE_FINISH":           {tool: "finish", anyArgs: true},
	"CORTEX_STEP_TYPE_READ_URL_CONTENT": {tool: "read_url_content", args: []string{"Url"}},
	"CORTEX_STEP_TYPE_ASK_QUESTION":     {tool: "ask_question", args: []string{"questions"}},
}

func nativeToolName(native protoreflect.Message) string {
	return genericToolName(native)
}

// argumentsPresented requires the wrapper's arguments, which the presentation
// drops, to be the same call the presented native Step records: the same set of
// names, and for each the same value (a string as is, anything else as JSON,
// ignoring formatting). Without it a changed wrapper argument would vanish from
// the comparison.
func argumentsPresented(generic, native protoreflect.Message) bool {
	args := generic.Get(generic.Descriptor().Fields().ByName("args")).Map()
	called, ok := calledArguments(native)
	if !ok || called == nil || len(called) != args.Len() {
		return false
	}
	args.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		raw, found := called[k.String()]
		ok = found && jsonArgumentEqual(raw, v.String())
		return ok
	})
	return ok
}

// jsonArgumentEqual compares one presented tool-call argument with the wrapper's
// saved copy. A JSON string must equal the saved text exactly; null, "" and
// every other value are never conflated with a string. Any other value is
// compared as JSON, ignoring formatting, and each side must be exactly one
// value with nothing after it. Neither side may hold malformed Unicode, which
// encoding/json would decode to U+FFFD (review findings rv-1de6.1, rv-ta2k.1).
func jsonArgumentEqual(raw json.RawMessage, arg string) bool {
	if len(raw) > 0 && raw[0] == '"' {
		text, isString := literalString(raw)
		return isString && text == arg
	}
	decode := func(b []byte) (any, bool) {
		// encoding/json keeps only the last value of a repeated object key, so
		// conflicting repeats must be refused before the lossy decode, and so
		// must malformed Unicode escapes, which it would decode to U+FFFD.
		if !objectKeysUnique(b) || !validJSONText(b) {
			return nil, false
		}
		d := json.NewDecoder(bytes.NewReader(b))
		d.UseNumber()
		var v any
		if d.Decode(&v) != nil {
			return nil, false
		}
		// Only end of input may follow the value (Decoder.More would stop at a
		// stray closing bracket or brace).
		if _, err := d.Token(); err != io.EOF {
			return nil, false
		}
		return v, true
	}
	a, aok := decode(raw)
	b, bok := decode([]byte(arg))
	return aok && bok && reflect.DeepEqual(a, b)
}

// objectKeysUnique reports whether no object in the JSON text repeats a key,
// at any depth. It applies the exact-spelling, key-once rule of
// decodeArtifactMetadata (same token walk, same seen-set check) to nested
// values. The walk keeps its own stack rather than recursing, and refuses
// nesting deeper than encoding/json's own limit, so hostile text cannot
// exhaust the call stack. Numbers are read as json.Number so a valid but huge
// one such as 1e400 does not fail the walk. Malformed text is left for the
// caller's own decode to refuse.
func objectKeysUnique(b []byte) bool {
	const maxDepth = 10000
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	// One frame per open container. seen is nil for an array; for an object it
	// holds the keys so far, and expectKey says whether the next token is a key.
	type frame struct {
		seen      map[string]bool
		expectKey bool
	}
	var open []frame
	// valueDone records that a value finished inside the enclosing object, so
	// the next token there is a key.
	valueDone := func() {
		if n := len(open); n > 0 && open[n-1].seen != nil {
			open[n-1].expectKey = true
		}
	}
	for {
		tok, err := decoder.Token()
		if err != nil {
			return err == io.EOF
		}
		if n := len(open); n > 0 && open[n-1].seen != nil && open[n-1].expectKey {
			if d, isDelim := tok.(json.Delim); isDelim && d == '}' {
				open = open[:n-1]
				valueDone()
				continue
			}
			key, isString := tok.(string)
			if !isString || open[n-1].seen[key] {
				return false
			}
			open[n-1].seen[key] = true
			open[n-1].expectKey = false
			continue
		}
		d, isDelim := tok.(json.Delim)
		if !isDelim {
			valueDone()
			continue
		}
		switch d {
		case '{', '[':
			if len(open) >= maxDepth {
				return false
			}
			f := frame{}
			if d == '{' {
				f = frame{seen: map[string]bool{}, expectKey: true}
			}
			open = append(open, f)
		default:
			open = open[:len(open)-1]
			valueDone()
		}
	}
}

// statusName returns the step status name, or "" for an undeclared value.
func statusName(step protoreflect.Message) protoreflect.Name {
	f := step.Descriptor().Fields().ByName("status")
	if v := f.Enum().Values().ByNumber(step.Get(f).Enum()); v != nil {
		return v.Name()
	}
	return ""
}

// outerErrorTypes lists the native Step types observed in retained sessions
// under a failed wrapper with an outer error. Add a type only with an observed
// source, full RPC and default RPC triplet; an outer error over any other
// native Step, including a qualified invoke_subagent, fails as it did before.
var outerErrorTypes = map[protoreflect.Name]bool{
	"CORTEX_STEP_TYPE_CODE_ACTION":      true,
	"CORTEX_STEP_TYPE_GENERATE_IMAGE":   true,
	"CORTEX_STEP_TYPE_GREP_SEARCH":      true,
	"CORTEX_STEP_TYPE_MCP_TOOL":         true,
	"CORTEX_STEP_TYPE_READ_URL_CONTENT": true,
	"CORTEX_STEP_TYPE_RUN_COMMAND":      true,
	"CORTEX_STEP_TYPE_SEARCH_WEB":       true,
	"CORTEX_STEP_TYPE_VIEW_FILE":        true,
}

// outerErrorRepeated reports whether a failed wrapper's outer error only
// repeats the embedded native Step's own error. The daemon presents the native
// Step, with its inner error, and drops the outer one, so both steps must be
// ERROR and the outer short error must equal the inner one. The outer full
// error, which the presentation drops, is still covered by full raw equality.
func outerErrorRepeated(step, native protoreflect.Message) bool {
	f, nf := step.Descriptor().Fields(), native.Descriptor().Fields()
	if statusName(step) != "CORTEX_STEP_STATUS_ERROR" || statusName(native) != "CORTEX_STEP_STATUS_ERROR" || !native.Has(nf.ByName("error")) {
		return false
	}
	outer, inner := step.Get(f.ByName("error")).Message(), native.Get(nf.ByName("error")).Message()
	if !onlyFields(outer, fieldSet("short_error", "full_error")) {
		return false
	}
	short := outer.Descriptor().Fields().ByName("short_error")
	return outer.Get(short).String() != "" && outer.Get(short).String() == inner.Get(short).String()
}

// qualifyResultless accepts a generic wrapper that failed before its tool ran,
// so it stores arguments and an outer error but no result. The daemon presents
// most of these steps unchanged, so the pass-through comparison covers
// everything shown; tools and arguments are bounded to the observed shapes. A
// resultless view_file that names its tool source is presented instead as a
// VIEW_FILE step built from its arguments; qualifyResultless returns that step
// (see resultlessViewStep) and nil for a step presented unchanged. A
// generate_image call with invalid arguments is presented unchanged; a
// run_command that a pre-tool hook denied is presented as a RUN_COMMAND step
// (see projectDeniedRunCommand, applied by the caller).
func qualifyResultless(step, generic protoreflect.Message) (proto.Message, error) {
	f := step.Descriptor().Fields()
	if !onlyFields(step, fieldSet("type", "status", "metadata", "generic", "error")) {
		return nil, errors.New("unsupported generic wrapper")
	}
	// INVALID was observed only on write_to_file and generate_image; every
	// other tool was ERROR. generate_image was observed only as INVALID (a
	// rejected argument), so ERROR over it fails.
	tool, status := genericToolName(step), statusName(step)
	switch tool {
	case "write_to_file":
		if status != "CORTEX_STEP_STATUS_ERROR" && status != "CORTEX_STEP_STATUS_INVALID" {
			return nil, errors.New("unsupported generic wrapper")
		}
	case "generate_image":
		if status != "CORTEX_STEP_STATUS_INVALID" {
			return nil, errors.New("unsupported generic wrapper")
		}
	default:
		if status != "CORTEX_STEP_STATUS_ERROR" {
			return nil, errors.New("unsupported generic wrapper")
		}
	}
	// A run_command call that a pre-tool hook denied and a generate_image call
	// with arguments the tool rejected were observed with these errors only.
	switch tool {
	case "run_command":
		if !errorStartsWith(step, "tool call denied by pre-tool hook: ") {
			return nil, errors.New("unsupported generic wrapper")
		}
	case "generate_image":
		if !errorStartsWith(step, "invalid arguments:") {
			return nil, errors.New("unsupported generic wrapper")
		}
	}
	var args []string
	switch tool {
	case "find_by_name":
		args = []string{"Excludes", "Extensions", "Pattern", "SearchDirectory"}
	case "list_dir":
		args = []string{"DirectoryPath"}
	case "schedule":
		args = []string{"DurationSeconds", "Prompt", "TimerCondition"}
	case "run_command":
		args = []string{"CommandLine", "Cwd", "WaitMsBeforeAsync"}
	case "generate_image":
		args = []string{"AspectRatio", "ImageName", "Prompt"}
	case "view_file":
		metadata := step.Get(f.ByName("metadata")).Message()
		if metadata.Has(metadata.Descriptor().Fields().ByName("tool_source")) {
			return resultlessViewStep(step, generic)
		}
		args = []string{"AbsolutePath", "StartLine", "EndLine"}
	case "write_to_file":
		args = []string{"ArtifactMetadata", "CodeContent", "Description", "Overwrite", "TargetFile"}
	default:
		return nil, errors.New("unsupported generic result")
	}
	if !genericArgsAllowed(generic, args...) {
		return nil, errors.New("unsupported generic arguments")
	}
	return nil, nil
}

// resultlessViewStep returns the VIEW_FILE step that the daemon presents for a
// failed resultless view_file wrapper that records a tool source
// (agy-reader#2xqw). In every retained and audited sample (12 steps in 8
// retained sessions, 2 in the agy 1.2.17 audit) the presentation keeps the
// wrapper's type-independent fields (status, metadata, error), drops the
// generic arguments, and adds a view_file with the path as a file URI and the
// one-based StartLine and EndLine as zero-based start_line and end_line.
//
// The daemon also sets is_skill_file when it takes the path for a skill file,
// which the source does not record. Across 3,270 presented VIEW_FILE steps in
// retained copies it was never set for a path without "skill" in it, while the
// same skill path was marked in one session and not in another. So the step
// is built without is_skill_file only for a path that does not contain
// "skill" in any letter case; a possible skill file still fails, as do any
// tool source but BUILTIN, any status but ERROR, an outer error with fields
// beyond short and full text, a path that would need escaping or
// normalizing, a line range other than both lines or neither, and arguments
// that differ from the presented tool call, which still shows the dropped
// arguments.
func resultlessViewStep(step, generic protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported resultless view_file")
	f := step.Descriptor().Fields()
	metadata := step.Get(f.ByName("metadata")).Message()
	source := metadata.Descriptor().Fields().ByName("tool_source")
	if v := source.Enum().Values().ByNumber(metadata.Get(source).Enum()); v == nil || v.Name() != "CORTEX_COMPONENT_SOURCE_BUILTIN" {
		return nil, unsupported
	}
	if statusName(step) != "CORTEX_STEP_STATUS_ERROR" || !onlyFields(step.Get(f.ByName("error")).Message(), fieldSet("short_error", "full_error")) {
		return nil, unsupported
	}
	if !genericArgsAllowed(generic, "AbsolutePath", "StartLine", "EndLine") || !argumentsPresented(generic, step) {
		return nil, unsupported
	}
	path, ok := genericArg(generic, "AbsolutePath")
	if !ok || !plainAbsolutePath(path) || strings.Contains(strings.ToLower(path), "skill") {
		return nil, unsupported
	}
	startArg, hasStart := genericArg(generic, "StartLine")
	endArg, hasEnd := genericArg(generic, "EndLine")
	var start, end uint64
	if hasStart || hasEnd {
		var err1, err2 error
		start, err1 = strconv.ParseUint(startArg, 10, 32)
		end, err2 = strconv.ParseUint(endArg, 10, 32)
		if !hasStart || !hasEnd || err1 != nil || err2 != nil || strconv.FormatUint(start, 10) != startArg ||
			strconv.FormatUint(end, 10) != endArg || start < 1 || end < start {
			return nil, unsupported
		}
		start, end = start-1, end-1
	}
	out := proto.Clone(step.Interface())
	m := out.ProtoReflect()
	m.Set(f.ByName("type"), protoreflect.ValueOfEnum(f.ByName("type").Enum().Values().ByName("CORTEX_STEP_TYPE_VIEW_FILE").Number()))
	m.Clear(f.ByName("generic"))
	view := m.Mutable(f.ByName("view_file")).Message()
	vf := view.Descriptor().Fields()
	view.Set(vf.ByName("absolute_path_uri"), protoreflect.ValueOfString("file://"+path))
	if start != 0 {
		view.Set(vf.ByName("start_line"), protoreflect.ValueOfUint32(uint32(start)))
	}
	if end != 0 {
		view.Set(vf.ByName("end_line"), protoreflect.ValueOfUint32(uint32(end)))
	}
	return out, nil
}

// plainAbsolutePath reports whether a path is absolute, already clean, and
// made only of characters that every retained presentation copied into its
// file URI unescaped (letters, digits and / . _ - @ +). How the daemon would
// escape or normalize any other path was never observed.
func plainAbsolutePath(path string) bool {
	if !strings.HasPrefix(path, "/") || pathpkg.Clean(path) != path {
		return false
	}
	for _, r := range path {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune("/._-@+", r):
		default:
			return false
		}
	}
	return true
}

// errorStartsWith reports whether the wrapper's outer error holds only short
// and full error text, and its short error starts with prefix.
func errorStartsWith(step protoreflect.Message, prefix string) bool {
	outer := step.Get(step.Descriptor().Fields().ByName("error")).Message()
	if !onlyFields(outer, fieldSet("short_error", "full_error")) {
		return false
	}
	return strings.HasPrefix(outer.Get(outer.Descriptor().Fields().ByName("short_error")).String(), prefix)
}

// projectDeniedRunCommand returns the step the ordinary response presents for a
// run_command call that a PreToolUse hook denied. The daemon replaces the
// wrapper with a RUN_COMMAND step that keeps the wrapper's status, metadata
// (including the hook result and tool source) and error, and synthesizes the
// command from the arguments: the command line, repeated as the proposed
// command line, the working directory, a blocking call and an empty combined
// output. The metadata must record exactly one hook result, a denial whose
// reason the error repeats, and the call must carry a wait argument, which was
// observed as 5000 with the call blocking; any other presentation still fails
// the comparison.
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

// calledArguments returns the JSON arguments of the native step's tool call.
// Arguments with malformed Unicode anywhere, in a name or a value, are refused:
// encoding/json would decode them to U+FFFD, so two different calls could
// compare equal.
func calledArguments(native protoreflect.Message) (map[string]json.RawMessage, bool) {
	metadata := native.Get(native.Descriptor().Fields().ByName("metadata")).Message()
	call := metadata.Get(metadata.Descriptor().Fields().ByName("tool_call")).Message()
	text := []byte(call.Get(call.Descriptor().Fields().ByName("arguments_json")).String())
	var called map[string]json.RawMessage
	if json.Unmarshal(text, &called) != nil || !validJSONText(text) {
		return nil, false
	}
	return called, true
}

// genericArg returns a generic argument and whether the call carries it.
func genericArg(generic protoreflect.Message, name string) (string, bool) {
	args := generic.Get(generic.Descriptor().Fields().ByName("args")).Map()
	key := protoreflect.ValueOfString(name).MapKey()
	if !args.Has(key) {
		return "", false
	}
	return args.Get(key).String(), true
}

// contentOffsetPresented lets view_file drop its ContentOffset argument only
// when the presented native step carries the same canonical value, both in its
// tool-call arguments and in view_file.content_offset. Without this, a changed
// argument would vanish from the comparison. An absent argument is the shape
// qualified before agy 1.2.16 wrote ContentOffset.
func contentOffsetPresented(generic, native protoreflect.Message) bool {
	arg, ok := genericArg(generic, "ContentOffset")
	if !ok {
		return true
	}
	offset, err := strconv.ParseUint(arg, 10, 32)
	if err != nil || strconv.FormatUint(offset, 10) != arg {
		return false
	}
	nf := native.Descriptor().Fields()
	view := native.Get(nf.ByName("view_file")).Message()
	if view.Get(view.Descriptor().Fields().ByName("content_offset")).Uint() != offset {
		return false
	}
	called, ok := calledArguments(native)
	return ok && string(called["ContentOffset"]) == arg
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

type artifactMetadataArg struct {
	RequestFeedback bool
	Summary         string
	UserFacing      bool
	// Keys names the keys the object carried, sorted, so that an object that
	// spells out a false or empty value is not equal to one that omits the
	// key (review finding rv-2jgy.1), while two serializations of the same
	// object in different key orders stay equal (rv-krh4.1).
	Keys string
}

// decodeArtifactMetadata reads one JSON object with only the observed keys.
// It walks the tokens itself: encoding/json matches field names without
// regard to case and keeps the last of two spellings, so "Summary" and
// "summary" together would merge and one value would be dropped silently
// (review finding rv-tw0s.1). Each key must appear once, spelled exactly, and
// nothing may follow the object.
func decodeArtifactMetadata(text string) (meta artifactMetadataArg, ok bool) {
	decoder := json.NewDecoder(strings.NewReader(text))
	if tok, err := decoder.Token(); err != nil || tok != json.Delim('{') {
		return artifactMetadataArg{}, false
	}
	seen := map[string]bool{}
	for decoder.More() {
		tok, err := decoder.Token()
		if err != nil {
			return artifactMetadataArg{}, false
		}
		key, isString := tok.(string)
		if !isString || seen[key] {
			return artifactMetadataArg{}, false
		}
		seen[key] = true
		// Read each value as raw text first: encoding/json turns null into
		// the zero value, so false and "" would pass for a missing value
		// (review finding rv-4e8b.1). Only the literal forms are accepted.
		var raw json.RawMessage
		if err = decoder.Decode(&raw); err != nil {
			return artifactMetadataArg{}, false
		}
		switch key {
		case "RequestFeedback":
			meta.RequestFeedback, ok = literalBool(raw)
		case "Summary":
			meta.Summary, ok = literalString(raw)
		case "UserFacing":
			meta.UserFacing, ok = literalBool(raw)
		default:
			return artifactMetadataArg{}, false
		}
		if !ok {
			return artifactMetadataArg{}, false
		}
	}
	if tok, err := decoder.Token(); err != nil || tok != json.Delim('}') {
		return artifactMetadataArg{}, false
	}
	// Only end of input may follow the object.
	if _, err := decoder.Token(); err != io.EOF {
		return artifactMetadataArg{}, false
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	meta.Keys = strings.Join(keys, ",")
	return meta, true
}

// literalBool accepts exactly the JSON literals true and false; null or any
// other value is not a boolean the source presented.
func literalBool(raw json.RawMessage) (bool, bool) {
	switch string(raw) {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// literalString accepts exactly one JSON string; null, numbers and objects
// are not strings the source presented. The string must also be valid Unicode
// text: encoding/json replaces invalid UTF-8 and an unpaired surrogate escape
// such as \ud800 with U+FFFD, so two different malformed strings would decode
// to the same value and compare equal (review finding rv-1de6.1).
func literalString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || raw[0] != '"' {
		return "", false
	}
	var text string
	if json.Unmarshal(raw, &text) != nil || !validJSONText(raw) {
		return "", false
	}
	return text, true
}

// validJSONText reports whether JSON text decodes without substitution: its
// bytes are valid UTF-8, and every \u escape names a scalar value, with a high
// surrogate escape followed at once by a low surrogate escape and no low
// surrogate escape on its own. It reads escapes only; the text must otherwise
// be JSON that encoding/json accepts, where a backslash appears only inside a
// string.
func validJSONText(raw []byte) bool {
	if !utf8.Valid(raw) {
		return false
	}
	escape := func(i int) (rune, bool) {
		if i+6 > len(raw) || raw[i] != '\\' || raw[i+1] != 'u' {
			return 0, false
		}
		n, err := strconv.ParseUint(string(raw[i+2:i+6]), 16, 16)
		return rune(n), err == nil
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		r, isUnicode := escape(i)
		if !isUnicode {
			i++ // a one-character escape such as \" or \\
			continue
		}
		i += 5
		switch {
		case utf16.IsSurrogate(r) && r >= 0xdc00:
			return false
		case utf16.IsSurrogate(r):
			low, ok := escape(i + 1)
			if !ok || utf16.DecodeRune(r, low) == utf8.RuneError {
				return false
			}
			i += 6
		}
	}
	return true
}

// artifactMetadataPresented lets write_to_file drop its ArtifactMetadata
// argument only when the presented CODE_ACTION step carries the same object in
// its tool-call arguments and in code_action.artifact_metadata, and is marked
// as an artifact file. The generic argument is the JSON of the object, compared
// by value because the tool-call text may escape characters differently. Only
// RequestFeedback, Summary and UserFacing were observed, so any other key
// fails. The native step adds timestamps of its own, which it presents.
func artifactMetadataPresented(tool string, generic, native protoreflect.Message) bool {
	arg, ok := genericArg(generic, "ArtifactMetadata")
	if !ok {
		return true
	}
	called, ok := calledArguments(native)
	if !ok || tool != "write_to_file" {
		return false
	}
	meta, ok := decodeArtifactMetadata(arg)
	if again, ok2 := decodeArtifactMetadata(string(called["ArtifactMetadata"])); !ok || !ok2 || again != meta {
		return false
	}
	action := native.Get(native.Descriptor().Fields().ByName("code_action")).Message()
	af := action.Descriptor().Fields()
	if !action.Get(af.ByName("is_artifact_file")).Bool() {
		return false
	}
	presented := action.Get(af.ByName("artifact_metadata")).Message()
	pf := presented.Descriptor().Fields()
	return presented.Get(pf.ByName("summary")).String() == meta.Summary &&
		presented.Get(pf.ByName("request_feedback")).Bool() == meta.RequestFeedback &&
		presented.Get(pf.ByName("user_facing")).Bool() == meta.UserFacing
}

func projectImage(step, result protoreflect.Message, data []byte) (proto.Message, error) {
	payload, err := decode("exa.cortex.core.tools.GenerateImage", data)
	if err != nil {
		return nil, err
	}
	image := payload.ProtoReflect()
	if !onlyFields(image, fieldSet("prompt", "aspect_ratio", "image_uri", "image_num_bytes", "model_name", "image_name")) {
		return nil, errors.New("unsupported image payload fields")
	}
	media := result.Get(result.Descriptor().Fields().ByName("media")).List()
	imageURI := image.Get(image.Descriptor().Fields().ByName("image_uri")).String()
	if media.Len() != 1 || imageURI == "" || media.Get(0).Message().Get(media.Get(0).Message().Descriptor().Fields().ByName("uri")).String() != imageURI {
		return nil, errors.New("unsupported image payload/media difference")
	}
	// Unlike embedded native Steps, the image projection retains outer metadata.
	native := proto.Clone(step.Interface())
	m := native.ProtoReflect()
	f := m.Descriptor().Fields()
	m.Set(f.ByName("type"), protoreflect.ValueOfEnum(f.ByName("type").Enum().Values().ByName("CORTEX_STEP_TYPE_GENERATE_IMAGE").Number()))
	m.Clear(f.ByName("generic"))
	generated := m.Mutable(f.ByName("generate_image")).Message()
	for _, name := range []protoreflect.Name{"prompt", "image_name", "aspect_ratio", "model_name"} {
		generated.Set(generated.Descriptor().Fields().ByName(name), image.Get(image.Descriptor().Fields().ByName(name)))
	}
	generated.Set(generated.Descriptor().Fields().ByName("generated_media"), media.Get(0))
	return native, nil
}

func listsEqual(a, b protoreflect.List) bool {
	if a.Len() != b.Len() {
		return false
	}
	for i := 0; i < a.Len(); i++ {
		if !proto.Equal(a.Get(i).Message().Interface(), b.Get(i).Message().Interface()) {
			return false
		}
	}
	return true
}
