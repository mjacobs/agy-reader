package completeness

import (
	"strings"
	"testing"
)

func TestCommandEnvironment1517ComparedInBothResponses(t *testing.T) {
	raw := options145Raw("executor", `{"runCommand":{"env":{"FOO":"expected"}}}`)
	snapshot := snapshot1216(t, raw)
	require1216(t, snapshot, raw, raw, Verified)
	changed := strings.Replace(raw, "expected", "different", 1)
	options145RequireBoth(t, snapshot, raw, changed)
}

func TestCommandEnvironment1517RejectsLossyWire(t *testing.T) {
	const message = "exa.cortex_pb.RunCommandToolConfig"
	entry := append(bytesField(1, 'A'), bytesField(2, 'x')...)
	valid := bytesField(28, entry...)
	if _, err := decode(message, valid); err != nil {
		t.Fatal(err)
	}
	for label, wire := range map[string][]byte{
		"duplicate key":       append(bytesField(28, entry...), valid...),
		"unknown entry field": bytesField(28, append(entry, options145Varint(3, 1)...)...),
		"wrong value type":    bytesField(28, append(bytesField(1, 'A'), options145Varint(2, 1)...)...),
	} {
		t.Run(label, func(t *testing.T) {
			if _, err := decode(message, wire); err == nil {
				t.Fatal("accepted lossy command environment")
			}
		})
	}
}
