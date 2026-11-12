package completeness

import (
	"encoding/json"
	"errors"
	"strconv"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// qualifySwarm155 covers the completed payload-free calls captured from the
// 1.5.5 coordinator/worker and extended 1.5.6 captures. The daemon retains
// these steps unchanged. Other actions and wrapper fields fail closed;
// captured failed and canceled swarm calls have separate rules.
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
		ok = swarmArgs156(generic)
	case "read_board":
		ok = exact() || (exact("since") && number("since", false)) || (exact("limit") && number("limit", true)) ||
			(exact("since", "limit") && number("since", false) && number("limit", true)) ||
			(exact("limit", "status") && number("limit", true) && value("status") == "true")
		if exact("seqs") {
			var seqs []uint64
			ok = json.Unmarshal([]byte(value("seqs")), &seqs) == nil && len(seqs) == 1 && seqs[0] > 0
		}
	case "post_message":
		ok = exact("text") && value("text") != ""
	case "swarm_wait":
		ok = (exact("since") || (exact("since", "mentions_only") && (value("mentions_only") == "true" || value("mentions_only") == "false"))) && number("since", false)
	}
	if !ok {
		return unsupported
	}
	return nil
}
