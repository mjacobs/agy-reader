package completeness

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// skillMetadata150 qualifies only the plain name/description frontmatter seen
// in the 1.5.0 audit. It reads the persisted, numbered tool result, never the
// filesystem or incoming RPC. Other YAML forms remain unsupported.
func skillMetadata150(historical proto.Message) proto.Message {
	out := proto.Clone(historical)
	steps := tool143Get(out.ProtoReflect(), "steps").List()
	for i := 0; i < steps.Len(); i++ {
		step := steps.Get(i).Message()
		viewField := step.Descriptor().Fields().ByName("view_file")
		if !step.Has(viewField) {
			continue
		}
		view := step.Get(viewField).Message()
		if !tool143Get(view, "is_skill_file").Bool() || tool143Get(view, "start_line").Uint() != 0 || view.Has(view.Descriptor().Fields().ByName("skill_metadata")) {
			continue
		}
		lines := strings.Split(tool143Get(view, "content").String(), "\n")
		if len(lines) < 5 || !strings.HasPrefix(lines[0], "Showing lines 1-") || lines[1] != "1: ---" || lines[4] != "4: ---" {
			continue
		}
		values := make(map[string]string)
		valid := true
		for n := 2; n <= 3; n++ {
			line, ok := strings.CutPrefix(lines[n], fmt.Sprintf("%d: ", n))
			if !ok {
				valid = false
				break
			}
			key, value, ok := strings.Cut(line, ": ")
			if !ok || (key != "name" && key != "description") || values[key] != "" || !plainSkillScalar150(value) {
				valid = false
				break
			}
			values[key] = value
		}
		if !valid || len(values) != 2 {
			continue
		}
		metadata := tool143Mutable(view, "skill_metadata")
		for key, value := range values {
			tool143Set(metadata, protoreflect.Name(key), protoreflect.ValueOfString(value))
		}
	}
	return out
}

func plainSkillScalar150(s string) bool {
	if s == "" || strings.TrimSpace(s) != s || strings.ContainsAny(s, "\r\t") || strings.Contains(s, ": ") || strings.Contains(s, " #") {
		return false
	}
	if strings.ContainsRune("-?:,[]{}#&*!|>'\"%@`", rune(s[0])) {
		return false
	}
	switch strings.ToLower(s) {
	case "true", "false", "null", "~":
		return false
	}
	return true
}
