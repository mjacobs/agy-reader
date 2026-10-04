package main

import (
	"net"
	"net/url"
	"testing"
)

func TestDaemonDialAddrDefaultsThePort(t *testing.T) {
	for raw, want := range map[string]string{
		"http://127.0.0.1":        "127.0.0.1:80",
		"https://daemon.example":  "daemon.example:443",
		"http://127.0.0.1:41234":  "127.0.0.1:41234",
		"http://[::1]":            "[::1]:80",
		"https://[::1]:8443/path": "[::1]:8443",
	} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := daemonDialAddr(u); got != want {
			t.Errorf("daemonDialAddr(%q) = %q, want %q", raw, got, want)
		}
	}
}

// A URL with no scheme is refused even when something listens at its
// address, so the test fails if the scheme check is removed: dialing the
// listener would otherwise succeed.
func TestDaemonReachableRefusesURLWithoutScheme(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	addr := listener.Addr().String()
	if err := daemonReachable("http://" + addr); err != nil {
		t.Fatalf("listener at %s not reachable with a scheme: %v", addr, err)
	}
	if err := daemonReachable("//" + addr); err == nil {
		t.Fatalf("a daemon URL with no scheme was reported reachable although %s was listening", addr)
	}
}
