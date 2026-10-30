package render_test

import (
	"strings"
	"testing"
)

// Executor metadata shapes follow agy 1.4.11 sessions: a --print-timeout run
// stored only the prompt and no lastStepIdx; a PreInvocation hook stopped the
// turn after its injected error; ordinary turns end with NO_TOOL_CALL.
func TestMarkdownTurnsThatEndedEarly(t *testing.T) {
	const prompt = `{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"userResponse":"hello"}}`
	const blocked = `{"type":"CORTEX_STEP_TYPE_ERROR_MESSAGE","errorMessage":{"error":{"userErrorMessage":"BLOCKED"}}}`
	for _, tc := range []struct {
		name, raw string
		want      []string
	}{
		{"timeout", `{"steps":[` + prompt + `],"executorMetadatas":[{"terminationReason":"EXECUTOR_TERMINATION_REASON_USER_CANCELED","executionId":"e1"}]}`,
			[]string{"## Turns That Ended Early", "> A turn was canceled, for example with Ctrl+C or when `--print-timeout` expired (`EXECUTOR_TERMINATION_REASON_USER_CANCELED`)."}},
		{"hook", `{"steps":[` + prompt + `,` + blocked + `],"executorMetadatas":[{"terminationReason":"EXECUTOR_TERMINATION_REASON_TERMINAL_CUSTOM_HOOK","lastStepIdx":1}]}`,
			[]string{"> The turn ending at step 2 was stopped by a hook (`EXECUTOR_TERMINATION_REASON_TERMINAL_CUSTOM_HOOK`)."}},
		{"several", `{"steps":[` + prompt + `,` + prompt + `],"executorMetadatas":[{"terminationReason":"EXECUTOR_TERMINATION_REASON_ERROR","lastStepIdx":0},{"terminationReason":"EXECUTOR_TERMINATION_REASON_NO_TOOL_CALL","lastStepIdx":1},{"terminationReason":"EXECUTOR_TERMINATION_REASON_MAX_FORCED_INVOCATIONS","lastStepIdx":7}]}`,
			[]string{"> The turn ending at step 1 ended with an error", ">\n> A turn reached the limit on forced model calls"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := renderString(t, decodeTrajectory(t, tc.raw))
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Errorf("missing %q in:\n%s", want, out)
				}
			}
			if strings.Count(out, "## Turns That Ended Early") != 1 || strings.Contains(out, "NO_TOOL_CALL") {
				t.Errorf("unexpected turn endings in:\n%s", out)
			}
		})
	}
}

func TestMarkdownNormalTurnsHaveNoEndingNotice(t *testing.T) {
	for _, raw := range []string{
		`{"steps":[]}`,
		`{"steps":[],"executorMetadatas":[{"terminationReason":"EXECUTOR_TERMINATION_REASON_NO_TOOL_CALL"},{"terminationReason":"EXECUTOR_TERMINATION_REASON_TERMINAL_STEP_TYPE"},{"terminationReason":"EXECUTOR_TERMINATION_REASON_FUTURE"}]}`,
		`{"steps":[],"executorMetadatas":{"unexpected":"shape"}}`,
	} {
		if out := renderString(t, decodeTrajectory(t, raw)); strings.Contains(out, "Ended Early") {
			t.Errorf("notice rendered for %s:\n%s", raw, out)
		}
	}
}
