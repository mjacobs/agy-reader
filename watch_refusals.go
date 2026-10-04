package main

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/discovery"
)

// refusalMemo remembers SQLite sources the watcher refused as unknown for a
// reason the source itself proves, keyed by source path, so an unchanged source
// is not fetched again on every tick. Each fetch makes the daemon load the
// trajectory, and such a refusal is fixed by the source content and the
// completeness profile: retrying with neither changed repeats it, whatever the
// daemon answers.
//
// A held source is retried as soon as anything that could change the outcome
// changes: its source mtime, its source digest, its sidecar, the daemon
// endpoint, or the profile. Refusals that come from the daemon's responses are
// never held, and neither are incomplete results: both can heal at the same
// endpoint without any local change (agy-reader#29sp).
//
// A nil memo holds nothing.
type refusalMemo map[string]heldRefusal

type heldRefusal struct {
	endpoint      string
	profile       string
	sourceModTime time.Time
	sourceDigest  string // empty when the source snapshot could not be read
	sidecarState  sidecarState
}

type sidecarState struct {
	exists  bool
	modTime time.Time
	size    int64
}

func (a sidecarState) equal(b sidecarState) bool {
	return a.exists == b.exists && a.size == b.size && a.modTime.Equal(b.modTime)
}

func statSidecar(path string) sidecarState {
	info, err := os.Stat(path)
	if err != nil {
		return sidecarState{}
	}
	return sidecarState{exists: true, modTime: info.ModTime(), size: info.Size()}
}

// heldSourceDigest reads the current source snapshot digest, or "" when the
// source cannot be read. A read that later succeeds therefore releases a hold
// recorded for an unreadable source.
func heldSourceDigest(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snapshot, err := completeness.ReadSnapshot(ctx, path)
	if err != nil {
		return ""
	}
	return snapshot.Digest
}

// holdableRefusal reports whether err is an unknown refusal that the source and
// profile alone prove, so an unchanged retry would repeat it whatever the
// daemon answered, and the source digest it was refused against.
//
// Held: a source that cannot be read or lies outside the profile, refused
// before any RPC (with an empty digest, so a later successful read releases
// it); an empty persisted identity; and a projection the profile does not
// support, which is checked only after the full RPC representation equals the
// source and reads nothing else. Not held: refusals from validating the daemon's
// responses, such as malformed JSON or an RPC enum name outside the profile; a
// failed re-read after the responses were accepted; and incomplete results.
func holdableRefusal(err error) (digest string, ok bool) {
	var contentErr *completenessError
	if !errors.As(err, &contentErr) || contentErr.result.State != completeness.Unknown {
		return "", false
	}
	if contentErr.beforeRPC {
		return "", true
	}
	return contentErr.sourceDigest, contentErr.sourceDigest != ""
}

// prune forgets sources that are no longer listed under the root.
func (m refusalMemo) prune(sessions []discovery.Session) {
	if len(m) == 0 {
		return
	}
	present := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		present[s.PBPath] = true
	}
	for path := range m {
		if !present[path] {
			delete(m, path)
		}
	}
}

// holds reports whether s is still held at endpoint. A source whose hold no
// longer applies is forgotten, so the caller fetches it again.
func (m refusalMemo) holds(s discovery.Session, endpoint string) bool {
	held, ok := m[s.PBPath]
	if !ok {
		return false
	}
	if held.endpoint == endpoint &&
		held.profile == completeness.Profile &&
		held.sourceModTime.Equal(s.ModTime) &&
		held.sidecarState.equal(statSidecar(s.SidecarPath)) &&
		held.sourceDigest == heldSourceDigest(s.PBPath) {
		return true
	}
	delete(m, s.PBPath)
	return false
}

// hold records a refusal of s observed at endpoint. sourceModTime is the mtime
// listed before the fetch, so a write that lands during the fetch releases the
// hold on the next tick.
func (m refusalMemo) hold(s discovery.Session, endpoint, digest string) {
	if m == nil {
		return
	}
	m[s.PBPath] = heldRefusal{
		endpoint:      endpoint,
		profile:       completeness.Profile,
		sourceModTime: s.ModTime,
		sourceDigest:  digest,
		sidecarState:  statSidecar(s.SidecarPath),
	}
}
