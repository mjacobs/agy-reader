# Supported SQLite completeness profile

The reader only marks a SQLite sidecar verified when the persisted trajectory,
the full raw RPC, and the ordinary RPC's supported projection agree. This
prevents a reachable daemon with an old in-memory cache from certifying a
partial or same-step stale response.

## Supported representation

Profile `sqlite-v1-format0-agy-0d0d3eba22da` supports SQLite `user_version=1`,
the seven known tables and their exact columns, contiguous zero-based indices,
and `step_format=0`. All persisted protobuf fields and enum values must be
known. Decomposed subtrajectories and nonempty render blobs are unsupported.
The decoder checks redundant step columns against the complete step payload.
Unknown versions, fields, formats, tables, and columns return unknown; they
cannot create or refresh completeness evidence.

The full RPC uses `CLIENT_TRAJECTORY_VERBOSITY_FULL` and
`disableRehydration=true`. It must equal the reconstructed persisted protobuf.
The default response must equal its supported display projection: ordinary
steps pass through; generic wrappers containing recognized run-command,
code-action, or view-file native steps are replaced with the embedded step.
Wrapper fields and argument names are explicitly bounded in `projection.go`.
Wrapper result text, URI, args, and outer metadata are checked by the raw RPC,
but are intentionally absent from the ordinary sidecar when the daemon omits
them. Unknown RPC-only JSON fields remain in the saved payload and its digest.

The source digest includes the complete reconstructed protobuf and profile.
The payload digest includes all ordinary RPC fields except the reader-owned
`agyReader` namespace, with exact JSON numbers and canonical object order.
A matching existing attestation can be rechecked offline. Missing evidence is
unknown; changed source or payload is incomplete. Content can change after any
snapshot; the next watch/doctor check detects it without trusting mtime.

## Schema provenance and maintenance

`internal/completeness/schema.pb.gz` contains a transitive descriptor set for
`gemini_coder.Trajectory`, extracted from the agy 1.2.14 binary with SHA-256
`0d0d3eba22daf29504dd290151c7ed9a4d33b0c6aa0acfc5da27bc3b01d2f029`.
It contains schema definitions only, no conversation payloads or credentials.
The generator consumes a separate directory of `FileDescriptorProto` records
and a manifest declaring that hash; it removes unavailable annotation-only
imports and extension declarations, and writes deterministic gzip output.

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
