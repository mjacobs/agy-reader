# Repairing historical parent metadata

Use this workflow when `parent-cycle` or stale-parent diagnostics refer to
existing `agyReader.parentCascadeId` stamps. Repair recomputes these pointers
from directional evidence in the complete sidecar collection. It does not
change daemon-owned conversation data or decrypt missing sessions.

Use a current reader build with the metadata freshness fix. From a checkout:

```sh
go build -o bin/agy-reader .
reader="$PWD/bin/agy-reader"
root="$HOME/.gemini/antigravity-cli"
```

The repair command requires only the reader binary. The backup and verification
helper below requires Python 3 with its standard library. Run it from the
checkout. For an IDE store, set `root="$HOME/.gemini/antigravity"`; repair each
store separately. Do not use a sampled or partial collection.

## 1. Stop sidecar writers

For the supplied systemd setup, note which units are active, then stop both:

```sh
systemctl --user is-active agy-reader.path agy-reader.service
systemctl --user stop agy-reader.path agy-reader.service
systemctl --user is-active agy-reader.path agy-reader.service
```

The final command should report both inactive (and returns nonzero). Stopping
only the service is insufficient: the path trigger can restart it during
maintenance. Also stop any manually launched reader watches or sync commands.
Do not issue prompts or run other maintenance against this store until done.
The idle agy daemon itself can remain open.

For launchd, unload the watcher using the same plist used to install it:

```sh
launchctl unload "$HOME/Library/LaunchAgents/dev.mjacobs.agy-reader.plist"
```

For a terminal watcher, interrupt it and wait for shutdown. On every platform,
confirm that no other reader process will write the selected store. The helper
checks for changed files but does not lock the store or manage processes.

## 2. Back up and repair

The backup contains decrypted conversations. Keep it private and outside the
repository. The helper refuses to reuse a backup directory, rejects symlinks
and invalid JSON, and records file hashes and original freshness timestamps.
An interrupted backup without a manifest must not be used.

```sh
work=$(mktemp -d "${TMPDIR:-/tmp}/agy-reader-repair.XXXXXX")
python3 scripts/sidecar_metadata.py backup --root "$root" --backup "$work/before"
"$reader" backfill-parent-links --repair --root "$root" 2>"$work/repair.log"
cat "$work/repair.log"
python3 scripts/sidecar_metadata.py verify --root "$root" --backup "$work/before"
```

Stop on a failed command. Inspect repair diagnostics even when the command
exits zero: ambiguous evidence is reported without guessing a parent. A clean
repair has `diagnostics=0`. `unresolved` includes root conversations and entries
without sufficient parent evidence; it is not a count of failed writes.

Verification fails if the sidecar set changed, the backup was altered, any
non-parent payload changed, any freshness timestamp changed, or parent cycles
remain. It preserves other `agyReader` fields in the comparison. Missing
parent sidecars and the semantic correctness of all inferred relationships
still require inspecting the repair diagnostics and source evidence.

## 3. Check idempotence

Keep writers stopped. Back up the repaired state, then repeat the repair:

```sh
python3 scripts/sidecar_metadata.py backup --root "$root" --backup "$work/after"
"$reader" backfill-parent-links --repair --root "$root" 2>"$work/second-repair.log"
cat "$work/second-repair.log"
python3 scripts/sidecar_metadata.py verify --root "$root" --backup "$work/after" --unchanged
```

The second repair should report `stamped=0 cleared=0`; `--unchanged` also checks
byte-for-byte equality. Retain both logs and the backups until the consumer
shows the expected hierarchy.

## Roll back if necessary

Keep writers stopped. Use only the complete `before` backup created above.
Restore its sidecars with their original timestamps:

```sh
cp -p "$work/before/conversations/"*.trajectory.json "$root/conversations/"
```

This restores the old parent metadata too, including any old cycles. It does
not remove newly created files. If the file set changed or writers ran after
backup, investigate before restoring: copying could overwrite new transcript
content. Do not delete session databases or sidecars to force a repair.

## 4. Resume and refresh consumers

Restore the process-manager state noted in step 1. For an event-driven setup
whose service and path trigger were both active:

```sh
systemctl --user start agy-reader.service agy-reader.path
journalctl --user -u agy-reader.service -n 30 --no-pager
"$reader" --root "$root" doctor
```

If only the path trigger was active, start only `agy-reader.path`. For launchd,
load the plist again; for a terminal watcher, restart the original command.
Check a normal sync for recurring parent diagnostics as new sessions arrive.

Downstream consumers may skip unchanged source files: the repair deliberately
preserves sidecar freshness timestamps. Re-ingest affected sessions using the
consumer's supported explicit sync operation; the verification helper's
`changedFiles` lists their sidecar filenames. In agentsview versions offering
`session sync`, for example:

```sh
agentsview session sync --help
agentsview session sync "$root/conversations/<session-id>.db"
```

Use the original `.pb` path for a legacy session. Run this against the writable
agentsview instance that ingests these files; a read-only replica cannot do it.
Confirm the hierarchy in that consumer. Older consumers may need their own
full-rescan workflow. Do not touch timestamps or remove their databases as a
substitute for documented re-ingestion.
