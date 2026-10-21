package completeness

import (
	"encoding/json"
	"errors"
	"io"
	pathpkg "path"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// resultlessViewStep returns the VIEW_FILE step that the daemon presents for a
// failed resultless view_file wrapper that records a tool source
// (agy-reader#2xqw). In every retained and audited sample (12 steps in 8
// retained sessions, 2 in the agy 1.2.17 audit) the presentation keeps the
// wrapper's type-independent fields (status, metadata, error), drops the
// generic arguments, and adds a view_file with the path as a file URI and the
// one-based StartLine and EndLine as zero-based start_line and end_line.
//
// Legacy resultless reads retain the skill marker from an exact enabled skill
// registration saved by the current executor or a bounded earlier executor.
// Unregistered paths stay unmarked, regardless of their filename. The separate
// legacy registry rule below does not change modern ReadFile projection.
func resultlessViewStep(trajectory, step, generic protoreflect.Message, stepIndex int) (proto.Message, error) {
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
	if !ok || !plainAbsolutePath(path) {
		return nil, unsupported
	}
	skill, err := registeredResultlessSkillRead(trajectory, step, path, stepIndex)
	if err != nil {
		return nil, err
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
	if skill {
		view.Set(vf.ByName("is_skill_file"), protoreflect.ValueOfBool(true))
	}
	if start != 0 {
		view.Set(vf.ByName("start_line"), protoreflect.ValueOfUint32(uint32(start)))
	}
	if end != 0 {
		view.Set(vf.ByName("end_line"), protoreflect.ValueOfUint32(uint32(end)))
	}
	return out, nil
}

// registeredResultlessSkillRead covers the legacy resultless view_file marker.
// The historical capture retains one enabled skill registration from an earlier
// executor even though the current executor no longer lists that path. Earlier
// means an ordered executor ending before this exact source step; future
// registrations never establish a marker. Execution identity, registration
// uniqueness and ordering matter only when an enabled skill registration could
// mark the read. Disabled or non-skill registrations leave the read unmarked.
// Modern ReadFile keeps its separate current-execution registry rule.
func registeredResultlessSkillRead(trajectory, step protoreflect.Message, path string, stepIndex int) (bool, error) {
	unsupported := errors.New("ambiguous resultless skill registration")
	metadata := tool143Get(step, "metadata").Message()
	executionID := tool143Get(metadata, "execution_id").String()
	if executionID == "" {
		return false, nil
	}
	executors := tool143Get(trajectory, "executor_metadatas").List()
	current := -1
	bindings := 0
	for i := 0; i < executors.Len(); i++ {
		id := tool143Get(executors.Get(i).Message(), "execution_id").String()
		if id == executionID {
			bindings++
			current = i
		}
	}
	if current < 0 {
		return false, nil
	}
	var registration protoreflect.Message
	registeredAt := -1
	matches := 0
	for i := 0; i <= current; i++ {
		registry := tool143Get(tool143Get(executors.Get(i).Message(), "loaded_customizations").Message(), "customizations").List()
		for j := 0; j < registry.Len(); j++ {
			candidate := registry.Get(j).Message()
			if tool143Get(candidate, "path").String() != path {
				continue
			}
			matches++
			f := candidate.Descriptor().Fields()
			if enumName(candidate, f.ByName("type")) == "REFRESH_CUSTOMIZATION_TYPE_SKILL" &&
				enumName(candidate, f.ByName("status")) == "STATUS_ENABLED" {
				registration, registeredAt = candidate, i
			}
		}
	}
	if registration == nil {
		return false, nil
	}
	if bindings != 1 || matches != 1 {
		return false, unsupported
	}
	if registeredAt < current {
		info := tool143Get(metadata, "source_trajectory_step_info").Message()
		if stepIndex < 0 || tool143Get(info, "step_index").Uint() != uint64(stepIndex) ||
			tool143Get(info, "trajectory_id").String() == "" ||
			tool143Get(info, "trajectory_id").String() != tool143Get(trajectory, "trajectory_id").String() ||
			tool143Get(info, "cascade_id").String() != tool143Get(trajectory, "cascade_id").String() {
			return false, unsupported
		}
		previous := int64(-1)
		seen := make(map[string]bool)
		for i := 0; i <= current; i++ {
			executor := executors.Get(i).Message()
			id := tool143Get(executor, "execution_id").String()
			end := tool143Get(executor, "last_step_idx").Int()
			if id == "" || seen[id] || end <= previous ||
				(i < current && end >= int64(stepIndex)) || (i == current && end < int64(stepIndex)) {
				return false, unsupported
			}
			seen[id] = true
			previous = end
		}
	}
	return true, nil
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

// appendPresented lets write_to_file drop its Append argument only when the
// presented CODE_ACTION step records the same value twice: as the same JSON
// literal in its tool-call arguments, and in
// code_action.action_spec.create_file.append. Observed in one retained agy
// 1.2.16 session (agy-reader#s42p): the generic argument "true", the literal
// true in the tool call, and append set on the createFile spec. false was not
// observed; it is accepted on the same terms as run_command's flags, with the
// literal false in the tool call and append unset. The argument must be "true"
// or "false", and the native step's action must be a createFile spec. Absent,
// the argument qualifies nothing.
func appendPresented(tool string, generic, native protoreflect.Message) bool {
	arg, ok := genericArg(generic, "Append")
	if !ok {
		return true
	}
	if tool != "write_to_file" || (arg != "true" && arg != "false") {
		return false
	}
	called, ok := calledArguments(native)
	if !ok || string(called["Append"]) != arg {
		return false
	}
	action := native.Get(native.Descriptor().Fields().ByName("code_action")).Message()
	spec := action.Get(action.Descriptor().Fields().ByName("action_spec")).Message()
	createField := spec.Descriptor().Fields().ByName("create_file")
	if !spec.Has(createField) {
		return false
	}
	create := spec.Get(createField).Message()
	return create.Get(create.Descriptor().Fields().ByName("append")).Bool() == (arg == "true")
}
