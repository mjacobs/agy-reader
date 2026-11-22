//go:build !goexperiment.jsonv2

package cache_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/daemon"
)

func TestWriteVerifiedPreservesReaderMetadataWithDeepUnrelatedField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.trajectory.json")
	deep := strings.Repeat("[", 10000) + "0" + strings.Repeat("]", 10000)
	existing := `{"cascadeId":"11111111-1111-4111-8111-111111111111","steps":[],"agyReader":{"parentCascadeId":"22222222-2222-4222-8222-222222222222","label":"keep"},"future":` + deep + `}`
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	fresh := []byte(`{"cascadeId":"11111111-1111-4111-8111-111111111111","steps":[]}`)
	if err := cache.Write(path, &daemon.Trajectory{RawJSON: fresh}); err != nil {
		t.Fatalf("cache.Write: %v", err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(saved, []byte(`"future"`)) {
		t.Fatalf("unrelated deep field survived refresh:\n%s", saved)
	}
	for _, want := range []string{`22222222-2222-4222-8222-222222222222`, `"label"`, `"keep"`} {
		if !bytes.Contains(saved, []byte(want)) {
			t.Fatalf("lost preserved reader metadata %s:\n%s", want, saved)
		}
	}
}
