package main

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/cli"
	"github.com/mjacobs/agy-reader/internal/discovery"
)

func TestWatchReadOnlyLockPolicy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ANTIGRAVITY_CLI_ROOT", "")
	t.Setenv("AGY_READER_PAIR_VERIFIER", "")
	root := filepath.Join(home, discovery.DefaultRootSubpath)
	lock := filepath.Join(root, ".agy-reader-watch.lock")
	writeFileT(t, lock, nil)
	if err := os.Chmod(lock, 0400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(lock, 0600) })
	f, err := os.OpenFile(lock, os.O_RDWR, 0)
	if err == nil {
		f.Close()
		t.Skip("the current account can write a mode-0400 lock")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatal(err)
	}

	for _, pin := range []string{"--root", "ANTIGRAVITY_CLI_ROOT"} {
		t.Run(pin, func(t *testing.T) {
			t.Setenv("ANTIGRAVITY_DAEMON_URL", "http://127.0.0.1:0")
			args := []string{"--watch", "--watch-interval=10ms", "--watch-idle-timeout=20ms"}
			if pin == "--root" {
				args = append(args, "--root", root)
			} else {
				t.Setenv(pin, root)
			}
			if err := cli.Run(args, io.Discard, io.Discard); !errors.Is(err, os.ErrPermission) {
				t.Fatalf("watch with %s returned %v, want the lock permission error", pin, err)
			}
		})
	}

	for _, timeout := range []string{"0", "20ms"} {
		t.Run("idle-timeout="+timeout, func(t *testing.T) {
			// Direct daemon reachability is a TCP dial. Count accepted connections
			// to distinguish probing from waiting for the store to become writable.
			l, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { l.Close() })
			t.Setenv("ANTIGRAVITY_DAEMON_URL", "http://"+l.Addr().String())
			env := append(os.Environ(), "AGY_READER_TEST_AS_CLI=1")
			logs, exited := startWatchCLI(t, env, "--watch-interval=10ms", "--watch-idle-timeout="+timeout)
			waitForLog(t, logs, exited, "the read-only lock error", func(s string) bool {
				return strings.Contains(s, lock)
			})
			if timeout == "0" {
				if err := l.SetDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
					t.Fatal(err)
				}
				conn, err := l.Accept()
				if err == nil {
					conn.Close()
					t.Fatal("watch probed the denied store with idle timeout disabled")
				}
				if !errors.Is(err, os.ErrDeadlineExceeded) {
					t.Fatal(err)
				}
				select {
				case <-exited:
					t.Fatalf("watch exited with idle timeout disabled:\n%s", logs.String())
				default:
				}
				return
			}
			if err := l.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			for range 4 { // More than the two ticks required to expire the timeout.
				conn, err := l.Accept()
				if err != nil {
					t.Fatalf("watch stopped probing a reachable read-only store: %v\n%s", err, logs.String())
				}
				conn.Close()
			}
			l.Close()
			select {
			case <-exited:
				if !strings.Contains(logs.String(), "all roots idle") {
					t.Fatalf("unexpected watcher exit:\n%s", logs.String())
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("unreachable read-only store kept watch alive:\n%s", logs.String())
			}
		})
	}
}
