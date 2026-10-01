package daemon

// Finish is the payload for CORTEX_STEP_TYPE_FINISH, the step that carries the
// final structured result when a run uses --json-schema. Output.ArgumentsJSON
// holds the result object as a JSON string; OutputString is the same result as
// the CLI reported it.
type Finish struct {
	Output       *FinishOutput `json:"output,omitempty"`
	OutputString string        `json:"outputString,omitempty"`
}

// FinishOutput is the structured half of a Finish payload.
type FinishOutput struct {
	ArgumentsJSON string `json:"arguments_json,omitempty"`
}
