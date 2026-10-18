# agy 1.4.3 compatibility audit

The reader qualifies agy 1.4.3 after adding bounded projections for six modern
tool payloads. The required parent and child conversations pass native
source/RPC verification. This is a compatibility result for the observed
formats, not a claim that every historical conversation or optional CLI
feature is supported.

## Provenance and gate

- Serving CLI: agy 1.4.3, confirmed from the running process's executable.
- Executable SHA-256:
  `d003c9d47e12f689d3448cad0f614289ccde5fb4c58bf8b4b543a9b514c097bc`.
- Reader implementation: `7fc34f9`, with profile
  `sqlite-v1-format0-agy-1.3.4-5b93565c5f04-planner50-permission2-prompt23-tools143-permission1`.
  The older version in the profile identifies the bundled descriptor's
  provenance; `tools143` identifies the additional qualified projections.
- The sweep used one authenticated endpoint for the CLI store. All 393 selected
  conversations fetched and rendered; the manifest is version 4 and complete.
- Native results: 295 verified, 75 verified with declared source-only fields,
  8 unsupported SQLite histories, and 15 unverifiable legacy protobuf sessions.
  There are 378 SQLite sessions in total.
- `AGY_QUALIFY_SESSIONS` explicitly included the parent and child described
  below. Both passed, as did the schema and package-test checks. The audit
  helper generated `COMPATIBILITY.md`; it was not hand-edited.

Private source/RPC snapshots, manifests, scenario identities and command logs
are retained separately. Public fixtures contain synthetic values only.

## Reader failure and correction

The required parent, called **parent-a** here, had a 276-step database and a
one-step sidecar. A stable database/full-RPC/default-RPC capture contained 276
steps in all three representations. The old reader refused the first modern
`RunCommand` payload as `unsupported generic payload type`.

The fix qualifies these observed mappings:

| Payload | Observed behavior |
| --- | --- |
| `RunCommand` | Native command step, retaining outer metadata/task details and the stored result text as output; synchronous, asynchronous and sandbox-error examples. |
| `ReadFile` | Native file-view step, converting one-based ranges to zero-based ranges and retaining result text and file sizes. |
| `FileEdit` | CREATE/EDIT code actions with paths, diffs and statistics, including an empty file. |
| `ManageTask` | Generic status/list/kill step with metadata derived from its single-task payload. |
| `Schedule` | Generic canceled timer with mode/outcome metadata. |
| `InvokeSubagent` | Native invocation preserving child specifications and results. |

All mappings still require full persisted/source-RPC equality before exact
presentation comparison. Unknown fields, argument names, unqualified statuses
and outcomes remain unsupported. Modern permission grants and completed
interactions remain unsupported until a native capture qualifies them. This change does not accept a transcript
merely because its step count or schema matches.

Normal sync on an isolated database copy produced a verified 276-step sidecar.
A fresh isolated watcher produced both the 276-step parent and its eight-step
child and stamped the child's parent link. It then reported two up-to-date
sessions, zero skips and zero failures. A deliberate timeout ended the watcher;
the installed service was not replaced or restarted. Markdown rendering of the
parent also completed.

## Checks

- `make check`: passed formatting, vet, lint, Go suites, build and 131 Python
  maintenance tests, with 64 skips.
- Synthetic positive fixtures cover all six types, with additional create/edit,
  command and task variants. Negative cases cover altered commands/output,
  ranges/diffs, task/child identity, unknown nested protobuf fields, unsupported
  arguments and statuses, and omitted presentation fields.
- Permission-record tests also prove that unsupported grants and completed
  interactions are refused from the source, avoiding repeated endpoint attempts.
- Race checks passed for completeness, session and watcher packages.
- `FuzzProjectionTools143`: 15-second budget, 428,259 executions, passed.
- Frozen native 276-step verification and attestation recheck passed.

## Fingerprints and coverage limits

SQLite schema remains `user_version=1`, seven tables, with fingerprint
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.

The fresh 393-session sidecar shape is
`sha256:28844594ff279094cc83ccbbf643815e5f8c9948b54fb09ffd443ba0151a4cac`,
with `explicit-version-scoped` provenance. The recorded 1.4.2 hash was
`sha256:66175b5056b03d00ae21cbf25436be7da67bb4927357c9ac54d6518dd4a9ad82`.
The original 1.4.2 path listing was not located, so this audit cannot attribute
every difference from that baseline to the release itself.

Compared with the earlier failed 392-session **1.4.3** sweep, the new corpus
adds 31 paths and removes none. Thirty are command-assessor/guardian and
experiment configuration under `metadata.agentScript` and `metadata.staticConfig`;
one is `steps.[].taskDetails.progress`. Compared with that earlier corpus
excluding parent-a, there are five further terminal-render paths. These
comparisons distinguish changing scenario coverage within 1.4.3 from a proven
cross-version schema change. The added assessor configuration is preserved
and passes full native comparison in the required child.

The release's empty-file and ordinary-edit paths, timer result, and ordinary
delegation are represented by the required parent/child. The earlier CLI audit
also reported successful valid/invalid plugin validation; that was not repeated
here because it does not establish reader compatibility.

No new interactive `/compact` or Auto-mode approval experiment was driven in
this rerun. Parent resolution reads stored invocation identities, not the
model's compacted prose summary; rendering uses stored compaction boundaries.
The new mappings preserve outer metadata, and full-source verification still
covers assessor configuration. This establishes the reader's handling of the
observed data, not that agy's new compaction or approval UX works in every case.
These limitations do not waive the native qualification gate.

The eight historical SQLite refusals remain: four unsupported generic wrappers,
two resultless file-view shapes and two generic-result metadata shapes. They
are explicitly non-gating historical cases under the audit runbook. One is
the older 11-of-784-step stale-sidecar case; this fix does not qualify that
different shape. Legacy protobuf sessions remain unverifiable. The optional
AgentsView consumer suite and live downstream ingestion were not exercised.

Recording this audit updates the checkout's compatibility baseline only. An
installed reader continues to use its old binary/profile until a separate
installation and restart.
