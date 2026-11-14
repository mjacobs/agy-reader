# agy 1.5.9 compatibility

The completed 310-step self-analysis conversation now verifies with all six
children: four workflow workers and two ordinary subagents. Ordinary sync and
Markdown output pass for all seven conversations. All six children receive
the correct parent link, and all four workflow workers appear at their
originating call in the parent's Markdown. The linked sources contain 339
steps, including both turns of the reusable workflow worker.

A broad copied-store sweep fetches and renders 507/507 conversations and
natively verifies 506/507: 496 ordinary verifications and ten with existing
declared source-only legacy model-choice fields. Offline export matches
fresh RPC steps, generator metadata and executor metadata for all 506.
The remaining refusal is the historical failed board post after a swarm
member was killed, already tracked in agy-reader#07da. Fetch/render success
does not qualify that source for an attestation.

## Corrections established by the session

| Observed form | Reader support |
| --- | --- |
| Completed `enter_plan_mode` and `exit_plan_mode` | Retain the generic wrapper, exact recorded arguments and timestamped result. These tools predate 1.5.9 but were newly exercised here. |
| Completed inline `run_workflow` | Preserve its script, JSON arguments, working directory, phase/log output and render information. |
| Completed file-based workflow | Preserve `ScriptPath`, parallel and pipeline results, and multi-turn worker output. |
| Failed inline workflow | Preserve equal short/full execution errors and the partial render-only result. The attempted `call_tool` failed with `NameError`; this is not evidence of a successful tool invocation. |
| Workflow child metadata | Decode the authenticated `ToolSourceMetadata.title` string field 4. No other 1.5.9 declarations enter the production bundle. |
| Workflow child ancestry and placement | Match `sourceMetadata.tool` against the parent's conversation ID, step index, call ID, tool name and argument JSON. Conflicting native pointers remain unresolved. |

Before correction, the parent sidecar stopped at 98 steps and all four
workflow children lacked sidecars. A seven-source copied case set verified
only the two ordinary subagents. Five sanitized full/default step pairs cover
plan entry, plan exit, a successful inline workflow, successful worker
orchestration and the failed inline workflow. Tests reject changed arguments,
unsupported statuses, extra permission/task/result fields, malformed source
links and mismatched full/default responses. Unknown nested protobuf fields
remain refused. The profile suffix is `workflow159`; earlier attestations
require reverification. There is no new source-only exemption.

The existing generic renderer already shows result text and retains nested
render blocks in its stored-details section. This update adds explicit worker
placement; it does not claim to introduce a rich terminal workflow-card UI.

An early copy taken during a running command was canceled by the isolated
daemon on load. Its resulting incomplete status history was excluded from
qualification. The final corpus uses a new backup after every step in the
self-analysis session completed. No cancellation rule was relaxed for that
collection artifact.

## Schema and release evidence

The [snapshot](../../artifacts/proto-snapshots/agy-1.5.9-39816d49d5ab/snapshot.json)
was reverified against the installed binary, SHA-256
`39816d49d5ab92229952987225c23ea3fed7580699dcb4a444e0388e0ac1ce43`:
402 embedded byte ranges, 35 closure files and 35 round-trip files. The live
self-analysis process executable had the same hash, and both its startup log
and the isolated audit daemon identify 1.5.9. Comparing with 1.5.8 gives 33
additive wire changes, zero JSON-facing changes and zero descriptor-only
changes. The unexercised additions remain outside the reader's qualified
schema rather than being admitted by a whole-release replacement.

`generate_workflow159.py` authenticates the previous bundle and snapshot before
adding exactly one field. Tests remove it and reproduce the complete previous
bundle. The new compressed bundle is 138,790 bytes, SHA-256
`89c0877cfe610ac674038826080672e1a692be4c5d5cb0dd550495e9c575da3a`;
its decompressed SHA-256 is
`bfcb3c2e926ced1cef32c2eb5bcf329554b9ad3d35183f16eba6749bb9a3ff6c`.

All 19 changelog entries were reviewed. Workflows, the Auto permission
reviewer's additional tools and browser MP4 recordings warrant transcript
inspection. The session exercises workflow success, failure and orchestration,
plus ordinary concurrent subagents. Its ancillary experiments report MCP
redirect-header stripping, BOM handling and malformed terminal-size handling;
those CLI behavior checks were not independently repeated for this reader
audit. It produced no browser recording and did not exercise authenticated
resume/ACP, managed plugin policy, the Auto reviewer, remote control or the
platform-specific fixes. MP4 decoding/projection remains unverified.

SQLite remains `user_version=1`, seven tables and two explicit indices, with
fingerprint
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
The fresh corpus adds 782 shape paths and lacks 22 compared with the retained
1.5.8 corpus. Of the additions, 762 come from workflow-child agent configuration,
ten from explicit source metadata and ten from checkpoint subagent snapshots.
The absent paths concern optional permission coverage. This is inspected
corpus drift, not evidence that 1.5.9 removed those fields. Every corpus file
was freshly serialized by 1.5.9; cached sidecars supplied no sweep evidence.

Private SQLite backups, full/default captures, manifests, shape paths and
validation logs remain under ignored `tmp/compat159/`. All seven scenario
sources and their WAL hashes stayed unchanged across backup collection. An
separate private copy preserves all 507 sources; copies pruned by isolated
daemon startup were restored to its disposable store before sweeping. Sync
writes only that disposable store. The generated `COMPATIBILITY.md` records
the qualified audit. The active reader and agy-pair were not installed or
restarted by this update.

## Validation

`make check` passes formatting, vet, lint, Go tests, the build and all 185
Python maintenance tests under protobuf 7.36.2 and protoc 35.1. The maintenance
checks include exact descriptor reproduction, historical preservation,
tampered-input refusal and snapshot verification.
`go test -race -p 2 ./...` passes. A baseline race run under concurrent load
hit the watcher alias test's 500 ms deadline; five focused reruns and the full
baseline rerun passed before the reader changes.
