# Recovery cohort rerun after the post-rerun fixes, 2026-10-07

Conversation labels such as `conversation-001` are consistent aliases across
these reports, not session IDs. Use actual IDs from your own store when
rerunning commands.

This repeats the 2026-10-06 rerun (`recovery-rerun-2026-10-06.md`) on the
182 retained conversations and the nine short exports, with a reader built
from local `main` at `975d24a`. **181 of the 182 conversations now verify
through ordinary sync and a fresh watcher, and one still fails.** Of the 181,
100 have a plain valid attestation and 81 have a valid attestation that
carries the declared source-only reason (CascadePlannerConfig field 50,
ChatMessagePrompt field 23, or both). Eight of the nine short exports now
verify, and each fresh sidecar's daemon payload equals the export audited by
hand on 2026-10-03.

Counted as the 191 cases agy-reader#h7d4 names (182 conversations plus the
nine short-export comparisons, which are a subset of the 182 checked a second
way), **189 of 191 verify**. The two that do not are the same conversation,
`conversation-098`, counted once as a retained source and
once as a short export. It is tracked in agy-reader#3193. This rerun changes no
reader code.

| Result | 2026-10-06 (reader `b186735`) | 2026-10-07 (reader `975d24a`) |
| --- | ---: | ---: |
| Verified, valid attestation | 92 | 100 |
| Verified, declared source-only reason | 57 | 81 |
| Still failing | 33 | 1 |
| **Total** | **182** | **182** |
| Short exports verified (of 9) | 7 | 8 |

Every conversation that verified on 2026-10-06 still verifies, under the same
result type. The 32 that moved from failing to verified split as follows:

| 2026-10-06 refusal | Conversations | Result now | Changed by |
| --- | ---: | --- | --- |
| `unknown persisted protobuf fields` (field 23) | 15 | Verified, declared source-only reason | agy-reader#g145 (one also needed agy-reader#mbpm) |
| `unsupported generic result` (`list_permissions`) | 9 | Verified, declared source-only reason | agy-reader#jgfc |
| `unsupported resultless view_file` | 6 of 7 | Verified, valid attestation | agy-reader#2xqw (one also needed agy-reader#axnr) |
| `unsupported native tool projection` (`generate_image`) | 1 | Verified, valid attestation | agy-reader#ntzm |
| `unsupported generic arguments` (`write_to_file` `Append`) | 1 | Verified, valid attestation | agy-reader#s42p |

"Changed by" names the fix for the shape the conversation was refused on. A
control run with the 2026-10-06 reader on the same daemon refused all 33 again
with exactly their 2026-10-06 reasons (see "How it ran"), so the change comes
from the reader, not from the environment.

All six IDE conversations now verify with the source-only reason; on
2026-10-06 five did and `conversation-161` failed on `list_permissions`.

## The nine short exports

| Conversation | 2026-10-06 | Result now | Fresh sidecar against the 2026-10-03 audited export |
| --- | --- | --- | --- |
| `conversation-098` | Still failing: `unsupported resultless view_file` | Still failing: `unsupported resultless view_file` at step 62 | no sidecar written |
| `conversation-148` | Still failing: `unsupported generic arguments` | Verified, valid attestation | daemon payload identical in both copies, 33 steps |
| `conversation-022` | Verified, valid attestation | Verified, valid attestation | daemon payload identical in both copies, 3 steps |
| `conversation-051` | Verified, valid attestation | Verified, valid attestation | daemon payload identical in both copies, 10 steps |
| `conversation-077` | Verified, valid attestation | Verified, valid attestation | daemon payload identical in both copies, 57 steps |
| `conversation-119` | Verified, valid attestation | Verified, valid attestation | daemon payload identical in both copies, 6 steps |
| `conversation-125` | Verified, valid attestation | Verified, valid attestation | daemon payload identical in both copies, 147 steps |
| `conversation-177` | Verified, valid attestation | Verified, valid attestation | daemon payload identical in both copies, 76 steps |
| `conversation-211` | Verified, valid attestation | Verified, valid attestation | daemon payload identical in both copies, 170 steps |

"Identical" means the synced sidecar, with the reader's own `agyReader` block
removed, equals the audited export as parsed JSON, in both the sync copy and
the watch copy. `conversation-148` is new since 2026-10-06. `conversation-098` still has only
its one-off audited export and no standard attestation.

## What still fails

| Shape | Reader reason | Conversations | Steps | Tracked in |
| --- | --- | ---: | ---: | --- |
| Failed `view_file` with no result whose path names a skill file | `unsupported resultless view_file` | 1 | 1 | agy-reader#3193 (new) |

`conversation-098` step 62 is a failed `view_file` call on a `SKILL.md` file. The
reader refuses it on purpose: agy-reader#2xqw accepts a failed, resultless
`view_file` only when the path does not contain `skill`, because the daemon
may mark such a step `isSkillFile` and the source does not record whether it
will (`docs/completeness.md`). The 2026-10-03 capture of this step does carry
`isSkillFile: true`. It is the only refused step: the other 354 steps,
projected on their own, are accepted. The reader reaches the projection only
after the full raw RPC equals the stored source, so the stored conversation
is complete; it just cannot be attested under the current rules.

## How it ran

- **Reader:** built with `make build` from `agent/h7d4` at `975d24a`, the
  local `main` this report is committed on. Profile
  `sqlite-v1-format0-agy-1.3.0-19be6af38f7b-planner50-permission2-prompt23`.
  Every sidecar from this run carries that profile; the 2026-10-06 sidecars
  carried the older `sqlite-v1-format0-agy-a759ce7c7a23-planner50`, which this
  reader reads as unknown, so all 181 attestations here were minted fresh.
- **Daemon:** agy 1.2.16, the same retained binary as on 2026-10-06 (SHA-256
  `a759ce7c7a235d9b6c281a25ead97cbbf2e92314a3ffd224e2f9144f3fae7a86`). These
  are historical conversations, so the rerun again pairs the reader with an
  older daemon. The installed agy is now 1.3.1; the rerun was not repeated
  with it.
- **Isolation:** two fresh copies of the 182 SQLite sources, each in its own
  scratch store; the six IDE conversations were placed in the CLI store copy,
  as on 2026-10-06. agy ran under `bwrap` with the host read-only, only the
  copy writable, `HOME` redirected into the copy, and `--gemini_dir` pointing
  at the copy, so the daemon loaded only the copies. `scripts/agy-pair.py
  start` started each agy with a fresh token. Every copy matched the recovery
  manifest's SHA-256 before use, and after the run every copy still did and
  none had WAL content.
- **Sync:** against the first copy, `agy-reader --root <store> --sync <id>`
  for each of the 182 conversations, one at a time, with the pair's own
  watcher pointed at an empty root. 181 exited 0 and wrote a sidecar; 81 of
  them printed `verified except declared source-only fields absent from RPC`.
  `conversation-098` exited 1 with `completeness unknown: unsupported resultless
  view_file`.
- **Watch:** after the sync pair stopped, a new agy daemon and a fresh watcher
  (`--watch --watch-interval=10s`) on the second copy. The first tick reported
  `181 synced, 1 skipped, 0 up-to-date, 1 failed` (the watcher counts a
  refused fetch as both failed and skipped). It logged the same reason as sync
  for `conversation-098` and held the refusal. Seven later ticks each reported
  `0 synced, 1 skipped, 181 up-to-date, 0 failed`.
- **Checks on the result:** an offline diagnostic, compiled into the
  completeness package with `go test -overlay` and not part of the
  repository, re-read every copy and rechecked each sidecar's attestation with
  the reader's own `CheckAttestation`. All 181 recheck as verified in both
  stores, with identical source and payload digests between sync and watch,
  sidecar step counts equal to the source's, and the source-only record equal
  to what the source holds. The records total 252 field-50 occurrences across
  78 conversations and 47 field-23 occurrences across 15, the same numbers the
  2026-10-03 wire scan found; the field-50 count of each of the 57 conversations
  that verified on 2026-10-06 is unchanged. For `conversation-098` the diagnostic
  projected each step alone; only step 62 is refused.
- **Negative controls:** the same diagnostic tampered with each of the 181
  real sidecars and rechecked it. Every tampering stopped it verifying:

  | Tampering | Result | Conversations |
  | --- | --- | ---: |
  | Last step removed from the sidecar | `incomplete: sidecar payload changed since verification` | 181 of 181 |
  | Same, with the payload digest re-minted to match | `incomplete: daemon presentation differs from supported source projection` | 181 of 181 |
  | Checked against another conversation's source | `incomplete: persisted source content changed` | 180 of 180 |
  | Attestation relabelled with the previous profile | `unknown: missing or unsupported completeness attestation` | 181 of 181 |
  | Source-only record removed | `incomplete: sidecar source-only record changed since verification` | 81 of 81 |
  | Source-only count changed | `incomplete: sidecar source-only record changed since verification` | 81 of 81 |

  The full-raw RPC equality check and the projection mutation tests are
  unchanged in this commit and pass in `go test ./...`.
- **Control reader:** a reader built from `b186735`, the 2026-10-06 reader,
  synced a third fresh copy of the 33 conversations that failed on 2026-10-06
  against the same agy 1.2.16 under the same confinement. All 33 exited 1
  with exactly their 2026-10-06 reasons (15 `unknown persisted protobuf
  fields`, 9 `unsupported generic result`, 7 `unsupported resultless
  view_file`, 1 `unsupported native tool projection`, 1 `unsupported generic
  arguments`).
- **Originals:** the private sources under the recovery directory
  `2026-10-03-85biqu17` were only read. After the run all 182 still match the
  manifest's SHA-256, and no file there is newer than the run. The scratch
  copies, sidecars and diagnostic are not committed.

## Per-conversation results

The step number is the zero-based index of the first refused step. "Steps
refused" counts steps the projection refuses when each is checked alone. For
verified conversations, "field 50" and "field 23" give the number of declared
source-only occurrences the attestation records. "Result now" is the same for
sync and for the fresh watcher in every row. "Changed by" names the fix for a
conversation that failed on 2026-10-06.

| # | Conversation | Surface | 2026-10-06 | Short export | Result now (sync and watch) | Detail | Changed by |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | `conversation-098` | CLI | Still failing: `unsupported resultless view_file` | yes | **Still failing** | `unsupported resultless view_file` at step 62; 1 of 355 steps refused; tracked in agy-reader#3193 |  |
| 2 | `conversation-004` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 116 steps; attestation rechecks verified in both copies; field 50 ×6; field 23 ×8 | agy-reader#g145 |
| 3 | `conversation-005` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 172 steps; attestation rechecks verified in both copies; field 50 ×3 | agy-reader#jgfc |
| 4 | `conversation-021` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 214 steps; attestation rechecks verified in both copies; field 50 ×11; field 23 ×6 | agy-reader#g145 |
| 5 | `conversation-024` | CLI | Still failing: `unsupported resultless view_file` |  | Verified, valid attestation | 401 steps; attestation rechecks verified in both copies | agy-reader#2xqw |
| 6 | `conversation-035` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 88 steps; attestation rechecks verified in both copies; field 23 ×3 | agy-reader#g145 |
| 7 | `conversation-049` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 96 steps; attestation rechecks verified in both copies; field 50 ×7; field 23 ×1 | agy-reader#g145 |
| 8 | `conversation-052` | CLI | Still failing: `unsupported resultless view_file` |  | Verified, valid attestation | 66 steps; attestation rechecks verified in both copies | agy-reader#2xqw |
| 9 | `conversation-053` | CLI | Still failing: `unsupported resultless view_file` |  | Verified, valid attestation | 51 steps; attestation rechecks verified in both copies | agy-reader#2xqw |
| 10 | `conversation-055` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 310 steps; attestation rechecks verified in both copies; field 50 ×3; field 23 ×1 | agy-reader#g145 |
| 11 | `conversation-060` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 80 steps; attestation rechecks verified in both copies; field 23 ×2 | agy-reader#g145 |
| 12 | `conversation-063` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 120 steps; attestation rechecks verified in both copies; field 50 ×7; field 23 ×10 | agy-reader#g145 |
| 13 | `conversation-070` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 288 steps; attestation rechecks verified in both copies; field 50 ×8; field 23 ×1 | agy-reader#g145 |
| 14 | `conversation-075` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 22 steps; attestation rechecks verified in both copies; field 50 ×2; field 23 ×1 | agy-reader#g145 |
| 15 | `conversation-092` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 224 steps; attestation rechecks verified in both copies; field 50 ×7 | agy-reader#jgfc |
| 16 | `conversation-099` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 108 steps; attestation rechecks verified in both copies; field 50 ×3 | agy-reader#jgfc |
| 17 | `conversation-105` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified in both copies; field 50 ×2 | agy-reader#jgfc |
| 18 | `conversation-107` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 1358 steps; attestation rechecks verified in both copies; field 50 ×12; field 23 ×1 | agy-reader#g145 |
| 19 | `conversation-113` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 76 steps; attestation rechecks verified in both copies; field 50 ×3; field 23 ×2 | agy-reader#g145 |
| 20 | `conversation-148` | CLI | Still failing: `unsupported generic arguments` | yes | Verified, valid attestation | 33 steps; attestation rechecks verified in both copies | agy-reader#s42p |
| 21 | `conversation-149` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 54 steps; attestation rechecks verified in both copies; field 50 ×2 | agy-reader#jgfc |
| 22 | `conversation-155` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 88 steps; attestation rechecks verified in both copies; field 50 ×3; field 23 ×4 | agy-reader#g145 |
| 23 | `conversation-161` | IDE | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 226 steps; attestation rechecks verified in both copies; field 50 ×3 | agy-reader#jgfc |
| 24 | `conversation-162` | CLI | Still failing: `unsupported native tool projection` |  | Verified, valid attestation | 38 steps; attestation rechecks verified in both copies | agy-reader#ntzm |
| 25 | `conversation-164` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 52 steps; attestation rechecks verified in both copies; field 23 ×3 | agy-reader#g145 |
| 26 | `conversation-167` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 16 steps; attestation rechecks verified in both copies; field 50 ×4 | agy-reader#jgfc |
| 27 | `conversation-174` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 72 steps; attestation rechecks verified in both copies; field 50 ×3 | agy-reader#jgfc |
| 28 | `conversation-175` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 44 steps; attestation rechecks verified in both copies; field 50 ×2; field 23 ×1 | agy-reader#g145, agy-reader#mbpm |
| 29 | `conversation-178` | CLI | Still failing: `unsupported resultless view_file` |  | Verified, valid attestation | 1114 steps; attestation rechecks verified in both copies | agy-reader#2xqw, agy-reader#axnr |
| 30 | `conversation-179` | CLI | Still failing: `unsupported resultless view_file` |  | Verified, valid attestation | 190 steps; attestation rechecks verified in both copies | agy-reader#2xqw |
| 31 | `conversation-180` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 175 steps; attestation rechecks verified in both copies; field 50 ×10; field 23 ×3 | agy-reader#g145 |
| 32 | `conversation-195` | CLI | Still failing: `unsupported resultless view_file` |  | Verified, valid attestation | 365 steps; attestation rechecks verified in both copies | agy-reader#2xqw |
| 33 | `conversation-201` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 138 steps; attestation rechecks verified in both copies; field 50 ×2 | agy-reader#jgfc |
| 34 | `conversation-001` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 135 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 35 | `conversation-002` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 36 | `conversation-003` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 13 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 37 | `conversation-006` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 38 | `conversation-008` | IDE | Verified, declared source-only reason |  | Verified, declared source-only reason | 66 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 39 | `conversation-009` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 112 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 40 | `conversation-013` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 41 | `conversation-014` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 42 | `conversation-015` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 18 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 43 | `conversation-020` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 44 | `conversation-025` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 56 steps; attestation rechecks verified in both copies; field 50 ×5 |  |
| 45 | `conversation-029` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 46 | `conversation-033` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 64 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 47 | `conversation-042` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 78 steps; attestation rechecks verified in both copies; field 50 ×7 |  |
| 48 | `conversation-045` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 49 | `conversation-048` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 50 | `conversation-050` | IDE | Verified, declared source-only reason |  | Verified, declared source-only reason | 6 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 51 | `conversation-056` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 52 | `conversation-057` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 55 steps; attestation rechecks verified in both copies; field 50 ×9 |  |
| 53 | `conversation-062` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 54 | `conversation-065` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 55 | `conversation-069` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 56 | `conversation-079` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 21 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 57 | `conversation-082` | IDE | Verified, declared source-only reason |  | Verified, declared source-only reason | 94 steps; attestation rechecks verified in both copies; field 50 ×6 |  |
| 58 | `conversation-083` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 59 | `conversation-085` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 3 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 60 | `conversation-087` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 61 | `conversation-093` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 56 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 62 | `conversation-097` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 7 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 63 | `conversation-111` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 64 | `conversation-115` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 20 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 65 | `conversation-120` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 90 steps; attestation rechecks verified in both copies; field 50 ×4 |  |
| 66 | `conversation-123` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 67 | `conversation-124` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 68 | `conversation-127` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 69 | `conversation-130` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 70 | `conversation-133` | IDE | Verified, declared source-only reason |  | Verified, declared source-only reason | 12 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 71 | `conversation-137` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 72 | `conversation-138` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 73 | `conversation-141` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 74 | `conversation-142` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 75 | `conversation-150` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 76 | `conversation-152` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 7 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 77 | `conversation-157` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 78 | `conversation-159` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 79 | `conversation-160` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 46 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 80 | `conversation-166` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 8 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 81 | `conversation-171` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 44 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 82 | `conversation-172` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 83 | `conversation-184` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 84 | `conversation-185` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 56 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 85 | `conversation-187` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 86 | `conversation-190` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 24 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 87 | `conversation-192` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 132 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 88 | `conversation-208` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 6 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 89 | `conversation-209` | IDE | Verified, declared source-only reason |  | Verified, declared source-only reason | 68 steps; attestation rechecks verified in both copies; field 50 ×7 |  |
| 90 | `conversation-212` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 91 | `conversation-007` | CLI | Verified, valid attestation |  | Verified, valid attestation | 22 steps; attestation rechecks verified in both copies |  |
| 92 | `conversation-010` | CLI | Verified, valid attestation |  | Verified, valid attestation | 146 steps; attestation rechecks verified in both copies |  |
| 93 | `conversation-012` | CLI | Verified, valid attestation |  | Verified, valid attestation | 106 steps; attestation rechecks verified in both copies |  |
| 94 | `conversation-016` | CLI | Verified, valid attestation |  | Verified, valid attestation | 89 steps; attestation rechecks verified in both copies |  |
| 95 | `conversation-018` | CLI | Verified, valid attestation |  | Verified, valid attestation | 30 steps; attestation rechecks verified in both copies |  |
| 96 | `conversation-022` | CLI | Verified, valid attestation | yes | Verified, valid attestation | 3 steps; attestation rechecks verified in both copies |  |
| 97 | `conversation-023` | CLI | Verified, valid attestation |  | Verified, valid attestation | 92 steps; attestation rechecks verified in both copies |  |
| 98 | `conversation-026` | CLI | Verified, valid attestation |  | Verified, valid attestation | 741 steps; attestation rechecks verified in both copies |  |
| 99 | `conversation-027` | CLI | Verified, valid attestation |  | Verified, valid attestation | 61 steps; attestation rechecks verified in both copies |  |
| 100 | `conversation-030` | CLI | Verified, valid attestation |  | Verified, valid attestation | 5 steps; attestation rechecks verified in both copies |  |
| 101 | `conversation-031` | CLI | Verified, valid attestation |  | Verified, valid attestation | 76 steps; attestation rechecks verified in both copies |  |
| 102 | `conversation-034` | CLI | Verified, valid attestation |  | Verified, valid attestation | 44 steps; attestation rechecks verified in both copies |  |
| 103 | `conversation-036` | CLI | Verified, valid attestation |  | Verified, valid attestation | 169 steps; attestation rechecks verified in both copies |  |
| 104 | `conversation-037` | CLI | Verified, valid attestation |  | Verified, valid attestation | 92 steps; attestation rechecks verified in both copies |  |
| 105 | `conversation-038` | CLI | Verified, valid attestation |  | Verified, valid attestation | 261 steps; attestation rechecks verified in both copies |  |
| 106 | `conversation-040` | CLI | Verified, valid attestation |  | Verified, valid attestation | 44 steps; attestation rechecks verified in both copies |  |
| 107 | `conversation-043` | CLI | Verified, valid attestation |  | Verified, valid attestation | 475 steps; attestation rechecks verified in both copies |  |
| 108 | `conversation-044` | CLI | Verified, valid attestation |  | Verified, valid attestation | 1877 steps; attestation rechecks verified in both copies |  |
| 109 | `conversation-046` | CLI | Verified, valid attestation |  | Verified, valid attestation | 106 steps; attestation rechecks verified in both copies |  |
| 110 | `conversation-047` | CLI | Verified, valid attestation |  | Verified, valid attestation | 33 steps; attestation rechecks verified in both copies |  |
| 111 | `conversation-051` | CLI | Verified, valid attestation | yes | Verified, valid attestation | 10 steps; attestation rechecks verified in both copies |  |
| 112 | `conversation-054` | CLI | Verified, valid attestation |  | Verified, valid attestation | 822 steps; attestation rechecks verified in both copies |  |
| 113 | `conversation-058` | CLI | Verified, valid attestation |  | Verified, valid attestation | 84 steps; attestation rechecks verified in both copies |  |
| 114 | `conversation-059` | CLI | Verified, valid attestation |  | Verified, valid attestation | 110 steps; attestation rechecks verified in both copies |  |
| 115 | `conversation-061` | CLI | Verified, valid attestation |  | Verified, valid attestation | 8 steps; attestation rechecks verified in both copies |  |
| 116 | `conversation-064` | CLI | Verified, valid attestation |  | Verified, valid attestation | 540 steps; attestation rechecks verified in both copies |  |
| 117 | `conversation-066` | CLI | Verified, valid attestation |  | Verified, valid attestation | 128 steps; attestation rechecks verified in both copies |  |
| 118 | `conversation-071` | CLI | Verified, valid attestation |  | Verified, valid attestation | 65 steps; attestation rechecks verified in both copies |  |
| 119 | `conversation-072` | CLI | Verified, valid attestation |  | Verified, valid attestation | 84 steps; attestation rechecks verified in both copies |  |
| 120 | `conversation-073` | CLI | Verified, valid attestation |  | Verified, valid attestation | 153 steps; attestation rechecks verified in both copies |  |
| 121 | `conversation-076` | CLI | Verified, valid attestation |  | Verified, valid attestation | 271 steps; attestation rechecks verified in both copies |  |
| 122 | `conversation-077` | CLI | Verified, valid attestation | yes | Verified, valid attestation | 57 steps; attestation rechecks verified in both copies |  |
| 123 | `conversation-078` | CLI | Verified, valid attestation |  | Verified, valid attestation | 46 steps; attestation rechecks verified in both copies |  |
| 124 | `conversation-080` | CLI | Verified, valid attestation |  | Verified, valid attestation | 154 steps; attestation rechecks verified in both copies |  |
| 125 | `conversation-081` | CLI | Verified, valid attestation |  | Verified, valid attestation | 6 steps; attestation rechecks verified in both copies |  |
| 126 | `conversation-084` | CLI | Verified, valid attestation |  | Verified, valid attestation | 42 steps; attestation rechecks verified in both copies |  |
| 127 | `conversation-086` | CLI | Verified, valid attestation |  | Verified, valid attestation | 78 steps; attestation rechecks verified in both copies |  |
| 128 | `conversation-088` | CLI | Verified, valid attestation |  | Verified, valid attestation | 24 steps; attestation rechecks verified in both copies |  |
| 129 | `conversation-089` | CLI | Verified, valid attestation |  | Verified, valid attestation | 273 steps; attestation rechecks verified in both copies |  |
| 130 | `conversation-090` | CLI | Verified, valid attestation |  | Verified, valid attestation | 28 steps; attestation rechecks verified in both copies |  |
| 131 | `conversation-091` | CLI | Verified, valid attestation |  | Verified, valid attestation | 152 steps; attestation rechecks verified in both copies |  |
| 132 | `conversation-095` | CLI | Verified, valid attestation |  | Verified, valid attestation | 18 steps; attestation rechecks verified in both copies |  |
| 133 | `conversation-100` | CLI | Verified, valid attestation |  | Verified, valid attestation | 52 steps; attestation rechecks verified in both copies |  |
| 134 | `conversation-101` | CLI | Verified, valid attestation |  | Verified, valid attestation | 5 steps; attestation rechecks verified in both copies |  |
| 135 | `conversation-102` | CLI | Verified, valid attestation |  | Verified, valid attestation | 10 steps; attestation rechecks verified in both copies |  |
| 136 | `conversation-103` | CLI | Verified, valid attestation |  | Verified, valid attestation | 302 steps; attestation rechecks verified in both copies |  |
| 137 | `conversation-104` | CLI | Verified, valid attestation |  | Verified, valid attestation | 276 steps; attestation rechecks verified in both copies |  |
| 138 | `conversation-106` | CLI | Verified, valid attestation |  | Verified, valid attestation | 5 steps; attestation rechecks verified in both copies |  |
| 139 | `conversation-108` | CLI | Verified, valid attestation |  | Verified, valid attestation | 253 steps; attestation rechecks verified in both copies |  |
| 140 | `conversation-109` | CLI | Verified, valid attestation |  | Verified, valid attestation | 88 steps; attestation rechecks verified in both copies |  |
| 141 | `conversation-112` | CLI | Verified, valid attestation |  | Verified, valid attestation | 94 steps; attestation rechecks verified in both copies |  |
| 142 | `conversation-116` | CLI | Verified, valid attestation |  | Verified, valid attestation | 18 steps; attestation rechecks verified in both copies |  |
| 143 | `conversation-118` | CLI | Verified, valid attestation |  | Verified, valid attestation | 42 steps; attestation rechecks verified in both copies |  |
| 144 | `conversation-119` | CLI | Verified, valid attestation | yes | Verified, valid attestation | 6 steps; attestation rechecks verified in both copies |  |
| 145 | `conversation-122` | CLI | Verified, valid attestation |  | Verified, valid attestation | 97 steps; attestation rechecks verified in both copies |  |
| 146 | `conversation-125` | CLI | Verified, valid attestation | yes | Verified, valid attestation | 147 steps; attestation rechecks verified in both copies |  |
| 147 | `conversation-126` | CLI | Verified, valid attestation |  | Verified, valid attestation | 32 steps; attestation rechecks verified in both copies |  |
| 148 | `conversation-128` | CLI | Verified, valid attestation |  | Verified, valid attestation | 58 steps; attestation rechecks verified in both copies |  |
| 149 | `conversation-129` | CLI | Verified, valid attestation |  | Verified, valid attestation | 369 steps; attestation rechecks verified in both copies |  |
| 150 | `conversation-131` | CLI | Verified, valid attestation |  | Verified, valid attestation | 39 steps; attestation rechecks verified in both copies |  |
| 151 | `conversation-135` | CLI | Verified, valid attestation |  | Verified, valid attestation | 27 steps; attestation rechecks verified in both copies |  |
| 152 | `conversation-136` | CLI | Verified, valid attestation |  | Verified, valid attestation | 127 steps; attestation rechecks verified in both copies |  |
| 153 | `conversation-139` | CLI | Verified, valid attestation |  | Verified, valid attestation | 198 steps; attestation rechecks verified in both copies |  |
| 154 | `conversation-140` | CLI | Verified, valid attestation |  | Verified, valid attestation | 22 steps; attestation rechecks verified in both copies |  |
| 155 | `conversation-143` | CLI | Verified, valid attestation |  | Verified, valid attestation | 14 steps; attestation rechecks verified in both copies |  |
| 156 | `conversation-145` | CLI | Verified, valid attestation |  | Verified, valid attestation | 201 steps; attestation rechecks verified in both copies |  |
| 157 | `conversation-146` | CLI | Verified, valid attestation |  | Verified, valid attestation | 35 steps; attestation rechecks verified in both copies |  |
| 158 | `conversation-147` | CLI | Verified, valid attestation |  | Verified, valid attestation | 9 steps; attestation rechecks verified in both copies |  |
| 159 | `conversation-153` | CLI | Verified, valid attestation |  | Verified, valid attestation | 44 steps; attestation rechecks verified in both copies |  |
| 160 | `conversation-154` | CLI | Verified, valid attestation |  | Verified, valid attestation | 59 steps; attestation rechecks verified in both copies |  |
| 161 | `conversation-158` | CLI | Verified, valid attestation |  | Verified, valid attestation | 44 steps; attestation rechecks verified in both copies |  |
| 162 | `conversation-163` | CLI | Verified, valid attestation |  | Verified, valid attestation | 42 steps; attestation rechecks verified in both copies |  |
| 163 | `conversation-165` | CLI | Verified, valid attestation |  | Verified, valid attestation | 70 steps; attestation rechecks verified in both copies |  |
| 164 | `conversation-169` | CLI | Verified, valid attestation |  | Verified, valid attestation | 19 steps; attestation rechecks verified in both copies |  |
| 165 | `conversation-170` | CLI | Verified, valid attestation |  | Verified, valid attestation | 114 steps; attestation rechecks verified in both copies |  |
| 166 | `conversation-173` | CLI | Verified, valid attestation |  | Verified, valid attestation | 154 steps; attestation rechecks verified in both copies |  |
| 167 | `conversation-176` | CLI | Verified, valid attestation |  | Verified, valid attestation | 73 steps; attestation rechecks verified in both copies |  |
| 168 | `conversation-177` | CLI | Verified, valid attestation | yes | Verified, valid attestation | 76 steps; attestation rechecks verified in both copies |  |
| 169 | `conversation-181` | CLI | Verified, valid attestation |  | Verified, valid attestation | 5 steps; attestation rechecks verified in both copies |  |
| 170 | `conversation-182` | CLI | Verified, valid attestation |  | Verified, valid attestation | 26 steps; attestation rechecks verified in both copies |  |
| 171 | `conversation-189` | CLI | Verified, valid attestation |  | Verified, valid attestation | 38 steps; attestation rechecks verified in both copies |  |
| 172 | `conversation-191` | CLI | Verified, valid attestation |  | Verified, valid attestation | 321 steps; attestation rechecks verified in both copies |  |
| 173 | `conversation-193` | CLI | Verified, valid attestation |  | Verified, valid attestation | 304 steps; attestation rechecks verified in both copies |  |
| 174 | `conversation-194` | CLI | Verified, valid attestation |  | Verified, valid attestation | 132 steps; attestation rechecks verified in both copies |  |
| 175 | `conversation-198` | CLI | Verified, valid attestation |  | Verified, valid attestation | 74 steps; attestation rechecks verified in both copies |  |
| 176 | `conversation-199` | CLI | Verified, valid attestation |  | Verified, valid attestation | 186 steps; attestation rechecks verified in both copies |  |
| 177 | `conversation-200` | CLI | Verified, valid attestation |  | Verified, valid attestation | 309 steps; attestation rechecks verified in both copies |  |
| 178 | `conversation-202` | CLI | Verified, valid attestation |  | Verified, valid attestation | 70 steps; attestation rechecks verified in both copies |  |
| 179 | `conversation-205` | CLI | Verified, valid attestation |  | Verified, valid attestation | 66 steps; attestation rechecks verified in both copies |  |
| 180 | `conversation-206` | CLI | Verified, valid attestation |  | Verified, valid attestation | 84 steps; attestation rechecks verified in both copies |  |
| 181 | `conversation-207` | CLI | Verified, valid attestation |  | Verified, valid attestation | 381 steps; attestation rechecks verified in both copies |  |
| 182 | `conversation-211` | CLI | Verified, valid attestation | yes | Verified, valid attestation | 170 steps; attestation rechecks verified in both copies |  |
