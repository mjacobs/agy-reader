# Supported SQLite completeness profile

The reader only marks a SQLite sidecar verified when the persisted trajectory,
the full raw RPC, and the ordinary RPC's supported projection agree. This
prevents a reachable daemon with an old in-memory cache from certifying a
partial or same-step stale response.

## Supported representation

Profile `sqlite-v1-format0-agy-a759ce7c7a23` supports SQLite `user_version=1`,
the seven known tables and their exact columns, contiguous zero-based indices,
and `step_format=0`. All persisted protobuf fields and enum values must be
known. Decomposed subtrajectories remain unsupported. The decoder checks
redundant step columns against the complete step payload. In particular,
`steps.render_info` must equal `generic.result.step_render_info`, including
presence (SQL NULL means absent); malformed and unknown content is rejected.
Unknown versions, fields, formats, tables, and columns return unknown; they
cannot create or refresh completeness evidence.

The full RPC uses `CLIENT_TRAJECTORY_VERBOSITY_FULL` and
`disableRehydration=true`. It must equal the reconstructed persisted protobuf.
The default response must equal its supported display projection:

- Ordinary steps pass through unchanged.
- Generic wrappers containing recognized run-command, code-action, view-file,
  or invoke-subagent native Steps are replaced with the embedded Step. Outer
  task details are preserved; conflicting or inner-only task details fail.
  Wrapper and native status must match, with one exception: a run-command
  wrapper `CANCELED` over a native `ERROR` that carries error details is
  presented with the outer `CANCELED` status. A canceled wrapper over any
  other native status still fails.
- A view-file `ContentOffset` argument is accepted only when it is a canonical
  decimal and the presented native Step carries the same value, both in its
  tool-call arguments and in `view_file.content_offset`. The ordinary response
  therefore still shows the offset that the dropped argument recorded.
- Payload-free `manage_task` and `send_message` steps remain generic, with
  bounded arguments and result fields. Task metadata keys are limited to the
  observed `task_description`, `task_id`, and `tool_name`. A failed
  `manage_task` step with status `ERROR` may also carry an outer `error`, which
  the ordinary response presents unchanged. An outer error on any other
  generic wrapper fails.
- The `generate_image` tool's `exa.cortex.core.tools.GenerateImage` Any becomes
  a native image step. It preserves outer metadata and maps prompt, image name,
  aspect ratio, model name, and the single returned media item. The media URI
  must agree with the payload's image URI. Unqualified payload fields, missing
  or multiple media items, and other Any types fail closed.

Arguments and wrapper/result fields are bounded separately for each projection.
Wrapper result text, media, output URI, arguments, and metadata omitted by the
ordinary RPC still have to match the full raw RPC. Image byte count is also
checked by raw equality even though the ordinary image step omits it. Unknown
RPC-only JSON fields remain in the saved payload and its digest.

The source digest includes the complete reconstructed protobuf and profile.
The payload digest includes all ordinary RPC fields except the reader-owned
`agyReader` namespace, with exact JSON numbers and canonical object order.
A matching existing attestation can be rechecked offline. Missing evidence is
unknown; changed source or payload is incomplete. Content can change after any
snapshot; the next watch/doctor check detects it without trusting mtime.

## Schema provenance and maintenance

`internal/completeness/schema.pb.gz` contains a transitive descriptor set for
`gemini_coder.Trajectory` plus `third_party/jetski/cortex/core/tools_pb/tools.proto`,
extracted from the agy 1.2.16 binary with SHA-256
`a759ce7c7a235d9b6c281a25ead97cbbf2e92314a3ffd224e2f9144f3fae7a86`.
The second root supplies `exa.cortex.core.tools.GenerateImage`, which travels
inside an Any and is absent from the trajectory's ordinary import closure.
The closure contains 31 descriptors (131,731 compressed bytes), with SHA-256
`31b70a3469613a9f66ee5d3767633ffe8b68372a5899485354744f10708e165c`.
All 245 extracted records were checked against bytes at their manifest offsets
in that binary before generation. The bundle contains schema definitions only,
no conversation payloads or credentials.
The generator consumes a separate directory of `FileDescriptorProto` records
and a manifest declaring that hash and the hash of each extracted record.
It verifies those hashes, removes unavailable annotation-only
imports and extension declarations, and writes deterministic gzip output.
The descriptor delta from 1.2.14 adds `AgentConfigResolution.contrib_variant_metadata`
(message `.exa.cortex_pb.ContribVariantMetadata`), additive `.exa.codeium_common_pb.Media`
fields (`start_page`, `end_page`, `start_time_ms`, `end_time_ms`, `fps`, and
`media_resolution`), and `.exa.cortex_pb.ViewFileToolConfig.enable_open_document`.
These schema definitions are extracted directly from the binary descriptors;
not all additive fields are exercised by the native audit scenarios. The
1.2.16 profile also includes the extracted image-agent and plugin metadata
schema additions. Descriptor availability alone does not qualify new generic
tool projections.

The generator is a maintenance aid, not a runtime dependency. It needs Python
and `google.protobuf`; ordinary builds need Go 1.25 or later. Runtime decoding
uses the pinned Go protobuf and pure-Go SQLite modules in `go.mod`, without
Python or a SQLite CLI. Updating descriptors alone does not authorize a new
projection: qualify the source format and both RPC representations, extend
synthetic rejection tests, and update the profile identifier before accepting
new content.

The normal test suite uses synthetic SQLite and HTTP fixtures. The opt-in
`TestNativeEvidence` accepts explicitly supplied disposable DB/raw/default RPC
files through `AGY_READER_SCRATCH_DB`, `AGY_READER_SCRATCH_RAW`, and
`AGY_READER_SCRATCH_PRESENTATION`; it does not discover live profiles or daemons.
