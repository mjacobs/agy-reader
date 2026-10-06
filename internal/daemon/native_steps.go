package daemon

import (
	"encoding/json"
	"errors"
)

// The payloads below stay json.RawMessage on Step so that a typed round trip
// (for example a sidecar written from the typed view) keeps every field the
// daemon sent, including inline media bytes this view never renders. These
// typed views decode only what the Markdown renderer shows.

// GenerateImage is the payload for CORTEX_STEP_TYPE_GENERATE_IMAGE
// (exa.cortex_pb.CortexStepGenerateImage).
type GenerateImage struct {
	Prompt         string          `json:"prompt,omitempty"`
	ImagePaths     []string        `json:"imagePaths,omitempty"`
	ImageName      string          `json:"imageName,omitempty"`
	AspectRatio    string          `json:"aspectRatio,omitempty"`
	ModelName      string          `json:"modelName,omitempty"`
	GeneratedMedia *GeneratedMedia `json:"generatedMedia,omitempty"`
	GeneratedImage *GeneratedImage `json:"generatedImage,omitempty"`
}

// GeneratedMedia is the generated image as an exa.codeium_common_pb.Media.
// InlineData is the base64 payload; it is kept only so the renderer can say
// that embedded bytes exist without printing them.
type GeneratedMedia struct {
	MimeType    string `json:"mimeType,omitempty"`
	URI         string `json:"uri,omitempty"`
	Description string `json:"description,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	InlineData  string `json:"inlineData,omitempty"`
}

// GeneratedImage is the older exa.codeium_common_pb.ImageData form of a
// generated image. Base64Data is never rendered.
type GeneratedImage struct {
	MimeType   string `json:"mimeType,omitempty"`
	URI        string `json:"uri,omitempty"`
	Caption    string `json:"caption,omitempty"`
	Base64Data string `json:"base64Data,omitempty"`
}

// InvokeSubagent is the payload for CORTEX_STEP_TYPE_INVOKE_SUBAGENT
// (exa.cortex_pb.CortexStepInvokeSubagent). Older daemons stored the step
// with no payload at all.
type InvokeSubagent struct {
	Subagents      []SubagentSpec   `json:"subagents,omitempty"`
	TaskMode       bool             `json:"taskMode,omitempty"`
	Results        []SubagentResult `json:"results,omitempty"`
	SubagentName   string           `json:"subagentName,omitempty"`
	Prompt         string           `json:"prompt,omitempty"`
	ConversationID string           `json:"conversationId,omitempty"`
}

// SubagentSpec describes one requested child agent.
type SubagentSpec struct {
	TypeName      string   `json:"typeName,omitempty"`
	Role          string   `json:"role,omitempty"`
	InitialPrompt string   `json:"initialPrompt,omitempty"`
	Inherit       bool     `json:"inherit,omitempty"`
	Branch        string   `json:"branch,omitempty"`
	WorkspaceURI  string   `json:"workspaceUri,omitempty"`
	Model         EnumName `json:"model,omitempty"`
	ModelTier     EnumName `json:"modelTier,omitempty"`
	RunAsTask     bool     `json:"runAsTask,omitempty"`
	Fork          bool     `json:"fork,omitempty"`
}

// SubagentResult identifies one child conversation the invocation created.
type SubagentResult struct {
	ConversationID string   `json:"conversationId,omitempty"`
	LogAbsoluteURI string   `json:"logAbsoluteUri,omitempty"`
	WorkspaceURIs  []string `json:"workspaceUris,omitempty"`
}

// EnumName is a protobuf enum as protojson writes it: normally the value name,
// but a bare number when the writer did not know the value. Numbers are kept
// as their decimal text so an unknown enum never fails the whole payload.
type EnumName string

// UnmarshalJSON accepts a JSON string or number.
func (e *EnumName) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*e = EnumName(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*e = EnumName(n.String())
	return nil
}

// errNoPayload reports a step whose payload field is absent.
var errNoPayload = errors.New("no payload")

// GenerateImageDetails decodes the step's generateImage payload.
func (s *Step) GenerateImageDetails() (*GenerateImage, error) {
	if s == nil || len(s.GenerateImage) == 0 {
		return nil, errNoPayload
	}
	var g GenerateImage
	if err := json.Unmarshal(s.GenerateImage, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

// InvokeSubagentDetails decodes the step's invokeSubagent payload.
func (s *Step) InvokeSubagentDetails() (*InvokeSubagent, error) {
	if s == nil || len(s.InvokeSubagent) == 0 {
		return nil, errNoPayload
	}
	var inv InvokeSubagent
	if err := json.Unmarshal(s.InvokeSubagent, &inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

// AskQuestion is the payload for CORTEX_STEP_TYPE_ASK_QUESTION
// (exa.cortex_pb.CortexStepAskQuestion): the questions agy put to the user and
// the answers given.
type AskQuestion struct {
	Questions []AskedQuestion `json:"questions,omitempty"`
}

// AskedQuestion is one question with its options and the answer given: the ids
// of the selected options, a written-in answer, whether it was skipped, or
// none when unanswered.
type AskedQuestion struct {
	Question          string           `json:"question,omitempty"`
	Options           []AskQuestionOpt `json:"options,omitempty"`
	SelectedOptionIDs []string         `json:"selectedOptionIds,omitempty"`
	WriteInResponse   string           `json:"writeInResponse,omitempty"`
	Skipped           bool             `json:"skipped,omitempty"`
}

// AskQuestionOpt is one choice offered for a question.
type AskQuestionOpt struct {
	ID   string `json:"id,omitempty"`
	Text string `json:"text,omitempty"`
}

// AskQuestionDetails decodes the step's askQuestion payload.
func (s *Step) AskQuestionDetails() (*AskQuestion, error) {
	if s == nil || len(s.AskQuestion) == 0 {
		return nil, errNoPayload
	}
	var a AskQuestion
	if err := json.Unmarshal(s.AskQuestion, &a); err != nil {
		return nil, err
	}
	return &a, nil
}
