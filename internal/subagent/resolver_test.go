package subagent_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/cache"
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
	if _, err := subagent.Backfill(dir, nil); err != nil {
		t.Fatalf("Backfill: %v", err)
	}

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
	if _, err := subagent.Backfill(dir, nil); err != nil {
		t.Fatalf("Backfill: %v", err)
	}

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
	if _, err := subagent.Backfill(dir, nil); err != nil {
		t.Fatalf("Backfill: %v", err)
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

func TestBackfillModernSpawnAgentsFixture(t *testing.T) {
	dir := t.TempDir()
	const (
		parent  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		childA  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		childB  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		execID  = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		promptA = "Scan the Antigravity IDE session directories."
		promptB = "Scan the Antigravity CLI session directory."
	)
	invokeArgs, err := json.Marshal(map[string]any{"Subagents": []map[string]any{
		{"Prompt": promptA, "Role": "IDE Session Data Scanner", "TypeName": "self"},
		{"Prompt": promptB, "Role": "CLI Session Data Scanner", "TypeName": "self"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(dir, parent+".trajectory.json"), map[string]any{
		"cascadeId": parent,
		"steps": []any{
			plannerStep(execID, []any{map[string]any{"name": "invoke_subagent", "argumentsJson": string(invokeArgs)}}),
			map[string]any{"type": "CORTEX_STEP_TYPE_INVOKE_SUBAGENT", "metadata": map[string]any{"executionId": execID}},
			map[string]any{
				"type":            "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
				"metadata":        map[string]any{"executionId": execID},
				"plannerResponse": map[string]any{"response": "Spawned children " + childA + " and " + childB + "."},
			},
			inboundMessageStep(childA),
			inboundMessageStep(childB),
		},
	})
	writeModernChild(t, dir, childA, parent, promptA)
	writeModernChild(t, dir, childB, parent, promptB)

	report, err := subagent.Backfill(dir, nil)
	if err != nil {
		t.Fatalf("Backfill: %v", err)
	}
	if report.Stamped != 2 || report.Unresolved != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(report.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", report.Diagnostics)
	}
	for _, child := range []string{childA, childB} {
		traj, err := cache.Read(filepath.Join(dir, child+".trajectory.json"))
		if err != nil {
			t.Fatal(err)
		}
		if got := traj.ParentCascadeID(); got != parent {
			t.Errorf("child %s parent = %q, want %q", child, got, parent)
		}
	}
	parentRaw, err := os.ReadFile(filepath.Join(dir, parent+".trajectory.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(parentRaw), `"agyReader"`) {
		t.Error("root session was stamped")
	}

	// The historical command is safe to rerun: no bytes are rewritten and no
	// additional stamps are reported.
	before, err := os.ReadFile(filepath.Join(dir, childA+".trajectory.json"))
	if err != nil {
		t.Fatal(err)
	}
	report, err = subagent.Backfill(dir, nil)
	if err != nil {
		t.Fatalf("second Backfill: %v", err)
	}
	if report.Stamped != 0 {
		t.Fatalf("second Backfill stamped %d sidecars", report.Stamped)
	}
	after, err := os.ReadFile(filepath.Join(dir, childA+".trajectory.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("second Backfill changed sidecar bytes")
	}
}

func TestBackfillAgentPathDepthTwo(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, dir, rootID, "", "", "2026-01-01T00:00:00Z")
	writeSidecar(t, dir, childID, rootID, "child", "2026-01-01T00:01:00Z")
	writeSidecar(t, dir, grandchildID, childID, "grandchild", "2026-01-01T00:02:00Z")

	report, err := subagent.Backfill(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Stamped != 2 {
		t.Fatalf("Stamped = %d, want 2 (%+v)", report.Stamped, report)
	}
	r, err := subagent.Build(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(r.Children(rootID)); len(got) != 1 || got[0] != childID {
		t.Fatalf("root children = %v", got)
	}
	if got := ids(r.Children(childID)); len(got) != 1 || got[0] != grandchildID {
		t.Fatalf("child children = %v", got)
	}
}

func TestBackfillConflictingEvidenceLeavesChildUnstamped(t *testing.T) {
	dir := t.TempDir()
	const otherParent = "55555555-5555-5555-5555-555555555555"
	writeSidecar(t, dir, rootID, "", "", "2026-01-01T00:00:00Z")
	writeJSON(t, filepath.Join(dir, otherParent+".trajectory.json"), map[string]any{
		"cascadeId": otherParent,
		"steps":     []any{inboundMessageStep(childID)},
	})
	writeJSON(t, filepath.Join(dir, childID+".trajectory.json"), map[string]any{
		"cascadeId": childID,
		"executorMetadatas": []any{map[string]any{"cascadeConfig": map[string]any{"plannerConfig": map[string]any{"customizationConfig": map[string]any{
			"agentPath": agentPath(rootID, "child"),
		}}}}},
		"steps": []any{outboundMessageStep(otherParent)},
	})

	report, err := subagent.Backfill(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(report, childID, subagent.DiagnosticConflict) {
		t.Fatalf("missing conflict diagnostic: %+v", report.Diagnostics)
	}
	assertUnstamped(t, filepath.Join(dir, childID+".trajectory.json"))
	r, err := subagent.Build(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Children(rootID); got != nil {
		t.Fatalf("conflicted child rendered under agentPath parent: %v", ids(got))
	}
	if got := r.Children(otherParent); got != nil {
		t.Fatalf("conflicted child rendered under message parent: %v", ids(got))
	}
}

func TestBackfillCanonicalizesMixedCaseEvidence(t *testing.T) {
	dir := t.TempDir()
	upperRoot := strings.ToUpper(rootID)
	upperChild := strings.ToUpper(childID)
	writeJSON(t, filepath.Join(dir, rootID+".trajectory.json"), map[string]any{
		"cascadeId": rootID,
		"steps":     []any{inboundMessageStep(childID)},
	})
	writeJSON(t, filepath.Join(dir, childID+".trajectory.json"), map[string]any{
		"cascadeId": upperChild,
		"executorMetadatas": []any{map[string]any{"cascadeConfig": map[string]any{"plannerConfig": map[string]any{"customizationConfig": map[string]any{
			"agentPath": agentPath(upperRoot, "child"),
		}}}}},
		"steps": []any{outboundMessageStep(upperRoot)},
	})

	report, err := subagent.Backfill(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Stamped != 1 || hasDiagnostic(report, childID, subagent.DiagnosticConflict) {
		t.Fatalf("mixed-case evidence did not converge: %+v", report)
	}
	traj, err := cache.Read(filepath.Join(dir, childID+".trajectory.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := traj.StampedParentCascadeID(); got != rootID {
		t.Fatalf("stamped parent = %q, want canonical %q", got, rootID)
	}
	r, err := subagent.Build(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(r.Children(upperRoot)); len(got) != 1 || got[0] != upperChild {
		t.Fatalf("uppercase parent lookup children = %v", got)
	}
}

func TestBackfillStaleStampIsDiagnosedAndNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	const stampedParent = "55555555-5555-5555-5555-555555555555"
	writeSidecar(t, dir, rootID, "", "", "2026-01-01T00:00:00Z")
	writeJSON(t, filepath.Join(dir, stampedParent+".trajectory.json"), map[string]any{"cascadeId": stampedParent, "steps": []any{}})
	writeJSON(t, filepath.Join(dir, childID+".trajectory.json"), map[string]any{
		"cascadeId": childID,
		"agyReader": map[string]any{"parentCascadeId": stampedParent},
		"executorMetadatas": []any{map[string]any{"cascadeConfig": map[string]any{"plannerConfig": map[string]any{"customizationConfig": map[string]any{
			"agentPath": agentPath(rootID, "child"),
		}}}}},
		"steps": []any{},
	})

	report, err := subagent.Backfill(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(report, childID, subagent.DiagnosticStaleStamp) {
		t.Fatalf("missing stale-stamp diagnostic: %+v", report.Diagnostics)
	}
	traj, err := cache.Read(filepath.Join(dir, childID+".trajectory.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := traj.ParentCascadeID(); got != stampedParent {
		t.Errorf("stale stamp overwritten: got %s want %s", got, stampedParent)
	}
}

func TestBackfillMissingParentIsNonFatalAndDiagnosed(t *testing.T) {
	dir := t.TempDir()
	const missingParent = "66666666-6666-6666-6666-666666666666"
	writeSidecar(t, dir, childID, missingParent, "child", "2026-01-01T00:00:00Z")

	report, err := subagent.Backfill(dir, nil)
	if err != nil {
		t.Fatalf("missing parent must not fail the corpus: %v", err)
	}
	if report.Stamped != 1 || !hasDiagnostic(report, childID, subagent.DiagnosticMissingParent) {
		t.Fatalf("unexpected report: %+v", report)
	}
	traj, err := cache.Read(filepath.Join(dir, childID+".trajectory.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := traj.ParentCascadeID(); got != missingParent {
		t.Errorf("parent = %q, want %q", got, missingParent)
	}
}

func TestBackfillSingleOutboundMessageIsInsufficient(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, rootID+".trajectory.json"), map[string]any{"cascadeId": rootID, "steps": []any{}})
	writeJSON(t, filepath.Join(dir, childID+".trajectory.json"), map[string]any{
		"cascadeId": childID,
		"steps":     []any{outboundMessageStep(rootID)},
	})
	report, err := subagent.Backfill(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Stamped != 0 {
		t.Fatalf("ambiguous message stamped a parent: %+v", report)
	}
	assertUnstamped(t, filepath.Join(dir, childID+".trajectory.json"))
}

func plannerStep(executionID string, toolCalls []any) map[string]any {
	return map[string]any{
		"type":            "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
		"metadata":        map[string]any{"executionId": executionID},
		"plannerResponse": map[string]any{"toolCalls": toolCalls},
	}
}

func inboundMessageStep(sender string) map[string]any {
	return map[string]any{
		"type": "CORTEX_STEP_TYPE_SYSTEM_MESSAGE",
		"systemMessage": map[string]any{
			"eventType": "agent_message",
			"message":   "[Message] timestamp=2026-07-15T06:04:00Z sender=" + sender + " priority=MESSAGE_PRIORITY_HIGH content=done",
		},
	}
}

func outboundMessageStep(recipient string) map[string]any {
	return map[string]any{
		"type": "CORTEX_STEP_TYPE_GENERIC",
		"generic": map[string]any{"args": map[string]any{
			"Message":   "done",
			"Recipient": recipient,
		}},
	}
}

func writeModernChild(t *testing.T, dir, child, parent, prompt string) {
	t.Helper()
	writeJSON(t, filepath.Join(dir, child+".trajectory.json"), map[string]any{
		"cascadeId": child,
		"steps": []any{
			map[string]any{"type": "CORTEX_STEP_TYPE_USER_INPUT", "userInput": map[string]any{"userResponse": prompt}},
			outboundMessageStep(parent),
		},
	})
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasDiagnostic(report subagent.BackfillReport, cascade, kind string) bool {
	for _, d := range report.Diagnostics {
		if d.CascadeID == cascade && d.Kind == kind {
			return true
		}
	}
	return false
}

func assertUnstamped(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"agyReader"`) {
		t.Errorf("sidecar unexpectedly stamped:\n%s", b)
	}
}

func ids(ts []*daemon.Trajectory) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.CascadeID
	}
	return out
}
