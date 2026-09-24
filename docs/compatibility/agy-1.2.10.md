# agy 1.2.10 compatibility audit

The 2026-09-24 audit fetched and rendered all 240 selected conversations through
the agy 1.2.10 daemon without errors. SQLite structure is unchanged, and the
reader preserved the newly exercised payload fields. A live three-worker test
also exposed a reader parent-link defect: bidirectional messages can create a
cycle in reader-owned metadata. This is a passing storage and raw-payload audit,
not a claim that subagent ancestry is correct.

The audited reader commit was `4b40487`, with a clean working tree before the
record was generated. See the generated [compatibility record](../../COMPATIBILITY.md)
and the [canonical field/type snapshot](agy-1.2.10-sidecar-shape.tsv).

## Release-driven scenario

The installed `agy changelog` supplied the release notes from 1.2.6 through
1.2.10. Three native workers processed a synthetic ledger concurrently. Two
used isolated worktrees; the third inherited the coordinator's workspace.
Each ran a staged command with flushed output and eight-second pauses. The
coordinator sent a progress request, scheduled timers, waited for completion,
and reconciled the results.

The ledger contained seven rows, five unique IDs, and two identical duplicates.
Unique deltas were `5, 7, -2, 10, -3`, giving a total of **17**. Conflicting
duplicate values had to be rejected. The maker implemented a reducer; the
checker independently implemented and tested the contract; the reviewer
checked the interpretation. This exercised real tool calls, not simulated roles.

| Release change | Observation |
| --- | --- |
| 1.2.10: subagent worktrees moved out of artifact directories | Both worktrees lived under the CLI's `worktrees/` directory. Ordinary Python, text, and JSON files were created successfully. Neither worktree contained a `*.metadata.json` file. |
| 1.2.9: `@subagent` messaging | An `@ledger-maker` prompt led to `send_message` targeting the same completed worker. It resumed, ran a read-only calculation, and returned total 17 with three duplicates. No fourth worker was created. This verifies the observed routing workflow, not the parser in isolation. |
| 1.2.8: history handling for background, messaging, management, and scheduling steps | The live history exercised all four families and was fetched and rendered successfully. This did not force automatic context compaction. |
| 1.2.10: nonzero command handling | The checker deliberately exited 7, then continued and passed four unit tests. The sidecar preserved exit code 7 in a completed command step. The terminal title change from `Errored` to `Failed` was not separately verified. |
| 1.2.10: sandbox scratch access | Background work ran successfully from a worker's own scratch directory. Initial shell writes to the checkout were read-only; file-edit tools worked. A reviewer write to the coordinator's artifact directory was rejected, then recovered in its own scratch directory. |

All workers finished before the coordinator emitted its completion marker.
The maker returned total 17, five unique records, and two duplicates; the
checker's **4/4 tests passed**. The later direct-message calculation returned
17 and three duplicates. Existing Gemini 3.8 Flash model selection was inherited
by the workers. Monetary cost was not exposed in the inspected trajectory data.

An initial terminal paste split the prompt into queued messages. That setup
attempt was stopped after three steps and excluded from scenario completion
counts. It remains in the broad corpus. The successful run needed one steering
message to use the native worker-definition and invocation tools.

## Watcher evidence

An external observer sampled the databases and sidecars every three seconds;
the reader retained its normal 30-second polling interval. All four successful
scenario conversations had intermediate sidecars containing running steps.
This demonstrates periodic snapshots of active work, not lossless capture of
every streamed token.

| Conversation | Observed nonempty sidecar step counts | Final DB / sidecar steps |
| --- | --- | --- |
| Coordinator | 7, 19, 29, 33, 42, 50, 67, 74, 79, 84 | 84 / 84 |
| Maker, including follow-up | 6, 15, 21, 26, 27 | 27 / 27 |
| Checker | 9, 17, 24 | 24 / 24 |
| Reviewer | 13, 27, 35 | 35 / 35 |

All four databases, plus the interrupted setup database, returned `ok` from
SQLite `PRAGMA quick_check`. The reviewer retained one error step for the
rejected artifact-directory write; its later work completed. The reader also
recovered automatically after one failed tick during the setup daemon restart.

## Parent-link finding

**Finding at audit time: bidirectional messages created parent cycles.**
The coordinator was stamped with the maker as its parent;
the maker was subsequently stamped with the coordinator as its parent. All
three workers' raw `metadata.parentConversationId` fields correctly identified
the coordinator. The coordinator's `invokeSubagent.results[].conversationId`
values also identified the workers.

The reader's `collectMessageEvidence` function treats a corroborated
sender-to-recipient exchange as child-to-parent evidence. A coordinator asking
a worker for progress therefore supplies evidence in the wrong direction.
Existing stamps are then preserved as authoritative. The reader currently
does not use the raw parent and invocation-result fields for this decision.
Both fields already occur in the 1.2.5 snapshot, so this is a reproduced reader
inference defect, not evidence that 1.2.10 introduced a format regression.

A separate two-sidecar synthetic reproduction included correct child parent
metadata and bidirectional `send_message` / `agent_message` pairs. Running
`agy-reader backfill-parent-links --root <repro-root>` produced
`scanned=2 stamped=2 unchanged=0 unresolved=0 diagnostics=0`, with each sidecar
pointing to the other. Raw daemon payloads remain intact, but consumers of
`agyReader.parentCascadeId` can display the wrong hierarchy. No production
code or existing stamps were repaired during this audit.

## Corpus and shape comparison

Every selected conversation was fetched anew with `LoadTrajectory` and
`GetCascadeTrajectory`, including the internal cascade-ID fallback where
needed. Responses went into a new private corpus; no cached sidecar fallback
was permitted. The live executable matched the installed 1.2.10 binary.

| Measure | Result |
| --- | ---: |
| Selected / freshly fetched / rendered | 240 / 240 / 240 |
| Fetch or render failures | 0 |
| Populated / empty trajectories | 219 / 21 |
| Steps / distinct step types | 22,157 / 21 |
| Done / canceled / error steps | 21,993 / 62 / 102 |
| Canonical field/type entries | 1,736 |
| Added / removed entries versus 1.2.5 | 107 / 0 |
| Prior corpus members retained | 229 / 229 |
| Shape additions / removals on those same 229 members | 0 / 0 |

Schema fingerprint:
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
Sidecar shape fingerprint:
`sha256:3acca61c897bdec898292c1646090faf1913781f0a4d11ca389bc043ba4ec92a`.

The full-corpus shape drift contains 96 configuration paths under
`metadata.agentScript` and `metadata.staticConfig`, the worker's
`metadata.subagentSpec.share` field, and ten step paths. The latter include
`invokeSubagent.subagents.[].share`, `runCommand.sandboxBypassDisabled`, and
user-configuration paths. No existing paths were removed or changed in type.
All additions come from the eleven conversations added since the prior
baseline; refreshing the same old 229 conversations reproduced their original
field list exactly. This does not distinguish newly introduced fields from
previously unexercised optional features.

The changelog has compatibility-relevant notes about large-diff elision,
fork/history behavior, concurrent SQLite locks, and worktree paths. It declares
no schema or encryption migration. Large-diff elision, fork/rewind behavior,
headless shutdown/error/background-wait fixes, and UI rendering were not tested
here. `/compact` was unavailable in this session. Agentsview integration tests
were not run.

## Recording and reproduction

After reviewing the shape delta, the following audit completed with exit 0:

```sh
AGY_SIDECAR_CORPUS=/path/to/fresh-corpus \
  skills/agy-format-audit/scripts/audit_format.sh --record --corpus-swept
```

The helper reported schema `UNCHANGED`, sidecar shape `DRIFT`, and reader unit
tests `PASS`. The explicit corpus scope is justified by 240 fresh daemon
responses. Its generated record covers the format checks; the parent-link
finding above limits the broader integration claim.

With the CLI root selected, `go run . doctor` returned exit 0 and reported
240/240 fresh sidecars. The installed reader still embeds the previous
compatibility record; its version warning will clear on its next rebuild.
The running watcher itself remained healthy.

Private evidence retains the prompts, polling observations, sidecar snapshots,
session manifest, coverage totals, audit output, and synthetic parent-cycle
reproduction. The repository snapshot contains field names and types only;
raw transcripts and live conversation IDs are not included. Future audits can
reuse the comparison procedure in the [1.2.5 report](agy-1.2.5.md).
