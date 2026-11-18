package discovery

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDarwinCLIHTTPListeners(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "cli.log")
	if err := os.WriteFile(logPath, []byte("listening on random port at 45001 for HTTP\nlistening on random port at 45002 for HTTPS (gRPC)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fields := strings.Join([]string{
		"p101", "\nf2", "tREG", "n" + logPath,
		"\nf3", "tIPv4", "n127.0.0.1:45002", "TST=LISTEN",
		"\nf4", "tIPv4", "n127.0.0.1:45001", "TST=ESTABLISHED",
		"\nf5", "tIPv4", "n0.0.0.0:45001", "TST=LISTEN",
		"\nf6", "tIPv6", "n[::1]:45001", "TST=LISTEN", "\n",
	}, "\x00")
	if got := darwinCLIHTTPListeners(root, []byte(fields)); !slices.Equal(got, []string{"[::1]:45001"}) {
		t.Fatalf("HTTP listeners = %v, want only the logged loopback listener", got)
	}
	if got := darwinCLIHTTPListeners(t.TempDir(), []byte(fields)); len(got) > 0 {
		t.Fatalf("returned %d listeners for a different root", len(got))
	}
}

func TestDarwinCLIEnvironmentToken(t *testing.T) {
	const address = "127.0.0.1:45001"
	const toolAddress = "ANTIGRAVITY_LS_ADDRESS=localhost:45001\x00"
	const toolToken = "ANTIGRAVITY_CSRF_TOKEN=fixture-token\x00"
	const toolVersion = "ANTIGRAVITY_LS_VERSION=cli-fixture\x00"
	args := binary.NativeEndian.AppendUint32(nil, 3)
	args = append(args, "/bin/sh\x00\x00\x00sh\x00"+toolToken+"\x00"...)
	for _, tc := range []struct {
		name, env, want string
	}{
		{"CLI tool", toolAddress + toolVersion + toolToken, "fixture-token"},
		{"argv token only", toolAddress + toolVersion, ""},
		{"wrong port", "ANTIGRAVITY_LS_ADDRESS=localhost:45002\x00" + toolVersion + toolToken, ""},
		{"wrong address family", "ANTIGRAVITY_LS_ADDRESS=[::1]:45001\x00" + toolVersion + toolToken, ""},
		{"non-CLI tool", toolAddress + toolToken, ""},
		{"duplicate token", toolAddress + toolVersion + toolToken + toolToken, ""},
		{"truncated token", toolAddress + toolVersion + strings.TrimSuffix(toolToken, "\x00"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := darwinCLIEnvironmentToken(append(slices.Clone(args), tc.env...), address); got != tc.want {
				t.Fatal("unexpected credential discovery result")
			}
		})
	}
}
