package watcher

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/session"
)

// RefusalMemo remembers SQLite sources the watcher refused as unknown for a
// reason the source itself proves, keyed by source path, so an unchanged source
// is not fetched again on every tick. Each fetch makes the daemon load the
// trajectory, and such a refusal is fixed by the source content and the
// completeness profile: retrying with neither changed repeats it, whatever the
// daemon answers.
type RefusalMemo map[string]HeldRefusal

type HeldRefusal struct {
	Endpoint      string
	Profile       string
	SourceModTime time.Time
	SourceDigest  string // empty when the source snapshot could not be read
	SidecarState  SidecarState
}

type SidecarState struct {
	Exists  bool
	ModTime time.Time
	Size    int64
}

func (a SidecarState) Equal(b SidecarState) bool {
	return a.Exists == b.Exists && a.Size == b.Size && a.ModTime.Equal(b.ModTime)
}

func StatSidecar(path string) SidecarState {
	info, err := os.Stat(path)
	if err != nil {
		return SidecarState{}
	}
	return SidecarState{Exists: true, ModTime: info.ModTime(), Size: info.Size()}
}

// HeldSourceDigest reads the current source snapshot digest, or "" when the
// source cannot be read. A read that later succeeds therefore releases a hold
// recorded for an unreadable source.
func HeldSourceDigest(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snapshot, err := completeness.ReadSnapshot(ctx, path)
	if err != nil {
		return ""
	}
	return snapshot.Digest
}

// HoldableRefusal reports whether err is an unknown refusal that the source and
// profile alone prove, so an unchanged retry would repeat it whatever the
// daemon answered, and the source digest it was refused against.
func HoldableRefusal(err error) (digest string, ok bool) {
	var contentErr *session.CompletenessError
	if !errors.As(err, &contentErr) || contentErr.Result.State != completeness.Unknown {
		return "", false
	}
	if contentErr.BeforeRPC {
		return "", true
	}
	return contentErr.SourceDigest, contentErr.SourceDigest != ""
}

// Prune forgets sources that are no longer listed under the root.
func (m RefusalMemo) Prune(sessions []discovery.Session) {
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

// Holds reports whether s is still held at endpoint. A source whose hold no
// longer applies is forgotten, so the caller fetches it again.
func (m RefusalMemo) Holds(s discovery.Session, endpoint string) bool {
	held, ok := m[s.PBPath]
	if !ok {
		return false
	}
	if held.Endpoint == endpoint &&
		held.Profile == completeness.Profile &&
		held.SourceModTime.Equal(s.ModTime) &&
		held.SidecarState.Equal(StatSidecar(s.SidecarPath)) &&
		held.SourceDigest == HeldSourceDigest(s.PBPath) {
		return true
	}
	delete(m, s.PBPath)
	return false
}

// Hold records a refusal of s observed at endpoint. sourceModTime is the mtime
// listed before the fetch, so a write that lands during the fetch releases the
// hold on the next tick.
func (m RefusalMemo) Hold(s discovery.Session, endpoint, digest string) {
	if m == nil {
		return
	}
	m[s.PBPath] = HeldRefusal{
		Endpoint:      endpoint,
		Profile:       completeness.Profile,
		SourceModTime: s.ModTime,
		SourceDigest:  digest,
		SidecarState:  StatSidecar(s.SidecarPath),
	}
}
