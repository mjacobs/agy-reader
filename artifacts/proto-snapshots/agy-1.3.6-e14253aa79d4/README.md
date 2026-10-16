# agy 1.3.6 protobuf snapshot

This snapshot permits offline decoding of the protobuf messages used by supported
trajectory storage. It contains schema definitions, not conversations. It does
not require agy-reader or an agy daemon at decode time. The consumer still needs
a protobuf runtime and the storage-layout rules that assemble SQLite rows into a
trajectory.

Captured on 2026-10-15, America/Los_Angeles. The installed `agy --version` returned
`1.3.6`; extraction used a private copy of the installed binary, with SHA-256:

```
e14253aa79d4e910cbf8bcb40b68e80f9e85476725e130ed085122c98face92b
```

The earlier 1.3.5 snapshot was lost from /tmp. This is a new 1.3.6 snapshot,
not a reconstruction of the lost evidence. These files live in the project
workspace and survive ordinary /tmp cleanup. They are not yet committed or backed up.

## Files

- `trajectory.protoset`: one binary FileDescriptorSet containing the 34-file
  trajectory import closure, plus the tools root needed for GenerateImage Any
  values. Use this for dynamic decoding; generated language bindings are optional.
- `trajectory.protoset.gz`: the same descriptor set, compressed deterministically.
- `proto/`: 34 readable reconstructed `.proto` files, with original package and
  import names. Start at `proto/third_party/gemini_coder/proto/trajectory.proto`.
  Multiple packages require separate files; the single-file consumer artifact
  is `trajectory.protoset`.
- `extracted/`: all 370 original serialized FileDescriptorProto records and a
  manifest with binary offsets, lengths, and individual SHA-256 values.
- `all-extracted.protoset` and `all-extracted.textproto`: the full extracted
  inventory in binary and readable descriptor notation. This inventory is not
  asserted to be a complete, loadable dependency closure for every agy subsystem.
- `roundtrip.protoset`: compiled from the reconstructed `.proto` files.
- `verification.json`: actual verification counts and a synthetic offline decode.
- `tools/`: extraction, normalization, printing, and verification sources.
- `SHA256SUMS`: artifact integrity checksums, excluding this checksum file itself.

The original records are the preservation copy. Normalization follows the reader
at commit `538e8dd7f06695e8ae99661137b91837e7bac88e`: it removes five named
annotation-only dependencies, extension declarations, unknown fields/options,
and public/weak import markers. Known options remain. The readable files are
printed from this normalized closure, not recovered original source: original
comments and formatting are unavailable. The snapshot-only generator pins the
1.3.6 hash; the production reader schema and qualification profile were not changed.

## Verification and reproduction

All 370 original records were compared with their byte ranges in the copied binary
and their hashes before saving. The 34-file normalized set loads in a descriptor
pool. All 34 reconstructed files compile with protoc 35.1. Recompiled descriptors
match after ignoring empty options presence and nested-message declaration order;
no other differences are ignored by `tools/verify.py`.

Verification used Python protobuf 7.36.2, grpcio-tools 1.84.0 (protoc 35.1), and
jhump/protoreflect 1.19.0 with Go protobuf 1.36.12. Go dependencies are pinned in
`tools/proto-printer/go.mod` and `go.sum`. Some input files use Editions 2024;
this snapshot cannot be compiled with the system's older protoc 3.19.6.

From this directory, with those tools available:

```sh
# Re-extract from the exact binary; output directory must not already exist.
python3 tools/extract_descriptors.py /path/to/matching-agy fresh-extracted
python3 tools/generate_schema.py fresh-extracted fresh-trajectory.protoset.gz

# Reprint the already saved normalized descriptor set.
(cd tools/proto-printer && go build -buildvcs=false -o /tmp/print-agy-protos .)
/tmp/print-agy-protos trajectory.protoset regenerated-proto

# Verify saved records, roundtrip descriptor equivalence, and offline decoding.
python3 tools/verify.py
sha256sum -c SHA256SUMS
```

`verification.json` records a synthetic message test, not a real-session SQLite
export or proof of compatibility with every 1.3.6 trajectory.

## Historical decoding model

Keep an immutable snapshot per binary hash, with its version, extraction method,
normalization rules, tool versions, and qualification evidence. Record the
producer binary hash alongside newly captured conversations where possible.
The binary installed when a file is read is not necessarily its writer. Resumed
sessions may span versions; use per-record provenance when available and retain
original bytes and unknown fields instead of guessing a historical schema.

A consumer can load the selected descriptor set, read SQLite in a consistent
read transaction including committed WAL content, decode each protobuf column,
and assemble the trajectory. The existing reference is
`internal/completeness/sqlite.go`: `steps`, generator/executor metadata, parent
references, battle-mode information, and the main metadata blob are separate
records. A `.db` file is not one serialized Trajectory message.

Schema snapshots do not supply legacy AES-GCM decryption keys, explain reserved
historical fields, reconstruct unsupported decomposed subtrajectories, or reproduce
all daemon rehydration and presentation rules (such as skill-file classification).
A wire decode can succeed while losing semantic context or retaining unknown
fields. Keep raw decoding, storage assembly, and presentation qualification separate.
