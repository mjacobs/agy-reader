//go:build !linux

package vcache

import "os"

// Until equally strong file identity/change metadata and a boot identifier are
// available on this platform, retain uncached content verification.
func BootID() string                            { return "" }
func verificationStat(os.FileInfo) (File, bool) { return File{}, false }
