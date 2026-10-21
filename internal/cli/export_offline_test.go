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

const offlineFixture = `{"cascadeId":"offline-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","status":"CORTEX_STEP_STATUS_DONE","metadata":{},"userInput":{"userResponse":"Offline hello"}}]}`

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
