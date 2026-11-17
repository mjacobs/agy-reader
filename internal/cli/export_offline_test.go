package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
)

const offlineFixture = `{"cascadeId":"offline-fixture","metadata":{},"generatorMetadata":[{"plannerConfig":{"planExecuteConfig":{"executionModel":1669}}}],"executorMetadatas":[{"cascadeConfig":{"plannerConfig":{"planExecuteConfig":{"executionModel":1669}}}}],"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","status":"CORTEX_STEP_STATUS_DONE","metadata":{},"userInput":{"userResponse":"Offline hello"}}]}`

func TestExportOfflineStandalone(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(500) }))
	defer server.Close()
	t.Setenv("ANTIGRAVITY_DAEMON_URL", server.URL)
	t.Setenv("ANTIGRAVITY_CLI_ROOT", filepath.Join(t.TempDir(), "missing"))
	dir := t.TempDir()
	path := filepath.Join(dir, "session.db")
	db := testfixture.Source(t, path, []byte(offlineFixture))
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	// Adjacent cached content must never supply the offline transcript.
	if err := os.WriteFile(filepath.Join(dir, "session.trajectory.json"), []byte(`{"cascadeId":"wrong","steps":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"json", "md"} {
		var out, diagnostics bytes.Buffer
		if err := Run([]string{"export-offline", "--format=" + format, path}, &out, &diagnostics); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Offline hello") || strings.Contains(out.String(), "wrong") {
			t.Fatal("missing source text or used sidecar")
		}
		if format == "json" {
			var value map[string]json.RawMessage
			if err := json.Unmarshal(out.Bytes(), &value); err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(value["agyReader"], []byte(`"offline"`)) || bytes.Contains(value["agyReader"], []byte(`"completeness"`)) {
				t.Fatal("incorrect provenance")
			}
		} else if !strings.HasPrefix(out.String(), offlineMarkdownNotice) {
			t.Fatal("missing offline notice")
		}
	}
	if requests.Load() != 0 {
		t.Fatal("offline command contacted a daemon")
	}
	target := filepath.Join(dir, "export.json")
	if err := Run([]string{"export-offline", "--format=json", "--out=" + target, path}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode %v", info.Mode())
	}
}

func TestExportOfflineCommittedWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.db")
	db := testfixture.Source(t, path, []byte(`{"cascadeId":"wal-fixture","steps":[]}`))
	testfixture.Exec(t, db, "PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0;")
	step := testfixture.Message(t, "gemini_coder.Step", []byte(`{"type":"CORTEX_STEP_TYPE_USER_INPUT","status":"CORTEX_STEP_STATUS_DONE","metadata":{},"userInput":{"userResponse":"Committed only in WAL"}}`))
	testfixture.InsertStep(t, db, 0, step.ProtoReflect())
	if info, err := os.Stat(path + "-wal"); err != nil || info.Size() == 0 {
		t.Fatal("fixture has no WAL")
	}
	var out bytes.Buffer
	if err := Run([]string{"export-offline", "--format=json", path}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Committed only in WAL") {
		t.Fatal("committed WAL step omitted")
	}
}

func TestExportOfflineRefusesExistingOutputs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.db")
	db := testfixture.Source(t, path, []byte(offlineFixture))
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	dangling := filepath.Join(dir, "dangling")
	if err := os.Symlink(filepath.Join(dir, "absent"), dangling); err != nil {
		t.Fatal(err)
	}
	hard := filepath.Join(dir, "hard")
	if err := os.Link(path, hard); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{path, link, dangling, hard, dir} {
		if err := Run([]string{"export-offline", "--out=" + target, path}, io.Discard, io.Discard); err == nil {
			t.Fatalf("overwrote %s", target)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("changed source")
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".agy-export-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("temporary export leaked")
	}
}

func TestExportOfflineErrorsAndEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.db")
	db := testfixture.Source(t, path, []byte(`{"cascadeId":"empty","steps":[]}`))
	var out bytes.Buffer
	if err := Run([]string{"export-offline", "--format=json", path}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Steps []json.RawMessage `json:"steps"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Steps == nil || len(result.Steps) != 0 {
		t.Fatal("empty steps not encoded as array")
	}
	sentinel := errors.New("write failed")
	if err := Run([]string{"export-offline", path}, offlineErrorWriter{sentinel}, io.Discard); !errors.Is(err, sentinel) {
		t.Fatalf("write error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := RunExportOffline(ctx, []string{path}, io.Discard, io.Discard); err == nil {
		t.Fatal("ignored cancellation")
	}
	for _, args := range [][]string{nil, {"--format=both", path}, {path, path}, {filepath.Join(dir, "missing.db")}, {dir}, {"session.pb"}} {
		out.Reset()
		if err := RunExportOffline(context.Background(), args, &out, io.Discard); err == nil || out.Len() != 0 {
			t.Fatalf("invalid input accepted: %v", args)
		}
	}
	testfixture.Exec(t, db, "PRAGMA user_version=999")
	target := filepath.Join(dir, "unsupported.json")
	if err := Run([]string{"export-offline", "--out=" + target, path}, io.Discard, io.Discard); err == nil {
		t.Fatal("unsupported schema accepted")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("unsupported input created output")
	}
}

type offlineErrorWriter struct{ err error }

func (w offlineErrorWriter) Write([]byte) (int, error) { return 0, w.err }

func TestNewExportRemovesFailedWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.json")
	failure := errors.New("input read failed")
	reader := io.MultiReader(strings.NewReader("partial content"), offlineErrorReader{failure})
	if err := writeNewExport(path, reader); !errors.Is(err, failure) {
		t.Fatalf("lost write error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("failed export left partial output")
	}
}

type offlineErrorReader struct{ err error }

func (r offlineErrorReader) Read([]byte) (int, error) { return 0, r.err }

func writeOfflineTestDB(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	db := testfixture.Source(t, path, raw)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func offlineUserStep(text, createdAt string) map[string]any {
	return map[string]any{
		"type":      "CORTEX_STEP_TYPE_USER_INPUT",
		"status":    "CORTEX_STEP_STATUS_DONE",
		"metadata":  map[string]any{"createdAt": createdAt},
		"userInput": map[string]any{"userResponse": text},
	}
}

func offlinePlannerStep(text, createdAt string, toolCalls ...map[string]any) map[string]any {
	resp := map[string]any{"response": text}
	if len(toolCalls) > 0 {
		resp["toolCalls"] = toolCalls
	}
	return map[string]any{
		"type":            "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
		"status":          "CORTEX_STEP_STATUS_DONE",
		"metadata":        map[string]any{"createdAt": createdAt},
		"plannerResponse": resp,
	}
}

func TestExportOfflineWithChildrenRejectsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.db")
	db := testfixture.Source(t, path, []byte(offlineFixture))
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "out.json")
	var out bytes.Buffer
	err := Run([]string{"export-offline", "--with-children", "--format=json", "--out=" + target, path}, &out, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "--with-children") {
		t.Fatalf("expected explicit --with-children error, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected stdout: %s", out.String())
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatal("rejected invocation created output file")
	}
}

func TestExportOfflineWithChildrenNestsSubagentSwarmAndWorkflow(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("ANTIGRAVITY_DAEMON_URL", server.URL)
	t.Setenv("ANTIGRAVITY_CLI_ROOT", filepath.Join(t.TempDir(), "missing"))

	const (
		rootID          = "11111111-1111-4111-8111-111111111111"
		rootUpperID     = "11111111-1111-4111-8111-111111111111"
		subagentChildID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
		swarmChildID    = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
		workflowChildID = "44444444-4444-4444-8444-444444444444"
		grandchildID    = "55555555-5555-4555-8555-555555555555"
		unrelatedID     = "66666666-6666-4666-8666-666666666666"
	)

	dir := t.TempDir()
	rootPath := filepath.Join(dir, rootID+".db")

	swarmArgs := map[string]any{"action": "add", "name": "worker", "brief": "Swarm child brief", "workspace": "inherit"}
	swarmArgsJSON, _ := json.Marshal(swarmArgs)
	swarmResult, _ := json.Marshal(map[string]any{"status": "added", "name": "worker", "conversation_id": strings.ToUpper(swarmChildID)})

	wfCallArgs := `{"Description":"Workflow test","ScriptPath":"/fixture/work/wf.py","WaitMsBeforeAsync":10000}`
	wfCall := map[string]any{"id": "call-wf", "name": "run_workflow", "argumentsJson": wfCallArgs}

	writeOfflineTestDB(t, rootPath, map[string]any{
		"cascadeId": strings.ToUpper(rootUpperID),
		"metadata":  map[string]any{},
		"steps": []any{
			map[string]any{
				"type":     "CORTEX_STEP_TYPE_INVOKE_SUBAGENT",
				"status":   "CORTEX_STEP_STATUS_DONE",
				"metadata": map[string]any{"createdAt": "2026-11-16T00:00:00Z"},
				"invokeSubagent": map[string]any{
					"subagents": []any{map[string]any{"typeName": "research", "role": "Researcher", "initialPrompt": "Inspect child", "inherit": true}},
					"results":   []any{map[string]any{"conversationId": subagentChildID}},
				},
			},
			map[string]any{
				"type":   "CORTEX_STEP_TYPE_GENERIC",
				"status": "CORTEX_STEP_STATUS_DONE",
				"metadata": map[string]any{
					"createdAt": "2026-11-16T00:00:01Z",
					"toolCall":  map[string]any{"name": "swarm", "argumentsJson": string(swarmArgsJSON)},
				},
				"generic": map[string]any{
					"args": swarmArgs,
					"result": map[string]any{
						"result":         "Created At: 2026-11-16T00:00:01Z\nCompleted At: 2026-11-16T00:00:02Z\n" + string(swarmResult),
						"fullOutputUri":  "file:///fixture/swarm.txt",
						"stepRenderInfo": map[string]any{},
					},
				},
			},
			map[string]any{
				"type":   "CORTEX_STEP_TYPE_GENERIC",
				"status": "CORTEX_STEP_STATUS_DONE",
				"metadata": map[string]any{
					"createdAt": "2026-11-16T00:00:02Z",
					"toolCall":  wfCall,
				},
				"generic": map[string]any{
					"args": map[string]any{
						"Description":       "Workflow test",
						"ScriptPath":        "/fixture/work/wf.py",
						"WaitMsBeforeAsync": "10000",
					},
					"result": map[string]any{
						"result":         "Created At: 2026-11-16T00:00:02Z\nCompleted At: 2026-11-16T00:00:03Z\nWorkflow \"Workflow test\" completed.\nOutput:\nOK",
						"fullOutputUri":  "file:///fixture/wf.txt",
						"stepRenderInfo": map[string]any{},
					},
				},
			},
		},
	})

	// Subagent child invokes a grandchild using legacy prompt + UUID correlation.
	writeOfflineTestDB(t, filepath.Join(dir, subagentChildID+".db"), map[string]any{
		"cascadeId": strings.ToUpper(subagentChildID),
		"metadata":  map[string]any{},
		"steps": []any{
			offlineUserStep("Inspect child", "2026-11-16T00:00:01Z"),
			offlinePlannerStep("Delegating grandchild", "2026-11-16T00:00:02Z", map[string]any{
				"name":          "spawn_agents",
				"argumentsJson": `{"subagents":[{"prompt":"Run grandchild check"}]}`,
			}),
			map[string]any{
				"type":     "CORTEX_STEP_TYPE_INVOKE_SUBAGENT",
				"status":   "CORTEX_STEP_STATUS_DONE",
				"metadata": map[string]any{"createdAt": "2026-11-16T00:00:02.5Z"},
			},
			offlinePlannerStep("Grandchild finished: "+grandchildID, "2026-11-16T00:00:04Z"),
		},
	})

	writeOfflineTestDB(t, filepath.Join(dir, grandchildID+".db"), map[string]any{
		"cascadeId": grandchildID,
		"metadata":  map[string]any{},
		"steps": []any{
			offlineUserStep("Run grandchild check", "2026-11-16T00:00:03Z"),
			offlinePlannerStep("GRANDCHILD_OK", "2026-11-16T00:00:03.5Z"),
		},
	})

	// Swarm child links via parentConversationId (and overrides an inherited agentPath).
	writeOfflineTestDB(t, filepath.Join(dir, swarmChildID+".db"), map[string]any{
		"cascadeId": swarmChildID,
		"metadata":  map[string]any{"parentConversationId": rootID},
		"executorMetadatas": []any{map[string]any{
			"cascadeConfig": map[string]any{"plannerConfig": map[string]any{"customizationConfig": map[string]any{
				"agentPath": "file:///fixture/brain/" + unrelatedID + "/.agents/agents/worker",
			}}},
		}},
		"steps": []any{
			offlinePlannerStep("SWARM_CHILD_OK", "2026-11-16T00:00:01.5Z"),
		},
	})

	// Workflow child links via sourceMetadata.tool.
	writeOfflineTestDB(t, filepath.Join(dir, workflowChildID+".db"), map[string]any{
		"cascadeId": workflowChildID,
		"metadata": map[string]any{
			"sourceMetadata": map[string]any{
				"tool": map[string]any{
					"conversationId": rootID,
					"stepIndex":      2,
					"toolCall":       wfCall,
					"title":          "Workflow worker",
				},
			},
		},
		"steps": []any{
			offlinePlannerStep("WORKFLOW_CHILD_OK", "2026-11-16T00:00:02.5Z"),
		},
	})

	// Unrelated sibling must never appear in the exported transcript.
	writeOfflineTestDB(t, filepath.Join(dir, unrelatedID+".db"), map[string]any{
		"cascadeId": unrelatedID,
		"metadata":  map[string]any{},
		"steps":     []any{offlinePlannerStep("UNRELATED_SECRET", "2026-11-16T00:00:00Z")},
	})

	// Adjacent sidecar files must be ignored and never modified.
	sidecarPath := filepath.Join(dir, rootID+".trajectory.json")
	sidecarContent := []byte(`{"cascadeId":"wrong-sidecar","steps":[]}`)
	if err := os.WriteFile(sidecarPath, sidecarContent, 0600); err != nil {
		t.Fatal(err)
	}

	var plainOut bytes.Buffer
	if err := Run([]string{"export-offline", rootPath}, &plainOut, io.Discard); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plainOut.String(), "GRANDCHILD_OK") || strings.Contains(plainOut.String(), "SWARM_CHILD_OK") {
		t.Fatal("plain export-offline nested children without --with-children")
	}

	var out, stderr bytes.Buffer
	if err := Run([]string{"export-offline", "--with-children", rootPath}, &out, &stderr); err != nil {
		t.Fatalf("export-offline --with-children failed: %v (stderr: %s)", err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr warnings: %s", stderr.String())
	}
	md := out.String()
	if !strings.HasPrefix(md, offlineMarkdownNotice) {
		t.Fatal("missing offline notice")
	}
	for _, want := range []string{
		"<code>" + strings.ToUpper(subagentChildID) + "</code>",
		"<code>" + swarmChildID + "</code>",
		"<code>" + workflowChildID + "</code>",
		"<code>" + grandchildID + "</code>",
		"GRANDCHILD_OK",
		"SWARM_CHILD_OK",
		"WORKFLOW_CHILD_OK",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in markdown output:\n%s", want, md)
		}
	}
	for _, unwanted := range []string{"UNRELATED_SECRET", "wrong-sidecar", "## Subagent Transcripts"} {
		if strings.Contains(md, unwanted) {
			t.Fatalf("unexpected %q in markdown output:\n%s", unwanted, md)
		}
	}
	// Verify relative step placement: subagent (and nested grandchild) before swarm before workflow.
	swarmIndex := strings.Index(md, "SWARM_CHILD_OK")
	if strings.Index(md, "GRANDCHILD_OK") >= swarmIndex ||
		swarmIndex >= strings.Index(md, "WORKFLOW_CHILD_OK") {
		t.Fatalf("children not placed under their respective invoking steps:\n%s", md)
	}

	// Verify no files on disk were created or modified.
	afterSidecar, err := os.ReadFile(sidecarPath)
	if err != nil || !bytes.Equal(afterSidecar, sidecarContent) {
		t.Fatal("export-offline modified existing sidecar")
	}
	sidecars, err := filepath.Glob(filepath.Join(dir, "*.trajectory.json"))
	if err != nil || len(sidecars) != 1 {
		t.Fatalf("export-offline created unexpected sidecars: %v", sidecars)
	}
	if requests.Load() != 0 {
		t.Fatal("offline command contacted a daemon")
	}
}

func TestExportOfflineWithChildrenSkipsBadSiblingsWithWarning(t *testing.T) {
	const (
		rootID  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		childID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	)
	dir := t.TempDir()
	rootPath := filepath.Join(dir, rootID+".db")
	writeOfflineTestDB(t, rootPath, map[string]any{
		"cascadeId": rootID,
		"metadata":  map[string]any{},
		"steps": []any{
			map[string]any{
				"type":     "CORTEX_STEP_TYPE_INVOKE_SUBAGENT",
				"status":   "CORTEX_STEP_STATUS_DONE",
				"metadata": map[string]any{"createdAt": "2026-11-16T00:00:00Z"},
				"invokeSubagent": map[string]any{
					"results": []any{map[string]any{"conversationId": childID}},
				},
			},
		},
	})
	writeOfflineTestDB(t, filepath.Join(dir, childID+".db"), map[string]any{
		"cascadeId": childID,
		"metadata":  map[string]any{},
		"steps":     []any{offlinePlannerStep("VALID_CHILD_OK", "2026-11-16T00:00:01Z")},
	})

	if err := os.WriteFile(filepath.Join(dir, "corrupt.db"), []byte("not sqlite"), 0600); err != nil {
		t.Fatal(err)
	}
	unsupportedDB := testfixture.Source(t, filepath.Join(dir, "unsupported.db"), []byte(`{"cascadeId":"cccccccc-cccc-4ccc-8ccc-cccccccccccc","steps":[]}`))
	testfixture.Exec(t, unsupportedDB, "PRAGMA user_version=999")
	if err := unsupportedDB.Close(); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(dir, "nested")
	if err := os.Mkdir(subDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(subDir, filepath.Join(dir, "dirlink.db")); err != nil {
		t.Fatal(err)
	}

	var out, stderr bytes.Buffer
	if err := Run([]string{"export-offline", "--with-children", rootPath}, &out, &stderr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "VALID_CHILD_OK") {
		t.Fatalf("valid child omitted:\n%s", out.String())
	}
	warnings := stderr.String()
	for _, bad := range []string{"corrupt.db", "unsupported.db", "dirlink.db"} {
		if !strings.Contains(warnings, bad) {
			t.Fatalf("expected warning for %s in stderr:\n%s", bad, warnings)
		}
	}
}

func TestExportOfflineWithChildrenRejectsConflictsAndCycles(t *testing.T) {
	const (
		rootID     = "10101010-1010-4010-8010-101010101010"
		conflictID = "20202020-2020-4020-8020-202020202020"
		otherID    = "30303030-3030-4030-8030-303030303030"
		cycleA     = "40404040-4040-4040-8040-404040404040"
		cycleB     = "50505050-5050-4050-8050-505050505050"
	)
	dir := t.TempDir()
	rootPath := filepath.Join(dir, rootID+".db")

	writeOfflineTestDB(t, rootPath, map[string]any{
		"cascadeId": rootID,
		"metadata":  map[string]any{},
		"steps": []any{
			map[string]any{
				"type":     "CORTEX_STEP_TYPE_INVOKE_SUBAGENT",
				"status":   "CORTEX_STEP_STATUS_DONE",
				"metadata": map[string]any{"createdAt": "2026-11-16T00:00:00Z"},
				"invokeSubagent": map[string]any{
					"results": []any{map[string]any{"conversationId": conflictID}},
				},
			},
		},
	})
	// Conflict: parentConversationId names otherID while rootID invokes conflictID.
	writeOfflineTestDB(t, filepath.Join(dir, conflictID+".db"), map[string]any{
		"cascadeId": conflictID,
		"metadata":  map[string]any{"parentConversationId": otherID},
		"steps":     []any{offlinePlannerStep("CONFLICT_HIDDEN", "2026-11-16T00:00:01Z")},
	})
	writeOfflineTestDB(t, filepath.Join(dir, otherID+".db"), map[string]any{
		"cascadeId": otherID,
		"metadata":  map[string]any{},
		"steps":     []any{},
	})

	var out, stderr bytes.Buffer
	if err := Run([]string{"export-offline", "--with-children", rootPath}, &out, &stderr); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "CONFLICT_HIDDEN") {
		t.Fatal("conflicting child leaked into transcript")
	}
	if !strings.Contains(stderr.String(), conflictID) {
		t.Fatalf("expected conflict warning for %s, got: %s", conflictID, stderr.String())
	}

	// Cycle: cycleA <-> cycleB via parentConversationId.
	cycleDir := t.TempDir()
	cycleAPath := filepath.Join(cycleDir, cycleA+".db")
	writeOfflineTestDB(t, cycleAPath, map[string]any{
		"cascadeId": cycleA,
		"metadata":  map[string]any{"parentConversationId": cycleB},
		"steps":     []any{offlinePlannerStep("CYCLE_A_BODY", "2026-11-16T00:00:00Z")},
	})
	writeOfflineTestDB(t, filepath.Join(cycleDir, cycleB+".db"), map[string]any{
		"cascadeId": cycleB,
		"metadata":  map[string]any{"parentConversationId": cycleA},
		"steps":     []any{offlinePlannerStep("CYCLE_B_BODY", "2026-11-16T00:00:01Z")},
	})

	out.Reset()
	stderr.Reset()
	if err := Run([]string{"export-offline", "--with-children", cycleAPath}, &out, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "CYCLE_A_BODY") || strings.Contains(out.String(), "CYCLE_B_BODY") {
		t.Fatalf("cycle not excluded cleanly:\n%s", out.String())
	}
	if !strings.Contains(stderr.String(), "cycle") {
		t.Fatalf("expected cycle warning, got: %s", stderr.String())
	}
}

func TestExportOfflineWithChildrenIgnoresForkedHistoryInvocations(t *testing.T) {
	const (
		parentID = "11111111-1111-4111-8111-111111111111"
		childID  = "22222222-2222-4222-8222-222222222222"
		forkID   = "33333333-3333-4333-8333-333333333333"
	)
	dir := t.TempDir()
	parentPath := filepath.Join(dir, parentID+".db")
	copiedInvoke := map[string]any{
		"type":   "CORTEX_STEP_TYPE_INVOKE_SUBAGENT",
		"status": "CORTEX_STEP_STATUS_DONE",
		"metadata": map[string]any{
			"createdAt":                "2026-11-16T00:00:00Z",
			"sourceTrajectoryStepInfo": map[string]any{"trajectoryId": parentID, "stepIndex": 0, "cascadeId": parentID},
		},
		"invokeSubagent": map[string]any{
			"results": []any{map[string]any{"conversationId": childID}},
		},
	}

	writeOfflineTestDB(t, parentPath, map[string]any{
		"cascadeId": parentID,
		"metadata":  map[string]any{},
		"steps": []any{
			map[string]any{
				"type":     "CORTEX_STEP_TYPE_INVOKE_SUBAGENT",
				"status":   "CORTEX_STEP_STATUS_DONE",
				"metadata": map[string]any{"createdAt": "2026-11-16T00:00:00Z"},
				"invokeSubagent": map[string]any{
					"results": []any{map[string]any{"conversationId": childID}},
				},
			},
		},
	})
	writeOfflineTestDB(t, filepath.Join(dir, childID+".db"), map[string]any{
		"cascadeId": childID,
		"metadata":  map[string]any{"parentConversationId": parentID},
		"steps":     []any{offlinePlannerStep("ORIGINAL_CHILD_BODY", "2026-11-16T00:00:01Z")},
	})
	writeOfflineTestDB(t, filepath.Join(dir, forkID+".db"), map[string]any{
		"cascadeId": forkID,
		"metadata":  map[string]any{"parentConversationId": parentID},
		"steps": []any{
			copiedInvoke,
			offlinePlannerStep("FORK_BODY", "2026-11-16T00:00:02Z"),
		},
	})

	var out, stderr bytes.Buffer
	if err := Run([]string{"export-offline", "--with-children", parentPath}, &out, &stderr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr warnings: %s", stderr.String())
	}
	md := out.String()
	childIdx := strings.Index(md, "ORIGINAL_CHILD_BODY")
	trailingIdx := strings.Index(md, "## Subagent Transcripts")
	if childIdx < 0 {
		t.Fatalf("original child omitted due to forked invocation:\n%s", md)
	}
	if trailingIdx >= 0 && childIdx > trailingIdx {
		t.Fatalf("original child misplaced into trailing section:\n%s", md)
	}
}

func TestExportOfflineWithChildrenPlacesForkChildAtLocalInvoke(t *testing.T) {
	const (
		originalID = "11111111-1111-4111-8111-111111111111"
		forkID     = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		childID    = "22222222-2222-4222-8222-222222222222"
		workflowID = "33333333-3333-4333-8333-333333333333"
	)
	for _, name := range []string{"payload-free fallback", "explicit result", "inherited workflow"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			forkPath := filepath.Join(dir, forkID+".db")
			inherited := map[string]any{
				"type":   "CORTEX_STEP_TYPE_INVOKE_SUBAGENT",
				"status": "CORTEX_STEP_STATUS_DONE",
				"metadata": map[string]any{
					"createdAt": "2026-11-16T00:00:00Z",
					"sourceTrajectoryStepInfo": map[string]any{
						"trajectoryId": originalID, "cascadeId": originalID,
					},
				},
			}
			local := map[string]any{
				"type":   "CORTEX_STEP_TYPE_INVOKE_SUBAGENT",
				"status": "CORTEX_STEP_STATUS_DONE",
				"metadata": map[string]any{
					"createdAt": "2026-11-16T00:00:02Z",
					"sourceTrajectoryStepInfo": map[string]any{
						"trajectoryId": forkID, "cascadeId": strings.ToUpper(forkID), "stepIndex": 2,
					},
				},
			}
			if name == "explicit result" {
				result := map[string]any{"results": []any{map[string]any{"conversationId": childID}}}
				inherited["invokeSubagent"] = result
				local["invokeSubagent"] = result
			}
			steps := []any{
				inherited,
				offlinePlannerStep("FORK_LOCAL_PLANNER", "2026-11-16T00:00:01Z"),
				local,
				offlinePlannerStep("FORK_LOCAL_END", "2026-11-16T00:00:04Z"),
			}
			if name == "inherited workflow" {
				call := map[string]any{
					"id": "copied-workflow", "name": "run_workflow",
					"argumentsJson": `{"Description":"Copied workflow","ScriptPath":"/fixture/wf.py"}`,
				}
				steps = append(steps, map[string]any{
					"type":   "CORTEX_STEP_TYPE_GENERIC",
					"status": "CORTEX_STEP_STATUS_DONE",
					"metadata": map[string]any{
						"toolCall": call,
						"sourceTrajectoryStepInfo": map[string]any{
							"trajectoryId": originalID, "cascadeId": originalID, "stepIndex": 1,
						},
					},
					"generic": map[string]any{
						"args": map[string]string{"Description": "Copied workflow", "ScriptPath": "/fixture/wf.py"},
						"result": map[string]string{
							"result":        "Created At: 2026-11-16T00:00:00Z\nCompleted At: 2026-11-16T00:00:01Z\nWorkflow \"Copied workflow\" completed.\nOutput:\nOK",
							"fullOutputUri": "file:///fixture/wf.txt",
						},
					},
				})
				writeOfflineTestDB(t, filepath.Join(dir, workflowID+".db"), map[string]any{
					"cascadeId": workflowID,
					"metadata": map[string]any{"sourceMetadata": map[string]any{"tool": map[string]any{
						"conversationId": forkID, "stepIndex": 4, "toolCall": call,
					}}},
					"steps": []any{offlinePlannerStep("COPIED_WORKFLOW_CHILD", "2026-11-16T00:00:05Z")},
				})
			}
			writeOfflineTestDB(t, forkPath, map[string]any{
				"cascadeId": forkID,
				"metadata":  map[string]any{"parentConversationId": originalID},
				"steps":     steps,
			})
			writeOfflineTestDB(t, filepath.Join(dir, childID+".db"), map[string]any{
				"cascadeId": childID,
				"metadata":  map[string]any{"parentConversationId": forkID},
				"steps":     []any{offlinePlannerStep("FORK_CHILD_BODY", "2026-11-16T00:00:03Z")},
			})

			var out, stderr bytes.Buffer
			if err := Run([]string{"export-offline", "--with-children", forkPath}, &out, &stderr); err != nil {
				t.Fatalf("export failed: %v (stderr: %s)", err, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("unexpected stderr warnings: %s", stderr.String())
			}
			md := out.String()
			marker := strings.Index(md, "FORK_LOCAL_PLANNER")
			invoke := strings.Index(md, "## Step 3: INVOKE_SUBAGENT")
			child := strings.Index(md, "FORK_CHILD_BODY")
			end := strings.Index(md, "FORK_LOCAL_END")
			if marker < 0 || marker >= invoke || invoke >= child || child >= end ||
				strings.Count(md, "FORK_CHILD_BODY") != 1 || strings.Contains(md, "## Subagent Transcripts") {
				t.Fatalf("expected the fork's child once under its local invoke:\n%s", md)
			}
			if strings.Contains(md, "COPIED_WORKFLOW_CHILD") {
				t.Fatalf("an inherited workflow established a child link:\n%s", md)
			}
		})
	}
}

func TestExportOfflineWithChildrenSortsTimestampsChronologically(t *testing.T) {
	const (
		primaryID = "12121212-1212-4212-8212-121212121212"
		childAID  = "34343434-3434-4434-8434-343434343434"
		childBID  = "56565656-5656-4656-8656-565656565656"
	)
	dir := t.TempDir()
	primaryPath := filepath.Join(dir, primaryID+".db")
	writeOfflineTestDB(t, primaryPath, map[string]any{
		"cascadeId": primaryID,
		"metadata":  map[string]any{},
		"steps": []any{
			map[string]any{
				"type":     "CORTEX_STEP_TYPE_INVOKE_SUBAGENT",
				"status":   "CORTEX_STEP_STATUS_DONE",
				"metadata": map[string]any{"createdAt": "2026-11-16T00:00:02Z"},
			},
			map[string]any{
				"type":     "CORTEX_STEP_TYPE_INVOKE_SUBAGENT",
				"status":   "CORTEX_STEP_STATUS_DONE",
				"metadata": map[string]any{"createdAt": "2026-11-16T00:00:02.400Z"},
			},
		},
	})
	agentPathMeta := []any{map[string]any{
		"cascadeConfig": map[string]any{"plannerConfig": map[string]any{"customizationConfig": map[string]any{
			"agentPath": "file:///fixture/brain/" + primaryID + "/.agents/agents/worker",
		}}},
	}}
	writeOfflineTestDB(t, filepath.Join(dir, childAID+".db"), map[string]any{
		"cascadeId":         childAID,
		"metadata":          map[string]any{},
		"executorMetadatas": agentPathMeta,
		"steps":             []any{offlinePlannerStep("CHILD_A", "2026-11-16T00:00:02Z")},
	})
	writeOfflineTestDB(t, filepath.Join(dir, childBID+".db"), map[string]any{
		"cascadeId":         childBID,
		"metadata":          map[string]any{},
		"executorMetadatas": agentPathMeta,
		"steps":             []any{offlinePlannerStep("CHILD_B", "2026-11-16T00:00:02.500Z")},
	})

	var out, stderr bytes.Buffer
	if err := Run([]string{"export-offline", "--with-children", primaryPath}, &out, &stderr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr warnings: %s", stderr.String())
	}
	md := out.String()
	idxA := strings.Index(md, "CHILD_A")
	idxB := strings.Index(md, "CHILD_B")
	if idxA < 0 || idxB < 0 || idxA > idxB {
		t.Fatalf("children attached in wrong chronological order (A=%d, B=%d):\n%s", idxA, idxB, md)
	}
}
