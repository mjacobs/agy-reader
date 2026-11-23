package completeness

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
)

func TestDirectorySnapshot1515(t *testing.T) {
	path := bytesField(1, []byte("/fixture/work")...)
	timestamp := bytesField(2, options145Varint(1, 1)...)
	directory := append(path, timestamp...)
	snapshot := bytesField(7, directory...)

	msg, err := decode("exa.cortex_pb.SnapshotMetadata", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Directory struct {
			Path      string `json:"path"`
			Timestamp string `json:"timestamp"`
		} `json:"x20Directory"`
	}
	raw, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Directory.Path != "/fixture/work" || got.Directory.Timestamp != "1970-01-01T00:00:01Z" {
		t.Fatalf("decoded directory snapshot = %+v", got.Directory)
	}

	unknown := bytesField(7, append(directory, options145Varint(99, 1)...)...)
	if _, err := decode("exa.cortex_pb.SnapshotMetadata", unknown); err == nil {
		t.Fatal("accepted an unknown field inside the directory snapshot")
	}

}

func TestOrchestratorFlag1515(t *testing.T) {
	// An explicit false must survive decoding rather than become absent.
	msg, err := decode("exa.cortex_pb.DefaultAgentConfig", options145Varint(6, 0))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"planExecute":false}` {
		t.Fatalf("decoded Orchestrator flag = %s", raw)
	}
}

func TestHookFreeBackgroundCancellation1515(t *testing.T) {
	c := bgcancel150NoTransition
	raw, shown := c.load(t)
	removeHook := func(step map[string]any) {
		delete(obj(step, "metadata"), "preToolHookResults")
	}
	raw, shown = c.editStep(t, raw, removeHook), c.editStep(t, shown, removeHook)
	require1216(t, snapshot1216(t, raw), raw, shown, Verified)

	daemon := c.editStep(t, raw, func(step map[string]any) {
		obj(step, "taskDetails")["isDaemon"] = true
	})
	require1216(t, snapshot1216(t, daemon), daemon, shown, Unknown)
}
