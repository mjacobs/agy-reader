package daemon

import (
	"encoding/json"
	"regexp"
)

// brainParentRe matches the parent cascade id embedded in a subagent's
// agent-definition URI. A subagent trajectory's agentPath looks like
//
//	file://.../brain/<PARENT_CASCADE_ID>/.agents/agents/<agent_name>
//
// and the parent id is the 36-char UUID path segment right after brain/.
var brainParentRe = regexp.MustCompile(`/brain/([0-9a-fA-F-]{36})/\.agents/agents/`)

// customizationConfig is the minimal decode of a plannerConfig's
// customizationConfig — we only need agentPath.
type customizationConfig struct {
	AgentPath string `json:"agentPath"`
}

// plannerConfigMeta is the minimal decode of a plannerConfig block, shared by
// executor and generator metadata.
type plannerConfigMeta struct {
	CustomizationConfig customizationConfig `json:"customizationConfig"`
}

// executorMetadataEntry is the minimal decode of one executorMetadatas entry:
// executorMetadatas[*].cascadeConfig.plannerConfig.customizationConfig.agentPath.
type executorMetadataEntry struct {
	CascadeConfig struct {
		PlannerConfig plannerConfigMeta `json:"plannerConfig"`
	} `json:"cascadeConfig"`
}

// generatorMetadataEntry is the minimal decode of one generatorMetadata entry:
// generatorMetadata[*].plannerConfig.customizationConfig.agentPath.
type generatorMetadataEntry struct {
	PlannerConfig plannerConfigMeta `json:"plannerConfig"`
}

// AgentPaths returns the agent-definition URIs embedded in this trajectory's
// executor/generator metadata. A SUBAGENT trajectory carries at least one
// (naming its parent's brain dir); a root trajectory carries none. The two
// metadata arrays stay json.RawMessage on Trajectory, so we decode just the
// agentPath leaf here rather than typing the whole config tree.
func (t *Trajectory) AgentPaths() []string {
	if t == nil {
		return nil
	}
	var out []string
	if len(t.ExecutorMetadatas) > 0 {
		var execs []executorMetadataEntry
		if json.Unmarshal(t.ExecutorMetadatas, &execs) == nil {
			for _, e := range execs {
				if p := e.CascadeConfig.PlannerConfig.CustomizationConfig.AgentPath; p != "" {
					out = append(out, p)
				}
			}
		}
	}
	if len(t.GeneratorMetadata) > 0 {
		var gens []generatorMetadataEntry
		if json.Unmarshal(t.GeneratorMetadata, &gens) == nil {
			for _, g := range gens {
				if p := g.PlannerConfig.CustomizationConfig.AgentPath; p != "" {
					out = append(out, p)
				}
			}
		}
	}
	return out
}

// ParentCascadeID returns the cascade id of this trajectory's parent, or ""
// when it is a root. Linkage is child-side: a subagent trajectory names its
// parent's brain dir in an agentPath URI. A cascade with no such URI — or one
// whose brain/ segment isn't a UUID (a built-in-path subagent we can't link) —
// is treated as a root, returning "" rather than crashing.
func (t *Trajectory) ParentCascadeID() string {
	for _, ap := range t.AgentPaths() {
		if m := brainParentRe.FindStringSubmatch(ap); m != nil {
			return m[1]
		}
	}
	return ""
}

// FirstStepTime returns the createdAt of the trajectory's first step as a
// stable sort key string (raw RFC3339Nano text sorts chronologically). Empty
// when there are no steps or no timestamp — callers fall back to cascade id.
func (t *Trajectory) FirstStepTime() string {
	if t == nil || len(t.Steps) == 0 {
		return ""
	}
	return t.Steps[0].Metadata.CreatedAt
}
