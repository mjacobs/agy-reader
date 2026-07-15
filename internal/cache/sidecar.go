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
	"strings"

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
	data, err := preserveReaderMetadata(sidecarPath, data)
	if err != nil {
		return err
	}
	_, err = writeAtomic(sidecarPath, data)
	return err
}

// preserveReaderMetadata carries the reader-owned namespace forward when a
// fresh daemon payload replaces an existing sidecar. The daemon never owns
// agyReader, so dropping it during refresh would lose lineage (and any future
// reader fields) before the reconciliation pass has a chance to inspect it.
func preserveReaderMetadata(sidecarPath string, fresh []byte) ([]byte, error) {
	current, err := os.ReadFile(sidecarPath)
	if errors.Is(err, fs.ErrNotExist) {
		return fresh, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read existing sidecar metadata: %w", err)
	}
	var oldTop map[string]json.RawMessage
	if err := json.Unmarshal(current, &oldTop); err != nil {
		return nil, fmt.Errorf("decode existing sidecar metadata: %w", err)
	}
	reader, ok := oldTop["agyReader"]
	if !ok {
		return fresh, nil
	}
	var freshTop map[string]json.RawMessage
	if err := json.Unmarshal(fresh, &freshTop); err != nil {
		return nil, fmt.Errorf("decode fresh sidecar payload: %w", err)
	}
	if freshTop == nil {
		return nil, errors.New("cache: fresh trajectory is not an object")
	}
	freshTop["agyReader"] = reader
	merged, err := json.MarshalIndent(freshTop, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode sidecar with reader metadata: %w", err)
	}
	return append(merged, '\n'), nil
}

// StampParentCascadeID inserts or updates agyReader.parentCascadeId without
// decoding any daemon-owned value. Existing sibling fields inside agyReader
// are retained as raw JSON too. It returns changed=false without touching the
// file when the requested value is already present.
func StampParentCascadeID(sidecarPath, parentCascadeID string) (changed bool, err error) {
	if !daemon.IsCascadeID(parentCascadeID) {
		return false, fmt.Errorf("cache: invalid parent cascade id %q", parentCascadeID)
	}
	parentCascadeID = strings.ToLower(parentCascadeID)
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
		if json.Unmarshal(raw, &existing) == nil && strings.EqualFold(existing, parentCascadeID) {
			return writeAtomic(sidecarPath, data)
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
// Sidecars contain decrypted conversation data and are always owner-only.
// Equal bytes are a true content no-op; an insecure existing mode is still
// tightened without replacing the file or changing its mtime.
func writeAtomic(path string, data []byte) (changed bool, err error) {
	if current, readErr := os.ReadFile(path); readErr == nil {
		if bytes.Equal(current, data) {
			info, statErr := os.Stat(path)
			if statErr != nil {
				return false, fmt.Errorf("stat existing sidecar: %w", statErr)
			}
			if info.Mode().Perm() == 0o600 {
				return false, nil
			}
			if err := os.Chmod(path, 0o600); err != nil {
				return false, fmt.Errorf("restrict existing sidecar: %w", err)
			}
			return true, nil
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
	if err := tmp.Chmod(0o600); err != nil {
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
