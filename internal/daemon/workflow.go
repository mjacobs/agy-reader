package daemon

import "encoding/json"

// WorkflowParentCascadeID reads the explicit directional pointer carried by a
// workflow child. Root conversation IDs and IDs in result prose are not parents.
func (t *Trajectory) WorkflowParentCascadeID() string {
	origin, ok := t.workflowOrigin()
	if !ok {
		return ""
	}
	return CanonicalCascadeID(origin.ConversationID)
}

type workflowOrigin struct {
	ConversationID string   `json:"conversationId"`
	StepIndex      *int     `json:"stepIndex"`
	ToolCall       ToolCall `json:"toolCall"`
}

func (t *Trajectory) workflowOrigin() (workflowOrigin, bool) {
	var source struct {
		Tool workflowOrigin `json:"tool"`
	}
	if t == nil || json.Unmarshal(t.Metadata.SourceMetadata, &source) != nil {
		return workflowOrigin{}, false
	}
	o := source.Tool
	parent := CanonicalCascadeID(o.ConversationID)
	child := CanonicalCascadeID(t.CascadeID)
	return o, child != "" && parent != "" && parent != child && o.StepIndex != nil && *o.StepIndex >= 0 &&
		o.ToolCall.Name == "run_workflow" && o.ToolCall.ID != "" && json.Valid([]byte(o.ToolCall.ArgumentsJSON))
}

// WorkflowStep requires the child's source record to repeat the parent's exact
// completed workflow invocation before it can establish a link or placement.
func (t *Trajectory) WorkflowStep(parent *Trajectory) (int, bool) {
	o, ok := t.workflowOrigin()
	if !ok || parent == nil || CanonicalCascadeID(o.ConversationID) != CanonicalCascadeID(parent.CascadeID) || *o.StepIndex >= len(parent.Steps) {
		return 0, false
	}
	s := parent.Steps[*o.StepIndex]
	call := s.Metadata.ToolCall
	if s.Type != "CORTEX_STEP_TYPE_GENERIC" || s.Status != "CORTEX_STEP_STATUS_DONE" || call == nil ||
		call.Name != o.ToolCall.Name || call.ID != o.ToolCall.ID || call.ArgumentsJSON != o.ToolCall.ArgumentsJSON {
		return 0, false
	}
	return *o.StepIndex, true
}
