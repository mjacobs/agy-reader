# Changelog

## Unreleased

### Fixed

- An idle watcher no longer keeps about a fifth of a core busy re-checking
  sessions that have not changed. The verification cache trusted unchanged
  file metadata for only four to five minutes, then checked every session in
  full again; on a store of 342 sessions, one such pass costs about 56
  CPU-seconds. Reuse now grows with how long a session had been quiet when it
  was checked: a session that changed in the last 20 minutes or so keeps the
  four-to-five-minute limit, and a quieter one is reused for a quarter of its
  quiet time, up to five to six hours (never six). Held unsupported sessions
  follow the same rule. Quiet time counts from the later of each file's mtime
  and ctime, so backdating a file cannot lengthen reuse, and any change the
  file metadata or the WAL-index header shows still forces a full check on the
  next tick. On a synthetic store of 60 sessions quiet for two days, an idle
  watcher at a 1-second interval used 0.20% of a core with no full checks in
  ten minutes, against 4.40% and 120 full checks before.
- A watcher no longer risks leaving a session unsynced for up to five minutes
  when agy's write to it stalls partway. SQLite writes a commit's WAL frames
  before it makes them visible to readers. A verification check in that gap
  saw the previous content, and the verification cache kept that result under
  the new file metadata until it expired, because making the commit visible
  changes nothing the cache compared. The cache now also compares the
  WAL-index header that SQLite updates when it makes a commit visible, and
  does not reuse a check while the WAL holds frames that header has not yet
  made visible. A session agy has open and is not changing is still reused,
  at about 0.07 ms per tick. A session whose rollback journal has content, or
  whose WAL has content while that header cannot be read, is checked in full
  on every tick.
- An idle watcher no longer keeps more than a core busy re-reading sessions
  it cannot verify. On every tick it read and decoded each held unsupported
  source in full to confirm its content had not changed. On one store, six
  such sessions (60 MB together) cost 1.35 CPU-seconds per tick, so the
  1-second polling that `scripts/agy-pair.py` uses ran at about 155% CPU with
  nothing to sync. A held source is now read once to confirm the hold; after
  that, unchanged file metadata (identity, size, mtime and ctime of the
  database, WAL and journal) stands in for the read until the confirmation
  expires under the reuse limit the verification cache allows (see the entry
  above: four to five minutes for a recently changed source, up to six hours
  for a quiet one). The same check for all six now takes about 30
  microseconds. A source held because it could not be read is still read on
  every tick, so it is retried as soon as it becomes readable. So is a source
  whose WAL or rollback journal has content, such as one agy is still writing:
  SQLite writes a commit's WAL frames before readers can see them, so a read
  during a commit can see the previous content under the new file metadata.
- A session in which a command was canceled while its permission prompt was
  still open can now be verified. This happens when agy is stopped before
  anyone answers the prompt. The reader had refused the whole session as
  `unsupported generic fields`: the step is a `run_command` wrapper with
  status `CANCELED`, the arguments, and no result, error or interaction
  record. The daemon presents it as a canceled `RUN_COMMAND` step built from
  the arguments, so the reader now builds the same step and compares, and the
  transcript shows the command as canceled with no output. The step must
  record one `allow` hook result and a status history ending in a wait for the
  answer, then cancellation; an error, a grant, an interaction record, an
  extra argument or field, or another status is still refused.
- Sessions in which a command's permission request was rejected can now be
  verified. This happens when a user running agy in review mode answers "No,
  cancel" or "No, deny", and when agy soft-denies a command that a subagent of
  a headless `-p` run asked for. The reader had refused the whole session as
  `unsupported generic wrapper`: the step is a `run_command` wrapper with
  status `ERROR`, the arguments, no result and no grant, an outer error saying
  the user denied the command, and the completed request answered without
  `allow`. The daemon presents it as a `RUN_COMMAND` step built from the
  arguments that keeps the error and the request, so the reader now builds the
  same step and compares. An answer that allows, another trigger, a changed
  error, a missing record, an extra field or another status is still refused.
- A session in which the agent asked for permission with `ask_permission`
  can now be verified. The reader had refused it as `unsupported generic
  result`: the step is a generic wrapper with status `DONE`, the `Action`,
  `Reason` and `Target` arguments, and a result of only text and render
  information, and the projection did not list it as a payload-free tool. The
  daemon's ordinary response presents it unchanged, so the reader now accepts
  that shape as presented. Any other set of arguments, a payload, any other
  result field, an outer error, task details or permission records, or any
  other status is still refused.
- Sessions that called `list_permissions` can now be verified. The reader had
  refused them as `unsupported generic result`: the step is a generic wrapper
  with status `DONE`, no arguments, and a result of only text and render
  information, and the projection did not list it as a payload-free tool. The
  daemon's ordinary response presents it unchanged, so the reader now accepts
  that shape as presented. An argument, a payload, any other result field, an
  outer error or task details, or any other status is still refused.
- Sessions with a successful `generate_image` call stored as an embedded
  `GENERATE_IMAGE` step can now be verified. The reader had refused them as
  `unsupported native tool projection`, because it accepted that stored form
  only for a failed call. The daemon presents the embedded step unchanged and
  drops the wrapper's arguments, result text and media, so the reader now
  accepts the call only when the step is `DONE` inside and out with no error,
  the wrapper's one media item equals the step's generated media, and the
  wrapper's prompt, image name and aspect ratio equal the step's. Changed
  media or arguments, an unknown field, or another status is still refused.
- Sessions with a `write_to_file` call that appends to a file can now be
  verified. The reader had refused them as `unsupported generic arguments`,
  because the projection did not know the `Append` argument. The daemon's
  ordinary response still shows the value, in the presented tool call and as
  `append` on the file-creation spec, so the reader now accepts `Append` only
  when both agree with it. A changed value, a value other than `true` or
  `false`, or the argument on another tool is still refused.
- A watcher you start yourself, such as the systemd service, can now
  authenticate to an agy started by `scripts/agy-pair.py`. It never found such
  an agy before, in any token mode including `argv`: discovery matches an agy
  to its session store only through an open log in that store, and the helper
  gave agy a log in its temporary run directory. The helper now writes agy's
  log to `log/cli-pair-*.log` under `--root` and links it from the run
  directory as `agy.log`. In `shim` mode the token now stays in agy's own
  `/proc/<pid>/environ`, which only your user and root can read, and discovery
  reads it there, so the watcher authenticates while agy is idle; tools agy
  runs still do not inherit it. In `no-flag` mode the watcher authenticates
  once it sees a tool process, as with any agy started without the flag. The
  helper puts agy's log in the store only if no other account can change that
  `log` directory or a directory above it; otherwise the log stays in the
  private run directory, and a separately started watcher does not find that
  agy. The helper reads agy's port through the log file it created rather than
  by name, sends a token only to a listener agy itself holds, and refuses a
  `--log-dir` that another account could change.
- Sessions with a `view_file` call that agy rejected before running it, such
  as a view of a named pipe or `/dev/null`, can now be verified. The reader
  had refused them as `unsupported resultless view_file`, because the daemon
  presents such a step as a `VIEW_FILE` step built from the call's arguments
  rather than as stored. The reader now builds the same step and accepts it
  only when the path cannot be a skill file: the daemon marks skill files
  with a flag the source does not record, and in retained sessions it never
  set that flag for a path without `skill` in it. A view of a path containing
  `skill`, an unusual path, any other argument, or a changed line range is
  still refused with the same message.
- A forked conversation no longer stops `audit-sweep`, appears twice in
  `--list`, or makes the watcher fail on every tick. agy 1.2.17 writes
  `<id>.pb` when a conversation is forked and `<id>.db` once the fork
  continues, and leaves both. The reader had listed each file as a session, so
  `audit-sweep` refused the whole store with `invalid or duplicate session ID`,
  and the watcher fetched the `.pb` entry every tick and then failed to write
  its sidecar because the `.db` beside it had no matching attestation. The
  reader now lists such an id once, with the `.db` as its source, because the
  `.db` holds the later, complete history; the `.pb` is not read. A `.pb` with
  no `.db` beside it is still a legacy session.
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
  argument, and a changed status.
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
- A watcher reaching its daemon directly no longer stops a whole batch when
  one conversation's fetch fails with an HTTP error that quotes a refused
  connection or unknown host, such as a 502 `dial tcp ...: connection refused`.
  It had counted the daemon as down, so the older conversations behind that
  one were never synced and an idle timeout could end the watcher while the
  daemon still answered. The watcher now asks the daemon's own listing RPC,
  once per tick. If it answers, that conversation is skipped and the batch
  goes on; if not, the daemon counts as down as before, so a reverse proxy
  pinned as the daemon URL whose daemon has stopped still idles the watcher
  out. Refused connections, and HTTP answers through a forward proxy, are
  handled as before.

### Changed — completeness profile `sqlite-v1-format0-agy-1.3.0-19be6af38f7b-planner50-permission2-prompt23`

- **Existing SQLite attestations need re-verification.** The profile id
  changed, so every attestation minted under
  `sqlite-v1-format0-agy-1.3.0-19be6af38f7b-planner50-permission2` reads as
  `unknown` until the next live sync or watch re-verifies it.
- Historical CLI sessions that store ChatMessagePrompt field 23 are no longer
  refused outright as `unknown persisted protobuf fields`. 15 retained
  sources, written between mid-June and late July 2026, carry it 47 times. It
  holds a structured copy of a `grep_search` step's results on the tool-result
  prompt that answers it; later builds write the same content to field 25,
  which the reader already compares. The daemon never returns field 23, so the
  reader now accepts it as a second declared source-only field, by number and
  wire type only, with no invented name. It is accepted only on a generator
  record's chat-model prompt, once, on a tool-result prompt that names its tool
  call and step, when the payload is a strict `google.protobuf.Struct` of the
  observed shape and every value in it comes from the named `grep_search`
  step: each path, line and line number + 1 from the matching result, and any
  remaining count from `total_results`. Anything else is still unknown. These
  sources still have to pass the projection checks before they verify.
- An attestation's `sourceOnly` record now has one entry per declared field
  present: field 50 first, then field 23, each with its own count, SHA-256
  and payloads.

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
