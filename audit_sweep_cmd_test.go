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

func TestAuditSweepRejectsEmptyOrDuplicateSelection(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		root := t.TempDir()
		dest := filepath.Join(t.TempDir(), "out")
		if duplicate {
			seedPB(t, root, "conversations", sweepTestID, time.Now())
			writeFileT(t, filepath.Join(root, "conversations", sweepTestID+".db"), []byte("placeholder"))
		}
		if err := runAuditSweepTo(t.Context(), []string{"--root", root, "--out", dest}, &bytes.Buffer{}); err == nil {
			t.Fatal("accepted empty/duplicate selection")
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Fatal("created output for invalid selection")
		}
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
