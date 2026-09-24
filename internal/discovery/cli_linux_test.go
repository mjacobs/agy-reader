package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func procFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func procLink(t *testing.T, path, target string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}

func cliProcFixture(t *testing.T) (proc, root string) {
	t.Helper()
	proc, root = t.TempDir(), t.TempDir()
	procLink(t, filepath.Join(proc, "self/ns/net"), "net:[42]")
	procLink(t, filepath.Join(proc, "100/ns/net"), "net:[42]")
	procLink(t, filepath.Join(proc, "200/ns/net"), "net:[42]")
	procFile(t, filepath.Join(proc, "100/cmdline"), "/usr/bin/agy\x00")
	logPath := filepath.Join(root, "log/cli-20260915_231752.log")
	procFile(t, logPath, "Language server listening on random port at 45993 for HTTP\n")
	procLink(t, filepath.Join(proc, "100/fd/2"), logPath)
	procLink(t, filepath.Join(proc, "100/fd/9"), "socket:[123]")
	procFile(t, filepath.Join(proc, "100/net/tcp"), fmt.Sprintf("0: 0100007F:%04X 00000000:0000 0A 0 0 0 1000 0 123\n", 45993))
	procFile(t, filepath.Join(proc, "200/environ"), "ANTIGRAVITY_LS_ADDRESS=localhost:45993\x00ANTIGRAVITY_CSRF_TOKEN=test-secret\x00ANTIGRAVITY_LS_VERSION=cli-1.2.4\x00")
	// A short-lived invocation overwrote the shared log with a dead port.
	procFile(t, filepath.Join(root, "cli.log"), "Language server listening on random port at 1 for HTTP\n")
	return proc, root
}

func TestCLIEndpointUsesOwnedSocketAndExportedCredentials(t *testing.T) {
	proc, root := cliProcFixture(t)
	for _, pinned := range []string{"", "http://127.0.0.1:45993", "http://localhost:45993/"} {
		base, token := cliEndpointFromProc(proc, root, pinned)
		if base != "http://127.0.0.1:45993" || token != "test-secret" {
			t.Fatalf("did not discover matched endpoint and token for pin %q", pinned)
		}
	}
}

func TestCLIEndpointRejectsUnrelatedCredentials(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(t *testing.T, proc, root string)
		wantURL bool
	}{
		{"missing environment", func(t *testing.T, p, r string) { os.Remove(filepath.Join(p, "200/environ")) }, true},
		{"other port", func(t *testing.T, p, r string) {
			procFile(t, filepath.Join(p, "200/environ"), "ANTIGRAVITY_LS_ADDRESS=localhost:12345\x00ANTIGRAVITY_CSRF_TOKEN=other\x00ANTIGRAVITY_LS_VERSION=cli-1.2.4\x00")
		}, true},
		{"remote address", func(t *testing.T, p, r string) {
			procFile(t, filepath.Join(p, "200/environ"), "ANTIGRAVITY_LS_ADDRESS=example.com:45993\x00ANTIGRAVITY_CSRF_TOKEN=other\x00ANTIGRAVITY_LS_VERSION=cli-1.2.4\x00")
		}, true},
		{"IDE process", func(t *testing.T, p, r string) {
			procFile(t, filepath.Join(p, "200/environ"), "ANTIGRAVITY_LS_ADDRESS=localhost:45993\x00ANTIGRAVITY_CSRF_TOKEN=other\x00ANTIGRAVITY_LS_VERSION=2.0\x00")
		}, true},
		{"conflicting tokens", func(t *testing.T, p, r string) {
			procLink(t, filepath.Join(p, "201/ns/net"), "net:[42]")
			procFile(t, filepath.Join(p, "201/environ"), "ANTIGRAVITY_LS_ADDRESS=localhost:45993\x00ANTIGRAVITY_CSRF_TOKEN=other\x00ANTIGRAVITY_LS_VERSION=cli-1.2.4\x00")
		}, true},
		{"other namespace", func(t *testing.T, p, r string) {
			os.Remove(filepath.Join(p, "200/ns/net"))
			procLink(t, filepath.Join(p, "200/ns/net"), "net:[99]")
		}, true},
		{"not socket owner", func(t *testing.T, p, r string) { os.Remove(filepath.Join(p, "100/fd/9")) }, false},
		{"wrong daemon executable", func(t *testing.T, p, r string) { procFile(t, filepath.Join(p, "100/cmdline"), "other\x00") }, false},
		{"other root log", func(t *testing.T, p, r string) {
			os.Remove(filepath.Join(p, "100/fd/2"))
			procLink(t, filepath.Join(p, "100/fd/2"), filepath.Join(t.TempDir(), "log/cli-other.log"))
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, r := cliProcFixture(t)
			tc.change(t, p, r)
			base, token := cliEndpointFromProc(p, r, "")
			if (base != "") != tc.wantURL || token != "" {
				t.Fatalf("unexpected endpoint availability or credential accepted: endpoint=%q", base)
			}
		})
	}
}

func TestCLIEndpointDoesNotSendTokenToPinnedOtherEndpoint(t *testing.T) {
	p, r := cliProcFixture(t)
	for _, pinned := range []string{"http://127.0.0.1:12345", "http://example.com:45993", "https://localhost:45993", "http://localhost:45993/other"} {
		if base, token := cliEndpointFromProc(p, r, pinned); base != "" || token != "" {
			t.Fatalf("discovered credentials for unrelated pin %q", pinned)
		}
	}
}

func TestCLIEndpointRequiresListenerNotEstablishedSocket(t *testing.T) {
	p, r := cliProcFixture(t)
	path := filepath.Join(p, "100/net/tcp")
	data, _ := os.ReadFile(path)
	procFile(t, path, strings.ReplaceAll(string(data), " 0A ", " 01 "))
	if base, _ := cliEndpointFromProc(p, r, ""); base != "" {
		t.Fatalf("selected a non-listening socket: %s", base)
	}
}

func TestCLIEndpointPrefersAuthenticatedDaemonOverNewerWithoutToken(t *testing.T) {
	p, r := cliProcFixture(t)
	procLink(t, filepath.Join(p, "300/ns/net"), "net:[42]")
	procFile(t, filepath.Join(p, "300/cmdline"), "agy\x00")
	logPath := filepath.Join(r, "log/cli-newer.log")
	procFile(t, logPath, "Language server listening on random port at 35905 for HTTP\n")
	procLink(t, filepath.Join(p, "300/fd/2"), logPath)
	procLink(t, filepath.Join(p, "300/fd/9"), "socket:[456]")
	procFile(t, filepath.Join(p, "300/net/tcp"), fmt.Sprintf("0: 0100007F:%04X 00000000:0000 0A 0 0 0 1000 0 456\n", 35905))
	base, token := cliEndpointFromProc(p, r, "")
	if base != "http://127.0.0.1:45993" || token == "" {
		t.Fatalf("newer unauthenticated daemon hid the usable endpoint: %s", base)
	}
}

func TestCLIEndpointUsesLaunchTokenWithoutToolProcesses(t *testing.T) {
	for _, args := range []string{
		"--csrf_token=launch-secret\x00",
		"--csrf_token\x00launch-secret\x00",
		"-csrf_token=launch-secret\x00",
		"-csrf_token\x00launch-secret\x00",
	} {
		p, r := cliProcFixture(t)
		if err := os.RemoveAll(filepath.Join(p, "200")); err != nil {
			t.Fatal(err)
		}
		procFile(t, filepath.Join(p, "100/cmdline"), "agy\x00"+args+"--input-format=stream-json\x00")
		for _, pin := range []string{"", "http://127.0.0.1:45993", "http://localhost:45993/"} {
			base, token := cliEndpointFromProc(p, r, pin)
			if base != "http://127.0.0.1:45993" || token != "launch-secret" {
				t.Fatal("idle daemon's launch credentials were not discovered")
			}
		}
	}
}

func TestCLIEndpointLaunchTokenRequiresOwnedHTTPListener(t *testing.T) {
	for _, tc := range []struct {
		name   string
		pin    string
		change func(t *testing.T, p, r string)
	}{
		{"other root", "", func(t *testing.T, p, r string) {
			os.Remove(filepath.Join(p, "100/fd/2"))
			procLink(t, filepath.Join(p, "100/fd/2"), filepath.Join(t.TempDir(), "cli.log"))
		}},
		{"other namespace", "", func(t *testing.T, p, r string) {
			os.Remove(filepath.Join(p, "100/ns/net"))
			procLink(t, filepath.Join(p, "100/ns/net"), "net:[99]")
		}},
		{"no socket", "", func(t *testing.T, p, r string) { os.Remove(filepath.Join(p, "100/fd/9")) }},
		{"no HTTP log", "", func(t *testing.T, p, r string) {
			procFile(t, filepath.Join(r, "log/cli-20260915_231752.log"), "no HTTP listener yet\n")
		}},
		{"wrong process", "", func(t *testing.T, p, r string) {
			procFile(t, filepath.Join(p, "100/cmdline"), "other\x00--csrf_token=launch-secret\x00")
		}},
		{"other port", "http://127.0.0.1:12345", nil},
		{"remote host", "http://example.com:45993", nil},
		{"HTTPS", "https://localhost:45993", nil},
		{"owned non-HTTP port", "http://localhost:35905", func(t *testing.T, p, r string) {
			procLink(t, filepath.Join(p, "100/fd/10"), "socket:[456]")
			procFile(t, filepath.Join(p, "100/net/tcp"), fmt.Sprintf("0: 0100007F:%04X 00000000:0000 0A 0 0 0 1000 0 123\n1: 0100007F:%04X 00000000:0000 0A 0 0 0 1000 0 456\n", 45993, 35905))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, r := cliProcFixture(t)
			os.RemoveAll(filepath.Join(p, "200"))
			procFile(t, filepath.Join(p, "100/cmdline"), "agy\x00--csrf_token=launch-secret\x00")
			if tc.change != nil {
				tc.change(t, p, r)
			}
			if _, token := cliEndpointFromProc(p, r, tc.pin); token != "" {
				t.Fatal("launch token escaped its owning HTTP endpoint")
			}
		})
	}
}

func TestCLIEndpointDoesNotReadLaunchTokenFromPrompt(t *testing.T) {
	for _, args := range []string{
		"--print\x00--csrf_token=prompt-text\x00",
		"--\x00--csrf_token=prompt-text\x00",
		"--csrf_token_file=some-file\x00",
		"--csrf_token\x00--print\x00prompt\x00",
		"--csrf_token=\x00",
		"--csrf_token\x00",
		"--csrf_token=first\x00--csrf_token=second\x00",
	} {
		p, r := cliProcFixture(t)
		os.RemoveAll(filepath.Join(p, "200"))
		procFile(t, filepath.Join(p, "100/cmdline"), "agy\x00"+args)
		if _, token := cliEndpointFromProc(p, r, ""); token != "" {
			t.Fatal("ambiguous launch arguments accepted as credentials")
		}
	}
}
