// Package cache reads and writes the <uuid>.trajectory.json sidecar files
// that sit next to encrypted .pb files. This is the integration contract
// agentsview consumes — a future agentsview release will detect these
// sidecars and render full-fidelity transcripts.
//
// The daemon-owned sidecar contents are the raw trajectory member of the
// GetCascadeTrajectory response. agy-reader may add one namespaced top-level
// block, "agyReader", without decoding or reshaping the daemon-owned values.
package cache

import (
	"bytes"
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
	var data []byte
	if len(t.RawJSON) > 0 {
		// Validate that the retained payload is a trajectory object before it
		// reaches disk. Its member values remain json.RawMessage bytes; in
		// particular, large JSON numbers never pass through float64.
		var top map[string]json.RawMessage
		if err := json.Unmarshal(t.RawJSON, &top); err != nil {
			return fmt.Errorf("cache: invalid raw trajectory: %w", err)
		}
		if top == nil {
			return errors.New("cache: raw trajectory is not an object")
		}
		data = append([]byte(nil), t.RawJSON...)
		if len(data) == 0 || data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
	} else {
		var err error
		data, err = json.MarshalIndent(t, "", "  ")
		if err != nil {
			return fmt.Errorf("encode sidecar: %w", err)
		}
		data = append(data, '\n')
	}
	_, err := writeAtomic(sidecarPath, data)
	return err
}

// StampParentCascadeID inserts or updates agyReader.parentCascadeId without
// decoding any daemon-owned value. Existing sibling fields inside agyReader
// are retained as raw JSON too. It returns changed=false without touching the
// file when the requested value is already present.
func StampParentCascadeID(sidecarPath, parentCascadeID string) (changed bool, err error) {
	if !daemon.IsCascadeID(parentCascadeID) {
		return false, fmt.Errorf("cache: invalid parent cascade id %q", parentCascadeID)
	}
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		return false, err
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return false, fmt.Errorf("unmarshal sidecar for parent stamp: %w", err)
	}
	if top == nil {
		return false, errors.New("cache: sidecar trajectory is not an object")
	}

	reader := map[string]json.RawMessage{}
	if raw, ok := top["agyReader"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &reader); err != nil || reader == nil {
			if err == nil {
				err = errors.New("value is not an object")
			}
			return false, fmt.Errorf("cache: invalid agyReader block: %w", err)
		}
	}
	if raw, ok := reader["parentCascadeId"]; ok {
		var existing string
		if json.Unmarshal(raw, &existing) == nil && existing == parentCascadeID {
			return false, nil
		}
	}
	encodedParent, _ := json.Marshal(parentCascadeID) // strings cannot fail
	reader["parentCascadeId"] = encodedParent
	encodedReader, err := json.Marshal(reader)
	if err != nil {
		return false, fmt.Errorf("encode agyReader block: %w", err)
	}
	top["agyReader"] = encodedReader
	updated, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode stamped sidecar: %w", err)
	}
	updated = append(updated, '\n')
	return writeAtomic(sidecarPath, updated)
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
	t.RawJSON = append(json.RawMessage(nil), data...)
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

// writeAtomic writes data beside the destination and renames it into place.
// Existing permissions are retained. Equal bytes are a true no-op so repeated
// sync/backfill runs do not churn mtimes or downstream file watchers.
func writeAtomic(path string, data []byte) (changed bool, err error) {
	mode := fs.FileMode(0o600)
	if current, readErr := os.ReadFile(path); readErr == nil {
		if bytes.Equal(current, data) {
			return false, nil
		}
		if info, statErr := os.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		}
	} else if !errors.Is(readErr, fs.ErrNotExist) {
		return false, fmt.Errorf("read existing sidecar: %w", readErr)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".trajectory-*.json.tmp")
	if err != nil {
		return false, fmt.Errorf("create temp sidecar: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	if err := tmp.Chmod(mode); err != nil {
		cleanup()
		return false, fmt.Errorf("chmod temp sidecar: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return false, fmt.Errorf("write temp sidecar: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return false, fmt.Errorf("sync temp sidecar: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return false, fmt.Errorf("close temp sidecar: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return false, fmt.Errorf("rename sidecar: %w", err)
	}
	return true, nil
}
