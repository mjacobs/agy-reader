# Changelog

## Unreleased

### Fixed

- Current agy 1.2.16 sessions that read a file with an explicit
  `ContentOffset`, cancel a running command, or fail a `manage_task` call sync
  again. Ordinary sync and the watcher had refused them as `unsupported generic
  arguments`, `unsupported wrapper/native status difference`, or `unsupported
  generic wrapper`. The reader now accepts exactly the presentation the daemon
  gives these three steps, and still refuses a changed offset, a canceled
  command whose inner status is not `ERROR`, an outer error on any other
  payload-free wrapper, and unknown arguments.
- Sessions with tool calls that failed inside a generic wrapper can now be
  verified. Sync had refused them as `unsupported generic wrapper` or
  `unsupported generic fields`. The reader now accepts exactly three observed
  shapes, with the presentation the daemon gives each: an outer error over an
  embedded run-command, code-action or view-file Step; a failed
  `generate_image` call, which embeds an image Step under an outer error; and
  a `write_to_file`, `view_file`, `find_by_name`, `list_dir` or `schedule`
  call that failed before its tool ran and so stored no result, with status
  `ERROR` (or `INVALID`, for `write_to_file` only). The error text appears on
  the rendered step. An outer error over any other native Step, including an
  `invoke_subagent` Step, still fails, as do an outer error that does not
  repeat the native Step's own error, an extra wrapper field, an unknown
  argument, and a changed status. A resultless `view_file` that the daemon
  presents as a synthesized `VIEW_FILE` step stays unknown.
- The watcher no longer refetches, on every tick, a conversation whose own
  source it cannot support. Each refetch made the daemon load the trajectory
  again: with 178 unsupported conversations a paired watcher spent about 25
  seconds per tick on them and logged each refusal on every tick. Watch now
  logs such a refusal once and skips the conversation until its source content
  or modification time, its sidecar, or the daemon endpoint changes. A refusal
  caused by the daemon's response rather than the source, such as malformed
  JSON or an enum name outside the profile, is still retried on every tick, as
  are incomplete (stale daemon) responses.
- Generated images and subagent invocations now show what they stored. A
  `GENERATE_IMAGE` step lists its prompt, model, aspect ratio, image name, and
  generated media link and type instead of "No additional details". An
  `INVOKE_SUBAGENT` step lists each requested agent's type, role, model, and
  prompt, and each child conversation's id and result-log link. Inline image
  bytes are summarised by length, never printed.
- Sessions that set a timer with `schedule`, list or stop subagents with
  `manage_subagents`, or define one with `define_subagent` can now be verified.
  Ordinary sync and the watcher had refused them as `unsupported generic
  result`. The daemon shows these steps exactly as stored, so the reader now
  accepts them unchanged, including a failed `schedule` call with its error. It
  still refuses unknown or unrelated arguments, unexpected result fields or
  metadata, statuses other than the observed ones, and payload-free steps from
  any other tool.
- Format audits no longer offer a compatibility record for conversations the
  reader cannot sync. `audit-sweep` still fetches and renders every
  conversation, and now also runs ordinary sync's verification on each SQLite
  session without writing sidecars, recording `readerSync` per session in a
  version 2 manifest and printing verified and not-verified counts separately
  from fetch/render counts. `audit_format.sh` refuses a record unless the
  newest conversation, plus any listed in `AGY_QUALIFY_SESSIONS`, is natively
  verified: from the sweep manifest beside the corpus, or else from its live
  sidecar attestation. An explicit corpus no longer stands in for a missing
  live sidecar. Older unverified conversations are reported but do not block a
  broad audit. The new `audit-qualify` subcommand performs this check. The
  newest conversation uses the modification time the reader's watcher uses,
  so an open conversation whose latest writes are still in its `-wal` file
  counts, and the helper's own test step no longer inherits
  `AGY_QUALIFY_SESSIONS`, which had failed audits that named valid scenario
  conversations.
- Subagent messages no longer create reversed parent links or parent cycles.
  Parent discovery now prefers native parent metadata and structured invocation
  results, rejects cycles, and excludes existing cycles from Markdown trees.
  Use `backfill-parent-links --repair` on a complete sidecar corpus to recompute
  historical stamps, clearing links without supporting directional evidence.
- Image views no longer render as an empty text read. A `VIEW_FILE` step that
  carries media now shows the file link, media type, stored media link, and
  byte size instead of a `Lines 0-799` range and an empty code fence.
- `audit-sweep` no longer counts a daemon answer about another conversation
  as the selected one's evidence. It had accepted any valid trajectory ID, so a
  wrong answer could be stored in the corpus under the selected filename and
  the same conversation could appear twice. The sweep now requires the
  answer's ID to be the filename's ID or the internal ID recorded in that
  session file, refuses an ID the sweep already fetched for another file, and
  stops with an incomplete manifest like any other fetch failure.
- `audit-qualify --manifest` no longer qualifies a conversation whose source
  changed after the sweep. It had trusted the sweep's saved result, so a
  conversation that later gained a persisted field the profile does not
  support still qualified a compatibility record although ordinary sync now
  refused it. Each verified sweep entry now records the digest of the source
  it verified and the completeness profile, and `audit-qualify` reads a fresh
  snapshot of each required conversation and compares both. A changed,
  undecodable or missing source is refused with "rerun audit-sweep". The
  manifest version is now 4, and `audit-qualify` refuses a version 3
  manifest, which has no source digests, with "rerun audit-sweep".
- `sidecar_metadata.py` now protects the timestamps a rollback restores.
  Rollback copies the backup back with `cp -p`, but `verify` had checked only
  the backup's content, so a backup whose timestamp changed would have restored
  the wrong freshness timestamp. `backup` now confirms each copy carries the
  sidecar's timestamp, and `verify` fails with `backup timestamp changed` when
  a backup file's timestamp differs from the manifest.

### Changed — completeness profile `sqlite-v1-format0-agy-a759ce7c7a23-planner50`

- **Existing SQLite attestations need re-verification.** The profile id
  changed, so every attestation minted under
  `sqlite-v1-format0-agy-a759ce7c7a23` (84 standard attestations at the time)
  reads as `unknown` until the next live sync or watch re-verifies it.
- Historical sessions that store CascadePlannerConfig field 50 are no longer
  refused outright as `unknown persisted protobuf fields`. 78 retained sources
  (72 CLI, 6 IDE) carry it in generator and executor metadata. The current
  schema reserves the number and the daemon never returns it, so the full RPC
  could never match. The reader now accepts it as a declared source-only
  field, by number and wire type only, with no invented name or type, and only
  with the payloads seen so far: empty, or nested fields 2 and/or 4 set to
  zero. It is accepted at the two places it was observed and nowhere else:
  `planner_config` in a generator record and `cascade_config.planner_config`
  in an executor record. Every other planner is still unknown, including an
  executor record's `request_override_config.planner_config` and a planner
  nested deeper, for example in an agent definition, as is any other number,
  wire type, location or nested content. ChatMessagePrompt field 23 stays
  unknown. These sources still have to pass the projection checks before they
  verify.
- The attestation for such a source carries the reason
  `verified except declared source-only fields absent from RPC` and records
  the declared path, the number of occurrences, a SHA-256 of the stored bytes,
  and the payloads themselves, so the sidecar keeps them under `agyReader`.
  The source digest still covers the bytes; only the comparison with the RPC
  leaves them out.
- Each output that reports a verification result shows such a source apart
  from plain verified. `audit-qualify` prints `VERIFIED WITH SOURCE-ONLY
  FIELDS` and the reason instead of `VERIFIED`. `doctor` counts it as
  `verified with source-only fields`, separately from `verified`. An
  `audit-sweep` manifest records the reason for that conversation and counts
  it in a new `verifiedSourceOnly` field, which the summary line prints as
  `verified-with-source-only-fields`; `verified` now counts plain verified
  results only. The manifest version is now 3, and `audit-qualify` refuses a
  version 2 manifest with "rerun audit-sweep", because its `verified` does not
  mean the same thing. Watch and `--sync` add the reason to their `synced` and
  `wrote` lines, and the format-audit helper's `Reader sync: PASS` line and
  the record it writes say how many of the required conversations verified
  with source-only fields. Nothing else changes: such a source is fresh, needs
  no re-sync, keeps doctor healthy, and qualifies a format-audit record as a
  plain verified one does.

### Changed — bare invocations now cover every session store that exists

Previously a bare `agy-reader` (no `--root`, no `ANTIGRAVITY_CLI_ROOT`)
operated only on the CLI store `~/.gemini/antigravity-cli`. It now operates on
**each default store that exists on disk**: the CLI store and the
Antigravity 2.0 (IDE) store `~/.gemini/antigravity`.

What this means for existing CLI users:

- If your machine does not have `~/.gemini/antigravity`, nothing changes.
- If it does, bare `--list`/`--watch`/`doctor` now also see the IDE store's
  sessions. `--list` gains a trailing surface column (`cli`/`ide`) whenever
  more than one root is in play; single-root output is unchanged.
- The IDE surface never breaks the CLI one: a store whose daemon is down (the
  IDE is closed) is treated as *waiting* — `--watch` polls it quietly while
  continuing to sync the CLI root, and `doctor` still exits 0 as long as a
  discovered root is healthy. Only explicitly requested roots (`--root`,
  `ANTIGRAVITY_CLI_ROOT`) are hard requirements.
- To pin the old behavior exactly, set `ANTIGRAVITY_CLI_ROOT` or pass
  `--root ~/.gemini/antigravity-cli` — explicit configuration suppresses store
  discovery.

### Added

- `scripts/agy-pair.py --token-mode` can now pair agy with a reader without
  putting agy's token in any process's arguments, where other local accounts
  could read it. `no-flag` lets agy pick its own token and catches it from
  agy's first tool process, so sync starts after that tool. `shim` (opt-in,
  built with `make shim`, Linux and glibc only) hands agy the token through a
  preload library before agy reads its arguments, so sync starts at once;
  `--agy` must then name the agy executable itself, not a wrapper. The default
  stays `argv`, which still passes `--csrf_token` on the command line.
- `audit-sweep --out NEW_DIRECTORY` fetches and renders a fresh corpus from
  one store without cached-sidecar fallback, with a private completion manifest.
- A metadata repair/rollback runbook and Python backup/verification helper
  check payloads, freshness timestamps, parent cycles, and idempotence.
  Installation and audit runbooks now cover service paths, prerequisites,
  and rebuilding the embedded compatibility record. Audit file selection
  and hashing use Python 3 instead of GNU-only utilities.
- Sidecars now carry an optional reader-owned
  `agyReader.parentCascadeId` immediate-parent pointer for subagent sessions.
  Sync/watch resolve relationships in a second directory-wide pass, while the
  `backfill-parent-links` command migrates historical sidecars offline. Raw
  daemon JSON values are preserved across both paths, conflicts are diagnostic,
  and repeated backfill is a no-op.
- Sidecar shape fingerprints completely exclude the reader-owned `agyReader`
  subtree, keeping daemon-format compatibility checks byte-stable as reader
  metadata changes.
- `--root` is repeatable: `--root A --root B` operates on both roots in order.
  Sync-by-id searches the roots in order and uses the daemon, CSRF config, and
  sidecar location of the root that holds the session; an id on no root's disk
  is probed against each root's daemon in order.
- `--watch` runs one loop per root in a single process, with per-surface log
  labels; each loop (re)discovers its own daemon independently.
  `--watch-idle-timeout` exits only once every root has been idle that long.
- The example systemd units (`deploy/systemd/`) trigger on Antigravity 2.0
  activity too: the `.path` unit also watches the IDE's logs dir and its
  `conversations/`, so an idle-exited service re-arms on IDE activity, not just
  agy's.
- `doctor` prints one block per root and reports a `surface:` line plus, for
  IDE roots, a `csrf:` line; IDE daemon port and CSRF token are discovered
  automatically from the Antigravity 2.0 logs. The `watch:` line is root-aware:
  it reports a watcher as running only when that watcher actually covers the
  reported root (explicit `--root` match, or a bare watcher for default roots).
