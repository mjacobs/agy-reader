package vcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mjacobs/agy-reader/internal/discovery"
)

func TestDarwinFingerprintWithoutVerificationReuse(t *testing.T) {
	root := t.TempDir()
	s := discovery.Session{PBPath: filepath.Join(root, "session.db"), SidecarPath: filepath.Join(root, "session.trajectory.json")}
	for _, path := range []string{s.PBPath, s.SidecarPath} {
		if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	before, ok := SourceFingerprint(s.PBPath)
	if !ok || !before.Source.Exists || before.Source.Inode == 0 || before.Source.Ctime == 0 {
		t.Fatalf("missing file identity: ok=%v files=%+v", ok, before)
	}
	cache := New(root)
	calls := 0
	check := func(discovery.Session) (bool, string) { calls++; return false, "verified" }
	cache.IsStale(s, check)
	cache.IsStale(s, check)
	if calls != 2 {
		t.Fatalf("verification reused without a boot identifier: %d checks", calls)
	}
	info, err := os.Stat(s.PBPath)
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(root, "replacement")
	if err := os.WriteFile(replacement, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(replacement, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, s.PBPath); err != nil {
		t.Fatal(err)
	}
	if after, ok := SourceFingerprint(s.PBPath); !ok || after == before {
		t.Fatalf("same-size replacement with restored mtime went unnoticed: ok=%v files=%+v", ok, after)
	}
}
