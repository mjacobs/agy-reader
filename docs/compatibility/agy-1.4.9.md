# agy 1.4.9 compatibility

The reader now syncs and exports the corrupt-image errors and empty subagent
lists reported in the Gemini research. User and agent attachments are visible
in Markdown. Three fresh agy scenarios passed watcher verification and offline
JSON/Markdown export; the original research conversation also verified.

The full sweep fetched and rendered **503 conversations / 21,911 steps**.
Native completeness verification passed **502/503**, with no source-only
exceptions. One existing `SendMessage` presentation remains unsupported by
both the previous reader and this update; it is tracked separately as
`agy-reader#dw1x`. This is not a claim that every historical conversation can
be synced or exported offline.

## Review of the research

| Finding | Assessment and change |
| --- | --- |
| Corrupt-image `read_file` errors block export and sync | Confirmed. The old mapping only accepted `file does not exist`. The reader now preserves equal, nonempty short/full errors with the same strict wrapper and path checks. |
| Empty `manage_subagents` lists block export and sync | Confirmed, with a different code path from the report. Empty state lists reach the older two-ID projection, not the one-child 1.4.8 projection. The empty list now adds only `action: list` metadata, matching agy. |
| Prompt attachments disappear | Confirmed for typed access and Markdown. Raw sidecar JSON and offline JSON already preserve user media; the field predates 1.4.9. User and planner attachments now use the existing safe checkpoint-media renderer. |
| Two new protobuf fields are missing | Confirmed by binary extraction: planner `media` field 20 and generic `result_value` field 11. A narrow overlay registers them without replacing historical declarations. Planner media has synthetic retention and loss-detection tests. Populated structured generic results remain fail-closed until a native presentation mapping is qualified. |
| Canceled background commands reject `toolSummary` | Incorrect. `genericArgsAllowed` already admits `toolAction` and `toolSummary`; the existing canceled-background fixture includes a summary and passes. No production change was needed. |

The installed changelog confirms the 14 listed release notes. Its prompt image
pasting/dragging and corrupt-image validation changes are relevant to stored
transcripts. The research did not establish that invisible characters were
never persisted across all sessions, nor exercise every UI, permission,
browser, history-trimming or thinking-block fix. Those broad claims are not
compatibility guarantees from this audit.

## Native scenarios

An owned daemon ran the installed agy 1.4.9 executable with SHA-256
`1c9102d0a36194d57f6b4a66249217ece0bf563e108bcdd1046b81c5b6d6004c`.
Prompts and the image attachment were submitted through authenticated native
`StartCascade` / `SendUserCascadeMessage` RPCs. The model executed the tools;
no source records or RPC responses were hand-edited.

| Scenario | Final steps | Observed result |
| --- | ---: | --- |
| Empty list, corrupt PNG, valid PNG, next turn | 10 | Zero subagents; invalid PNG header at byte offset 77; valid 69-byte PNG read; recovery marker and a successful later model turn. |
| Prompt image attachment | 2 | A PNG submitted with the prompt remains in native JSON and appears with its MIME type and stored reference in Markdown. |
| Standalone corrupt PNG read | 4 | Isolates the failed-read mapping from the empty-list failure. The baseline offline exporter rejects it; the updated reader exports and verifies it. |

The unchanged baseline was built from `5012444`. It refused the combined
scenario in both watch mode and offline export with `unsupported
manage_subagents projection`, and the standalone image error with `unsupported
missing read_file projection`.

A separate reader watcher used read-only SQLite backups in a disposable
conversation root and the owned daemon endpoint. It created verified sidecars
for all three scenarios and updated the combined conversation from 8 to 10
steps after its second turn. The saved sidecars matched the native source,
full/unhydrated RPC, and default RPC under the new profile. Offline JSON and
Markdown exports passed with an unreachable daemon address; the backup hashes
did not change. Errors, media and step counts were checked explicitly.

The original Gemini research conversation verified at 293 steps in the broad
sweep. The sweep's remaining five-step `SendMessage` conversation failed with
`unsupported generic payload type` on both baseline and updated readers.

These tests qualify reader behavior after native input and tool execution.
They do not test terminal clipboard shortcuts, drag-and-drop UI, generated
planner media, structured generic results, or actual permission-denied file
reads. Permission-like error text and planner media have synthetic regression
coverage, not claims of native feature exercise. Existing limits on subagent
counts/actions and failed-read wrapper shapes remain documented in the
[completeness profile](../completeness.md#agy-149-messages-and-tool-results).

## Schema and validation

The [snapshot](../../artifacts/proto-snapshots/agy-1.4.9-1c9102d0a361/snapshot.json)
contains 388 extracted records and a 34-file verified closure/roundtrip. The
[1.4.8-to-1.4.9 delta](agy-1.4.9-proto-delta.json) contains two wire additions,
zero JSON-facing changes, and 12 descriptor-only changes. The production
schema retains 31 historical descriptors and adds only the two fields and the
required import of the existing `genai.Value` definition. No source-only
allowance was added. Old attestations require reverification.

Validation includes the complete Go suite, race-enabled suite, `go vet`,
`golangci-lint` (zero issues), schema regeneration equality, and maintenance
checks. The protobuf-capable Python ran 161 maintenance tests with 17 pidfd
subtests skipped because that interpreter lacks `os.pidfd_open`; launcher
coverage is checked separately with the system Python.

Raw RPCs, database backups, private transcripts, credentials and process logs
remain outside the repository. The committed fixtures contain synthetic
values following the captured shapes. `COMPATIBILITY.md` is generated by the
qualified format-audit helper; its required set includes these three scenarios
and the original research conversation. The broad sweep's unsupported
historical conversation is reported rather than excluded from the manifest.
