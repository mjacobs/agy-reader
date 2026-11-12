# agy 1.5.6 compatibility

All fifteen required conversations pass native SQLite/full/default RPC
verification, offline JSON equality, ordinary sync and saved-attestation
validation. The supplied coordinator has 314 steps and the Stop-hook session
21. A bounded native swarm exercise added a 35-step coordinator and six child
conversations. All five successfully returned workers nest beneath their
creation steps; the child left by a failed add stays visible in the trailing
section. No production reader was installed or restarted.

## Reader changes

The `tools156` profile adds these captured representations:

- Hook-free canceled background commands with a 5000 ms wait, task details and
  background timestamp, completion time, pending/running/canceled history, no
  bypass argument, and a render-only result. Historical allowing-hook rules
  keep their existing bounds. The complete wrapper must match full RPC before
  the displayed command is synthesized; no exit code is invented.
- Completed swarm model switches for workers and coordinator; omitted model
  and workspace defaults; `share` and `branch` workspaces; repeated workers
  with `n`; task lists; and member-name `fork` selectors. The entire generic
  wrapper is preserved. Combined `n` and `tasks`, unknown arguments and
  uncaptured failures remain refused.
- Board reads without arguments, with since/limit together, a single sequence
  ID, or status plus limit; waits with true or false mention filtering.
- The captured mid-turn model-switch error, failed coordinator-fork add,
  duplicate member error, and the canceled in-flight add copied into that
  forked child's history. These keep their complete wrappers and errors;
  failed adds never supply child-placement IDs.
- Timestamped empty-file reads and timestamped folded skill descriptions.
  Two RFC3339 timestamp lines are checked, the saved content stays intact,
  and all prior zero-count/range and frontmatter guards remain.

The embedded descriptor gains only `ExecutorTerminationReason` value 12,
`EXECUTOR_TERMINATION_REASON_CHAINED`, observed on the coordinator after a
model switch. `generate_tools156.py` authenticates the 1.5.6 closure and prior
bundle, copies this single declaration and verifies pool loading. Maintenance
tests prove exact preservation and reproduction of the rest of the bundle.
No source-only exception changed. Old attestations require reverification.

Multi-worker results contain parallel `names` and `conversation_ids` arrays.
Placement checks their lengths against `n` or the task count, ordered worker
suffixes, and unique nonempty IDs. Only children already linked by the resolver
can be placed. Missing, duplicate, reordered, contradictory and failed results
cannot steal another child; unmatched children remain visible.

## Native evidence

| Scenario | Steps | Result |
| --- | ---: | --- |
| Supplied coordinator | 314 | Two hook-free cancellations, two timestamped empty reads, folded skill read and basic swarm calls verified |
| Supplied Stop-hook session | 21 | Fresh daemon loads all saved steps; offline output equals displayed RPC |
| Six retained swarm/board conversations | 553 total | Defaults and additional board argument sets verified |
| Exercise coordinator | 35 | Repeated workers, task workers, model switches, branch fork, errors and kill verified |
| Six exercise children | 55 total | Full histories, including copied canceled fork, verified |

The fifteen private captures pass `TestNativeEvidence`, including saved
attestation validation. Offline JSON equals the complete displayed RPC after
removing reader provenance. Ordinary sync and `audit-qualify` pass in the
private store. Native rendering places both repeated workers at step 3, both
task workers at step 9, and the successful branch worker at step 29 (one-based).
Its stderr is empty when rendered from an exercise-only store.

The final sweep fetched and rendered 508/508 conversations, zero failures;
492/508 natively verify. The sixteen refusals are six remaining projection
cases tracked in agy-reader#jqch and the ten source/full-RPC mismatches tracked
in agy-reader#cpqn. The latter have the same IDs and reasons as the retained
1.5.5 audit. A fully verified historical corpus is not claimed.

Go package tests, race tests, vet and lint pass. Python maintenance tests run
177 cases with 17 skips. Focused synthetic tests exercise the new mappings,
full/default mismatches, malformed arguments and results, error boundaries,
unknown enums, profile invalidation and safe multi-worker placement.

## Schema, protobuf and corpus provenance

SQLite remains `user_version=1`, seven tables and two explicit indices;
its fingerprint is unchanged:
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.

The [verified protobuf snapshot](../../artifacts/proto-snapshots/agy-1.5.6-7e5bbdcbf689/snapshot.json)
contains 399 authenticated embedded records, 34 closure files and 34 compiled
round-trip files. Verification rechecked all 399 byte ranges against the
installed binary, SHA-256
`7e5bbdcbf689354300500ba22d55c2ff1497e185ae18407325d480577a87db5d`.
The serving daemon startup log reports 1.5.6 and its process executable matches
that hash. Compared with 1.5.5 there are nine additive wire changes, zero
JSON-facing changes, and one descriptor-only deprecation. Additions include
Stop-hook continuation, compaction configuration, file-range fields, MCP tool
and sidecar title-bar declarations. Names do not establish runtime behavior;
these other declarations are not blanket additions to the production decoder.

The full corpus shape differs from the prior audit because the selected
conversation sets differ. The 460 conversations common to the initial 1.5.5
and 1.5.6 sweeps have exactly the same path/type union: zero additions and zero
removals. Corpus differences therefore are not evidence that 1.5.6 removed a
rendered field. The generated compatibility record contains the final shape
fingerprint and explicit-version-scoped provenance.

## Exercise and operational limits

Changelog verdict: swarm coordinator behavior, Stop-hook continuation, model
configuration and native gateway tool declarations merit inspection. The
captured sources cover swarm tools, continuations and command cancellation.
TUI bees/navigation, plugin import/rollback, gateway delivery, custom-model
output limits, ADC access and sandbox approval behavior were not behaviorally
qualified. The exercise account refused Auto mode and used its default mode;
the hook-free cancellation evidence comes from the supplied session.

The bounded exercise created six workers in total, with at most five live
workers. A coordinator fork first failed after registering a child; after
killing that member, a branch fork from an idle worker succeeded. Both worker
and coordinator model calls selected the current model, so this verifies
serialization of model switching, not behavior under a different provider.
Workers only posted completion messages. `fork` was a member name, not the
boolean suggested by the initial exercise request. Swarm workspace selectors
and optional arguments are not claimed to have originated in this release.

The supplied Stop-hook log independently confirms camelCase
`continuationActive: false` at execution numbers 0, 1 and 2. The reader retains
the resulting trajectory; it cannot repair the upstream continuation flag.
The stale live daemon's one-step response is not completeness evidence: a
fresh daemon loaded and matched all 21 persisted steps.

An initial audit daemon ignored directory override variables and used the
normal store. It was stopped after this was detected. The expired source was
restored from the hash-verified backup. Later daemons used an isolated HOME;
their logs confirm the private storage path. Sources expired by the bounded
exercise's retention were likewise restored without overwriting current files.
All originally backed-up source names remained present after recovery. Live
sidecars were never written by the reader audit. Private sources, RPC captures,
logs and comparison output remain under ignored `tmp/agy-1.5.6-audit/`.
