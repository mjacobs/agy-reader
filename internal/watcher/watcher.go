package watcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/discovery"
	"github.com/mjacobs/agy-reader/internal/session"
	"github.com/mjacobs/agy-reader/internal/subagent"
	"github.com/mjacobs/agy-reader/internal/vcache"
)

// Watcher holds the state of a --watch session that persists across ticks: the
// current daemon base URL (which goes stale whenever agy restarts on a new
// port), the client bound to it, and the consecutive-failure streak used for
// retry/log bookkeeping.
type Watcher struct {
	Ctx                    context.Context
	Root                   string
	BaseURL                string
	Client                 *daemon.Client
	UrlPinned              bool
	Logger                 *log.Logger
	ConsecutiveFailures    int
	AuthBlocked            bool
	Held                   RefusalMemo
	Verification           *vcache.Cache
	VerificationSaveFailed bool

	Interval    time.Duration
	IdleTimeout time.Duration
	IdleTicks   int
	IdleExpired bool
}

func NewWatcher(
	ctx context.Context,
	root string,
	conn discovery.Connection,
	urlPinned bool,
	logger *log.Logger,
	interval, idleTimeout time.Duration,
	verification *vcache.Cache,
) *Watcher {
	return &Watcher{
		Ctx:          ctx,
		Root:         root,
		BaseURL:      conn.BaseURL,
		Client:       session.NewDaemonClient(root, conn),
		UrlPinned:    urlPinned,
		Logger:       logger,
		Interval:     interval,
		IdleTimeout:  idleTimeout,
		Verification: verification,
	}
}

// Tick performs one watch iteration: (re)discover the daemon URL when needed,
// then sync stale sidecars.
func (w *Watcher) Tick() (idleExpired bool) {
	haveScannedToken := false
	if (w.ConsecutiveFailures > 0 || w.AuthBlocked || w.BaseURL == "") && !w.UrlPinned {
		next, moved := RediscoverDaemonConnection(w.Root, w.BaseURL, w.Logger)
		switch {
		case moved:
			w.BaseURL = next.BaseURL
			w.Client = session.NewDaemonClient(w.Root, next)
			haveScannedToken = w.Client.CSRFToken != ""
		case next.Token != "":
			w.Client.CSRFToken = next.Token
			haveScannedToken = true
		}
	}
	if w.BaseURL == "" {
		w.Logger.Printf("tick: 0 synced, 0 skipped, 0 up-to-date, 0 failed (agy daemon not found yet; retrying every %s)", w.Interval)
		return w.UpdateIdle(true)
	}
	if !haveScannedToken {
		if token := discovery.DiscoverCSRFTokenForURL(w.Root, w.BaseURL); token != "" {
			w.Client.CSRFToken = token
		}
	}
	if w.Held == nil {
		w.Held = RefusalMemo{}
	}
	if w.Verification != nil {
		w.Verification.ResetStats()
	}
	synced, skipped, upToDate, failed, authBlocked := WatchTickCachedPolicy(w.Ctx, w.Client, w.Root, w.Logger, &w.ConsecutiveFailures, w.UrlPinned, w.Held, w.Verification)
	if w.Verification != nil {
		if err := w.Verification.Save(); err != nil {
			if !w.VerificationSaveFailed {
				w.Logger.Printf("watch: verification cache save failed (continuing in memory): %v", err)
			}
			w.VerificationSaveFailed = true
		} else {
			w.VerificationSaveFailed = false
		}
	}
	w.BaseURL = w.Client.BaseURL
	if w.AuthBlocked && !authBlocked && synced > 0 {
		w.Logger.Printf("watch: authentication recovered; syncing resumed")
	}
	w.AuthBlocked = authBlocked
	cacheNote := ""
	if w.Verification != nil {
		reused, checked := w.Verification.Stats()
		cacheNote = fmt.Sprintf(" (verification: %d reused, %d checked)", reused, checked)
	}
	w.Logger.Printf("tick: %d synced, %d skipped, %d up-to-date, %d failed%s", synced, skipped, upToDate, failed, cacheNote)
	return w.UpdateIdle(w.ConsecutiveFailures > 0)
}

func (w *Watcher) UpdateIdle(idle bool) (idleExpired bool) {
	if !idle {
		w.IdleTicks = 0
		w.IdleExpired = false
		return false
	}
	w.IdleTicks++
	if w.IdleTimeout <= 0 {
		return false
	}
	if time.Duration(w.IdleTicks)*w.Interval >= w.IdleTimeout {
		if !w.IdleExpired {
			w.Logger.Printf("watch: daemon unreachable for ~%s; root is idle (still polling; the process exits once every root is idle)", w.IdleTimeout)
		}
		w.IdleExpired = true
		return true
	}
	return false
}

// Run polls every session root each interval, fetching trajectories for
// any conversations/ .pb whose sidecar is missing or older than the source's
// ModTime.
func Run(roots []string, interval, idleTimeout time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("--watch-interval must be positive, got %s", interval)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return RunLoop(ctx, roots, interval, idleTimeout)
}

func WatchLogLabels(roots []string) []string {
	if len(roots) == 1 {
		return []string{""}
	}
	surfaces := make([]discovery.Surface, len(roots))
	counts := map[discovery.Surface]int{}
	for i, r := range roots {
		surfaces[i] = discovery.DetectSurface(r)
		counts[surfaces[i]]++
	}
	labels := make([]string, len(roots))
	for i := range roots {
		label := string(surfaces[i])
		if counts[surfaces[i]] > 1 {
			label = roots[i]
		}
		labels[i] = "[" + label + "] "
	}
	return labels
}

// RunLoop is the cancellable core of Run.
func RunLoop(ctx context.Context, roots []string, interval, idleTimeout time.Duration) error {
	pinned := strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL")) != ""
	labels := WatchLogLabels(roots)
	watchers := make([]*Watcher, 0, len(roots))
	for i, root := range roots {
		logger := log.New(os.Stderr, labels[i], log.LstdFlags)
		conn := discovery.Connection{}
		if v := strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL")); v != "" {
			conn.BaseURL = v
		} else if discovered, err := discovery.DiscoverConnection(root); err == nil {
			conn = discovered
		} else {
			logger.Printf("watch: daemon not running yet; starting with auto-discovery pending: %v", err)
		}
		logger.Printf("watch: root=%s daemon=%s interval=%s idle-timeout=%s", root, conn.BaseURL, interval, idleTimeout)

		watchers = append(watchers, &Watcher{
			Ctx:          ctx,
			Root:         root,
			BaseURL:      conn.BaseURL,
			Client:       session.NewDaemonClient(root, conn),
			UrlPinned:    pinned,
			Logger:       logger,
			Interval:     interval,
			IdleTimeout:  idleTimeout,
			Verification: vcache.New(root),
		})
	}

	tickAll := func() (allIdleExpired bool) {
		allIdleExpired = true
		for _, w := range watchers {
			if !w.Tick() {
				allIdleExpired = false
			}
		}
		return allIdleExpired
	}
	processLogger := log.New(os.Stderr, "", log.LstdFlags)
	exitLine := func() {
		processLogger.Printf("watch: all roots idle for ~%s; exiting (a path-triggered unit relaunches on the next agy/IDE activity)", idleTimeout)
	}

	if tickAll() {
		exitLine()
		return nil
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			processLogger.Printf("watch: shutdown signal received")
			return nil
		case <-t.C:
			if tickAll() {
				exitLine()
				return nil
			}
		}
	}
}

func RediscoverDaemonConnection(root, current string, logger *log.Logger) (discovery.Connection, bool) {
	next, err := discovery.DiscoverConnection(root)
	if err != nil {
		return discovery.Connection{}, false
	}
	if next.BaseURL == current {
		return next, false
	}
	if current == "" {
		logger.Printf("watch: agy daemon discovered at %s", next.BaseURL)
	} else {
		logger.Printf("watch: agy daemon moved %s -> %s", current, next.BaseURL)
	}
	return next, true
}

func WatchTick(
	ctx context.Context,
	client *daemon.Client,
	root string,
	logger *log.Logger,
	consecutiveFailures *int,
) (synced, skipped, upToDate, failed int, authBlocked bool) {
	return WatchTickPolicy(ctx, client, root, logger, consecutiveFailures, strings.TrimSpace(os.Getenv("ANTIGRAVITY_DAEMON_URL")) != "", nil)
}

func WatchTickPolicy(ctx context.Context, client *daemon.Client, root string, logger *log.Logger, consecutiveFailures *int, pinned bool, held RefusalMemo) (synced, skipped, upToDate, failed int, authBlocked bool) {
	return WatchTickCachedPolicy(ctx, client, root, logger, consecutiveFailures, pinned, held, nil)
}

func WatchTickCachedPolicy(ctx context.Context, client *daemon.Client, root string, logger *log.Logger, consecutiveFailures *int, pinned bool, held RefusalMemo, verified *vcache.Cache) (synced, skipped, upToDate, failed int, authBlocked bool) {
	if ctx.Err() != nil {
		return
	}
	sessions, err := discovery.ListConversationSessions(root)
	if err != nil {
		logger.Printf("watch: discovery error: %v", err)
		return
	}
	defer func() {
		if synced == 0 {
			return
		}
		if report, err := ReconcileParentLinks(filepath.Join(root, "conversations"), logger.Writer()); err != nil {
			logger.Printf("watch: parent-link resolution failed: %v", err)
		} else if report.Stamped > 0 {
			logger.Printf("watch: stamped %d parent link(s)", report.Stamped)
		}
	}()

	tickHadDaemonFailure := false
	tickHadSuccess := false
	tickContactedDaemon := false
	var answerProbe *error
	held.Prune(sessions)
	verified.Prune(sessions)

	for _, s := range sessions {
		if ctx.Err() != nil {
			return
		}
		if held.Holds(s, client.BaseURL) {
			skipped++
			continue
		}
		stale, reason := verified.IsStale(s, session.IsStale)
		if !stale {
			upToDate++
			continue
		}
		endpoint := client.BaseURL
		traj, err := session.FetchRecover(ctx, client, root, s, pinned)
		proxied := daemon.ClientProxied(client)
		if FetchContactedDaemon(err, proxied) {
			tickContactedDaemon = true
		}
		if err != nil {
			failed++
			if errors.Is(err, daemon.ErrAuthentication) {
				authBlocked = true
				logger.Printf("watch: authentication failed; sync blocked, retrying discovery next tick (%v)", err)
				if !tickContactedDaemon {
					if err := daemon.ReachesDaemon(ctx, client); err != nil {
						logger.Printf("watch: daemon not confirmed behind its proxy after the authentication failure (%v); retrying next tick", err)
						*consecutiveFailures++
						return
					}
				}
				*consecutiveFailures = 0
				return
			}
			down := IsConnRefused(err, proxied)
			if down && AnswerQuotesRefusal(err, proxied) {
				if answerProbe == nil {
					probeErr := daemon.RPCResponds(ctx, client)
					answerProbe = &probeErr
				}
				down = *answerProbe != nil
			}
			if down {
				tickHadDaemonFailure = true
				if *consecutiveFailures == 0 {
					if answerProbe != nil && *answerProbe != nil {
						logger.Printf("watch: daemon unreachable (%v; its listing RPC failed too: %v); retrying next tick", err, *answerProbe)
					} else {
						logger.Printf("watch: daemon unreachable (%v); retrying next tick", err)
					}
				}
				*consecutiveFailures++
				return
			}
			if digest, ok := HoldableRefusal(err); ok && held != nil && ctx.Err() == nil {
				held.Hold(s, endpoint, digest)
				logger.Printf("watch: skip %s (%v); not retrying until its source or the daemon endpoint changes", s.CascadeID, err)
			} else {
				logger.Printf("watch: skip %s (%v)", s.CascadeID, err)
			}
			skipped++
			continue
		}
		tickHadSuccess = true
		if err := cache.WriteVerified(s.SidecarPath, traj, s.ModTime); err != nil {
			logger.Printf("watch: write sidecar %s failed: %v", s.SidecarPath, err)
			failed++
			continue
		}
		size := SidecarSize(s.SidecarPath)
		logger.Printf("synced %s (%d steps, %dKB) [%s]%s", s.CascadeID, len(traj.Steps), size/1024, reason, session.VerifiedNote(traj))
		synced++
	}

	if skipped > 0 && !tickContactedDaemon {
		if err := daemon.ReachesDaemon(ctx, client); err != nil {
			logger.Printf("watch: daemon unreachable after a fetch that did not reach it (%v); retrying next tick", err)
			*consecutiveFailures++
			return
		}
	}
	if !tickHadDaemonFailure {
		if *consecutiveFailures > 0 && tickHadSuccess {
			logger.Printf("watch: daemon recovered after %d failed tick(s)", *consecutiveFailures)
		}
		*consecutiveFailures = 0
	}
	return
}

func FetchContactedDaemon(err error, proxied bool) bool {
	if err == nil {
		return true
	}
	var contentErr *session.CompletenessError
	if errors.As(err, &contentErr) {
		return !contentErr.BeforeRPC
	}
	return !proxied
}

func ReconcileParentLinks(dir string, logw io.Writer) (subagent.BackfillReport, error) {
	report, err := subagent.Backfill(dir, logw)
	if err != nil {
		return report, fmt.Errorf("resolve parent links in %s: %w", dir, err)
	}
	return report, nil
}

func SidecarSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func AnswerQuotesRefusal(err error, proxied bool) bool {
	if proxied {
		return false
	}
	var statusErr *daemon.StatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	var urlErr *url.Error
	var dnsErr *net.DNSError
	return !errors.As(err, &urlErr) && !errors.As(err, &dnsErr) && !errors.Is(err, syscall.ECONNREFUSED)
}

func IsConnRefused(err error, proxied bool) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var statusErr *daemon.StatusError
	if proxied && errors.As(err, &statusErr) {
		return false
	}

	s := err.Error()
	return strings.Contains(s, "connection refused") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "connect: connection")
}
