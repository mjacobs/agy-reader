package watcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/session"
	"github.com/mjacobs/agy-reader/internal/vcache"
)

// RefusalMemo remembers source-proven unknown refusals and short-lived RPC
// mismatch backoffs, keyed by source path. An unsupported source is held until
// its evidence changes; a stale daemon response is retried after a deadline.
type RefusalMemo map[string]HeldRefusal

// refusalNow is the clock Holds reads; tests move it forward.
var refusalNow = time.Now

const mismatchRetryDelay = 30 * time.Second

type HeldRefusal struct {
	RetryAt       time.Time // transient RPC mismatch; never an indefinite source refusal
	Endpoint      string
	Profile       string
	SourceModTime time.Time
	SourceDigest  string // empty when the source snapshot could not be read
	SidecarState  SidecarState
	// SourceFiles is the source metadata at a transient mismatch, or when
	// its digest last matched SourceDigest at DigestMatchedAt.
	SourceFiles     vcache.Files
	DigestMatchedAt time.Time
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
	digest, _ := heldSourceDigest(path)
	return digest
}

func heldSourceDigest(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snapshot, err := completeness.ReadSnapshot(ctx, path)
	if err != nil {
		return "", err
	}
	return snapshot.Digest, nil
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
//
// Reading the digest decodes the whole source, which for a large session costs
// more than a second of CPU. So once the digest has matched, unchanged source
// file metadata stands in for it until the match is older than
// vcache.ReuseLimit, the reuse the verification cache allows: four to five
// minutes for a source that changed recently, growing with the time it had
// been quiet up to six hours. Legacy authentication/schema refusals can reuse stable file metadata even
// without a decoded digest; changes and expiry force another native check.
// Two kinds of hold are read on every tick instead: an unreadable SQLite source
// has no digest to trust, and a source whose WAL or
// rollback journal has content may be mid-commit: SQLite writes a commit's WAL
// frames before readers can see them, so a read in that gap sees the previous
// content under the new metadata.
func (m RefusalMemo) Holds(s discovery.Session, endpoint string) bool {
	return m.holds(s, endpoint, heldSourceDigest)
}

func (m RefusalMemo) holds(s discovery.Session, endpoint string, readDigest func(string) (string, error)) bool {
	held, ok := m[s.PBPath]
	if !ok {
		return false
	}
	if held.Endpoint != endpoint ||
		held.Profile != completeness.ProfileForPath(s.PBPath) ||
		!held.SourceModTime.Equal(s.ModTime) ||
		!held.SidecarState.Equal(StatSidecar(s.SidecarPath)) {
		delete(m, s.PBPath)
		return false
	}
	if !held.RetryAt.IsZero() {
		files, ok := vcache.SourceFingerprint(s.PBPath)
		if !ok || files != held.SourceFiles || !refusalNow().Before(held.RetryAt) {
			delete(m, s.PBPath)
			return false
		}
		return true
	}
	before, stable := vcache.SourceFingerprint(s.PBPath)
	stable = stable && (held.SourceDigest != "" || filepath.Ext(s.PBPath) == ".pb") && before.WAL.Size == 0 && before.Journal.Size == 0
	now := refusalNow()
	if stable && !held.DigestMatchedAt.IsZero() && held.SourceFiles == before {
		if age := now.Sub(held.DigestMatchedAt); age >= 0 && age < vcache.ReuseLimit(s.PBPath, held.SourceFiles, held.DigestMatchedAt) {
			return true
		}
	}
	digest, err := readDigest(s.PBPath)
	// Transient failures can recover without any file metadata changing.
	// Re-read them each tick without repeating the refusal log. Only
	// format/schema refusals may reuse an empty digest from metadata.
	if err != nil && !completeness.IsUnsupported(err) {
		stable = false
	}
	if held.SourceDigest != digest {
		delete(m, s.PBPath)
		return false
	}
	// Remember the match only if no write landed during the read.
	held.SourceFiles, held.DigestMatchedAt = vcache.Files{}, time.Time{}
	if after, ok := vcache.SourceFingerprint(s.PBPath); stable && ok && after == before {
		held.SourceFiles, held.DigestMatchedAt = after, now
	}
	m[s.PBPath] = held
	return true
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
		Profile:       completeness.ProfileForPath(s.PBPath),
		SourceModTime: s.ModTime,
		SourceDigest:  digest,
		SidecarState:  StatSidecar(s.SidecarPath),
	}
}

// HoldMismatch bounds retries when the daemon serves stale in-memory content.
// File, sidecar, endpoint and profile changes release it immediately. Even with
// none of those changed, the same endpoint is retried after a short deadline.
func (m RefusalMemo) HoldMismatch(s discovery.Session, endpoint string) {
	if m == nil {
		return
	}
	files, ok := vcache.SourceFingerprint(s.PBPath)
	if !ok {
		return
	}
	m[s.PBPath] = HeldRefusal{Endpoint: endpoint, Profile: completeness.ProfileForPath(s.PBPath), SourceModTime: s.ModTime, SidecarState: StatSidecar(s.SidecarPath), SourceFiles: files, RetryAt: refusalNow().Add(mismatchRetryDelay)}
}

func mismatchedResponse(err error) bool {
	var contentErr *session.CompletenessError
	return errors.As(err, &contentErr) && contentErr.Result.State == completeness.Incomplete
}
