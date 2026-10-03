package completeness

import (
	"bytes"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const renderRaw = `{"cascadeId":"11111111-1111-4111-8111-111111111111","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","generic":{"result":{"stepRenderInfo":{"title":"Task started","markdown":"Task details","contentBlocks":[{"agent":{"conversationId":"child-1","title":"Image"}}]}}}}]}`

func TestRenderInfoColumn(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"populated", renderRaw},
		{"present empty", strings.Replace(renderRaw, `{"title":"Task started","markdown":"Task details","contentBlocks":[{"agent":{"conversationId":"child-1","title":"Image"}}]}`, `{}`, 1)},
		{"absent", syntheticRaw},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "render.db")
			testfixture.Source(t, path, []byte(tc.raw))
			snapshot, err := ReadSnapshot(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			if result := snapshot.CompareFullRaw([]byte(tc.raw)); result.State != Verified {
				t.Fatalf("raw equality: %+v", result)
			}
		})
	}

	t.Run("equivalent field ordering", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "render.db")
		db := testfixture.Source(t, path, []byte(renderRaw))
		var original []byte
		if err := db.QueryRow("SELECT render_info FROM steps WHERE idx=0").Scan(&original); err != nil {
			t.Fatal(err)
		}
		var reordered []byte
		for rest := original; len(rest) > 0; {
			_, _, size := protowire.ConsumeField(rest)
			if size < 0 {
				t.Fatal("invalid test fixture wire encoding")
			}
			reordered = append(append([]byte{}, rest[:size]...), reordered...)
			rest = rest[size:]
		}
		if bytes.Equal(original, reordered) {
			t.Fatal("test fixture must change field ordering")
		}
		testfixture.Exec(t, db, "UPDATE steps SET render_info=? WHERE idx=0", reordered)
		if _, err := ReadSnapshot(t.Context(), path); err != nil {
			t.Fatalf("semantically equal render_info rejected: %v", err)
		}
	})
}

func TestRenderInfoColumnRejects(t *testing.T) {
	unknown := protowire.AppendVarint(protowire.AppendTag(nil, 99999, protowire.VarintType), 1)
	for _, tc := range []struct {
		name   string
		raw    string
		mutate func(*testing.T, *sql.DB)
		want   string
	}{
		{
			name: "changed title",
			mutate: func(t *testing.T, db *sql.DB) {
				changeRenderColumn(t, db, func(render protoreflect.Message) {
					render.Set(render.Descriptor().Fields().ByName("title"), protoreflect.ValueOfString("Different title"))
				})
			},
			want: "source step render_info column disagrees",
		},
		{
			name: "changed nested lineage",
			mutate: func(t *testing.T, db *sql.DB) {
				changeRenderColumn(t, db, func(render protoreflect.Message) {
					blocks := render.Get(render.Descriptor().Fields().ByName("content_blocks")).List()
					block := blocks.Get(0).Message()
					agent := block.Get(block.Descriptor().Fields().ByName("agent")).Message()
					agent.Set(agent.Descriptor().Fields().ByName("conversation_id"), protoreflect.ValueOfString("different-child"))
				})
			},
			want: "source step render_info column disagrees",
		},
		{
			name: "column only on native step",
			raw:  syntheticPresentation,
			mutate: func(t *testing.T, db *sql.DB) {
				render := testfixture.Message(t, "exa.cortex_pb.StepRenderInfo", []byte(`{"title":"Unexpected"}`))
				testfixture.Exec(t, db, "UPDATE steps SET render_info=?", testfixture.Binary(t, render.ProtoReflect()))
			},
			want: "source step render_info column disagrees",
		},
		{
			name: "column only on generic step",
			raw:  syntheticRaw,
			mutate: func(t *testing.T, db *sql.DB) {
				testfixture.Exec(t, db, "UPDATE steps SET render_info=X'0a0178'")
			},
			want: "source step render_info column disagrees",
		},
		{
			name: "payload only",
			mutate: func(t *testing.T, db *sql.DB) {
				testfixture.Exec(t, db, "UPDATE steps SET render_info=NULL")
			},
			want: "source step render_info column disagrees",
		},
		{
			name: "empty column only",
			raw:  syntheticRaw,
			mutate: func(t *testing.T, db *sql.DB) {
				testfixture.Exec(t, db, "UPDATE steps SET render_info=X''")
			},
			want: "source step render_info column disagrees",
		},
		{
			name: "empty payload only",
			raw:  `{"metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","generic":{"result":{"stepRenderInfo":{}}}}]}`,
			mutate: func(t *testing.T, db *sql.DB) {
				testfixture.Exec(t, db, "UPDATE steps SET render_info=NULL")
			},
			want: "source step render_info column disagrees",
		},
		{
			name: "malformed column",
			mutate: func(t *testing.T, db *sql.DB) {
				testfixture.Exec(t, db, "UPDATE steps SET render_info=X'80'")
			},
			want: "invalid persisted protobuf",
		},
		{
			name: "unknown column field",
			mutate: func(t *testing.T, db *sql.DB) {
				changeRenderColumn(t, db, func(render protoreflect.Message) { render.SetUnknown(unknown) })
			},
			want: "unknown persisted protobuf fields",
		},
		{
			name: "unknown nested column field",
			mutate: func(t *testing.T, db *sql.DB) {
				changeRenderColumn(t, db, func(render protoreflect.Message) {
					blocks := render.Get(render.Descriptor().Fields().ByName("content_blocks")).List()
					blocks.Get(0).Message().SetUnknown(unknown)
				})
			},
			want: "unknown persisted protobuf fields",
		},
		{
			name: "unknown payload render field",
			mutate: func(t *testing.T, db *sql.DB) {
				trajectory := testfixture.Message(t, "gemini_coder.Trajectory", []byte(renderRaw)).ProtoReflect()
				step := trajectory.Get(trajectory.Descriptor().Fields().ByName("steps")).List().Get(0).Message()
				current := step
				for _, name := range []protoreflect.Name{"generic", "result", "step_render_info"} {
					current = current.Get(current.Descriptor().Fields().ByName(name)).Message()
				}
				current.SetUnknown(unknown)
				testfixture.Exec(t, db, "UPDATE steps SET step_payload=?", testfixture.Binary(t, step))
			},
			want: "unknown persisted protobuf fields",
		},
		{
			name: "decomposed subtrajectory",
			mutate: func(t *testing.T, db *sql.DB) {
				testfixture.Exec(t, db, "UPDATE steps SET has_subtrajectory=1")
			},
			want: "unsupported decomposed step representation",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := tc.raw
			if raw == "" {
				raw = renderRaw
			}
			path := filepath.Join(t.TempDir(), "render.db")
			db := testfixture.Source(t, path, []byte(raw))
			tc.mutate(t, db)
			if _, err := ReadSnapshot(t.Context(), path); err == nil || err.Error() != tc.want {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

func changeRenderColumn(t *testing.T, db *sql.DB, change func(protoreflect.Message)) {
	t.Helper()
	var data []byte
	if err := db.QueryRow("SELECT render_info FROM steps WHERE idx=0").Scan(&data); err != nil {
		t.Fatal(err)
	}
	render, err := decode("exa.cortex_pb.StepRenderInfo", data)
	if err != nil {
		t.Fatal(err)
	}
	change(render.ProtoReflect())
	testfixture.Exec(t, db, "UPDATE steps SET render_info=? WHERE idx=0", testfixture.Binary(t, render.ProtoReflect()))
}
