package render

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

// earlyEndings explains the executor termination reasons that cut a turn
// short. Normal endings (NO_TOOL_CALL, TERMINAL_STEP_TYPE) and reasons whose
// meaning has not been observed are not reported.
var earlyEndings = map[string]string{
	"EXECUTOR_TERMINATION_REASON_USER_CANCELED":             "was canceled, for example with Ctrl+C or when `--print-timeout` expired",
	"EXECUTOR_TERMINATION_REASON_ERROR":                     "ended with an error",
	"EXECUTOR_TERMINATION_REASON_TERMINAL_CUSTOM_HOOK":      "was stopped by a hook",
	"EXECUTOR_TERMINATION_REASON_MAX_INVOCATIONS":           "reached the limit on model calls",
	"EXECUTOR_TERMINATION_REASON_MAX_FORCED_INVOCATIONS":    "reached the limit on forced model calls",
	"EXECUTOR_TERMINATION_REASON_MAX_TOKEN_BUDGET_EXCEEDED": "exceeded its token budget",
	"EXECUTOR_TERMINATION_REASON_HALTED_STEP":               "was halted at a step",
}

// writeTurnEndings notes each turn whose executor ended early, so a transcript
// that stops after a prompt does not read as a complete answer. The executor
// metadata is daemon-owned and kept raw; anything unreadable is skipped.
func writeTurnEndings(buf *bytes.Buffer, t *daemon.Trajectory) {
	var executors []struct {
		TerminationReason string `json:"terminationReason"`
		LastStepIdx       *int   `json:"lastStepIdx"`
	}
	if len(t.ExecutorMetadatas) == 0 || json.Unmarshal(t.ExecutorMetadatas, &executors) != nil {
		return
	}
	var lines []string
	for _, e := range executors {
		explanation, ok := earlyEndings[e.TerminationReason]
		if !ok {
			continue
		}
		where := "A turn"
		if e.LastStepIdx != nil && *e.LastStepIdx >= 0 && *e.LastStepIdx < len(t.Steps) {
			where = fmt.Sprintf("The turn ending at step %d", *e.LastStepIdx+1)
		}
		lines = append(lines, fmt.Sprintf("%s %s (`%s`).", where, explanation, e.TerminationReason))
	}
	if len(lines) == 0 {
		return
	}
	buf.WriteString("## Turns That Ended Early\n")
	buf.WriteString("> [!NOTE]\n")
	for i, line := range lines {
		if i > 0 {
			buf.WriteString(">\n")
		}
		fmt.Fprintf(buf, "> %s\n", line)
	}
	buf.WriteString("\n---\n\n")
}
