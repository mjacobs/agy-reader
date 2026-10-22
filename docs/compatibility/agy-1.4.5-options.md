# agy 1.4.5 configuration-field qualification

The reader now verifies conversations that store the three new agy 1.4.5
configuration fields. Previously, it refused such a source with `unknown
persisted protobuf fields`. Two agy-written conversations now pass native
source/full/default comparison, ordinary sync, and saved-attestation rechecks.
The full CLI corpus also passes: **402/402 conversations**, including all
15 legacy sources, with no fetch/render or native-verification failures.

This qualifies storage and RPC preservation. The conversations were idle, with
zero steps: no model prompt, browser action or tool execution was submitted.
Output-truncation behavior, browser auto-invocation and plugin-logo display were
not exercised. The separate `BASH` language enum remains unsupported.

## Native evidence

An owned temporary process ran the installed agy 1.4.5 executable, SHA-256
`8af8e7a2a68f72fa7164e00bb59b8902c018fa43e528fc401ad7db8c2972583c`.
Authenticated `StartCascade` requests supplied the options through
`customAgentSpec.cascadeConfig.plannerConfig.toolConfig`. agy itself wrote the
databases; no protobuf bytes or source records were edited for these cases.

| Option | Enabled case | Defaults and overrides case |
| --- | --- | --- |
| `antigravityBrowser.enableBrowserSubagentAutoInvoke` | `true` | explicitly `false` |
| `viewFile.skipPluginLogo` | `true` | explicitly `false` |
| `toolOutput.perTool` | `read_file: 2048`, `run_command: 4096` | unset, `0`, `-1`, and `8192` limits on separate entries |

The persisted trajectory metadata and both independent RPC responses retained
these exact values under `metadata.staticConfig`. Database and WAL content
hashes stayed unchanged during each capture. SQLite backups supplied the
disposable `TestNativeEvidence` fixtures, and both passed. The old reader's
offline export failed on the enabled case before the change. The updated
reader's normal `--sync` then wrote verified sidecars to disposable copies;
`audit-qualify` rechecked both saved attestations successfully.

[Sanitized evidence](agy-1.4.5-options-evidence.json) retains results and private
capture hashes without conversation identities or transcript content. Synthetic
tests cover seven configuration locations, optional presence, changed or dropped
full/default values, historical fields and stale profile attestations. Those
wider-location tests are not additional native scenarios.

## Bounded schema change

The `options145` profile retains the qualified 1.3.4 descriptor base and adds
only the three fields plus `ToolOutputOverride` and its map-entry message.
The [descriptor delta](agy-1.4.5-options-proto-delta.json) is exactly five
additive wire changes, zero JSON-facing changes and zero descriptor-only changes.
A full 1.4.5 replacement would remove historical `PolicyGuardianConfig.use_v2`
field 14; this update preserves it. Unrelated newer definitions, including
`GenericStepResult.result_struct` and `BASH`, remain refused.

The generator verifies the retained base, the 1.4.5 manifest and all 379 raw
descriptor records before copying the authentic definitions. It preserves
optional-field presence, checks that the resulting pool loads, and writes
atomically. [Rebuild instructions](../completeness.md#schema-provenance-and-maintenance)
identify the pinned inputs and output hash. No source-only exception was added.

The decoder also checks per-tool map entries before protobuf unmarshalling.
Without this check, the library can silently discard unknown map-entry fields
or earlier duplicate keys. Tests reproduce eight such accepted cases with the
guard disabled. The guard rejects unknown, malformed, wrongly encoded and
duplicate entry members, including duplicate keys split across repeated
singular enclosing messages. Valid split messages with distinct keys and
protobuf default map members remain accepted.

Repeated singular messages are checked as they arrive, retaining only child
state and seen map keys. Regression tests show that one and 4,096 repeated
fragments use the same allocation count. Nested JSON also retains the existing
protobuf decoder's acceptance boundary.

## Corpus and verification

The fresh version 5 manifest contains 387 SQLite and 15 legacy conversations:
312 fully projected successes and 90 successes with the existing declared
source-only fields. All 402 conversations were explicit record-gate inputs.
Both the read-only format audit and `--record --corpus-swept` exited 0.
[COMPATIBILITY.md](../../COMPATIBILITY.md) records clean implementation commit
`cb502d2`, with all 402 conversations required. The temporary daemon was stopped
after capture; no installed service was restarted.

SQLite's schema fingerprint is unchanged. Sidecar shape gained exactly
17 paths and lost none, all under the newly exercised configuration subtree.
The [shape delta](agy-1.4.5-options-shape-delta.json) includes data-dependent map
keys from the two cases; this is added feature coverage within the same release,
not a release-specific format migration. The new union fingerprint is
`sha256:d81d128d431fcfb6797e7a1c390b2042d843c7c257b736ca6057bdfe21de96fa`.

`make vet lint test build` passed, with zero lint issues. The completeness race
test passed. Python maintenance checks ran 144 tests: 127 passed and 17 skipped;
all ten new generator tests passed. The optional AgentsView suite and live
downstream ingestion were not tested. No installed reader or watcher was changed.
