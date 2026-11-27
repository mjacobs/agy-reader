package watchlock

// OwnerName binds a process-local record of the polling root to the descriptor
// that holds the flock. Linux exposes memfd names through /proc/<pid>/fd.
func OwnerName(lockFD string) string {
	return "agy-reader-watch-root-" + lockFD
}

const MaxRootBytes = 4096
