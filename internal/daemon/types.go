// Package daemon defines the wire types for the Antigravity-CLI local
// language-server daemon (Connect-RPC over plain JSON at 127.0.0.1:43555).
//
// Field names follow the JSON shape returned by GetCascadeTrajectory. Step
// payloads vary by step type; only the discriminator-relevant subset is
// strongly typed. Anything whose shape we don't need for v0 stays as
// json.RawMessage so unmarshal stays loose-friendly across versions.
package daemon

import (
	"encoding/json"
	"strings"
)

// Trajectory is the top-level shape returned by GetCascadeTrajectory under
// the "trajectory" key.
type Trajectory struct {
	TrajectoryID      string             `json:"trajectoryId,omitempty"`
	CascadeID         string             `json:"cascadeId,omitempty"`
	TrajectoryType    string             `json:"trajectoryType,omitempty"`
	Source            string             `json:"source,omitempty"`
	Steps             []Step             `json:"steps,omitempty"`
	GeneratorMetadata json.RawMessage    `json:"generatorMetadata,omitempty"`
	ExecutorMetadatas json.RawMessage    `json:"executorMetadatas,omitempty"`
	Metadata          TrajectoryMetadata `json:"metadata,omitempty"`
}

// TrajectoryMetadata is the top-level metadata block.
type TrajectoryMetadata struct {
	CreatedAt             string          `json:"createdAt,omitempty"`
	InitializationStateID string          `json:"initializationStateId,omitempty"`
	MendelExperimentIDs   json.RawMessage `json:"mendelExperimentIds,omitempty"`
	ProjectID             string          `json:"projectId,omitempty"`
}

// Step is a single entry in Trajectory.Steps. Only one of the payload fields
// is populated, selected by Type. Unknown step types unmarshal cleanly with
// all payload pointers nil.
type Step struct {
	Type     string       `json:"type,omitempty"`
	Status   string       `json:"status,omitempty"`
	Metadata StepMetadata `json:"metadata,omitempty"`

	UserInput           *UserInput       `json:"userInput,omitempty"`
	PlannerResponse     *PlannerResponse `json:"plannerResponse,omitempty"`
	RunCommand          *RunCommand      `json:"runCommand,omitempty"`
	ViewFile            *ViewFile        `json:"viewFile,omitempty"`
	CodeAction          *CodeAction      `json:"codeAction,omitempty"`
	GrepSearch          *GrepSearch      `json:"grepSearch,omitempty"`
	ErrorMessage        *ErrorMessage    `json:"errorMessage,omitempty"`
	SystemMessage       *SystemMessage   `json:"systemMessage,omitempty"`
	Checkpoint          *Checkpoint      `json:"checkpoint,omitempty"`
	ListDirectory       *ListDirectory   `json:"listDirectory,omitempty"`
	Generic             json.RawMessage  `json:"generic,omitempty"`
	ConversationHistory json.RawMessage  `json:"conversationHistory,omitempty"`
}

// StepMetadata is the per-step metadata block.
type StepMetadata struct {
	CreatedAt                string          `json:"createdAt,omitempty"`
	Source                   string          `json:"source,omitempty"`
	ExecutionID              string          `json:"executionId,omitempty"`
	SourceTrajectoryStepInfo json.RawMessage `json:"sourceTrajectoryStepInfo,omitempty"`
	InternalMetadata         json.RawMessage `json:"internalMetadata,omitempty"`
}

// UserInput is the payload for CORTEX_STEP_TYPE_USER_INPUT.
type UserInput struct {
	UserResponse    string          `json:"userResponse,omitempty"`
	Items           json.RawMessage `json:"items,omitempty"`
	ActiveUserState json.RawMessage `json:"activeUserState,omitempty"`
	UserConfig      json.RawMessage `json:"userConfig,omitempty"`
}

// PlannerResponse is the payload for CORTEX_STEP_TYPE_PLANNER_RESPONSE.
// Thinking and Response are documented by the Python reference renderer but
// not present in every fixture — keep them optional.
type PlannerResponse struct {
	MessageID  string     `json:"messageId,omitempty"`
	Thinking   string     `json:"thinking,omitempty"`
	Response   string     `json:"response,omitempty"`
	StopReason string     `json:"stopReason,omitempty"`
	ToolCalls  []ToolCall `json:"toolCalls,omitempty"`
}

// ToolCall is one entry in PlannerResponse.ToolCalls.
type ToolCall struct {
	Name          string `json:"name,omitempty"`
	ArgumentsJSON string `json:"argumentsJson,omitempty"`
	ID            string `json:"id,omitempty"`
}

// RunCommand is the payload for CORTEX_STEP_TYPE_RUN_COMMAND.
type RunCommand struct {
	CommandLine         string          `json:"commandLine,omitempty"`
	ProposedCommandLine string          `json:"proposedCommandLine,omitempty"`
	Cwd                 string          `json:"cwd,omitempty"`
	WaitMsBeforeAsync   json.RawMessage `json:"waitMsBeforeAsync,omitempty"`
	Blocking            bool            `json:"blocking,omitempty"`
	ExitCode            *int            `json:"exitCode,omitempty"`
	CombinedOutput      json.RawMessage `json:"combinedOutput,omitempty"`
}

// CombinedOutputString returns CombinedOutput as a human-readable string.
// The daemon returns this field as either a JSON string or a JSON object
// (e.g. {"stdout":"...","stderr":"..."}), so we tolerate both.
func (rc *RunCommand) CombinedOutputString() string {
	if rc == nil || len(rc.CombinedOutput) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(rc.CombinedOutput, &s); err == nil {
		return s
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(rc.CombinedOutput, &obj); err == nil {
		parts := make([]string, 0, 4)
		for _, key := range []string{"stdout", "stderr", "output", "text"} {
			if raw, ok := obj[key]; ok {
				var v string
				if json.Unmarshal(raw, &v) == nil && v != "" {
					parts = append(parts, v)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}
	return string(rc.CombinedOutput)
}

// ViewFile is the payload for CORTEX_STEP_TYPE_VIEW_FILE.
type ViewFile struct {
	AbsolutePathURI string `json:"absolutePathUri,omitempty"`
	StartLine       int    `json:"startLine,omitempty"`
	EndLine         int    `json:"endLine,omitempty"`
	Content         string `json:"content,omitempty"`
	NumLines        int    `json:"numLines,omitempty"`
	NumBytes        int    `json:"numBytes,omitempty"`
}

// CodeAction is the payload for CORTEX_STEP_TYPE_CODE_ACTION.
type CodeAction struct {
	Description      string          `json:"description,omitempty"`
	ActionSpec       json.RawMessage `json:"actionSpec,omitempty"`
	ActionResult     json.RawMessage `json:"actionResult,omitempty"`
	UseFastApply     bool            `json:"useFastApply,omitempty"`
	IsArtifactFile   bool            `json:"isArtifactFile,omitempty"`
	ArtifactMetadata json.RawMessage `json:"artifactMetadata,omitempty"`
}

// ActionResultString returns CodeAction.ActionResult as a human-readable
// string. Tolerates both string and object shapes, the same way
// RunCommand.CombinedOutputString does.
func (ca *CodeAction) ActionResultString() string {
	if ca == nil || len(ca.ActionResult) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(ca.ActionResult, &s); err == nil {
		return s
	}
	// Object — pretty-print so the renderer can fence it.
	var v any
	if err := json.Unmarshal(ca.ActionResult, &v); err == nil {
		if b, err := json.MarshalIndent(v, "", "  "); err == nil {
			return string(b)
		}
	}
	return string(ca.ActionResult)
}

// GrepSearch is the payload for CORTEX_STEP_TYPE_GREP_SEARCH.
type GrepSearch struct {
	SearchPathURI   string          `json:"searchPathUri,omitempty"`
	Query           string          `json:"query,omitempty"`
	MatchPerLine    json.RawMessage `json:"matchPerLine,omitempty"`
	CaseInsensitive bool            `json:"caseInsensitive,omitempty"`
	CommandRun      string          `json:"commandRun,omitempty"`
}

// ErrorMessage is the payload for CORTEX_STEP_TYPE_ERROR_MESSAGE.
type ErrorMessage struct {
	Error           ErrorMessageError `json:"error,omitempty"`
	ShouldShowModel bool              `json:"shouldShowModel,omitempty"`
}

// ErrorMessageError is the structured error body returned inside an
// ErrorMessage step. The daemon returns this as an object, not a string.
type ErrorMessageError struct {
	UserErrorMessage  string `json:"userErrorMessage,omitempty"`
	ModelErrorMessage string `json:"modelErrorMessage,omitempty"`
}

// SystemMessage is the payload for CORTEX_STEP_TYPE_SYSTEM_MESSAGE.
type SystemMessage struct {
	Message   string          `json:"message,omitempty"`
	EventType string          `json:"eventType,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// Checkpoint is the payload for CORTEX_STEP_TYPE_CHECKPOINT.
type Checkpoint struct {
	IntentOnly           bool     `json:"intentOnly,omitempty"`
	IncludedStepIndexEnd int      `json:"includedStepIndexEnd,omitempty"`
	UserIntent           string   `json:"userIntent,omitempty"`
	ConversationLogURIs  []string `json:"conversationLogUris,omitempty"`
	UserRequests         []string `json:"userRequests,omitempty"`
	SessionSummary       string   `json:"sessionSummary,omitempty"`
}

// ListDirectory is the payload for CORTEX_STEP_TYPE_LIST_DIRECTORY.
type ListDirectory struct {
	DirectoryPathURI string          `json:"directoryPathUri,omitempty"`
	Results          json.RawMessage `json:"results,omitempty"`
}

// LoadTrajectoryRequest is the body for the LoadTrajectory RPC.
type LoadTrajectoryRequest struct {
	CascadeID string `json:"cascadeId"`
}

// GetCascadeTrajectoryRequest is the body for the GetCascadeTrajectory RPC.
type GetCascadeTrajectoryRequest struct {
	CascadeID string `json:"cascadeId"`
}

// GetCascadeTrajectoryResponse wraps a Trajectory under the "trajectory" key.
type GetCascadeTrajectoryResponse struct {
	Trajectory Trajectory `json:"trajectory"`
}
