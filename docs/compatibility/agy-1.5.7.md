# agy 1.5.7 compatibility

The runtime follow-up verifies 506/506 copied conversations against an isolated
agy 1.5.7 daemon: 496 ordinary verifications and ten verifications with the
existing declared source-only legacy model-choice fields. The sweep fetched
and rendered all 506, with zero failures and zero unverified sources. This
extends the earlier retained-session audit with fresh permission, task-input
and background-command captures.

Before these corrections, the same retained-store sweep verified 459/500
conversations (449 ordinary, ten source-only) and refused 41. The original
report's 16 disposable conversations had been removed; their exact counts and
timing cannot be reproduced from that evidence. The fresh captures below
reproduce the reader failures without depending on those deleted files.

## Confirmed corrections

| Reported behavior | Verified cause and correction |
| --- | --- |
| `manage_task send_input` refused | Modern `ManageTask` payloads carry `input`. The projection now requires matching action, input and task identity in the payload, arguments and retained call, then adds the daemon's action/input/task metadata. |
| Denied or canceled commands refused | New captures lack the historical implicit allow hook. User-rule errors, headless prompt cancellations and explicit prompted rejections have separate bounded rules. The latter uses the captured `unsandboxed` wording and effective-grant trigger. |
| Running and reloaded background commands refused | Running calls retain their complete generic wrapper. On reload the captured command becomes canceled without a completion timestamp; the projection preserves its task details and synthesizes the same empty-output command as the daemon. |
| Strict edit approvals and canceled reads refused | Modern command/edit approvals retain both the grant and completed interaction. Their action and target must match the call and one allowing response. A canceled path-only read becomes a file view with its original status and metadata. |
| Unknown `template_overrides` | The embedded descriptor adds only authenticated `PromptSectionCustomizationConfig.template_overrides` and its map entry. Unknown entry members, duplicate keys and malformed wire encodings remain refused. |
| Cron and swarm refusals | Zero-iteration canceled cron results omit iteration metadata; reloaded in-flight cron wrappers remain generic. The reported swarm actions already worked; the extra captured form was `models` with `all=true`. |
| Further refusal found by the sweep | A modern append is stored as `FileEdit OVERWRITE` and presented as an overwrite. Its explicit append argument must remain in the matching retained call. |
| Markdown fences and omitted records | All renderer-owned code fences now widen for embedded backticks, including diffs, with a single linear scan. Saved display Markdown is fenced as literal text so unfinished fences or HTML cannot absorb later steps. Transcripts show hook results, permissions, completed/pending interactions, task details and generic result content blocks, retaining remaining wrapper details. |
| Watcher retry/log noise | Unchanged content mismatches wait 30 seconds before retry. Source, sidecar, profile and endpoint changes release the delay immediately; expiry retries the same endpoint. Ordinary sync logs use the actual staleness reason. |
| IPv6 discovery | Pure `::1` listeners use bracketed IPv6 URLs. Exported credentials and explicit endpoint pins remain separated by address family, as well as existing ownership checks. |

The profile suffix is `runtime157`. Prior attestations require native
reverification. Full-source comparison remains mandatory before presentation
comparison, and no new source-only exemption was introduced.

## Verification and limits

Six fresh runtime conversations pass exact equality of `steps`,
`generatorMetadata` and `executorMetadatas` between offline export, fresh RPC
and ordinary verified sync in the isolated store. Separately, offline output
matches those three fields for all 506 swept sources after normalizing an
absent RPC step list to the exporter's explicit empty list. A private capture
also verifies the background command while it is still `RUNNING`, before
reload changes it to `CANCELED`.

Committed fixtures cover 13 captured step forms and rejection boundaries.
Tests also exercise map-wire loss prevention, prior-profile invalidation,
embedded fences, visible execution records, bounded retry/recovery and IPv6
credential ownership. Go tests, race tests, vet, lint and Python maintenance
checks accompany the qualified format audit.

The stale in-memory daemon problem remains an upstream/runtime limitation;
retry delays reduce repeated work but cannot reload a daemon's cached history.
The existing endpoint recovery policy remains in place. The isolated stdin
wait did not produce the claimed waiting-for-input notification; the reader
preserves the task's recorded progress and does not invent a notification.

Sidecars without a surviving `.db` or `.pb` are already excluded from source
discovery. A source can disappear between discovery and reading, but orphan
transcripts are not repeatedly discovered or deleted. Bare agy processes that
have exposed no CSRF token still need the documented
[token-launch workflow](../../README.md#watch-mode); this audit does
not bypass authentication. Organization-enforced admin-policy branches,
Windows OAuth, unavailable gateway models and TUI animation timing were not
independently qualified.

## Changelog and schema evidence

The default Auto mode, strict edits, admin review/rule enforcement, background
input and `manage_task` formatting notes are compatibility red flags for
trajectory consumers even though the SQLite schema is unchanged. The two
long-output animation/wrapping fixes are UI changes; embedded output still
needs safe transcript fences. Windows OAuth, model-gateway request formatting
and Remote Control link filtering do not themselves establish a storage or
RPC-format change. All ten changelog items were reviewed with these limits.

The existing [1.5.7 protobuf snapshot](../../artifacts/proto-snapshots/agy-1.5.7-5df525a2d690/snapshot.json)
was reverified against the installed binary: 401 embedded byte ranges, 34
closure files and 34 round-trip files. The isolated daemon's startup version
and process executable identify the same 1.5.7 binary, SHA-256
`5df525a2d6908989c57b897723caea66e00d7c4d2e24411f91ad306989574642`.
No duplicate snapshot was needed.

Compared with 1.5.6, the snapshot has four additive wire fields and no
JSON-facing or descriptor-only changes: marketplace bundle IDs/flag, plugin
bundle members and a code-execution working directory. The missing template
map predates this release; it is authenticated from the 1.5.7 snapshot rather
than replacing the reader's historical descriptor bundle wholesale.

SQLite remains at `user_version=1`, seven tables and two explicit indices,
with schema fingerprint
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
The generated `COMPATIBILITY.md` records the final corpus fingerprint and
qualification. Its shape differs from the earlier 500-session corpus by 37
added and 25 removed paths: chiefly newly exercised permission records and
configuration/experiment coverage, plus optional tool fields absent from the
new retained corpus. This is inspected corpus drift, not evidence of a SQLite
format change. Raw sources, RPC captures, source hashes and audit logs remain
private under ignored `tmp/runtime157/`. Audit collection and sync used only
disposable copies; this change does not install or restart the live reader.
