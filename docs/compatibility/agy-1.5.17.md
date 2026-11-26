# agy 1.5.17 compatibility

agy-reader can sync and export 1.5.17 transcripts that contain command
environment settings and background-command notifications. The preceding
reader refused the exercise transcript during offline export because it
encountered an unknown protobuf field. After decoding the new settings it
could export that transcript, but strict sync still refused the timed
commands. The updated reader checks their saved arguments and reproduces
the command shown by the daemon.

The exercise used three explicit notification timeouts. The step numbers
below are zero-based. Each background command produced a still-running
message and then a completion message associated with the same task.

| Command step | Wait before backgrounding | Notification timeout | Result |
| --- | ---: | ---: | --- |
| 6 | 500 ms | 3 s | Background, exited 0. Notifications at steps 8 and 10. |
| 14 | 1,000 ms | 4 s | Completed in the foreground, exited 0. |
| 17 | 500 ms | 2 s | Background, exited 3. Notifications at steps 19 and 21. |

The reader exported all 23 steps from SQLite. All eight daemon-owned
top-level fields in its JSON were identical to the ordinary RPC response,
including all four task notifications. Markdown export produced 36,033
bytes. The verifier also accepted the 977-step historical conversation
with older completed, canceled and rejected timeout calls. Its offline JSON
likewise matched all eight fields in the ordinary response.

Under strict permissions, the reader verified a four-step conversation with
a single-use approval for a timed foreground command. It also verified an
11-step conversation with an approved background command and a declined
timed command. The background command kept both task notifications and exit
code 0. The declined command kept its denial response and error status,
with no exit code. Both offline JSON exports matched every field in the
ordinary RPC response. Regression tests reject completed commands whose
saved wait is missing from that response.

A fourth conversation followed the same command while it ran, after an
explicit cancellation, and after the pair stopped. The reader verified
all three captures (8, 18 and 19 steps respectively). In a separate test,
a fresh daemon reopened the 8-step running database and marked the command
canceled. The reader reproduced that response as well. All four offline
JSON exports matched every daemon-owned field. The running command still
appeared as a generic tool call. The canceled command kept its background
task details and had no exit code.

The [verified protobuf snapshot](../../artifacts/proto-snapshots/agy-1.5.17-f3f384bd2c15/snapshot.json)
contains 404 records. All 35 files in the trajectory and tools closure
compiled back to the extracted schema. Compared with 1.5.16, the reader
now decodes the command environment map, model icon details and additional
guardian configuration. The 17 wire additions are covered by schema
comparison and decoding tests. Of those additions, the release exercise
populated the command environment map.

A fresh 1.5.17 daemon served private copies of all 508 SQLite conversations
in the CLI store. The reader fetched and rendered 34,374 steps across all
508. It fully verified 507 conversations, including ten with declared
source-only fields. The remaining 936-step conversation qualified for
transcript-only export because its executor metadata exceeded the reader’s
SQLite source budget. The historical 977-step conversation already passed
on the pre-update reader.

SQLite kept the same seven tables, two step indices and user version 1.
The ordinary RPC corpus grew from 2,026 to 2,052 JSON path-and-type
entries. The 26 additions cover the interaction mode, permission requests,
grants and responses, and the strict session’s list of rules that ask the
user for approval. All previous entries remained present.
The [release evidence](agy-1.5.17-evidence.json) records the checksums
and the tested decoder's Git tree ID, reproducible with
`git rev-parse HEAD:internal/completeness` at this release.

The exercise preserved the saved Orchestrator configuration. It did not
compare equal and mixed Planner/Executor effort selections. Remote Control,
account and service retries, and platform-specific UI behavior were outside
this transcript exercise.
