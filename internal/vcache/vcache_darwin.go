package vcache

import (
	"os"
	"syscall"
)

// File identity supports in-process refusal backoffs. Without a boot identifier,
// the persisted verification cache remains disabled on macOS.
func BootID() string { return "" }

func verificationStat(info os.FileInfo) (File, bool) {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return File{}, false
	}
	return File{Exists: true, Device: uint64(s.Dev), Inode: s.Ino, Size: info.Size(), Mtime: info.ModTime().UnixNano(), Ctime: s.Ctimespec.Sec*1e9 + s.Ctimespec.Nsec}, true
}
