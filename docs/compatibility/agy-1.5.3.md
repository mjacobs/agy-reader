# agy 1.5.3 compatibility

The exercise parent, child, canceled-command sample, and newest conversation
passed reader-sync qualification. The fresh daemon fetched and rendered
500/500 conversations (19,718 steps). Native sync verified 490/500. The ten
remaining refusals are the same historical persisted-source and RPC
mismatches tracked in agy-reader#cpqn.

## What the reader now supports

The reader retains `SubagentSpec.model_option` in invocation payloads and
child trajectory metadata. The field is present in both the 1.5.2 and 1.5.3
binary descriptors. The exercise persisted it in both locations, where the
reader's older embedded descriptor refused it. The schema overlay adds only
that field and retains all historical declarations.

Completed text reads with a nonnegative byte offset preserve the saved
arguments and display the payload's line range, byte counts and content.
A canceled background command can omit the bypass argument when its
10,000 ms wait, missing completion time, and pending and running history
match the captured shape. Offline JSON also restores skill name and
description from the saved read result.

The 210-step parent, six-step child, and 17-step canceled-command sample
matched persisted SQLite against both retained RPC responses. Their entire
offline JSON trajectories matched default RPC after removing offline
provenance. The sampled frontmatter and completed sandbox-bypass commands
already passed online. The running bypass case remains unverified.

## Format evidence

SQLite still uses `user_version=1`, seven tables, and two explicit indices.
Its schema fingerprint matches the 1.5.2 record. The full sweep's sidecar
shape differs by 54 added and 18 absent path/type entries. Two additions
are the model option in child metadata and invocation results. The remaining
differences are in configuration, checkpoints and tool payloads. The two
corpora cover different conversations and optional features.

The fresh daemon reported 1.5.3 and its executable matched the
[verified binary snapshot](../../artifacts/proto-snapshots/agy-1.5.3-7ca212748ca0/snapshot.json).
The snapshot verified 397 embedded byte ranges and 34 closure and round-trip
files. Its 13 wire additions include `resolved_customizations` and a new
executor termination reason. The three retained scenarios did not populate
those declarations.

The release notes flagged subagent model display, file encodings, Unicode
tool summaries, and ACP shutdown persistence as relevant. The exercise
verified the model option, BOM and UTF-16 file operations, and Unicode path
presentation. The audit used a copied CLI store and a fresh isolated daemon.
