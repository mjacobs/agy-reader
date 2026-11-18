package discovery

import (
	"bytes"
	"cmp"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func discoverCLIEndpoint(root, wantedURL string) (string, string) {
	if found := darwinCLIConnections(root, wantedURL); len(found) != 0 {
		return found[0].BaseURL, found[0].Token
	}
	return "", ""
}

func discoverCLIConnections(root string) []Connection { return darwinCLIConnections(root, "") }

func darwinCLIConnections(root, wantedURL string) []Connection {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil
	}
	uid := os.Getuid()
	processes, err := unix.SysctlKinfoProcSlice("kern.proc.uid", uid)
	if err != nil {
		return nil
	}
	children := make(map[int32][]unix.KinfoProc)
	var cli []unix.KinfoProc
	for _, p := range processes {
		if p.Eproc.Pcred.P_ruid != uint32(uid) || p.Proc.P_pid <= 0 {
			continue
		}
		children[p.Eproc.Ppid] = append(children[p.Eproc.Ppid], p)
		if unix.ByteSliceToString(p.Proc.P_comm[:]) == "agy" {
			cli = append(cli, p)
		}
	}
	slices.SortFunc(cli, func(a, b unix.KinfoProc) int {
		return cmp.Compare(b.Proc.P_starttime.Nano(), a.Proc.P_starttime.Nano())
	})

	var authenticated, pending []Connection
	for _, p := range cli {
		listeners := darwinCLIListeners(root, p.Proc.P_pid)
		for _, address := range listeners {
			base := "http://" + address
			if wantedURL != "" && !sameLocalEndpoint(wantedURL, base) {
				continue
			}
			token := darwinCLIToolToken(p.Proc.P_pid, children, address)
			if !sameDarwinProcess(p) {
				break
			}
			conn := Connection{BaseURL: base, Token: token}
			if token != "" {
				authenticated = append(authenticated, conn)
			} else {
				pending = append(pending, conn)
			}
		}
	}
	return append(authenticated, pending...)
}

// Use PID selection because lsof may truncate the executable's name. Accept
// only a loopback TCP listener whose port is declared HTTP in a CLI log
// this process has open inside the requested store.
func darwinCLIListeners(root string, pid int32) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "/usr/sbin/lsof", "-nPa", "-p", strconv.Itoa(int(pid)), "-FfntT0").Output()
	if err != nil {
		return nil
	}
	return darwinCLIHTTPListeners(root, data)
}

func darwinCLIHTTPListeners(root string, data []byte) []string {
	var name, fileType string
	var descriptor, listening bool
	var listeners []string
	ports := make(map[string]bool)
	logs := make(map[string]bool)
	finishFile := func() {
		if !descriptor {
			return
		}
		if listening && (fileType == "IPv4" || fileType == "IPv6") {
			if address := localAddressKey(name); address != "" {
				listeners = append(listeners, address)
			}
		}
		if fileType == "REG" && !logs[name] && (name == filepath.Join(root, "cli.log") ||
			(filepath.Dir(name) == filepath.Join(root, "log") && strings.HasPrefix(filepath.Base(name), "cli-") && strings.HasSuffix(name, ".log"))) {
			logs[name] = true
			if log, err := os.ReadFile(name); err == nil {
				if port, ok := parseDaemonPort(string(log)); ok {
					ports[port] = true
				}
			}
		}
	}
	for _, field := range strings.Split(string(data), "\x00") {
		field = strings.TrimPrefix(field, "\n")
		if field == "" {
			continue
		}
		switch field[0] {
		case 'f':
			finishFile()
			_, err := strconv.Atoi(field[1:])
			descriptor = err == nil
			name, fileType, listening = "", "", false
		case 't':
			fileType = field[1:]
		case 'n':
			name = field[1:]
		case 'T':
			listening = listening || field == "TST=LISTEN"
		}
	}
	finishFile()
	return slices.DeleteFunc(listeners, func(address string) bool {
		return !ports[address[strings.LastIndexByte(address, ':')+1:]]
	})
}

// Same-user descendants can carry the credentials agy delegates to tools.
// The watcher never probes unrelated environments or crosses daemon endpoints.
func darwinCLIToolToken(pid int32, children map[int32][]unix.KinfoProc, address string) string {
	queue := append([]unix.KinfoProc(nil), children[pid]...)
	token := ""
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if p.Proc.P_pid == pid || !sameDarwinProcess(p) {
			continue
		}
		queue = append(queue, children[p.Proc.P_pid]...)
		data, err := unix.SysctlRaw("kern.procargs2", int(p.Proc.P_pid))
		if err != nil || !sameDarwinProcess(p) {
			continue
		}
		candidate := darwinCLIEnvironmentToken(data, address)
		if candidate == "" {
			continue
		}
		if token != "" && token != candidate {
			return ""
		}
		token = candidate
	}
	return token
}

func sameDarwinProcess(p unix.KinfoProc) bool {
	current, err := unix.SysctlKinfoProc("kern.proc.pid", int(p.Proc.P_pid))
	return err == nil && current.Proc.P_starttime == p.Proc.P_starttime &&
		current.Eproc.Ppid == p.Eproc.Ppid && current.Eproc.Pcred.P_ruid == p.Eproc.Pcred.P_ruid &&
		current.Eproc.Ucred.Uid == p.Eproc.Ucred.Uid
}

// KERN_PROCARGS2 starts with argc, the executable path and alignment padding,
// then argc NUL-terminated arguments followed by the initial environment.
func darwinCLIEnvironmentToken(data []byte, address string) string {
	if len(data) < 4 {
		return ""
	}
	argc := int(binary.NativeEndian.Uint32(data[:4]))
	_, args, ok := bytes.Cut(data[4:], []byte{0})
	if !ok || argc <= 0 {
		return ""
	}
	args = bytes.TrimLeft(args, "\x00")
	for range argc {
		_, args, ok = bytes.Cut(args, []byte{0})
		if !ok {
			return ""
		}
	}
	var endpoint, token, version string
	for len(args) > 0 && args[0] != 0 {
		var entry []byte
		entry, args, ok = bytes.Cut(args, []byte{0})
		if !ok {
			return ""
		}
		key, value, hasValue := bytes.Cut(entry, []byte{'='})
		if !hasValue {
			continue
		}
		var field *string
		switch string(key) {
		case "ANTIGRAVITY_LS_ADDRESS":
			field = &endpoint
		case "ANTIGRAVITY_CSRF_TOKEN":
			field = &token
		case "ANTIGRAVITY_LS_VERSION":
			field = &version
		default:
			continue
		}
		if *field != "" || len(value) == 0 {
			return ""
		}
		*field = string(value)
	}
	if !strings.HasPrefix(version, "cli-") || localAddressKey(endpoint) != address {
		return ""
	}
	return token
}
