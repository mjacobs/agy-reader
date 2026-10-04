package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/testfixture"
)

// heldRefusalDaemon answers the trajectory RPCs with the unsupported projection
// payload, so the watcher holds the source after its first fetch, and answers
// the read-only GetAllCascadeTrajectories RPC that probes it. rpcs counts the
// trajectory RPCs, probes the probe. Other paths get 404. With token set, a
// request without that CSRF token gets 403. Once gone is set, every request
// gets 502, as from a proxy whose daemon has disappeared.
type heldRefusalDaemon struct {
	token  string
	rpcs   atomic.Int64
	probes atomic.Int64
	gone   atomic.Bool
}

func (d *heldRefusalDaemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if d.gone.Load() {
		http.Error(w, "upstream unreachable", http.StatusBadGateway)
		return
	}
	if d.token != "" && r.Header.Get(daemon.CSRFHeader) != d.token {
		http.Error(w, "missing CSRF token", http.StatusForbidden)
		return
	}
	switch {
	case strings.HasSuffix(r.URL.Path, "/LoadTrajectory"):
		d.rpcs.Add(1)
		io.WriteString(w, `{}`)
	case strings.HasSuffix(r.URL.Path, "/GetCascadeTrajectory"):
		d.rpcs.Add(1)
		w.Write(append(append([]byte(`{"trajectory":`), unsupportedProjectionPayload("first")...), '}'))
	case strings.HasSuffix(r.URL.Path, "/GetAllCascadeTrajectories"):
		d.probes.Add(1)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"trajectorySummaries":{}}`)
	default:
		http.NotFound(w, r)
	}
}

// lockedBuffer collects a child process's stderr while the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// startProxiedWatch runs the real watch command in a child process (the test
// binary in CLI mode), so Go's default transport reads HTTP_PROXY fresh. The
// daemon host does not resolve; only the proxy reaches it. extraEnv is added
// to the child's environment.
func startProxiedWatch(t *testing.T, root, proxyURL string, extraEnv ...string) (logs *lockedBuffer, exited <-chan struct{}) {
	t.Helper()
	env := []string{}
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(name) {
		case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "REQUEST_METHOD",
			"ANTIGRAVITY_DAEMON_URL", "ANTIGRAVITY_CSRF_TOKEN", "ANTIGRAVITY_CLI_ROOT":
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"AGY_READER_TEST_AS_CLI=1",
		"ANTIGRAVITY_DAEMON_URL=http://daemon.invalid:41234",
		"HTTP_PROXY="+proxyURL,
		"NO_PROXY=",
	)
	env = append(env, extraEnv...)
	cmd := exec.Command(os.Args[0], "--watch", "--root", root, "--watch-interval=1s", "--watch-idle-timeout=2s")
	cmd.Env = env
	logs = &lockedBuffer{}
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		cmd.Process.Kill()
		<-done
	})
	return logs, done
}

// waitForLog waits until logs satisfy ok, failing if the child exits first or
// the deadline passes.
func waitForLog(t *testing.T, logs *lockedBuffer, exited <-chan struct{}, what string, ok func(string) bool) {
	t.Helper()
	deadline := time.After(15 * time.Second)
	for !ok(logs.String()) {
		select {
		case <-exited:
			if ok(logs.String()) {
				return
			}
			t.Fatalf("watcher exited before %s:\n%s", what, logs.String())
		case <-deadline:
			t.Fatalf("timed out waiting for %s:\n%s", what, logs.String())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// A daemon reachable only through HTTP_PROXY keeps the watcher alive after its
// one source is held. The held ticks make no RPC, so the reachability check
// must go the way the RPC client does; a direct dial of the unresolvable
// daemon host counted each held tick as idle and exited after the 2s timeout
// (review finding rv-bgjm.1).
func TestWatchHeldRefusalKeepsProxyReachableDaemonAlive(t *testing.T) {
	t.Parallel()
	root, _, _ := unsupportedProjectionFixture(t)
	d := &heldRefusalDaemon{}
	proxy := httptest.NewServer(d)
	t.Cleanup(proxy.Close)

	logs, exited := startProxiedWatch(t, root, proxy.URL)
	waitForLog(t, logs, exited, "the refusal to be held", func(s string) bool {
		return strings.Contains(s, "not retrying until its source or the daemon endpoint changes")
	})
	held := d.rpcs.Load()
	if held == 0 {
		t.Fatalf("source was not fetched through the proxy:\n%s", logs.String())
	}
	// The initial tick plus three held ones: past the 2s idle timeout.
	waitForLog(t, logs, exited, "three held ticks", func(s string) bool {
		return strings.Count(s, "tick: ") >= 4
	})
	select {
	case <-exited:
		t.Fatalf("watcher exited while the daemon was reachable through the proxy:\n%s", logs.String())
	default:
	}
	if s := logs.String(); strings.Contains(s, "unreachable") || strings.Contains(s, "exiting") {
		t.Fatalf("proxy-reachable daemon reported unreachable:\n%s", s)
	}
	if got := d.rpcs.Load(); got != held {
		t.Fatalf("held source fetched again through the proxy: %d RPCs, want %d", got, held)
	}
	if d.probes.Load() == 0 {
		t.Fatalf("held ticks did not probe the daemon through the proxy:\n%s", logs.String())
	}
}

// A daemon that requires its CSRF token stays reachable on held ticks when the
// token is configured: the probe carries the token like the RPCs do. A
// tokenless probe got 403 and the watcher exited on its idle timeout (review
// finding rv-rc1b.2).
func TestWatchHeldRefusalKeepsCSRFDaemonBehindProxyAlive(t *testing.T) {
	t.Parallel()
	root, _, _ := unsupportedProjectionFixture(t)
	d := &heldRefusalDaemon{token: "test-csrf-token"}
	proxy := httptest.NewServer(d)
	t.Cleanup(proxy.Close)

	logs, exited := startProxiedWatch(t, root, proxy.URL, "ANTIGRAVITY_CSRF_TOKEN="+d.token)
	waitForLog(t, logs, exited, "the refusal to be held", func(s string) bool {
		return strings.Contains(s, "not retrying until its source or the daemon endpoint changes")
	})
	held := d.rpcs.Load()
	if held == 0 {
		t.Fatalf("source was not fetched through the proxy:\n%s", logs.String())
	}
	waitForLog(t, logs, exited, "three held ticks", func(s string) bool {
		return strings.Count(s, "tick: ") >= 4
	})
	select {
	case <-exited:
		t.Fatalf("watcher exited while the daemon accepted its token through the proxy:\n%s", logs.String())
	default:
	}
	if s := logs.String(); strings.Contains(s, "unreachable") || strings.Contains(s, "exiting") {
		t.Fatalf("daemon accepting the configured token reported unreachable:\n%s", s)
	}
	if got := d.rpcs.Load(); got != held {
		t.Fatalf("held source fetched again through the proxy: %d RPCs, want %d", got, held)
	}
	if d.probes.Load() == 0 {
		t.Fatalf("held ticks did not probe the daemon through the proxy:\n%s", logs.String())
	}
}

// When the proxy can no longer reach the daemon behind it, held ticks count as
// idle again and the watcher exits on its idle timeout.
func TestWatchHeldRefusalExitsWhenDaemonBehindProxyIsGone(t *testing.T) {
	t.Parallel()
	root, _, _ := unsupportedProjectionFixture(t)
	d := &heldRefusalDaemon{}
	proxy := httptest.NewServer(d)
	t.Cleanup(proxy.Close)

	logs, exited := startProxiedWatch(t, root, proxy.URL)
	waitForLog(t, logs, exited, "the refusal to be held", func(s string) bool {
		return strings.Contains(s, "not retrying until its source or the daemon endpoint changes")
	})
	d.gone.Store(true)
	select {
	case <-exited:
	case <-time.After(15 * time.Second):
		t.Fatalf("watcher kept running with the daemon behind the proxy gone:\n%s", logs.String())
	}
	if s := logs.String(); !strings.Contains(s, "daemon error 502") || !strings.Contains(s, "all roots idle") {
		t.Fatalf("exit not caused by the unreachable daemon:\n%s", s)
	}
}

// A proxy that answers the probe itself and never forwards it says nothing
// about the daemon, whatever its status code. With a source refused before any
// RPC, that probe is the only contact, and the watcher must exit on its idle
// timeout rather than run forever: a 407 asking for the proxy's credentials
// (review finding rv-by1c.1), a 200 login page (review finding rv-rc1b.1),
// a 200 JSON object that is not the RPC's response (review finding rv-8grd.1),
// or a 401 or 403 of the proxy's own, which looks the same as the daemon
// refusing the token (agy-reader#c8vx).
func TestWatchUnreadableSourceExitsWhenOnlyProxyAnswers(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		answer  http.HandlerFunc
		wantLog string
	}{
		"proxy authentication": {
			answer: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Proxy-Authenticate", `Basic realm="proxy"`)
				http.Error(w, "proxy authentication required", http.StatusProxyAuthRequired)
			},
			wantLog: "daemon error 407",
		},
		"proxy policy refusal": {
			answer: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "access denied by proxy policy", http.StatusForbidden)
			},
			wantLog: "HTTP 403",
		},
		"proxy login required": {
			answer: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="gateway"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			},
			wantLog: "HTTP 401",
		},
		"login page": {
			answer: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				io.WriteString(w, "<html><body>Sign in to continue</body></html>")
			},
			wantLog: "unmarshal GetAllCascadeTrajectories response",
		},
		"unrelated json object": {
			answer: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"error":"upstream unavailable"}`)
			},
			wantLog: "not a GetAllCascadeTrajectories response",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, _, db := completeFixture(t)
			testfixture.Exec(t, db, "PRAGMA user_version=2")
			var probes atomic.Int64
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				probes.Add(1)
				tc.answer(w, r)
			}))
			t.Cleanup(proxy.Close)

			logs, exited := startProxiedWatch(t, root, proxy.URL)
			select {
			case <-exited:
			case <-time.After(15 * time.Second):
				t.Fatalf("watcher kept running while only the proxy answered:\n%s", logs.String())
			}
			if probes.Load() == 0 {
				t.Fatalf("reachability probe did not go through the proxy:\n%s", logs.String())
			}
			if s := logs.String(); !strings.Contains(s, tc.wantLog) || !strings.Contains(s, "all roots idle") {
				t.Fatalf("exit not caused by the proxy's own answer:\n%s", s)
			}
		})
	}
}

// A proxied probe counts as reachable only when the daemon's read-only RPC
// succeeds with a response message, sent with the client's CSRF token. Any
// answer the proxy gives on its own fails, whatever its status: an error, a
// login page served as 200, a redirect to one (not followed), or a body that
// is not the RPC's response.
func TestClientReachesDaemonThroughProxyNeedsRPCResponse(t *testing.T) {
	t.Parallel()
	const token = "test-csrf-token"
	rpc := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"trajectorySummaries":{}}`)
	}
	status := func(code int, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { http.Error(w, body, code) }
	}
	for name, tc := range map[string]struct {
		answer http.HandlerFunc
		token  string
		want   bool
	}{
		"rpc response":       {answer: rpc, want: true},
		"empty rpc response": {answer: func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) }, want: true},
		"rpc response with sessions": {answer: func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"trajectorySummaries":{"c1":{"summary":"s"}},"newerField":1}`)
		}, want: true},
		"rpc response with token": {answer: func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(daemon.CSRFHeader) != token {
				http.Error(w, "missing CSRF token", http.StatusForbidden)
				return
			}
			rpc(w, r)
		}, token: token, want: true},
		"login page as 200": {answer: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, "<html><body>Sign in to continue</body></html>")
		}},
		"redirect to login page": {answer: func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/login" {
				rpc(w, r)
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
		}},
		"unrelated json object": {answer: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"error":"upstream unavailable"}`)
		}},
		"null summaries":        {answer: func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"trajectorySummaries":null}`) }},
		"null body":             {answer: func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `null`) }},
		"json array":            {answer: func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `[]`) }},
		"empty body":            {answer: func(w http.ResponseWriter, r *http.Request) {}},
		"missing token":         {answer: status(http.StatusForbidden, "missing CSRF token")},
		"daemon auth":           {answer: status(http.StatusUnauthorized, "missing CSRF token")},
		"not found":             {answer: status(http.StatusNotFound, "404 page not found")},
		"proxy authentication":  {answer: status(http.StatusProxyAuthRequired, "proxy authentication required")},
		"proxy bad gateway":     {answer: status(http.StatusBadGateway, "upstream unreachable")},
		"proxy gateway timeout": {answer: status(http.StatusGatewayTimeout, "upstream timed out")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			proxy := httptest.NewServer(tc.answer)
			t.Cleanup(proxy.Close)
			proxyURL, err := url.Parse(proxy.URL)
			if err != nil {
				t.Fatal(err)
			}
			transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
			t.Cleanup(transport.CloseIdleConnections)
			client := daemon.NewClient("http://daemon.invalid:41234")
			client.HTTP.Transport = transport
			client.CSRFToken = tc.token
			err = clientReachesDaemon(t.Context(), client)
			if got := err == nil; got != tc.want {
				t.Fatalf("reachable=%v (err %v), want %v", got, err, tc.want)
			}
		})
	}
}

// A held source does not hide a directly reachable daemon that has died: the
// held tick's dial fails and counts toward the idle timeout.
func TestWatchHeldRefusalDoesNotHideDeadDaemon(t *testing.T) {
	root, _, _ := unsupportedProjectionFixture(t)
	srv := httptest.NewServer(&heldRefusalDaemon{})
	t.Cleanup(srv.Close)
	var logs bytes.Buffer
	w := heldWatcher(t, root, srv.URL, &logs)
	w.idleTimeout = time.Second

	if w.tick() || w.consecutiveFailures != 0 {
		t.Fatalf("reachable daemon treated as idle on the refused fetch:\n%s", logs.String())
	}
	if w.tick() || w.consecutiveFailures != 0 {
		t.Fatalf("reachable daemon treated as idle on a held tick:\n%s", logs.String())
	}
	srv.Close()
	if !w.tick() || w.consecutiveFailures != 1 {
		t.Fatalf("held source hid a dead daemon (failures=%d):\n%s", w.consecutiveFailures, logs.String())
	}
}

// A held tick's probe refused with 403 through a proxy counts toward the idle
// timeout, even when the daemon behind the proxy is alive and has only rotated
// its CSRF token. Nothing known about the daemon's own refusal tells it apart
// from a proxy answering 401 or 403 on its own, and treating that answer as a
// live daemon would keep an absent daemon's watcher running forever (see
// TestWatchUnreadableSourceExitsWhenOnlyProxyAnswers). So the watcher exits,
// and a relaunch with the current token reaches the daemon again. Review
// finding rv-8grd.2 asked for the watcher to stay alive; this documents why it
// does not.
func TestWatchHeldRefusalProbeAuthFailureExpiresAndRelaunchRecovers(t *testing.T) {
	t.Parallel()
	root, _, _ := unsupportedProjectionFixture(t)
	d := &heldRefusalDaemon{}
	var token atomic.Pointer[string]
	oldToken, newToken := "old-token", "new-token"
	token.Store(&oldToken)
	var rejected atomic.Int64
	// The daemon checks its current token; the proxy forwards everything.
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(daemon.CSRFHeader) != *token.Load() {
			rejected.Add(1)
			http.Error(w, "missing CSRF token", http.StatusForbidden)
			return
		}
		d.ServeHTTP(w, r)
	}))
	t.Cleanup(proxy.Close)

	logs, exited := startProxiedWatch(t, root, proxy.URL, "ANTIGRAVITY_CSRF_TOKEN="+oldToken)
	waitForLog(t, logs, exited, "the refusal to be held", func(s string) bool {
		return strings.Contains(s, "not retrying until its source or the daemon endpoint changes")
	})
	held := d.rpcs.Load()
	token.Store(&newToken)
	select {
	case <-exited:
	case <-time.After(15 * time.Second):
		t.Fatalf("watcher kept running on a probe refused with 403 through a proxy:\n%s", logs.String())
	}
	s := logs.String()
	if !strings.Contains(s, "HTTP 403") || !strings.Contains(s, "all roots idle") {
		t.Fatalf("exit not caused by the refused probe:\n%s", s)
	}
	if strings.Contains(s, "authentication failed; sync blocked") {
		t.Fatalf("refused probe handled as a live daemon's authentication failure:\n%s", s)
	}
	if rejected.Load() == 0 {
		t.Fatalf("held ticks did not probe the daemon after the token rotation:\n%s", s)
	}
	if got := d.rpcs.Load(); got != held {
		t.Fatalf("held source fetched again: %d RPCs, want %d", got, held)
	}

	// A relaunch with the current token reaches the daemon and holds the
	// source again, so it fetches it once more.
	relogs, reexited := startProxiedWatch(t, root, proxy.URL, "ANTIGRAVITY_CSRF_TOKEN="+newToken)
	waitForLog(t, relogs, reexited, "the relaunched watcher to hold the refusal", func(s string) bool {
		return strings.Contains(s, "not retrying until its source or the daemon endpoint changes")
	})
	if got := d.rpcs.Load(); got <= held {
		t.Fatalf("relaunched watcher did not reach the daemon: %d RPCs, want more than %d\n%s", got, held, relogs.String())
	}
	if s := relogs.String(); strings.Contains(s, "HTTP 403") {
		t.Fatalf("relaunched watcher refused with the current token:\n%s", s)
	}
}
