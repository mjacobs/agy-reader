package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
)

type completenessError struct {
	result    completeness.Result
	beforeRPC bool
	// sourceDigest is the digest of the source snapshot, set only when that
	// snapshot and the profile alone prove the refusal: an empty persisted
	// identity or a projection the profile does not support. Retrying the same
	// source would be refused whatever the daemon answered. A refusal that
	// comes from the daemon's responses leaves it empty, so watch retries it.
	sourceDigest string
}

func (e *completenessError) Error() string {
	return fmt.Sprintf("completeness %s: %s", e.result.State, e.result.Reason)
}
func unverified(state completeness.State, reason string) error {
	return &completenessError{result: completeness.Result{State: state, Reason: reason}}
}

// fetchSession uses one authenticated endpoint and persisted identity for both
// representations. A second source snapshot rejects same-step writes in flight.
func fetchSession(ctx context.Context, client *daemon.Client, s discovery.Session) (*daemon.Trajectory, error) {
	if !strings.HasSuffix(s.PBPath, ".db") {
		return client.FetchTrajectoryWithFallback(ctx, s.CascadeID, s.InternalCascadeID)
	}
	before, err := completeness.ReadSnapshot(ctx, s.PBPath)
	if err != nil {
		return nil, &completenessError{result: completeness.Result{State: completeness.Unknown, Reason: err.Error()}, beforeRPC: true}
	}
	if err = client.LoadTrajectory(ctx, s.CascadeID); err != nil {
		return nil, err
	}
	id := before.CascadeID()
	if id == "" {
		return nil, &completenessError{result: completeness.Result{State: completeness.Unknown, Reason: "persisted cascade identity is empty"}, sourceDigest: before.Digest}
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
		refusal := &completenessError{result: result}
		if sourceProven {
			refusal.sourceDigest = before.Digest
		}
		return nil, refusal
	}
	after, err := completeness.ReadSnapshot(ctx, s.PBPath)
	if err != nil {
		return nil, unverified(completeness.Unknown, err.Error())
	}
	if after.Digest != before.Digest {
		return nil, unverified(completeness.Incomplete, "persisted source changed during fetch")
	}
	presentation.Verification = attestation
	presentation.SourcePath = s.PBPath
	return presentation, nil
}

// fetchSessionRecover tries a finite scan of existing root-owned candidates
// after a content mismatch. It never falls back from an explicitly pinned URL.
func fetchSessionRecover(ctx context.Context, client *daemon.Client, root string, s discovery.Session, pinned bool) (*daemon.Trajectory, error) {
	return fetchSessionCandidates(ctx, client, root, s, pinned, discovery.Connections)
}
func fetchSessionCandidates(ctx context.Context, client *daemon.Client, root string, s discovery.Session, pinned bool, candidates func(string) []discovery.Connection) (*daemon.Trajectory, error) {
	traj, err := fetchSession(ctx, client, s)
	var contentErr *completenessError
	if err == nil || pinned || !errors.As(err, &contentErr) || contentErr.result.State != completeness.Incomplete {
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
		next := newDaemonClient(root, conn)
		recovered, nextErr := fetchSession(ctx, next, s)
		if nextErr == nil {
			*client = *next
			return recovered, nil
		}
		// Keep the reachable-but-incomplete diagnosis: an alternative endpoint's
		// failure must not turn a content problem into an idle/unreachable state.
	}
	return nil, err
}

func sessionCompleteness(s discovery.Session) completeness.Result {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snapshot, err := completeness.ReadSnapshot(ctx, s.PBPath)
	if err != nil {
		return completeness.Result{State: completeness.Unknown, Reason: err.Error()}
	}
	raw, err := os.ReadFile(s.SidecarPath)
	if err != nil {
		return completeness.Result{State: completeness.Unknown, Reason: "sidecar unavailable"}
	}
	return snapshot.CheckAttestation(raw, completeness.ReadAttestation(raw))
}

func fetchDiskSession(ctx context.Context, conn discovery.Connection, root string, s discovery.Session) (*daemon.Trajectory, error) {
	client := newDaemonClient(root, conn)
	traj, err := fetchSessionRecover(ctx, client, root, s, strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL")) != "")
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
