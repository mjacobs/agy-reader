//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package watcher

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func tryWatchLock(f *os.File) (bool, error) {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return false, nil
	}
	return err == nil, err
}
