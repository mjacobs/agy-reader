# Auditing an agy upgrade

An audit checks whether the reader can still load and render conversations
and whether the storage schema or daemon payload structure changed. It does
not require an agent harness. The tracked helper and commands below are the
same tools used by the `agy-format-audit` skill.

## Prerequisites

Use a Git checkout of this repository, Go 1.24+, Bash, Python 3, and the
`sqlite3` command-line tool. Python uses only its standard library. `jq` is
optional and improves the latest-sidecar step-count display. The shell helper
uses Python for timestamp selection and SHA-256 hashing, so GNU `find` and
`sha256sum` are not required on macOS. These audit tools are additional to the
reader's runtime; `go install` installs the binary, not these scripts or docs.

Start agy and ensure the reader can authenticate. See the
[CSRF instructions](../README.md#watch-mode). On Linux, an explicit agy launch
token allows discovery even while the daemon is idle. On other platforms,
supply the matching `ANTIGRAVITY_DAEMON_URL` and `ANTIGRAVITY_CSRF_TOKEN` as
needed. Neither a token-refresh service nor tmux is required.

```sh
go build -o bin/agy-reader .
bin/agy-reader --root "$HOME/.gemini/antigravity-cli" doctor
```

A version-skew warning is expected before recording the upgrade. Fix rejected
authentication or an unreachable daemon before collecting a corpus.

## Collect a fresh corpus

Verify that the **serving daemon** runs the agy version being audited. Updating
the binary on disk does not update an already-running agy process. Restart the
session after an upgrade; when multiple sessions are open, confirm the endpoint
shown by `doctor` or pin it explicitly. `agy --version` alone describes the
installed executable, not necessarily the process serving RPCs.

```sh
work=$(mktemp -d "${TMPDIR:-/tmp}/agy-reader-audit.XXXXXX")
bin/agy-reader audit-sweep \
  --root "$HOME/.gemini/antigravity-cli" \
  --out "$work/sweep" --timeout 30s
```

The sweep selects every syncable conversation in one store, including SQLite
and legacy protobuf sessions, but excludes unsupported `implicit/` traces.
It calls `LoadTrajectory` and `GetCascadeTrajectory`, uses the internal
cascade-ID fallback where needed, and renders each response. It writes raw
responses under `corpus/` and a `manifest.json` listing selection count,
successful sessions, step counts, endpoint, times, and the **installed** agy
version. That version field is a convenience, not proof of the daemon version.
Credentials are not included.

The output directory must not already exist. Files are private, and live
sidecars are not modified. Daemon loads can update the source store's own
state. Any fetch, render, or write failure exits nonzero; a failed or interrupted
run leaves an incomplete manifest and possibly a partial corpus. Do not assert
`--corpus-swept` for that directory. Fix the cause and rerun into a new directory.
The timeout applies per conversation; SIGINT/SIGTERM cancels an in-flight fetch.

The selected file list is captured at the start. Keep the store idle during an
audit if you need stable coverage; this is not an atomic snapshot of an active
session store. Empty histories are included but provide no step-type coverage.
A successful sweep guarantees fresh RPC responses and successful rendering,
not coverage of every optional agy feature.

Do not replace this sweep with a loop over ordinary `--sync`: the ordinary
command may use an existing sidecar when the daemon fails. The audit sweep
never falls back to cached transcript data.

## Compare and record

```sh
AGY_SIDECAR_CORPUS="$work/sweep/corpus" \
  skills/agy-format-audit/scripts/audit_format.sh --corpus-swept
bin/agy-reader shape-fingerprint --paths "$work/sweep/corpus" > "$work/shape.tsv"
```

Inspect the manifest's `complete: true`, the helper's schema and shape results,
and the changelog delta. Compare the path listing with a prior snapshot when
shape drift appears. Optional-feature coverage can cause drift; an unchanged
schema does not rule out changed payloads. Neither fingerprint covers changes
inside intentionally opaque configuration fields.

After reviewing a qualified passing run, record it:

```sh
AGY_SIDECAR_CORPUS="$work/sweep/corpus" \
  skills/agy-format-audit/scripts/audit_format.sh --record --corpus-swept
```

`--corpus-swept` asserts that every file was serialized by the version under
audit. The script still supports manually curated corpora, so this assertion
remains the operator's responsibility. Without it, an explicit corpus gets a
partial/version-specific scope. Without any explicit corpus, the helper checks
only the newest database's paired sidecar: a useful spot check, not broad
coverage. The helper inspects the newest SQLite schema, not the integrity of
every historical database. A legacy-only store cannot produce a SQLite
compatibility record.

Use a clean working tree for a record whose commit identifies all tested code.
A dirty tree is marked in the record. Do not record from an IDE root: CLI
version provenance would mislabel it. `--record` replaces `COMPATIBILITY.md`
only after the audit passes. It does not commit, install, restart services, or
update a running reader. Raw corpora and repair backups contain private
conversation data; keep them outside the repository. Retain a public report of
coverage, limitations, and fingerprints instead.

The reader unit suite runs automatically. To include the separate agentsview
consumer suite, set `AGENTSVIEW_DIR=/path/to/agentsview`. Its own dependencies
must be installed; a successful consumer unit suite still does not demonstrate
live downstream re-ingestion.

## Update the installed baseline

`COMPATIBILITY.md` is compiled into the reader. When you choose to install the
new record, rebuild from that checkout into the path your watcher uses:

```sh
make install PREFIX="$HOME/.local"
systemctl --user restart agy-reader.service # supplied Linux service
"$HOME/.local/bin/agy-reader" doctor
```

For another installation prefix or process manager, use its binary path and
restart procedure. A checkout record changing while the installed `doctor`
still reports the old version is expected until this rebuild and restart.
