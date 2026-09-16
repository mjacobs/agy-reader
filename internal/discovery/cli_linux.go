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

// agy 1.2.4 embeds the language server and exports its address and CSRF
// token to tool processes. /proc exposes those inherited values, but not
// values added to agy's own environment after startup. Match the pair to a
// listening socket owned by an agy process with an open log in this root.
// Never infer root ownership from a tool's cwd or reuse another root's token.
func discoverCLIEndpoint(root, wantedURL string) (string, string) {
	return cliEndpointFromProc("/proc", root, wantedURL)
}

func cliEndpointFromProc(proc, root, wantedURL string) (string, string) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", ""
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	entries, err := os.ReadDir(proc)
	if err != nil {
		return "", ""
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
	fallback := ""
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
		for _, port := range ports {
			base := "http://127.0.0.1:" + port
			if (wantedURL == "" || sameLocalEndpoint(wantedURL, base)) && tokens[port] != "" && !ambiguous[port] {
				return base, tokens[port]
			}
		}
		// Without exported credentials we can still identify the live HTTP
		// listener, so authentication failures aren't reported as "not running".
		if data, err := os.ReadFile(logPath); err == nil {
			if port, ok := parseDaemonPort(string(data)); ok {
				for _, listening := range ports {
					base := "http://127.0.0.1:" + port
					if fallback == "" && port == listening && (wantedURL == "" || sameLocalEndpoint(wantedURL, base)) {
						fallback = base
					}
				}
			}
		}
	}
	return fallback, ""
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
