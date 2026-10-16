# Recovered retired agy protobuf fields

Both definitions were recovered from official agy CLI release binaries. They are
not inferred names. The same parent fields are declared in 1.0.4 and 1.0.10:

| Message | Field | Name | Message type |
| --- | ---: | --- | --- |
| exa.cortex_pb.CascadePlannerConfig | 50 | context_engine_config | exa.cortex_pb.ContextEngineConfig |
| exa.chat_pb.ChatMessagePrompt | 23 | prompt_struct | exa.codeium_common_pb.Struct |

Official sources:
- https://github.com/google-antigravity/antigravity-cli/releases/tag/1.0.4
- https://github.com/google-antigravity/antigravity-cli/releases/tag/1.0.10

The archived release metadata gives publication dates June 1 and June 18, 2026,
in America/Los_Angeles (UTC timestamps are retained in provenance.json).

## What the recovered fields contain

In 1.0.10, ContextEngineConfig declares:

```proto
message ContextEngineConfig {
  optional bool enable_prehook = 1;
  optional bool enable_tools = 2;
  optional int32 rpc_timeout_seconds = 3;
  optional bool enable_reflection_agent = 4;
  exa.codeium_common_pb.Model reflection_agent_model = 5;
}
```

The 1.0.4 definition contains only fields 1–3. The names/types/numbers of those
three fields agree. This is direct evidence of historical schema growth, not a
precise identification of the release that first added fields 4–5.

The older prompt_struct is a custom Struct, with repeated Field entries and a
Value oneof supporting null, number, string, bool, nested Struct, ListValue and
Media. It is not google.protobuf.Struct. The Struct, Field, Value and ListValue
message definitions are identical between these two extracted versions.

The newer 1.3.6 schema (not the pinned profile, which is agy-1.3.4) reserves
field 23 and uses field 25, with a genai.Struct type, for prompt_struct. A
matching JSON name is not enough to treat those wire formats as
interchangeable. Historical type definitions should be selected
explicitly or applied through a separately verified migration.

## Verification against older real trajectories

The four sessions from the earlier compatibility sample contained 25 occurrences
that the current schema could not interpret. With the 1.0.10 definitions:

- All 16 ContextEngineConfig payloads decode with no unknown fields. Three store
  enableTools=false; thirteen additionally store enableReflectionAgent=false.
  These are explicitly present optional fields, not inferred defaults.
- All nine old prompt_struct payloads decode as exa.codeium_common_pb.Struct,
  each with one top-level Field entry and no unknown fields anywhere in the
  recovered message. Private content is not printed or saved in this report.
- Original DB/WAL/sidecar hashes remained unchanged. SQLite opened only private
  copies. No agy daemon calls and no execution of downloaded binaries occurred.

This establishes authentic definitions and their fit to the observed bytes. It
does not identify each session's exact writer binary or prove every historical
trajectory can be decoded by one version. A field named enable_reflection_agent
shows a configuration capability; it does not establish how reflection works or
that it was enabled. In the sampled records where it was present, it was false.

## Retained evidence

- provenance.json: official asset URLs/digests, binary hashes, extraction counts,
  and the two recovered parent field declarations.
- releases.json: GitHub release API response used to locate the assets.
- 1.0.4-extracted/: 257 raw descriptors and manifest.
- 1.0.10-extracted/: 271 raw descriptors and manifest.
- recovered-definitions.json: field/message excerpts and binary offset/hash anchors.
- 1.0.10-all.protoset: complete extracted inventory (not a claim of complete import closure).
- 1.0.10-trajectory.protoset[.gz]: a load-verified 26-file trajectory closure.
- 1.0.10-proto/: readable reconstructed definitions from that closure.
- recovered-payload-validation.json and validate-recovered.py: the real-data check.
- generate-historical-schema.py: snapshot-only normalization, with the historical
  binary hash and trajectory root. The newer GenerateImage tools root does not
  exist in this release and was not added artificially.

Both downloaded archives matched their SHA-256 values published by the GitHub
API. All 528 extracted descriptor records were checked against their hashes and
exact byte ranges in the corresponding binaries. The original raw records remain
unmodified; the decoding closure follows the existing generator's documented
annotation/extension normalization. No reader runtime qualification was changed.

These are extractor/protobuf observations, not REA decompilation Evidence IDs.
No native decompiler was needed. Downloads were held in /tmp; the extracted
schemas, provenance and report are in the durable workspace. They were
committed in 633967e.
