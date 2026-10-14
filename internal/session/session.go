package session

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
)

type CompletenessError struct {
	Result    completeness.Result
	BeforeRPC bool
	// SourceDigest is the digest of the source snapshot, set only when that
	// snapshot and the profile alone prove the refusal: an empty persisted
	// identity or a projection the profile does not support. Retrying the same
	// source would be refused whatever the daemon answered. A refusal that
	// comes from the daemon's responses leaves it empty, so watch retries it.
	SourceDigest string
}

func (e *CompletenessError) Error() string {
	return fmt.Sprintf("completeness %s: %s", e.Result.State, e.Result.Reason)
}

func Unverified(state completeness.State, reason string) error {
	return &CompletenessError{Result: completeness.Result{State: state, Reason: reason}}
}

// NewDaemonClient builds a client for a discovered daemon connection. The
// token found alongside the endpoint is used as-is; only a connection that
// carries none (a pinned URL, or a rediscovery that found no credential)
// falls back to a fresh token scan.
func NewDaemonClient(root string, conn discovery.Connection) *daemon.Client {
	c := daemon.NewClient(conn.BaseURL)
	c.CSRFToken = conn.Token
	if c.CSRFToken == "" {
		c.CSRFToken = discovery.DiscoverCSRFTokenForURL(root, conn.BaseURL)
	}
	return c
}

// Fetch uses one authenticated endpoint and persisted identity for both
// representations. A second source snapshot rejects same-step writes in flight.
func Fetch(ctx context.Context, client *daemon.Client, s discovery.Session) (*daemon.Trajectory, error) {
	if !strings.HasSuffix(s.PBPath, ".db") {
		return client.FetchTrajectoryWithFallback(ctx, s.CascadeID, s.InternalCascadeID)
	}
	before, err := completeness.ReadSnapshot(ctx, s.PBPath)
	if err != nil {
		return nil, &CompletenessError{Result: completeness.Result{State: completeness.Unknown, Reason: err.Error()}, BeforeRPC: true}
	}
	if err = client.LoadTrajectory(ctx, s.CascadeID); err != nil {
		return nil, err
	}
	id := before.CascadeID()
	if id == "" {
		return nil, &CompletenessError{Result: completeness.Result{State: completeness.Unknown, Reason: "persisted cascade identity is empty"}, SourceDigest: before.Digest}
	}
	presentation, err := client.GetCascadeTrajectory(ctx, id)
	if err != nil {
		return nil, err
	}
	raw, err := client.GetFullRawTrajectory(ctx, id)
	if err != nil {
		return nil, err
	}
	attestation, result, sourceProven := before.Verify(raw.RawJSON, presentation.RawJSON)
	if result.State != completeness.Verified {
		refusal := &CompletenessError{Result: result}
		if sourceProven {
			refusal.SourceDigest = before.Digest
		}
		return nil, refusal
	}
	after, err := completeness.ReadSnapshot(ctx, s.PBPath)
	if err != nil {
		return nil, Unverified(completeness.Unknown, err.Error())
	}
	if after.Digest != before.Digest {
		return nil, Unverified(completeness.Incomplete, "persisted source changed during fetch")
	}
	presentation.Verification = attestation
	presentation.SourcePath = s.PBPath
	return presentation, nil
}

// FetchRecover tries a finite scan of existing root-owned candidates
// after a content mismatch. It never falls back from an explicitly pinned URL.
func FetchRecover(ctx context.Context, client *daemon.Client, root string, s discovery.Session, pinned bool) (*daemon.Trajectory, error) {
	return FetchCandidates(ctx, client, root, s, pinned, discovery.Connections)
}

func FetchCandidates(ctx context.Context, client *daemon.Client, root string, s discovery.Session, pinned bool, candidates func(string) []discovery.Connection) (*daemon.Trajectory, error) {
	traj, err := Fetch(ctx, client, s)
	var contentErr *CompletenessError
	if err == nil || pinned || !errors.As(err, &contentErr) || contentErr.Result.State != completeness.Incomplete {
		return traj, err
	}
	seen := map[string]bool{client.BaseURL: true}
	attempts := 0
	for _, conn := range candidates(root) {
		if seen[conn.BaseURL] {
			continue
		}
		seen[conn.BaseURL] = true
		if attempts >= 8 || ctx.Err() != nil {
			break
		}
		attempts++
		next := NewDaemonClient(root, conn)
		recovered, nextErr := Fetch(ctx, next, s)
		if nextErr == nil {
			*client = *next
			return recovered, nil
		}
		// Keep the reachable-but-incomplete diagnosis: an alternative endpoint's
		// failure must not turn a content problem into an idle/unreachable state.
	}
	return nil, err
}

// SourceOnlyVerified reports a verified result that relies on declared
// source-only content the RPC never carries. Each output that reports a
// verification result labels or counts it apart from plain verified. It is
// otherwise a verified result: fresh, not stale, and qualifying wherever plain
// verified does.
func SourceOnlyVerified(state, reason string) bool {
	return state == string(completeness.Verified) && reason == completeness.SourceOnlyReason
}

// VerifiedNote is the suffix a sync message adds for a source-only
// verification, so that message never reads as a plain verified write.
func VerifiedNote(t *daemon.Trajectory) string {
	if t == nil || t.Verification == nil || t.Verification.Reason != completeness.SourceOnlyReason {
		return ""
	}
	return "; " + t.Verification.Reason
}

func Completeness(s discovery.Session) completeness.Result {
	res, _ := CompletenessClassified(s)
	return res
}

// SourceClass says what a result that is not verified asks of the user.
type SourceClass int

const (
	// SourceRepairable: verified, or a watch run can verify the source and
	// rewrite its sidecar. A missing sidecar, one without completeness
	// metadata, one minted under an older profile, a malformed one, and
	// changed content all belong here.
	SourceRepairable SourceClass = iota
	// SourceUnsupported: the profile refuses the source or its projection.
	// Watch reads the same source and refuses it again, so only an
	// agy-reader compatibility update can verify it.
	SourceUnsupported
	// SourceUnreadable: the source could not be read now, for example
	// while agy holds an exclusive SQLite lock past the busy timeout, or
	// for lack of permission. That says nothing about support; retry once
	// the lock clears or access is fixed.
	SourceUnreadable
)

// CompletenessClassified also reports which class a result falls in.
// A result is unsupported only on the profile's own refusal
// (completeness.IsUnsupported); an Unknown result alone is not enough.
func CompletenessClassified(s discovery.Session) (res completeness.Result, class SourceClass) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snapshot, err := completeness.ReadSnapshot(ctx, s.PBPath)
	if err != nil {
		res = completeness.Result{State: completeness.Unknown, Reason: err.Error()}
		if completeness.IsUnsupported(err) {
			return res, SourceUnsupported
		}
		return res, SourceUnreadable
	}
	// Checked before the sidecar: watch never writes a sidecar for a source
	// with no persisted identity or a projection the profile refuses
	// (Fetch), so neither a missing nor an old one is watch work.
	if err = snapshot.Refusal(); err != nil {
		return completeness.Result{State: completeness.Unknown, Reason: err.Error()}, SourceUnsupported
	}
	raw, err := os.ReadFile(s.SidecarPath)
	if err != nil {
		return completeness.Result{State: completeness.Unknown, Reason: "sidecar unavailable"}, SourceRepairable
	}
	// With a supported projection, an Unknown here is missing, obsolete or
	// malformed evidence, which the next verified fetch replaces.
	return snapshot.CheckAttestation(raw, completeness.ReadAttestation(raw)), SourceRepairable
}

func FetchDisk(ctx context.Context, conn discovery.Connection, root string, s discovery.Session) (*daemon.Trajectory, error) {
	client := NewDaemonClient(root, conn)
	traj, err := FetchRecover(ctx, client, root, s, strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL")) != "")
	if err == nil {
		return traj, nil
	}
	if cached, cacheErr := cache.Read(s.SidecarPath); cacheErr == nil {
		// A cached fallback remains useful for reading, but never grants write
		// authority or claims the current source is complete.
		fmt.Fprintf(os.Stderr, "warning: daemon fetch not verified (%v); using cached sidecar without refreshing it\n", err)
		return cached, nil
	}
	return nil, err
}

// IsStale reports whether a session's sidecar is missing or older than its source.
func IsStale(s discovery.Session) (bool, string) {
	if strings.HasSuffix(s.PBPath, ".db") {
		result := Completeness(s)
		return result.State != completeness.Verified, string(result.State)
	}

	info, err := os.Stat(s.SidecarPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, "missing"
		}
		return true, "stat-error"
	}
	if s.ModTime.After(info.ModTime()) {
		return true, "modtime-advanced"
	}
	return false, ""
}
