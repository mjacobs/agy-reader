# agy 1.5.1 compatibility

The final agy 1.5.1 sweep fetched and rendered **499/499 conversations
(19,723 steps)**. Native reader verification passed for **498/499**, including
the supplied 67-step feature-exercise conversation. The remaining conversation
is refused at two artifact overwrites; it is tracked as `agy-reader#48m3`.
This is a qualified compatibility baseline, not universal feature coverage.

## Binary and protobuf evidence

The installed `agy --version` printed 1.5.1. The supplied pair's running
executable matched its SHA-256,
`72738cf0445918483956a5f0da33abdfcfb21a12dd9a421fd57d063782826669`;
its startup log independently reported 1.5.1. The fresh daemon used for the
final sweep also logged 1.5.1 and ran that same executable.

The [protobuf snapshot](../../artifacts/proto-snapshots/agy-1.5.1-72738cf04459/snapshot.json)
rechecks **395/395 embedded byte ranges** against the binary. All **34/34
closure files** load and compile back to matching descriptors. The
[comparison with 1.5.0](agy-1.5.1-proto-delta.json) contains **10 additive wire
changes, zero JSON-facing changes, and one deprecation annotation**:

- `CortexStepUserInput.client_message_id` (field 19).
- `SubagentSpec.model_option` (field 12).
- Four new interaction messages and their four call/result fields:
  experimental tools and Finish.
- `CascadeToolConfig` field 38 is marked deprecated.

These names establish definitions, not runtime behavior. None of the new
fields appeared in the fresh corpus's RPC JSON, and native decoding succeeded
for every selected source. The reader's historical embedded schema and
verification profile therefore remain unchanged. The new release's complete
definitions are preserved in the snapshot; no unobserved presentation rule
or source-only exemption is enabled.

## Changelog risks and scenario coverage

The 1.5.1 changelog's compatibility risks are ACP session cancellation,
resume and notification ordering, and quicker terminal completion when a
background child holds output open. Model availability and blocked-subagent
labels may affect configuration or presentation. The remaining notes concern
OAuth sandboxing, Windows file/command behavior, plugin availability, and
terminal UI. No storage migration or encryption change is announced.

The supplied exercise, `eedbe984-027a-446f-8513-2637c665c5ae`, was still
running during the first sweep. It was interrupted with SIGINT to obtain a
stable source; the pair exited. Its final capture contains 67 steps: one
user input, 32 planner responses, seven file reads, 25 commands, one generic
step, and one system message. An isolated SQLite copy and independently
fetched full/default RPCs passed `TestNativeEvidence`, including cached
attestation checking. Source hashes stayed unchanged throughout capture.

The exercise mostly inspected the reader and audit tooling. It does **not**
demonstrate ACP cancellation/resume, all-client model discovery, held-output
timing, remote-agent behavior, or Windows fixes. No successful exercise of
those release changes is claimed. The broad sweep supplies retained tool and
step coverage, not proof of every new optional feature.

## Storage, shape, and retained refusal

SQLite remains `user_version=1`, seven tables, and the same two indices.
Its fingerprint is unchanged:
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
The fully refreshed corpus has sidecar shape fingerprint
`sha256:6fd327e667813e6f3225a4eb7cf16f57132b15e4f432fe7c8d82c1a2e1f5b868`.

Compared with the retained 1.5.0 canonical path inventory, this union adds
**two paths and removes zero**:

```text
steps.[].userInput.lastUserConfig.plannerConfig.toolConfig.antigravityBrowser -> object
steps.[].userInput.lastUserConfig.plannerConfig.toolConfig.antigravityBrowser.enableBrowserSubagentWebview -> boolean
```

That browser setting is already supported by the `config150` overlay. This is
optional configuration coverage, not evidence of a newly introduced schema
field. Corpus turnover still limits release-level attribution.

Conversation `d9eefb53-23fe-49de-aee1-383a3db80927` fetches and renders all
126 steps but refuses native verification at zero-based steps 108 and 118.
Both are completed modern `write_file` calls with `FileEdit.kind=OVERWRITE`
and `isArtifactFile=true`. One includes artifact metadata; the other omits it.
`overwrite1411` deliberately excludes artifact overwrites. The refusal
reproduces with the fresh daemon and isolated source, so it is not explained
by the supplied pair's stale cache. It predates the supplied exercise; this
audit does not establish which release introduced it. `agy-reader#48m3`
records the bounded qualification work and private evidence location.

## Checks and evidence retention

- Strict sweep: complete manifest, 499 fetched/rendered, zero fetch/render
  failures, 498 natively verified, one refused.
- Format helper: read-only and `--record --corpus-swept` runs pass, requiring
  the scenario above and the newest source (the same conversation).
- `go test ./...`: passes during both helper runs.
- Python maintenance suite: 172 tests, 17 skipped, no failures.
- Snapshot verification: 395 byte ranges and 34 roundtrip files pass.
- Isolated native scenario: 67 steps verify, including cached attestation.
- AgentsView consumer tests and the IDE surface were not tested.

The first diagnostic sweep selected 500 sources; the stable final sweep
selected 499. The final result covers exactly its selected inventory. A private
source backup, both sweep manifests/corpora, full/default captures, canonical
path inventories, and command logs remain under ignored
`tmp/agy-1.5.1-audit/`. The sweeps did not write live sidecars. The old pair's
watcher stopped with it; the temporary audit daemon is stopped after capture.

Only schema definitions and the [content-free evidence summary](agy-1.5.1-evidence.json)
are committed. Recording updates both checkout compatibility files. It does
not install the new baseline or restart a deployed reader.
