# agy 1.4.6 compatibility audit

agy 1.4.6 is **not qualified** by this audit. All **405 conversations and
36,870 steps** fetched and rendered, but only **404/405 conversations** passed
native source/full/default verification. One conversation differs at a skill-file
marker. The format audit exited 1 and produced no compatibility record;
`COMPATIBILITY.md` and its embedded copy remain at 1.4.5.

The protobuf snapshot is independently verified. Its new attachment-compaction
fields are absent from both the production descriptor and this replay corpus,
so this audit also does not qualify that new feature. No reader schema,
projection, or source-only allowance was changed.

## Corpus and failing gate

A temporary idle process served the installed agy 1.4.6 binary. Its running
executable reported 1.4.6 and matched SHA-256
`128780f5776959ba6a991bde1f0a8b699eee37a59ed30deb9926147fedfa56c4`.
Listener ownership and authenticated RPC access were checked before capture.
The sweep ran from 09:31:26 to 09:32:49 UTC on 2026-10-23 and produced a
complete version 5 manifest. Raw conversations, identifiers, and credentials
remain outside the repository.

| Check | Result |
| --- | --- |
| Fetch and Markdown render | 405/405, zero failures |
| Source formats | 390 SQLite, 15 legacy protobuf |
| Native/full/default verification | 314 fully projected; 90 with declared source-only fields |
| Unverified conversations | 1 SQLite conversation, 217 steps |
| Required record gate | All 405 conversations required; 404 passed, 1 failed |
| Read-only format audit | Exit 1; no record produced |
| `go test ./...` | Passed through the audit helper |
| `--record` | Not run because qualification failed |

The audit helper ran at clean reader commit `6bae514`, whose only change from
`36c99e8` is the new snapshot. The sweep binary was built from `36c99e8`.
Every selected conversation was supplied through `AGY_QUALIFY_SESSIONS`; the
failing conversation was not excluded as historical, non-gating evidence.

SQLite remains `user_version=1` with seven tables and fingerprint
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
The union of all 405 freshly serialized sidecars has fingerprint
`sha256:d81d128d431fcfb6797e7a1c390b2042d843c7c257b736ca6057bdfe21de96fa`.
Both equal the recorded 1.4.5 values. These unchanged fingerprints do not
override the failed native-verification gate.

## Skill-file presentation mismatch

The failed conversation's default RPC contains
`steps[185].viewFile.isSkillFile: true`; the reader's offline projection omits
that marker. The rest of the projected and presented JSON matches after
removing reader-owned metadata. A read-only SQLite backup remained stable
across capture, and offline export succeeded.

The modern `ReadFile` payload has no skill marker of its own. The matching
executor contains 73 customizations, but none matches the read path exactly
or after resolving filesystem aliases. The current
`registeredSkillRead144` rule therefore predicts no marker. Independent
full/default captures reproduce the failure with `TestNativeEvidence`.
Removing only the marker from a private diagnostic response makes native
verification and its saved-attestation recheck pass for all 217 steps. That
altered response is diagnostic only and does not qualify compatibility.

This is a native observation of a mismatch, not proof that 1.4.6 introduced
it. There was no comparison with a running 1.4.5 daemon. The reader correctly
refused to attest to a presentation it could not predict from its supported
source projection.

Follow-up `agy-reader#xpeh` tracks finding a source-justified mapping and
rerunning the complete corpus. It does not authorize ignoring the flag or
loosening verification. New attachment-compaction coverage remains separate.

## Protobuf evidence and feature limits

The [new snapshot](../../artifacts/proto-snapshots/agy-1.4.6-128780f57769/snapshot.json)
passed verification of all **380 embedded records and binary byte ranges**,
its **34-file decoding closure**, and its **34-file compiled roundtrip**.
The prior 1.4.5 snapshot passed offline verification; its original binary
byte ranges were not rechecked.

The [1.4.5 to 1.4.6 closure comparison](agy-1.4.6-proto-delta.json) contains
**10 additive wire changes, zero JSON-facing changes, and one descriptor
change**. There are no wire removals or type changes. Six new fields and four
messages cover:

- Captured checkpoint user requests containing text and media. The previous
  `user_requests` field is deprecated; it remains defined.
- Model context-window options and checkpoint context/media limits.
- Customization-kind enablement and subagent prompt-section settings.

The production descriptor defines none of these six fields or four messages.
A structural scan found none of the six new field keys, in camel or snake
case, in any of the 405 default-RPC responses. Persisted occurrences remain
unsupported. Tolerant direct JSON rendering is also not qualification: the
current checkpoint renderer reads the legacy user-request and summary fields,
not the new captured text/media payload.

The full raw inventory comparison, separate from the decoding closure,
contains **72 additive wire changes, zero JSON-facing changes, and 23
descriptor changes**. The reader's three RPC methods (`LoadTrajectory`,
`GetCascadeTrajectory`, and `GetAllCascadeTrajectories`) and their immediate
request/response definitions are unchanged. This does not establish that
every transitive type or subsystem is unchanged.

## Changelog verdict and scope

Attachment-preserving compaction is the main compatibility red flag: the
release notes describe retained images, PDFs, and files, and the protobuf
delta supplies a corresponding text/media payload. No fresh attachment or
compaction scenario was submitted, so that behavior remains unqualified.
Shared main-agent/subagent permissions and the large reasoning-signature
command-start fix also lack fresh behavioral scenarios. Other release notes
primarily concern terminal UI, plugin setup, authentication, and sandbox policy;
none announces a SQLite, encryption, session-layout, or reader-RPC format change.

The sweep wrote its private corpus rather than live sidecars. No model prompt
was submitted. No reader was installed and no existing service was restarted.
The owned temporary daemon was stopped after capture.
The optional AgentsView consumer suite, live downstream ingestion, and IDE
daemon were not tested. [Sanitized evidence](agy-1.4.6-evidence.json) preserves
counts, provenance, and hashes of the private captures.
