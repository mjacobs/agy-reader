package daemon

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
		if got := DaemonDialAddr(u); got != want {
			t.Errorf("DaemonDialAddr(%q) = %q, want %q", raw, got, want)
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
	if err := DaemonReachable("http://" + addr); err != nil {
		t.Fatalf("listener at %s not reachable with a scheme: %v", addr, err)
	}
	if err := DaemonReachable("//" + addr); err == nil {
		t.Fatalf("a daemon URL with no scheme was reported reachable although %s was listening", addr)
	}
}

func TestAuthenticationErrorsPreserveClassificationWithoutResponseSecrets(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "sensitive-token", status)
		}))
		client := NewClient(srv.URL)
		_, err := client.FetchTrajectory(t.Context(), "aaa")
		srv.Close()
		if !errors.Is(err, ErrAuthentication) || strings.Contains(err.Error(), "sensitive-token") {
			t.Fatalf("expected sanitized, classifiable error for HTTP %d", status)
		}
	}
}
