package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// acpProcFixture models T3 Code's `antigravity acp` server: no cli.log, an
// open log/acp-server-<time>-<pid>.log naming the HTTP port, and a second
// (gRPC) listener that must not be chosen. All IDs, PIDs and ports are
// synthetic.
func acpProcFixture(t *testing.T) (proc, root string) {
	t.Helper()
	proc, root = t.TempDir(), filepath.Join(t.TempDir(), "providers/antigravity/fixturehash/antigravity-acp")
	procLink(t, filepath.Join(proc, "self/ns/net"), "net:[42]")
	procLink(t, filepath.Join(proc, "300/ns/net"), "net:[42]")
	procFile(t, filepath.Join(proc, "300/cmdline"), "/opt/fixture/antigravity\x00acp\x00--gemini-dir\x00/fixture/gemini\x00--app-data-dir\x00antigravity-acp\x00")
	// A previous server's log remains after a restart.
	procFile(t, filepath.Join(root, "log/acp-server-20260101_000000-111.log"), "Language server listening on random port at 41000 for HTTP\n")
	logPath := filepath.Join(root, "log/acp-server-20260102_000000-300.log")
	procFile(t, logPath, "Language server listening on random port at 41110 for HTTPS (gRPC)\nLanguage server listening on random port at 41111 for HTTP\n")
	procLink(t, filepath.Join(proc, "300/fd/2"), logPath)
	procLink(t, filepath.Join(proc, "300/fd/12"), "socket:[500]")
	procLink(t, filepath.Join(proc, "300/fd/14"), "socket:[501]")
	procFile(t, filepath.Join(proc, "300/net/tcp"),
		fmt.Sprintf("0: 0100007F:%04X 00000000:0000 0A 0 0 0 1000 0 500\n1: 0100007F:%04X 00000000:0000 0A 0 0 0 1000 0 501\n", 41110, 41111))
	return proc, root
}

func TestACPEndpointUsesOpenServerLogWithoutToken(t *testing.T) {
	proc, root := acpProcFixture(t)
	for _, pinned := range []string{"", "http://127.0.0.1:41111"} {
		got := cliEndpointsFromProc(proc, root, pinned)
		if len(got) != 1 || got[0].BaseURL != "http://127.0.0.1:41111" || got[0].Token != "" {
			t.Fatalf("pin %q: got %+v, want the logged HTTP listener without a token", pinned, got)
		}
	}
	if got := cliEndpointsFromProc(proc, root, "http://127.0.0.1:41110"); len(got) != 0 {
		t.Fatalf("selected the gRPC listener: %+v", got)
	}
}

func TestACPEndpointUsesUnambiguousToolToken(t *testing.T) {
	proc, root := acpProcFixture(t)
	procLink(t, filepath.Join(proc, "301/ns/net"), "net:[42]")
	procFile(t, filepath.Join(proc, "301/environ"), "ANTIGRAVITY_LS_ADDRESS=localhost:41111\x00ANTIGRAVITY_CSRF_TOKEN=fixture-token\x00ANTIGRAVITY_LS_VERSION=cli-1.5.16\x00")
	if base, token := cliEndpointFromProc(proc, root, ""); base != "http://127.0.0.1:41111" || token != "fixture-token" {
		t.Fatalf("got %q with token %v, want the tool token for the HTTP listener", base, token != "")
	}
	procLink(t, filepath.Join(proc, "302/ns/net"), "net:[42]")
	procFile(t, filepath.Join(proc, "302/environ"), "ANTIGRAVITY_LS_ADDRESS=localhost:41111\x00ANTIGRAVITY_CSRF_TOKEN=other-token\x00ANTIGRAVITY_LS_VERSION=cli-1.5.16\x00")
	if base, token := cliEndpointFromProc(proc, root, ""); base != "http://127.0.0.1:41111" || token != "" {
		t.Fatalf("conflicting tool tokens: got %q with token %v, want tokenless", base, token != "")
	}
}

func TestACPEndpointRejectsOtherProcessesAndLogs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(t *testing.T, proc, root string)
	}{
		{"not acp subcommand", func(t *testing.T, p, r string) {
			procFile(t, filepath.Join(p, "300/cmdline"), "/opt/fixture/antigravity\x00--gemini-dir\x00acp\x00")
		}},
		{"other executable", func(t *testing.T, p, r string) {
			procFile(t, filepath.Join(p, "300/cmdline"), "/opt/fixture/other\x00acp\x00")
		}},
		{"other root log", func(t *testing.T, p, r string) {
			os.Remove(filepath.Join(p, "300/fd/2"))
			procLink(t, filepath.Join(p, "300/fd/2"), filepath.Join(t.TempDir(), "log/acp-server-20260102_000000-300.log"))
		}},
		{"cli log name", func(t *testing.T, p, r string) {
			logPath := filepath.Join(r, "log/cli-20260102_000000.log")
			procFile(t, logPath, "Language server listening on random port at 41111 for HTTP\n")
			os.Remove(filepath.Join(p, "300/fd/2"))
			procLink(t, filepath.Join(p, "300/fd/2"), logPath)
		}},
		{"nested log", func(t *testing.T, p, r string) {
			logPath := filepath.Join(r, "log/old/acp-server-20260102_000000-300.log")
			procFile(t, logPath, "Language server listening on random port at 41111 for HTTP\n")
			os.Remove(filepath.Join(p, "300/fd/2"))
			procLink(t, filepath.Join(p, "300/fd/2"), logPath)
		}},
		{"other namespace", func(t *testing.T, p, r string) {
			os.Remove(filepath.Join(p, "300/ns/net"))
			procLink(t, filepath.Join(p, "300/ns/net"), "net:[99]")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, r := acpProcFixture(t)
			tc.change(t, p, r)
			if got := cliEndpointsFromProc(p, r, ""); len(got) != 0 {
				t.Fatalf("accepted an endpoint not owned by this root's ACP server: %+v", got)
			}
		})
	}
}

// agy itself is never matched through an ACP server log, so the CLI and ACP
// ownership rules stay separate.
func TestAgyProcessIgnoresACPServerLog(t *testing.T) {
	proc, root := acpProcFixture(t)
	procFile(t, filepath.Join(proc, "300/cmdline"), "/usr/bin/agy\x00")
	if got := cliEndpointsFromProc(proc, root, ""); len(got) != 0 {
		t.Fatalf("agy matched through an ACP server log: %+v", got)
	}
}

// A surviving tool process whose advertised address hits the ACP server's gRPC
// listener must not promote that gRPC port over the logged HTTP port.
func TestACPEndpointIgnoresToolTokenOnGRPCPort(t *testing.T) {
	proc, root := acpProcFixture(t)
	procLink(t, filepath.Join(proc, "301/ns/net"), "net:[42]")
	procFile(t, filepath.Join(proc, "301/environ"), "ANTIGRAVITY_LS_ADDRESS=localhost:41110\x00ANTIGRAVITY_CSRF_TOKEN=grpc-token\x00ANTIGRAVITY_LS_VERSION=cli-1.5.16\x00")
	got := cliEndpointsFromProc(proc, root, "")
	if len(got) != 1 || got[0].BaseURL != "http://127.0.0.1:41111" || got[0].Token != "" {
		t.Fatalf("got %+v, want only the logged HTTP listener without the gRPC tool token", got)
	}
}

// Log cleanup can truncate the running server's log in place. The open log still
// identifies the server as this root's, but it no longer names a port, so it
// cannot rule out the listener a live tool process advertises with its token.
func TestACPEndpointKeepsToolTokenAfterLogTruncation(t *testing.T) {
	proc, root := acpProcFixture(t)
	procLink(t, filepath.Join(proc, "301/ns/net"), "net:[42]")
	procFile(t, filepath.Join(proc, "301/environ"), "ANTIGRAVITY_LS_ADDRESS=localhost:41111\x00ANTIGRAVITY_CSRF_TOKEN=fixture-token\x00ANTIGRAVITY_LS_VERSION=cli-1.5.16\x00")
	// Truncate the active log in place; its descriptor stays open. Drop the
	// older log so no logged port is left to fall back to.
	if err := os.Truncate(filepath.Join(root, "log/acp-server-20260102_000000-300.log"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "log/acp-server-20260101_000000-111.log")); err != nil {
		t.Fatal(err)
	}
	for _, pinned := range []string{"", "http://127.0.0.1:41111"} {
		got := cliEndpointsFromProc(proc, root, pinned)
		if len(got) != 1 || got[0].BaseURL != "http://127.0.0.1:41111" || got[0].Token != "fixture-token" {
			t.Fatalf("pin %q: got %+v, want the advertised HTTP listener with the tool token", pinned, got)
		}
	}
	if got := cliEndpointsFromProc(proc, root, "http://127.0.0.1:41110"); len(got) != 0 {
		t.Fatalf("selected the gRPC listener, which no tool advertises: %+v", got)
	}
}

// A tool token does not show which server issued it. When the log has lost the
// HTTP port, a stale tool that advertises the server's gRPC port must not come
// first just because a valid tool also advertises the HTTP port: with two
// listeners matching tool tokens, nothing says which is HTTP, so no endpoint is
// returned, as when the log is unreadable.
func TestACPEndpointRefusesStaleGRPCToolTokenAfterLogTruncation(t *testing.T) {
	proc, root := acpProcFixture(t)
	procLink(t, filepath.Join(proc, "301/ns/net"), "net:[42]")
	procFile(t, filepath.Join(proc, "301/environ"), "ANTIGRAVITY_LS_ADDRESS=localhost:41110\x00ANTIGRAVITY_CSRF_TOKEN=grpc-token\x00ANTIGRAVITY_LS_VERSION=cli-1.5.16\x00")
	procLink(t, filepath.Join(proc, "302/ns/net"), "net:[42]")
	procFile(t, filepath.Join(proc, "302/environ"), "ANTIGRAVITY_LS_ADDRESS=localhost:41111\x00ANTIGRAVITY_CSRF_TOKEN=fixture-token\x00ANTIGRAVITY_LS_VERSION=cli-1.5.16\x00")
	// The open log keeps its complete gRPC line and loses the HTTP line.
	if err := os.WriteFile(filepath.Join(root, "log/acp-server-20260102_000000-300.log"), []byte("Language server listening on random port at 41110 for HTTPS (gRPC)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if base, token := cliEndpointFromProc(proc, root, ""); base != "" || token != "" {
		t.Fatalf("got %q with token %v, want no endpoint when two listeners match tool tokens", base, token != "")
	}
	for _, pinned := range []string{"", "http://127.0.0.1:41110"} {
		if got := cliEndpointsFromProc(proc, root, pinned); len(got) != 0 {
			t.Fatalf("pin %q: got %+v, want no candidate", pinned, got)
		}
	}
}
