package completeness

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const syntheticRaw = `{"cascadeId":"11111111-1111-4111-8111-111111111111","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","generic":{"result":{"result":"outer-only result","fullOutputUri":"scratch://output","payload":{"@type":"type.googleapis.com/gemini_coder.Step","type":"CORTEX_STEP_TYPE_RUN_COMMAND","runCommand":{"commandLine":"echo fixture"}}}}}]}`
const syntheticPresentation = `{"cascadeId":"11111111-1111-4111-8111-111111111111","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","runCommand":{"commandLine":"echo fixture"}}]}`

func fixture(t *testing.T) (*Snapshot, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "source.db")
	testfixture.Source(t, p, []byte(syntheticRaw))
	s, err := ReadSnapshot(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	return s, p
}
func TestProjectionRequiresBothRepresentations(t *testing.T) {
	s, _ := fixture(t)
	if _, r := s.VerifyResponses([]byte(syntheticRaw), []byte(syntheticPresentation)); r.State != Verified {
		t.Fatal(r)
	}
	for name, pair := range map[string][2]string{
		"raw outer-only change":  {string(bytes.ReplaceAll([]byte(syntheticRaw), []byte("outer-only result"), []byte("stale result"))), syntheticPresentation},
		"display command change": {syntheticRaw, string(bytes.ReplaceAll([]byte(syntheticPresentation), []byte("echo fixture"), []byte("stale command")))},
		"both missing step":      {`{"cascadeId":"11111111-1111-4111-8111-111111111111","metadata":{}}`, `{"cascadeId":"11111111-1111-4111-8111-111111111111","metadata":{}}`},
	} {
		t.Run(name, func(t *testing.T) {
			if _, r := s.VerifyResponses([]byte(pair[0]), []byte(pair[1])); r.State != Incomplete {
				t.Fatal(r)
			}
		})
	}
}
func TestRPCExtrasEnumsAndAttestation(t *testing.T) {
	s, _ := fixture(t)
	add := func(raw string) []byte {
		var v map[string]json.RawMessage
		json.Unmarshal([]byte(raw), &v)
		v["future"] = json.RawMessage(`{"n":9007199254740993,"nested":{"a":1,"b":2}}`)
		b, _ := json.Marshal(v)
		return b
	}
	raw, pres := add(syntheticRaw), add(syntheticPresentation)
	a, r := s.VerifyResponses(raw, pres)
	if r.State != Verified {
		t.Fatal(r)
	}
	reordered := bytes.ReplaceAll(pres, []byte(`"a":1,"b":2`), []byte(`"b":2,"a":1`))
	if r = s.CheckAttestation(reordered, a); r.State != Verified {
		t.Fatal("object order", r)
	}
	changed := bytes.ReplaceAll(pres, []byte("9007199254740993"), []byte("9007199254740992"))
	if r = s.CheckAttestation(changed, a); r.State != Incomplete {
		t.Fatal("precision", r)
	}
	if r = s.CompareFullRaw(bytes.Replace(raw, []byte(`"cascadeId"`), []byte(`"trajectoryType":"FUTURE_ENUM","cascadeId"`), 1)); r.State != Unknown {
		t.Fatal("top enum", r)
	}
	if r = s.CompareFullRaw(bytes.Replace(raw, []byte(`"commandLine"`), []byte(`"commandLine"`), 1)); r.State != Verified {
		t.Fatal(r)
	}
	unknown := bytes.Replace(raw, []byte(`"runCommand"`), []byte(`"status":"FUTURE_STATUS","runCommand"`), 1)
	if r = s.CompareFullRaw(unknown); r.State != Unknown {
		t.Fatal("Any enum", r)
	}
	var top map[string]json.RawMessage
	json.Unmarshal(pres, &top)
	top["agyReader"] = json.RawMessage(`{"parentCascadeId":"22222222-2222-4222-8222-222222222222","future":7}`)
	parent, _ := json.Marshal(top)
	if r = s.CheckAttestation(parent, a); r.State != Verified {
		t.Fatal("reader namespace", r)
	}
	a.Profile = "future"
	if r = s.CheckAttestation(pres, a); r.State != Unknown {
		t.Fatal("profile", r)
	}
}
func TestSourceUnknownAndWAL(t *testing.T) {
	for name, q := range map[string]string{"version": "PRAGMA user_version=2", "format": "UPDATE steps SET step_format=1", "generated column": "ALTER TABLE steps ADD COLUMN future TEXT GENERATED ALWAYS AS ('unknown') VIRTUAL", "gap": "UPDATE steps SET idx=2", "header": "UPDATE steps SET status=123456", "render": "UPDATE steps SET render_info=X'01'", "subtrajectory": "UPDATE steps SET has_subtrajectory=1"} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "s.db")
			db := testfixture.Source(t, p, []byte(syntheticRaw))
			testfixture.Exec(t, db, q)
			if _, err := ReadSnapshot(t.Context(), p); err == nil {
				t.Fatal("unsupported source accepted")
			}
		})
	}
	t.Run("unknown embedded Any", func(t *testing.T) {
		s, _ := fixture(t)
		m := proto.Clone(s.trajectory).ProtoReflect()
		step := m.Mutable(m.Descriptor().Fields().ByName("steps")).List().Get(0).Message()
		g := step.Get(step.Descriptor().Fields().ByName("generic")).Message()
		r := g.Get(g.Descriptor().Fields().ByName("result")).Message()
		a := r.Get(r.Descriptor().Fields().ByName("payload")).Message()
		f := a.Descriptor().Fields().ByName("value")
		b := protowire.AppendVarint(protowire.AppendTag(append([]byte(nil), a.Get(f).Bytes()...), 50000, protowire.VarintType), 1)
		a.Set(f, protoreflect.ValueOfBytes(b))
		if _, err := finishSnapshot(m.Interface()); err == nil {
			t.Fatal("unknown Any field accepted")
		}
	})
	t.Run("committed WAL and checkpoint", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "s.db")
		db := testfixture.Source(t, p, []byte(syntheticRaw))
		testfixture.Exec(t, db, "PRAGMA journal_mode=WAL;PRAGMA wal_autocheckpoint=0")
		before, err := ReadSnapshot(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec("UPDATE trajectory_meta SET trajectory_id='committed-change'"); err != nil {
			t.Fatal(err)
		}
		during, err := ReadSnapshot(t.Context(), p)
		if err != nil {
			t.Fatal(err)
		}
		if before.Digest != during.Digest {
			t.Fatal("uncommitted change visible")
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
		after, err := ReadSnapshot(t.Context(), p)
		if err != nil {
			t.Fatal(err)
		}
		if after.Digest == before.Digest {
			t.Fatal("committed change hidden")
		}
		testfixture.Exec(t, db, "PRAGMA wal_checkpoint(TRUNCATE)")
		checkpoint, err := ReadSnapshot(t.Context(), p)
		if err != nil || checkpoint.Digest != after.Digest {
			t.Fatal("checkpoint changed content", err)
		}
	})
}

func TestSQLiteSourceURI(t *testing.T) {
	for _, tc := range []struct{ name, goos, path, want string }{
		{"unix", "linux", "/tmp/source #?.db", "file:///tmp/source%20%23%3F.db"},
		{"windows drive", "windows", `C:\Users\user-a\source #?.db`, "file:///C:/Users/user-a/source%20%23%3F.db"},
		{"windows UNC", "windows", `\\host-a.example\share\source.db`, "file:////host-a.example/share/source.db"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uri := sqliteSourceURI(tc.path, tc.goos)
			if uri.String() != tc.want || uri.Host != "" {
				t.Fatalf("got %s, want %s", uri.String(), tc.want)
			}
		})
	}
}

func TestAttestationRejectsTrailingJSON(t *testing.T) {
	s, _ := fixture(t)
	a, result := s.VerifyResponses([]byte(syntheticRaw), []byte(syntheticPresentation))
	if result.State != Verified {
		t.Fatal(result)
	}
	for _, suffix := range []string{` {}`, ` null`, ` junk`} {
		if result := s.CheckAttestation([]byte(syntheticPresentation+suffix), a); result.State != Unknown {
			t.Fatal(result)
		}
	}
}

func TestContribVariantMetadata(t *testing.T) {
	rawJSON := `{"cascadeId":"11111111-1111-4111-8111-111111111111","metadata":{},"executorMetadatas":[{"agentConfigResolution":{"requestedAgentSpec":{},"appliedContribVariant":"v1","contribVariantMetadata":{"assignedVariant":"v1","eligibleVariants":["v1","v2"],"appliedVariant":"v1"}}}],"steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","runCommand":{"commandLine":"echo test"}}]}`
	presJSON := `{"cascadeId":"11111111-1111-4111-8111-111111111111","metadata":{},"executorMetadatas":[{"agentConfigResolution":{"requestedAgentSpec":{},"appliedContribVariant":"v1","contribVariantMetadata":{"assignedVariant":"v1","eligibleVariants":["v1","v2"],"appliedVariant":"v1"}}}],"steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","runCommand":{"commandLine":"echo test"}}]}`

	p := filepath.Join(t.TempDir(), "contrib.db")
	testfixture.Source(t, p, []byte(rawJSON))
	s, err := ReadSnapshot(t.Context(), p)
	if err != nil {
		t.Fatalf("ReadSnapshot failed: %v", err)
	}

	// Positive test: verified when raw and presentation match with ContribVariantMetadata
	if _, r := s.VerifyResponses([]byte(rawJSON), []byte(presJSON)); r.State != Verified {
		t.Fatalf("expected Verified, got %+v", r)
	}

	// Negative test 1: raw RPC differs in contribVariantMetadata -> Incomplete
	mismatchedRaw := bytes.ReplaceAll([]byte(rawJSON), []byte(`"appliedVariant":"v1"`), []byte(`"appliedVariant":"v2"`))
	if _, r := s.VerifyResponses(mismatchedRaw, []byte(presJSON)); r.State != Incomplete {
		t.Fatalf("expected Incomplete for differing contribVariantMetadata, got %+v", r)
	}

	// Negative test 2: unknown persisted field inside ContribVariantMetadata -> ReadSnapshot error
	badCVM := protowire.AppendVarint(protowire.AppendTag(nil, 99999, protowire.VarintType), 1)
	badACR := protowire.AppendBytes(protowire.AppendTag(nil, 3, protowire.BytesType), badCVM)
	badEM := protowire.AppendBytes(protowire.AppendTag(nil, 16, protowire.BytesType), badACR)
	pBad := filepath.Join(t.TempDir(), "bad_cvm.db")
	db := testfixture.Source(t, pBad, []byte(rawJSON))
	testfixture.Exec(t, db, "UPDATE executor_metadata SET data=? WHERE idx=0", badEM)
	if _, err := ReadSnapshot(t.Context(), pBad); err == nil {
		t.Fatal("expected ReadSnapshot to reject unknown persisted field in ContribVariantMetadata")
	}

	// Negative test 3: raw RPC has invalid structure (string instead of message) -> Unknown ("invalid RPC message JSON")
	badRPCRaw := bytes.ReplaceAll([]byte(rawJSON), []byte(`"contribVariantMetadata":{"assignedVariant":"v1","eligibleVariants":["v1","v2"],"appliedVariant":"v1"}`), []byte(`"contribVariantMetadata":"invalid"`))
	if !json.Valid(badRPCRaw) {
		t.Fatal("expected badRPCRaw to be valid JSON")
	}
	if _, r := s.VerifyResponses(badRPCRaw, []byte(presJSON)); r.State != Unknown || r.Reason != "invalid RPC message JSON" {
		t.Fatalf("expected Unknown with 'invalid RPC message JSON', got %+v", r)
	}
}

func TestGenericResultMedia(t *testing.T) {
	rawJSON := `{"cascadeId":"11111111-1111-4111-8111-111111111111","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","generic":{"args":{"AbsolutePath":"/test/image.webp"},"result":{"result":"image displayed","media":[{"mimeType":"image/webp","uri":"file:///test/media.webp"}],"payload":{"@type":"type.googleapis.com/gemini_coder.Step","type":"CORTEX_STEP_TYPE_VIEW_FILE","status":"CORTEX_STEP_STATUS_DONE","viewFile":{"absolutePathUri":"file:///test/image.webp","mediaData":{"mimeType":"image/webp","uri":"file:///test/media.webp"}}}}}}]}`
	presJSON := `{"cascadeId":"11111111-1111-4111-8111-111111111111","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_VIEW_FILE","status":"CORTEX_STEP_STATUS_DONE","viewFile":{"absolutePathUri":"file:///test/image.webp","mediaData":{"mimeType":"image/webp","uri":"file:///test/media.webp"}}}]}`

	p := filepath.Join(t.TempDir(), "media.db")
	testfixture.Source(t, p, []byte(rawJSON))
	s, err := ReadSnapshot(t.Context(), p)
	if err != nil {
		t.Fatalf("ReadSnapshot failed: %v", err)
	}

	// Positive test: verified when raw has generic.result.media and presentation has projected viewFile
	if _, r := s.VerifyResponses([]byte(rawJSON), []byte(presJSON)); r.State != Verified {
		t.Fatalf("expected Verified, got %+v", r)
	}

	// Negative test 1a: wrapper-only generic.result.media mutation (leaving embedded mediaData and ordinary RPC unchanged) -> Incomplete
	wrapperMutatedRaw := bytes.Replace([]byte(rawJSON), []byte(`"mimeType":"image/webp"`), []byte(`"mimeType":"image/png"`), 1)
	if _, r := s.VerifyResponses(wrapperMutatedRaw, []byte(presJSON)); r.State != Incomplete {
		t.Fatalf("expected Incomplete for wrapper-only media mutation, got %+v", r)
	}

	// Negative test 1b: wrapper-only generic.result.media removal (leaving embedded mediaData and ordinary RPC unchanged) -> Incomplete
	wrapperRemovedRaw := bytes.ReplaceAll([]byte(rawJSON), []byte(`"media":[{"mimeType":"image/webp","uri":"file:///test/media.webp"}],`), nil)
	if _, r := s.VerifyResponses(wrapperRemovedRaw, []byte(presJSON)); r.State != Incomplete {
		t.Fatalf("expected Incomplete for wrapper-only media removal, got %+v", r)
	}

	// Negative test 1c: alter ordinary viewFile.mediaData (leaving raw RPC unchanged) -> Incomplete
	ordinaryAlteredPres := bytes.ReplaceAll([]byte(presJSON), []byte(`"file:///test/media.webp"`), []byte(`"file:///test/altered.webp"`))
	if _, r := s.VerifyResponses([]byte(rawJSON), ordinaryAlteredPres); r.State != Incomplete {
		t.Fatalf("expected Incomplete for ordinary viewFile.mediaData alteration, got %+v", r)
	}

	// Negative test 2: generic.result has unsupported field -> Unknown
	badResultRaw := bytes.ReplaceAll([]byte(rawJSON), []byte(`"media":`), []byte(`"metadata":{"unsupported":"value"},"media":`))
	pBad := filepath.Join(t.TempDir(), "bad_result.db")
	testfixture.Source(t, pBad, badResultRaw)
	sBad, err := ReadSnapshot(t.Context(), pBad)
	if err != nil {
		t.Fatalf("ReadSnapshot failed: %v", err)
	}
	if _, r := sBad.VerifyResponses(badResultRaw, []byte(presJSON)); r.State != Unknown || r.Reason != "unsupported generic result" {
		t.Fatalf("expected Unknown (unsupported generic result), got %+v", r)
	}
}

func TestBackgroundTaskDetails(t *testing.T) {
	rawJSON := `{"cascadeId":"11111111-1111-4111-8111-111111111111","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","taskDetails":{"id":"task-1","logUri":"file:///task-1.log","description":"bg command","title":"bg title"},"generic":{"args":{"CommandLine":"sleep 10","Cwd":"/tmp"},"result":{"result":"started","payload":{"@type":"type.googleapis.com/gemini_coder.Step","type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_DONE","runCommand":{"commandLine":"sleep 10"}}}}}]}`
	presJSON := `{"cascadeId":"11111111-1111-4111-8111-111111111111","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","status":"CORTEX_STEP_STATUS_DONE","taskDetails":{"id":"task-1","logUri":"file:///task-1.log","description":"bg command","title":"bg title"},"runCommand":{"commandLine":"sleep 10"}}]}`

	p := filepath.Join(t.TempDir(), "task.db")
	testfixture.Source(t, p, []byte(rawJSON))
	s, err := ReadSnapshot(t.Context(), p)
	if err != nil {
		t.Fatalf("ReadSnapshot failed: %v", err)
	}

	// Positive test 1: verified when wrapper has taskDetails and presentation preserves it (embedded has none)
	if _, r := s.VerifyResponses([]byte(rawJSON), []byte(presJSON)); r.State != Verified {
		t.Fatalf("expected Verified, got %+v", r)
	}

	// Positive test 2: verified when outer wrapper and embedded payload have equal taskDetails
	rawEqualTask := bytes.ReplaceAll([]byte(rawJSON), []byte(`"runCommand":{"commandLine":"sleep 10"}`), []byte(`"taskDetails":{"id":"task-1","logUri":"file:///task-1.log","description":"bg command","title":"bg title"},"runCommand":{"commandLine":"sleep 10"}`))
	pEqual := filepath.Join(t.TempDir(), "equal_task.db")
	testfixture.Source(t, pEqual, rawEqualTask)
	sEqual, err := ReadSnapshot(t.Context(), pEqual)
	if err != nil {
		t.Fatalf("ReadSnapshot failed: %v", err)
	}
	if _, r := sEqual.VerifyResponses(rawEqualTask, []byte(presJSON)); r.State != Verified {
		t.Fatalf("expected Verified for equal outer/embedded taskDetails, got %+v", r)
	}

	// Negative test 1: presentation missing taskDetails -> Incomplete
	presMissingTask := bytes.ReplaceAll([]byte(presJSON), []byte(`"taskDetails":{"id":"task-1","logUri":"file:///task-1.log","description":"bg command","title":"bg title"},`), nil)
	if _, r := s.VerifyResponses([]byte(rawJSON), presMissingTask); r.State != Incomplete {
		t.Fatalf("expected Incomplete when presentation lacks taskDetails, got %+v", r)
	}

	// Negative test 2: presentation has differing taskDetails -> Incomplete
	presDifferentTask := bytes.ReplaceAll([]byte(presJSON), []byte("task-1"), []byte("task-2"))
	if _, r := s.VerifyResponses([]byte(rawJSON), presDifferentTask); r.State != Incomplete {
		t.Fatalf("expected Incomplete when presentation taskDetails differs, got %+v", r)
	}

	// Negative test 3: inner native step has conflicting taskDetails -> Unknown
	rawConflictingInner := bytes.ReplaceAll([]byte(rawJSON), []byte(`"runCommand":{"commandLine":"sleep 10"}`), []byte(`"taskDetails":{"id":"task-diff"},"runCommand":{"commandLine":"sleep 10"}`))
	pConflict := filepath.Join(t.TempDir(), "conflict.db")
	testfixture.Source(t, pConflict, rawConflictingInner)
	sConflict, err := ReadSnapshot(t.Context(), pConflict)
	if err != nil {
		t.Fatalf("ReadSnapshot failed: %v", err)
	}
	if _, r := sConflict.VerifyResponses(rawConflictingInner, []byte(presJSON)); r.State != Unknown || r.Reason != "unsupported wrapper/native task_details difference" {
		t.Fatalf("expected Unknown (unsupported wrapper/native task_details difference), got %+v", r)
	}

	// Negative test 4: inner native step has taskDetails while outer wrapper does not -> Unknown
	rawInnerOnly := bytes.ReplaceAll([]byte(rawJSON), []byte(`"taskDetails":{"id":"task-1","logUri":"file:///task-1.log","description":"bg command","title":"bg title"},`), nil)
	rawInnerOnly = bytes.ReplaceAll(rawInnerOnly, []byte(`"runCommand":{"commandLine":"sleep 10"}`), []byte(`"taskDetails":{"id":"task-1","logUri":"file:///task-1.log","description":"bg command","title":"bg title"},"runCommand":{"commandLine":"sleep 10"}`))
	pInnerOnly := filepath.Join(t.TempDir(), "inner_only.db")
	testfixture.Source(t, pInnerOnly, rawInnerOnly)
	sInnerOnly, err := ReadSnapshot(t.Context(), pInnerOnly)
	if err != nil {
		t.Fatalf("ReadSnapshot failed: %v", err)
	}
	if _, r := sInnerOnly.VerifyResponses(rawInnerOnly, []byte(presJSON)); r.State != Unknown || r.Reason != "unsupported wrapper/native task_details difference" {
		t.Fatalf("expected Unknown (unsupported wrapper/native task_details difference), got %+v", r)
	}

	// Negative test 5: outer step has unsupported field (e.g. errorDetails on wrapper) -> Unknown
	rawUnsupportedOuter := bytes.ReplaceAll([]byte(rawJSON), []byte(`"taskDetails":`), []byte(`"error":{"shortError":"err"},"taskDetails":`))
	pUnsupported := filepath.Join(t.TempDir(), "unsupported_outer.db")
	testfixture.Source(t, pUnsupported, rawUnsupportedOuter)
	sUnsupported, err := ReadSnapshot(t.Context(), pUnsupported)
	if err != nil {
		t.Fatalf("ReadSnapshot failed: %v", err)
	}
	if _, r := sUnsupported.VerifyResponses(rawUnsupportedOuter, []byte(presJSON)); r.State != Unknown || r.Reason != "unsupported generic wrapper" {
		t.Fatalf("expected Unknown (unsupported generic wrapper), got %+v", r)
	}
}
