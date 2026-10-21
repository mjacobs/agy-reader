# agy 1.4.4 compatibility audit

This records the initial `tools144-skill1` qualification. The
[historical-conversation follow-up](agy-1.4.4-historical.md) covers the eight
SQLite refusals below with the later `history2` profile. Counts and installation
status in this initial report describe that earlier audit.

agy 1.4.4 passes the reader's compatibility gate after four bounded tool
mappings were added. All five required conversations verify against their
persisted source, full RPC and default RPC. The format-audit skill exited 0
and generated [COMPATIBILITY.md](../../COMPATIBILITY.md) from clean commit
`280ccb7e114af5d044fdadb332b74b9050807508`. The corrected protobuf snapshot
contains 378 descriptors, with both false candidates removed.

This qualifies the observed reader mappings and corpus. Eight historical
SQLite shapes remain unsupported, and 15 legacy protobuf conversations remain
unverifiable. No reader installation, service restart or downstream deployment
was performed.

## Native qualification

The serving process was checked through its executable: version 1.4.4, SHA-256
`337dcde668441a2828968644d7982829c2d2a5e0c4144f346e1f3fc6abb3cdc7`.
A temporary idle process supplied a pinned authenticated endpoint. Private
conversations, source identities and raw RPC responses stay outside this
repository. No model prompt was submitted by this audit.

The initial audit failed on the newest 73-step conversation and left the
baseline at 1.4.3. A fresh full/default capture of that same stable source
qualified the following mappings, added in `63c6fac` and corrected in
`280ccb7`:

- Canceled background commands with render-only results become native command
  steps with the stored command and empty output. Metadata and task details
  survive; no exit code, working directory or blocking flag is invented.
- Completed `ManageSubagents` list results with two conversation IDs remain
  generic and gain the exact action and comma-joined IDs in result metadata.
- Subagent invocations accept `Workspace=inherit` when the stored arguments
  match the retained tool call. Child specifications and results survive intact.
- Modern reads carry `isSkillFile=true` only when the step's execution ID
  selects an executor whose persisted registry lists that exact path as an
  enabled skill. Unregistered `SKILL.md` reads remain unmarked; an unexpected
  or missing flag fails comparison. Duplicate executor or registration entries
  are refused.

Review identified that a filename alone cannot predict the skill flag. Commit
`280ccb7` replaced that initial rule with the persisted registration lookup.
The rule agrees with all 38 modern reads in the captured corpus: four marked
and 34 unmarked, with no mismatches. Synthetic tests also preserve unmarked reads
of the same filename and reject conflicting registrations.

The final profile ends in `tools144-skill1`; older attestations require
re-verification, including those from the initial filename rule.
The embedded 1.3.4 descriptor and declared source-only allowances did not change.
Permissions, completed interactions, unsupported actions, argument names,
workspace selectors and unknown fields remain refused. These are observed
shapes, not evidence of when agy introduced them.

`TestNativeEvidence` passed for all **73 steps**, including attestation recheck,
with source digest
`3598dd2eb84d57cf0217f543a68557270a971c6170e2a1879311f13ccc6831bd`.
The original database and WAL content hashes stayed unchanged during capture;
the first daemon load created an empty WAL, and a repeated capture then had
stable database and WAL hashes and timestamps. Ordinary `--sync` on a disposable
copy also wrote a 73-step sidecar with the new profile and that source digest.
[Native structural evidence](agy-1.4.4-native-qualification.json) records the
bounded cases and hashes of the private captures, without prompt or tool content.
The [initial refusal shapes](agy-1.4.4-refused-step-shapes.json) remain as the
pre-fix diagnostic record.

## Full corpus and record gate

The fresh version 4 manifest is complete: **396 selected, 396 fetched, 396
rendered, zero fetch/render failures**. Of 381 SQLite conversations, 298 verified
normally, 75 verified with declared source-only fields, and eight did not verify.
Those eight are exactly the historical refusals from the earlier audit; no new
refusal appeared. The current conversation is the one newly verified source.
The 15 legacy protobuf conversations remain unverifiable.

The record gate passed **5/5**: the current 73-step parent and its 36- and
14-step children, plus the prior 276-step parent and eight-step child. Existing
conversations were replayed under 1.4.4; this does not create new feature coverage.
Both read-only and `--record` format-audit runs passed. Formatting, vet, lint,
Go package tests and build passed. Python maintenance tests passed with
**134 run, 17 skipped**. Targeted mapping tests cover unsupported source shapes,
unknown wire fields and changed full/default responses.

SQLite remains `user_version=1` with seven tables and the same fingerprint:
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
The current shape fingerprint is
`sha256:7f21000a784e88dd45487781ef2821c3d724d14018ecf17a8cad55b642e4d8ea`.
It matches the earlier final 1.4.4 corpus. Against the recorded 1.4.3 corpus,
[the shape comparison](agy-1.4.4-shape-delta.json) adds five paths and removes
none: four browser-subagent experiment configuration paths and a system-message
terminal line-count field. This is observed coverage growth; it does not prove
a release-specific rendering change.

[Sanitized evidence](agy-1.4.4-evidence.json) records counts, provenance,
fingerprints and private evidence hashes. The compatibility baseline is generated
by the skill; recording changes the checkout, not the installed reader.

## Protobuf snapshot

[Snapshot metadata](../../artifacts/proto-snapshots/agy-1.4.4-337dcde66844/snapshot.json)
records **378 descriptors**, all rechecked against the installed binary.
The 34-file trajectory/tools closure loaded and roundtripped successfully.
The corrected closure is byte-identical to the original 1.4.4 snapshot, and
all surviving raw records retain their bytes, offsets and hashes.

The extractor now removes descriptor-shaped fragments contained inside another
record before selecting the longest occurrence by name. This removes the two
false `google/cloud/businessaicode/v1main` management and telemetry entries
and their spurious `v1beta` import. There are **zero unresolved inventory
imports**. The older 1.3.6 raw inventory retains its historical false entries;
its decoding closure is unaffected and is the comparison input below.

No 1.4.3-to-1.4.4 release-specific delta is claimed. There was
no saved 1.4.3 snapshot, so comparisons use the 1.3.6 closure and separately the
production 1.3.4 descriptor. The rerun reports are unchanged:

| Comparison | Wire | JSON-facing | Descriptor-only |
| --- | ---: | ---: | ---: |
| [1.3.6 to 1.4.4](agy-1.4.4-proto-delta.json) | 23 | 0 | 4 |
| [Production 1.3.4 to 1.4.4](agy-1.4.4-production-proto-delta.json) | 99 | 0 | 10 |

The 1.3.6 delta contains 14 added fields, seven added messages, and two type
changes: `genai.ViewFileCallStep` fields 2 and 3 changed from unsigned to signed
32-bit integers. Additions include `GenericStepResult.result_struct` and modern
`AskQuestion`, `Finish`, and `SearchWeb` messages. These definitions do not
qualify new reader projections. The production comparison includes removal of
`PolicyGuardianConfig.use_v2`, which already predates 1.3.6.

The full inventory contains one Edition 2026 record outside the selected closure;
this audit does not claim that the full inventory is loadable. The selected
closure uses supported editions and passed every check. The older snapshot's
original binary was unavailable for a new byte-range comparison.

## Changelog verdict and limits

The 1.4.4 notes state no database, encryption, session-layout or reader-RPC
change. Root-conversation metadata belongs to outbound MCP `tools/call`
requests. Compaction retaining tool declarations could affect persisted
configuration, which is why source/full/default verification supplements the
fingerprints.

Fresh compaction, nested MCP metadata, parameterized media attachments, plugin
inheritance and Auto-mode approval scenarios were not independently exercised.
Modern permission-bearing tool payloads remain unsupported. The optional
AgentsView consumer suite and live downstream ingestion were not tested.
