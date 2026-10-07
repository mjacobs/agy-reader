//go:build !linux

package main

import "os"

// Until equally strong file identity/change metadata and a boot identifier are
// available on this platform, retain uncached content verification.
func verificationBootID() string                            { return "" }
func verificationStat(os.FileInfo) (verificationFile, bool) { return verificationFile{}, false }
