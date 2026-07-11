package subagent_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/subagent"
)

// Placeholder cascade ids (structurally real UUIDs, no personal data).
const (
	rootID       = "11111111-1111-1111-1111-111111111111"
	childID      = "22222222-2222-2222-2222-222222222222"
	grandchildID = "33333333-3333-3333-3333-333333333333"
	unrelatedID  = "44444444-4444-4444-4444-444444444444"
)

// agentPath builds a realistic brain/ agent-definition URI naming parent.
func agentPath(parent, name string) string {
	return "file:///home/user/.gemini/antigravity-cli/brain/" + parent + "/.agents/agents/" + name
}

// writeSidecar writes a minimal sidecar linking cascade -> parent (via an
// executorMetadatas agentPath) into dir. A "" parent writes a root (no
// agentPath). ts is the first step's timestamp for stable ordering.
func writeSidecar(t *testing.T, dir, cascade, parent, agentName, ts string) {
	t.Helper()
	var execMeta string
	if parent != "" {
		execMeta = `"executorMetadatas":[{"cascadeConfig":{"plannerConfig":{"customizationConfig":{"agentPath":"` +
			agentPath(parent, agentName) + `"}}}}],`
	}
	body := `{"cascadeId":"` + cascade + `",` + execMeta +
		`"steps":[{"type":"CORTEX_STEP_TYPE_PLANNER_RESPONSE","metadata":{"createdAt":"` + ts + `"}}]}`
	path := filepath.Join(dir, cascade+".trajectory.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write sidecar %s: %v", path, err)
	}
}

func TestResolverBuildInvertsLinks(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, dir, rootID, "", "", "2026-01-01T00:00:00Z")
	writeSidecar(t, dir, childID, rootID, "child_agent", "2026-01-01T00:01:00Z")
	writeSidecar(t, dir, grandchildID, childID, "grandchild_agent", "2026-01-01T00:02:00Z")
	writeSidecar(t, dir, unrelatedID, "", "", "2026-01-01T00:03:00Z")

	r, err := subagent.Build(dir, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	kids := r.Children(rootID)
	if len(kids) != 1 || kids[0].CascadeID != childID {
		t.Fatalf("root children = %+v, want [%s]", ids(kids), childID)
	}
	gk := r.Children(childID)
	if len(gk) != 1 || gk[0].CascadeID != grandchildID {
		t.Fatalf("child children = %v, want [%s]", ids(gk), grandchildID)
	}
	if got := r.Children(grandchildID); got != nil {
		t.Errorf("grandchild should be a leaf, got %v", ids(got))
	}
	if got := r.Children(unrelatedID); got != nil {
		t.Errorf("unrelated root should have no children, got %v", ids(got))
	}
}

func TestResolverChildrenSortedByTimestamp(t *testing.T) {
	dir := t.TempDir()
	// Two children of the same parent, written newest-first on disk to prove
	// the resolver re-sorts them chronologically by first-step timestamp.
	writeSidecar(t, dir, "aaaaaaaa-0000-0000-0000-000000000000", rootID, "late", "2026-01-01T00:05:00Z")
	writeSidecar(t, dir, "bbbbbbbb-0000-0000-0000-000000000000", rootID, "early", "2026-01-01T00:01:00Z")

	r, err := subagent.Build(dir, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	kids := r.Children(rootID)
	if len(kids) != 2 {
		t.Fatalf("want 2 children, got %d", len(kids))
	}
	if kids[0].CascadeID != "bbbbbbbb-0000-0000-0000-000000000000" {
		t.Errorf("children not sorted by timestamp: %v", ids(kids))
	}
}

func TestResolverSkipsUnparseableSidecar(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, dir, childID, rootID, "child_agent", "2026-01-01T00:01:00Z")
	if err := os.WriteFile(filepath.Join(dir, "broken.trajectory.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := subagent.Build(dir, nil)
	if err != nil {
		t.Fatalf("Build should not fail on one bad sidecar: %v", err)
	}
	if len(r.Children(rootID)) != 1 {
		t.Errorf("expected the good sidecar to still index, got %v", ids(r.Children(rootID)))
	}
}

func TestResolverNilSafe(t *testing.T) {
	var r *subagent.Resolver
	if got := r.Children(rootID); got != nil {
		t.Errorf("nil resolver Children = %v, want nil", ids(got))
	}
}

func ids(ts []*daemon.Trajectory) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.CascadeID
	}
	return out
}
