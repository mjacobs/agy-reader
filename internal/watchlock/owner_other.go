//go:build !linux

package watchlock

import "os"

func RecordRoot(_ *os.File, _ string) (*os.File, error) {
	return nil, nil
}

func ReadRoot(_ string) string { return "" }
