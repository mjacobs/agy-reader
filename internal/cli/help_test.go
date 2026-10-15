package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpSucceeds(t *testing.T) {
	for _, arg := range []string{"-h", "--help"} {
		t.Run(arg, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := Run([]string{arg}, &stdout, &stderr); err != nil {
				t.Fatalf("help failed: %v", err)
			}
			if !strings.Contains(stderr.String(), "Usage") {
				t.Fatalf("missing usage: %s", &stderr)
			}
		})
	}
}
