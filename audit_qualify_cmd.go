package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
)

func runAuditQualify(args []string) error {
	return runAuditQualifyTo(args, os.Stdout)
}

// runAuditQualifyTo gates a compatibility record on reader sync, not on fetch
// and render coverage. Every named conversation must be a SQLite session the
// current reader verifies natively: persisted source decoded under the
// completeness profile and matched against full and default RPC. Evidence
// comes from an audit-sweep manifest when one is supplied, otherwise from the
// live sidecar's attestation rechecked against the current source.
func runAuditQualifyTo(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("audit-qualify", flag.ContinueOnError)
	fs.SetOutput(out)
	root := fs.String("root", "", "session store the conversations belong to (required)")
	manifestPath := fs.String("manifest", "", "audit-sweep manifest.json to qualify from (default: live sidecar attestations)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ids := fs.Args()
	if *root == "" || len(ids) == 0 {
		return fmt.Errorf("usage: agy-reader audit-qualify --root=PATH [--manifest=PATH] CONVERSATION_ID...")
	}
	for _, id := range ids {
		if daemon.CanonicalCascadeID(id) == "" {
			return fmt.Errorf("audit-qualify: invalid conversation ID %q", id)
		}
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	var check func(id string) (string, string)
	source := "live sidecar attestation"
	if *manifestPath != "" {
		m, err := readQualifyingManifest(*manifestPath, absRoot)
		if err != nil {
			return err
		}
		source = "audit-sweep manifest"
		check = manifestQualification(m)
		r := m.ReaderSync
		// Historical sessions outside the profile are reported, never gating:
		// the broad sweep must stay usable on a store with old conversations.
		if _, err := fmt.Fprintf(out, "reader sync (whole sweep, not gating): %s\n", r.counts()); err != nil {
			return err
		}
	} else {
		sessions, err := discovery.ListConversationSessions(absRoot)
		if err != nil {
			return err
		}
		check = liveQualification(sessions)
	}
	failed := 0
	for _, id := range ids {
		state, reason := check(id)
		// A source-only acceptance qualifies like plain verified, but its line
		// names it and gives the reason, so it never reads as plain verified.
		if sourceOnlyVerified(state, reason) {
			if _, err := fmt.Fprintf(out, "reader sync: %s VERIFIED WITH SOURCE-ONLY FIELDS (%s): %s\n", id, source, reason); err != nil {
				return err
			}
			continue
		}
		if state == readerSyncVerified {
			if _, err := fmt.Fprintf(out, "reader sync: %s VERIFIED (%s)\n", id, source); err != nil {
				return err
			}
			continue
		}
		failed++
		if _, err := fmt.Fprintf(out, "reader sync: %s NOT VERIFIED (%s): %s: %s\n", id, source, state, reason); err != nil {
			return err
		}
	}
	if failed > 0 {
		return fmt.Errorf("audit-qualify: %d of %d required conversation(s) not verified by the reader; fetch/render success does not qualify a record", failed, len(ids))
	}
	return nil
}

func readQualifyingManifest(path, absRoot string) (sweepManifest, error) {
	var m sweepManifest
	b, err := os.ReadFile(path)
	if err != nil {
		return m, fmt.Errorf("audit-qualify: read manifest: %w", err)
	}
	if err = json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("audit-qualify: parse manifest: %w", err)
	}
	switch {
	case m.Version < readerSyncManifestVersion:
		return m, fmt.Errorf("audit-qualify: manifest version %d predates reader-sync qualification; rerun audit-sweep", m.Version)
	case m.Version < sweepManifestVersion:
		return m, fmt.Errorf("audit-qualify: manifest version %d does not separate source-only acceptances from verified; rerun audit-sweep", m.Version)
	case m.Version > sweepManifestVersion:
		return m, fmt.Errorf("audit-qualify: unsupported manifest version %d", m.Version)
	case !m.Complete:
		return m, errors.New("audit-qualify: manifest is incomplete; rerun audit-sweep into a new directory")
	case !sameDir(m.Root, absRoot):
		return m, fmt.Errorf("audit-qualify: manifest root %s is not the audited root %s", m.Root, absRoot)
	}
	return m, nil
}

func sameDir(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

func manifestQualification(m sweepManifest) func(string) (string, string) {
	byID := map[string]sweepSession{}
	for _, s := range m.Sessions {
		byID[s.FileID] = s
	}
	return func(id string) (string, string) {
		s, ok := byID[id]
		switch {
		case !ok:
			return string(completeness.Unknown), "not in the sweep selection (sweep predates it, or it is not a syncable conversation)"
		case s.Source != sourceSQLite:
			return s.ReaderSync, "only SQLite sessions can be natively qualified"
		case s.ReaderSync == "":
			return string(completeness.Unknown), "no reader-sync result recorded"
		}
		return s.ReaderSync, s.ReaderSyncReason
	}
}

func liveQualification(sessions []discovery.Session) func(string) (string, string) {
	byID := map[string]discovery.Session{}
	for _, s := range sessions {
		byID[s.CascadeID] = s
	}
	return func(id string) (string, string) {
		s, ok := byID[id]
		switch {
		case !ok:
			return string(completeness.Unknown), "no syncable conversation with this ID in the root"
		case !strings.HasSuffix(s.PBPath, ".db"):
			return readerSyncLegacy, "only SQLite sessions can be natively qualified"
		}
		if _, err := os.Stat(s.SidecarPath); err != nil {
			return string(completeness.Unknown), "no sidecar: ordinary sync has not verified and written this conversation"
		}
		r := sessionCompleteness(s)
		return string(r.State), r.Reason
	}
}
