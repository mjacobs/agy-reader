# agy 1.5.16 compatibility

The reader exports conversations that use the interactive step checklist.
During the release exercise, agy created a scratch file, ran successful and
failed commands, and updated two checklists through completion. The reader
initially refused offline export at the first completed `update_steps` call.
The fixed reader preserves all seven calls, including their arguments, result
text and hidden checklist rendering. All eight daemon-owned top-level fields
in the offline JSON matched the ordinary RPC response for the 116-step
conversation. Markdown export also succeeded.

The [verified protobuf snapshot](../../artifacts/proto-snapshots/agy-1.5.16-05245f4346d1/snapshot.json)
contains 403 records. All 35 files in the trajectory and tools closure
compiled back to the extracted schema. Compared with 1.5.15, the schema adds
structured GenAI steps to chat prompts and planner responses, and policy
coverage to the guardian configuration. Wire-decoding tests exercise those
three fields separately from the checklist tool, including a named PDF inside
a structured prompt. Conflicting duplicate checklist arguments are refused.

A fresh 1.5.16 daemon served a private copy of the CLI conversation store.
The reader fetched and rendered all 504 conversations and fully verified 502,
including ten with declared source-only fields. One oversized conversation
qualified for transcript-only export with executor metadata omitted. A
977-step historical conversation contained a tool result that both the
previous reader and this release refused to project offline. SQLite retained
its seven tables, two step indices and user version 1. The two release
corpora had the same 2,026 JSON path and type entries.

The exercise covered pending, in-progress and done checklist items, and a
command that exited with status 1. Skipped and failed checklist items, ACP
prompt forwarding, swarm notification attribution and remote media handling
remain unverified.
