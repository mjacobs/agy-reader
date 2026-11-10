package completeness

import (
	"errors"
	"strconv"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// qualifySwarm155 covers the completed payload-free calls captured from the
// 1.5.5 coordinator and worker. The daemon retains these steps unchanged.
// Unobserved actions, workspace modes, failures and wrapper fields fail closed.
func qualifySwarm155(step, generic, result protoreflect.Message) error {
	unsupported := errors.New("unsupported swarm tool projection")
	if statusName(step) != "CORTEX_STEP_STATUS_DONE" ||
		!onlyFields(step, fieldSet("type", "status", "metadata", "generic")) ||
		!onlyFields(result, fieldSet("result", "full_output_uri", "step_render_info")) ||
		!argumentsPresented(generic, step) {
		return unsupported
	}
	args := generic.Get(generic.Descriptor().Fields().ByName("args")).Map()
	exact := func(names ...string) bool { return args.Len() == len(names) && stringMapKeysAllowed(args, names...) }
	value := func(name string) string { v, _ := genericArg(generic, name); return v }
	number := func(name string, positive bool) bool {
		v := value(name)
		n, err := strconv.ParseUint(v, 10, 63)
		return err == nil && strconv.FormatUint(n, 10) == v && (!positive || n > 0)
	}
	ok := false
	switch genericToolName(step) {
	case "swarm":
		switch value("action") {
		case "status", "models":
			ok = exact("action")
		case "title":
			ok = exact("action", "title") && value("title") != ""
		case "kill":
			ok = exact("action", "name") && value("name") != ""
		case "add":
			ok = exact("action", "brief", "model", "name", "workspace") && value("brief") != "" &&
				value("model") != "" && value("name") != "" && value("workspace") == "inherit"
		}
	case "read_board":
		ok = (exact("since") && number("since", false)) || (exact("limit") && number("limit", true))
	case "post_message":
		ok = exact("text") && value("text") != ""
	case "swarm_wait":
		ok = exact("since") && number("since", false)
	}
	if !ok {
		return unsupported
	}
	return nil
}
