package watcher

import (
	"os"
	"path/filepath"
)

// Keep this file in the store, rather than a user cache or runtime directory:
// watchers with different HOME/XDG settings must contend on the same inode.
// Never unlink it, even on exit, since a replacement inode would let a third
// watcher acquire a second lock while another process still holds the first.
const watchLockName = ".agy-reader-watch.lock"

// acquireWatchLock returns nil, nil when another process owns the store.
// Ownership lasts until the returned file is closed or the process exits.
// A per-tick lock would still let different verification profiles repeatedly
// replace each other's sidecars between ticks.
func acquireWatchLock(root string) (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(root, watchLockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	locked, err := tryWatchLock(f)
	if err != nil || !locked {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}
