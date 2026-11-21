# agy 1.5.14 compatibility

The reader decodes plan mode, response language, customization scope and tool
settings in the historical trajectory schema. A regression test encodes six
examples with the native 1.5.14 descriptors and decodes them with the shipped
schema. It covers plan mode without requiring a remote account.

The [verified snapshot](../../artifacts/proto-snapshots/agy-1.5.14-37873b806239/snapshot.json)
contains 402 records. All 35 files in the trajectory and tools closure compiled
back to the extracted schema. Compared with 1.5.13, the snapshot has 24 wire
changes and four descriptor-only changes. The reader keeps historical
declarations, including types the newer binary removed.

## Transcript checks

The release-analysis session exercised a workflow ending in a top-level return,
read files, and wrote and edited a scratch file. Its completed SQLite export
matched ordinary 1.5.14 RPC for all 44 steps, trajectory metadata, generator
metadata and executor metadata. A separate paired session read a file containing
a known sentence. Markdown reproduced the sentence and all four steps. Its
watcher had saved three steps by the time print mode exited. A fresh daemon
verified the completed four-step database and its ordinary RPC matched the
offline export in the same four sections.

The full sweep fetched and rendered 500 conversations. The reader verified 489
directly and 10 with declared source-only fields. For one oversized conversation
it verified the transcript with executor metadata omitted. The SQLite format
retains user version 1, seven tables and two step indices. The sidecar corpus
included provider-assigned message IDs in planner responses and model usage.
The 1.5.13 descriptors already defined those fields.

The captures exercised local sessions. Remote plan execution, the interactive
thinking toggle, and recovery from a truncated model response were outside the
runtime coverage of this audit.

The native decoding tests include grouped model defaults and the RPC allowlist
inside a sidecar's sandbox settings. A checkout with automatic line-ending
conversion also reproduced all 443 snapshot checksums.

The reader validates grouped-model map entries before decoding their values.
This preserves the verification contract when a saved entry contains data
that a protobuf map decoder would otherwise discard.

The generated compatibility record names the clean implementation commit used
for the sweep. After squashing the audit into the release change, the audited
decoder can still be identified by its Git tree. Run
`git rev-parse HEAD:internal/completeness` at the release commit and compare
the result with `0b0f4a93997882a4c3799e98129a11377de965a5`.
