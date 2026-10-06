# Auditing an agy upgrade

An audit checks whether the reader can still load, render, and **sync**
conversations, and whether the storage schema or daemon payload structure
changed. These are separate results. A conversation can fetch and render
cleanly while ordinary sync refuses it, because its persisted source falls
outside the reader's completeness profile; the agy 1.2.15 and 1.2.16 audits
both found this. Only reader-sync qualification shows that the reader can
verify and persist a conversation. The audit does not require an agent harness. The tracked helper and commands below are the
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

The sweep also qualifies reader sync for every SQLite conversation. It runs the
same verification ordinary sync performs before writing a sidecar: decode the
persisted source under the completeness profile, then match it against full and
default RPC. It writes no sidecar, and it adds a load, an ordinary fetch, and
a full-verbosity fetch per SQLite conversation under the same per-conversation
timeout. Each manifest entry records `source` and
`readerSync` (`verified`, `incomplete`, `unknown`, or `unverifiable-legacy`
for legacy protobuf sessions) with a reason, and the manifest's `readerSync`
block counts them. A verified entry also records `sourceDigest` and `profile`:
the persisted source the verification read and the completeness profile it
ran under. A conversation that fails this check does not stop the
sweep: historical stores often hold old conversations outside the current
profile, and broad inspection must still cover them. The summary prints the
counts separately from the fetch/render counts and lists up to 20 unverified
conversations, for example:

```text
audit-sweep: selected=3 fetched=3 rendered=3 failures=0 corpus=...
audit-sweep: reader sync verified=1/2 SQLite not-verified=1 legacy-unverifiable=1 (ordinary sync verification; no sidecars written)
audit-sweep: NOT VERIFIED <id>: unknown: unknown persisted protobuf fields
```

`failures=0` means fetched and rendered, not synced.

A conversation that verifies while its source carries a declared source-only
field (see [completeness.md](completeness.md)) is recorded as `verified` with
that field's reason. The manifest counts it in `verifiedSourceOnly`, not in
`verified`, and the summary adds `verified-with-source-only-fields=N` when
there is one. Manifest version 3 introduced this split; `audit-qualify`
refuses an older manifest, whose `verified` did not separate such
conversations, and asks for a new sweep. `audit-qualify` prints such a
conversation as `VERIFIED WITH SOURCE-ONLY FIELDS` with the reason. It still
qualifies the record; the helper's `Reader sync: PASS` line and the record say
how many of the required conversations verified that way.

Do not replace this sweep with a loop over ordinary `--sync`: the ordinary
command may use an existing sidecar when the daemon fails. The audit sweep
never falls back to cached transcript data.

## Compare and record

```sh
AGY_SIDECAR_CORPUS="$work/sweep/corpus" \
  skills/agy-format-audit/scripts/audit_format.sh --corpus-swept
bin/agy-reader shape-fingerprint --paths "$work/sweep/corpus" > "$work/shape.tsv"
```

Name the audit's scenario conversations (the ones created to exercise the
release's changes) so that they gate the record alongside the newest session:

```sh
AGY_QUALIFY_SESSIONS="<scenario-id> <child-id>" \
AGY_SIDECAR_CORPUS="$work/sweep/corpus" \
  skills/agy-format-audit/scripts/audit_format.sh --corpus-swept
```

The helper refuses to print or write a record unless every required
conversation passes reader-sync qualification. The newest conversation is the
most recently written one, by the modification time the reader's watcher
uses: a conversation agy still holds open counts by its nonempty `.db-wal`
file, whose writes reach the main `.db` only at a checkpoint. With a sweep
corpus it reads the sweep's `manifest.json` (beside `corpus/`), which must be
a complete version 4 manifest for the same root; rerun the sweep if it reports
an older version. A verified result counts only while the conversation's
current source still has the recorded digest under the current profile: a
conversation whose source changed after the sweep, no longer decodes, or is
gone is refused with "rerun audit-sweep", because ordinary sync may now refuse
it. Without one it rechecks each conversation's live sidecar
attestation against its current source; a missing sidecar is a finding, and
an explicit corpus does not stand in for it. Unverified historical conversations from the sweep are printed as
`not gating`. Run the check directly with
`bin/agy-reader audit-qualify --root ROOT [--manifest PATH] ID...`.

Inspect the manifest's `complete: true`, the reader-sync results, the
helper's schema and shape results, and the changelog delta. Compare the path listing with a prior snapshot when
shape drift appears. Optional-feature coverage can cause drift; an unchanged
schema does not rule out changed payloads. Neither fingerprint covers changes
inside intentionally opaque configuration fields, such as executor metadata or
generic payloads: the field that blocked 1.2.15 sync was invisible to both.
Reader-sync qualification, not shape drift, detects that kind of change.

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
