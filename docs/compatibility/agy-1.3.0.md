# agy 1.3.0 schema refresh

The reader's protobuf schema now comes from the agy 1.3.0 binary, so it no
longer refuses conversations written by agy 1.2.17 and 1.3.0 for their new
configuration field, `RunCommandToolConfig.enable_plugin_auth`
(`agy-reader#8mnx`). The completeness profile was
`sqlite-v1-format0-agy-1.3.0-19be6af38f7b-planner50-permission1` (the `permission1` suffix was added on the same day, before any
attestation was minted under the first form, when agy-reader#pzg7 tightened
the permission-record rule). The rule that refuses an
undefined persisted field is unchanged. A later suffix, `permission2`
(agy-reader#axnr) is the current profile and extends the same refusal to a native `VIEW_FILE` step whose
permission record has no copy on its wrapper; attestations minted under
`permission1` read as unknown until re-verified.

This is a schema refresh, not a full 1.3.0 format audit. `COMPATIBILITY.md`
still records 1.2.16, and `doctor` still reports the version mismatch. The
step shapes that the 1.2.17 audit found unsupported are separate issues.

## The binary

- `agy --version`: 1.3.0, installed at `~/.local/bin/agy` on 2026-10-05 at
  23:10 PDT.
- SHA-256 `19be6af38f7beeaa0db415df9297e314ab3d33fdd6f853434d49f88819bc68e4`.
  The extraction read a copy of the binary with the same hash, and the paired
  daemons used for verification ran the installed file with that hash.
- `scripts/extract_descriptors.py` found 322 descriptor files. All 322 match
  the bytes at their recorded offsets. (The first extraction reported 316;
  review finding rv-q2yj.1 showed the scanner could drop a record whose tail
  looked like one more field, and the six it had dropped are outside the
  trajectory closure, so the bundle did not change. The follow-up `rv-yct0.1`
  showed the opposite error: two records outside the closure had absorbed
  trailing text bytes as `public_dependency` entries. The scanner now stops at
  a field numbered below the previous one, which marshalled descriptors never
  produce, and accepts only known `syntax` values.) The trajectory closure is the same
  31 files as before. The bundle is 131,880 bytes with SHA-256
  `c52f0a2af19d559d0c0555ca56a29b3a6207452c1651a0648982f299333e36eb`.

The 1.2.17 binary was not kept. The comparison with 1.2.17 uses the bundle
that the 1.2.17 audit generated with the same extractor and generator.

## Field delta

From 1.2.17 to 1.3.0, three fields were added. Nothing was removed or
renumbered, and no enum value changed.

| Message | Field | Number | Type |
| --- | --- | ---: | --- |
| `exa.cortex_pb.CheckpointConfig` | `media_tokens_per_second` | 34 | optional int64 |
| `exa.cortex_pb.CheckpointConfig` | `use_model_reported_token_count` | 35 | optional bool |
| `exa.codeium_common_pb.CascadeNUXConfig` | `g1_redirect_url` | 21 | optional string |

1.3.0 also marks six existing fields deprecated:
`CascadeExecutorConfig` fields 5 and 6, `CheckpointConfig` fields 1, 2 and 13,
and `CortexStepCheckpoint` field 9. It reorders `CheckpointConfig`'s
declarations, which renumbers the synthetic oneofs of its optional fields.
Neither change affects the wire format or the JSON names.

From the previously embedded 1.2.16 bundle, the refresh adds those three fields
and the two that 1.2.17 added: `RunCommandToolConfig.enable_plugin_auth` (23,
optional bool) and `WorkflowSpec.depot_path` (19, string). The remaining
differences from that bundle are options only: explicit `json_name` values in
`google.api`, `google.protobuf.Any` and `google.rpc.Status`, and two more
deprecation marks. The 1.2.17 audit found the same option-only differences
between its extraction and the embedded bundle.

## Generator change

The 1.2.16 generator required exactly five annotation-only imports to be
missing. In 1.3.0, the trajectory closure no longer imports
`third_party/boundary_proxy/proto_compare/annotation.proto`, so only four are
missing. The 1.2.17 audit also deleted those five files from the extractor's
output by hand before generating. The generator now drops the five files when
the extractor finds them and accepts any subset of them as missing. Any other
missing import still stops it. Generating from the full extraction and from a
copy with the five files deleted gives byte-identical bundles.

## Verification

All checks ran on 2026-10-06, with a reader built from this change on base
commit `b186735`.

- **Frozen 1.2.17 conversations.** The audit's 16 frozen databases were
  copied, with their WAL files, to a private store under `tmp/`. A paired
  idle 1.3.0 agy started by `scripts/agy-pair.py` served the RPC.
  `audit-sweep` verified 11 of 16 with the new reader. The reader built from
  `b186735` without this change verified 0 of 16, each with `unknown
  persisted protobuf fields`.
  - The 7 that the 1.2.17-schema diagnostic build verified all verify:
    `2ee7ed05` (no tool calls), `fef08bde`, `a41ca2b8`, `1499237a`,
    `e7494d7b`, `ba5ad337` and `e456399f`.
  - 4 that the diagnostic build refused now also verify, because projection
    fixes landed on `main` after the 1.2.17 audit: `647710b9` (agy stopped
    mid-command), `ce7fd3a0` (interactive session with `ask_question`),
    `a5347748` (`/btw` side question) and `f198b2dc` (fork and rewind).
  - 5 still fail on their step shapes: `93684d00` and `5a205e6e` (`unsupported
    generic result`; hook-blocked commands), `5b41ec7b` and `76ff1287`
    (`unsupported generic wrapper`; permission request and invalid image
    request), and `abe64365` (`unsupported resultless view_file`). These are
    tracked separately (`agy-reader#pzg7`, `#azya`, `#2xqw`).
- **Fresh 1.3.0 conversation.** One headless run through `scripts/agy-pair.py
  run` with the prompt `Reply with the single word ok.` and no tool calls
  created conversation `4a233658`. On a private copy of it, the old reader
  refused it with `unknown persisted protobuf fields`. The new reader verified
  it through `--sync`, through a fresh watcher, and through `audit-sweep`. The
  sidecar carries `enablePluginAuth`.
- `make vet test` and `make maintenance-test` pass. The new Go tests check
  that the five added fields decode and that an undefined field is still
  refused. They also check that a source storing `enable_plugin_auth = false`
  in both metadata records verifies. All of these tests fail against the
  1.2.16 bundle.

## Not done

- A 1.3.0 changelog review, scenario runs, a corpus sweep of the live store,
  and a new `COMPATIBILITY.md` record.
- Live downstream ingestion.

The 1.2.17 audit report (`docs/compatibility/agy-1.2.17.md`) gives the
original diagnosis.
