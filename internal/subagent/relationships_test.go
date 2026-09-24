package subagent_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/subagent"
)

func nativeInvocation(children ...string) map[string]any {
	results := []any{}
	for _, child := range children {
		results = append(results, map[string]any{"conversationId": child})
	}
	return map[string]any{
		"type":           "CORTEX_STEP_TYPE_INVOKE_SUBAGENT",
		"invokeSubagent": map[string]any{"results": results},
	}
}

func stampedParent(t *testing.T, dir, id string) string {
	t.Helper()
	traj, err := cache.Read(filepath.Join(dir, id+".trajectory.json"))
	if err != nil {
		t.Fatal(err)
	}
	return traj.StampedParentCascadeID()
}

func TestBackfillNativeParentsAcrossStreamingUpdates(t *testing.T) {
	dir := t.TempDir()
	parent := map[string]any{"cascadeId": rootID, "steps": []any{nativeInvocation(grandchildID), outboundMessageStep(childID)}}
	writeJSON(t, filepath.Join(dir, rootID+".trajectory.json"), parent)
	// A child-side pointer works even before the parent's invocation results
	// exist. The other child below has only a structured invocation result.
	child := map[string]any{"cascadeId": childID, "metadata": map[string]any{"parentConversationId": rootID}, "steps": []any{inboundMessageStep(rootID)}}
	writeJSON(t, filepath.Join(dir, childID+".trajectory.json"), child)
	writeJSON(t, filepath.Join(dir, grandchildID+".trajectory.json"), map[string]any{"cascadeId": grandchildID, "steps": []any{}})
	for round := 0; round < 2; round++ {
		report, err := subagent.Backfill(dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Diagnostics) != 0 {
			t.Fatalf("round %d: %+v", round, report)
		}
		if got := stampedParent(t, dir, rootID); got != "" {
			t.Fatalf("root acquired parent %s", got)
		}
		for _, id := range []string{childID, grandchildID} {
			if got := stampedParent(t, dir, id); got != rootID {
				t.Fatalf("child %s parent = %q", id, got)
			}
		}
		// The worker replies later. Refresh the files as a watcher would,
		// preserving existing reader stamps before running the second pass.
		if round == 0 {
			parent["steps"] = []any{nativeInvocation(childID, grandchildID), outboundMessageStep(childID), inboundMessageStep(childID)}
			writeJSON(t, filepath.Join(dir, rootID+".trajectory.json"), parent)
			child["agyReader"] = map[string]any{"parentCascadeId": rootID}
			child["steps"] = []any{inboundMessageStep(rootID), outboundMessageStep(rootID)}
			writeJSON(t, filepath.Join(dir, childID+".trajectory.json"), child)
		}
	}
}

func TestBackfillMessagesNeverEstablishAncestry(t *testing.T) {
	dir := t.TempDir()
	for id, peer := range map[string]string{rootID: childID, childID: rootID} {
		writeJSON(t, filepath.Join(dir, id+".trajectory.json"), map[string]any{
			"cascadeId": id, "steps": []any{outboundMessageStep(peer), inboundMessageStep(peer)},
		})
	}
	report, err := subagent.Backfill(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Stamped != 0 {
		t.Fatalf("messages inferred ancestry: %+v", report)
	}
}

func TestBackfillNativeParentOverridesInheritedAgentDefinition(t *testing.T) {
	dir := t.TempDir()
	// Reusing a type defined by a grandparent does not make it the parent.
	writeJSON(t, filepath.Join(dir, grandchildID+".trajectory.json"), map[string]any{
		"cascadeId":         grandchildID,
		"metadata":          map[string]any{"parentConversationId": childID},
		"executorMetadatas": []any{map[string]any{"cascadeConfig": map[string]any{"plannerConfig": map[string]any{"customizationConfig": map[string]any{"agentPath": agentPath(rootID, "shared")}}}}},
		"steps":             []any{},
	})
	if _, err := subagent.Backfill(dir, nil); err != nil {
		t.Fatal(err)
	}
	if got := stampedParent(t, dir, grandchildID); got != childID {
		t.Fatalf("parent = %q", got)
	}
}

func TestBackfillRejectsCyclesBeforeAnyStamp(t *testing.T) {
	for _, n := range []int{1, 2, 3} {
		t.Run(string(rune('0'+n)), func(t *testing.T) {
			dir := t.TempDir()
			ids := []string{rootID, childID, grandchildID}[:n]
			for i, id := range ids {
				writeJSON(t, filepath.Join(dir, id+".trajectory.json"), map[string]any{
					"cascadeId": id, "metadata": map[string]any{"parentConversationId": ids[(i+1)%n]}, "steps": []any{},
				})
			}
			// A separate child leading into the component isn't itself cyclic.
			writeSidecar(t, dir, unrelatedID, rootID, "leaf", "")
			report, err := subagent.Backfill(dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			if report.Stamped != 1 {
				t.Fatalf("report = %+v", report)
			}
			for _, id := range ids {
				if !hasDiagnostic(report, id, subagent.DiagnosticCycle) {
					t.Fatalf("missing cycle for %s: %+v", id, report)
				}
				if got := stampedParent(t, dir, id); got != "" {
					t.Fatalf("cycle stamped: %s -> %s", id, got)
				}
			}
		})
	}
}

func TestRepairExistingCycleAndUnsupportedStamp(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, rootID+".trajectory.json"), map[string]any{
		"cascadeId": rootID, "agyReader": map[string]any{"parentCascadeId": childID, "future": true},
		"steps": []any{nativeInvocation(childID), outboundMessageStep(childID), inboundMessageStep(childID)},
	})
	writeJSON(t, filepath.Join(dir, childID+".trajectory.json"), map[string]any{
		"cascadeId": childID, "metadata": map[string]any{"parentConversationId": rootID},
		"agyReader": map[string]any{"parentCascadeId": rootID}, "steps": []any{inboundMessageStep(rootID), outboundMessageStep(rootID)},
	})
	writeJSON(t, filepath.Join(dir, unrelatedID+".trajectory.json"), map[string]any{
		"cascadeId": unrelatedID, "agyReader": map[string]any{"parentCascadeId": rootID}, "steps": []any{},
	})
	// Ordinary reconciliation preserves old stamps but exposes the cycle.
	report, err := subagent.Backfill(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(report, rootID, subagent.DiagnosticCycle) || !hasDiagnostic(report, childID, subagent.DiagnosticCycle) {
		t.Fatalf("no cycle diagnostic: %+v", report)
	}
	r, err := subagent.Build(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Children(rootID)) != 1 || len(r.Children(childID)) != 0 {
		t.Fatal("cyclic edges reached render tree")
	}
	before, _ := os.ReadFile(filepath.Join(dir, childID+".trajectory.json"))
	if err := os.Chmod(filepath.Join(dir, childID+".trajectory.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err = subagent.BackfillWithOptions(dir, nil, subagent.BackfillOptions{Repair: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Cleared != 2 || report.Stamped != 0 {
		t.Fatalf("repair: %+v", report)
	}
	if got := stampedParent(t, dir, rootID); got != "" {
		t.Fatalf("root parent = %s", got)
	}
	if got := stampedParent(t, dir, unrelatedID); got != "" {
		t.Fatalf("unsupported parent = %s", got)
	}
	if got := stampedParent(t, dir, childID); got != rootID {
		t.Fatalf("child parent = %s", got)
	}
	after, _ := os.ReadFile(filepath.Join(dir, childID+".trajectory.json"))
	if string(before) != string(after) {
		t.Fatal("correct existing child link rewritten")
	}
	report, err = subagent.BackfillWithOptions(dir, nil, subagent.BackfillOptions{Repair: true})
	if err != nil || report.Cleared != 0 || report.Stamped != 0 {
		t.Fatalf("repair not idempotent: %+v, %v", report, err)
	}
	r, err = subagent.Build(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(r.Children(rootID)); len(got) != 1 || got[0] != childID {
		t.Fatalf("repaired children = %v", got)
	}
}

func TestRepairReplacesStaleParentAndClearsConflicts(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, childID+".trajectory.json"), map[string]any{
		"cascadeId": childID, "metadata": map[string]any{"parentConversationId": rootID},
		"agyReader": map[string]any{"parentCascadeId": unrelatedID}, "steps": []any{},
	})
	if _, err := subagent.BackfillWithOptions(dir, nil, subagent.BackfillOptions{Repair: true}); err != nil {
		t.Fatal(err)
	}
	if got := stampedParent(t, dir, childID); got != rootID {
		t.Fatalf("stale parent not replaced: %s", got)
	}
	writeJSON(t, filepath.Join(dir, unrelatedID+".trajectory.json"), map[string]any{"cascadeId": unrelatedID, "steps": []any{nativeInvocation(childID)}})
	report, err := subagent.BackfillWithOptions(dir, nil, subagent.BackfillOptions{Repair: true})
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(report, childID, subagent.DiagnosticConflict) || report.Cleared != 1 {
		t.Fatalf("conflicting repair: %+v", report)
	}
	if got := stampedParent(t, dir, childID); got != "" {
		t.Fatalf("conflicted parent retained: %s", got)
	}
}
