# Recovery cohort rerun through normal sync and watch, 2026-10-06

Conversation labels such as `conversation-001` are consistent aliases across
these reports, not session IDs. Use actual IDs from your own store when
rerunning commands.

The 2026-10-03 session recovery left 182 retained conversations that the reader
could not attest: 101 refused at the display projection and 81 refused for
unknown historical fields. After the projection fixes and the field-50
source-only profile landed, **149 of the 182 now verify through ordinary sync
and a fresh watcher, and 33 still fail.** Of the 149, 92 have a plain valid
attestation and 57 have a valid attestation that carries the declared
source-only reason for CascadePlannerConfig field 50. Seven of the nine short
exports recovered by hand on 2026-10-03 now verify normally; two still fail.

The 33 failures fall into five step shapes plus the expected field-23 group.
Each has a tracker issue; this rerun changes no reader code.

| Result | Count | Prior projection failures (101) | Prior unknown-field failures (81) |
| --- | ---: | ---: | ---: |
| Verified, valid attestation | 92 | 92 | 0 |
| Verified, declared source-only reason (field 50) | 57 | 0 | 57 |
| Still failing | 33 | 9 | 24 |
| **Total** | **182** | **101** | **81** |

The six IDE conversations are among the 81: five verify with the source-only
reason and one still fails (`list_permissions`, below).

## The nine short exports

| Conversation | Result now | Fresh sidecar against the 2026-10-03 audited export |
| --- | --- | --- |
| `conversation-022` | Verified, valid attestation | daemon payload identical, 3 steps |
| `conversation-051` | Verified, valid attestation | daemon payload identical, 10 steps |
| `conversation-077` | Verified, valid attestation | daemon payload identical, 57 steps |
| `conversation-119` | Verified, valid attestation | daemon payload identical, 6 steps |
| `conversation-125` | Verified, valid attestation | daemon payload identical, 147 steps |
| `conversation-177` | Verified, valid attestation | daemon payload identical, 76 steps |
| `conversation-211` | Verified, valid attestation | daemon payload identical, 170 steps |
| `conversation-098` | Still failing: `unsupported resultless view_file` at step 62 | no sidecar written |
| `conversation-148` | Still failing: `unsupported generic arguments` at step 31 | no sidecar written |

"Identical" means the synced sidecar, with the reader's own `agyReader` block
removed, equals the audited export as parsed JSON. The two failing exports keep
only their one-off audited export and still have no standard attestation.

## What still fails

| Shape | Reader reason | Conversations | Steps | Tracked in |
| --- | --- | ---: | ---: | --- |
| ChatMessagePrompt field 23 in generator metadata | `unknown persisted protobuf fields` | 15 | whole source | agy-reader#g145 (spike, open) |
| Payload-free `list_permissions` call | `unsupported generic result` | 9 | 10 | agy-reader#jgfc (new) |
| Failed `view_file` with no result | `unsupported resultless view_file` | 7 | 12 | agy-reader#2xqw (existing) |
| Successful `generate_image` stored as a native image step | `unsupported native tool projection` | 1 | 3 | agy-reader#ntzm (new) |
| `write_to_file` with the `Append` argument | `unsupported generic arguments` | 1 | 1 | agy-reader#s42p (new) |
| `view_file` carrying a recorded permission request | `unsupported generic wrapper` | 1 (not its first failure) | 1 | agy-reader#axnr (new) |

The 15 field-23 conversations are exactly the expected group: each still holds
ChatMessagePrompt field 23 (wire type 2) in `gen_metadata`, and no other
unknown field besides the accepted field 50. They fail before any step is
read, so they have no per-step count.

The nine `list_permissions` conversations used to fail on field 50. They now
pass source decoding and full raw RPC equality, then stop at the projection.
This is the case the task anticipated: a source that clears its first gate and
fails a later one. The `view_file` permission-request step sits behind five
`view_file` failures in the same conversation, so it becomes the blocker only
after agy-reader#2xqw. agy-reader#2xqw describes eight sessions with 12 steps;
this cohort has seven conversations holding those 12 steps.

For every still-failing conversation except the field-23 group, the full raw
RPC equals the stored source exactly. Every step the projection accepts on its
own also equals the daemon's ordinary presentation of that step. That is
per-step evidence only: a conversation becomes verified when the reader's
whole-conversation check passes, not when these counts reach zero.

## How it ran

- **Reader:** built with `make build` from `agent/xwtr` at `b186735`, the
  current local `main`. Profile `sqlite-v1-format0-agy-a759ce7c7a23-planner50`.
- **Daemon:** agy 1.2.16, the retained binary whose SHA-256
  (`a759ce7c7a235d9b6c281a25ead97cbbf2e92314a3ffd224e2f9144f3fae7a86`) is the
  one the reader's schema was extracted from. The installed agy is now 1.3.0,
  and the reader does not yet accept conversations written by 1.2.17 or later
  (agy-reader#8mnx). These are historical conversations, so the rerun pairs
  the reader with the daemon its profile describes. It was not repeated with
  agy 1.3.0.
- **Isolation:** two fresh copies of the 182 SQLite sources, each in its own
  scratch store. agy ran under `bwrap` with the host read-only and
  `--gemini_dir` pointing at the copy, so the daemon loaded only the copies.
  `scripts/agy-pair.py` started each agy/reader pair with a fresh token.
  Every copy matched the recovery manifest's SHA-256 before use; the daemon
  left no WAL content in either store.
- **Sync:** against the first copy, `agy-reader --root <store> --sync <id>` for
  each of the 182 conversations, one at a time. 149 exited 0 and wrote a
  sidecar; 33 exited 1 with the reasons above.
- **Watch:** after the sync pair stopped, a new agy daemon and a fresh watcher
  (`--watch --watch-interval=10s`) on the second copy. The first tick reported
  149 synced and 33 skipped; later ticks reported 149 up-to-date, 33 skipped,
  0 failed. The watcher logged the same reason as sync for each refusal.
- **Checks on the result:** for every sidecar from both phases, an offline
  diagnostic re-read the copy and rechecked the attestation with the reader's
  own `CheckAttestation`. All 149 recheck as verified in both stores, with
  identical source and payload digests between sync and watch, and the
  source-only reason present on exactly the 57 field-50 conversations. For the
  33 refusals, the same diagnostic ran the projection on each step alone and
  compared accepted steps with freshly captured daemon responses. The
  diagnostic was compiled with `go test -overlay` and is not part of the
  repository.
- **Originals:** the private sources under the recovery directory
  `2026-10-03-85biqu17` were only read. After the run all 182 still match the
  manifest's SHA-256, and no file there is newer than the run. The scratch
  copies, sidecars and RPC captures are not committed.

## Per-conversation results

The step number is the zero-based index of the first refused step. "Steps
refused" counts steps the projection refuses when each is checked alone. For
verified conversations, "field-50 records" is the number of declared
source-only occurrences the attestation records. "Result now" is the same for
sync and for the fresh watcher in every row.

| # | Conversation | Surface | First failure on 2026-10-03 | Short export | Result now (sync and watch) | Detail |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `conversation-004` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×8; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 2 | `conversation-005` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 16; 1 of 172 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 3 | `conversation-021` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×6; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 4 | `conversation-024` | CLI | unsupported generic wrapper |  | **Still failing** | `unsupported resultless view_file` at step 2; 1 of 401 steps refused; 0 other steps differ from presentation; tracked in agy-reader#2xqw |
| 5 | `conversation-035` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×3; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 6 | `conversation-049` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×1; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 7 | `conversation-052` | CLI | unsupported native tool projection |  | **Still failing** | `unsupported resultless view_file` at step 48; 1 of 66 steps refused; 0 other steps differ from presentation; tracked in agy-reader#2xqw |
| 8 | `conversation-053` | CLI | unsupported wrapper/native status difference |  | **Still failing** | `unsupported resultless view_file` at step 20; 2 of 51 steps refused; 0 other steps differ from presentation; tracked in agy-reader#2xqw |
| 9 | `conversation-055` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×1; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 10 | `conversation-060` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×2; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 11 | `conversation-063` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×10; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 12 | `conversation-070` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×1; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 13 | `conversation-075` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×1; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 14 | `conversation-092` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 3; 2 of 224 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 15 | `conversation-098` | CLI | unsupported native tool projection | yes | **Still failing** | `unsupported resultless view_file` at step 62; 1 of 355 steps refused; 0 other steps differ from presentation; tracked in agy-reader#2xqw |
| 16 | `conversation-099` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 18; 1 of 108 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 17 | `conversation-105` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 3; 1 of 14 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 18 | `conversation-107` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×1; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 19 | `conversation-113` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×2; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 20 | `conversation-148` | CLI | unsupported generic arguments | yes | **Still failing** | `unsupported generic arguments` at step 31; 1 of 33 steps refused; 0 other steps differ from presentation; tracked in agy-reader#s42p |
| 21 | `conversation-149` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 3; 1 of 54 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 22 | `conversation-155` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×4; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 23 | `conversation-161` | IDE | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 14; 1 of 226 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 24 | `conversation-162` | CLI | unsupported native tool projection |  | **Still failing** | `unsupported native tool projection` at step 26; 3 of 38 steps refused; 0 other steps differ from presentation; tracked in agy-reader#ntzm |
| 25 | `conversation-164` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×3; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 26 | `conversation-167` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 3; 1 of 16 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 27 | `conversation-174` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 8; 1 of 72 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 28 | `conversation-175` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×1; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 29 | `conversation-178` | CLI | unsupported native tool projection |  | **Still failing** | `unsupported resultless view_file` at step 309; 6 of 1114 steps refused (5 unsupported resultless view_file, 1 unsupported generic wrapper); 0 other steps differ from presentation; tracked in agy-reader#2xqw, agy-reader#axnr |
| 30 | `conversation-179` | CLI | unsupported generic wrapper |  | **Still failing** | `unsupported resultless view_file` at step 84; 1 of 190 steps refused; 0 other steps differ from presentation; tracked in agy-reader#2xqw |
| 31 | `conversation-180` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×3; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 32 | `conversation-195` | CLI | unsupported generic arguments |  | **Still failing** | `unsupported resultless view_file` at step 347; 1 of 365 steps refused; 0 other steps differ from presentation; tracked in agy-reader#2xqw |
| 33 | `conversation-201` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 6; 1 of 138 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 34 | `conversation-001` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 135 steps; attestation rechecks verified; 3 field-50 records |
| 35 | `conversation-002` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 36 | `conversation-003` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 13 steps; attestation rechecks verified; 3 field-50 records |
| 37 | `conversation-006` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 38 | `conversation-008` | IDE | unknown persisted protobuf fields |  | Verified, declared source-only reason | 66 steps; attestation rechecks verified; 3 field-50 records |
| 39 | `conversation-009` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 112 steps; attestation rechecks verified; 3 field-50 records |
| 40 | `conversation-013` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 41 | `conversation-014` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 42 | `conversation-015` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 18 steps; attestation rechecks verified; 2 field-50 records |
| 43 | `conversation-020` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified; 3 field-50 records |
| 44 | `conversation-025` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 56 steps; attestation rechecks verified; 5 field-50 records |
| 45 | `conversation-029` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 46 | `conversation-033` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 64 steps; attestation rechecks verified; 2 field-50 records |
| 47 | `conversation-042` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 78 steps; attestation rechecks verified; 7 field-50 records |
| 48 | `conversation-045` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 49 | `conversation-048` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 50 | `conversation-050` | IDE | unknown persisted protobuf fields |  | Verified, declared source-only reason | 6 steps; attestation rechecks verified; 2 field-50 records |
| 51 | `conversation-056` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 52 | `conversation-057` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 55 steps; attestation rechecks verified; 9 field-50 records |
| 53 | `conversation-062` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 54 | `conversation-065` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified; 2 field-50 records |
| 55 | `conversation-069` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 56 | `conversation-079` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 21 steps; attestation rechecks verified; 2 field-50 records |
| 57 | `conversation-082` | IDE | unknown persisted protobuf fields |  | Verified, declared source-only reason | 94 steps; attestation rechecks verified; 6 field-50 records |
| 58 | `conversation-083` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 59 | `conversation-085` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 3 steps; attestation rechecks verified; 2 field-50 records |
| 60 | `conversation-087` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 61 | `conversation-093` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 56 steps; attestation rechecks verified; 3 field-50 records |
| 62 | `conversation-097` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 7 steps; attestation rechecks verified; 3 field-50 records |
| 63 | `conversation-111` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 64 | `conversation-115` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 20 steps; attestation rechecks verified; 2 field-50 records |
| 65 | `conversation-120` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 90 steps; attestation rechecks verified; 4 field-50 records |
| 66 | `conversation-123` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 67 | `conversation-124` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 68 | `conversation-127` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 69 | `conversation-130` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 70 | `conversation-133` | IDE | unknown persisted protobuf fields |  | Verified, declared source-only reason | 12 steps; attestation rechecks verified; 2 field-50 records |
| 71 | `conversation-137` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 72 | `conversation-138` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 73 | `conversation-141` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified; 2 field-50 records |
| 74 | `conversation-142` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 75 | `conversation-150` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 76 | `conversation-152` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 7 steps; attestation rechecks verified; 2 field-50 records |
| 77 | `conversation-157` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 78 | `conversation-159` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 79 | `conversation-160` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 46 steps; attestation rechecks verified; 2 field-50 records |
| 80 | `conversation-166` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 8 steps; attestation rechecks verified; 2 field-50 records |
| 81 | `conversation-171` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 44 steps; attestation rechecks verified; 3 field-50 records |
| 82 | `conversation-172` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 83 | `conversation-184` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 84 | `conversation-185` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 56 steps; attestation rechecks verified; 2 field-50 records |
| 85 | `conversation-187` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified; 3 field-50 records |
| 86 | `conversation-190` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 24 steps; attestation rechecks verified; 2 field-50 records |
| 87 | `conversation-192` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 132 steps; attestation rechecks verified; 2 field-50 records |
| 88 | `conversation-208` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 6 steps; attestation rechecks verified; 2 field-50 records |
| 89 | `conversation-209` | IDE | unknown persisted protobuf fields |  | Verified, declared source-only reason | 68 steps; attestation rechecks verified; 7 field-50 records |
| 90 | `conversation-212` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 91 | `conversation-007` | CLI | unsupported generic arguments |  | Verified, valid attestation | 22 steps; attestation rechecks verified |
| 92 | `conversation-010` | CLI | unsupported generic arguments |  | Verified, valid attestation | 146 steps; attestation rechecks verified |
| 93 | `conversation-012` | CLI | unsupported native tool projection |  | Verified, valid attestation | 106 steps; attestation rechecks verified |
| 94 | `conversation-016` | CLI | unsupported generic arguments |  | Verified, valid attestation | 89 steps; attestation rechecks verified |
| 95 | `conversation-018` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 30 steps; attestation rechecks verified |
| 96 | `conversation-022` | CLI | unsupported wrapper/native status difference | yes | Verified, valid attestation | 3 steps; attestation rechecks verified |
| 97 | `conversation-023` | CLI | unsupported native tool projection |  | Verified, valid attestation | 92 steps; attestation rechecks verified |
| 98 | `conversation-026` | CLI | unsupported native tool projection |  | Verified, valid attestation | 741 steps; attestation rechecks verified |
| 99 | `conversation-027` | CLI | unsupported wrapper/native status difference |  | Verified, valid attestation | 61 steps; attestation rechecks verified |
| 100 | `conversation-030` | CLI | unsupported generic arguments |  | Verified, valid attestation | 5 steps; attestation rechecks verified |
| 101 | `conversation-031` | CLI | unsupported native tool projection |  | Verified, valid attestation | 76 steps; attestation rechecks verified |
| 102 | `conversation-034` | CLI | unsupported generic arguments |  | Verified, valid attestation | 44 steps; attestation rechecks verified |
| 103 | `conversation-036` | CLI | unsupported native tool projection |  | Verified, valid attestation | 169 steps; attestation rechecks verified |
| 104 | `conversation-037` | CLI | unsupported generic arguments |  | Verified, valid attestation | 92 steps; attestation rechecks verified |
| 105 | `conversation-038` | CLI | unsupported native tool projection |  | Verified, valid attestation | 261 steps; attestation rechecks verified |
| 106 | `conversation-040` | CLI | unsupported native tool projection |  | Verified, valid attestation | 44 steps; attestation rechecks verified |
| 107 | `conversation-043` | CLI | unsupported wrapper/native status difference |  | Verified, valid attestation | 475 steps; attestation rechecks verified |
| 108 | `conversation-044` | CLI | unsupported native tool projection |  | Verified, valid attestation | 1877 steps; attestation rechecks verified |
| 109 | `conversation-046` | CLI | unsupported generic arguments |  | Verified, valid attestation | 106 steps; attestation rechecks verified |
| 110 | `conversation-047` | CLI | unsupported wrapper/native status difference |  | Verified, valid attestation | 33 steps; attestation rechecks verified |
| 111 | `conversation-051` | CLI | unsupported generic arguments | yes | Verified, valid attestation | 10 steps; attestation rechecks verified |
| 112 | `conversation-054` | CLI | unsupported generic arguments |  | Verified, valid attestation | 822 steps; attestation rechecks verified |
| 113 | `conversation-058` | CLI | unsupported generic arguments |  | Verified, valid attestation | 84 steps; attestation rechecks verified |
| 114 | `conversation-059` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 110 steps; attestation rechecks verified |
| 115 | `conversation-061` | CLI | unsupported generic arguments |  | Verified, valid attestation | 8 steps; attestation rechecks verified |
| 116 | `conversation-064` | CLI | unsupported generic result |  | Verified, valid attestation | 540 steps; attestation rechecks verified |
| 117 | `conversation-066` | CLI | unsupported generic result |  | Verified, valid attestation | 128 steps; attestation rechecks verified |
| 118 | `conversation-071` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 65 steps; attestation rechecks verified |
| 119 | `conversation-072` | CLI | unsupported generic result |  | Verified, valid attestation | 84 steps; attestation rechecks verified |
| 120 | `conversation-073` | CLI | unsupported native tool projection |  | Verified, valid attestation | 153 steps; attestation rechecks verified |
| 121 | `conversation-076` | CLI | unsupported native tool projection |  | Verified, valid attestation | 271 steps; attestation rechecks verified |
| 122 | `conversation-077` | CLI | unsupported wrapper/native status difference | yes | Verified, valid attestation | 57 steps; attestation rechecks verified |
| 123 | `conversation-078` | CLI | unsupported generic arguments |  | Verified, valid attestation | 46 steps; attestation rechecks verified |
| 124 | `conversation-080` | CLI | unsupported native tool projection |  | Verified, valid attestation | 154 steps; attestation rechecks verified |
| 125 | `conversation-081` | CLI | unsupported generic arguments |  | Verified, valid attestation | 6 steps; attestation rechecks verified |
| 126 | `conversation-084` | CLI | unsupported native tool projection |  | Verified, valid attestation | 42 steps; attestation rechecks verified |
| 127 | `conversation-086` | CLI | unsupported generic arguments |  | Verified, valid attestation | 78 steps; attestation rechecks verified |
| 128 | `conversation-088` | CLI | unsupported generic arguments |  | Verified, valid attestation | 24 steps; attestation rechecks verified |
| 129 | `conversation-089` | CLI | unsupported native tool projection |  | Verified, valid attestation | 273 steps; attestation rechecks verified |
| 130 | `conversation-090` | CLI | unsupported generic arguments |  | Verified, valid attestation | 28 steps; attestation rechecks verified |
| 131 | `conversation-091` | CLI | unsupported native tool projection |  | Verified, valid attestation | 152 steps; attestation rechecks verified |
| 132 | `conversation-095` | CLI | unsupported generic arguments |  | Verified, valid attestation | 18 steps; attestation rechecks verified |
| 133 | `conversation-100` | CLI | unsupported generic arguments |  | Verified, valid attestation | 52 steps; attestation rechecks verified |
| 134 | `conversation-101` | CLI | unsupported generic arguments |  | Verified, valid attestation | 5 steps; attestation rechecks verified |
| 135 | `conversation-102` | CLI | unsupported native tool projection |  | Verified, valid attestation | 10 steps; attestation rechecks verified |
| 136 | `conversation-103` | CLI | unsupported generic arguments |  | Verified, valid attestation | 302 steps; attestation rechecks verified |
| 137 | `conversation-104` | CLI | unsupported native tool projection |  | Verified, valid attestation | 276 steps; attestation rechecks verified |
| 138 | `conversation-106` | CLI | unsupported native tool projection |  | Verified, valid attestation | 5 steps; attestation rechecks verified |
| 139 | `conversation-108` | CLI | unsupported generic arguments |  | Verified, valid attestation | 253 steps; attestation rechecks verified |
| 140 | `conversation-109` | CLI | unsupported generic arguments |  | Verified, valid attestation | 88 steps; attestation rechecks verified |
| 141 | `conversation-112` | CLI | unsupported generic arguments |  | Verified, valid attestation | 94 steps; attestation rechecks verified |
| 142 | `conversation-116` | CLI | unsupported native tool projection |  | Verified, valid attestation | 18 steps; attestation rechecks verified |
| 143 | `conversation-118` | CLI | unsupported native tool projection |  | Verified, valid attestation | 42 steps; attestation rechecks verified |
| 144 | `conversation-119` | CLI | unsupported generic wrapper | yes | Verified, valid attestation | 6 steps; attestation rechecks verified |
| 145 | `conversation-122` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 97 steps; attestation rechecks verified |
| 146 | `conversation-125` | CLI | unsupported wrapper/native status difference | yes | Verified, valid attestation | 147 steps; attestation rechecks verified |
| 147 | `conversation-126` | CLI | unsupported native tool projection |  | Verified, valid attestation | 32 steps; attestation rechecks verified |
| 148 | `conversation-128` | CLI | unsupported generic arguments |  | Verified, valid attestation | 58 steps; attestation rechecks verified |
| 149 | `conversation-129` | CLI | unsupported native tool projection |  | Verified, valid attestation | 369 steps; attestation rechecks verified |
| 150 | `conversation-131` | CLI | unsupported native tool projection |  | Verified, valid attestation | 39 steps; attestation rechecks verified |
| 151 | `conversation-135` | CLI | unsupported generic arguments |  | Verified, valid attestation | 27 steps; attestation rechecks verified |
| 152 | `conversation-136` | CLI | unsupported native tool projection |  | Verified, valid attestation | 127 steps; attestation rechecks verified |
| 153 | `conversation-139` | CLI | unsupported generic arguments |  | Verified, valid attestation | 198 steps; attestation rechecks verified |
| 154 | `conversation-140` | CLI | unsupported native tool projection |  | Verified, valid attestation | 22 steps; attestation rechecks verified |
| 155 | `conversation-143` | CLI | unsupported native tool projection |  | Verified, valid attestation | 14 steps; attestation rechecks verified |
| 156 | `conversation-145` | CLI | unsupported generic arguments |  | Verified, valid attestation | 201 steps; attestation rechecks verified |
| 157 | `conversation-146` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 35 steps; attestation rechecks verified |
| 158 | `conversation-147` | CLI | unsupported native tool projection |  | Verified, valid attestation | 9 steps; attestation rechecks verified |
| 159 | `conversation-153` | CLI | unsupported wrapper/native status difference |  | Verified, valid attestation | 44 steps; attestation rechecks verified |
| 160 | `conversation-154` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 59 steps; attestation rechecks verified |
| 161 | `conversation-158` | CLI | unsupported native tool projection |  | Verified, valid attestation | 44 steps; attestation rechecks verified |
| 162 | `conversation-163` | CLI | unsupported generic arguments |  | Verified, valid attestation | 42 steps; attestation rechecks verified |
| 163 | `conversation-165` | CLI | unsupported generic arguments |  | Verified, valid attestation | 70 steps; attestation rechecks verified |
| 164 | `conversation-169` | CLI | unsupported native tool projection |  | Verified, valid attestation | 19 steps; attestation rechecks verified |
| 165 | `conversation-170` | CLI | unsupported generic arguments |  | Verified, valid attestation | 114 steps; attestation rechecks verified |
| 166 | `conversation-173` | CLI | unsupported native tool projection |  | Verified, valid attestation | 154 steps; attestation rechecks verified |
| 167 | `conversation-176` | CLI | unsupported generic arguments |  | Verified, valid attestation | 73 steps; attestation rechecks verified |
| 168 | `conversation-177` | CLI | unsupported generic arguments | yes | Verified, valid attestation | 76 steps; attestation rechecks verified |
| 169 | `conversation-181` | CLI | unsupported native tool projection |  | Verified, valid attestation | 5 steps; attestation rechecks verified |
| 170 | `conversation-182` | CLI | unsupported native tool projection |  | Verified, valid attestation | 26 steps; attestation rechecks verified |
| 171 | `conversation-189` | CLI | unsupported native tool projection |  | Verified, valid attestation | 38 steps; attestation rechecks verified |
| 172 | `conversation-191` | CLI | unsupported native tool projection |  | Verified, valid attestation | 321 steps; attestation rechecks verified |
| 173 | `conversation-193` | CLI | unsupported native tool projection |  | Verified, valid attestation | 304 steps; attestation rechecks verified |
| 174 | `conversation-194` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 132 steps; attestation rechecks verified |
| 175 | `conversation-198` | CLI | unsupported generic arguments |  | Verified, valid attestation | 74 steps; attestation rechecks verified |
| 176 | `conversation-199` | CLI | unsupported generic result |  | Verified, valid attestation | 186 steps; attestation rechecks verified |
| 177 | `conversation-200` | CLI | unsupported wrapper/native status difference |  | Verified, valid attestation | 309 steps; attestation rechecks verified |
| 178 | `conversation-202` | CLI | unsupported generic arguments |  | Verified, valid attestation | 70 steps; attestation rechecks verified |
| 179 | `conversation-205` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 66 steps; attestation rechecks verified |
| 180 | `conversation-206` | CLI | unsupported generic arguments |  | Verified, valid attestation | 84 steps; attestation rechecks verified |
| 181 | `conversation-207` | CLI | unsupported wrapper/native status difference |  | Verified, valid attestation | 381 steps; attestation rechecks verified |
| 182 | `conversation-211` | CLI | unsupported native tool projection | yes | Verified, valid attestation | 170 steps; attestation rechecks verified |
