package main

import (
	"os"
	"testing"
)

// TestMain clears the ambient daemon discovery overrides so `go test ./...`
// behaves the same whether or not the developer has a daemon URL or CSRF token
// configured. Tests that exercise those variables set them with t.Setenv, which
// restores the (now cleared) value afterwards.
//
// With AGY_READER_TEST_AS_CLI=1 the test binary runs the real command instead,
// so shell-helper tests exercise actual subcommands without a separate build.
func TestMain(m *testing.M) {
	if os.Getenv("AGY_READER_TEST_AS_CLI") == "1" {
		main()
		os.Exit(0)
	}
	for _, k := range []string{"ANTIGRAVITY_DAEMON_URL", "ANTIGRAVITY_CSRF_TOKEN"} {
		os.Unsetenv(k)
	}
	os.Exit(m.Run())
}
