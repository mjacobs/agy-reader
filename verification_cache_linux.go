//go:build linux

package main

import (
	"os"
	"strings"
	"syscall"
)

func verificationBootID() string {
	b, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func verificationStat(info os.FileInfo) (verificationFile, bool) {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return verificationFile{}, false
	}
	return verificationFile{Exists: true, Device: uint64(s.Dev), Inode: s.Ino, Size: info.Size(), Mtime: info.ModTime().UnixNano(), Ctime: int64(s.Ctim.Sec)*1e9 + int64(s.Ctim.Nsec)}, true
}
