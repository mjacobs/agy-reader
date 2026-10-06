# Recovery cohort rerun after the post-rerun fixes, 2026-10-07

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
`7b3888f6-9f76-4504-969a-187c95705773`, counted once as a retained source and
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
2026-10-06 five did and `c0d48394` failed on `list_permissions`.

## The nine short exports

| Conversation | 2026-10-06 | Result now | Fresh sidecar against the 2026-10-03 audited export |
| --- | --- | --- | --- |
| `7b3888f6-9f76-4504-969a-187c95705773` | Still failing: `unsupported resultless view_file` | Still failing: `unsupported resultless view_file` at step 62 | no sidecar written |
| `b1211347-041b-496c-b7d6-c8c48a059f89` | Still failing: `unsupported generic arguments` | Verified, valid attestation | daemon payload identical in both copies, 33 steps |
| `16cdf910-8b9a-4c4b-80aa-6cbc7fc10f2f` | Verified, valid attestation | Verified, valid attestation | daemon payload identical in both copies, 3 steps |
| `3ee509bc-71e6-43f6-925e-184cde5a6bef` | Verified, valid attestation | Verified, valid attestation | daemon payload identical in both copies, 10 steps |
| `66a24054-e957-4494-879b-39d1dffc59fb` | Verified, valid attestation | Verified, valid attestation | daemon payload identical in both copies, 57 steps |
| `961cfb93-0c77-45d2-b76a-2efbb7cf844d` | Verified, valid attestation | Verified, valid attestation | daemon payload identical in both copies, 6 steps |
| `9af85a5e-4afa-4a96-aecf-f307e6d0c740` | Verified, valid attestation | Verified, valid attestation | daemon payload identical in both copies, 147 steps |
| `e1a27237-8e78-4698-acef-004c847c4dec` | Verified, valid attestation | Verified, valid attestation | daemon payload identical in both copies, 76 steps |
| `ff7b965e-835c-4905-af72-f3d9d1fb0baa` | Verified, valid attestation | Verified, valid attestation | daemon payload identical in both copies, 170 steps |

"Identical" means the synced sidecar, with the reader's own `agyReader` block
removed, equals the audited export as parsed JSON, in both the sync copy and
the watch copy. `b1211347` is new since 2026-10-06. `7b3888f6` still has only
its one-off audited export and no standard attestation.

## What still fails

| Shape | Reader reason | Conversations | Steps | Tracked in |
| --- | --- | ---: | ---: | --- |
| Failed `view_file` with no result whose path names a skill file | `unsupported resultless view_file` | 1 | 1 | agy-reader#3193 (new) |

`7b3888f6` step 62 is a failed `view_file` call on a `SKILL.md` file. The
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
  `7b3888f6` exited 1 with `completeness unknown: unsupported resultless
  view_file`.
- **Watch:** after the sync pair stopped, a new agy daemon and a fresh watcher
  (`--watch --watch-interval=10s`) on the second copy. The first tick reported
  `181 synced, 1 skipped, 0 up-to-date, 1 failed` (the watcher counts a
  refused fetch as both failed and skipped). It logged the same reason as sync
  for `7b3888f6` and held the refusal. Seven later ticks each reported
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
  that verified on 2026-10-06 is unchanged. For `7b3888f6` the diagnostic
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
| 1 | `7b3888f6-9f76-4504-969a-187c95705773` | CLI | Still failing: `unsupported resultless view_file` | yes | **Still failing** | `unsupported resultless view_file` at step 62; 1 of 355 steps refused; tracked in agy-reader#3193 |  |
| 2 | `06a18622-4a71-4ac5-b7c7-c9570319d1df` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 116 steps; attestation rechecks verified in both copies; field 50 ×6; field 23 ×8 | agy-reader#g145 |
| 3 | `06b66779-eebe-4869-ba1b-ccf3d42be70b` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 172 steps; attestation rechecks verified in both copies; field 50 ×3 | agy-reader#jgfc |
| 4 | `169edd42-df38-4bf3-a682-b4394ff877f4` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 214 steps; attestation rechecks verified in both copies; field 50 ×11; field 23 ×6 | agy-reader#g145 |
| 5 | `19017719-a37d-43ef-b7ea-6e442107b96e` | CLI | Still failing: `unsupported resultless view_file` |  | Verified, valid attestation | 401 steps; attestation rechecks verified in both copies | agy-reader#2xqw |
| 6 | `24eb7de5-2fc4-4595-9f3f-5d2223d0f2c8` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 88 steps; attestation rechecks verified in both copies; field 23 ×3 | agy-reader#g145 |
| 7 | `3cae60ae-0915-4a4e-b2cf-d04ecc04b0b9` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 96 steps; attestation rechecks verified in both copies; field 50 ×7; field 23 ×1 | agy-reader#g145 |
| 8 | `4085478b-647c-4e3e-9d36-86ae979a51c4` | CLI | Still failing: `unsupported resultless view_file` |  | Verified, valid attestation | 66 steps; attestation rechecks verified in both copies | agy-reader#2xqw |
| 9 | `40b43605-9cdc-4d5a-8179-1eb671ed3435` | CLI | Still failing: `unsupported resultless view_file` |  | Verified, valid attestation | 51 steps; attestation rechecks verified in both copies | agy-reader#2xqw |
| 10 | `4304cac0-6cf7-4b4d-bcb0-3f7ccd902b66` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 310 steps; attestation rechecks verified in both copies; field 50 ×3; field 23 ×1 | agy-reader#g145 |
| 11 | `4c47d422-0502-40a9-a676-7df0d424cb34` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 80 steps; attestation rechecks verified in both copies; field 23 ×2 | agy-reader#g145 |
| 12 | `4eb0934c-89a7-454f-8bbc-99ea73c88717` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 120 steps; attestation rechecks verified in both copies; field 50 ×7; field 23 ×10 | agy-reader#g145 |
| 13 | `61f98d57-fbc1-4cd9-bf41-ee14a3ba1c79` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 288 steps; attestation rechecks verified in both copies; field 50 ×8; field 23 ×1 | agy-reader#g145 |
| 14 | `64b8076c-07c0-4788-8ede-5e059a92434c` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 22 steps; attestation rechecks verified in both copies; field 50 ×2; field 23 ×1 | agy-reader#g145 |
| 15 | `757a7119-8df8-4c36-aa7f-d09a32a74156` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 224 steps; attestation rechecks verified in both copies; field 50 ×7 | agy-reader#jgfc |
| 16 | `7d2442ac-71be-4a4e-83fd-6996491eca79` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 108 steps; attestation rechecks verified in both copies; field 50 ×3 | agy-reader#jgfc |
| 17 | `873bc209-a5f3-453f-ad06-f75ac1fd2101` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified in both copies; field 50 ×2 | agy-reader#jgfc |
| 18 | `8b746591-5e9d-4d55-98b0-b2e3183ebc0a` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 1358 steps; attestation rechecks verified in both copies; field 50 ×12; field 23 ×1 | agy-reader#g145 |
| 19 | `90c21ef6-80ad-4cbe-a71f-a6dd6616523d` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 76 steps; attestation rechecks verified in both copies; field 50 ×3; field 23 ×2 | agy-reader#g145 |
| 20 | `b1211347-041b-496c-b7d6-c8c48a059f89` | CLI | Still failing: `unsupported generic arguments` | yes | Verified, valid attestation | 33 steps; attestation rechecks verified in both copies | agy-reader#s42p |
| 21 | `b57f9326-0450-4aba-9b4d-abe4304c6496` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 54 steps; attestation rechecks verified in both copies; field 50 ×2 | agy-reader#jgfc |
| 22 | `b86184c5-71bc-41d9-aa11-bffd04bcfc84` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 88 steps; attestation rechecks verified in both copies; field 50 ×3; field 23 ×4 | agy-reader#g145 |
| 23 | `c0d48394-351d-4fd7-96c0-8dc73ff397e4` | IDE | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 226 steps; attestation rechecks verified in both copies; field 50 ×3 | agy-reader#jgfc |
| 24 | `c2d181b1-515f-44e8-ac07-847ef24024e3` | CLI | Still failing: `unsupported native tool projection` |  | Verified, valid attestation | 38 steps; attestation rechecks verified in both copies | agy-reader#ntzm |
| 25 | `c30dc2ff-1ec1-4cf1-bb91-9cb0342df8d0` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 52 steps; attestation rechecks verified in both copies; field 23 ×3 | agy-reader#g145 |
| 26 | `cbc70748-feb7-4a96-b914-83444a92bf88` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 16 steps; attestation rechecks verified in both copies; field 50 ×4 | agy-reader#jgfc |
| 27 | `de24bfc3-ebae-4c6c-a803-665f84edf23c` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 72 steps; attestation rechecks verified in both copies; field 50 ×3 | agy-reader#jgfc |
| 28 | `de385d5a-e4cc-489b-802f-71aaa50470c4` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 44 steps; attestation rechecks verified in both copies; field 50 ×2; field 23 ×1 | agy-reader#g145, agy-reader#mbpm |
| 29 | `e2b37e93-3ffd-4f05-9447-da747b231675` | CLI | Still failing: `unsupported resultless view_file` |  | Verified, valid attestation | 1114 steps; attestation rechecks verified in both copies | agy-reader#2xqw, agy-reader#axnr |
| 30 | `e2e12f5f-ea5e-4643-9730-01024cb6e8ed` | CLI | Still failing: `unsupported resultless view_file` |  | Verified, valid attestation | 190 steps; attestation rechecks verified in both copies | agy-reader#2xqw |
| 31 | `e31ab9a6-eba7-4f81-a5ca-d45e2f5617ed` | CLI | Still failing: `unknown persisted protobuf fields` (field 23) |  | Verified, declared source-only reason | 175 steps; attestation rechecks verified in both copies; field 50 ×10; field 23 ×3 | agy-reader#g145 |
| 32 | `efc122f2-8247-44a1-9cd1-3a580ec8a6e2` | CLI | Still failing: `unsupported resultless view_file` |  | Verified, valid attestation | 365 steps; attestation rechecks verified in both copies | agy-reader#2xqw |
| 33 | `f3c0899f-0ead-4299-a562-00d9311bbe46` | CLI | Still failing: `unsupported generic result` |  | Verified, declared source-only reason | 138 steps; attestation rechecks verified in both copies; field 50 ×2 | agy-reader#jgfc |
| 34 | `017690ff-19f6-4db8-92a9-48e2afadd729` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 135 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 35 | `04c5dd15-fab6-4f65-9a55-5951f323da42` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 36 | `05b227b2-e660-49f3-8d17-ea1f3059441a` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 13 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 37 | `072c15ad-d596-4bc9-b7c7-ce9a0df5fc70` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 38 | `0806de5a-10f1-411d-9124-6f4c2aac3d96` | IDE | Verified, declared source-only reason |  | Verified, declared source-only reason | 66 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 39 | `0a120ed9-201b-44fa-9c22-d99870a1bce9` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 112 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 40 | `0be2394b-f41d-4008-85c1-411a0676aa4d` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 41 | `0d84d312-b514-4388-b8b7-c82296041294` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 42 | `11d45cca-1c62-479d-a281-fca9df426354` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 18 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 43 | `152700d3-fe94-40d3-b6f2-a3365a9037cc` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 44 | `19026f19-13db-4c28-9008-a842b4dcd5c4` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 56 steps; attestation rechecks verified in both copies; field 50 ×5 |  |
| 45 | `1aab9e49-0e6f-4ebf-b7ee-956094001f60` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 46 | `2266004f-2dbf-4b15-9058-0c7533ce88bc` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 64 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 47 | `3269edd8-3f7e-4a6e-9200-a1aaaa071c53` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 78 steps; attestation rechecks verified in both copies; field 50 ×7 |  |
| 48 | `337a5d4a-cdae-480d-88d9-566a4f4f50c8` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 49 | `3aec5ef4-d5ca-484e-911b-04bb63b1953f` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 50 | `3d094635-6ec4-4770-8b3e-b62de000818c` | IDE | Verified, declared source-only reason |  | Verified, declared source-only reason | 6 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 51 | `453c72cb-2aa5-45a9-a24a-2520e5b4631a` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 52 | `459c4b94-947a-4969-8d69-c133ef188850` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 55 steps; attestation rechecks verified in both copies; field 50 ×9 |  |
| 53 | `4dc0c441-7246-4acc-843a-62a91e8a6ab2` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 54 | `55d64b2d-9748-4917-88f8-fec71d90a2f1` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 55 | `6024206c-a421-4c7b-bd5a-46a3a28e8691` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 56 | `68c3c00f-5870-49eb-9c48-3e2513479ddd` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 21 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 57 | `6c7be1e4-7e1e-4b37-9f16-0a09feade911` | IDE | Verified, declared source-only reason |  | Verified, declared source-only reason | 94 steps; attestation rechecks verified in both copies; field 50 ×6 |  |
| 58 | `6e4be3da-24d5-415c-a87b-8d316816c785` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 59 | `70d6c75e-ece8-476e-a19f-424ba94e1050` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 3 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 60 | `7241b9b2-f6dc-4388-99b0-0878e0df8cef` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 61 | `76ed9323-5309-44a5-9c35-aebdc3683586` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 56 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 62 | `79bdd736-1868-4eef-a56c-785a6970b79b` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 7 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 63 | `8e7f8a2e-3872-4c24-a291-b38162b90ab8` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 64 | `92d82d04-fe14-4130-9e04-b74e036bf035` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 20 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 65 | `974d713a-4372-470c-8c3f-c67a49ff443d` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 90 steps; attestation rechecks verified in both copies; field 50 ×4 |  |
| 66 | `9845be35-4e52-40b1-951e-d0328a451bc2` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 67 | `993ea5ed-a7e6-4709-b94f-74b472202e95` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 68 | `9e58ab9d-28c4-4450-8bc6-74d7575a6811` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 69 | `a18c2e5e-8ce7-468a-b097-3e902fe94dbd` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 70 | `a4c9094e-7912-41a7-bfc5-91d261ca6007` | IDE | Verified, declared source-only reason |  | Verified, declared source-only reason | 12 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 71 | `a59a5104-dc04-40e8-958e-b22b37c7aee4` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 72 | `a6788d9b-5d34-4305-9deb-73c6bfab43f4` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 73 | `a95cf8aa-44cf-44eb-8186-05f69eda6fd6` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 74 | `a96279a8-3592-43c7-b001-7ec483dd99de` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 75 | `b6492909-f8d8-4091-a4d2-e185cfe5d83d` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 76 | `b729c1ec-a905-4b69-84a3-5deacd50a406` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 7 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 77 | `bb05de36-10b5-4442-a931-eaba13b8adca` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 78 | `bd64b535-3ca7-4729-bd53-36b6caa793a2` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 79 | `bfd91871-e244-4d89-be7c-1a505a9deaa0` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 46 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 80 | `cb8cbb8b-72cd-41f3-9c02-b5d0aba035b1` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 8 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 81 | `d0a870aa-d473-4d32-8aef-ebcd21fad824` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 44 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 82 | `daf804ba-f911-46a1-95bd-a6450a7eee80` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 83 | `e6fa5434-3701-4d14-9aae-13e37832a502` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 84 | `e730d5da-5908-45c3-b318-4abea2f0efd9` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 56 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 85 | `e82708c4-01f6-44ef-8dbb-f83a89f9fb52` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified in both copies; field 50 ×3 |  |
| 86 | `eb75850f-20a3-45ce-a221-abb375e38aa8` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 24 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 87 | `edee31a6-06f9-46ca-942c-60c0da915e3c` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 132 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 88 | `fb2d9a19-54c7-401b-a361-2123222731a0` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 6 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 89 | `fcf78168-0559-4217-8f8b-f6712cfa854d` | IDE | Verified, declared source-only reason |  | Verified, declared source-only reason | 68 steps; attestation rechecks verified in both copies; field 50 ×7 |  |
| 90 | `ffa883c6-0747-4686-800a-efb8f1fd5605` | CLI | Verified, declared source-only reason |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified in both copies; field 50 ×2 |  |
| 91 | `0753b4eb-8bfb-44d8-b698-2cacb86a34b4` | CLI | Verified, valid attestation |  | Verified, valid attestation | 22 steps; attestation rechecks verified in both copies |  |
| 92 | `0a4ffcaa-5348-4235-9753-66e9fb8b78d1` | CLI | Verified, valid attestation |  | Verified, valid attestation | 146 steps; attestation rechecks verified in both copies |  |
| 93 | `0b84db4a-0ec3-4409-9fb5-6c1b444fb7b4` | CLI | Verified, valid attestation |  | Verified, valid attestation | 106 steps; attestation rechecks verified in both copies |  |
| 94 | `11e42ea7-27ef-476f-917c-773ccabe6734` | CLI | Verified, valid attestation |  | Verified, valid attestation | 89 steps; attestation rechecks verified in both copies |  |
| 95 | `140748a3-b47a-4229-a6bc-109af71b8ffc` | CLI | Verified, valid attestation |  | Verified, valid attestation | 30 steps; attestation rechecks verified in both copies |  |
| 96 | `16cdf910-8b9a-4c4b-80aa-6cbc7fc10f2f` | CLI | Verified, valid attestation | yes | Verified, valid attestation | 3 steps; attestation rechecks verified in both copies |  |
| 97 | `17569afa-d1b4-40e5-927d-5932f3032df3` | CLI | Verified, valid attestation |  | Verified, valid attestation | 92 steps; attestation rechecks verified in both copies |  |
| 98 | `193843a6-bfb5-4465-b4ac-f772b9da76e5` | CLI | Verified, valid attestation |  | Verified, valid attestation | 741 steps; attestation rechecks verified in both copies |  |
| 99 | `19de72b6-f658-474e-a964-ed1ad63fd760` | CLI | Verified, valid attestation |  | Verified, valid attestation | 61 steps; attestation rechecks verified in both copies |  |
| 100 | `1ca56a1a-507b-45ba-bf70-6f8a0a75e6c5` | CLI | Verified, valid attestation |  | Verified, valid attestation | 5 steps; attestation rechecks verified in both copies |  |
| 101 | `1f20540b-2613-4160-a709-65dfc9f5d3de` | CLI | Verified, valid attestation |  | Verified, valid attestation | 76 steps; attestation rechecks verified in both copies |  |
| 102 | `243aa9f1-690f-4e17-b5bc-9ae6cf37908e` | CLI | Verified, valid attestation |  | Verified, valid attestation | 44 steps; attestation rechecks verified in both copies |  |
| 103 | `2c2a3c3e-a484-4663-992d-1a77b73a4b99` | CLI | Verified, valid attestation |  | Verified, valid attestation | 169 steps; attestation rechecks verified in both copies |  |
| 104 | `2cdb66ff-7112-4274-9902-4bbc9bbfc612` | CLI | Verified, valid attestation |  | Verified, valid attestation | 92 steps; attestation rechecks verified in both copies |  |
| 105 | `2d3470e0-4ad8-4ca1-b60f-c9c258a6ba28` | CLI | Verified, valid attestation |  | Verified, valid attestation | 261 steps; attestation rechecks verified in both copies |  |
| 106 | `2eeda05c-fe5f-402f-88a4-54964d9ba3ba` | CLI | Verified, valid attestation |  | Verified, valid attestation | 44 steps; attestation rechecks verified in both copies |  |
| 107 | `32aee83b-2fe3-400b-9d70-8caa86eb7a98` | CLI | Verified, valid attestation |  | Verified, valid attestation | 475 steps; attestation rechecks verified in both copies |  |
| 108 | `334f9877-6342-40ee-8c30-8a1efedcaf1c` | CLI | Verified, valid attestation |  | Verified, valid attestation | 1877 steps; attestation rechecks verified in both copies |  |
| 109 | `3627c10e-dddc-4567-89a6-81e694d91ba4` | CLI | Verified, valid attestation |  | Verified, valid attestation | 106 steps; attestation rechecks verified in both copies |  |
| 110 | `3a79a49b-acd2-436a-ad0a-6f89b26262a7` | CLI | Verified, valid attestation |  | Verified, valid attestation | 33 steps; attestation rechecks verified in both copies |  |
| 111 | `3ee509bc-71e6-43f6-925e-184cde5a6bef` | CLI | Verified, valid attestation | yes | Verified, valid attestation | 10 steps; attestation rechecks verified in both copies |  |
| 112 | `410395a3-b763-4114-8b12-6a8f63b3a020` | CLI | Verified, valid attestation |  | Verified, valid attestation | 822 steps; attestation rechecks verified in both copies |  |
| 113 | `4731c2d1-06e3-4eb4-a224-61d5509845fc` | CLI | Verified, valid attestation |  | Verified, valid attestation | 84 steps; attestation rechecks verified in both copies |  |
| 114 | `49685b38-5f90-4bb4-8ed1-5feee886617b` | CLI | Verified, valid attestation |  | Verified, valid attestation | 110 steps; attestation rechecks verified in both copies |  |
| 115 | `4d81f901-4165-458a-82d8-8ca11f617e76` | CLI | Verified, valid attestation |  | Verified, valid attestation | 8 steps; attestation rechecks verified in both copies |  |
| 116 | `4ee65cac-5201-417c-82f3-bc0ae66721ec` | CLI | Verified, valid attestation |  | Verified, valid attestation | 540 steps; attestation rechecks verified in both copies |  |
| 117 | `57cf1685-bb30-4538-9c6d-684eba752955` | CLI | Verified, valid attestation |  | Verified, valid attestation | 128 steps; attestation rechecks verified in both copies |  |
| 118 | `62ca97b5-405c-4a97-a9fa-a9aa9f1759d9` | CLI | Verified, valid attestation |  | Verified, valid attestation | 65 steps; attestation rechecks verified in both copies |  |
| 119 | `62ce7231-48cc-4efc-acd1-5c7acb6bed65` | CLI | Verified, valid attestation |  | Verified, valid attestation | 84 steps; attestation rechecks verified in both copies |  |
| 120 | `634209b0-c989-4528-8416-f3af877fda50` | CLI | Verified, valid attestation |  | Verified, valid attestation | 153 steps; attestation rechecks verified in both copies |  |
| 121 | `64f508ef-7a8e-4807-872f-1ad5e9094558` | CLI | Verified, valid attestation |  | Verified, valid attestation | 271 steps; attestation rechecks verified in both copies |  |
| 122 | `66a24054-e957-4494-879b-39d1dffc59fb` | CLI | Verified, valid attestation | yes | Verified, valid attestation | 57 steps; attestation rechecks verified in both copies |  |
| 123 | `67f07231-2e17-454d-99f4-725b3e32380b` | CLI | Verified, valid attestation |  | Verified, valid attestation | 46 steps; attestation rechecks verified in both copies |  |
| 124 | `6c06bc3e-9243-4871-af2c-d105ae82317f` | CLI | Verified, valid attestation |  | Verified, valid attestation | 154 steps; attestation rechecks verified in both copies |  |
| 125 | `6c06f1d1-1716-4a2b-98d7-3b06b29625d7` | CLI | Verified, valid attestation |  | Verified, valid attestation | 6 steps; attestation rechecks verified in both copies |  |
| 126 | `70255f33-8572-4f88-878e-42c3b4a074b6` | CLI | Verified, valid attestation |  | Verified, valid attestation | 42 steps; attestation rechecks verified in both copies |  |
| 127 | `71b930f9-6258-4b55-91e6-185794a38932` | CLI | Verified, valid attestation |  | Verified, valid attestation | 78 steps; attestation rechecks verified in both copies |  |
| 128 | `745a9c7a-ef6b-4bee-ad6c-bb2741ec20c9` | CLI | Verified, valid attestation |  | Verified, valid attestation | 24 steps; attestation rechecks verified in both copies |  |
| 129 | `7520fb2d-6b25-48cd-baf9-82202f8f4dfc` | CLI | Verified, valid attestation |  | Verified, valid attestation | 273 steps; attestation rechecks verified in both copies |  |
| 130 | `7529b27d-27bc-47f3-bb42-7c35d19709a3` | CLI | Verified, valid attestation |  | Verified, valid attestation | 28 steps; attestation rechecks verified in both copies |  |
| 131 | `755f81a3-1632-44ff-9bde-33fa1d5f7970` | CLI | Verified, valid attestation |  | Verified, valid attestation | 152 steps; attestation rechecks verified in both copies |  |
| 132 | `771f3335-f72f-41d6-aa7d-eb238aef309b` | CLI | Verified, valid attestation |  | Verified, valid attestation | 18 steps; attestation rechecks verified in both copies |  |
| 133 | `7e2aa696-5b90-4cad-8411-b3c1b27d41f9` | CLI | Verified, valid attestation |  | Verified, valid attestation | 52 steps; attestation rechecks verified in both copies |  |
| 134 | `8301bfa9-cc8e-4267-89da-c6b1cf481661` | CLI | Verified, valid attestation |  | Verified, valid attestation | 5 steps; attestation rechecks verified in both copies |  |
| 135 | `834ccc26-7a46-4534-9580-bf996f42ea31` | CLI | Verified, valid attestation |  | Verified, valid attestation | 10 steps; attestation rechecks verified in both copies |  |
| 136 | `856dbc95-58d9-4874-a497-b8c0b3374365` | CLI | Verified, valid attestation |  | Verified, valid attestation | 302 steps; attestation rechecks verified in both copies |  |
| 137 | `870ae1f8-f253-49d7-8376-5f6d12096bb0` | CLI | Verified, valid attestation |  | Verified, valid attestation | 276 steps; attestation rechecks verified in both copies |  |
| 138 | `88c70b76-5d47-44b8-a184-d84f21df1f2c` | CLI | Verified, valid attestation |  | Verified, valid attestation | 5 steps; attestation rechecks verified in both copies |  |
| 139 | `8c9e4262-3c17-4594-bf03-aeeccbfca9eb` | CLI | Verified, valid attestation |  | Verified, valid attestation | 253 steps; attestation rechecks verified in both copies |  |
| 140 | `8cb69ec2-a9f5-49f4-b47b-7fd1c4229394` | CLI | Verified, valid attestation |  | Verified, valid attestation | 88 steps; attestation rechecks verified in both copies |  |
| 141 | `8fc67023-092f-4984-83cc-26e88f98eda4` | CLI | Verified, valid attestation |  | Verified, valid attestation | 94 steps; attestation rechecks verified in both copies |  |
| 142 | `92f4948a-942f-40bd-be1e-7f185fa5e57a` | CLI | Verified, valid attestation |  | Verified, valid attestation | 18 steps; attestation rechecks verified in both copies |  |
| 143 | `94793426-865e-40d0-a7d9-d56929d74ab4` | CLI | Verified, valid attestation |  | Verified, valid attestation | 42 steps; attestation rechecks verified in both copies |  |
| 144 | `961cfb93-0c77-45d2-b76a-2efbb7cf844d` | CLI | Verified, valid attestation | yes | Verified, valid attestation | 6 steps; attestation rechecks verified in both copies |  |
| 145 | `981b08af-6edb-4dbb-920a-2d2499a26ccf` | CLI | Verified, valid attestation |  | Verified, valid attestation | 97 steps; attestation rechecks verified in both copies |  |
| 146 | `9af85a5e-4afa-4a96-aecf-f307e6d0c740` | CLI | Verified, valid attestation | yes | Verified, valid attestation | 147 steps; attestation rechecks verified in both copies |  |
| 147 | `9e1c541d-2cb7-478b-8544-707b1cbd0e37` | CLI | Verified, valid attestation |  | Verified, valid attestation | 32 steps; attestation rechecks verified in both copies |  |
| 148 | `9e7db543-69cd-42a0-a1d6-4b5ecd205c3e` | CLI | Verified, valid attestation |  | Verified, valid attestation | 58 steps; attestation rechecks verified in both copies |  |
| 149 | `a071509f-262e-4030-803a-cc64b7ec105e` | CLI | Verified, valid attestation |  | Verified, valid attestation | 369 steps; attestation rechecks verified in both copies |  |
| 150 | `a3ee2b48-31fd-4fba-853d-170c9bb6b66e` | CLI | Verified, valid attestation |  | Verified, valid attestation | 39 steps; attestation rechecks verified in both copies |  |
| 151 | `a536674f-8942-47ea-86f6-ae6e8b9cf391` | CLI | Verified, valid attestation |  | Verified, valid attestation | 27 steps; attestation rechecks verified in both copies |  |
| 152 | `a542d750-1d72-4278-a7ec-690dd2ef1f7a` | CLI | Verified, valid attestation |  | Verified, valid attestation | 127 steps; attestation rechecks verified in both copies |  |
| 153 | `a8c89436-6771-4751-9da4-324796a4998b` | CLI | Verified, valid attestation |  | Verified, valid attestation | 198 steps; attestation rechecks verified in both copies |  |
| 154 | `a8d2a036-1740-4376-8ae5-9a12883b8268` | CLI | Verified, valid attestation |  | Verified, valid attestation | 22 steps; attestation rechecks verified in both copies |  |
| 155 | `aa77bc8b-3928-4a2f-acfd-83dc9c5c794b` | CLI | Verified, valid attestation |  | Verified, valid attestation | 14 steps; attestation rechecks verified in both copies |  |
| 156 | `ad78a1a1-54ae-4954-bb30-621adfbe8c7e` | CLI | Verified, valid attestation |  | Verified, valid attestation | 201 steps; attestation rechecks verified in both copies |  |
| 157 | `afcdb6da-6264-4671-9f89-082dd6271d08` | CLI | Verified, valid attestation |  | Verified, valid attestation | 35 steps; attestation rechecks verified in both copies |  |
| 158 | `afef2def-56bd-467a-a9eb-33e1696704fb` | CLI | Verified, valid attestation |  | Verified, valid attestation | 9 steps; attestation rechecks verified in both copies |  |
| 159 | `b765c9ad-f852-42d7-b8a6-b0005eecc50b` | CLI | Verified, valid attestation |  | Verified, valid attestation | 44 steps; attestation rechecks verified in both copies |  |
| 160 | `b7ae1a0d-f841-4aa4-bbb3-3e92438cb383` | CLI | Verified, valid attestation |  | Verified, valid attestation | 59 steps; attestation rechecks verified in both copies |  |
| 161 | `bd21a161-c93d-4475-94bc-7efae7c6acca` | CLI | Verified, valid attestation |  | Verified, valid attestation | 44 steps; attestation rechecks verified in both copies |  |
| 162 | `c305573a-afb1-4fde-94fc-59fdb310a082` | CLI | Verified, valid attestation |  | Verified, valid attestation | 42 steps; attestation rechecks verified in both copies |  |
| 163 | `c886758f-edfd-4a1d-9bea-3f10e559217b` | CLI | Verified, valid attestation |  | Verified, valid attestation | 70 steps; attestation rechecks verified in both copies |  |
| 164 | `cfc9ca4b-0ab0-41a2-9cf5-6bc631637d74` | CLI | Verified, valid attestation |  | Verified, valid attestation | 19 steps; attestation rechecks verified in both copies |  |
| 165 | `cfcb3d84-51dc-44f6-8adb-743057283922` | CLI | Verified, valid attestation |  | Verified, valid attestation | 114 steps; attestation rechecks verified in both copies |  |
| 166 | `dc193c58-0123-4828-90b3-4f63ee11f10b` | CLI | Verified, valid attestation |  | Verified, valid attestation | 154 steps; attestation rechecks verified in both copies |  |
| 167 | `e18a9fe7-c591-4b14-a617-2e86604ddee6` | CLI | Verified, valid attestation |  | Verified, valid attestation | 73 steps; attestation rechecks verified in both copies |  |
| 168 | `e1a27237-8e78-4698-acef-004c847c4dec` | CLI | Verified, valid attestation | yes | Verified, valid attestation | 76 steps; attestation rechecks verified in both copies |  |
| 169 | `e37018a5-751f-4684-b243-7c7e0e6aaf28` | CLI | Verified, valid attestation |  | Verified, valid attestation | 5 steps; attestation rechecks verified in both copies |  |
| 170 | `e406fbb6-5830-4661-94d3-7c04f00ec49c` | CLI | Verified, valid attestation |  | Verified, valid attestation | 26 steps; attestation rechecks verified in both copies |  |
| 171 | `eb695ac1-86bf-41b2-81ba-bd398201e611` | CLI | Verified, valid attestation |  | Verified, valid attestation | 38 steps; attestation rechecks verified in both copies |  |
| 172 | `ed3e3fbe-5cb2-4032-bc9e-fe28fbfaa80e` | CLI | Verified, valid attestation |  | Verified, valid attestation | 321 steps; attestation rechecks verified in both copies |  |
| 173 | `ef47b8d7-8a14-42ca-89b2-b302146a3c9d` | CLI | Verified, valid attestation |  | Verified, valid attestation | 304 steps; attestation rechecks verified in both copies |  |
| 174 | `ef8876f5-22de-4518-80d9-c5e0a133c7eb` | CLI | Verified, valid attestation |  | Verified, valid attestation | 132 steps; attestation rechecks verified in both copies |  |
| 175 | `f228c323-f857-4fea-b39e-60aa0555bebc` | CLI | Verified, valid attestation |  | Verified, valid attestation | 74 steps; attestation rechecks verified in both copies |  |
| 176 | `f229226f-b178-46fa-a37a-39244aa6d0e1` | CLI | Verified, valid attestation |  | Verified, valid attestation | 186 steps; attestation rechecks verified in both copies |  |
| 177 | `f373ab08-b0c5-460d-94c0-d1cb3d6b834c` | CLI | Verified, valid attestation |  | Verified, valid attestation | 309 steps; attestation rechecks verified in both copies |  |
| 178 | `f5c5abe6-50ea-4785-b7be-9ab0db4070d4` | CLI | Verified, valid attestation |  | Verified, valid attestation | 70 steps; attestation rechecks verified in both copies |  |
| 179 | `f839db4d-258d-4ca1-a06a-2ec2bd40bfe6` | CLI | Verified, valid attestation |  | Verified, valid attestation | 66 steps; attestation rechecks verified in both copies |  |
| 180 | `f86ea576-596c-4fd9-9d86-971b7c4fce49` | CLI | Verified, valid attestation |  | Verified, valid attestation | 84 steps; attestation rechecks verified in both copies |  |
| 181 | `f94771fb-0308-4c6f-b9b4-e2b6294aa655` | CLI | Verified, valid attestation |  | Verified, valid attestation | 381 steps; attestation rechecks verified in both copies |  |
| 182 | `ff7b965e-835c-4905-af72-f3d9d1fb0baa` | CLI | Verified, valid attestation | yes | Verified, valid attestation | 170 steps; attestation rechecks verified in both copies |  |
