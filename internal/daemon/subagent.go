package daemon

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// brainParentRe matches the parent cascade id embedded in a subagent's
// agent-definition URI. A subagent trajectory's agentPath looks like
//
//	file://.../brain/<PARENT_CASCADE_ID>/.agents/agents/<agent_name>
//
// and the parent id is the 36-char UUID path segment right after brain/.
var (
	brainParentRe = regexp.MustCompile(`/brain/([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})/\.agents/agents/`)
	cascadeIDRe   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// IsCascadeID reports whether s is a bare cascade UUID. The reader-owned
// parent field stores this unqualified form; downstream consumers add their
// own source prefix at ingestion time.
func IsCascadeID(s string) bool {
	return cascadeIDRe.MatchString(s)
}

// CanonicalCascadeID returns the lowercase representation of a bare cascade
// UUID, or an empty string when s is not a cascade UUID.
func CanonicalCascadeID(s string) string {
	if !IsCascadeID(s) {
		return ""
	}
	return strings.ToLower(s)
}

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

// AgentPathParentCascadeIDs returns the distinct parent IDs encoded in this
// trajectory's agentPath values. Multiple values are retained so the corpus
// resolver can diagnose conflicting high-confidence evidence instead of
// silently choosing the first one.
func (t *Trajectory) AgentPathParentCascadeIDs() []string {
	seen := map[string]bool{}
	for _, ap := range t.AgentPaths() {
		if m := brainParentRe.FindStringSubmatch(ap); m != nil {
			seen[CanonicalCascadeID(m[1])] = true
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// StampedParentCascadeID returns a valid reader-owned parent pointer, or an
// empty string when the block is absent or malformed.
func (t *Trajectory) StampedParentCascadeID() string {
	if t == nil || t.AgyReader == nil || !IsCascadeID(t.AgyReader.ParentCascadeID) {
		return ""
	}
	return CanonicalCascadeID(t.AgyReader.ParentCascadeID)
}

// ParentCascadeID returns the immediate parent from valid reader metadata,
// falling back to the legacy child-side agentPath layout. Evidence that needs
// sibling trajectories is handled by subagent.Backfill before this method is
// used to build a render tree.
func (t *Trajectory) ParentCascadeID() string {
	if stamped := t.StampedParentCascadeID(); stamped != "" {
		return stamped
	}
	parents := t.AgentPathParentCascadeIDs()
	if len(parents) > 0 {
		return parents[0]
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
