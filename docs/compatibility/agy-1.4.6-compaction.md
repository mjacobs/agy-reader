# agy 1.4.6 attachment-compaction qualification

The reader now verifies and exports captured checkpoint requests with image,
PDF, and text-file attachments. Four native agy 1.4.6 conversations passed
source/full/default verification, offline JSON and Markdown export, ordinary
sync, and saved-attestation checks: **20 steps and five manual checkpoints**.
The previous reader refused all four sources because their persisted fields
were unknown.

This qualifies the feature within the observed bounds below. The whole-release
audit still fails the existing skill-file marker mismatch tracked as
`agy-reader#xpeh`: **410/411 conversations passed**. Both compatibility records
remain at 1.4.5. The mismatch was not excluded or given a new source-only rule.

## Native cases

| Case | Steps / checkpoints | Attachments retained | Checkpoint settings observed |
| --- | --- | --- | --- |
| Repeated mixed requests | 8 / 2 | PNG and PDF in the first checkpoint; both plus a text file in the second | Context window absent; media budget 1200 |
| Text-only control | 4 / 1 | None | Context window absent; media budget 1200 |
| Explicit settings | 4 / 1 | PNG and PDF | Context window `"32768"`; media budget 8192 |
| Explicit empty/zero settings | 4 / 1 | None | Context window `""` present; media budget 0 present |

These were genuine conversations and manual compactions, not hand-edited
checkpoint records. Synthetic attachments were submitted to
`SendUserCascadeMessage`, followed by its structured `compact` slash-command
item. The installed binary's CLI parser uses this same item shape:

```json
{"items":[{"item":{"slashCommand":{"info":{"name":"compact"}}}}]}
```

Configuration belongs directly under `cascadeConfig.checkpointConfig`.
The explicit-settings cases retained those settings in user configuration,
executor configuration, and static metadata, including optional empty/zero
presence. The two default cases qualify their actual persisted defaults only.
Captured requests include the compact command itself: the repeated case has
two captured requests in its first checkpoint and four in its second.

An owned temporary daemon reported agy 1.4.6 and ran the executable with SHA-256
`128780f5776959ba6a991bde1f0a8b699eee37a59ed30deb9926147fedfa56c4`.
Listener ownership and authenticated local RPC access were verified. The
selected model was Gemini 3.8 Flash Medium, with its actual plan-model mapping
checked against a CLI-generated conversation. For each case, a stable,
read-only SQLite backup was compared with independent full/unhydrated and
ordinary RPC captures. Source DB/WAL hashes did not change during capture.
Sync and saved-attestation checks used disposable copies of the sources.
The temporary daemon was stopped after qualification.

Native retained attachments used absolute local paths in `media.uri`.
The PNG (82 bytes), one-page PDF (651 bytes), and text file (55 bytes) all
matched the uploaded fixtures by SHA-256 after compaction. Markdown export
produced five attachment links across the repeated case, two in the explicit
settings case, and none in the controls. Raw conversations, local paths,
identifiers, credentials, and generated summaries remain outside the repository.
The [sanitized evidence](agy-1.4.6-compaction-evidence.json) records source,
capture, asset, render, and gate hashes.

## Reader changes and compatibility bounds

The production descriptor adds six declarations from the verified snapshot:

- `CortexStepCheckpoint.captured_user_requests` and `CapturedUserRequest`.
- Optional `CheckpointConfig.context_window` and
  `CheckpointConfig.max_user_request_media_tokens`.
- `CustomizationDiscoveryConfig.kind_enablement` and its nested map entry.

The customization map is a prerequisite observed in native planner and
executor configuration. It includes a false value as well as true values.
Its contents participate in exact source/full/default comparison. Strict wire
checks reject unknown or malformed map members, duplicate members, and duplicate
keys, including keys split across repeated enclosing singular messages.

The [production descriptor delta](agy-1.4.6-compaction-proto-delta.json) has six
additions, zero JSON-facing changes, and zero descriptor-only changes. Every
older declaration remains intact, including legacy `user_requests`; existing
`Media` and `Blobref` definitions are unchanged. The bundle has 31 descriptors
and SHA-256 `3d9a1ebb2ae55b316ab35cf19875f50d9f9c568ff16d8a04b95c90703ef026fa`.
The profile now ends in `-options145-compaction146`, requiring old attestations
to be reverified. No projection rule or source-only allowance was added.

Markdown displays captured request text and attachment names, MIME types,
descriptions, IDs, references, and page bounds when present. Absolute local
paths become escaped `file://` links. Inline data and thumbnail bytes are
summarized rather than printed. Legacy requests and captured requests both
remain visible if both are present.

Native evidence covers manual compaction and local attachment references.
Automatic compaction, remote blob availability, and portable attachment
archiving were not exercised. Inline/blob representations, empty references,
escaping, changed or reordered requests/media, unknown fields, and stale
attestations have synthetic regression coverage. Configuration qualification
proves retained values and presence; it does not prove token-budget enforcement.

## Corpus and validation

The final sweep ran from 16:56:18 to 16:57:38 UTC on 2026-10-23 using clean reader
commit `a869602`. Its complete version 5 manifest contains **411 conversations
and 36,892 steps**: 396 SQLite sources and 15 legacy protobuf sources. All
411 fetched and rendered, with zero fetch/render failures. Native verification
passed 320 fully projected sources and 90 with existing declared source-only
fields. One source still fails at `steps[185].viewFile.isSkillFile`, as recorded
in the [initial audit](agy-1.4.6.md).

All 411 conversations were required by the read-only format audit. It exited 1
and produced no compatibility record; `--record` was not run. The four new
feature scenarios passed their separate required qualification gate with exit 0.

The SQLite fingerprint remains
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
The explicitly version-scoped union of freshly serialized sidecars is now
`sha256:9131cdb95302c5a6dac0d22b2d4137d92b6c25a353a05717a26a57f8c889d72a`.
Compared with the initial 1.4.6 corpus shape (also the recorded 1.4.5 shape),
the [shape delta](agy-1.4.6-compaction-shape-delta.json) adds 24 paths and removes
none. They cover captured requests/media, attached user input, checkpoint
settings, and model selection retained in static configuration. These shape
fingerprints do not replace native comparison.

Validation passed:

- `make vet lint test maintenance-test build`: lint reported zero issues; Go
  tests passed; Python ran 154 tests, with 137 passing and 17 skipped.
- All 17 focused descriptor-generator tests passed. Content comparisons use
  decompressed descriptors so different zlib implementations remain supported;
  repeated output generation is also checked for determinism on one runtime.
- `go test -race ./internal/completeness ./internal/render` passed.
- Native checks, offline exports, sync, and saved-attestation rechecks passed
  for all four feature conversations.

The maintenance gate also needed a separate tmux 3.8 compatibility fix:
terminated panes can report an empty PID. Five existing lifecycle tests failed
on the unchanged baseline for that reason. The parser now accepts absent PIDs
only for dead panes, preserves empty tab fields, and never signals an empty
PID. All five lifecycle tests plus three parser/control tests passed.

No reader was installed, no existing service was restarted, and no AgentsView
or downstream ingestion qualification was performed.
