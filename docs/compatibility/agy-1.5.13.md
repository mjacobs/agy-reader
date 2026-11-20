# agy 1.5.13 compatibility

Markdown exports now include the unsaved text an editor attached to an ACP
prompt. The reader takes it from the conversation, so the export still shows
what the model saw if the file on disk changes. Text file reads also display
one-based line numbers. The two reads in the parallel-tool scenario both show
lines 1–2, matching their contents.

## What changed in storage

The SQLite schema has user version 1, seven tables and two step indices. The
[verified snapshot](../../artifacts/proto-snapshots/agy-1.5.13-c3fd2c848abf/snapshot.json)
contains 405 embedded records and a 35-file trajectory and tools closure.
Comparison with 1.5.12 found 35 wire changes, zero JSON-only changes and zero
descriptor-only changes. Of these, 27 give model-feature booleans explicit
presence. An updated historical decoder retains an explicitly disabled
capability in JSON. The other eight changes add six fields, a message and an
enum. The snapshot and generator tests cover these additions.

## Release scenarios

| Scenario | What the reader verified |
| --- | --- |
| Headless Neon, JSON | Six steps. One planner response proposed two file reads and a chained shell command. Both reads and the completed command appear in the export. |
| Headless Neon, stream JSON | Two steps. The CLI emitted a successful final result. The two headless runs cover success, without forcing a failed attempt followed by a retry. |
| ACP | Four steps. The user input contains the attached editor text. The shell step ran in the session project folder. The ACP model picker returned 33 entries without internal placeholder IDs. Trajectories still carry the model IDs written by agy. |
| Release investigation | The self-analysis finished with 116 steps. A separate completed conversation had 120. All steps, trajectory metadata, generator metadata and executor metadata in both offline exports matched ordinary RPC. |
| Custom model | Three isolated API-key captures persisted false values for both thinking and tool calls. The old reader omitted both values in offline JSON. Two loopback requests completed successfully. In the inspected request agy sent a zero thinking budget and 14 function declarations. Disabling tool calls in the stored configuration did not suppress the declarations in that request. |

The custom-model check used the documented settings shape, a fixture API key
and a loopback Gemini endpoint. The [settings schema](https://antigravity.google/schemas/v1/settings.json)
describes the provider, model key and capabilities used for this check.

## Watcher behavior

A watcher sharing a store with another agy process can see a conversation
before the writer finishes. In the ACP case it saved one step. Its daemon
skipped subsequent duplicate loads, so the later four-step database failed
the reader's content comparison. A fresh 1.5.13 daemon serving a private
copy verified all four steps. Offline export also recovered all four.
The [paired watcher](../../README.md#running-a-test-pair) now starts a
short-lived agy process against a private SQLite copy when the paired
daemon serves older content. It publishes the recovered sidecar after
RPC verification and a fresh check against the live database. The paired
watcher documentation lists the supported launch modes.

## Corpus results

A dedicated 1.5.13 daemon fetched and rendered all 503 copied SQLite
conversations. Full and ordinary RPC matched the native source for 492,
plus ten with declared source-only fields. The remaining 936-step
conversation passed transcript-only verification with executor metadata
omitted. The compatibility record qualifies all 502 fully verified sources.

Offline JSON for those 502 conversations matched ordinary RPC across steps,
trajectory metadata, generator metadata and executor metadata when absent
repeated fields were treated as empty. The only differences in a strict
comparison were 26 empty conversations whose offline export includes an
empty steps array and whose RPC response omits it.

The sidecar shape gained 14 path/type entries relative to the recorded
1.5.12 corpus and lost none. Nine describe running-task snapshots at
checkpoints. Five describe file references and attached text in user input.
These counts depend on the features exercised in each corpus.
