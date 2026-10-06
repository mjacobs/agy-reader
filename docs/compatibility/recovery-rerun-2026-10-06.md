# Recovery cohort rerun through normal sync and watch, 2026-10-06

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
| `16cdf910-8b9a-4c4b-80aa-6cbc7fc10f2f` | Verified, valid attestation | daemon payload identical, 3 steps |
| `3ee509bc-71e6-43f6-925e-184cde5a6bef` | Verified, valid attestation | daemon payload identical, 10 steps |
| `66a24054-e957-4494-879b-39d1dffc59fb` | Verified, valid attestation | daemon payload identical, 57 steps |
| `961cfb93-0c77-45d2-b76a-2efbb7cf844d` | Verified, valid attestation | daemon payload identical, 6 steps |
| `9af85a5e-4afa-4a96-aecf-f307e6d0c740` | Verified, valid attestation | daemon payload identical, 147 steps |
| `e1a27237-8e78-4698-acef-004c847c4dec` | Verified, valid attestation | daemon payload identical, 76 steps |
| `ff7b965e-835c-4905-af72-f3d9d1fb0baa` | Verified, valid attestation | daemon payload identical, 170 steps |
| `7b3888f6-9f76-4504-969a-187c95705773` | Still failing: `unsupported resultless view_file` at step 62 | no sidecar written |
| `b1211347-041b-496c-b7d6-c8c48a059f89` | Still failing: `unsupported generic arguments` at step 31 | no sidecar written |

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
| 1 | `06a18622-4a71-4ac5-b7c7-c9570319d1df` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×8; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 2 | `06b66779-eebe-4869-ba1b-ccf3d42be70b` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 16; 1 of 172 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 3 | `169edd42-df38-4bf3-a682-b4394ff877f4` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×6; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 4 | `19017719-a37d-43ef-b7ea-6e442107b96e` | CLI | unsupported generic wrapper |  | **Still failing** | `unsupported resultless view_file` at step 2; 1 of 401 steps refused; 0 other steps differ from presentation; tracked in agy-reader#2xqw |
| 5 | `24eb7de5-2fc4-4595-9f3f-5d2223d0f2c8` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×3; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 6 | `3cae60ae-0915-4a4e-b2cf-d04ecc04b0b9` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×1; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 7 | `4085478b-647c-4e3e-9d36-86ae979a51c4` | CLI | unsupported native tool projection |  | **Still failing** | `unsupported resultless view_file` at step 48; 1 of 66 steps refused; 0 other steps differ from presentation; tracked in agy-reader#2xqw |
| 8 | `40b43605-9cdc-4d5a-8179-1eb671ed3435` | CLI | unsupported wrapper/native status difference |  | **Still failing** | `unsupported resultless view_file` at step 20; 2 of 51 steps refused; 0 other steps differ from presentation; tracked in agy-reader#2xqw |
| 9 | `4304cac0-6cf7-4b4d-bcb0-3f7ccd902b66` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×1; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 10 | `4c47d422-0502-40a9-a676-7df0d424cb34` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×2; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 11 | `4eb0934c-89a7-454f-8bbc-99ea73c88717` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×10; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 12 | `61f98d57-fbc1-4cd9-bf41-ee14a3ba1c79` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×1; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 13 | `64b8076c-07c0-4788-8ede-5e059a92434c` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×1; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 14 | `757a7119-8df8-4c36-aa7f-d09a32a74156` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 3; 2 of 224 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 15 | `7b3888f6-9f76-4504-969a-187c95705773` | CLI | unsupported native tool projection | yes | **Still failing** | `unsupported resultless view_file` at step 62; 1 of 355 steps refused; 0 other steps differ from presentation; tracked in agy-reader#2xqw |
| 16 | `7d2442ac-71be-4a4e-83fd-6996491eca79` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 18; 1 of 108 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 17 | `873bc209-a5f3-453f-ad06-f75ac1fd2101` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 3; 1 of 14 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 18 | `8b746591-5e9d-4d55-98b0-b2e3183ebc0a` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×1; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 19 | `90c21ef6-80ad-4cbe-a71f-a6dd6616523d` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×2; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 20 | `b1211347-041b-496c-b7d6-c8c48a059f89` | CLI | unsupported generic arguments | yes | **Still failing** | `unsupported generic arguments` at step 31; 1 of 33 steps refused; 0 other steps differ from presentation; tracked in agy-reader#s42p |
| 21 | `b57f9326-0450-4aba-9b4d-abe4304c6496` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 3; 1 of 54 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 22 | `b86184c5-71bc-41d9-aa11-bffd04bcfc84` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×4; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 23 | `c0d48394-351d-4fd7-96c0-8dc73ff397e4` | IDE | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 14; 1 of 226 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 24 | `c2d181b1-515f-44e8-ac07-847ef24024e3` | CLI | unsupported native tool projection |  | **Still failing** | `unsupported native tool projection` at step 26; 3 of 38 steps refused; 0 other steps differ from presentation; tracked in agy-reader#ntzm |
| 25 | `c30dc2ff-1ec1-4cf1-bb91-9cb0342df8d0` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×3; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 26 | `cbc70748-feb7-4a96-b914-83444a92bf88` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 3; 1 of 16 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 27 | `de24bfc3-ebae-4c6c-a803-665f84edf23c` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 8; 1 of 72 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 28 | `de385d5a-e4cc-489b-802f-71aaa50470c4` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×1; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 29 | `e2b37e93-3ffd-4f05-9447-da747b231675` | CLI | unsupported native tool projection |  | **Still failing** | `unsupported resultless view_file` at step 309; 6 of 1114 steps refused (5 unsupported resultless view_file, 1 unsupported generic wrapper); 0 other steps differ from presentation; tracked in agy-reader#2xqw, agy-reader#axnr |
| 30 | `e2e12f5f-ea5e-4643-9730-01024cb6e8ed` | CLI | unsupported generic wrapper |  | **Still failing** | `unsupported resultless view_file` at step 84; 1 of 190 steps refused; 0 other steps differ from presentation; tracked in agy-reader#2xqw |
| 31 | `e31ab9a6-eba7-4f81-a5ca-d45e2f5617ed` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unknown persisted protobuf fields`: ChatMessagePrompt field 23 ×3; whole source refused, no per-step count; tracked in agy-reader#g145 |
| 32 | `efc122f2-8247-44a1-9cd1-3a580ec8a6e2` | CLI | unsupported generic arguments |  | **Still failing** | `unsupported resultless view_file` at step 347; 1 of 365 steps refused; 0 other steps differ from presentation; tracked in agy-reader#2xqw |
| 33 | `f3c0899f-0ead-4299-a562-00d9311bbe46` | CLI | unknown persisted protobuf fields |  | **Still failing** | `unsupported generic result` at step 6; 1 of 138 steps refused; 0 other steps differ from presentation; tracked in agy-reader#jgfc |
| 34 | `017690ff-19f6-4db8-92a9-48e2afadd729` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 135 steps; attestation rechecks verified; 3 field-50 records |
| 35 | `04c5dd15-fab6-4f65-9a55-5951f323da42` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 36 | `05b227b2-e660-49f3-8d17-ea1f3059441a` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 13 steps; attestation rechecks verified; 3 field-50 records |
| 37 | `072c15ad-d596-4bc9-b7c7-ce9a0df5fc70` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 38 | `0806de5a-10f1-411d-9124-6f4c2aac3d96` | IDE | unknown persisted protobuf fields |  | Verified, declared source-only reason | 66 steps; attestation rechecks verified; 3 field-50 records |
| 39 | `0a120ed9-201b-44fa-9c22-d99870a1bce9` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 112 steps; attestation rechecks verified; 3 field-50 records |
| 40 | `0be2394b-f41d-4008-85c1-411a0676aa4d` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 41 | `0d84d312-b514-4388-b8b7-c82296041294` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 42 | `11d45cca-1c62-479d-a281-fca9df426354` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 18 steps; attestation rechecks verified; 2 field-50 records |
| 43 | `152700d3-fe94-40d3-b6f2-a3365a9037cc` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified; 3 field-50 records |
| 44 | `19026f19-13db-4c28-9008-a842b4dcd5c4` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 56 steps; attestation rechecks verified; 5 field-50 records |
| 45 | `1aab9e49-0e6f-4ebf-b7ee-956094001f60` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 46 | `2266004f-2dbf-4b15-9058-0c7533ce88bc` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 64 steps; attestation rechecks verified; 2 field-50 records |
| 47 | `3269edd8-3f7e-4a6e-9200-a1aaaa071c53` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 78 steps; attestation rechecks verified; 7 field-50 records |
| 48 | `337a5d4a-cdae-480d-88d9-566a4f4f50c8` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 49 | `3aec5ef4-d5ca-484e-911b-04bb63b1953f` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 50 | `3d094635-6ec4-4770-8b3e-b62de000818c` | IDE | unknown persisted protobuf fields |  | Verified, declared source-only reason | 6 steps; attestation rechecks verified; 2 field-50 records |
| 51 | `453c72cb-2aa5-45a9-a24a-2520e5b4631a` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 52 | `459c4b94-947a-4969-8d69-c133ef188850` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 55 steps; attestation rechecks verified; 9 field-50 records |
| 53 | `4dc0c441-7246-4acc-843a-62a91e8a6ab2` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 54 | `55d64b2d-9748-4917-88f8-fec71d90a2f1` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified; 2 field-50 records |
| 55 | `6024206c-a421-4c7b-bd5a-46a3a28e8691` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 56 | `68c3c00f-5870-49eb-9c48-3e2513479ddd` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 21 steps; attestation rechecks verified; 2 field-50 records |
| 57 | `6c7be1e4-7e1e-4b37-9f16-0a09feade911` | IDE | unknown persisted protobuf fields |  | Verified, declared source-only reason | 94 steps; attestation rechecks verified; 6 field-50 records |
| 58 | `6e4be3da-24d5-415c-a87b-8d316816c785` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 59 | `70d6c75e-ece8-476e-a19f-424ba94e1050` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 3 steps; attestation rechecks verified; 2 field-50 records |
| 60 | `7241b9b2-f6dc-4388-99b0-0878e0df8cef` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 61 | `76ed9323-5309-44a5-9c35-aebdc3683586` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 56 steps; attestation rechecks verified; 3 field-50 records |
| 62 | `79bdd736-1868-4eef-a56c-785a6970b79b` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 7 steps; attestation rechecks verified; 3 field-50 records |
| 63 | `8e7f8a2e-3872-4c24-a291-b38162b90ab8` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 64 | `92d82d04-fe14-4130-9e04-b74e036bf035` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 20 steps; attestation rechecks verified; 2 field-50 records |
| 65 | `974d713a-4372-470c-8c3f-c67a49ff443d` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 90 steps; attestation rechecks verified; 4 field-50 records |
| 66 | `9845be35-4e52-40b1-951e-d0328a451bc2` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 67 | `993ea5ed-a7e6-4709-b94f-74b472202e95` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 68 | `9e58ab9d-28c4-4450-8bc6-74d7575a6811` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 69 | `a18c2e5e-8ce7-468a-b097-3e902fe94dbd` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 70 | `a4c9094e-7912-41a7-bfc5-91d261ca6007` | IDE | unknown persisted protobuf fields |  | Verified, declared source-only reason | 12 steps; attestation rechecks verified; 2 field-50 records |
| 71 | `a59a5104-dc04-40e8-958e-b22b37c7aee4` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 72 | `a6788d9b-5d34-4305-9deb-73c6bfab43f4` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 73 | `a95cf8aa-44cf-44eb-8186-05f69eda6fd6` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified; 2 field-50 records |
| 74 | `a96279a8-3592-43c7-b001-7ec483dd99de` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 75 | `b6492909-f8d8-4091-a4d2-e185cfe5d83d` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 76 | `b729c1ec-a905-4b69-84a3-5deacd50a406` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 7 steps; attestation rechecks verified; 2 field-50 records |
| 77 | `bb05de36-10b5-4442-a931-eaba13b8adca` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 78 | `bd64b535-3ca7-4729-bd53-36b6caa793a2` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 79 | `bfd91871-e244-4d89-be7c-1a505a9deaa0` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 46 steps; attestation rechecks verified; 2 field-50 records |
| 80 | `cb8cbb8b-72cd-41f3-9c02-b5d0aba035b1` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 8 steps; attestation rechecks verified; 2 field-50 records |
| 81 | `d0a870aa-d473-4d32-8aef-ebcd21fad824` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 44 steps; attestation rechecks verified; 3 field-50 records |
| 82 | `daf804ba-f911-46a1-95bd-a6450a7eee80` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 83 | `e6fa5434-3701-4d14-9aae-13e37832a502` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 84 | `e730d5da-5908-45c3-b318-4abea2f0efd9` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 56 steps; attestation rechecks verified; 2 field-50 records |
| 85 | `e82708c4-01f6-44ef-8dbb-f83a89f9fb52` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 14 steps; attestation rechecks verified; 3 field-50 records |
| 86 | `eb75850f-20a3-45ce-a221-abb375e38aa8` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 24 steps; attestation rechecks verified; 2 field-50 records |
| 87 | `edee31a6-06f9-46ca-942c-60c0da915e3c` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 132 steps; attestation rechecks verified; 2 field-50 records |
| 88 | `fb2d9a19-54c7-401b-a361-2123222731a0` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 6 steps; attestation rechecks verified; 2 field-50 records |
| 89 | `fcf78168-0559-4217-8f8b-f6712cfa854d` | IDE | unknown persisted protobuf fields |  | Verified, declared source-only reason | 68 steps; attestation rechecks verified; 7 field-50 records |
| 90 | `ffa883c6-0747-4686-800a-efb8f1fd5605` | CLI | unknown persisted protobuf fields |  | Verified, declared source-only reason | 4 steps; attestation rechecks verified; 2 field-50 records |
| 91 | `0753b4eb-8bfb-44d8-b698-2cacb86a34b4` | CLI | unsupported generic arguments |  | Verified, valid attestation | 22 steps; attestation rechecks verified |
| 92 | `0a4ffcaa-5348-4235-9753-66e9fb8b78d1` | CLI | unsupported generic arguments |  | Verified, valid attestation | 146 steps; attestation rechecks verified |
| 93 | `0b84db4a-0ec3-4409-9fb5-6c1b444fb7b4` | CLI | unsupported native tool projection |  | Verified, valid attestation | 106 steps; attestation rechecks verified |
| 94 | `11e42ea7-27ef-476f-917c-773ccabe6734` | CLI | unsupported generic arguments |  | Verified, valid attestation | 89 steps; attestation rechecks verified |
| 95 | `140748a3-b47a-4229-a6bc-109af71b8ffc` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 30 steps; attestation rechecks verified |
| 96 | `16cdf910-8b9a-4c4b-80aa-6cbc7fc10f2f` | CLI | unsupported wrapper/native status difference | yes | Verified, valid attestation | 3 steps; attestation rechecks verified |
| 97 | `17569afa-d1b4-40e5-927d-5932f3032df3` | CLI | unsupported native tool projection |  | Verified, valid attestation | 92 steps; attestation rechecks verified |
| 98 | `193843a6-bfb5-4465-b4ac-f772b9da76e5` | CLI | unsupported native tool projection |  | Verified, valid attestation | 741 steps; attestation rechecks verified |
| 99 | `19de72b6-f658-474e-a964-ed1ad63fd760` | CLI | unsupported wrapper/native status difference |  | Verified, valid attestation | 61 steps; attestation rechecks verified |
| 100 | `1ca56a1a-507b-45ba-bf70-6f8a0a75e6c5` | CLI | unsupported generic arguments |  | Verified, valid attestation | 5 steps; attestation rechecks verified |
| 101 | `1f20540b-2613-4160-a709-65dfc9f5d3de` | CLI | unsupported native tool projection |  | Verified, valid attestation | 76 steps; attestation rechecks verified |
| 102 | `243aa9f1-690f-4e17-b5bc-9ae6cf37908e` | CLI | unsupported generic arguments |  | Verified, valid attestation | 44 steps; attestation rechecks verified |
| 103 | `2c2a3c3e-a484-4663-992d-1a77b73a4b99` | CLI | unsupported native tool projection |  | Verified, valid attestation | 169 steps; attestation rechecks verified |
| 104 | `2cdb66ff-7112-4274-9902-4bbc9bbfc612` | CLI | unsupported generic arguments |  | Verified, valid attestation | 92 steps; attestation rechecks verified |
| 105 | `2d3470e0-4ad8-4ca1-b60f-c9c258a6ba28` | CLI | unsupported native tool projection |  | Verified, valid attestation | 261 steps; attestation rechecks verified |
| 106 | `2eeda05c-fe5f-402f-88a4-54964d9ba3ba` | CLI | unsupported native tool projection |  | Verified, valid attestation | 44 steps; attestation rechecks verified |
| 107 | `32aee83b-2fe3-400b-9d70-8caa86eb7a98` | CLI | unsupported wrapper/native status difference |  | Verified, valid attestation | 475 steps; attestation rechecks verified |
| 108 | `334f9877-6342-40ee-8c30-8a1efedcaf1c` | CLI | unsupported native tool projection |  | Verified, valid attestation | 1877 steps; attestation rechecks verified |
| 109 | `3627c10e-dddc-4567-89a6-81e694d91ba4` | CLI | unsupported generic arguments |  | Verified, valid attestation | 106 steps; attestation rechecks verified |
| 110 | `3a79a49b-acd2-436a-ad0a-6f89b26262a7` | CLI | unsupported wrapper/native status difference |  | Verified, valid attestation | 33 steps; attestation rechecks verified |
| 111 | `3ee509bc-71e6-43f6-925e-184cde5a6bef` | CLI | unsupported generic arguments | yes | Verified, valid attestation | 10 steps; attestation rechecks verified |
| 112 | `410395a3-b763-4114-8b12-6a8f63b3a020` | CLI | unsupported generic arguments |  | Verified, valid attestation | 822 steps; attestation rechecks verified |
| 113 | `4731c2d1-06e3-4eb4-a224-61d5509845fc` | CLI | unsupported generic arguments |  | Verified, valid attestation | 84 steps; attestation rechecks verified |
| 114 | `49685b38-5f90-4bb4-8ed1-5feee886617b` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 110 steps; attestation rechecks verified |
| 115 | `4d81f901-4165-458a-82d8-8ca11f617e76` | CLI | unsupported generic arguments |  | Verified, valid attestation | 8 steps; attestation rechecks verified |
| 116 | `4ee65cac-5201-417c-82f3-bc0ae66721ec` | CLI | unsupported generic result |  | Verified, valid attestation | 540 steps; attestation rechecks verified |
| 117 | `57cf1685-bb30-4538-9c6d-684eba752955` | CLI | unsupported generic result |  | Verified, valid attestation | 128 steps; attestation rechecks verified |
| 118 | `62ca97b5-405c-4a97-a9fa-a9aa9f1759d9` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 65 steps; attestation rechecks verified |
| 119 | `62ce7231-48cc-4efc-acd1-5c7acb6bed65` | CLI | unsupported generic result |  | Verified, valid attestation | 84 steps; attestation rechecks verified |
| 120 | `634209b0-c989-4528-8416-f3af877fda50` | CLI | unsupported native tool projection |  | Verified, valid attestation | 153 steps; attestation rechecks verified |
| 121 | `64f508ef-7a8e-4807-872f-1ad5e9094558` | CLI | unsupported native tool projection |  | Verified, valid attestation | 271 steps; attestation rechecks verified |
| 122 | `66a24054-e957-4494-879b-39d1dffc59fb` | CLI | unsupported wrapper/native status difference | yes | Verified, valid attestation | 57 steps; attestation rechecks verified |
| 123 | `67f07231-2e17-454d-99f4-725b3e32380b` | CLI | unsupported generic arguments |  | Verified, valid attestation | 46 steps; attestation rechecks verified |
| 124 | `6c06bc3e-9243-4871-af2c-d105ae82317f` | CLI | unsupported native tool projection |  | Verified, valid attestation | 154 steps; attestation rechecks verified |
| 125 | `6c06f1d1-1716-4a2b-98d7-3b06b29625d7` | CLI | unsupported generic arguments |  | Verified, valid attestation | 6 steps; attestation rechecks verified |
| 126 | `70255f33-8572-4f88-878e-42c3b4a074b6` | CLI | unsupported native tool projection |  | Verified, valid attestation | 42 steps; attestation rechecks verified |
| 127 | `71b930f9-6258-4b55-91e6-185794a38932` | CLI | unsupported generic arguments |  | Verified, valid attestation | 78 steps; attestation rechecks verified |
| 128 | `745a9c7a-ef6b-4bee-ad6c-bb2741ec20c9` | CLI | unsupported generic arguments |  | Verified, valid attestation | 24 steps; attestation rechecks verified |
| 129 | `7520fb2d-6b25-48cd-baf9-82202f8f4dfc` | CLI | unsupported native tool projection |  | Verified, valid attestation | 273 steps; attestation rechecks verified |
| 130 | `7529b27d-27bc-47f3-bb42-7c35d19709a3` | CLI | unsupported generic arguments |  | Verified, valid attestation | 28 steps; attestation rechecks verified |
| 131 | `755f81a3-1632-44ff-9bde-33fa1d5f7970` | CLI | unsupported native tool projection |  | Verified, valid attestation | 152 steps; attestation rechecks verified |
| 132 | `771f3335-f72f-41d6-aa7d-eb238aef309b` | CLI | unsupported generic arguments |  | Verified, valid attestation | 18 steps; attestation rechecks verified |
| 133 | `7e2aa696-5b90-4cad-8411-b3c1b27d41f9` | CLI | unsupported generic arguments |  | Verified, valid attestation | 52 steps; attestation rechecks verified |
| 134 | `8301bfa9-cc8e-4267-89da-c6b1cf481661` | CLI | unsupported generic arguments |  | Verified, valid attestation | 5 steps; attestation rechecks verified |
| 135 | `834ccc26-7a46-4534-9580-bf996f42ea31` | CLI | unsupported native tool projection |  | Verified, valid attestation | 10 steps; attestation rechecks verified |
| 136 | `856dbc95-58d9-4874-a497-b8c0b3374365` | CLI | unsupported generic arguments |  | Verified, valid attestation | 302 steps; attestation rechecks verified |
| 137 | `870ae1f8-f253-49d7-8376-5f6d12096bb0` | CLI | unsupported native tool projection |  | Verified, valid attestation | 276 steps; attestation rechecks verified |
| 138 | `88c70b76-5d47-44b8-a184-d84f21df1f2c` | CLI | unsupported native tool projection |  | Verified, valid attestation | 5 steps; attestation rechecks verified |
| 139 | `8c9e4262-3c17-4594-bf03-aeeccbfca9eb` | CLI | unsupported generic arguments |  | Verified, valid attestation | 253 steps; attestation rechecks verified |
| 140 | `8cb69ec2-a9f5-49f4-b47b-7fd1c4229394` | CLI | unsupported generic arguments |  | Verified, valid attestation | 88 steps; attestation rechecks verified |
| 141 | `8fc67023-092f-4984-83cc-26e88f98eda4` | CLI | unsupported generic arguments |  | Verified, valid attestation | 94 steps; attestation rechecks verified |
| 142 | `92f4948a-942f-40bd-be1e-7f185fa5e57a` | CLI | unsupported native tool projection |  | Verified, valid attestation | 18 steps; attestation rechecks verified |
| 143 | `94793426-865e-40d0-a7d9-d56929d74ab4` | CLI | unsupported native tool projection |  | Verified, valid attestation | 42 steps; attestation rechecks verified |
| 144 | `961cfb93-0c77-45d2-b76a-2efbb7cf844d` | CLI | unsupported generic wrapper | yes | Verified, valid attestation | 6 steps; attestation rechecks verified |
| 145 | `981b08af-6edb-4dbb-920a-2d2499a26ccf` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 97 steps; attestation rechecks verified |
| 146 | `9af85a5e-4afa-4a96-aecf-f307e6d0c740` | CLI | unsupported wrapper/native status difference | yes | Verified, valid attestation | 147 steps; attestation rechecks verified |
| 147 | `9e1c541d-2cb7-478b-8544-707b1cbd0e37` | CLI | unsupported native tool projection |  | Verified, valid attestation | 32 steps; attestation rechecks verified |
| 148 | `9e7db543-69cd-42a0-a1d6-4b5ecd205c3e` | CLI | unsupported generic arguments |  | Verified, valid attestation | 58 steps; attestation rechecks verified |
| 149 | `a071509f-262e-4030-803a-cc64b7ec105e` | CLI | unsupported native tool projection |  | Verified, valid attestation | 369 steps; attestation rechecks verified |
| 150 | `a3ee2b48-31fd-4fba-853d-170c9bb6b66e` | CLI | unsupported native tool projection |  | Verified, valid attestation | 39 steps; attestation rechecks verified |
| 151 | `a536674f-8942-47ea-86f6-ae6e8b9cf391` | CLI | unsupported generic arguments |  | Verified, valid attestation | 27 steps; attestation rechecks verified |
| 152 | `a542d750-1d72-4278-a7ec-690dd2ef1f7a` | CLI | unsupported native tool projection |  | Verified, valid attestation | 127 steps; attestation rechecks verified |
| 153 | `a8c89436-6771-4751-9da4-324796a4998b` | CLI | unsupported generic arguments |  | Verified, valid attestation | 198 steps; attestation rechecks verified |
| 154 | `a8d2a036-1740-4376-8ae5-9a12883b8268` | CLI | unsupported native tool projection |  | Verified, valid attestation | 22 steps; attestation rechecks verified |
| 155 | `aa77bc8b-3928-4a2f-acfd-83dc9c5c794b` | CLI | unsupported native tool projection |  | Verified, valid attestation | 14 steps; attestation rechecks verified |
| 156 | `ad78a1a1-54ae-4954-bb30-621adfbe8c7e` | CLI | unsupported generic arguments |  | Verified, valid attestation | 201 steps; attestation rechecks verified |
| 157 | `afcdb6da-6264-4671-9f89-082dd6271d08` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 35 steps; attestation rechecks verified |
| 158 | `afef2def-56bd-467a-a9eb-33e1696704fb` | CLI | unsupported native tool projection |  | Verified, valid attestation | 9 steps; attestation rechecks verified |
| 159 | `b765c9ad-f852-42d7-b8a6-b0005eecc50b` | CLI | unsupported wrapper/native status difference |  | Verified, valid attestation | 44 steps; attestation rechecks verified |
| 160 | `b7ae1a0d-f841-4aa4-bbb3-3e92438cb383` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 59 steps; attestation rechecks verified |
| 161 | `bd21a161-c93d-4475-94bc-7efae7c6acca` | CLI | unsupported native tool projection |  | Verified, valid attestation | 44 steps; attestation rechecks verified |
| 162 | `c305573a-afb1-4fde-94fc-59fdb310a082` | CLI | unsupported generic arguments |  | Verified, valid attestation | 42 steps; attestation rechecks verified |
| 163 | `c886758f-edfd-4a1d-9bea-3f10e559217b` | CLI | unsupported generic arguments |  | Verified, valid attestation | 70 steps; attestation rechecks verified |
| 164 | `cfc9ca4b-0ab0-41a2-9cf5-6bc631637d74` | CLI | unsupported native tool projection |  | Verified, valid attestation | 19 steps; attestation rechecks verified |
| 165 | `cfcb3d84-51dc-44f6-8adb-743057283922` | CLI | unsupported generic arguments |  | Verified, valid attestation | 114 steps; attestation rechecks verified |
| 166 | `dc193c58-0123-4828-90b3-4f63ee11f10b` | CLI | unsupported native tool projection |  | Verified, valid attestation | 154 steps; attestation rechecks verified |
| 167 | `e18a9fe7-c591-4b14-a617-2e86604ddee6` | CLI | unsupported generic arguments |  | Verified, valid attestation | 73 steps; attestation rechecks verified |
| 168 | `e1a27237-8e78-4698-acef-004c847c4dec` | CLI | unsupported generic arguments | yes | Verified, valid attestation | 76 steps; attestation rechecks verified |
| 169 | `e37018a5-751f-4684-b243-7c7e0e6aaf28` | CLI | unsupported native tool projection |  | Verified, valid attestation | 5 steps; attestation rechecks verified |
| 170 | `e406fbb6-5830-4661-94d3-7c04f00ec49c` | CLI | unsupported native tool projection |  | Verified, valid attestation | 26 steps; attestation rechecks verified |
| 171 | `eb695ac1-86bf-41b2-81ba-bd398201e611` | CLI | unsupported native tool projection |  | Verified, valid attestation | 38 steps; attestation rechecks verified |
| 172 | `ed3e3fbe-5cb2-4032-bc9e-fe28fbfaa80e` | CLI | unsupported native tool projection |  | Verified, valid attestation | 321 steps; attestation rechecks verified |
| 173 | `ef47b8d7-8a14-42ca-89b2-b302146a3c9d` | CLI | unsupported native tool projection |  | Verified, valid attestation | 304 steps; attestation rechecks verified |
| 174 | `ef8876f5-22de-4518-80d9-c5e0a133c7eb` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 132 steps; attestation rechecks verified |
| 175 | `f228c323-f857-4fea-b39e-60aa0555bebc` | CLI | unsupported generic arguments |  | Verified, valid attestation | 74 steps; attestation rechecks verified |
| 176 | `f229226f-b178-46fa-a37a-39244aa6d0e1` | CLI | unsupported generic result |  | Verified, valid attestation | 186 steps; attestation rechecks verified |
| 177 | `f373ab08-b0c5-460d-94c0-d1cb3d6b834c` | CLI | unsupported wrapper/native status difference |  | Verified, valid attestation | 309 steps; attestation rechecks verified |
| 178 | `f5c5abe6-50ea-4785-b7be-9ab0db4070d4` | CLI | unsupported generic arguments |  | Verified, valid attestation | 70 steps; attestation rechecks verified |
| 179 | `f839db4d-258d-4ca1-a06a-2ec2bd40bfe6` | CLI | unsupported generic wrapper |  | Verified, valid attestation | 66 steps; attestation rechecks verified |
| 180 | `f86ea576-596c-4fd9-9d86-971b7c4fce49` | CLI | unsupported generic arguments |  | Verified, valid attestation | 84 steps; attestation rechecks verified |
| 181 | `f94771fb-0308-4c6f-b9b4-e2b6294aa655` | CLI | unsupported wrapper/native status difference |  | Verified, valid attestation | 381 steps; attestation rechecks verified |
| 182 | `ff7b965e-835c-4905-af72-f3d9d1fb0baa` | CLI | unsupported native tool projection | yes | Verified, valid attestation | 170 steps; attestation rechecks verified |
