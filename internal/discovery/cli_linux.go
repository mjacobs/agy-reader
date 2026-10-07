package discovery

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// agy embeds the language server. An explicit --csrf_token launch argument
// remains readable while agy is idle, and so does the token agy-pair's shim
// mode leaves in agy's own environment (cliPairToken); otherwise agy's
// generated credentials are only exposed through tool-process environments.
// Match credentials to
// a listening socket owned by an agy process with an open log in this root.
// Never infer root ownership from a tool's cwd or reuse another root's token.
func discoverCLIEndpoint(root, wantedURL string) (string, string) {
	return cliEndpointFromProc("/proc", root, wantedURL)
}

func cliEndpointFromProc(proc, root, wantedURL string) (string, string) {
	candidates := cliEndpointsFromProc(proc, root, wantedURL)
	if len(candidates) > 0 {
		return candidates[0].BaseURL, candidates[0].Token
	}
	return "", ""
}

func discoverCLIConnections(root string) []Connection { return cliEndpointsFromProc("/proc", root, "") }

func cliEndpointsFromProc(proc, root, wantedURL string) []Connection {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	entries, err := os.ReadDir(proc)
	if err != nil {
		return nil
	}
	var pids []int
	for _, e := range entries {
		if pid, err := strconv.Atoi(e.Name()); err == nil && e.IsDir() {
			pids = append(pids, pid)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(pids)))

	// All candidates stay local to this scan. Conflicting tokens for one
	// address are ambiguous (e.g. a recycled port); don't guess between them.
	tokens := map[string]string{}
	ambiguous := map[string]bool{}
	var fallbacks []string
	var candidates []Connection
	seen := map[string]bool{}
	add := func(base, token string) {
		if !seen[base] {
			candidates = append(candidates, Connection{BaseURL: base, Token: token})
			seen[base] = true
		}
	}
	for _, pid := range pids {
		dir := filepath.Join(proc, strconv.Itoa(pid))
		if !sameNetworkNamespace(proc, dir) {
			continue
		}
		data, err := readProcFile(filepath.Join(dir, "environ"))
		if err != nil {
			continue
		}
		env := procEnvironment(data)
		if !strings.HasPrefix(env["ANTIGRAVITY_LS_VERSION"], "cli-") {
			continue
		}
		port := localAddressPort(env["ANTIGRAVITY_LS_ADDRESS"])
		token := env["ANTIGRAVITY_CSRF_TOKEN"]
		if port == "" || token == "" {
			continue
		}
		if old, ok := tokens[port]; ok && old != token {
			ambiguous[port] = true
		}
		tokens[port] = token
	}
	for _, pid := range pids {
		dir := filepath.Join(proc, strconv.Itoa(pid))
		cmdline, err := readProcFile(filepath.Join(dir, "cmdline"))
		if err != nil || filepath.Base(strings.SplitN(string(cmdline), "\x00", 2)[0]) != "agy" || !sameNetworkNamespace(proc, dir) {
			continue
		}
		fds, err := os.ReadDir(filepath.Join(dir, "fd"))
		if err != nil {
			continue
		}
		sockets := map[string]bool{}
		logPath := ""
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(dir, "fd", fd.Name()))
			if err != nil {
				continue
			}
			if strings.HasPrefix(target, "socket:[") {
				sockets[strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")] = true
			}
			if target == filepath.Join(root, "cli.log") ||
				(filepath.Dir(target) == filepath.Join(root, "log") && strings.HasPrefix(filepath.Base(target), "cli-") && strings.HasSuffix(target, ".log")) {
				logPath = target
			}
		}
		if logPath == "" {
			continue
		}
		ports := listeningPorts(dir, sockets)
		logData, _ := os.ReadFile(logPath)
		httpPort, _ := parseDaemonPort(string(logData))
		token := cliLaunchToken(cmdline)
		if token == "" {
			if environ, err := readProcFile(filepath.Join(dir, "environ")); err == nil {
				token = cliPairToken(cmdline, environ)
			}
		}
		if token != "" {
			// agy owns other listeners too (gRPC, sidecars). Its launch or
			// pair token is usable here only with the HTTP port from its own
			// open log.
			for _, port := range ports {
				base := "http://127.0.0.1:" + port
				if port == httpPort && (wantedURL == "" || sameLocalEndpoint(wantedURL, base)) {
					add(base, token)
				}
			}
		}
		for _, port := range ports {
			base := "http://127.0.0.1:" + port
			if (wantedURL == "" || sameLocalEndpoint(wantedURL, base)) && tokens[port] != "" && !ambiguous[port] {
				add(base, tokens[port])
			}
		}
		// Without exported credentials we can still identify the live HTTP
		// listener, so authentication failures aren't reported as "not running".
		for _, port := range ports {
			base := "http://127.0.0.1:" + port
			if port == httpPort && (wantedURL == "" || sameLocalEndpoint(wantedURL, base)) {
				fallbacks = append(fallbacks, base)
			}
		}
	}
	for _, fallback := range fallbacks {
		add(fallback, "")
	}
	return candidates
}

// Only accept the first argument: without agy's complete flag schema, a
// later --csrf_token-looking argument could be a prompt or another flag's
// value. Reject repeated token flags rather than guessing which one agy used.
// A separate value that starts with "-" may be the next flag, but an inline
// value (--csrf_token=-abc) is agy's token whatever its first character;
// agy-pair's URL-safe tokens start with "-" about one time in 64.
func cliLaunchToken(cmdline []byte) string {
	args := strings.Split(string(cmdline), "\x00")
	if len(args) < 2 {
		return ""
	}
	name, token, inline := strings.Cut(args[1], "=")
	if name != "--csrf_token" && name != "-csrf_token" {
		return ""
	}
	next := 2
	if !inline {
		if len(args) < 3 {
			return ""
		}
		token = args[2]
		next = 3
	}
	if token == "" || (!inline && strings.HasPrefix(token, "-")) {
		return ""
	}
	for _, arg := range args[next:] {
		name, _, _ := strings.Cut(arg, "=")
		if name == "--csrf_token" || name == "-csrf_token" {
			return ""
		}
	}
	return token
}

// scripts/agy-pair.py --token-mode=shim starts agy with the placeholder
// --pair_token_slot as its first argument and the token in
// ANTIGRAVITY_PAIR_TOKEN. Its preload library swaps the placeholder for
// --csrf_token=<token> in agy's memory before agy reads its arguments, so
// /proc/<pid>/cmdline shows only the placeholder. The library removes the
// variable from agy's live environment, so agy's tools do not inherit it, but
// /proc/<pid>/environ keeps the block agy started with. Only the same user
// (and root) can read that file, unlike cmdline.
//
// agy rejects the unknown placeholder and exits with status 2 unless the swap
// happened, so a live agy whose first argument is still the placeholder runs
// with the token from that variable. Trust the variable only then: an agy
// started without the placeholder may have inherited a stale value. Accept
// exactly one non-empty value; the library reads the first, and a second one
// would be ambiguous.
func cliPairToken(cmdline, environ []byte) string {
	args := strings.Split(string(cmdline), "\x00")
	if len(args) < 2 {
		return ""
	}
	if name, _, _ := strings.Cut(args[1], "="); name != "--pair_token_slot" {
		return ""
	}
	token, found := "", false
	for _, entry := range strings.Split(string(environ), "\x00") {
		if name, value, ok := strings.Cut(entry, "="); ok && name == "ANTIGRAVITY_PAIR_TOKEN" {
			if found {
				return ""
			}
			token, found = value, true
		}
	}
	return token
}

func readProcFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, 1<<20))
}

func procEnvironment(data []byte) map[string]string {
	env := map[string]string{}
	for _, entry := range strings.Split(string(data), "\x00") {
		if k, v, ok := strings.Cut(entry, "="); ok {
			switch k {
			case "ANTIGRAVITY_LS_ADDRESS", "ANTIGRAVITY_CSRF_TOKEN", "ANTIGRAVITY_LS_VERSION":
				env[k] = v
			}
		}
	}
	return env
}

func sameNetworkNamespace(proc, dir string) bool {
	self, err := os.Readlink(filepath.Join(proc, "self", "ns", "net"))
	if err != nil {
		return false
	}
	other, err := os.Readlink(filepath.Join(dir, "ns", "net"))
	return err == nil && self == other
}

func listeningPorts(dir string, sockets map[string]bool) []string {
	var ports []string
	for _, name := range []string{"tcp", "tcp6"} {
		data, err := os.ReadFile(filepath.Join(dir, "net", name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			f := strings.Fields(line)
			if len(f) < 10 || f[3] != "0A" || !sockets[f[9]] {
				continue
			}
			host, port, ok := strings.Cut(f[1], ":")
			// agy binds localhost. Accept IPv4 loopback and its IPv6-mapped
			// form; the client connects over IPv4, as legacy discovery does.
			if !ok || (host != "0100007F" && host != "0000000000000000FFFF00000100007F") {
				continue
			}
			n, err := strconv.ParseUint(port, 16, 16)
			if err == nil && n != 0 {
				ports = append(ports, strconv.FormatUint(n, 10))
			}
		}
	}
	return ports
}

func localAddressPort(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil || (host != "localhost" && host != "127.0.0.1") {
		return ""
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		return ""
	}
	return strconv.FormatUint(n, 10)
}

func sameLocalEndpoint(a, b string) bool {
	const prefix = "http://"
	if !strings.HasPrefix(a, prefix) || !strings.HasPrefix(b, prefix) {
		return false
	}
	port := localAddressPort(strings.TrimSuffix(strings.TrimPrefix(a, prefix), "/"))
	return port != "" && port == localAddressPort(strings.TrimPrefix(b, prefix))
}
