// Package discovery enumerates Antigravity-CLI session .pb files on disk.
package discovery

import (
	"errors"
	"fmt"
	"io/fs"
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

// Session describes one discovered .pb file.
type Session struct {
	// CascadeID is the bare UUID (filename minus .pb).
	CascadeID string
	// PBPath is the absolute path to the encrypted .pb file.
	PBPath string
	// SidecarPath is where the decrypted sidecar lives (or would live)
	// — same dir, suffix replaced with .trajectory.json.
	SidecarPath string
	// Bucket is "conversations" or "implicit".
	Bucket string
	// ModTime is the .pb file's last-modified time.
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
	if _, err := os.Stat(root); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []Session{}, nil
		}
		return nil, fmt.Errorf("stat root: %w", err)
	}

	out := []Session{}
	for _, sub := range subdirs {
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
			if e.IsDir() || !strings.HasSuffix(name, ".pb") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			id := strings.TrimSuffix(name, ".pb")
			out = append(out, Session{
				CascadeID:   id,
				PBPath:      filepath.Join(dir, name),
				SidecarPath: filepath.Join(dir, id+".trajectory.json"),
				Bucket:      sub,
				ModTime:     info.ModTime(),
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
