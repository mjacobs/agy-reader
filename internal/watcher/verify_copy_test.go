package watcher

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/session"
)

func TestVerifyCopyRequiresPersistedContentMismatch(t *testing.T) {
	trajectory := discovery.Session{PBPath: "session.db"}
	stale := session.Unverified(completeness.Incomplete, completeness.PersistedRPCMismatch)
	if !canVerifyCopy(trajectory, stale) {
		t.Fatal("stored-content mismatch should be recovered")
	}
	staleTranscript := fmt.Errorf("%w: %w", daemon.ErrRecovery, daemon.ErrTranscriptTotalsMismatch)
	if !canVerifyCopy(trajectory, staleTranscript) || canVerifyCopy(trajectory, daemon.ErrRecovery) {
		t.Fatal("only a record-count mismatch should make a failed transcript fetch recoverable")
	}
	for _, reason := range []string{
		"daemon presentation differs from supported source projection",
		"persisted source changed during fetch",
	} {
		if canVerifyCopy(trajectory, session.Unverified(completeness.Incomplete, reason)) {
			t.Fatalf("verifier would start for %q", reason)
		}
	}
}

func TestBackupConversationIncludesCommittedWAL(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "original.db")
	copyPath := filepath.Join(dir, "copy.db")
	writer, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	for _, query := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA wal_autocheckpoint=0",
		"CREATE TABLE steps (idx INTEGER PRIMARY KEY)",
		"INSERT INTO steps VALUES (1), (2)",
	} {
		if _, err := writer.Exec(query); err != nil {
			t.Fatal(err)
		}
	}

	wal, err := os.Stat(source + "-wal")
	if err != nil || wal.Size() == 0 {
		t.Fatalf("expected committed WAL data, got %v, %v", wal, err)
	}
	if err := backupConversation(context.Background(), source, copyPath); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Exec("INSERT INTO steps VALUES (3)"); err != nil {
		t.Fatal(err)
	}

	backup, err := sql.Open("sqlite", copyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var count int
	if err := backup.QueryRow("SELECT count(*) FROM steps").Scan(&count); err != nil || count != 2 {
		t.Fatalf("backup contains %d steps, want 2 (query error: %v)", count, err)
	}
}

func TestCopyVerifierGivesOtherSourcesATurn(t *testing.T) {
	recent := time.Now()
	v := copyVerifier{lastAttempt: map[string]time.Time{"busy.db": recent, "gone.db": recent}}
	// The writer updated busy.db while its first verification was running.
	sessions := []discovery.Session{
		{PBPath: "busy.db", ModTime: recent.Add(time.Second)},
		{PBPath: "waiting.db", ModTime: recent.Add(-time.Hour)},
	}
	v.order(sessions)
	if sessions[0].PBPath != "waiting.db" || len(v.lastAttempt) != 1 {
		t.Fatalf("expected the unchanged conversation to go first and gone.db to be forgotten: %+v", sessions)
	}
	v.lastAttempt["waiting.db"] = recent.Add(2 * time.Second)
	v.order(sessions)
	if sessions[0].PBPath != "busy.db" {
		t.Fatalf("expected oldest attempt to go first: %+v", sessions)
	}
}
