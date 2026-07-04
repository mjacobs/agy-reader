package discovery_test

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/discovery"
)

func TestDetectSurface(t *testing.T) {
	cases := []struct {
		name  string
		files []string // files created inside the root
		want  discovery.Surface
	}{
		{"cli root with cli.log", []string{"cli.log"}, discovery.SurfaceCLI},
		{"ide root with state file", []string{"antigravity_state.pbtxt"}, discovery.SurfaceIDE},
		{"cli.log wins over ide marker", []string{"cli.log", "antigravity_state.pbtxt"}, discovery.SurfaceCLI},
		{"bare root defaults to cli", nil, discovery.SurfaceCLI},
		{"cli root markers", []string{"jetski_state.pbtxt", "history.jsonl"}, discovery.SurfaceCLI},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, f := range tc.files {
				if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
					t.Fatalf("write %s: %v", f, err)
				}
			}
			if got := discovery.DetectSurface(root); got != tc.want {
				t.Errorf("DetectSurface = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDetectSurfaceDefaultIDERoot(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("UserHomeDir not available:", err)
	}
	root := filepath.Join(home, discovery.DefaultIDERootSubpath)
	if _, err := os.Stat(filepath.Join(root, "cli.log")); err == nil {
		t.Skipf("unexpected cli.log in %s; default IDE root not usable for this test", root)
	}
	if got := discovery.DetectSurface(root); got != discovery.SurfaceIDE {
		t.Errorf("DetectSurface(%s) = %q, want ide", root, got)
	}
}

// ideLogsDirUnder points IDELogsDir at a temp config tree via
// XDG_CONFIG_HOME and returns the Antigravity logs dir inside it, skipping
// the test on platforms where os.UserConfigDir does not honor XDG.
func ideLogsDirUnder(t *testing.T) string {
	t.Helper()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	logsDir, err := discovery.IDELogsDir()
	if err != nil {
		t.Fatalf("IDELogsDir: %v", err)
	}
	if !strings.HasPrefix(logsDir, cfg) {
		t.Skipf("os.UserConfigDir ignores XDG_CONFIG_HOME on this platform (got %s)", logsDir)
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatalf("mkdir logs dir: %v", err)
	}
	return logsDir
}

// ideRoot returns a temp root carrying the IDE marker file.
func ideRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "antigravity_state.pbtxt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write ide marker: %v", err)
	}
	return root
}

func TestDiscoverDaemonURLIDERoot(t *testing.T) {
	logsDir := ideLogsDirUnder(t)
	root := ideRoot(t)

	live, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer live.Close()
	_, port, err := net.SplitHostPort(live.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}

	// Real IDE language_server.log shape: gRPC line first, HTTP line second.
	logContent := "I0702 22:12:11.358699 32171 server.go:514] Language server listening on random port at 40953 for HTTPS (gRPC)\n" +
		"I0702 22:12:11.358803 32171 server.go:522] Language server listening on random port at " + port + " for HTTP\n"
	if err := os.WriteFile(filepath.Join(logsDir, "language_server.log"), []byte(logContent), 0o644); err != nil {
		t.Fatalf("write language_server.log: %v", err)
	}

	got, err := discovery.DiscoverDaemonURL(root)
	if err != nil {
		t.Fatalf("DiscoverDaemonURL: %v", err)
	}
	want := "http://127.0.0.1:" + port
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestDiscoverDaemonURLIDERootMissingLog(t *testing.T) {
	ideLogsDirUnder(t) // exists but holds no language_server.log
	root := ideRoot(t)

	_, err := discovery.DiscoverDaemonURL(root)
	if err == nil {
		t.Fatal("expected error for missing language_server.log")
	}
	if !strings.Contains(err.Error(), "language_server.log") {
		t.Errorf("error should name language_server.log, got %v", err)
	}
}

// A CLI root must keep using cli.log even when IDE logs exist on the machine.
func TestDiscoverDaemonURLCLIRootIgnoresIDELogs(t *testing.T) {
	logsDir := ideLogsDirUnder(t)

	// IDE log advertises a dead port; if the CLI path consulted it, discovery
	// would fail with "unreachable" rather than the CLI's own port.
	dead, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, deadPort, _ := net.SplitHostPort(dead.Addr().String())
	dead.Close()
	ideLog := "Language server listening on random port at " + deadPort + " for HTTP\n"
	if err := os.WriteFile(filepath.Join(logsDir, "language_server.log"), []byte(ideLog), 0o644); err != nil {
		t.Fatal(err)
	}

	live, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer live.Close()
	_, livePort, _ := net.SplitHostPort(live.Addr().String())

	root := t.TempDir()
	cliLog := "Language server listening on random port at " + livePort + " for HTTP\n"
	if err := os.WriteFile(filepath.Join(root, "cli.log"), []byte(cliLog), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := discovery.DiscoverDaemonURL(root)
	if err != nil {
		t.Fatalf("DiscoverDaemonURL: %v", err)
	}
	want := "http://127.0.0.1:" + livePort
	if got != want {
		t.Errorf("got %q want %q (CLI root must read its own cli.log)", got, want)
	}
}
