package completeness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func TestOfflineImplicitEnvelope(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "implicit")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "activity.pb")
	raw, err := proto.Marshal(testfixture.Message(t, "gemini_coder.Trajectory", []byte(legacyFixture)))
	if err != nil {
		t.Fatal(err)
	}
	scope := protowire.AppendString(protowire.AppendTag(nil, 3, protowire.BytesType), "topic")
	envelope := protowire.AppendBytes(protowire.AppendTag(nil, 5, protowire.BytesType), scope)
	envelope = protowire.AppendBytes(protowire.AppendTag(envelope, 1, protowire.BytesType), raw)
	testfixture.LegacyBytes(t, path, envelope)
	t.Chdir(dir)
	snapshot, err := ReadOfflineSnapshot(t.Context(), filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	data, err := snapshot.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Steps     []json.RawMessage
		AgyReader struct {
			Offline struct {
				Profile       string
				ImplicitScope struct{ BranchName string }
			}
		}
	}
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Steps) != 1 || got.AgyReader.Offline.Profile != ImplicitProfile || got.AgyReader.Offline.ImplicitScope.BranchName != "topic" {
		t.Fatal("implicit steps or provenance lost")
	}
	for _, bad := range [][]byte{nil, envelope[:len(envelope)-1], append(append([]byte{}, envelope...), envelope...), protowire.AppendTag(envelope, 7, protowire.VarintType)} {
		if _, _, err := implicitTrajectory(bad); err == nil {
			t.Fatal("accepted unsupported implicit envelope")
		}
	}
}
