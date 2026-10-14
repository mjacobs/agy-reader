package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/cli"
)

func TestVerifyContractCLI(t *testing.T) {
	t.Run("schema flag outputs schema", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := cli.Run([]string{"verify-contract", "--schema"}, &stdout, &stderr)
		if err != nil {
			t.Fatalf("unexpected error: %v, stderr: %s", err, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "https://json-schema.org/draft/2020-12/schema") {
			t.Fatalf("expected Draft 2020-12 schema in output, got: %s", out)
		}
		if !strings.Contains(out, "agyReader") {
			t.Fatalf("expected agyReader in schema output, got: %s", out)
		}
	})

	t.Run("no args reports usage error", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := cli.Run([]string{"verify-contract"}, &stdout, &stderr)
		if err == nil {
			t.Fatalf("expected usage error, but succeeded")
		}
		if !strings.Contains(err.Error(), "usage: agy-reader verify-contract") {
			t.Fatalf("expected usage message, got: %v", err)
		}
	})

	t.Run("valid fixtures directory passes", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		testdataDir := filepath.Join("..", "render", "testdata")
		err := cli.Run([]string{"verify-contract", testdataDir}, &stdout, &stderr)
		if err != nil {
			t.Fatalf("expected success, got error: %v, stderr: %s", err, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "contract valid:") {
			t.Fatalf("expected contract valid confirmation, got: %s", out)
		}
	})

	t.Run("invalid sidecar reports failure", func(t *testing.T) {
		dir := t.TempDir()
		badSidecar := filepath.Join(dir, "invalid.trajectory.json")
		if err := os.WriteFile(badSidecar, []byte(`{"bad": "data"}`), 0o644); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		err := cli.Run([]string{"verify-contract", badSidecar}, &stdout, &stderr)
		if err == nil {
			t.Fatalf("expected validation failure, got success")
		}
		errOut := stderr.String()
		if !strings.Contains(errOut, "error:") {
			t.Fatalf("expected error details in stderr, got: %s", errOut)
		}
	})
}
