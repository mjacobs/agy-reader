// Package discovery enumerates Antigravity session files on disk and locates
// the language-server daemon serving them. It covers the surfaces that share
// one on-disk format: the Antigravity CLI (`agy`, ~/.gemini/antigravity-cli),
// the Antigravity IDE (~/.gemini/antigravity), and T3 Code's Antigravity ACP
// provider (~/.t3/userdata/providers/antigravity/<hash>/antigravity-acp),
// which runs the CLI's language server as `antigravity acp`.
package discovery

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// DefaultRootSubpath is the user-relative location of the CLI session tree.
const DefaultRootSubpath = ".gemini/antigravity-cli"

// acpProvidersSubpath is the user-relative directory where T3 Code's
// Antigravity ACP provider creates per-instance subdirectories, each holding
// the app data directory T3 Code passes as --app-data-dir.
var acpProvidersSubpath = filepath.Join(".t3", "userdata", "providers", "antigravity")

// Subdirs that hold session files inside the root.
var subdirs = []string{"conversations", "implicit"}

// Session describes one discovered .pb or .db session file.
type Session struct {
	// CascadeID is the bare UUID (filename minus suffix).
	CascadeID string
	// InternalCascadeID is the cascade_id recorded inside the session file,
	// when known and distinct from CascadeID.
	InternalCascadeID string
	// PBPath is the absolute path to the session file (.db or legacy .pb).
	PBPath string
	// SidecarPath is where the trajectory sidecar lives (or would live)
	// — same dir, suffix replaced with .trajectory.json.
	SidecarPath string
	// Bucket is "conversations" or "implicit".
	Bucket string
	// ModTime is the .pb or .db file's last-modified time.
	ModTime time.Time
}

// Root returns the configured root directory for CLI sessions. Honors
// ANTIGRAVITY_CLI_ROOT if set, otherwise ~/.gemini/antigravity-cli.
func Root() (string, error) {
	if v := os.Getenv("ANTIGRAVITY_CLI_ROOT"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, DefaultRootSubpath), nil
}

// DefaultRoots returns the session roots a bare invocation (no --root)
// operates on. ANTIGRAVITY_CLI_ROOT, when set, wins as the single root —
// explicit configuration never grows extra stores, so existing scripts keep
// their exact behavior. Otherwise each of the known store locations that
// exists on disk is included, CLI first: ~/.gemini/antigravity-cli (the agy
// CLI), ~/.gemini/antigravity (Antigravity 2.0), then each T3 Code ACP root
// under ~/.t3/userdata/providers/antigravity in path order. Nothing else under
// ~/.gemini is ever scanned — that directory is Gemini CLI's config home,
// and the empty antigravity-ide/ dir some installs leave there is a known
// red herring. With neither store on disk the CLI default is returned alone.
func DefaultRoots() ([]string, error) {
	if v := os.Getenv("ANTIGRAVITY_CLI_ROOT"); v != "" {
		return []string{v}, nil
	}
	return DefaultStoreRoots()
}

// DefaultStoreRoots is DefaultRoots without the ANTIGRAVITY_CLI_ROOT
// override: the roots a bare invocation with no env pin would operate on.
// Use it to reason about some OTHER process's bare invocation (e.g. doctor
// attributing a running `--watch`), where honoring this process's env pin
// would give the wrong answer.
func DefaultStoreRoots() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home: %w", err)
	}
	return DefaultStoreRootsUnder(home), nil
}

// readProvidersDir lists the ACP providers directory. A variable so tests can
// inject a read that fails partway through.
var readProvidersDir = os.ReadDir

func acpStoreRootsUnder(home string) []string {
	dir := filepath.Join(home, acpProvidersSubpath)
	// os.ReadDir returns the entries it read before an error along with the
	// error. A read that fails partway still names instances that exist, and
	// DefaultStoreRootsUnder stats each candidate, so keep what was listed. A
	// read that fails at once lists nothing.
	entries, _ := readProvidersDir(dir)
	roots := make([]string, 0, len(entries))
	for _, entry := range entries {
		// os.ReadDir returns entries sorted by name and leaves directory/symlink
		// checks to os.Stat in DefaultStoreRootsUnder.
		roots = append(roots, filepath.Join(dir, entry.Name(), "antigravity-acp"))
	}
	return roots
}

// DefaultStoreRootsUnder computes the default store roots under an explicit
// home directory — for reasoning about a process whose home is not this
// process's (e.g. doctor attributing another user context's watcher).
func DefaultStoreRootsUnder(home string) []string {
	cli := filepath.Join(home, DefaultRootSubpath)
	roots := []string{}
	candidates := []string{cli, filepath.Join(home, DefaultIDERootSubpath)}
	candidates = append(candidates, acpStoreRootsUnder(home)...)
	for _, root := range candidates {
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			roots = append(roots, root)
		}
	}
	if len(roots) == 0 {
		roots = []string{cli}
	}
	return roots
}

// ListSessions walks the conversations/ and implicit/ subdirs under root
// and returns one Session per conversation, sorted by ModTime descending.
//
// A conversation is normally one .pb or one .db file. agy 1.2.17 leaves both
// for a forked conversation: <id>.pb written when the fork is created and
// <id>.db once the fork continues. Such an id is listed once, with the .db as
// its source, because the .db holds the later, complete history (see
// agy-reader#3g5w). The .pb is not read.
//
// Missing subdirs are silently skipped; a missing root returns an empty list
// without error.
func ListSessions(root string) ([]Session, error) {
	return listSessions(root, subdirs)
}

// ListConversationSessions returns sessions the daemon can currently load via
// LoadTrajectory. The RPC accepts only a cascade ID and resolves it under
// conversations/, so implicit/ entries are visible to ListSessions but not
// syncable here.
func ListConversationSessions(root string) ([]Session, error) {
	return listSessions(root, []string{"conversations"})
}

func listSessions(root string, buckets []string) ([]Session, error) {
	if _, err := os.Stat(root); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []Session{}, nil
		}
		return nil, fmt.Errorf("stat root: %w", err)
	}

	out := []Session{}
	for _, sub := range buckets {
		dir := filepath.Join(root, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", dir, err)
		}
		// One source per id within a bucket: a .db shadows a .pb of the same
		// id whatever order the directory lists them in.
		byID := map[string]int{}
		for _, e := range entries {
			name := e.Name()
			var suffix string
			if strings.HasSuffix(name, ".pb") {
				suffix = ".pb"
			} else if strings.HasSuffix(name, ".db") {
				suffix = ".db"
			}
			if e.IsDir() || suffix == "" {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			id := strings.TrimSuffix(name, suffix)
			mtime := info.ModTime()
			var internalID string
			if suffix == ".db" {
				// SQLite in WAL mode writes updates to the -wal sidecar file first,
				// which may have a newer modification time than the main .db file
				// before checkpointing. An empty WAL means everything has been
				// checkpointed — its mtime then only reflects the daemon opening
				// the file, so only honor it when it has content.
				if walInfo, err := os.Stat(filepath.Join(dir, name+"-wal")); err == nil {
					if walInfo.Size() > 0 && walInfo.ModTime().After(mtime) {
						mtime = walInfo.ModTime()
					}
				}
				// Also check rollback journal just in case
				if journalInfo, err := os.Stat(filepath.Join(dir, name+"-journal")); err == nil {
					if journalInfo.Size() > 0 && journalInfo.ModTime().After(mtime) {
						mtime = journalInfo.ModTime()
					}
				}
				if idFromDB, err := ReadSQLiteCascadeID(filepath.Join(dir, name)); err == nil && idFromDB != "" && idFromDB != id {
					internalID = idFromDB
				}
			}
			session := Session{
				CascadeID:         id,
				InternalCascadeID: internalID,
				PBPath:            filepath.Join(dir, name),
				SidecarPath:       filepath.Join(dir, id+".trajectory.json"),
				Bucket:            sub,
				ModTime:           mtime,
			}
			if i, ok := byID[id]; ok {
				if suffix == ".db" {
					out[i] = session
				}
				continue
			}
			byID[id] = len(out)
			out = append(out, session)
		}
	}
	slices.SortFunc(out, func(a, b Session) int {
		return b.ModTime.Compare(a.ModTime)
	})
	return out, nil
}

// FindByID returns the Session whose CascadeID matches id, searching both
// subdirs under root. Returns ok=false if no such session exists.
//
// A filename id is authoritative, so every session is checked for an exact
// CascadeID match before any internal id is considered. Checking both in one
// newest-first pass let an imported database that happens to carry another
// session's internal cascade_id answer for a filename id that names a
// different transcript.
func FindByID(root, id string) (Session, bool, error) {
	id = strings.TrimSuffix(id, ".pb")
	id = strings.TrimSuffix(id, ".db")
	sessions, err := ListSessions(root)
	if err != nil {
		return Session{}, false, err
	}
	for _, s := range sessions {
		if s.CascadeID == id {
			return s, true, nil
		}
	}
	for _, s := range sessions {
		if s.InternalCascadeID != "" && s.InternalCascadeID == id {
			return s, true, nil
		}
	}
	return Session{}, false, nil
}

// Connection is a daemon endpoint together with the credential found for it.
// Token is empty when the daemon needs none or none could be discovered.
type Connection struct {
	BaseURL string
	Token   string
}

// DiscoverConnection finds the daemon serving root and the token that
// authenticates it in a single scan. CLI credentials live in short-lived tool
// processes, so a second scan can miss a process that has just exited: that
// would keep the endpoint and silently lose its valid token, leaving one-shot
// commands unauthenticated and watchers blocked until another tool appears.
// Callers that already hold an endpoint use DiscoverCSRFTokenForURL instead.
func DiscoverConnection(root string) (Connection, error) {
	if DetectSurface(root) == SurfaceIDE {
		logsDir, err := IDELogsDir()
		if err != nil {
			return Connection{}, err
		}
		base, err := discoverIDEDaemonURL(logsDir)
		if err != nil {
			return Connection{}, err
		}
		return Connection{BaseURL: base, Token: DiscoverCSRFTokenForURL(root, base)}, nil
	}
	if base, token := discoverCLIEndpoint(root, ""); base != "" {
		if v := strings.TrimSpace(os.Getenv("ANTIGRAVITY_CSRF_TOKEN")); v != "" {
			token = v
		}
		return Connection{BaseURL: base, Token: token}, nil
	}
	base, err := daemonURLFromLog(filepath.Join(root, "cli.log"))
	if err != nil {
		if !fileExists(filepath.Join(root, "cli.log")) {
			if logs := acpServerLogs(root); len(logs) > 0 {
				base, err = daemonURLFromLogs(logs)
			}
		}
		if err != nil {
			return Connection{}, err
		}
	}
	return Connection{BaseURL: base, Token: DiscoverCSRFTokenForURL(root, base)}, nil
}

// acpServerLogs lists root's ACP server logs newest first. `antigravity acp`
// writes no cli.log: each server start opens log/acp-server-<time>-<pid>.log,
// so a root that T3 Code restarted holds several. Names order by start time;
// a later modification time wins, and the name breaks ties.
func acpServerLogs(root string) []string {
	entries, err := os.ReadDir(filepath.Join(root, "log"))
	if err != nil {
		return nil
	}
	type logFile struct {
		path string
		mod  time.Time
	}
	var logs []logFile
	for _, e := range entries {
		path := filepath.Join(root, "log", e.Name())
		if e.IsDir() || !isACPServerLog(root, path) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		logs = append(logs, logFile{path, info.ModTime()})
	}
	slices.SortFunc(logs, func(a, b logFile) int {
		if c := b.mod.Compare(a.mod); c != 0 {
			return c
		}
		return strings.Compare(b.path, a.path)
	})
	paths := make([]string, len(logs))
	for i, l := range logs {
		paths[i] = l.path
	}
	return paths
}

// isACPServerLog reports whether path is an ACP server log directly in
// root/log.
func isACPServerLog(root, path string) bool {
	name := filepath.Base(path)
	return filepath.Dir(path) == filepath.Join(root, "log") && strings.HasPrefix(name, "acp-server-") && strings.HasSuffix(name, ".log")
}

// daemonURLFromLogs returns the first reachable daemon URL from logs, tried in
// order. The error names the newest log when none answers.
func daemonURLFromLogs(logs []string) (string, error) {
	var first error
	for _, log := range logs {
		base, err := daemonURLFromLog(log)
		if err == nil {
			return base, nil
		}
		if first == nil {
			first = err
		}
	}
	return "", first
}

// DiscoverDaemonURL attempts to find the HTTP URL of the language server
// serving root. For a CLI root it prefers the live Linux process and falls
// back to cli.log, or, for a T3 Code ACP root without one, to its
// log/acp-server-*.log files newest first. For an IDE root it parses the
// IDE's language_server.log under IDELogsDir — the IDE never writes a
// cli.log. Returns the URL (e.g. "http://127.0.0.1:36871") or an error if not
// found or unreachable.
func DiscoverDaemonURL(root string) (string, error) {
	conn, err := DiscoverConnection(root)
	return conn.BaseURL, err
}

// daemonURLFromLog parses a language-server glog file for the most recent
// HTTP port line and dial-verifies that the port is still listening.
func daemonURLFromLog(logPath string) (string, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", filepath.Base(logPath), err)
	}

	port, ok := parseDaemonPort(string(data))
	if !ok {
		return "", fmt.Errorf("no active HTTP daemon port found in %s", filepath.Base(logPath))
	}

	url := "http://127.0.0.1:" + port
	// Verification check: verify the port is active and responding
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 150*time.Millisecond)
	if err != nil {
		return "", fmt.Errorf("discovered daemon port %s is unreachable: %w", port, err)
	}
	conn.Close()

	return url, nil
}

// parseDaemonPort scans language-server log output for the newest
// "listening on random port at <port> for HTTP" line and returns that port.
// The server logs a companion "listening on random port at <port> for HTTPS
// (gRPC)" line alongside it; that line also contains " for HTTP" as a
// substring, so the match explicitly rejects the HTTPS variant — the gRPC
// port does not speak the JSON dialect this tool uses.
func parseDaemonPort(data string) (string, bool) {
	const marker = "listening on random port at "
	const suffix = " for HTTP"
	lines := strings.Split(data, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		idx := strings.Index(lines[i], marker)
		if idx == -1 {
			continue
		}
		rest := lines[i][idx+len(marker):]
		endIdx := strings.Index(rest, suffix)
		if endIdx == -1 {
			continue
		}
		// " for HTTPS (gRPC)" also matches the suffix; skip it.
		if after := rest[endIdx+len(suffix):]; strings.HasPrefix(after, "S") {
			continue
		}
		if port := strings.TrimSpace(rest[:endIdx]); port != "" {
			return port, true
		}
	}
	return "", false
}
