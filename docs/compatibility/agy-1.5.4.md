# agy 1.5.4 compatibility

The 610-step release exercise, four-step hook scenario, and two empty ACP
sessions pass native reader verification against both full and displayed RPC
responses. Offline JSON for all four matches displayed RPC, treating omitted
`steps` as an empty list. The command's tool-call ID matches the planner's ID.

## Reader changes

Empty text reads now accept the captured zero line numbers and byte counts
with the result `File is empty`. The displayed range stays zero rather than
underflowing when converting one-based line numbers. Nonempty content or
counts with that zero range remain unsupported.

Artifact overwrites now retain metadata, including `RequestFeedback: true`,
when the saved arguments, retained tool call and payload agree. This was an
`OVERWRITE` case, not a newly created artifact. Feedback on `CREATE` remains
unqualified. The reported overwrite step matches its persisted source and
both RPC representations, but its historical conversation still fails whole
source/RPC comparison. This does not resolve that separate mismatch.

The exercise also contains a canceled command with a 5000 ms wait,
`BypassSandbox: true`, a completion timestamp, no background task, and
pending/running/canceled status history. That exact combination is now
supported, preserving the bypass argument in the tool call.

Markdown output displays tool-call IDs on both planner proposals and executed
steps, allowing correlation with hook events. Empty IDs add no label. The
sidecar schema accepts an omitted `steps` property, as observed on both empty
ACP sessions. Sidecars still retain the daemon's raw JSON; offline exports
continue to emit `steps: []`.

## Corpus qualification

The fresh sweep fetched and rendered 499/499 conversations with zero failures.
Native sync verified 489/499; ten historical source/RPC mismatches remain,
matching the unresolved set from the 1.5.3 audit tracked in agy-reader#cpqn.
The exercise moves from unsupported to verified. All four release scenarios
pass the format helper's current-source qualification alongside the newest
conversation. No live sidecars were written.

SQLite remains at `user_version=1`, seven tables and two explicit indices;
its schema fingerprint is unchanged. Compared with the retained 1.5.3 shape
listing, the corpus adds two boolean paths for checkpoint file-diff `isNoop`
and removes none. These are optional presentation fields already defined in
the older schema, not any of the five new wire fields. The raw sidecars retain
them. The corpus union is not proof that every optional feature was exercised.

## Binary and format evidence

The [snapshot](../../artifacts/proto-snapshots/agy-1.5.4-c77851d1fc99/snapshot.json)
verifies 398 embedded records against the installed executable, with 34 closure
and round-trip files. Its comparison with 1.5.3 has five wire additions and
zero JSON-facing or descriptor-only changes: browser recording dimensions,
hook event name and working directory, and hook tool-call ID. None requires a
new embedded descriptor for the qualified scenarios. Historical declarations
and source-only rules remain unchanged; the projection profile adds `tools154`.

Changelog verdict: ACP mode/resume and permission-event changes, and added
hook fields, are relevant protocol risks. The supplied scenarios verify their
stored trajectories, not ACP mode notifications or live hook delivery; those
behavioral checks are reported by the original exercise. TUI rendering,
remote-control reconnect, and sign-in fixes do not describe storage changes.

The audit uses private, hash-checked copies of the CLI stores and a separate
idle daemon. Its startup log reports 1.5.4 and its executable SHA-256 matches
`c77851d1fc99b45bb7fc1dc4cddf09f5b668418cc498f92a09c73cddbc696343`.
The existing pair is neither restarted nor stopped. Raw captures, source
copies, manifests and test logs remain under ignored `tmp/agy-1.5.4-audit/`.
Recording compatibility updates the checkout baseline only; it does not
install or restart the deployed reader.
