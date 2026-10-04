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
  wrapper, and unknown arguments.
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
