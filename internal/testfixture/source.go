// Package testfixture builds disposable SQLite sources from synthetic protobuf
// JSON. Only tests import it; it never reads profiles or contacts a daemon.
package testfixture

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	_ "modernc.org/sqlite"
)

func Message(t testing.TB, name string, data []byte) proto.Message {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../completeness/schema.pb.gz"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b, err = io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	var set descriptorpb.FileDescriptorSet
	if err = proto.Unmarshal(b, &set); err != nil {
		t.Fatal(err)
	}
	files, err := protodesc.NewFiles(&set)
	if err != nil {
		t.Fatal(err)
	}
	types := dynamicpb.NewTypes(files)
	mt, err := types.FindMessageByName(protoreflect.FullName(name))
	if err != nil {
		t.Fatal(err)
	}
	m := mt.New().Interface()
	if err = (protojson.UnmarshalOptions{Resolver: types}).Unmarshal(data, m); err != nil {
		t.Fatal(err)
	}
	return m
}

// Schema creates the tables of an agy SQLite session in the order agy creates
// them, so trajectory_meta lands on page 2 where discovery's quick identity
// read expects it. A test that needs the tables elsewhere passes its own order
// to SourceWithSchema.
const Schema = `PRAGMA user_version=1;
 CREATE TABLE trajectory_meta(trajectory_id TEXT,cascade_id TEXT,trajectory_type INTEGER,source INTEGER);
 CREATE TABLE steps(idx INTEGER,step_type INTEGER,status INTEGER,has_subtrajectory INTEGER,metadata BLOB,error_details BLOB,permissions BLOB,task_details BLOB,render_info BLOB,step_payload BLOB,step_format INTEGER);
 CREATE TABLE gen_metadata(idx INTEGER,data BLOB,size INTEGER);
 CREATE TABLE executor_metadata(idx INTEGER,data BLOB);
 CREATE TABLE parent_references(idx INTEGER,data BLOB);
 CREATE TABLE trajectory_metadata_blob(id TEXT,data BLOB);
 CREATE TABLE battle_mode_infos(idx INTEGER,data BLOB);`

func Source(t testing.TB, path string, raw []byte) *sql.DB {
	t.Helper()
	return SourceWithSchema(t, path, raw, Schema)
}

// SourceWithSchema builds the source with the given table-creation statements,
// which must define the same tables as Schema.
func SourceWithSchema(t testing.TB, path string, raw []byte, schema string) *sql.DB {
	t.Helper()
	m := Message(t, "gemini_coder.Trajectory", raw).ProtoReflect()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	Exec(t, db, schema)
	f := m.Descriptor().Fields()
	Exec(t, db, "INSERT INTO trajectory_meta VALUES(?,?,?,?)", m.Get(f.ByName("trajectory_id")).String(), m.Get(f.ByName("cascade_id")).String(), int32(m.Get(f.ByName("trajectory_type")).Enum()), int32(m.Get(f.ByName("source")).Enum()))
	steps := m.Get(f.ByName("steps")).List()
	for i := 0; i < steps.Len(); i++ {
		InsertStep(t, db, i, steps.Get(i).Message())
	}
	for table, field := range map[string]string{"gen_metadata": "generator_metadata", "executor_metadata": "executor_metadatas", "parent_references": "parent_references", "battle_mode_infos": "battle_mode_infos"} {
		list := m.Get(f.ByName(protoreflect.Name(field))).List()
		for i := 0; i < list.Len(); i++ {
			b := Binary(t, list.Get(i).Message())
			if table == "gen_metadata" {
				Exec(t, db, "INSERT INTO gen_metadata VALUES(?,?,?)", i, b, len(b))
			} else {
				Exec(t, db, "INSERT INTO "+table+" VALUES(?,?)", i, b)
			}
		}
	}
	Exec(t, db, "INSERT INTO trajectory_metadata_blob VALUES('main',?)", Binary(t, m.Get(f.ByName("metadata")).Message()))
	return db
}
func Binary(t testing.TB, m protoreflect.Message) []byte {
	t.Helper()
	b, err := proto.Marshal(m.Interface())
	if err != nil {
		t.Fatal(err)
	}
	if b == nil {
		return []byte{}
	}
	return b
}
func InsertStep(t testing.TB, db *sql.DB, index int, m protoreflect.Message) {
	t.Helper()
	f := m.Descriptor().Fields()
	blob := func(name protoreflect.Name) []byte {
		fd := f.ByName(name)
		if !m.Has(fd) {
			return []byte{}
		}
		return Binary(t, m.Get(fd).Message())
	}
	var render []byte
	current := m
	for _, name := range []protoreflect.Name{"generic", "result", "step_render_info"} {
		field := current.Descriptor().Fields().ByName(name)
		if !current.Has(field) {
			break
		}
		current = current.Get(field).Message()
		if name == "step_render_info" {
			render = Binary(t, current)
		}
	}
	Exec(t, db, "INSERT INTO steps VALUES(?,?,?,?,?,?,?,?,?,?,?)", index, int32(m.Get(f.ByName("type")).Enum()), int32(m.Get(f.ByName("status")).Enum()), 0, blob("metadata"), blob("error"), blob("permissions"), blob("task_details"), render, Binary(t, m), 0)
}
func Exec(t testing.TB, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatal(err)
	}
}
