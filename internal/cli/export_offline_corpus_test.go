package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/render"
)

// Opt-in comparison against independently captured daemon exports. No discovery
// or RPC is performed. The supplied manifest binds each response to its source.
func TestExportOfflineCorpus(t *testing.T) {
	manifestPath, root := os.Getenv("AGY_READER_OFFLINE_MANIFEST"), os.Getenv("AGY_READER_OFFLINE_SOURCES")
	if manifestPath == "" || root == "" {
		t.Skip("explicit offline corpus not supplied")
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Complete            bool   `json:"complete"`
		InstalledAgyVersion string `json:"installedAgyVersion"`
		Sessions            []struct {
			FileID       string `json:"fileId"`
			Source       string `json:"source"`
			SourceDigest string `json:"sourceDigest"`
			Profile      string `json:"profile"`
			Steps        int    `json:"steps"`
		} `json:"sessions"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if !manifest.Complete || manifest.InstalledAgyVersion != "1.4.4" {
		t.Fatal("expected complete agy 1.4.4 manifest")
	}
	checked, steps, emptyArrays := 0, 0, 0
	for i, s := range manifest.Sessions {
		if s.Source != "sqlite" {
			continue
		}
		if filepath.Base(s.FileID) != s.FileID || s.FileID == "." || s.FileID == "" {
			t.Fatal("invalid manifest file ID")
		}
		t.Run(fmt.Sprintf("session-%03d", i), func(t *testing.T) {
			path := filepath.Join(root, s.FileID+".db")
			var out bytes.Buffer
			if err := Run([]string{"export-offline", "--format=json", path}, &out, io.Discard); err != nil {
				t.Fatal(err)
			}
			var offline struct {
				AgyReader struct {
					Offline struct {
						SourceDigest string
						Profile      string
					}
				}
				Steps []json.RawMessage
			}
			if err := json.Unmarshal(out.Bytes(), &offline); err != nil {
				t.Fatal(err)
			}
			if offline.AgyReader.Offline.SourceDigest != s.SourceDigest || offline.AgyReader.Offline.Profile != s.Profile {
				t.Fatal("source differs from independently captured response")
			}
			if len(offline.Steps) != s.Steps {
				t.Fatal("step count differs")
			}
			expected, err := os.ReadFile(filepath.Join(filepath.Dir(manifestPath), "corpus", s.FileID+".trajectory.json"))
			if err != nil {
				t.Fatal(err)
			}
			var got, want map[string]any
			gotDecoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
			gotDecoder.UseNumber()
			if err := gotDecoder.Decode(&got); err != nil {
				t.Fatal(err)
			}
			wantDecoder := json.NewDecoder(bytes.NewReader(expected))
			wantDecoder.UseNumber()
			if err := wantDecoder.Decode(&want); err != nil {
				t.Fatal(err)
			}
			delete(got, "agyReader")
			delete(want, "agyReader")
			// agy omits steps for empty sessions; offline exports make it explicit.
			if _, exists := want["steps"]; !exists && s.Steps == 0 {
				want["steps"] = []any{}
				emptyArrays++
			}
			if !reflect.DeepEqual(got, want) {
				t.Error("JSON representation differs from daemon export")
			}
			var trajectory daemon.Trajectory
			if err = json.Unmarshal(expected, &trajectory); err != nil {
				t.Fatal(err)
			}
			var md bytes.Buffer
			if _, err = render.Markdown(&md, &trajectory, time.Unix(0, 0)); err != nil {
				t.Fatal(err)
			}
			out.Reset()
			if err = Run([]string{"export-offline", path}, &out, io.Discard); err != nil {
				t.Fatal(err)
			}
			generated := regexp.MustCompile(`(?m)^- \*\*Generated on:\*\* .*\n`)
			actual := bytes.TrimPrefix(out.Bytes(), []byte(offlineMarkdownNotice))
			if !bytes.Equal(generated.ReplaceAll(actual, nil), generated.ReplaceAll(md.Bytes(), nil)) {
				t.Error("Markdown differs from daemon export")
			}
			checked++
			steps += s.Steps
		})
	}
	if checked == 0 {
		t.Fatal("no SQLite sessions checked")
	}
	t.Logf("compared %d SQLite sessions, %d steps; %d empty step arrays normalized", checked, steps, emptyArrays)
}
