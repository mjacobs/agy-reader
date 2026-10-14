//go:build linux

package vcache

import (
	"os"
	"strings"
	"syscall"
)

func BootID() string {
	b, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func verificationStat(info os.FileInfo) (File, bool) {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return File{}, false
	}
	return File{Exists: true, Device: uint64(s.Dev), Inode: s.Ino, Size: info.Size(), Mtime: info.ModTime().UnixNano(), Ctime: int64(s.Ctim.Sec)*1e9 + int64(s.Ctim.Nsec)}, true
}
