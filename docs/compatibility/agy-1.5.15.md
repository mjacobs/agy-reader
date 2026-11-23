# agy 1.5.15 compatibility

The reader verifies Orchestrator conversations that carry the saved
plan-and-execute flag. It also verifies the observed hook-free cancellation
of a non-daemon command that had moved to the background before the
cancellation was recorded. The ordinary RPC presentation retains the task
details and command with empty output. The full RPC must match the saved
arguments and cancellation history.

The [verified protobuf snapshot](../../artifacts/proto-snapshots/agy-1.5.15-7b004d26c3fd/snapshot.json)
contains 403 records. All 35 files in the trajectory and tools closure compiled
back to the extracted schema. Compared with 1.5.14, it adds the directory
snapshot message and its field in snapshot metadata, and removes and reserves
two actor ID fields in source metadata. The reader's overlay adds the directory
snapshot and the Orchestrator flag while retaining the existing decoder.
The directory snapshot's path and timestamp are covered by a regression test.

## Transcript checks

A fresh 1.5.15 daemon loaded a private copy of the CLI store. The release
exercise's 87-step parent and 7-step child matched offline export in steps,
trajectory metadata, generator metadata and executor metadata. The parent
covered an Orchestrator plan-to-execution handoff, scratch file reads and
edits, a zero-output command with exit code 1, a background completion
notification and a subagent reply. A separate 554-step Orchestrator
conversation containing the canceled background command matched in the same
four sections.

The sweep fetched and rendered all 502 conversations. The reader fully
verified 501, including 10 with declared source-only fields. The remaining
oversized conversation qualified for transcript-only output with executor
metadata omitted. SQLite retained user version 1, seven tables and both step
indices. The sidecar fingerprint changed. A path diff against the recorded
1.5.14 corpus found 24 added paths in configuration and usage metadata and no
removed paths. Those paths include the saved Orchestrator flag and
provider-assigned message IDs in retry usage.

The 1.5.15 changelog removes startup pruning beyond 500 conversations.
ACP attachment and cancellation handling, turn boundaries and combined usage,
Orchestrator side questions, and forced compaction remain unverified at runtime.
