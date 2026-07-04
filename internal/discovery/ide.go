package discovery

import (
	"fmt"
	"os"
	"path/filepath"
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
// with the same RPCs, but they log to different places and only the IDE's
// daemon is launched with CSRF enforcement.
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

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
