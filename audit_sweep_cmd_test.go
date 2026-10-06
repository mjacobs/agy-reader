package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/testfixture"
)

const sweepTestID = "11111111-1111-1111-1111-111111111111"

func sweepFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	seedPB(t, root, "conversations", sweepTestID, time.Now().Add(-time.Hour))
	// Implicit sessions must never enter a corpus of syncable conversations.
	seedPB(t, root, "implicit", "22222222-2222-2222-2222-222222222222", time.Now())
	sidecar := filepath.Join(root, "conversations", sweepTestID+".trajectory.json")
	writeFileT(t, sidecar, []byte(`{"cascadeId":"`+sweepTestID+`","steps":[],"old":true}`))
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "fixture-secret")
	return root, filepath.Join(t.TempDir(), "sweep"), sidecar
}

func readSweepManifest(t *testing.T, dest string) sweepManifest {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dest, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m sweepManifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("fixture-secret")) {
		t.Fatal("token leaked into manifest")
	}
	return m
}

func TestAuditSweepFreshRawCorpus(t *testing.T) {
	root, dest, sidecar := sweepFixture(t)
	before, _ := os.ReadFile(sidecar)
	stat, _ := os.Stat(sidecar)
	raw := `{"cascadeId":"` + sweepTestID + `","future":{"integer":9007199254740993123456789},"steps":[{"type":"FUTURE_STEP","future":{"kept":true}}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-codeium-csrf-token") != "fixture-secret" {
			t.Error("missing token")
		}
		if strings.HasSuffix(r.URL.Path, "/LoadTrajectory") {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(`{"trajectory":` + raw + `}`))
	}))
	defer srv.Close()
	t.Setenv("ANTIGRAVITY_DAEMON_URL", srv.URL)
	var out bytes.Buffer
	if err := runAuditSweepTo(t.Context(), []string{"--root", root, "--out", dest}, &out); err != nil {
		t.Fatal(err)
	}
	m := readSweepManifest(t, dest)
	if !m.Complete || m.Selected != 1 || len(m.Sessions) != 1 || m.Sessions[0].Steps != 1 {
		t.Fatalf("bad manifest: %+v", m)
	}
	b, err := os.ReadFile(filepath.Join(dest, "corpus", sweepTestID+".trajectory.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != raw {
		t.Fatalf("payload changed: %s", b)
	}
	after, _ := os.ReadFile(sidecar)
	now, _ := os.Stat(sidecar)
	if !bytes.Equal(before, after) || !stat.ModTime().Equal(now.ModTime()) {
		t.Fatal("live sidecar changed")
	}
	for _, path := range []string{dest, filepath.Join(dest, "corpus"), filepath.Join(dest, "manifest.json"), filepath.Join(dest, "corpus", sweepTestID+".trajectory.json")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("non-private output %s", path)
		}
	}
	if err := runAuditSweepTo(t.Context(), []string{"--root", root, "--out", dest}, &out); err == nil {
		t.Fatal("reused output directory")
	}
}

func TestAuditSweepNeverUsesCacheOnFailure(t *testing.T) {
	for _, mode := range []string{"auth", "server", "empty", "malformed", "cancel", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			root, dest, _ := sweepFixture(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "timeout":
					<-time.After(300 * time.Millisecond)
				case "auth":
					http.Error(w, "fixture-secret", http.StatusUnauthorized)
				case "server":
					http.Error(w, "unavailable", http.StatusServiceUnavailable)
				default:
					if strings.HasSuffix(r.URL.Path, "/LoadTrajectory") {
						_, _ = w.Write([]byte(`{}`))
						return
					}
					if mode == "malformed" {
						_, _ = w.Write([]byte(`{"trajectory":`))
						return
					}
					_, _ = w.Write([]byte(`{}`))
				}
			}))
			defer srv.Close()
			t.Setenv("ANTIGRAVITY_DAEMON_URL", srv.URL)
			ctx := t.Context()
			if mode == "cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			var out bytes.Buffer
			err := runAuditSweepTo(ctx, []string{"--root", root, "--out", dest, "--timeout", "100ms"}, &out)
			if err == nil {
				t.Fatal("accepted failed fetch despite cached sidecar")
			}
			if mode == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected deadline, got %v", err)
			}
			if strings.Contains(err.Error(), "fixture-secret") {
				t.Fatal("token leaked")
			}
			m := readSweepManifest(t, dest)
			if m.Complete || len(m.Sessions) != 0 || m.FailedID != sweepTestID {
				t.Fatalf("bad failure manifest: %+v", m)
			}
			files, _ := os.ReadDir(filepath.Join(dest, "corpus"))
			if len(files) != 0 {
				t.Fatal("cached or invalid payload in corpus")
			}
		})
	}
}

func TestAuditSweepRejectsEmptySelection(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(t.TempDir(), "out")
	if err := runAuditSweepTo(t.Context(), []string{"--root", root, "--out", dest}, &bytes.Buffer{}); err == nil {
		t.Fatal("accepted empty selection")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("created output for invalid selection")
	}
}

// A forked conversation leaves <id>.pb from the fork and <id>.db once it
// continues. The sweep must treat that id as one conversation sourced from the
// .db and fetch it once, not refuse the whole store as a duplicate
// (agy-reader#3g5w).
func TestAuditSweepFetchesForkedConversationOnceFromItsDB(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "conversations", sweepTestID+".db")
	db := testfixture.Source(t, path, qualifyPayload(sweepTestID))
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	seedPB(t, root, "conversations", sweepTestID, time.Now().Add(-time.Hour))
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "fixture-secret")
	sweepIdentityDaemon(t, sweepTestID)
	dest := filepath.Join(t.TempDir(), "sweep")
	if err := runAuditSweepTo(t.Context(), []string{"--root", root, "--out", dest}, &bytes.Buffer{}); err != nil {
		t.Fatalf("sweep refused a store with a forked conversation: %v", err)
	}
	m := readSweepManifest(t, dest)
	if !m.Complete || m.Selected != 1 || len(m.Sessions) != 1 {
		t.Fatalf("fork not swept exactly once: %+v", m)
	}
	if got := m.Sessions[0]; got.FileID != sweepTestID || got.Source != sourceSQLite {
		t.Fatalf("fork swept from the wrong source: %+v", got)
	}
}

func TestAuditSweepRetainsIncompleteManifestAfterPartialSuccess(t *testing.T) {
	root, dest, _ := sweepFixture(t)
	second := "22222222-2222-2222-2222-222222222222"
	seedPB(t, root, "conversations", second, time.Now().Add(-2*time.Hour))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CascadeID string `json:"cascadeId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.CascadeID == second {
			http.Error(w, "unavailable", 500)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/LoadTrajectory") {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(`{"trajectory":{"cascadeId":"` + sweepTestID + `","steps":[]}}`))
	}))
	defer srv.Close()
	t.Setenv("ANTIGRAVITY_DAEMON_URL", srv.URL)
	if err := runAuditSweepTo(t.Context(), []string{"--root", root, "--out", dest}, &bytes.Buffer{}); err == nil {
		t.Fatal("partial corpus accepted")
	}
	m := readSweepManifest(t, dest)
	if m.Complete || m.Selected != 2 || len(m.Sessions) != 1 || m.FailedID != second {
		t.Fatalf("incorrect partial manifest: %+v", m)
	}
}

// sweepIdentityDaemon answers every trajectory request with the conversation
// named by answer, whatever the request asked for.
func sweepIdentityDaemon(t *testing.T, answer string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/LoadTrajectory") {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(`{"trajectory":{"cascadeId":"` + answer + `","steps":[]}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ANTIGRAVITY_DAEMON_URL", srv.URL)
}

// assertSweepRefusedIdentity checks the failure path every fetch error takes:
// an incomplete manifest naming the failed file, and no corpus entry for it.
func assertSweepRefusedIdentity(t *testing.T, err error, dest, failedID, want string, fetched int) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("sweep accepted a trajectory for another conversation: %v", err)
	}
	m := readSweepManifest(t, dest)
	if m.Complete || len(m.Sessions) != fetched || m.FailedID != failedID {
		t.Fatalf("bad failure manifest: %+v", m)
	}
	if _, err := os.Stat(filepath.Join(dest, "corpus", failedID+".trajectory.json")); !os.IsNotExist(err) {
		t.Fatalf("corpus kept the other conversation's payload: %v", err)
	}
}

func TestAuditSweepRejectsAnotherConversationsTrajectory(t *testing.T) {
	root, dest, _ := sweepFixture(t)
	sweepIdentityDaemon(t, "99999999-9999-4999-8999-999999999999")
	err := runAuditSweepTo(t.Context(), []string{"--root", root, "--out", dest}, &bytes.Buffer{})
	assertSweepRefusedIdentity(t, err, dest, sweepTestID, "not the selected conversation", 0)
}

func TestAuditSweepRejectsRepeatedReturnedIdentity(t *testing.T) {
	root, dest, _ := sweepFixture(t)
	// An imported database whose persisted cascade_id is the first session's
	// ID. The daemon answering it with that ID matches the file's known
	// internal identity, but that conversation was already fetched.
	other := "55555555-5555-4555-8555-555555555555"
	path := filepath.Join(root, "conversations", other+".db")
	db := testfixture.Source(t, path, qualifyPayload(sweepTestID))
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	sweepIdentityDaemon(t, sweepTestID)
	err := runAuditSweepTo(t.Context(), []string{"--root", root, "--out", dest}, &bytes.Buffer{})
	assertSweepRefusedIdentity(t, err, dest, other, "already fetched for "+sweepTestID, 1)
}

// A rebuilt or imported database can hold its tables in another order, so
// trajectory_meta is not on the page discovery's quick identity read inspects
// and the session carries no internal ID. The daemon still answers with the
// stored cascade_id; the sweep must recognise it through the native source
// reader instead of refusing a valid conversation.
func TestAuditSweepAcceptsStoredIdentityDiscoveryCannotRead(t *testing.T) {
	root := t.TempDir()
	stored := "22222222-2222-4222-8222-222222222222"
	path := filepath.Join(root, "conversations", sweepTestID+".db")
	lines := strings.Split(testfixture.Schema, "\n")
	lines[1], lines[2] = lines[2], lines[1] // steps before trajectory_meta
	db := testfixture.SourceWithSchema(t, path, qualifyPayload(stored), strings.Join(lines, "\n"))
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if id, err := discovery.ReadSQLiteCascadeID(path); err != nil || id != "" {
		t.Fatalf("fixture does not exercise the fallback: discovery read internal ID %q, %v", id, err)
	}
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "fixture-secret")
	sweepIdentityDaemon(t, stored)
	dest := filepath.Join(t.TempDir(), "sweep")
	if err := runAuditSweepTo(t.Context(), []string{"--root", root, "--out", dest}, &bytes.Buffer{}); err != nil {
		t.Fatalf("sweep refused the conversation's stored identity: %v", err)
	}
	m := readSweepManifest(t, dest)
	if !m.Complete || len(m.Sessions) != 1 || m.Sessions[0].FileID != sweepTestID || m.Sessions[0].CascadeID != stored {
		t.Fatalf("bad manifest: %+v", m)
	}
}
