# agy 1.5.2 compatibility

The final fresh-daemon sweep fetched and rendered 499/499 conversations
(19,782 steps), with 489 natively verified. All four required conversations
passed qualification, and the compatibility baseline was recorded. Ten
historical conversations remain refused; compatibility is qualified, not
universal feature coverage.

The 1.5.2 audit found a new persisted subagent model-choice representation.
The reader now decodes `inherit` and `named_model` while retaining the older
model-choice fields for historical conversations. No source-only allowance or
presentation relaxation was added.

## Binary and schema evidence

The installed CLI printed 1.5.2. Its SHA-256 and the running pair's executable
(`/proc/3622779/exe`) both matched
`da1751e7658469fa80bf36eda2c60ee61216d8e42b735a184382ba31d1f6b88e`.
The [snapshot](../../artifacts/proto-snapshots/agy-1.5.2-da1751e76584/snapshot.json)
verified 397 embedded byte ranges and 34 closure/roundtrip files.
The [delta from 1.5.1](agy-1.5.2-proto-delta.json) has 22 wire changes,
zero JSON-facing changes, and 17 descriptor-only changes. It includes removals;
the reader therefore retains its historical schema and adds only the new
model-choice fields and their nested message.

The changelog's relevant risks are permission enforcement, outside-workspace
file access, and remote-agent plugin skills. No storage or encryption migration
is announced. The snapshot also exposes media and planner configuration changes;
names alone do not establish their behavior or qualify unused features.

## Native evidence

An isolated capture of the supplied conversation verified all 346 persisted
steps against independently fetched full and default responses, and passed
cached-attestation checking. Original database/WAL hashes were unchanged during
capture. It contains 170 planner responses, 169 generic steps, six system
messages and one user input. This does not establish coverage of every new
permission, enterprise-policy, media or remote-agent feature.

The second broad sweep fetched and rendered 500/500 conversations (19,748
steps), with 487 native verifications. Twelve conversations had source/RPC
content differences; one source disappeared during verification. The first
sweep failed when a selected source disappeared. Neither active-store turnover
nor a successful render is evidence of native sync compatibility. The supplied
conversation verified at 298 steps in the second sweep but changed afterward,
so that manifest could not qualify the compatibility record.

SQLite schema remains unchanged, with seven tables and two indices. The second
sweep's shape fingerprint is
`sha256:77ee9d65ac565c8604d3cd7ded4228366ff33e58efc2cfcd41ca2cd154ee5492`.
Compared with the retained 1.5.1 corpus, five optional paths disappear and none
are added: agent-script customization configuration and its path, hidden,
subagent share, and invoked-subagent share. Corpus turnover prevents attributing
these omissions to the release itself.

## Checks

The Go suite passes. The Python maintenance suite passes 174 tests with 17
skipped. The schema overlay test checks the exact additions and preservation
of every historical declaration. Runtime tests require full/default equality
and reject changed model choices. Native evidence and raw transcripts remain
private; only definitions and this content-free summary are committed.

## Final qualification

A fresh idle 1.5.2 daemon, whose executable matched the snapshot hash and whose
startup log reported 1.5.2, resolved the supplied feature tests' stale-cache
mismatches. The parent conversation verified at 346 steps, the MCP scenario at
six, and print mode at two. Together with the newest conversation, all four
passed the format helper's current-source checks. The recorded reader commit
captures the tested code in a clean checkout.

The MCP trajectory contains the supplied explanation, toolSummary, toolAction,
message and waitForPreviousTools arguments. The print-mode trajectory omits
metadata.title. Native equality passed for both entire conversations. This
confirms payload preservation and title omission, not the absence of every
possible background model request.

The final shape digest matches the earlier sweep. Ten historical conversations
still have source/RPC mismatches with the fresh daemon, including the artifact
overwrite case tracked in agy-reader#48m3. The remaining investigation is tracked as agy-reader#cpqn. No validation was
weakened to accept them. Private evidence is retained under ignored tmp/agy-1.5.2-audit/, including
the final manifest/corpus, isolated native captures and test logs. Recording
updates checkout baselines only; it does not install or restart the reader.
