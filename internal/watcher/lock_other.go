//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !windows

package watcher

import (
	"errors"
	"os"
)

func tryWatchLock(_ *os.File) (bool, error) {
	return false, errors.New("watcher ownership locks are unsupported on this platform")
}
