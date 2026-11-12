package render

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

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
			ids = append(ids, swarmChildIDs(s)...)
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

// Swarm adds return one ID (1.5.5), or parallel names/IDs arrays for n/tasks
// (1.5.6). Require matching names and counts; never extract IDs from prose or
// failed calls. These IDs only place children already linked by the resolver.
func swarmChildIDs(s daemon.Step) []string {
	if s.Status != "CORTEX_STEP_STATUS_DONE" {
		return nil
	}
	var g struct {
		Args   map[string]string `json:"args"`
		Result struct {
			Result string `json:"result"`
		} `json:"result"`
	}
	if json.Unmarshal(s.Generic, &g) != nil || g.Args["action"] != "add" || g.Args["name"] == "" {
		return nil
	}
	text := g.Result.Result
	if strings.HasPrefix(text, "Created At: ") {
		lines := strings.SplitN(text, "\n", 3)
		if len(lines) != 3 || !strings.HasPrefix(lines[1], "Completed At: ") {
			return nil
		}
		for i, prefix := range []string{"Created At: ", "Completed At: "} {
			if _, err := time.Parse(time.RFC3339, strings.TrimPrefix(lines[i], prefix)); err != nil {
				return nil
			}
		}
		text = lines[2]
	}
	var result struct {
		Status string   `json:"status"`
		Name   string   `json:"name"`
		ID     string   `json:"conversation_id"`
		Names  []string `json:"names"`
		IDs    []string `json:"conversation_ids"`
	}
	if json.Unmarshal([]byte(text), &result) != nil || result.Status != "added" {
		return nil
	}
	if len(result.Names) == 0 && len(result.IDs) == 0 {
		if n, hasN := g.Args["n"]; hasN && n != "1" {
			return nil
		}
		if _, hasTasks := g.Args["tasks"]; hasTasks {
			var tasks []string
			if _, hasN := g.Args["n"]; hasN || json.Unmarshal([]byte(g.Args["tasks"]), &tasks) != nil || len(tasks) != 1 || tasks[0] == "" {
				return nil
			}
		}
		if result.Name == g.Args["name"] && result.ID != "" {
			return []string{result.ID}
		}
		return nil
	}
	if result.Name != "" || result.ID != "" || len(result.Names) != len(result.IDs) {
		return nil
	}
	var count uint64
	if n, ok := g.Args["n"]; ok {
		if _, hasTasks := g.Args["tasks"]; hasTasks {
			return nil
		}
		var err error
		count, err = strconv.ParseUint(n, 10, 63)
		if err != nil || strconv.FormatUint(count, 10) != n {
			return nil
		}
	} else {
		var tasks []string
		if json.Unmarshal([]byte(g.Args["tasks"]), &tasks) != nil {
			return nil
		}
		for _, task := range tasks {
			if task == "" {
				return nil
			}
		}
		count = uint64(len(tasks))
	}
	if count == 0 || uint64(len(result.IDs)) != count {
		return nil
	}
	seen := map[string]bool{}
	for i, id := range result.IDs {
		if id == "" || seen[id] || result.Names[i] != g.Args["name"]+"-"+strconv.Itoa(i+1) {
			return nil
		}
		seen[id] = true
	}
	return result.IDs
}
