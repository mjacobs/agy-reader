package discovery

import (
	"os"
	"testing"
)

// TestMain clears the ambient daemon discovery overrides so `go test ./...`
// behaves the same whether or not the developer has a daemon URL or CSRF token
// configured. Tests that exercise those variables set them with t.Setenv, which
// restores the (now cleared) value afterwards.
func TestMain(m *testing.M) {
	for _, k := range []string{"ANTIGRAVITY_DAEMON_URL", "ANTIGRAVITY_CSRF_TOKEN"} {
		os.Unsetenv(k)
	}
	os.Exit(m.Run())
}
