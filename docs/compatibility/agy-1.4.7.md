# agy 1.4.7 compatibility audit

agy 1.4.7 is qualified by the [replay audit below](#replay-qualification),
which resolves the original nine failures. The initial audit remains recorded here as historical evidence.

## Initial audit (before the replay fix)

agy 1.4.7 is **not qualified**. All **419 conversations and 37,436 steps**
fetched and rendered, but **410/419 conversations** passed native
source/full/default verification. The format audit exited 1 and produced no
compatibility record. `COMPATIBILITY.md` remains at 1.4.5.

Follow-up `agy-reader#e6hm` tracks the nine verification failures and the
fresh full-corpus gate required before recording compatibility.

The [protobuf snapshot](../../artifacts/proto-snapshots/agy-1.4.7-40349135371a/snapshot.json)
is verified independently. No production descriptor, projection rule, or
source-only allowance changed in this audit.

## Qualification and failures

The complete version 5 sweep ran from 05:06:07 to 05:07:36 UTC on
2026-10-24 (October 23 locally). An owned temporary daemon reported 1.4.7;
its running executable matched the installed binary's SHA-256,
`40349135371a8ed1f1af1e9f8b741de99174ae9fce6380b96c2d1f543480af7a`.
Listener ownership and authenticated RPC access were checked before capture.

| Check | Result |
| --- | --- |
| Fetch and Markdown render | 419/419; zero failures |
| Source formats | 404 SQLite, 15 legacy protobuf |
| Native verification | 320 fully projected; 90 with declared source-only fields |
| Failed native verification | 7 presentation mismatches; 2 unsupported sources/projections |
| Required record gate | All 419 conversations required; 9 failed |
| Format audit | Exit 1; no record produced |
| Reader unit tests | Passed through the audit helper |

The sweep binary was built from `4c5f5e7`, before the `skill146` marker fix.
The audit helper ran at clean commit `646d23f`, whose only change was the new
snapshot. All selected IDs were supplied through `AGY_QUALIFY_SESSIONS`; no
failure was excluded as historical, non-gating evidence. `--record` was not run after the failed gate.

Offline JSON comparison with the pre-fix `4c5f5e7` reader located
**257 `runCommand.waitMsBeforeAsync` fields** in the daemon's default responses
that its output omitted, across all seven presentation failures. Two of those
conversations also had one `viewFile.isSkillFile` discrepancy each under that
reader; case-06 is the exact conversation tracked as `agy-reader#xpeh`. That
reader's RunCommand projection copied command/output details but did not
populate the wait field.

Case-06's 217-step source is byte-identical to the archived 1.4.6 capture
(SHA-256 `15efa68ae9814fdb649557c1e2bac18d03da822230236d48afa61974e0085fcb`).
Comparing the two default responses finds exactly 86 differences: 1.4.7 adds
`runCommand.waitMsBeforeAsync` to 86 steps; 1.4.6 omitted it. This establishes
changed daemon presentation for that source. A fix must account for both
observed presentations rather than unconditionally adding the field to every
version. The unchanged SQLite schema and union shape cannot expose this change.

One other conversation contains an `exa.cortex.core.tools.Finish` Any payload,
absent from the production descriptor. The last fails with
`unsupported 1.4.3 tool projection`; its precise rejected condition remains
to be diagnosed. The unsupported cases have no same-source version comparison
here, so this audit does not establish when they began. The version comparison
above uses archived evidence; no older daemon was launched in this audit.

All nine initial failures reproduced using disposable SQLite backups and
independent full/default captures with `TestNativeEvidence`, both with the
pre-fix `4c5f5e7` reader and the historical `skill146` diagnostic build at
`d4fca92`. The skill-marker fix was first merged as `7dd87e5`, before the
original report commit `6d89853`, so it was already in the reader when that
report was filed. Main's history was later rewritten; the same patch is now
`ba7d3ce` on main.
As the [1.4.6 follow-up](agy-1.4.6.md) records, `skill146` resolves case-06
against its original 217-step 1.4.6 response and the same source hash above.
It did not resolve the 1.4.7 wait-field omissions, so the skill fix alone did
not qualify the initial corpus. These are historical failures; the
[replay qualification](#replay-qualification) below supersedes that verdict.
During the second capture, original DB bytes stayed unchanged; newly appearing
WAL files were empty. The raw captures remain private. The
[sanitized evidence](agy-1.4.7-evidence.json) records counts, source/capture
hashes, and test outcomes.

## Schema and protobuf changes

SQLite remains `user_version=1` with seven tables and fingerprint
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`,
unchanged from the compatibility record.

The freshly serialized corpus has shape fingerprint
`sha256:181263a6969d3305374cf4580491deca01c05a394681d7b96b4dcfd4b7a00ffc`.
Compared with the initial 1.4.6 corpus, whose digest equals the recorded
1.4.5 baseline, the [shape delta](agy-1.4.7-shape-delta.json) adds 28 paths
and removes none. These cover captured checkpoint requests/media, attachment
inputs, checkpoint/model settings, and skill-discovery settings. Some are
coverage added by the earlier compaction qualification; this is not a claim
that 1.4.7 introduced all 28 paths. The wait-field mismatch is invisible in
this union because the path already exists in other conversations.

The snapshot passed **383 embedded-record and binary-byte-range checks**,
a **34-file decoding closure**, and its **34-file compiled roundtrip**.
The prior 1.4.6 snapshot passed offline verification; its original binary
byte ranges were not rechecked.

The [1.4.6 to 1.4.7 closure delta](agy-1.4.7-proto-delta.json) has three
additive wire changes, zero JSON-facing changes, and four descriptor-option
changes. It adds grounding-support metadata and an MCP app entrypoint title;
the option changes concern deprecation flags on usage and checkpoint fields.
There are no wire removals or type changes in this closure. The full raw
inventory comparison separately reports 61 wire additions, zero JSON-facing
changes, and 36 descriptor changes. Saving definitions does not enable their
use in the production reader or qualify new feature behavior.

## Changelog verdict and scope

Compaction is the main compatibility concern in the 1.4.6–1.4.7 changelog:
attachment retention changed in 1.4.6, and 1.4.7 fixes repeated compaction for
oversized input. Larger inline MCP results also warrant payload scrutiny.
The sweep replays existing attachment-compaction scenarios but does not
exercise fresh oversized-input compaction or the new MCP size threshold.
Hook input gains `lastUserInput`, outside the reader RPC contract. Other
notes concern UI, performance, permissions, plugin setup, and authentication;
none announces a SQLite, encryption, or session-layout migration.

No model prompt was submitted. The sweep wrote a private corpus, not live
sidecars. The temporary daemon was stopped. No reader was installed and no
existing service was restarted. AgentsView tests, downstream ingestion, and
the IDE daemon were not tested.

## Offline export follow-up

The `finish147` reader profile now exports the 233-step case-02 conversation.
The earlier proto snapshot saved `exa.cortex.core.tools.Finish` without adding
it to the production decoder. The production bundle now includes that one
message, and completed calls with `has_result_schema=true` become `FINISH`
steps. Their `arguments_json` text is copied verbatim into both
`finish.output.arguments_json` and `finish.outputString`; outer metadata stays
intact. Calls without a result schema remain unsupported.

After decoding passed, the same source exposed three failed `read_file`
wrappers whose results contain only render information. The reader now maps
the observed `file does not exist` errors with a plain absolute `Path` to
`VIEW_FILE`, retaining error and metadata. Other errors, line-range arguments,
permission records, and missing skill-file reads remain outside this rule.

The reported live database exports successfully as Markdown and JSON. All
233 steps are present; steps 5, 6, 13 and 232 exactly match the archived default
response. Comparing every exported step with that response leaves only 33
missing `runCommand.waitMsBeforeAsync` fields. Full native verification still
fails on that known presentation difference, so this fix does **not** qualify
1.4.7 or change `COMPATIBILITY.md`.

Synthetic regression pairs cover both conversions, altered output and wrapper
data, unsupported variants, unknown wire fields and stale attestations. The
schema generator checks the pinned snapshot hash and copies only `Finish`;
its tests prove every prior declaration survives unchanged. Regenerate from
the repository root with Python and `google.protobuf` available:

```sh
python internal/completeness/generate_finish147.py \
  artifacts/proto-snapshots/agy-1.4.5-8af8e7a2a68f/extracted \
  artifacts/proto-snapshots/agy-1.4.6-128780f57769/extracted \
  artifacts/proto-snapshots/agy-1.4.7-40349135371a/all-extracted.protoset \
  internal/completeness/schema.pb.gz
```

## Replay qualification

The `replay147` profile passes all nine archived native reproductions and their
attestation rechecks. It also passes the archived 217-step agy 1.4.6 response
for case-06 against the same source. The two presentations remain distinct:
modern command payloads omit waits throughout in the historical form, or derive
them from stored arguments throughout in the 1.4.7 form. The comparison accepts
neither altered waits nor a mixture of those forms. Explicit zero cannot hide a
nonzero source value as an old omission. Offline exports preserve stored waits.

Across the nine cases, 324 modern command steps carry waits: the original 257
presentation mismatches plus 33 in the Finish case and 34 in case-05. Case-05's
previously unidentified rejection is 25 binary image reads: two JPEG, 17 PNG,
and six WebP. Each source holds exactly one inline media item, copied unchanged
into `viewFile.mediaData`, with its stored byte count, path and result text.
The rule does not read the original image files. Other MIME types, external
media URIs, multiple items, line ranges and permission records remain outside
this mapping. No descriptor or source-only declaration changed.

The fresh version 5 sweep on October 24, 2026 fetched and rendered **421/421
conversations and 37,436 steps**, with zero failures. Native verification passed
for **331 fully projected conversations and 90 with existing declared
source-only fields**. The corpus contains 406 SQLite and 15 legacy protobuf
sources. All 421 conversations were required by the format-audit gate, which
exited 0. The record gate passed again from clean implementation commit
`103ee3a`, and `COMPATIBILITY.md` now records 1.4.7. The owned idle process
reported agy 1.4.7; its executable SHA-256
matched the verified snapshot, and listener ownership and authenticated RPC
access were checked before capture.

The SQLite schema fingerprint is unchanged. The shape fingerprint is
`sha256:181263a6969d3305374cf4580491deca01c05a394681d7b96b4dcfd4b7a00ffc`,
identical to the initial 1.4.7 sweep. Its difference from the 1.4.5 baseline is
the already inspected [28-path additive delta](agy-1.4.7-shape-delta.json).
The changelog scope remains the one described above: replay covers existing
attachment-compaction data, not a newly exercised oversized-input compaction
or MCP size threshold. This qualification does not establish those features.

The [sanitized replay evidence](agy-1.4.7-replay-evidence.json) records capture
hashes, counts and outcomes. Synthetic regression pairs cover old and current
wait output, corrupted zero and other wait values, mixed forms, image payload
changes, unsupported image-read variants, full-wrapper equality and stale
attestations. Review found and fixed a case-insensitive JSON-key ambiguity in
wait detection; differently cased keys cannot bypass the zero-wait check.
The committed embedded doctor baseline matches the root compatibility record.
Raw captures remain private. The owned process was stopped and its temporary
credentials removed. No live sidecars were written;
no reader was installed and no existing service was restarted. Downstream
AgentsView ingestion was not tested.
