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
