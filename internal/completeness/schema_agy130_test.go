package completeness

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Fields that agy 1.2.17 and 1.3.0 added after the 1.2.16 descriptors. agy
// 1.2.17 and later store enable_plugin_auth as false in every conversation's
// generator and executor metadata; without its definition every new
// conversation read as unknown.
var agy130AddedFields = []struct {
	message protoreflect.FullName
	number  protowire.Number
	name    protoreflect.Name
	wire    protowire.Type
}{
	{"exa.cortex_pb.RunCommandToolConfig", 23, "enable_plugin_auth", protowire.VarintType},
	{"exa.cortex_pb.WorkflowSpec", 19, "depot_path", protowire.BytesType},
	{"exa.cortex_pb.CheckpointConfig", 34, "media_tokens_per_second", protowire.VarintType},
	{"exa.cortex_pb.CheckpointConfig", 35, "use_model_reported_token_count", protowire.VarintType},
	{"exa.codeium_common_pb.CascadeNUXConfig", 21, "g1_redirect_url", protowire.BytesType},
}

func TestAgy130AddedFieldsAreDefined(t *testing.T) {
	for _, f := range agy130AddedFields {
		t.Run(string(f.message)+"."+string(f.name), func(t *testing.T) {
			m, err := newMessage(f.message)
			if err != nil {
				t.Fatal(err)
			}
			fd := m.ProtoReflect().Descriptor().Fields().ByNumber(f.number)
			if fd == nil || fd.Name() != f.name {
				t.Fatalf("field %d: got %v, want %s", f.number, fd, f.name)
			}
			data := protowire.AppendTag(nil, f.number, f.wire)
			if f.wire == protowire.VarintType {
				data = protowire.AppendVarint(data, 0)
			} else {
				data = protowire.AppendString(data, "x")
			}
			if _, err := decode(f.message, data); err != nil {
				t.Fatalf("decode: %v", err)
			}
		})
	}
	// The refusal policy is unchanged: an undefined field is still unknown.
	undefined := protowire.AppendVarint(protowire.AppendTag(nil, 9999, protowire.VarintType), 0)
	if _, err := decode("exa.cortex_pb.RunCommandToolConfig", undefined); err == nil || err.Error() != "unknown persisted protobuf fields" {
		t.Fatalf("undefined field: got %v", err)
	}
}

// The persisted shape agy 1.2.17 and 1.3.0 write: enable_plugin_auth=false at
// planner_config.tool_config.run_command in both metadata records.
func TestEnablePluginAuthSourceVerifies(t *testing.T) {
	const runCommand = `{"toolConfig":{"runCommand":{"enablePluginAuth":false}}}`
	raw := `{"cascadeId":"plugin-auth-fixture","metadata":{},` +
		`"generatorMetadata":[{"plannerConfig":` + runCommand + `}],` +
		`"executorMetadatas":[{"cascadeConfig":{"plannerConfig":` + runCommand + `}}],` +
		`"steps":[{"type":"CORTEX_STEP_TYPE_RUN_COMMAND","runCommand":{"commandLine":"echo fixture"}}]}`
	p := filepath.Join(t.TempDir(), "source.db")
	db := testfixture.Source(t, p, []byte(raw))
	var gen, exec []byte
	if err := db.QueryRow("SELECT data FROM gen_metadata WHERE idx=0").Scan(&gen); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT data FROM executor_metadata WHERE idx=0").Scan(&exec); err != nil {
		t.Fatal(err)
	}
	// Field 23, varint, value 0: the bytes the reader refused before 1.2.17 support.
	stored := protowire.AppendVarint(protowire.AppendTag(nil, 23, protowire.VarintType), 0)
	for name, record := range map[string][]byte{"generator": gen, "executor": exec} {
		if !bytes.Contains(record, stored) {
			t.Fatalf("%s record does not store enable_plugin_auth: %x", name, record)
		}
	}
	s, err := ReadSnapshot(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if _, r := s.VerifyResponses([]byte(raw), []byte(raw)); r.State != Verified {
		t.Fatalf("got %+v", r)
	}
}
