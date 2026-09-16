package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultIDERootSubpath is the user-relative location of the Antigravity IDE
// session tree. The IDE stores conversations in the same shape as the CLI
// (conversations/<uuid>.db, identical SQLite schema) but is served by its own
// language-server daemon, which logs under IDELogsDir rather than writing a
// cli.log inside the root.
const DefaultIDERootSubpath = ".gemini/antigravity"

// ideStateFile is an IDE-only marker file inside the session root. The CLI
// root writes jetski_state.pbtxt instead, so this file identifies an IDE
// tree even under a nonstandard path.
const ideStateFile = "antigravity_state.pbtxt"

// ideServerLogName is the IDE language server's glog file (port lines) and
// ideMainLogName is the IDE process log that records the exact command the
// language server was spawned with (including --csrf_token). Both live in
// IDELogsDir.
const (
	ideServerLogName = "language_server.log"
	ideMainLogName   = "main.log"
)

// Surface identifies which Antigravity product owns a session root: the CLI
// (`agy`) or the IDE. Both run the same Exafunction language-server daemon
// with the same RPCs, but they publish connection details differently.
type Surface string

const (
	SurfaceCLI Surface = "cli"
	SurfaceIDE Surface = "ide"
)

// DetectSurface reports whether root belongs to the Antigravity CLI or the
// IDE. A cli.log inside the root is authoritative for the CLI — agy writes
// it on every run and the IDE never does. Otherwise the root is the IDE's
// when it carries the IDE-only antigravity_state.pbtxt marker or is the
// default IDE location. Everything else defaults to the CLI surface,
// preserving the original behavior for custom CLI roots.
func DetectSurface(root string) Surface {
	if fileExists(filepath.Join(root, "cli.log")) {
		return SurfaceCLI
	}
	if fileExists(filepath.Join(root, ideStateFile)) {
		return SurfaceIDE
	}
	if home, err := os.UserHomeDir(); err == nil {
		if filepath.Clean(root) == filepath.Join(home, DefaultIDERootSubpath) {
			return SurfaceIDE
		}
	}
	return SurfaceCLI
}

// IDELogsDir returns the Antigravity IDE's log directory for this OS,
// resolved via os.UserConfigDir: ~/.config/Antigravity/logs on Linux,
// ~/Library/Application Support/Antigravity/logs on macOS, and
// %AppData%\Antigravity\logs on Windows. Verified on Linux; the other
// locations follow the IDE's Electron user-data convention.
func IDELogsDir() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(cfg, "Antigravity", "logs"), nil
}

// discoverIDEDaemonURL finds the IDE language server's HTTP URL by parsing
// the port lines it logs to language_server.log under logsDir, using the
// same line format and dial verification as CLI cli.log discovery.
func discoverIDEDaemonURL(logsDir string) (string, error) {
	return daemonURLFromLog(filepath.Join(logsDir, ideServerLogName))
}

// DiscoverCSRFToken returns credentials for the root's discovered daemon.
// The explicit override wins; older CLI daemons may need no token.
func DiscoverCSRFToken(root string) string {
	base, _ := DiscoverDaemonURL(root)
	return DiscoverCSRFTokenForURL(root, base)
}

// DiscoverCSRFTokenForURL binds CLI credentials to the selected endpoint,
// including when the URL was pinned explicitly. Tokens are never logged.
func DiscoverCSRFTokenForURL(root, baseURL string) string {
	if v := strings.TrimSpace(os.Getenv("ANTIGRAVITY_CSRF_TOKEN")); v != "" {
		return v
	}
	if DetectSurface(root) != SurfaceIDE {
		if baseURL == "" {
			return ""
		}
		_, token := discoverCLIEndpoint(root, baseURL)
		return token
	}
	logsDir, err := IDELogsDir()
	if err != nil {
		return ""
	}
	tok, err := CSRFTokenFromLog(filepath.Join(logsDir, ideMainLogName))
	if err != nil {
		return ""
	}
	return tok
}

// CSRFTokenFromLog extracts the newest --csrf_token value from an Antigravity
// IDE main.log. Returns "" with a nil error when the log records no token;
// the error reports an unreadable log file.
func CSRFTokenFromLog(logPath string) (string, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", filepath.Base(logPath), err)
	}
	return parseCSRFToken(string(data)), nil
}

// parseCSRFToken scans log lines newest-first for a --csrf_token flag and
// returns its value. Both "--csrf_token <value>" (the form the IDE logs) and
// "--csrf_token=<value>" are accepted; longer flags sharing the prefix
// (e.g. a hypothetical --csrf_token_file) and a flag with a missing value
// are ignored.
func parseCSRFToken(data string) string {
	lines := strings.Split(data, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if tok := csrfTokenInLine(lines[i]); tok != "" {
			return tok
		}
	}
	return ""
}

// csrfTokenInLine finds the first --csrf_token value in one log line.
func csrfTokenInLine(line string) string {
	const flagMarker = "--csrf_token"
	for {
		idx := strings.Index(line, flagMarker)
		if idx == -1 {
			return ""
		}
		rest := line[idx+len(flagMarker):]
		line = rest // continue after this occurrence if it doesn't parse
		switch {
		case strings.HasPrefix(rest, "="):
			rest = rest[1:]
		case strings.HasPrefix(rest, " "):
			rest = strings.TrimLeft(rest, " ")
		default:
			continue // different flag sharing the prefix, or value missing
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "-") {
			continue // "--csrf_token --next_flag": no value was passed
		}
		return fields[0]
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
