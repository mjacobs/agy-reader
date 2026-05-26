// Package cache reads and writes the <uuid>.trajectory.json sidecar files
// that sit next to encrypted .pb files. This is the integration contract
// agentsview consumes — a future agentsview release will detect these
// sidecars and render full-fidelity transcripts.
//
// The sidecar contents are the raw GetCascadeTrajectory response under the
// "trajectory" key, persisted as-is. We do not invent a schema.
package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

// Write atomically writes a trajectory to sidecarPath. Parent dir must
// already exist (we don't create directories outside the supplied path).
func Write(sidecarPath string, t *daemon.Trajectory) error {
	if t == nil {
		return errors.New("cache: nil trajectory")
	}
	dir := filepath.Dir(sidecarPath)
	tmp, err := os.CreateTemp(dir, ".trajectory-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp sidecar: %w", err)
	}
	tmpPath := tmp.Name()
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(t); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("encode sidecar: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp sidecar: %w", err)
	}
	if err := os.Rename(tmpPath, sidecarPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename sidecar: %w", err)
	}
	return nil
}

// Read loads a previously-written sidecar. Returns os.ErrNotExist when the
// file is missing so callers can fall back to the daemon.
func Read(sidecarPath string) (*daemon.Trajectory, error) {
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		return nil, err
	}
	var t daemon.Trajectory
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("unmarshal sidecar: %w", err)
	}
	return &t, nil
}

// Exists reports whether a sidecar already exists on disk.
func Exists(sidecarPath string) bool {
	_, err := os.Stat(sidecarPath)
	if err == nil {
		return true
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	return false
}
