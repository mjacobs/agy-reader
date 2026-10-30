package completeness_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/render"
	"github.com/mjacobs/agy-reader/internal/testfixture"
)

// Exercise offline export to Markdown for captured parent messages.
func TestParentMsgs1410OfflineMarkdownExport(t *testing.T) {
	const raw = `{"cascadeId":"parentmsgs1410-export-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_CHECKPOINT","checkpoint":{"capturedParentMessages":[{"text":"Synthetic parent instruction alpha.","media":[{"mimeType":"image/png","inlineData":"AQID","description":"first attachment","uri":"file:///fixture/synthetic/image.png"}]},{"text":"Synthetic parent instruction beta.","media":[{"mimeType":"text/plain","displayName":"context.txt","uri":"file:///fixture/synthetic/context.txt"}]},{"text":"Synthetic parent instruction gamma with no media."}]}}]}`
	p := filepath.Join(t.TempDir(), "source.db")
	testfixture.Source(t, p, []byte(raw))
	s, err := completeness.ReadSnapshot(t.Context(), p)
	if err != nil {
		t.Fatalf("ReadSnapshot failed: %v", err)
	}

	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatalf("OfflineJSON failed: %v", err)
	}

	var traj daemon.Trajectory
	if err := json.Unmarshal(offline, &traj); err != nil {
		t.Fatalf("unmarshal offline JSON: %v", err)
	}

	var buf bytes.Buffer
	if _, err := render.Markdown(&buf, &traj, time.Time{}); err != nil {
		t.Fatalf("render Markdown: %v", err)
	}
	md := buf.String()
	for _, want := range []string{
		"**Captured Parent Messages:**",
		"#### Captured Parent Message 1",
		"Synthetic parent instruction alpha.",
		"#### Captured Parent Message 2",
		"Synthetic parent instruction beta.",
		"#### Captured Parent Message 3",
		"Synthetic parent instruction gamma with no media.",
		"**Media Type:** `image/png`",
		"file:///fixture/synthetic/image.png",
		"**Display Name:** `context.txt`",
		"file:///fixture/synthetic/context.txt",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in markdown export:\n%s", want, md)
		}
	}
}
