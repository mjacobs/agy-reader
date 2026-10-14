package cli

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
	"github.com/mjacobs/agy-reader/internal/doctor"
	"github.com/mjacobs/agy-reader/internal/render"
	"github.com/mjacobs/agy-reader/internal/session"
)

const SweepManifestVersion = 4

const (
	ReaderSyncManifestVersion = 2
	SourceOnlyManifestVersion = 3
)

const (
	ReaderSyncVerified   = string(completeness.Verified)
	ReaderSyncLegacy     = "unverifiable-legacy"
	SourceSQLite         = "sqlite"
	SourceLegacyProtobuf = "legacy-protobuf"
)

type SweepSession struct {
	FileID           string `json:"fileId"`
	CascadeID        string `json:"cascadeId"`
	Steps            int    `json:"steps"`
	Source           string `json:"source"`
	ReaderSync       string `json:"readerSync"`
	ReaderSyncReason string `json:"readerSyncReason,omitempty"`
	SourceDigest     string `json:"sourceDigest,omitempty"`
	Profile          string `json:"profile,omitempty"`
}

type SweepReaderSummary struct {
	SQLite             int `json:"sqlite"`
	Verified           int `json:"verified"`
	VerifiedSourceOnly int `json:"verifiedSourceOnly"`
	NotVerified        int `json:"notVerified"`
	Legacy             int `json:"legacyUnverifiable"`
}

func (r SweepReaderSummary) Counts() string {
	sourceOnly := ""
	if r.VerifiedSourceOnly > 0 {
		sourceOnly = fmt.Sprintf(" verified-with-source-only-fields=%d", r.VerifiedSourceOnly)
	}
	return fmt.Sprintf("verified=%d/%d SQLite%s not-verified=%d legacy-unverifiable=%d", r.Verified, r.SQLite, sourceOnly, r.NotVerified, r.Legacy)
}

type SweepManifest struct {
	Version    int                `json:"manifestVersion"`
	Complete   bool               `json:"complete"`
	Root       string             `json:"root"`
	Endpoint   string             `json:"endpoint"`
	AgyVersion string             `json:"installedAgyVersion"`
	Started    time.Time          `json:"started"`
	Finished   time.Time          `json:"finished"`
	Selected   int                `json:"selected"`
	FailedID   string             `json:"failedId,omitempty"`
	ReaderSync SweepReaderSummary `json:"readerSync"`
	Sessions   []SweepSession     `json:"sessions"`
}

const maxListedUnverified = 20

func RunAuditSweep(args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return RunAuditSweepTo(ctx, args, os.Stderr)
}

func RunAuditSweepTo(ctx context.Context, args []string, out io.Writer) error {
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
	seen := map[string]bool{}
	for _, s := range sessions {
		if daemon.CanonicalCascadeID(s.CascadeID) == "" || seen[s.CascadeID] {
			return fmt.Errorf("audit-sweep: invalid or duplicate session ID %q", s.CascadeID)
		}
		seen[s.CascadeID] = true
	}
	conn, err := RequireDaemonConnection(absRoot)
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
	m := SweepManifest{
		Version: SweepManifestVersion, Root: absRoot, Endpoint: conn.BaseURL, AgyVersion: doctor.AgyVersion(),
		Started: time.Now().UTC(), Selected: len(sessions), Sessions: []SweepSession{},
	}
	if err := WriteSweepManifest(*dest, m); err != nil {
		return err
	}
	client := session.NewDaemonClient(absRoot, conn)
	client.HTTP.Timeout = *timeout
	returned := map[string]string{}
	for _, s := range sessions {
		fetchCtx, cancel := context.WithTimeout(ctx, *timeout)
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
			if err := WriteSweepManifest(*dest, m); err != nil {
				return fmt.Errorf("audit-sweep: fetch failed (%v); write incomplete manifest: %w", fetchErr, err)
			}
			return fmt.Errorf("audit-sweep: %s: %w; incomplete corpus retained at %s (do not use --corpus-swept)", s.CascadeID, fetchErr, *dest)
		}
		entry := SweepSession{FileID: s.CascadeID, CascadeID: t.CascadeID, Steps: len(t.Steps)}
		if qualifyErr := qualifySweepSession(ctx, client, s, *timeout, &entry); qualifyErr != nil {
			m.FailedID = s.CascadeID
			m.Finished = time.Now().UTC()
			if err := WriteSweepManifest(*dest, m); err != nil {
				return fmt.Errorf("audit-sweep: qualification interrupted (%v); write incomplete manifest: %w", qualifyErr, err)
			}
			return fmt.Errorf("audit-sweep: %s: %w; incomplete corpus retained at %s (do not use --corpus-swept)", s.CascadeID, qualifyErr, *dest)
		}
		m.ReaderSync.Add(entry)
		m.Sessions = append(m.Sessions, entry)
		if len(m.Sessions)%40 == 0 {
			if _, err := fmt.Fprintf(out, "audit-sweep: fetched and rendered %d/%d\n", len(m.Sessions), m.Selected); err != nil {
				return err
			}
		}
	}
	m.Complete = true
	m.Finished = time.Now().UTC()
	if err := WriteSweepManifest(*dest, m); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "audit-sweep: selected=%d fetched=%d rendered=%d failures=0 corpus=%s\n", m.Selected, len(m.Sessions), len(m.Sessions), corpus); err != nil {
		return err
	}
	return printSweepReaderSync(out, m)
}

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

func qualifySweepSession(ctx context.Context, client *daemon.Client, s discovery.Session, timeout time.Duration, entry *SweepSession) error {
	if !strings.HasSuffix(s.PBPath, ".db") {
		entry.Source = SourceLegacyProtobuf
		entry.ReaderSync = ReaderSyncLegacy
		entry.ReaderSyncReason = "legacy protobuf sessions have no native source to verify"
		return nil
	}
	entry.Source = SourceSQLite
	verifyCtx, cancel := context.WithTimeout(ctx, timeout)
	t, err := session.Fetch(verifyCtx, client, s)
	cancel()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	var contentErr *session.CompletenessError
	switch {
	case err == nil:
		entry.ReaderSync = ReaderSyncVerified
		if t.Verification != nil {
			entry.ReaderSyncReason = t.Verification.Reason
			entry.SourceDigest = t.Verification.SourceDigest
			entry.Profile = t.Verification.Profile
		}
	case errors.As(err, &contentErr):
		entry.ReaderSync = string(contentErr.Result.State)
		entry.ReaderSyncReason = contentErr.Result.Reason
	default:
		entry.ReaderSync = string(completeness.Unknown)
		entry.ReaderSyncReason = "verification fetch failed: " + err.Error()
	}
	return nil
}

func (r *SweepReaderSummary) Add(s SweepSession) {
	switch {
	case s.Source == SourceLegacyProtobuf:
		r.Legacy++
	case session.SourceOnlyVerified(s.ReaderSync, s.ReaderSyncReason):
		r.SQLite++
		r.VerifiedSourceOnly++
	case s.ReaderSync == ReaderSyncVerified:
		r.SQLite++
		r.Verified++
	default:
		r.SQLite++
		r.NotVerified++
	}
}

func printSweepReaderSync(out io.Writer, m SweepManifest) error {
	r := m.ReaderSync
	if _, err := fmt.Fprintf(out, "audit-sweep: reader sync %s (ordinary sync verification; no sidecars written)\n", r.Counts()); err != nil {
		return err
	}
	listed := 0
	for _, s := range m.Sessions {
		if s.Source != SourceSQLite || s.ReaderSync == ReaderSyncVerified {
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
	_, err := fmt.Fprintln(out, "audit-sweep: fetched and rendered is not synced; audit_format.sh refuses a record unless the newest and AGY_QUALIFY_SESSIONS conversations are verified")
	return err
}

func WriteSweepManifest(dest string, m SweepManifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dest, "manifest.json.tmp")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dest, "manifest.json"))
}
