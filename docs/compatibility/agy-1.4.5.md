# agy 1.4.5 compatibility audit

This records the initial corpus replay before the `options145` profile.
The [configuration-field follow-up](agy-1.4.5-options.md) qualifies the three
options that were absent here. The counts and unsupported-field statements
below describe this earlier audit.

agy 1.4.5 passes the reader's compatibility gate for the observed CLI corpus:
**399 conversations, 36,651 steps, zero fetch/render or native-verification
failures**. SQLite and sidecar-shape fingerprints are unchanged from the
recorded 1.4.4 baseline. The format-audit skill generated
[COMPATIBILITY.md](../../COMPATIBILITY.md) from clean reader commit
`73bd0619e5042972f29cce96d1a2149d2ea9c496` on 2026-10-22.

No reader mappings, embedded protobuf schema or source-only allowances changed.
This is a replay of existing conversations, not qualification of every new
1.4.5 option. No model prompt was submitted.

## Corpus and record gate

A temporary idle process supplied a pinned authenticated endpoint. Its running
executable reported 1.4.5 and matched the installed binary SHA-256:
`8af8e7a2a68f72fa7164e00bb59b8902c018fa43e528fc401ad7db8c2972583c`.
The sweep ran from 16:50:17 to 16:51:42 UTC and produced a complete version 5
manifest. Raw conversations, identifiers and credentials remain outside the
repository.

| Check | Result |
| --- | --- |
| Fetch and Markdown render | 399/399, zero failures |
| Source formats | 384 SQLite, 15 legacy protobuf |
| Native/full/default verification | 309 fully projected; 90 with declared source-only fields |
| Unverified conversations | 0 |
| Required record gate | 399/399, including every selected conversation |
| Read-only audit and `--record --corpus-swept` | Both exited 0 |
| `go test ./...` | Passed in both audit runs |

The record gate rechecked each source digest under the current verification
profile. The sweep did not write live sidecars. Recording updated the root
compatibility file and its embedded copy; it did not install a reader or restart
an existing service.

SQLite remains `user_version=1` with seven tables and the fingerprint
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
The union of all 399 freshly serialized sidecars has fingerprint
`sha256:7f21000a784e88dd45487781ef2821c3d724d14018ecf17a8cad55b642e4d8ea`.
Both match the recorded comparable baseline. [Sanitized evidence](agy-1.4.5-evidence.json)
retains counts, provenance and hashes of the private audit outputs.

## Protobuf verification and limits

The [existing snapshot](../../artifacts/proto-snapshots/agy-1.4.5-8af8e7a2a68f/snapshot.json)
matches the installed binary. Verification passed for all **379 raw records and
binary byte ranges**, the **34-file closure**, and its **34-file saved
roundtrip**. No duplicate snapshot was created. The 1.4.4 snapshot passed offline
verification; its original binary byte ranges were not rechecked.

The [1.4.4 to 1.4.5 decoding-closure delta](agy-1.4.5-proto-delta.json) contains
six additive wire changes, zero JSON-facing changes and zero descriptor-only
changes: three configuration fields, two associated messages, and a `BASH`
language enum value. There are no removals or type changes in this closure.

The configuration additions are browser auto-invoke, per-tool output overrides,
and `ViewFileToolConfig.skip_plugin_logo`. The last is configuration, not a new
file-view step payload. The current production descriptor does not define these
additions; their encoded presence in a persisted source would still be refused.
The source-only exceptions were not widened.

A structural scan of the default RPC corpus found none of the new field keys
or the new language value. Existing file-view coverage includes 4,960 native
view-file steps and 57 `read_file` calls. That scan supplements native
verification; it does not establish behavior for the absent options.

The full raw descriptor inventory is broader than that decoding closure: its
comparison reports 68 wire, zero JSON-facing and 93 descriptor-only changes.
One added file receives an existing iOS telemetry message. An agent-executor
input gains `AppendTrajectorySuffixRequest`, carrying steps and metadata; this
is a protocol change worth monitoring, but its definition alone does not prove
a storage change. The reader's three RPC methods (`LoadTrajectory`,
`GetCascadeTrajectory`, `GetAllCascadeTrajectories`) and their immediate
request/response descriptors are unchanged. This is not a claim that every
transitive message or every subsystem is unchanged.

## Changelog verdict

The 1.4.5 notes announce no database, encryption, session-layout or reader-RPC
format change. Most changes concern terminal rendering, CPU use and plugin
installation. Conversation loading, summary refresh, schedule timing and
safety-filter retries are relevant behavioral changes; the completed source
verification and render sweep cover the stored corpus, but do not test fresh
schedules or blocked-model responses.

Fresh optional-feature scenarios, the IDE daemon, the AgentsView consumer suite
and live downstream ingestion were not tested. The new optional protobuf fields
remain outside the qualified coverage.
