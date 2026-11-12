package completeness

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// emptyRead156 adds the timestamped empty-file reads captured with agy 1.5.6.
// The complete saved result remains the displayed content.
func emptyRead156(content string) bool {
	if content == "File is empty" {
		return true
	}
	lines := strings.Split(content, "\n")
	if len(lines) != 3 || lines[2] != "File is empty" {
		return false
	}
	for i, prefix := range []string{"Created At: ", "Completed At: "} {
		value, ok := strings.CutPrefix(lines[i], prefix)
		if !ok {
			return false
		}
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			return false
		}
	}
	return true
}

// swarmArgs156 accepts captured swarm argument families. These calls keep their
// entire wrapper; arguments must separately match the retained tool call.
func swarmArgs156(generic protoreflect.Message) bool {
	args := tool143Get(generic, "args").Map()
	value := func(name string) string { v, _ := genericArg(generic, name); return v }
	exact := func(names ...string) bool { return args.Len() == len(names) && stringMapKeysAllowed(args, names...) }
	switch value("action") {
	case "status":
		return exact("action")
	case "models":
		return exact("action") || (exact("action", "all") && value("all") == "true")
	case "title":
		return exact("action", "title") && value("title") != ""
	case "kill":
		return exact("action", "name") && value("name") != ""
	case "model":
		return exact("action", "name", "model") && value("name") != "" && value("model") != ""
	case "add":
		if !stringMapKeysAllowed(args, "action", "brief", "model", "name", "workspace", "n", "tasks", "fork") || value("name") == "" || value("brief") == "" {
			return false
		}
		if model, ok := genericArg(generic, "model"); ok && model == "" {
			return false
		}
		if workspace, ok := genericArg(generic, "workspace"); ok && workspace != "inherit" && workspace != "share" && workspace != "branch" {
			return false
		}
		if fork, ok := genericArg(generic, "fork"); ok && fork == "" {
			return false
		}
		n, hasN := genericArg(generic, "n")
		tasks, hasTasks := genericArg(generic, "tasks")
		if hasN && hasTasks {
			return false
		}
		if hasN {
			v, err := strconv.ParseUint(n, 10, 63)
			if err != nil || v == 0 || strconv.FormatUint(v, 10) != n {
				return false
			}
		}
		if hasTasks {
			var list []string
			if json.Unmarshal([]byte(tasks), &list) != nil || len(list) == 0 {
				return false
			}
			for _, task := range list {
				if task == "" {
					return false
				}
			}
		}
		return true
	}
	return false
}

// The failed model switch and forked add captured with 1.5.6 remain generic.
// Keep their errors exactly, rather than treating them as successful additions.
func qualifyFailedSwarm156(step, generic protoreflect.Message) error {
	unsupported := errors.New("unsupported failed swarm projection")
	if statusName(step) != "CORTEX_STEP_STATUS_ERROR" || !plainFailure(step) || !argumentsPresented(generic, step) || !swarmArgs156(generic) {
		return unsupported
	}
	action, _ := genericArg(generic, "action")
	name, _ := genericArg(generic, "name")
	text := tool143Get(tool143Get(step, "error").Message(), "short_error").String()
	if action == "model" && text == "worker "+strconv.Quote(name)+" is mid-turn; switch its model once status shows it waiting" {
		return nil
	}
	if action == "add" && coordinatorFork156(step, generic) {
		if text == "failed to start worker "+strconv.Quote(name)+": executor has not processed the previous input yet" || text == "the swarm already has a member named "+strconv.Quote(name) {
			return nil
		}
	}
	return unsupported
}

func coordinatorFork156(step, generic protoreflect.Message) bool {
	workspace, _ := genericArg(generic, "workspace")
	fork, _ := genericArg(generic, "fork")
	action, _ := genericArg(generic, "action")
	return action == "add" && workspace == "branch" && fork == "coordinator" &&
		exactGenericArgs(step, generic, "action", "brief", "model", "name", "workspace", "fork")
}

// The forked child's copied history contains the in-flight add as CANCELED,
// without a result or error. The daemon keeps it unchanged.
func qualifyCanceledSwarm156(step, generic protoreflect.Message) error {
	if statusName(step) != "CORTEX_STEP_STATUS_CANCELED" || !onlyFields(step, fieldSet("type", "status", "metadata", "generic")) ||
		!swarmArgs156(generic) || !coordinatorFork156(step, generic) {
		return errors.New("unsupported canceled swarm projection")
	}
	return nil
}
