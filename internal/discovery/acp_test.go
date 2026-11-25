package discovery_test

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/discovery"
)

func listenLocal(t *testing.T) (port string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, port, err = net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return port, func() { ln.Close() }
}

func deadPort(t *testing.T) string {
	t.Helper()
	port, stop := listenLocal(t)
	stop()
	return port
}

func acpLog(port string) string {
	return "Language server listening on random port at 1 for HTTPS (gRPC)\nLanguage server listening on random port at " + port + " for HTTP\n"
}

// T3 Code's `antigravity acp` writes no cli.log: discovery falls back to its
// acp-server logs, newest first, and skips a server that has since exited.
func TestDiscoverDaemonURLFromACPServerLogs(t *testing.T) {
	now := time.Now()
	live, stop := listenLocal(t)
	defer stop()
	for _, tc := range []struct {
		name   string
		older  string
		newer  string
		wantOK bool
	}{
		{"newest live", deadPort(t), live, true},
		{"newest exited", live, deadPort(t), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "log", "acp-server-20260101_000000-111.log"), acpLog(tc.older), now.Add(-time.Hour))
			writeFile(t, filepath.Join(root, "log", "acp-server-20260102_000000-222.log"), acpLog(tc.newer), now)
			// Logs of another name never name this root's server.
			writeFile(t, filepath.Join(root, "log", "other-20260103.log"), acpLog(deadPort(t)), now.Add(time.Hour))
			got, err := discovery.DiscoverDaemonURL(root)
			if err != nil || got != "http://127.0.0.1:"+live {
				t.Fatalf("got %q, %v; want the live server", got, err)
			}
		})
	}
}

func TestDiscoverDaemonURLACPServerLogsAllExited(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "log", "acp-server-20260101_000000-111.log"), acpLog(deadPort(t)), time.Time{})
	_, err := discovery.DiscoverDaemonURL(root)
	if err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("got %v, want the newest log's unreachable port", err)
	}
}

// A root with a cli.log keeps its CLI behavior even when ACP logs exist.
func TestDiscoverDaemonURLPrefersCLILog(t *testing.T) {
	root := t.TempDir()
	live, stop := listenLocal(t)
	defer stop()
	writeFile(t, filepath.Join(root, "cli.log"), "Language server listening on random port at "+deadPort(t)+" for HTTP\n", time.Time{})
	writeFile(t, filepath.Join(root, "log", "acp-server-20260101_000000-111.log"), acpLog(live), time.Time{})
	if got, err := discovery.DiscoverDaemonURL(root); err == nil {
		t.Fatalf("got %q from an ACP log although cli.log is present", got)
	}
}

func TestACPRootIsCLISurface(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "jetski_state.pbtxt"), "", time.Time{})
	writeFile(t, filepath.Join(root, "log", "acp-server-20260101_000000-111.log"), "", time.Time{})
	if got := discovery.DetectSurface(root); got != discovery.SurfaceCLI {
		t.Fatalf("surface %q, want cli: ACP roots never read the IDE's main.log", got)
	}
}

// Bare invocations include each T3 Code ACP root after the CLI and IDE
// stores, in path order. Other provider directories are ignored.
func TestDefaultRootsIncludesT3ACPRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ANTIGRAVITY_CLI_ROOT", "")
	cli := filepath.Join(home, ".gemini", "antigravity-cli")
	mkdirAll(t, cli)
	providers := filepath.Join(home, ".t3", "userdata", "providers")
	b := filepath.Join(providers, "antigravity", "bbbb", "antigravity-acp")
	a := filepath.Join(providers, "antigravity", "aaaa", "antigravity-acp")
	mkdirAll(t, b)
	mkdirAll(t, a)
	mkdirAll(t, filepath.Join(providers, "antigravity", "cccc", "other"))
	mkdirAll(t, filepath.Join(providers, "codex", "dddd", "antigravity-acp"))
	writeFile(t, filepath.Join(providers, "antigravity", "eeee", "antigravity-acp"), "not a directory", time.Time{})

	roots, err := discovery.DefaultRoots()
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 3 || roots[0] != cli || roots[1] != a || roots[2] != b {
		t.Fatalf("want [%s %s %s], got %v", cli, a, b, roots)
	}

	t.Setenv("ANTIGRAVITY_CLI_ROOT", "/custom/root")
	if roots, _ := discovery.DefaultRoots(); len(roots) != 1 || roots[0] != "/custom/root" {
		t.Fatalf("env pin must stay a single root, got %v", roots)
	}
}

// With only a T3 Code ACP root on disk, it is the only default root.
func TestDefaultRootsOnlyT3ACPRoot(t *testing.T) {
	home := t.TempDir()
	acp := filepath.Join(home, ".t3", "userdata", "providers", "antigravity", "ffff", "antigravity-acp")
	mkdirAll(t, acp)
	if roots := discovery.DefaultStoreRootsUnder(home); len(roots) != 1 || roots[0] != acp {
		t.Fatalf("want [%s], got %v", acp, roots)
	}
}

// Glob metacharacters in the home directory name must be treated literally.
func TestDefaultRootsLiteralHomeBrackets(t *testing.T) {
	parent := t.TempDir()
	sibling := filepath.Join(parent, "user1")
	home := filepath.Join(parent, "user[1]")
	mkdirAll(t, filepath.Join(sibling, ".t3", "userdata", "providers", "antigravity", "sib", "antigravity-acp"))
	expected := filepath.Join(home, ".t3", "userdata", "providers", "antigravity", "own", "antigravity-acp")
	mkdirAll(t, expected)

	roots := discovery.DefaultStoreRootsUnder(home)
	if len(roots) != 1 || roots[0] != expected {
		t.Fatalf("want [%s], got %v", expected, roots)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
