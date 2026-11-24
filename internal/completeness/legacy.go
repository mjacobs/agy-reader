package completeness

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LegacyProfile separates authenticated monolithic trajectories from SQLite
// snapshots while sharing their strict schema and presentation policy.
const LegacyProfile = "legacy-aes256gcm-v1-agy-1.4.4-337dcde66844/" + Profile

// legacyKey is the fixed format key distributed in agy, not a user credential.
// Provenance and the independently captured fixture are documented in
// docs/completeness.md. Unknown keys/layouts fail authentication, never fall back.
//
//go:embed legacy_key.bin
var legacyKey []byte

func ProfileForPath(path string) string {
	if filepath.Ext(path) == ".pb" {
		return LegacyProfile
	}
	return Profile
}

func readLegacySnapshot(ctx context.Context, path string) (*Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// A migration can create SQLite after discovery selected this legacy file.
	// Refuse the obsolete source rather than overwrite the newer format's sidecar.
	if _, err := os.Stat(strings.TrimSuffix(path, ".pb") + ".db"); err == nil {
		return nil, errors.New("legacy source superseded by SQLite")
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return readLegacyFile(ctx, path, false)
}

func readLegacyFile(ctx context.Context, path string, implicit bool) (*Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Reject special files before opening (a FIFO could otherwise block forever).
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, refusal("legacy source is not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, refusal("legacy source is not a regular file")
	}
	const overhead = 12 + 16
	if info.Size() > maxBytes+overhead {
		return nil, refusal("legacy source size limit")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+overhead+1))
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if len(data) > maxBytes+overhead {
		return nil, refusal("legacy source size limit")
	}
	block, err := aes.NewCipher(legacyKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < aead.NonceSize()+aead.Overhead() {
		return nil, refusal("truncated legacy ciphertext")
	}
	plain, err := aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], nil)
	if err != nil {
		return nil, refusal("legacy source authentication failed")
	}
	profile := LegacyProfile
	var scope json.RawMessage
	if implicit {
		plain, scope, err = implicitTrajectory(plain)
		if err != nil {
			return nil, err
		}
		profile = ImplicitProfile
	}
	m, err := newMessage("gemini_coder.Trajectory")
	if err != nil {
		return nil, refuse(err)
	}
	if err := unmarshalRecord(plain, m); err != nil {
		return nil, refusal("invalid persisted protobuf")
	}
	s, err := finishSnapshotProfile(m, profile)
	if err != nil {
		msg := m.ProtoReflect()
		id := msg.Get(msg.Descriptor().Fields().ByName("cascade_id")).String()
		if id != "" && isShapeError(err) {
			return nil, &UnknownShape{refuse(err), id, msg.Get(msg.Descriptor().Fields().ByName("steps")).List().Len()}
		}
		return nil, refuse(err)
	}
	// Bind evidence to the exact authenticated file, including its nonce and tag.
	s.implicitScope = scope
	sum := sha256.Sum256(append([]byte(profile+"\x00"), data...))
	s.Digest = hex.EncodeToString(sum[:])
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return s, nil
}
