package cache

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAtomicReplacementCommitsTimestampWithContent(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "timestamp failure"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "sidecar.json")
			oldTime := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
			verifiedAt := oldTime.Add(time.Hour)
			if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(path, oldTime, oldTime); err != nil {
				t.Fatal(err)
			}
			stampErr := errors.New("timestamp unavailable")
			called := false
			stamp := func(temp string, atime, mtime time.Time) error {
				called = true
				data, err := os.ReadFile(path)
				if err != nil || string(data) != "old" {
					t.Fatalf("destination replaced before timestamp: %q, %v", data, err)
				}
				if temp == path || filepath.Dir(temp) != dir || !mtime.Equal(verifiedAt) {
					t.Fatalf("stamp target=%s mtime=%v", temp, mtime)
				}
				if fail {
					return stampErr
				}
				return os.Chtimes(temp, atime, mtime)
			}
			changed, rewrote, err := writeAtomicWithStamp(path, []byte("new"), verifiedAt, stamp)
			if !called {
				t.Fatal("timestamp was not applied")
			}
			want, wantTime := "new", verifiedAt
			if fail {
				want, wantTime = "old", oldTime
				if changed || rewrote || !errors.Is(err, stampErr) {
					t.Fatalf("failure: changed=%v rewrote=%v err=%v", changed, rewrote, err)
				}
			} else if !changed || !rewrote || err != nil {
				t.Fatalf("success: changed=%v rewrote=%v err=%v", changed, rewrote, err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != want {
				t.Fatalf("content=%q want=%q err=%v", data, want, err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !info.ModTime().Equal(wantTime) {
				t.Fatalf("mtime=%v want=%v", info.ModTime(), wantTime)
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 1 {
				t.Fatalf("temporary file leaked: %v, %v", files, err)
			}
		})
	}
}
