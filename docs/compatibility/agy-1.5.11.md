# agy 1.5.11 compatibility

The reader verifies and exports the complete 199-step self-analysis and its
six-step research subagent. Before the update, the live watcher had saved 12
parent steps and one child step. The final SQLite captures passed verification
against both full and ordinary responses from an isolated 1.5.11 daemon.
Offline JSON also matched the original live daemon's final steps, generator
metadata and executor metadata for both conversations.

| Release exercise | What the transcript establishes |
| --- | --- |
| File reads and edits | File references in full tool results carry optional line and byte totals. The native exercise includes a BOM-prefixed plugin manifest, range and byte-offset reads, a 75-byte PNG and a 590-byte PDF. |
| Subagent invocation | The render reference carries a log URI and workspace URIs. The child completed and replied to the parent. |
| Background tasks | A builtin command moved to the background after a 500 ms wait and was killed. The completed, hook-free cancellation presents the command and empty combined output, with its task details intact. |
| Plugin validation and API-key mode | Command output records successful validation of plain and BOM-prefixed plugins as both directories and ZIPs. The three headless quota commands each returned exit code 2. |

The Remote Control changes make subagent mentions, attachment uploads and
quotes worth checking. The exercise established how the CLI stored the
underlying file and agent references. Browser gestures, uploads up to 20 MB
and navigation to an older quoted passage remain untested.

## Protobuf evidence

The [verified snapshot](../../artifacts/proto-snapshots/agy-1.5.11-01b063665007/snapshot.json)
rechecked 405 embedded records against the executable serving the live session.
All 35 closure files compiled back to equivalent descriptors. Compared with
1.5.10 there are 18 additive wire changes, four JSON-facing changes and one
descriptor-only change. The four JSON changes describe two task identifier
renames from `call_id` to `task_id` (JSON `callId` to `taskId`).

The production schema retains the four fields observed in persisted render
references. Full-source equality checks their values even when the ordinary
RPC presents a file read or subagent invocation differently. Source metadata,
citation configuration, trajectory-size compaction, hook source, background task
results and the task identifier renames remain outside this release's new
qualification. The snapshot also marks `ImageData` deprecated.

## Corpus verification

A fresh 1.5.11 daemon fetched and rendered all 502 copied CLI conversations.
Of those, 491 verified without exceptions and ten verified with previously
declared source-only fields. The same historical conversation refused native
verification before and after the update. It contains a failed board post
after a swarm member was killed. All 501 verified offline exports
matched the ordinary RPC steps and generator and executor metadata.

The SQLite fingerprint remains unchanged at schema version 1, with seven
tables and two explicit indices. Compared with the recorded 1.5.10 shape,
the ordinary-response corpus gained eight paths and lost none. The eight
paths describe four policy guardian settings under both agent-script and
static configuration. The release's new file and agent fields were visible
in full tool results. They were identified by checking the persisted source
and the full response, independently of the ordinary-response fingerprint.

## What the self-analysis captured

The parent had 137 SQLite steps when the exercise took its diagnostic copy.
Decoding that copy with the pre-update reader schema reproduced 66 unknown
field occurrences: 32 file references and one subagent reference, each with
two new fields. The six-step child had two occurrences on one file read.
The 199-step result above comes from the later, completed parent. These counts
establish which references carried the fields in this exercise.

The following zero-based parent step indices locate the evidence in the full
trajectory:

| Steps | Observation |
| --- | --- |
| 73–74 | The BOM-prefixed manifest and the skill file each had eight lines. Their file references reported 124 and 96 bytes respectively. |
| 80–81 | The range and offset reads both reported the whole file's ten lines and 158 bytes. |
| 84–85 | The PNG and PDF references explicitly set the optional line count to zero. Their byte counts matched the inline data. |
| 88–89, 92–94 | The parent listed the tasks, checked the first and killed the second. The brain log recorded both commands as `RUNNING` when they backgrounded. Final SQLite and RPC recorded `DONE` and `CANCELED`, respectively. |
| 108 | The enclosing command succeeded. Each of its three quota probes exited 2, pointed to AI Studio and left the isolated conversations directory empty. |
| 110 | The subagent reference identified the child, its transcript log and its workspace. |

A sidecar's attestation describes the capture it contains. The old watcher
saved the parent's 12-step sidecar as unverified and retained the child's
one-step attested sidecar when later verification failed. A final brain-log
entry count can match SQLite while individual task statuses still reflect an
earlier state.

## Task identifiers and host watcher

The `enable-iapi-tool-steps` experiment was false in the captured parent.
The release snapshots authenticate the task-ID renames and
`ToolResultStep.background` field 22. The reader's embedded descriptor bundle
omits the interactions `steps.proto` file entirely. The proposed explanation
that its `ManageTaskCallStep` descriptor still expects `callId` therefore
does not fit the shipped reader. The effect of enabling this experiment
remains unqualified. A capture containing the typed task messages, with its
persisted source and full and ordinary RPC responses, would establish which
decoding or comparison path needs support.

On the audit host on November 18, both user systemd units were inactive. A
separate watcher launched by `agy-pair.py` was running in a terminal scope.
Its executable identified revision `c8cf76f`, which embeds the 1.5.10
baseline. The 1.5.11 record on local main applies to a reader built from
the updated checkout. The inactive units alone do not establish a watcher
outage.
