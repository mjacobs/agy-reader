# Changelog

## Unreleased

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

- `--root` is repeatable: `--root A --root B` operates on both roots in order.
  Sync-by-id searches the roots in order and uses the daemon, CSRF config, and
  sidecar location of the root that holds the session; an id on no root's disk
  is probed against each root's daemon in order.
- `--watch` runs one loop per root in a single process, with per-surface log
  labels; each loop (re)discovers its own daemon independently.
  `--watch-idle-timeout` exits only once every root has been idle that long.
- `doctor` prints one block per root and reports a `surface:` line plus, for
  IDE roots, a `csrf:` line; IDE daemon port and CSRF token are discovered
  automatically from the Antigravity 2.0 logs. The `watch:` line is root-aware:
  it reports a watcher as running only when that watcher actually covers the
  reported root (explicit `--root` match, or a bare watcher for default roots).
