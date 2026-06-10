// Package discovery enumerates Antigravity-CLI session .pb files on disk.
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

// Subdirs that hold encrypted .pb session files inside the root.
var subdirs = []string{"conversations", "implicit"}

// Session describes one discovered .pb or .db session file.
type Session struct {
	// CascadeID is the bare UUID (filename minus suffix).
	CascadeID string
	// PBPath is the absolute path to the encrypted .pb or SQLite .db file.
	PBPath string
	// SidecarPath is where the decrypted sidecar lives (or would live)
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

// ListSessions walks the conversations/ and implicit/ subdirs under root
// and returns one Session per .pb file, sorted by ModTime descending.
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
			}
			out = append(out, Session{
				CascadeID:   id,
				PBPath:      filepath.Join(dir, name),
				SidecarPath: filepath.Join(dir, id+".trajectory.json"),
				Bucket:      sub,
				ModTime:     mtime,
			})
		}
	}
	slices.SortFunc(out, func(a, b Session) int {
		return b.ModTime.Compare(a.ModTime)
	})
	return out, nil
}

// FindByID returns the Session whose CascadeID matches id, searching both
// subdirs under root. Returns ok=false if no such session exists.
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
	return Session{}, false, nil
}

// DiscoverDaemonURL attempts to find the active language server's HTTP URL
// by parsing the cli.log file inside root. Returns the URL (e.g. "http://127.0.0.1:36871")
// or an error if not found or unreachable.
func DiscoverDaemonURL(root string) (string, error) {
	logPath := filepath.Join(root, "cli.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("read cli.log: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var foundPort string
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if strings.Contains(line, "listening on random port at ") && strings.Contains(line, " for HTTP") {
			idx := strings.Index(line, "listening on random port at ")
			if idx == -1 {
				continue
			}
			rest := line[idx+len("listening on random port at "):]
			endIdx := strings.Index(rest, " for HTTP")
			if endIdx == -1 {
				continue
			}
			port := strings.TrimSpace(rest[:endIdx])
			if port != "" {
				foundPort = port
				break
			}
		}
	}

	if foundPort == "" {
		return "", errors.New("no active HTTP daemon port found in cli.log")
	}

	url := "http://127.0.0.1:" + foundPort
	// Verification check: verify the port is active and responding
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+foundPort, 150*time.Millisecond)
	if err != nil {
		return "", fmt.Errorf("discovered daemon port %s is unreachable: %w", foundPort, err)
	}
	conn.Close()

	return url, nil
}
