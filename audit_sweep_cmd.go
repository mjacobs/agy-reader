package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/render"
)

// sweepManifestVersion 4 records, for each verified conversation, the digest
// of the persisted source the verification covered and the completeness
// profile it ran under, so audit-qualify can tell whether the source changed
// after the sweep. Version 3 records the reason for a source-only acceptance
// and counts it in verifiedSourceOnly, so readerSync.verified holds plain
// verified results only. Version 2 added per-session reader-sync
// qualification, but its verified count and entries do not separate
// source-only acceptances. Version 1 manifests only prove fetch/render
// coverage. None of the older versions can qualify a record.
const sweepManifestVersion = 4

// readerSyncManifestVersion is the first version with reader-sync results, and
// sourceOnlyManifestVersion the first that separates source-only acceptances.
const (
	readerSyncManifestVersion = 2
	sourceOnlyManifestVersion = 3
)

// Reader-sync states recorded per session. Verified, incomplete and unknown are
// the completeness states ordinary sync reports; legacy protobuf sessions have
// no native source the reader can verify, so ordinary sync never certifies them.
const (
	readerSyncVerified   = string(completeness.Verified)
	readerSyncLegacy     = "unverifiable-legacy"
	sourceSQLite         = "sqlite"
	sourceLegacyProtobuf = "legacy-protobuf"
)

type sweepSession struct {
	FileID    string `json:"fileId"`
	CascadeID string `json:"cascadeId"`
	Steps     int    `json:"steps"`
	Source    string `json:"source"`
	// ReaderSync is the result of the same persisted-source and full/default RPC
	// verification ordinary sync performs before it may write a sidecar.
	ReaderSync       string `json:"readerSync"`
	ReaderSyncReason string `json:"readerSyncReason,omitempty"`
	// SourceDigest and Profile identify the persisted source a verified result
	// covered. A verification is evidence only for that source: audit-qualify
	// compares them with a fresh snapshot, because a conversation can change
	// after the sweep in a way ordinary sync would now refuse.
	SourceDigest string `json:"sourceDigest,omitempty"`
	Profile      string `json:"profile,omitempty"`
}

// sweepReaderSummary keeps fetch/render coverage and reader qualification as
// separate counts so a fully rendered corpus can never read as a synced one.
// Verified counts plain verified results only; a source-only acceptance
// (verified, with declared source-only fields the RPC never carries) is
// counted in VerifiedSourceOnly. SQLite is the sum of those two and NotVerified.
type sweepReaderSummary struct {
	SQLite             int `json:"sqlite"`
	Verified           int `json:"verified"`
	VerifiedSourceOnly int `json:"verifiedSourceOnly"`
	NotVerified        int `json:"notVerified"`
	Legacy             int `json:"legacyUnverifiable"`
}

// counts formats the summary for a console line. The source-only count
// appears only when there is one, so a store without such sources prints the
// same line as before.
func (r sweepReaderSummary) counts() string {
	sourceOnly := ""
	if r.VerifiedSourceOnly > 0 {
		sourceOnly = fmt.Sprintf(" verified-with-source-only-fields=%d", r.VerifiedSourceOnly)
	}
	return fmt.Sprintf("verified=%d/%d SQLite%s not-verified=%d legacy-unverifiable=%d", r.Verified, r.SQLite, sourceOnly, r.NotVerified, r.Legacy)
}

type sweepManifest struct {
	Version    int                `json:"manifestVersion"`
	Complete   bool               `json:"complete"`
	Root       string             `json:"root"`
	Endpoint   string             `json:"endpoint"`
	AgyVersion string             `json:"installedAgyVersion"`
	Started    time.Time          `json:"started"`
	Finished   time.Time          `json:"finished"`
	Selected   int                `json:"selected"`
	FailedID   string             `json:"failedId,omitempty"`
	ReaderSync sweepReaderSummary `json:"readerSync"`
	Sessions   []sweepSession     `json:"sessions"`
}

// maxListedUnverified bounds the console listing; the manifest keeps them all.
const maxListedUnverified = 20

func runAuditSweep(args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return runAuditSweepTo(ctx, args, os.Stderr)
}

func runAuditSweepTo(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("audit-sweep", flag.ContinueOnError)
	fs.SetOutput(out)
	root := fs.String("root", "", "one session store (default: ANTIGRAVITY_CLI_ROOT or CLI store)")
	dest := fs.String("out", "", "new private directory for corpus/ and manifest.json; must not exist")
	timeout := fs.Duration("timeout", 30*time.Second, "timeout per conversation fetch")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *dest == "" || *timeout <= 0 {
		return fmt.Errorf("usage: agy-reader audit-sweep --out=NEW_DIRECTORY [--root=PATH] [--timeout=30s]")
	}
	if *root == "" {
		var err error
		*root, err = discovery.Root()
		if err != nil {
			return err
		}
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	sessions, err := discovery.ListConversationSessions(absRoot)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return fmt.Errorf("audit-sweep: no syncable conversations under %s", absRoot)
	}
	// A corpus file must correspond to exactly one fetch, and filenames must
	// remain inside the output dir. Discovery already lists a forked
	// conversation that has both <id>.pb and <id>.db once, by its .db, so a
	// duplicate here means the listing itself is wrong.
	seen := map[string]bool{}
	for _, s := range sessions {
		if daemon.CanonicalCascadeID(s.CascadeID) == "" || seen[s.CascadeID] {
			return fmt.Errorf("audit-sweep: invalid or duplicate session ID %q", s.CascadeID)
		}
		seen[s.CascadeID] = true
	}
	conn, err := requireDaemonConnection(absRoot)
	if err != nil {
		return err
	}
	if err := os.Mkdir(*dest, 0o700); err != nil {
		return fmt.Errorf("audit-sweep: create new output directory: %w", err)
	}
	corpus := filepath.Join(*dest, "corpus")
	if err := os.Mkdir(corpus, 0o700); err != nil {
		return err
	}
	m := sweepManifest{
		Version: sweepManifestVersion, Root: absRoot, Endpoint: conn.BaseURL, AgyVersion: agyVersion(),
		Started: time.Now().UTC(), Selected: len(sessions), Sessions: []sweepSession{},
	}
	if err := writeSweepManifest(*dest, m); err != nil {
		return err
	}
	client := newDaemonClient(absRoot, conn)
	client.HTTP.Timeout = *timeout
	// returned maps each trajectory identity the daemon answered with to the
	// selected file it answered for, so one conversation can never be counted
	// as fetched twice under two filenames.
	returned := map[string]string{}
	for _, s := range sessions {
		fetchCtx, cancel := context.WithTimeout(ctx, *timeout)
		// Deliberately bypass fetchTrajectory: audit evidence must never fall
		// back to an existing sidecar when the live daemon fails. This coverage
		// fetch proves only that the ordinary RPC fetches and renders; reader
		// sync is qualified separately below.
		t, fetchErr := client.FetchTrajectoryWithFallback(fetchCtx, s.CascadeID, s.InternalCascadeID)
		cancel()
		if fetchErr == nil {
			fetchErr = checkSweepIdentity(ctx, s, t.CascadeID, returned)
		}
		if fetchErr == nil {
			_, fetchErr = render.Markdown(io.Discard, t, time.Now())
		}
		if fetchErr == nil {
			fetchErr = os.WriteFile(filepath.Join(corpus, s.CascadeID+".trajectory.json"), t.RawJSON, 0o600)
		}
		if fetchErr != nil {
			m.FailedID = s.CascadeID
			m.Finished = time.Now().UTC()
			if err := writeSweepManifest(*dest, m); err != nil {
				return fmt.Errorf("audit-sweep: fetch failed (%v); write incomplete manifest: %w", fetchErr, err)
			}
			return fmt.Errorf("audit-sweep: %s: %w; incomplete corpus retained at %s (do not use --corpus-swept)", s.CascadeID, fetchErr, *dest)
		}
		entry := sweepSession{FileID: s.CascadeID, CascadeID: t.CascadeID, Steps: len(t.Steps)}
		if qualifyErr := qualifySweepSession(ctx, client, s, *timeout, &entry); qualifyErr != nil {
			m.FailedID = s.CascadeID
			m.Finished = time.Now().UTC()
			if err := writeSweepManifest(*dest, m); err != nil {
				return fmt.Errorf("audit-sweep: qualification interrupted (%v); write incomplete manifest: %w", qualifyErr, err)
			}
			return fmt.Errorf("audit-sweep: %s: %w; incomplete corpus retained at %s (do not use --corpus-swept)", s.CascadeID, qualifyErr, *dest)
		}
		m.ReaderSync.add(entry)
		m.Sessions = append(m.Sessions, entry)
		if len(m.Sessions)%40 == 0 {
			if _, err := fmt.Fprintf(out, "audit-sweep: fetched and rendered %d/%d\n", len(m.Sessions), m.Selected); err != nil {
				return err
			}
		}
	}
	m.Complete = true
	m.Finished = time.Now().UTC()
	if err := writeSweepManifest(*dest, m); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "audit-sweep: selected=%d fetched=%d rendered=%d failures=0 corpus=%s\n", m.Selected, len(m.Sessions), len(m.Sessions), corpus); err != nil {
		return err
	}
	return printSweepReaderSync(out, m)
}

// checkSweepIdentity accepts a fetched trajectory only when it is the selected
// conversation. A valid identity is not enough: a daemon that answers with
// another conversation would otherwise have that conversation counted as this
// file's evidence, and stored in the corpus a second time. The answer must name
// the selected conversation (see selectedSweepIdentity), and no earlier file in
// the sweep may have been answered with the same identity.
func checkSweepIdentity(ctx context.Context, s discovery.Session, got string, returned map[string]string) error {
	id := daemon.CanonicalCascadeID(got)
	if id == "" {
		return fmt.Errorf("daemon returned no valid trajectory identity")
	}
	if !selectedSweepIdentity(ctx, s, id) {
		return fmt.Errorf("daemon returned trajectory %s, not the selected conversation", id)
	}
	if earlier, ok := returned[id]; ok {
		return fmt.Errorf("daemon returned trajectory %s already fetched for %s", id, earlier)
	}
	returned[id] = s.CascadeID
	return nil
}

// selectedSweepIdentity reports whether id names the selected conversation: the
// filename's ID, the internal ID discovery read, or the cascade_id the native
// source reader finds in a SQLite session. Discovery reads the internal ID from
// a fixed database page, so a rebuilt or imported database whose tables sit
// elsewhere gives it no internal ID even though the reader verifies that
// database and the daemon answers with its stored identity. The reader is
// consulted only when the quicker identities do not match.
func selectedSweepIdentity(ctx context.Context, s discovery.Session, id string) bool {
	if id == daemon.CanonicalCascadeID(s.CascadeID) {
		return true
	}
	if internal := daemon.CanonicalCascadeID(s.InternalCascadeID); internal != "" && id == internal {
		return true
	}
	if !strings.HasSuffix(s.PBPath, ".db") {
		return false
	}
	snapshot, err := completeness.ReadSnapshot(ctx, s.PBPath)
	return err == nil && id == daemon.CanonicalCascadeID(snapshot.CascadeID())
}

// qualifySweepSession runs ordinary sync's verification (fetchSession) without
// writing a sidecar. A conversation that fetches and renders but fails it is
// recorded as not verified rather than aborting the sweep: broad inspection of
// a historical corpus must survive old sessions outside the current profile.
// Only cancellation of the whole sweep aborts it.
func qualifySweepSession(ctx context.Context, client *daemon.Client, s discovery.Session, timeout time.Duration, entry *sweepSession) error {
	if !strings.HasSuffix(s.PBPath, ".db") {
		entry.Source = sourceLegacyProtobuf
		entry.ReaderSync = readerSyncLegacy
		entry.ReaderSyncReason = "legacy protobuf sessions have no native source to verify"
		return nil
	}
	entry.Source = sourceSQLite
	verifyCtx, cancel := context.WithTimeout(ctx, timeout)
	t, err := fetchSession(verifyCtx, client, s)
	cancel()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	var contentErr *completenessError
	switch {
	case err == nil:
		entry.ReaderSync = readerSyncVerified
		// A source-only acceptance keeps its reason, so the manifest and the
		// summary never record it as plain verified.
		if t.Verification != nil {
			entry.ReaderSyncReason = t.Verification.Reason
			entry.SourceDigest = t.Verification.SourceDigest
			entry.Profile = t.Verification.Profile
		}
	case errors.As(err, &contentErr):
		entry.ReaderSync = string(contentErr.result.State)
		entry.ReaderSyncReason = contentErr.result.Reason
	default:
		// The coverage fetch for this session just succeeded, so a transport
		// failure here is recorded as unverified evidence, never as success.
		entry.ReaderSync = string(completeness.Unknown)
		entry.ReaderSyncReason = "verification fetch failed: " + err.Error()
	}
	return nil
}

func (r *sweepReaderSummary) add(s sweepSession) {
	switch {
	case s.Source == sourceLegacyProtobuf:
		r.Legacy++
	case sourceOnlyVerified(s.ReaderSync, s.ReaderSyncReason):
		r.SQLite++
		r.VerifiedSourceOnly++
	case s.ReaderSync == readerSyncVerified:
		r.SQLite++
		r.Verified++
	default:
		r.SQLite++
		r.NotVerified++
	}
}

func printSweepReaderSync(out io.Writer, m sweepManifest) error {
	r := m.ReaderSync
	if _, err := fmt.Fprintf(out, "audit-sweep: reader sync %s (ordinary sync verification; no sidecars written)\n", r.counts()); err != nil {
		return err
	}
	listed := 0
	for _, s := range m.Sessions {
		if s.Source != sourceSQLite || s.ReaderSync == readerSyncVerified {
			continue
		}
		if listed == maxListedUnverified {
			if _, err := fmt.Fprintf(out, "audit-sweep: ... %d more not verified; see manifest.json\n", r.NotVerified-listed); err != nil {
				return err
			}
			break
		}
		listed++
		if _, err := fmt.Fprintf(out, "audit-sweep: NOT VERIFIED %s: %s: %s\n", s.FileID, s.ReaderSync, s.ReaderSyncReason); err != nil {
			return err
		}
	}
	if r.NotVerified == 0 {
		return nil
	}
	// Fetch/render coverage is broad inspection, not reader qualification.
	_, err := fmt.Fprintln(out, "audit-sweep: fetched and rendered is not synced; audit_format.sh refuses a record unless the newest and AGY_QUALIFY_SESSIONS conversations are verified")
	return err
}

func writeSweepManifest(dest string, m sweepManifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	// Publish completeness atomically; an interrupted run retains an
	// incomplete manifest rather than a truncated or falsely complete one.
	tmp := filepath.Join(dest, "manifest.json.tmp")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dest, "manifest.json"))
}
