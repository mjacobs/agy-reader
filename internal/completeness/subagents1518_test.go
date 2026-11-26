package completeness

import "testing"

func TestSubagentCheckpointRunningState(t *testing.T) {
	checkpoint := func(state string) string {
		return parentmsgs1410Raw(`"subagentSnapshots":[{` + state + `}]`)
	}
	for _, state := range []string{"", `"running":false`, `"running":true`} {
		t.Run(state, func(t *testing.T) {
			raw := checkpoint(state)
			source := options145Snapshot(t, raw)
			require1216(t, source, raw, raw, Verified)
			offline, err := source.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, source, raw, string(offline), Verified)
			for _, changed := range []string{"", `"running":false`, `"running":true`} {
				if changed != state {
					options145RequireBoth(t, source, raw, checkpoint(changed))
				}
			}
		})
	}
}
