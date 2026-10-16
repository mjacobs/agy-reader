---
name: agy-proto-snapshot
description: Save, verify and compare the protobuf schema embedded in an agy binary as a durable, hash-named snapshot under artifacts/proto-snapshots/. Use when a new agy release may have changed its protobuf messages, when historical sessions need the schema of an older release, or when asked to extract, record or diff agy protobuf definitions. Snapshot maintenance only; production compatibility is the agy-format-audit skill.
---

# agy protobuf snapshots

An agy binary carries the protobuf definitions of the messages it stores in
conversation databases. This skill saves those definitions for one exact binary
as a snapshot directory, checks that the saved files really describe that
binary, and compares two snapshots so you can see what a release changed.

A snapshot is evidence about a schema. It does not change what agy-reader
accepts. Recording a snapshot does **not** update `COMPATIBILITY.md`, the
reader's embedded schema (`internal/completeness/schema.pb.gz` and
`schema.go`), its verification profile, or qualify any reader and daemon
pairing. Those decisions belong to the sibling skill
[`agy-format-audit`](../agy-format-audit/SKILL.md) and the completeness
workflow in [`docs/completeness.md`](../../docs/completeness.md), "Schema
provenance and maintenance".

> **Source & install.** The tracked source is `skills/agy-proto-snapshot/`.
> `make install-skills` links it into the gitignored `.claude/skills/` and
> `.agents/skills/` directories of this checkout. The tool is
> `scripts/agy_proto_snapshot.py`; its tests are
> `scripts/test_agy_proto_snapshot.py`. No agent harness is needed to run it.

## Requirements

Check what is installed before relying on a version:

```sh
python3 -c 'import google.protobuf as p; print(p.__version__)'
python3 -m grpc_tools.protoc --version     # or: protoc --version
go version
```

| Tool | Needed for | Minimum |
| --- | --- | --- |
| Python protobuf (`google.protobuf`) | every command | 6.32 for Edition 2024 files, 5.27 for Edition 2023 |
| protoc, from `grpcio-tools` by default or `--protoc PATH` | `snapshot` (compiles the printed files back) | 32.0 for Edition 2024, 27.0 for Edition 2023 |
| Go, to build `scripts/proto-printer` (or a prebuilt one via `--printer`) | `snapshot` (prints readable `.proto` files) | the Go version in `scripts/proto-printer/go.mod` |

The 1.0.10, 1.3.6 and 1.4.0 runs used Python protobuf 7.36.2, grpcio-tools
1.84.0 (protoc 35.1), jhump/protoreflect 1.19.0 and Go protobuf 1.36.12 (the
last two are pinned in `scripts/proto-printer/go.mod`). Current agy closures
contain Edition 2023 and 2024 files, so an old system protoc such as 3.19.6 is
refused rather than used. An edition the tool has no entry for (for example
`EDITION_2026`) is refused until someone confirms the tools handle it and adds
it to `EDITION_MINIMUMS` in the script.

A private environment that does not touch the repository:

```sh
venv="${TMPDIR:-/tmp}/agy-proto-venv"
uv venv --python 3.12 "$venv"
uv pip install --python "$venv/bin/python" protobuf==7.36.2 grpcio-tools==1.84.0
```

## Workflow

Run everything from the repository root. `$py` below is a Python with the
tools above, such as `"$venv/bin/python"`.

### 1. Pick the binary and its evidence

- **Installed release:** note what `agy --version` prints and pass the binary
  path. The tool copies it first, hashes the source before and after the copy,
  and refuses if it changed meanwhile.
- **Older release:** download the official asset from
  `https://github.com/google-antigravity/antigravity-cli/releases` and get its
  published digest (`gh api repos/google-antigravity/antigravity-cli/releases/tags/<tag>`
  lists each asset's `digest`). Pass the archive with `--archive`,
  `--archive-sha256` and `--archive-member` (`antigravity` in the 1.0.x
  linux archives). The tool checks the digest before reading the member.

The tool never runs the binary. `--version` is the release label and
`--version-evidence` says how you know it; both are recorded. Add
`--expected-sha256` whenever you already know the binary hash: a mismatch is
refused before anything is published.

### 2. Choose the roots

`--root` names the descriptor files whose import closure a decoder needs. It
is required and never guessed. Releases since 1.3.x use two roots; the second
supplies `GenerateImage`, which travels inside an `Any` and is outside the
trajectory's ordinary imports:

```
--root third_party/gemini_coder/proto/trajectory.proto
--root third_party/jetski/cortex/core/tools_pb/tools.proto
```

The 1.0.x releases have no `tools.proto`; use the first root alone. A root the
binary lacks is refused, not invented.

Five annotation-only imports (the set used by
`internal/completeness/generate_schema.py`) are dropped from the closure and may
be absent from the binary. Any other missing import is refused.
`--annotation-only` replaces that set; record why if you change it.

### 3. Take the snapshot

```sh
"$py" scripts/agy_proto_snapshot.py snapshot \
  --binary "$(command -v agy)" --version 1.4.0 \
  --version-evidence 'agy --version printed 1.4.0 on 2026-10-16' \
  --root third_party/gemini_coder/proto/trajectory.proto \
  --root third_party/jetski/cortex/core/tools_pb/tools.proto
```

From an official archive instead:

```sh
"$py" scripts/agy_proto_snapshot.py snapshot \
  --archive agy_cli_linux_x64.tar.gz --archive-member antigravity \
  --archive-sha256 6547cf9a37227f26004fa4b805418b1df96f54c57b9723ca7d10864d2610bb0f \
  --expected-sha256 3c9d88067e3ab6e5c59139ccb4fd7e8650aa39264e2548fc99fe2f700a271f96 \
  --version 1.0.10 --version-evidence 'official release tag 1.0.10, linux x64 asset' \
  --source-url https://github.com/google-antigravity/antigravity-cli/releases/download/1.0.10/agy_cli_linux_x64.tar.gz \
  --root third_party/gemini_coder/proto/trajectory.proto
```

It writes `artifacts/proto-snapshots/agy-<version>-<first 12 hex of the binary
SHA-256>/` (`--out-root` picks another parent). It refuses an existing
directory. All work happens in a hidden `.incomplete-*` directory that is
renamed into place only after every check passes; a failed run deletes it, so
a published `snapshot.json` always says `"status": "verified"`.

| Path | Contents |
| --- | --- |
| `extracted/NNNN.pb`, `extracted/manifest.json` | every embedded record exactly as found, with its offset, length and SHA-256; each was compared with the binary's bytes at that range |
| `all-extracted.protoset` | the same record bytes as one FileDescriptorSet, unmodified; not a claim that every subsystem's imports are present |
| `closure.protoset`, `closure.protoset.gz` | the normalized import closure of the roots, loaded in a descriptor pool before saving. Use this for decoding |
| `proto/` | readable `.proto` files printed from the closure (original comments and layout are not in the binary). Type references are as short as possible; if that makes one ambiguous (a nested message whose name matches an outer one), so that the files do not compile back to the closure, they are printed again fully qualified and `roundtrip.readable_type_names` in `snapshot.json` says so |
| `roundtrip.protoset` | protoc's compilation of `proto/`, which must match the closure |
| `snapshot.json` | version evidence, binary hash and size, provenance, tool revision and versions, roots, stripped imports, unresolved inventory imports, edition counts and check results |
| `SHA256SUMS` | checksums of every other file |

Normalization of the closure, and nothing else: drop the annotation-only
imports, discard unknown fields, remove extension declarations, remove
imports outside the closure (public and weak markers stay on the imports that
remain), and remove Edition 2024 option imports, which only supply options.
The production generator also drops public and weak markers; no agy
trajectory closure so far has any, or any option import. The raw records
are never normalized. The roundtrip check ignores only the presence of empty
options messages and the declaration order of nested messages. Compare the
decompressed `.gz` content, not its bytes: compressed output differs between
zlib builds.

### 4. Verify

```sh
"$py" scripts/agy_proto_snapshot.py verify artifacts/proto-snapshots/agy-<version>-<hash>
"$py" scripts/agy_proto_snapshot.py verify <snapshot> --binary <the same binary>
```

`verify` rechecks checksums, record hashes, the raw inventory, a rebuild of the
closure from the records, pool loading and the saved roundtrip, offline.
Rechecking each record against its byte range needs the original binary
(`--binary`, refused unless its hash matches). Say which checks you could not
rerun when the binary is gone.

### 5. Compare against an explicitly chosen snapshot

```sh
delta="$(mktemp -d)/delta.json"
"$py" scripts/agy_proto_snapshot.py compare \
  artifacts/proto-snapshots/agy-1.3.6-e14253aa79d4/trajectory.protoset \
  artifacts/proto-snapshots/agy-<version>-<hash> --json "$delta"
```

Each side is a snapshot directory made by this tool or a `.protoset[.gz]`
file. Directories that predate the tool (`agy-1.3.6-e14253aa79d4/`,
`artifacts/historical-schema-recovery/`) must be named by file:
`trajectory.protoset` and `1.0.10-trajectory.protoset`. Exit status is 0 when
identical, 1 when different, 2 on a refusal. A refusal prints one `refused:`
line on stderr and no report: an input that is missing, unreadable or not a
descriptor set, or a `--json` path that already exists. The `--json` check runs
before comparing, so a rerun refuses at once and keeps the earlier file; the
`mktemp -d` line above gives each run a new path.

Those older directories were extracted before the extractor accepted
`option_dependency` (Edition 2024 option imports, field 15). Their raw
records for files that declare option imports stop just before that field:
30 of 370 records for 1.3.6 and 26 of 271 for 1.0.10, none of them in the
trajectory closures, which are unaffected. A fresh snapshot of the same
binary saves those records whole, at the same offsets. In the other
direction, the 1.3.6 record for
`experiments/framework/extensions/heterodyne/proto/experiment_ids.proto`
includes two trailing bytes (`50 52`, a public import marker pointing past its
two imports) that a fresh snapshot leaves out, saving 485 bytes instead of 487.

Changes are grouped by what they affect:

- **Wire:** messages, enums, fields, extensions and enum values added or
  removed; field type as resolved for its edition (shown with its wire
  encoding, so a message field switched to delimited encoding reads as a
  group) and resolved target message or enum, cardinality (singular,
  repeated, map, required), presence, packing, UTF-8 validation of strings,
  oneof membership (which fields exclude each other, by number), enum
  open/closed and MessageSet encoding; services and RPC methods added or
  removed, and method input, output or streaming changes. Additions are
  additive; removals and type changes need a decision before decoding old
  bytes with a new schema.
- **JSON-facing:** field names, JSON names, enum value names, an extension
  moved to another scope (its JSON key is its full name), and the value a
  reader sees for an absent field (including a proto2 enum default that moves
  because the enum's first value changed). The bytes are unchanged, but decoded
  values, sidecars and JSON output change.
- **Descriptor-only:** files added or removed, packages, imports,
  syntax/edition, public, weak and option imports, the file that declares a
  message, enum, extension or service, Edition 2024 visibility, reserved
  numbers and names, extension ranges and their options, oneof names,
  explicit defaults that match the implicit one, and options, including oneof,
  enum value, service and method options (a newly deprecated value shows here).
  Custom options the runtime cannot name are compared by a digest of their
  bytes; closures made by this tool have none, because their normalization
  discards unknown fields. An edition change that alters behavior also shows
  up as a wire change such as presence.

Names are not behavior. A field called `enable_reflection_agent` shows that a
setting exists, not what Gemini does with it. Same-named fields of different
types are not interchangeable: in 1.0.x `ChatMessagePrompt` field 23
`prompt_struct` is the custom `exa.codeium_common_pb.Struct`, while 1.3.6
reserves 23 and uses field 25 of type `genai.Struct`, and neither is
`google.protobuf.Struct`. Likewise `CascadePlannerConfig` field 50
`context_engine_config` (`exa.cortex_pb.ContextEngineConfig`) exists in 1.0.x
and is gone by 1.3.6. Authentic recovered names do not justify relaxing the
reader's source-only validation or attestation rules.

### 6. Record and hand off

Commit the new snapshot directory on its own. Then hand the delta to the
right owner:

- Whether the reader is compatible with the release: run the
  [`agy-format-audit`](../agy-format-audit/SKILL.md) skill. Only its
  `--record` step writes `COMPATIBILITY.md`.
- Whether the reader's embedded schema or verification profile should move:
  follow `docs/completeness.md`, "Schema provenance and maintenance". The
  production generator pins its own binary hash; this tool never edits it.

## Refusals

Every refusal exits 2, prints `refused: <reason>` and publishes nothing.

| Situation | Message starts with |
| --- | --- |
| destination already exists | `refusing to overwrite existing snapshot` |
| binary or archive hash differs from the expected one | `binary hash mismatch` / `archive hash mismatch` |
| binary changed while being copied | `binary changed while it was being copied` |
| a root is not in the binary | `root not found in this binary` |
| an import outside the annotation-only set is missing | `unexpected missing imports in the closure` |
| an edition the toolchain cannot handle, or an unlisted edition | `unsupported toolchain or edition` |
| Python protobuf, protoc, Go or the printer is missing | `missing dependency` |
| printer, protoc or the roundtrip comparison fails | `printing .proto files failed`, `compiling the printed .proto files failed`, `roundtrip differs beyond the documented normalization` |

## Known limits

- The printer (protoreflect 1.19.0) does not write Edition 2024 `export` or
  `local` markers on messages and enums, so a closure that uses them fails
  the roundtrip check and is refused rather than saved wrong. No agy closure
  so far uses them.
- Snapshots record the tool revision that made them; `verify` rebuilds the
  closure with the current normalization, so a snapshot made by an older
  revision of this tool can fail `verify` if the normalization changed since.

## Checking real sessions (optional)

A snapshot proves the definitions came from the binary. It does not prove a
real conversation decodes without loss. If you test decoding real sessions,
keep it offline: copy the `.db` and `-wal` files to a private directory, hash
the originals before and after, make no daemon calls and never write to the
originals. Look for unknown fields recursively, including inside `Any`
values; JSON equality can hide dropped unknown fields. Keep prompt and tool
content out of anything committed. Earlier samples did not establish that
reading a live WAL while agy writes it is safe.
