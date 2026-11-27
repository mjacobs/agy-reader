package watchlock

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/unix"
)

const rootSeals = unix.F_SEAL_SEAL | unix.F_SEAL_SHRINK | unix.F_SEAL_GROW | unix.F_SEAL_WRITE

// RecordRoot publishes the actual path the watcher polls for as long as
// the returned descriptor stays open. Keeping this in memory leaves the
// lock file and any symlink target untouched.
func RecordRoot(lock *os.File, root string) (*os.File, error) {
	path, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if len(path) > MaxRootBytes {
		return nil, fmt.Errorf("watch root exceeds %d bytes", MaxRootBytes)
	}
	name := OwnerName(strconv.FormatUint(uint64(lock.Fd()), 10))
	fd, err := unix.MemfdCreate(name, unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		return nil, err
	}
	record := os.NewFile(uintptr(fd), name)
	_, err = record.WriteString(path)
	if err == nil {
		_, err = unix.FcntlInt(record.Fd(), unix.F_ADD_SEALS, rootSeals)
	}
	if err != nil {
		_ = record.Close()
		return nil, err
	}
	return record, nil
}

// ReadRoot reads a sealed ownership record from a process's fd directory.
func ReadRoot(path string) string {
	record, err := os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return ""
	}
	defer record.Close()
	seals, err := unix.FcntlInt(record.Fd(), unix.F_GET_SEALS, 0)
	if err != nil || seals&rootSeals != rootSeals {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(record, MaxRootBytes+1))
	root := string(data)
	if err != nil || len(data) > MaxRootBytes || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return ""
	}
	return root
}
