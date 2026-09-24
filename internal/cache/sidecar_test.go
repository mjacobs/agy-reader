package cache_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/cache"
	"github.com/mjacobs/agy-reader/internal/daemon"
)

func TestSidecarRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abc.trajectory.json")

	in := &daemon.Trajectory{
		CascadeID: "abc",
		Source:    "CORTEX_TRAJECTORY_SOURCE_CLI",
		Steps: []daemon.Step{
			{
				Type:      "CORTEX_STEP_TYPE_USER_INPUT",
				Status:    "COMPLETED",
				UserInput: &daemon.UserInput{UserResponse: "hello world"},
			},
			{
				Type:   "CORTEX_STEP_TYPE_RUN_COMMAND",
				Status: "COMPLETED",
				RunCommand: &daemon.RunCommand{
					CommandLine:    "ls",
					Cwd:            "/tmp",
					CombinedOutput: []byte(`"a\nb\n"`),
				},
			},
		},
	}
	if cache.Exists(path) {
		t.Fatal("sidecar should not exist yet")
	}
	if err := cache.Write(path, in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !cache.Exists(path) {
		t.Fatal("sidecar should exist after write")
	}
	out, err := cache.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if out.CascadeID != "abc" || len(out.Steps) != 2 {
		t.Fatalf("unexpected trajectory: %+v", out)
	}
	if out.Steps[0].UserInput == nil || out.Steps[0].UserInput.UserResponse != "hello world" {
		t.Errorf("user input lost: %+v", out.Steps[0])
	}
	if out.Steps[1].RunCommand == nil || out.Steps[1].RunCommand.CommandLine != "ls" {
		t.Errorf("run command lost: %+v", out.Steps[1])
	}
}

func TestSidecarReadMissing(t *testing.T) {
	_, err := cache.Read(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got %v", err)
	}
}

func TestWriteNil(t *testing.T) {
	err := cache.Write(filepath.Join(t.TempDir(), "x.json"), nil)
	if err == nil {
		t.Fatal("expected error for nil")
	}
}

func TestWritePreservesRawUnknownFieldsAndLargeNumbers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raw.trajectory.json")
	raw := []byte(`{"cascadeId":"11111111-1111-1111-1111-111111111111","future":{"large":900719925474099312345678901234567890,"nested":{"unknown":true}},"steps":[]}`)

	traj := &daemon.Trajectory{
		CascadeID: "11111111-1111-1111-1111-111111111111",
		RawJSON:   raw,
	}
	if err := cache.Write(path, traj); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"future"`,
		`900719925474099312345678901234567890`,
		`"unknown":true`,
	} {
		if !bytes.Contains(got, []byte(want)) {
			t.Errorf("raw sidecar lost %s:\n%s", want, got)
		}
	}

	reread, err := cache.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Contains(reread.RawJSON, []byte(`900719925474099312345678901234567890`)) {
		t.Errorf("Read did not retain raw JSON: %s", reread.RawJSON)
	}
}

func TestWritePreservesExistingReaderMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "refresh.trajectory.json")
	const parent = "11111111-1111-1111-1111-111111111111"
	existing := []byte(`{"cascadeId":"22222222-2222-2222-2222-222222222222","agyReader":{"parentCascadeId":"` + parent + `","future":{"large":900719925474099312345678901234567890}},"steps":[]}`)
	if err := os.WriteFile(path, existing, 0o600); err != nil {
		t.Fatal(err)
	}
	fresh := []byte(`{"cascadeId":"22222222-2222-2222-2222-222222222222","daemonFuture":{"kept":true},"steps":[]}`)
	if err := cache.Write(path, &daemon.Trajectory{RawJSON: fresh}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"parentCascadeId"`, parent, `900719925474099312345678901234567890`, `"daemonFuture"`} {
		if !bytes.Contains(got, []byte(want)) {
			t.Errorf("refreshed sidecar lost %s:\n%s", want, got)
		}
	}
}

// A sidecar left unparseable by a crash or a truncated write holds no
// recoverable agyReader block, so a fresh daemon payload must be allowed to
// replace it. Failing instead would wedge the cache on that file forever.
func TestWriteOverwritesCorruptExistingSidecar(t *testing.T) {
	fresh := []byte(`{"cascadeId":"22222222-2222-2222-2222-222222222222","steps":[]}`)
	for _, tc := range []struct {
		name     string
		existing string
	}{
		{"truncated", `{"cascadeId":"2222222`},
		{"empty", ``},
		{"not an object", `["nope"]`},
		{"json null", `null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "corrupt.trajectory.json")
			if err := os.WriteFile(path, []byte(tc.existing), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := cache.Write(path, &daemon.Trajectory{RawJSON: fresh}); err != nil {
				t.Fatalf("Write over %s sidecar: %v", tc.name, err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(got, []byte(`"cascadeId"`)) {
				t.Errorf("fresh payload did not replace %s sidecar:\n%s", tc.name, got)
			}
		})
	}
}

func TestWriteRestrictsExistingSidecarEvenWhenBytesMatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "same.trajectory.json")
	raw := []byte("{\"cascadeId\":\"11111111-1111-1111-1111-111111111111\",\"steps\":[]}\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cache.Write(path, &daemon.Trajectory{RawJSON: raw}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestWriteAdvancesMtimeWhenContentMatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "same.trajectory.json")
	raw := []byte("{\"cascadeId\":\"11111111-1111-1111-1111-111111111111\",\"steps\":[]}\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := cache.Write(path, &daemon.Trajectory{RawJSON: raw}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().After(oldTime) {
		t.Fatalf("mtime not advanced: got %v, want > %v", info.ModTime(), oldTime)
	}
}

func TestStampParentCascadeIDPreservesReaderFieldsAndIsIdempotent(t *testing.T) {
	const (
		oldParent = "11111111-1111-1111-1111-111111111111"
		newParent = "22222222-2222-2222-2222-222222222222"
	)
	dir := t.TempDir()
	path := filepath.Join(dir, "child.trajectory.json")
	original := []byte(`{
  "cascadeId": "33333333-3333-3333-3333-333333333333",
  "futureNumber": 900719925474099312345678901234567890,
  "futureObject": {"kept": [1, 2, 3]},
  "agyReader": {"parentCascadeId": "` + oldParent + `", "futureReaderField": {"kept": true}},
  "steps": []
}`)
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}

	changed, err := cache.StampParentCascadeID(path, newParent)
	if err != nil {
		t.Fatalf("StampParentCascadeID: %v", err)
	}
	if !changed {
		t.Fatal("first stamp should report a change")
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		newParent,
		`900719925474099312345678901234567890`,
		`"futureReaderField"`,
		`"futureObject"`,
	} {
		if !strings.Contains(string(first), want) {
			t.Errorf("stamped sidecar lost %q:\n%s", want, first)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
	mtime := info.ModTime()

	// Give a rewrite enough time to produce an observably different mtime.
	time.Sleep(10 * time.Millisecond)
	changed, err = cache.StampParentCascadeID(path, newParent)
	if err != nil {
		t.Fatalf("second StampParentCascadeID: %v", err)
	}
	if changed {
		t.Fatal("same stamp should be a no-op")
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("idempotent stamp changed file bytes")
	}
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(mtime) {
		t.Errorf("idempotent stamp changed mtime: %s -> %s", mtime, info.ModTime())
	}
}

func TestStampParentCascadeIDTreatsUUIDCaseAsEquivalent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "child.trajectory.json")
	const upper = "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA"
	original := []byte(`{"cascadeId":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","agyReader":{"parentCascadeId":"` + upper + `"},"steps":[]}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	mtime := info.ModTime()

	changed, err := cache.StampParentCascadeID(path, strings.ToLower(upper))
	if err != nil {
		t.Fatalf("StampParentCascadeID: %v", err)
	}
	if changed {
		t.Fatal("case-only equivalent stamp should be a no-op")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatal("case-only equivalent stamp changed bytes")
	}
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(mtime) {
		t.Fatal("case-only equivalent stamp changed mtime")
	}
}

func TestClearParentCascadeIDPreservesPayloadAndReaderSiblings(t *testing.T) {
	for _, sibling := range []string{"", `,"futureReaderField":{"kept":true}`} {
		t.Run(sibling, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sidecar.trajectory.json")
			original := `{"cascadeId":"33333333-3333-3333-3333-333333333333","futureNumber":900719925474099312345678901234567890,"futureObject":{"kept":[1,2,3]},"agyReader":{"parentCascadeId":"11111111-1111-1111-1111-111111111111"` + sibling + `},"steps":[]}`
			if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
				t.Fatal(err)
			}
			changed, err := cache.ClearParentCascadeID(path)
			if err != nil || !changed {
				t.Fatalf("clear: %v, %v", changed, err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "parentCascadeId") {
				t.Fatal("parent remains")
			}
			if strings.Contains(string(data), "agyReader") != (sibling != "") {
				t.Fatal("wrong reader namespace retention")
			}
			for _, value := range []string{"900719925474099312345678901234567890", "futureObject", "kept"} {
				if !strings.Contains(string(data), value) {
					t.Fatalf("lost payload %s", value)
				}
			}
			if sibling != "" && !strings.Contains(string(data), "futureReaderField") {
				t.Fatal("lost reader sibling")
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Fatal("sidecar is not owner-only")
			}
			changed, err = cache.ClearParentCascadeID(path)
			if err != nil || changed {
				t.Fatalf("second clear: %v, %v", changed, err)
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(data, after) {
				t.Fatal("no-op changed bytes")
			}
		})
	}
}

// A content-changing write must land the verification stamp, not the rename's
// wall clock. Watch mode passes the source mtime it observed *before* fetching;
// if the rewritten sidecar kept the (newer) rename time, a source write that
// landed during the fetch would look older than the sidecar and the next tick
// would consider the session fresh, hiding the missing turn.
func TestWriteVerifiedStampsRewrittenSidecarWithSourceMtime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rewritten.trajectory.json")
	source := filepath.Join(dir, "rewritten.pb")
	if err := os.WriteFile(path, []byte("{\"cascadeId\":\"11111111-1111-1111-1111-111111111111\",\"steps\":[]}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The fetch observed this source mtime before it started.
	verifiedAt := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(source, verifiedAt, verifiedAt); err != nil {
		t.Fatal(err)
	}

	// The source is written again while the fetch is in flight, so its mtime
	// now sits between verifiedAt and the write below.
	duringFetch := verifiedAt.Add(30 * time.Minute)
	if err := os.WriteFile(source, []byte("new turn"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(source, duringFetch, duringFetch); err != nil {
		t.Fatal(err)
	}

	fresh := []byte("{\"cascadeId\":\"11111111-1111-1111-1111-111111111111\",\"steps\":[{\"n\":1}]}\n")
	if err := cache.WriteVerified(path, &daemon.Trajectory{RawJSON: fresh}, verifiedAt); err != nil {
		t.Fatalf("WriteVerified: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(verifiedAt) {
		t.Fatalf("sidecar mtime = %v, want the verification stamp %v", info.ModTime(), verifiedAt)
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	if !sourceInfo.ModTime().After(info.ModTime()) {
		t.Fatalf("source (%v) is not newer than sidecar (%v): the next tick would not refetch",
			sourceInfo.ModTime(), info.ModTime())
	}
}

// A permission-only repair is not a content rewrite: the file keeps its mtime,
// so the forward-only guard must still apply and an already-newer stamp must
// not be walked backwards.
func TestWriteVerifiedDoesNotRewindMtimeForPermissionOnlyRepair(t *testing.T) {
	path := filepath.Join(t.TempDir(), "same.trajectory.json")
	raw := []byte("{\"cascadeId\":\"11111111-1111-1111-1111-111111111111\",\"steps\":[]}\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	// os.WriteFile's mode is masked by the process umask, so a restrictive
	// umask would create the file at 0600 and the test would never reach the
	// permission-repair path it exists to cover.
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o644 {
		t.Fatalf("precondition: mode = %o, want 644", info.Mode().Perm())
	}
	newer := time.Now().Truncate(time.Second)
	if err := os.Chtimes(path, newer, newer); err != nil {
		t.Fatal(err)
	}
	older := newer.Add(-1 * time.Hour)

	if err := cache.WriteVerified(path, &daemon.Trajectory{RawJSON: raw}, older); err != nil {
		t.Fatalf("WriteVerified: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
	if !info.ModTime().Equal(newer) {
		t.Fatalf("mtime = %v, want the existing newer mtime %v", info.ModTime(), newer)
	}
}
