package render_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/render"
)

// update regenerates the *.golden.md files instead of comparing against them:
//
//	go test ./internal/render -run TestMarkdownGolden -update
var update = flag.Bool("update", false, "update golden files")

// TestMarkdownGolden renders each testdata/*.trajectory.json fixture and pins
// the exact Markdown output in a sibling *.golden.md file. The fixtures are
// trimmed, sanitized captures of real trajectories.
func TestMarkdownGolden(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join("testdata", "*.trajectory.json"))
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no *.trajectory.json fixtures found in testdata")
	}

	// Pinned time for the deterministic "Generated on" header.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			data, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var traj daemon.Trajectory
			if err := json.Unmarshal(data, &traj); err != nil {
				t.Fatalf("unmarshal fixture: %v", err)
			}

			var buf bytes.Buffer
			if _, err := render.Markdown(&buf, &traj, now); err != nil {
				t.Fatalf("Markdown: %v", err)
			}
			got := buf.Bytes()

			golden := fixture[:len(fixture)-len(".trajectory.json")] + ".golden.md"
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("rendered output does not match %s\n--- got ---\n%s\n--- want ---\n%s", golden, got, want)
			}
		})
	}
}
