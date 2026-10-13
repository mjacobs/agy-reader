package completeness

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	_ "modernc.org/sqlite"
)

var columns = map[string][]string{
	"trajectory_meta":          {"trajectory_id", "cascade_id", "trajectory_type", "source"},
	"steps":                    {"idx", "step_type", "status", "has_subtrajectory", "metadata", "error_details", "permissions", "task_details", "render_info", "step_payload", "step_format"},
	"gen_metadata":             {"idx", "data", "size"},
	"executor_metadata":        {"idx", "data"},
	"parent_references":        {"idx", "data"},
	"trajectory_metadata_blob": {"id", "data"},
	"battle_mode_infos":        {"idx", "data"},
}

// sqliteSourceURI receives an absolute filesystem path. Windows drive paths
// need a leading slash; otherwise URL.String puts the drive in the authority.
// Keep UNC paths in the path component too, with an empty URI authority.
func sqliteSourceURI(path, goos string) url.URL {
	if goos == "windows" {
		path = strings.ReplaceAll(path, `\`, "/")
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}
	return url.URL{Scheme: "file", Path: path}
}

// ReadSnapshot opens a private, read-only SQLite connection and reads every
// component in one transaction, including committed WAL content. It never uses
// immutable=1, which would incorrectly hide committed WAL updates.
//
// An error is an *UnsupportedError (IsUnsupported) only when the profile
// refuses the source. Any other error, such as SQLITE_BUSY while the writer
// holds an exclusive lock past the busy timeout, means the source could not be
// read now and says nothing about whether it is supported.
func ReadSnapshot(ctx context.Context, path string) (*Snapshot, error) {
	s, err := readSnapshot(ctx, path)
	if err != nil && notADatabase(err) {
		return nil, refuse(err)
	}
	return s, err
}

func readSnapshot(ctx context.Context, path string) (*Snapshot, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	uri := sqliteSourceURI(path, runtime.GOOS)
	q := uri.Query()
	q.Set("mode", "ro")
	q.Set("cache", "private")
	q.Add("_pragma", "query_only(1)")
	q.Add("_pragma", "busy_timeout(250)")
	uri.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("read-only source snapshot: %w", err)
	}
	defer tx.Rollback()
	var version int
	if err = tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return nil, err
	}
	if version != 1 {
		return nil, refusal("unsupported SQLite source version")
	}
	if err = checkSchema(ctx, tx); err != nil {
		return nil, err
	}
	m, err := newMessage("gemini_coder.Trajectory")
	if err != nil {
		return nil, err
	}
	out := m.ProtoReflect()
	fields := out.Descriptor().Fields()
	rows, err := tx.QueryContext(ctx, "SELECT trajectory_id,cascade_id,trajectory_type,source FROM trajectory_meta")
	if err != nil {
		return nil, err
	}
	count := 0
	for rows.Next() {
		var id, cascade string
		var typ, source int32
		if err = rows.Scan(&id, &cascade, &typ, &source); err != nil {
			rows.Close()
			return nil, refuseValue(ctx, err)
		}
		count++
		out.Set(fields.ByName("trajectory_id"), protoreflect.ValueOfString(id))
		out.Set(fields.ByName("cascade_id"), protoreflect.ValueOfString(cascade))
		out.Set(fields.ByName("trajectory_type"), protoreflect.ValueOfEnum(protoreflect.EnumNumber(typ)))
		out.Set(fields.ByName("source"), protoreflect.ValueOfEnum(protoreflect.EnumNumber(source)))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, refusal("source must contain exactly one trajectory")
	}
	budget := maxBytes
	if err = readSteps(ctx, tx, out, &budget); err != nil {
		return nil, err
	}
	for table, field := range map[string]string{"gen_metadata": "generator_metadata", "executor_metadata": "executor_metadatas", "parent_references": "parent_references", "battle_mode_infos": "battle_mode_infos"} {
		if err = readList(ctx, tx, table, out, protoreflect.Name(field), &budget); err != nil {
			return nil, err
		}
	}
	rows, err = tx.QueryContext(ctx, "SELECT id,data FROM trajectory_metadata_blob")
	if err != nil {
		return nil, err
	}
	count = 0
	for rows.Next() {
		var id string
		var data []byte
		if err = rows.Scan(&id, &data); err != nil {
			rows.Close()
			return nil, refuseValue(ctx, err)
		}
		count++
		if id != "main" {
			rows.Close()
			return nil, refusal("unsupported trajectory metadata key")
		}
		budget -= len(data)
		if budget < 0 {
			rows.Close()
			return nil, refusal("source size limit")
		}
		value, e := decode(fields.ByName("metadata").Message().FullName(), data)
		if e != nil {
			rows.Close()
			return nil, refuse(e)
		}
		out.Set(fields.ByName("metadata"), protoreflect.ValueOfMessage(value.ProtoReflect()))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, refusal("source must contain one trajectory metadata record")
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	// finishSnapshot reads nothing but the content already read.
	snapshot, err := finishSnapshot(m)
	if err != nil {
		return nil, refuse(err)
	}
	return snapshot, nil
}

func checkSchema(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return err
	}
	names := map[string]bool{}
	for rows.Next() {
		var n string
		if err = rows.Scan(&n); err != nil {
			rows.Close()
			return refuseValue(ctx, err)
		}
		names[n] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(names) != len(columns) {
		return refusal("unsupported SQLite source tables")
	}
	for name, want := range columns {
		if !names[name] {
			return refusal("missing source table")
		}
		// name is a compile-time table name, never an input identifier.
		rows, err = tx.QueryContext(ctx, "PRAGMA table_xinfo("+name+")")
		if err != nil {
			return err
		}
		var got []string
		for rows.Next() {
			var cid, notnull, pk, hidden int
			var n, typ string
			var def any
			if err = rows.Scan(&cid, &n, &typ, &notnull, &def, &pk, &hidden); err != nil {
				rows.Close()
				return refuseValue(ctx, err)
			}
			got = append(got, n)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(got, want) {
			return refusal("unsupported SQLite source columns")
		}
	}
	return nil
}

func readSteps(ctx context.Context, tx *sql.Tx, out protoreflect.Message, budget *int) error {
	rows, err := tx.QueryContext(ctx, "SELECT idx,step_type,status,has_subtrajectory,metadata,error_details,permissions,task_details,render_info,render_info IS NOT NULL,step_payload,step_format FROM steps ORDER BY idx")
	if err != nil {
		return err
	}
	defer rows.Close()
	list := out.Mutable(out.Descriptor().Fields().ByName("steps")).List()
	index := 0
	for rows.Next() {
		var idx, typ, status, format int32
		var sub, renderPresent bool
		var metadata, details, permissions, task, render, payload []byte
		if err = rows.Scan(&idx, &typ, &status, &sub, &metadata, &details, &permissions, &task, &render, &renderPresent, &payload, &format); err != nil {
			return refuseValue(ctx, err)
		}
		if int(idx) != index || index >= 100000 {
			return refusal("unsupported step indexing")
		}
		index++
		if format != 0 {
			return refusal("unsupported step format")
		}
		if sub {
			return refusal("unsupported decomposed step representation")
		}
		*budget -= len(payload) + len(metadata) + len(details) + len(permissions) + len(task) + len(render)
		if *budget < 0 {
			return refusal("source size limit")
		}
		step, e := decode("gemini_coder.Step", payload)
		if e != nil {
			return refuse(e)
		}
		sm := step.ProtoReflect()
		sf := sm.Descriptor().Fields()
		if sm.Get(sf.ByName("type")).Enum() != protoreflect.EnumNumber(typ) || sm.Get(sf.ByName("status")).Enum() != protoreflect.EnumNumber(status) {
			return refusal("source step header columns disagree")
		}
		if err = checkRenderInfo(sm, render, renderPresent); err != nil {
			return refuse(err)
		}
		for name, data := range map[string][]byte{"metadata": metadata, "error": details, "permissions": permissions, "task_details": task} {
			fd := sf.ByName(protoreflect.Name(name))
			other, e := decode(fd.Message().FullName(), data)
			if e != nil {
				return refuse(e)
			}
			if !sm.Has(fd) {
				if proto.Size(other) != 0 {
					return refuse(fmt.Errorf("source step %s column disagrees", name))
				}
				continue
			}
			if !proto.Equal(sm.Get(fd).Message().Interface(), other) {
				return refuse(fmt.Errorf("source step %s column disagrees", name))
			}
		}
		list.Append(protoreflect.ValueOfMessage(sm))
	}
	return rows.Err()
}

// render_info is a redundant copy of generic.result.step_render_info, not a
// top-level Step field. Preserve NULL versus an empty BLOB so message presence
// must agree even when a present StepRenderInfo has no populated fields.
func checkRenderInfo(step protoreflect.Message, data []byte, present bool) error {
	var nested protoreflect.Message
	current := step
	for _, name := range []protoreflect.Name{"generic", "result", "step_render_info"} {
		field := current.Descriptor().Fields().ByName(name)
		if !current.Has(field) {
			break
		}
		current = current.Get(field).Message()
		if name == "step_render_info" {
			nested = current
		}
	}
	if !present {
		if nested != nil {
			return errors.New("source step render_info column disagrees")
		}
		return nil
	}
	column, err := decode("exa.cortex_pb.StepRenderInfo", data)
	if err != nil {
		return err
	}
	if nested == nil || !proto.Equal(column, nested.Interface()) {
		return errors.New("source step render_info column disagrees")
	}
	return nil
}

func readList(ctx context.Context, tx *sql.Tx, table string, out protoreflect.Message, field protoreflect.Name, budget *int) error {
	query := "SELECT idx,data FROM " + table + " ORDER BY idx"
	if table == "gen_metadata" {
		query = "SELECT idx,data,size FROM gen_metadata ORDER BY idx"
	}
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	fd := out.Descriptor().Fields().ByName(field)
	list := out.Mutable(fd).List()
	index := 0
	for rows.Next() {
		var idx int
		var data []byte
		var size int
		if table == "gen_metadata" {
			err = rows.Scan(&idx, &data, &size)
		} else {
			err = rows.Scan(&idx, &data)
		}
		if err != nil {
			return refuseValue(ctx, err)
		}
		if idx != index || index >= 100000 {
			return refusal("unsupported metadata indexing")
		}
		index++
		if table == "gen_metadata" && size != len(data) {
			return refusal("generator metadata size disagrees")
		}
		*budget -= len(data)
		if *budget < 0 {
			return refusal("source size limit")
		}
		value, e := decode(fd.Message().FullName(), data)
		if e != nil {
			return refuse(e)
		}
		list.Append(protoreflect.ValueOfMessage(value.ProtoReflect()))
	}
	return rows.Err()
}
