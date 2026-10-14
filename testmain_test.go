package main

import (
	"os"
	"path/filepath"
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
	// Watch caches are disposable test output, never write the user's cache.
	dir, err := os.MkdirTemp("", "agy-reader-test-cache-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CACHE_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func writeFileT(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
