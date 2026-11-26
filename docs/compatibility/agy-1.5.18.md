# agy 1.5.18 compatibility

The reader preserves a subagent's running state when exporting a checkpoint
from SQLite. Strict sync also checks that state against agy's ordinary and
full responses. Decoder tests cover running, stopped, and a checkpoint
without the field. Each test also changes the
daemon's answer and checks that verification refuses the mismatch.

The [1.5.18 protobuf snapshot](../../artifacts/proto-snapshots/agy-1.5.18-7c96ad8c944c/snapshot.json)
contains 405 records checked against the release binary. All 35 files in the
trajectory and tools closure compiled back to the extracted definitions. The
optional boolean `SubagentSnapshot.running` (field 8) is the only wire
addition in that closure since 1.5.17.

## Historical transcripts

A fresh 1.5.18 daemon reopened private SQLite backups of 508 conversations
written before the upgrade. The new build fetched and rendered every
conversation, covering 34,374 steps. Strict sync qualified 507, including
ten with declared source-only fields. The remaining 936-step conversation
qualified for transcript-only export. Its executor metadata exceeds the
reader's SQLite source budget. These results match the
[1.5.17 audit](agy-1.5.17.md). The SQLite schema and union of JSON field names
and types also retained their previous fingerprints. Seven subagent
snapshots appeared in the historical corpus, all without `running`.

The reader exported four of these conversations through its offline CLI.
Every daemon-owned field in each JSON export matched the ordinary response
from agy 1.5.18. The Markdown exports contained all 1,274 step headings.

| SQLite conversation | Steps | Fields matched | Markdown check |
| --- | ---: | ---: | --- |
| Timed commands | 23 | 8/8 | Three timed commands and four notifications |
| Subagent checkpoints | 313 | 8/8 | Both checkpoint summaries |
| Transcript-only export | 936 | 7/7 | Checkpoint summary, with 132 executor records omitted from the JSON presentation |
| Image attachment | 2 | 8/8 | PNG attachment metadata |

The image attachment predates 1.5.18. It covers replay of a saved image.
Testing the release's rejection of an unfinished image requires a new file
read in a signed-in 1.5.18 conversation.

The [audit evidence](agy-1.5.18-evidence.json) records the tested decoder
tree and checksums of the four JSON comparisons and Markdown exports.
