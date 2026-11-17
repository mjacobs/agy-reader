# agy 1.5.10 compatibility

The 317-step release exercise now syncs and exports in full, including its
cancelled timer and 2,179,522-byte PDF. Its two subagents also sync and export.
A separate native timer verifies while the countdown is active and again after
it fires. The reader preserves the timestamp until agy removes it from the
stored payload, and preserves the PDF's full contents in JSON.

| Native source | Baseline refusal | Qualified result |
| --- | --- | --- |
| Running timer | Unknown persisted protobuf fields | Retain the generic step and `fireTime`. Add the ordinary response's timer mode. |
| Cancelled timer in release exercise | Unsupported tool projection | Retain status `DONE`, cancellation text and task details. Repeat the stored mode and outcome in the projected result metadata. |
| Naturally fired timer | Completion outside the old timer projection | Retain status `DONE`, render information and the `fired` outcome. |
| Inline PDF read | Unsupported image read projection | Project to `VIEW_FILE` with the original path, byte count, MIME type and data. Markdown renders a media summary. |

The baseline PDF refusal was reproduced by appending just the saved PDF step
to a disposable child database. The final parent and timer captures each
verify from SQLite against fresh full and ordinary RPC responses.
Synthetic tests reject mutated timestamps, PDF bytes, argument mismatches,
unqualified statuses and extra result fields.

## Release and schema evidence

The [verified snapshot](../../artifacts/proto-snapshots/agy-1.5.10-20d6f74672ce/snapshot.json)
rechecks all 402 embedded byte ranges against the installed binary and
compiles 35 closure files back to the normalized descriptor set. The
isolated daemon identifies itself as 1.5.10 in its startup log.

The 1.5.9 comparison finds nine additive wire changes, two JSON-facing changes
and one new import. Both JSON changes describe the same field,
`exa.cortex_pb.SidecarSandboxConfig` field 5, renamed from `allowed_hosts`
and `allowedHosts` to `allowed_http_hosts` and `allowedHttpHosts`. The new
descriptors also cover agent mode, planner configuration, permission scope,
policy provenance and compaction. The production overlay adopts the one
new field observed in the native captures, `Schedule.fire_time`, with its
Timestamp import. The other new declarations and the renamed configuration
key remain outside the qualified profile. No source-only exception was added.

The release notes call out timers, large PDF reads and lazy history loading.
The native exercise verifies the relevant transcript behavior for those cases,
as well as Orchestrator plan transitions, a checkpoint, concurrent subagents,
task management and CRLF-preserving edits. A nameless `SKILL.md` read sets
`isSkillFile` and omits `skillMetadata` in the daemon's ordinary response.
The tool for plugin validation derives the name in its own output. The reader
retains the read response as observed. The exercise also contains ACP startup
and sandbox probes. These are evidence of stored tool results. Remote
accept-edits, authenticated ACP cancellation and the sandbox policy itself
were outside the reader's native verification.

The reader still requires a whole verified trajectory before replacing a
sidecar, so an unsupported step can leave the last verified transcript in
place. An in-flight database capture can also disagree with RPC while agy is
writing. The fired timer was qualified on a new capture after the conversation
settled. Its earlier source/RPC mismatch remains a refusal.
