package completeness

import "testing"

func TestTools153ReadByteOffset(t *testing.T) {
	for _, offset := range []string{"46080", "92160", "138240"} {
		t.Run(offset, func(t *testing.T) {
			raw, shown := ide225ReadPair(false)
			raw = replace1216(t, raw, `"ByteOffset":"0"`, `"ByteOffset":"`+offset+`"`)
			raw = replace1216(t, raw, `\"ByteOffset\":0`, `\"ByteOffset\":`+offset)
			shown = replace1216(t, shown, `\"ByteOffset\":0`, `\"ByteOffset\":`+offset)
			s := snapshot1216(t, raw)
			require1216(t, s, raw, shown, Verified)
			for _, replacement := range []string{"", `,\"ByteOffset\":1`} {
				changed := replace1216(t, shown, `,\"ByteOffset\":`+offset, replacement)
				require1216(t, s, raw, changed, Incomplete)
			}
		})
	}
}

func TestTools153SubagentModelOption(t *testing.T) {
	full, shown := tool143Fixtures[9].pair()
	parentFull := replace1216(t, full, `"typeName":"researcher"`, `"typeName":"researcher","modelOption":"fixture-model"`)
	parentShown := replace1216(t, shown, `"typeName":"researcher"`, `"typeName":"researcher","modelOption":"fixture-model"`)
	const child = `{"cascadeId":"fixture-child","metadata":{"subagentSpec":{"typeName":"researcher","modelOption":"fixture-model"}}}`
	for _, tc := range []struct{ name, full, shown string }{
		{"parent", parentFull, parentShown},
		{"child", child, child},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := options145Snapshot(t, tc.full)
			require1216(t, s, tc.full, tc.shown, Verified)
			for _, replacement := range []string{"", `,"modelOption":"other-model"`} {
				require1216(t, s, replace1216(t, tc.full, `,"modelOption":"fixture-model"`, replacement), tc.shown, Incomplete)
				require1216(t, s, tc.full, replace1216(t, tc.shown, `,"modelOption":"fixture-model"`, replacement), Incomplete)
			}
		})
	}
}

func TestTools153CanceledBackgroundCommand(t *testing.T) {
	// The retained 1.5.3 sample has the same short history and wait as the
	// 1.5.0 fixture, with no bypass argument in either copy of the tool call.
	full, shown := bgcancel150NoTransition.load(t)
	full = bgcancel150NoTransition.editStep(t, full, func(step map[string]any) {
		deleteArgument(step, "BypassSandbox")
	})
	shown = replace1216(t, shown, `\"BypassSandbox\":true,`, "")
	s := snapshot1216(t, full)
	require1216(t, s, full, shown, Verified)
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, full, string(offline), Verified)

	changed := bgcancel150NoTransition.editStep(t, full, func(step map[string]any) {
		obj(step, "metadata")["completedAt"] = "2026-01-01T00:00:02Z"
	})
	require1216(t, snapshot1216(t, changed), changed, shown, Unknown)
}
