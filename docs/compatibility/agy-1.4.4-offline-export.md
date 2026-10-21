# agy 1.4.4 direct SQLite export

The `export-offline` command reconstructs JSON and Markdown without an agy
process, reader service, proxy or existing sidecar. It reads a supported SQLite
session in a read-only transaction, including committed WAL records, and uses
the reader's qualified protobuf presentation mappings.

## Comparison performed on 2026-10-21

The independent baseline was the agy 1.4.4 audit sweep captured earlier that day
(17:12:39–17:14:06 UTC), not JSON produced by the new exporter. Its private archive
was recovered after the host's power interruption and checked against its saved
SHA-256: `8000316acf49c125a85c6bb952dce67a7c64bbb64a67f187a6c1a3eb2db6c275`.

All **381 SQLite sources**, totaling **33,895 steps**, matched the profiles and
native source digests in that manifest. All 381 JSON exports matched the saved
daemon trajectories after removing reader provenance and making omitted empty
`steps` arrays explicit (35 sessions). The comparison preserves JSON numbers;
it does not discard unknown fields or compare only typed transcript fields.

All 381 Markdown exports matched Markdown rendered from the saved daemon JSON,
excluding the generation timestamp and the offline notice. Both renderings used
only the selected session, without expanding children from adjacent sidecars.
The sources included 75 sessions with declared source-only fields; offline JSON
retains their raw payloads separately under `agyReader.offline.sourceOnlyFields`.
The baseline's 15 legacy encrypted protobuf conversations were outside this
SQLite command's scope.

## Checks and limits

`make check` passed: formatting, vet, lint, Go tests, Python maintenance tests
(134 run, 67 skipped), and build. Focused race tests covered `internal/cli` and
`internal/completeness`. Regression tests exercise committed WAL data, no daemon
requests, ignored adjacent sidecars, empty sessions, qualified generic tool
projections, source-only byte preservation, unsupported inputs, cancellation,
write failures, and refusal to replace existing files or links.

Reproduce the corpus comparison with `TestExportOfflineCorpus` and the explicit
private paths documented in [completeness](../completeness.md#direct-offline-export).
Private responses and run logs are retained under the ignored local
`tmp/offline-export/` directory; no conversation content is committed.

This comparison establishes parity for the retained SQLite corpus and qualified
profile. It is not a new completeness attestation for arbitrary future exports.
The JSON namespace and Markdown notice identify offline reconstruction. Unknown
schemas, fields or projections remain errors. Source databases remain the
backup format; exported presentations do not retain every generic wrapper.
