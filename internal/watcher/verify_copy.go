package watcher

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/session"
)

const verifierEnv = "AGY_READER_PAIR_VERIFIER"

// agy-pair supplies a helper command and its validated private run directory.
func pairedVerifierConfig() verifierConfig {
	var config verifierConfig
	if json.Unmarshal([]byte(os.Getenv(verifierEnv)), &config) != nil ||
		len(config.Command) == 0 || !filepath.IsAbs(config.Command[0]) || !filepath.IsAbs(config.Directory) {
		return verifierConfig{}
	}
	return config
}

type verifierConfig struct {
	Command   []string
	Directory string
}

type copyVerifier struct {
	config      verifierConfig
	lastAttempt map[string]time.Time
}

func (v *copyVerifier) order(sessions []discovery.Session) {
	if len(v.lastAttempt) == 0 {
		return
	}
	present := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		present[s.PBPath] = true
	}
	for path := range v.lastAttempt {
		if !present[path] {
			delete(v.lastAttempt, path)
		}
	}
	// Spend the next launch on an untried source or the one that has waited
	// longest. Keep discovery order for sources with equal priority.
	slices.SortStableFunc(sessions, func(a, b discovery.Session) int {
		return v.lastAttempt[a.PBPath].Compare(v.lastAttempt[b.PBPath])
	})
}

func canVerifyCopy(s discovery.Session, err error) bool {
	if filepath.Ext(s.PBPath) != ".db" {
		return false
	}
	if errors.Is(err, daemon.ErrTranscriptTotalsMismatch) {
		return true
	}
	var mismatch *session.CompletenessError
	return errors.As(err, &mismatch) && !mismatch.BeforeRPC && mismatch.CaptureID == "" &&
		mismatch.Result.State == completeness.Incomplete && mismatch.Result.Reason == completeness.PersistedRPCMismatch
}

func verifyCopy(ctx context.Context, config verifierConfig, source discovery.Session) (*daemon.Trajectory, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp(config.Directory, "agy-reader-verify-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	root := filepath.Join(dir, ".gemini", "antigravity-cli")
	conversations := filepath.Join(root, "conversations")
	if err := os.MkdirAll(conversations, 0700); err != nil {
		return nil, err
	}
	copySession := source
	copySession.PBPath = filepath.Join(conversations, filepath.Base(source.PBPath))
	if err := backupConversation(ctx, source.PBPath, copySession.PBPath); err != nil {
		return nil, fmt.Errorf("back up conversation: %w", err)
	}

	command := config.Command
	cmd := exec.CommandContext(ctx, command[0], append(command[1:len(command):len(command)], dir)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer stdout.Close()
	cmd.Cancel = stdin.Close
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start verifier: %w", err)
	}
	defer func() {
		// EOF asks the helper to stop and reap its child. Cancel also arms
		// WaitDelay in case the helper does not respond.
		cancel()
		_ = cmd.Wait()
	}()

	scanner := bufio.NewScanner(stdout)
	var ready struct {
		Port  int
		Token string
	}
	if !scanner.Scan() || json.Unmarshal(scanner.Bytes(), &ready) != nil ||
		ready.Port < 1 || ready.Port > 65535 || ready.Token == "" {
		return nil, errors.New("isolated daemon did not become ready")
	}
	client := daemon.NewClient("http://127.0.0.1:" + strconv.Itoa(ready.Port))
	client.CSRFToken = ready.Token
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	client.HTTP.Transport = transport
	client.HTTP.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	trajectory, err := session.Fetch(ctx, client, copySession)
	if err != nil {
		return nil, err
	}
	// WriteVerified checks this attestation against the live SQLite source
	// immediately before replacing the sidecar. The copy is never a write target.
	trajectory.SourcePath = source.PBPath
	return trajectory, nil
}

func backupConversation(ctx context.Context, source, destination string) error {
	absolute, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	uri := url.URL{Scheme: "file", Path: absolute, RawQuery: "mode=ro&_pragma=busy_timeout%28250%29"}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return err
	}
	defer db.Close()
	// VACUUM INTO takes a consistent snapshot including committed WAL pages,
	// without copying the writer's locks, journal or shared memory.
	_, err = db.ExecContext(ctx, "VACUUM INTO ?", destination)
	return err
}
