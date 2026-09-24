# agy 1.2.5 compatibility baseline

The 2026-09-17 audit refreshed 229 conversation snapshots through the agy 1.2.5
daemon. All 229 decoded and rendered without errors. The SQLite schema is
unchanged. This broad corpus replaces the four-trace spot check used during the
authentication investigation.

The reader code tested was `7505cb4`. The generated compatibility record is in
[`COMPATIBILITY.md`](../../COMPATIBILITY.md). The corresponding
[canonical field/type snapshot](agy-1.2.5-sidecar-shape.tsv) makes future shape
changes inspectable without access to private transcripts.

## Coverage

| Measure | Result |
| --- | ---: |
| Conversations selected at the start of the sweep | 229 |
| Fresh daemon responses | 229 |
| Fetch or render errors | 0 |
| Populated / empty trajectories | 209 / 20 |
| Steps | 21,620 |
| Distinct step types | 21 |
| Completed / canceled / error steps | 21,461 / 62 / 97 |
| Canonical field/type entries | 1,629 |

Each response was fetched through `LoadTrajectory` and `GetCascadeTrajectory`,
using the internal cascade ID fallback where needed. No cached sidecar fallback
was allowed. The raw responses were written to a new private corpus directory;
the sweep did not overwrite the original sidecars. Every selected response was
serialized by 1.2.5, so the record uses `explicit-version-scoped`.

The snapshot is an inventory taken at the start of the sweep, not a claim that
every later conversation has been audited. The private corpus retains its
session manifest and coverage report for local reproduction. Neither raw
transcripts nor session IDs are committed.

| Step type (`CORTEX_STEP_TYPE_` prefix omitted) | Count |
| --- | ---: |
| ASK_QUESTION | 26 |
| CHECKPOINT | 197 |
| CODE_ACTION | 1,118 |
| COMMAND_STATUS | 1 |
| CONVERSATION_HISTORY | 159 |
| EPHEMERAL_MESSAGE | 5 |
| ERROR_MESSAGE | 105 |
| FIND | 60 |
| GENERATE_IMAGE | 4 |
| GENERIC | 828 |
| GREP_SEARCH | 578 |
| INVOKE_SUBAGENT | 60 |
| LIST_DIRECTORY | 386 |
| MCP_TOOL | 12 |
| PLANNER_RESPONSE | 10,517 |
| READ_URL_CONTENT | 5 |
| RUN_COMMAND | 3,754 |
| SEARCH_WEB | 107 |
| SYSTEM_MESSAGE | 631 |
| USER_INPUT | 342 |
| VIEW_FILE | 2,725 |

## Fingerprints and interpretation

- Schema: `sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`
- Sidecar shape: `sha256:70e2c9386e88420d834beab7a4c917514bcf7b2cf9f74674bde02965601e0462`

The sidecar hash differs from the recorded 1.2.4 value
(`sha256:da255ec27ccc0b9366bb803dcaca0b832d15de1ce1b2fce52071de553529332e`).
However, the stored local sidecars immediately before this sweep produced the
same canonical field list as the freshly serialized corpus: zero added or
removed entries. Corpus growth or optional-feature coverage may explain the
historical difference, but the old record did not retain its field list or
coverage counts. A release-induced change cannot be ruled out retrospectively.
This snapshot supplies that missing evidence for future comparisons.

The 1.2.5 changelog declares no storage, encryption, or RPC structure changes.
Its canceled-command correction affects recorded status semantics; the audited
corpus includes canceled command steps that decode and render successfully.
Background task naming, subagent guidance, sign-in handling, and terminal UI
changes do not declare a serialization change.

The reader unit suite and format audit passed. Agentsview integration tests were
not run. Successful Markdown rendering is a smoke check, not proof that every
optional payload field is displayed. The fingerprint excludes reader-owned
metadata and opaque configuration subtrees, as specified in
[`internal/shapefp`](../../internal/shapefp/shapefp.go).

## Comparing a future release

An empty newest trace is a valid storage sample but a poor baseline for rendered
content. Reuse the private session manifest when available, or select a broad
corpus with comparable step-type coverage. Fetch every selected trajectory from
the version under audit into a new private directory. Do not mix fresh responses
with old sidecars or treat a cache fallback as a successful refresh.

After refreshing the corpus, generate its field list and compare it with this
snapshot:

```sh
export AGY_SIDECAR_CORPUS=/path/to/fresh-corpus
go run . shape-fingerprint --paths "$AGY_SIDECAR_CORPUS" > new-sidecar-shape.tsv
diff -u docs/compatibility/agy-1.2.5-sidecar-shape.tsv new-sidecar-shape.tsv
skills/agy-format-audit/scripts/audit_format.sh --corpus-swept
```

Inspect added, removed, and type-changed paths against the changelog and corpus
coverage. A path absent because its feature was not exercised is not evidence
that the daemon removed it. Once reviewed, record the new version:

```sh
skills/agy-format-audit/scripts/audit_format.sh --record --corpus-swept
```

Retain the new version's field list and coverage counts alongside its report.
The TSV has one final newline for text-tool convenience; the fingerprint hashes
the UTF-8 bytes without that newline. Its rows contain normalized field names
and JSON types only. Scan any new snapshot for private data in unexpected map
keys before committing it.

This audit used an isolated network namespace with outbound connectivity and
all inbound port forwarding disabled. The daemon was unreachable from host
loopback and was stopped after the sweep. Authentication and installed-watcher
health remain separate from this format baseline; passing this audit does not
resolve a missing credential or make command-line tokens safe on shared hosts.
