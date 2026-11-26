# agy 1.5.18 compatibility

agy-reader can sync and export a compacted 1.5.18 conversation whose
checkpoint records that a child has stopped. The 1.5.17 reader refused both
JSON and Markdown export when it encountered the new protobuf field. The
updated reader exported all 24 steps and verified the saved `running: false`
against both the ordinary and full daemon responses.

## Release exercise

A 1.5.18 conversation read a valid 123-byte PNG and an unfinished 44-byte
PNG. The transcript preserved the valid image byte for byte. The second
read recorded an error and attached no media. The parent then read a text
file successfully, started a research subagent, observed it running, and
received its reply with the same text marker. The child’s own SQLite
transcript records its file read and the message it sent to the parent.

The reader verified the 22-step parent and 6-step child. After compaction,
the same parent had 24 steps. Its final checkpoint contained one subagent
snapshot with `running` explicitly set to `false`. All eight daemon-owned
fields in each offline JSON export matched the ordinary response. The
24-step Markdown export produced 51,184 bytes.

Strict verification refused all four changes made to copies of the
checkpoint response. Each changed either the ordinary or full response by
removing `running` or setting it to `true`. Four additional checks changed
the short or full image error in either response, and all four were refused.
Regression tests also verify sources with `running: true` and with the field
omitted. The live status listing captured a running child. The checkpoint
was taken after the child finished.

## Corpus and schema

A fresh 1.5.18 daemon reopened **510 distinct SQLite conversations**,
including the release parent and child. The reader fetched and rendered
all 34,404 steps. Strict sync qualified 509 conversations, including ten
with declared source-only fields. The remaining 936-step conversation
qualified for transcript-only export with 132 executor records omitted
under the reader’s SQLite source budget. The earlier 22-step capture of
the parent contributes no additional conversation to this total.

All 508 historical conversations retained their previous step counts and
verification results. They contained seven subagent snapshots, each with
`running` omitted. Four historical offline exports matched every field in
their daemon presentations and rendered every step in Markdown.

| Historical conversation | Steps | JSON fields matched | Markdown check |
| --- | ---: | ---: | --- |
| Timed commands | 23 | 8/8 | Three timed commands and four notifications |
| Subagent checkpoints | 313 | 8/8 | Both checkpoint summaries |
| Transcript-only export | 936 | 7/7 | Checkpoint summary |
| Image attachment | 2 | 8/8 | PNG attachment metadata |

The SQLite schema fingerprint stayed the same. The ordinary response corpus
grew from 2,052 to 2,093 JSON path-and-type entries and retained every
previous entry. The additions comprise the checkpoint boolean, 28 planner
experiment entries, and 12 command-environment entries. The release exercise
covered local transcripts. Windows and Remote Control UI behavior remain
untested.

The release binary matched all 405 records in its protobuf snapshot. All
35 files in the trajectory and tools closure compiled back to the extracted
definitions. `SubagentSnapshot.running` (field 8) is the only wire addition
in that closure since 1.5.17. [Protobuf snapshot](../../artifacts/proto-snapshots/agy-1.5.18-7c96ad8c944c/snapshot.json) · [Audit evidence and checksums](agy-1.5.18-evidence.json)
