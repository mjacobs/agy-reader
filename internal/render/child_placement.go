package render

import (
	"encoding/json"
	"strings"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

// placeChildren uses returned conversation IDs before considering the old
// chronological fallback. Only payload-free legacy invocations use that
// fallback, and never in a parent containing swarm calls, whose workers could
// otherwise be mistaken for ordinary subagents. Unmatched children stay visible
// in the trailing section. This places already-resolved children; it creates no
// parent relationships from result text.
func placeChildren(steps []daemon.Step, children []*daemon.Trajectory) (map[int][]*daemon.Trajectory, []*daemon.Trajectory) {
	byID := map[string]*daemon.Trajectory{}
	for _, c := range children {
		if c != nil && c.CascadeID != "" {
			byID[c.CascadeID] = c
		}
	}
	placed := map[int][]*daemon.Trajectory{}
	used := map[string]bool{}
	hasSwarm := false
	for i, s := range steps {
		ids := []string{}
		if s.Type == "CORTEX_STEP_TYPE_INVOKE_SUBAGENT" {
			if payload, err := s.InvokeSubagentDetails(); err == nil {
				if payload.ConversationID != "" {
					ids = append(ids, payload.ConversationID)
				}
				for _, result := range payload.Results {
					if result.ConversationID != "" {
						ids = append(ids, result.ConversationID)
					}
				}
			}
		}
		if s.Type == "CORTEX_STEP_TYPE_GENERIC" && s.Metadata.ToolCall != nil && s.Metadata.ToolCall.Name == "swarm" {
			hasSwarm = true
			if id := swarmChildID(s); id != "" {
				ids = append(ids, id)
			}
		}
		for _, id := range ids {
			if c := byID[id]; c != nil && !used[id] {
				placed[i] = append(placed[i], c)
				used[id] = true
			}
		}
	}
	if !hasSwarm {
		next := 0
		for i, s := range steps {
			if s.Type != "CORTEX_STEP_TYPE_INVOKE_SUBAGENT" || len(s.InvokeSubagent) != 0 {
				continue
			}
			for next < len(children) && (children[next] == nil || used[children[next].CascadeID]) {
				next++
			}
			if next < len(children) {
				c := children[next]
				placed[i] = append(placed[i], c)
				used[c.CascadeID] = true
				next++
			}
		}
	}
	var leftover []*daemon.Trajectory
	for _, c := range children {
		if c != nil && !used[c.CascadeID] {
			leftover = append(leftover, c)
		}
	}
	return placed, leftover
}

// A 1.5.5 swarm add returns a JSON object after two timestamp lines. Require
// the successful add shape and matching worker name, never a UUID found in
// arbitrary prose or an unsuccessful tool call.
func swarmChildID(s daemon.Step) string {
	if s.Status != "CORTEX_STEP_STATUS_DONE" {
		return ""
	}
	var g struct {
		Args   map[string]string `json:"args"`
		Result struct {
			Result string `json:"result"`
		} `json:"result"`
	}
	if json.Unmarshal(s.Generic, &g) != nil || g.Args["action"] != "add" || g.Args["name"] == "" {
		return ""
	}
	text := g.Result.Result
	if strings.HasPrefix(text, "Created At: ") {
		lines := strings.SplitN(text, "\n", 3)
		if len(lines) != 3 || !strings.HasPrefix(lines[1], "Completed At: ") {
			return ""
		}
		text = lines[2]
	}
	var result struct {
		Status string `json:"status"`
		Name   string `json:"name"`
		ID     string `json:"conversation_id"`
	}
	if json.Unmarshal([]byte(text), &result) != nil || result.Status != "added" || result.Name != g.Args["name"] {
		return ""
	}
	return result.ID
}
