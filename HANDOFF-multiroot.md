# Handoff: agy-reader multi-root (phases 1+2 before public rollout)

Start-of-session context for the multi-root work. Plan of record: **kata 83gs**
(history: kata 65kf). Decision (Matt, 2026-07-08): do NOT roll out the current
explicit-root UX publicly; build repeatable `--root` (phase 1) and
default-discover-both (phase 2) first, ship once. The IDE-support BRANCH is
unshipped (merged to local main only), so its explicit-root UX never becomes a
compat surface — but agy-reader itself HAS real users today (agentsview users
on the agy CLI path, confirmed on GitHub). The CLI path is therefore a live
contract: default root, env vars, exit codes, and sidecar behavior must hold,
and phase 2's default flip is a user-visible change for them (bare
invocations start listing/watching IDE sessions too when the IDE store
exists). Treat that as a release-notes item plus an explicit test scenario:
CLI-only user, IDE store present, IDE daemon down -> everything stays quiet
(soft-fail) and CLI behavior is unchanged.

## Current state

- Local `main` at `197f6db` (branch `feat/ide-daemon-coverage` merged
  fast-forward; 13 files, +960/−87). NOT pushed to origin.
- Binary installed at `~/.local/bin/agy-reader`. A `--watch` process may still
  be running an older build, CLI root only.
- Live-verified on Antigravity 2.0 (v2.2.1): IDE root
  `~/.gemini/antigravity` synced 4/4 sessions end-to-end via auto-discovery,
  no env overrides (doctor: `surface: ide`, csrf found, 4/4 fresh, exit 0).
  CLI root regression-free (77/77 fresh, no CSRF sent — pinned by test).

## What phase 0 built (anchors for the next phases)

- `internal/discovery/ide.go` — IDE surface detection + daemon discovery from
  `~/.config/Antigravity/logs/language_server.log`. TRAP: the log has a
  `for HTTPS (gRPC)` line whose text contains " for HTTP"; the parser must
  (and does) match the HTTP line exactly.
- CSRF: IDE daemons are launched with `--csrf_token` and reject requests
  lacking `x-codeium-csrf-token`; the token is per-daemon-instance, scraped
  from the spawn line in `~/.config/Antigravity/logs/main.log`. CLI daemons
  are launched WITHOUT it — the header must never be sent there (test-pinned).
  `ANTIGRAVITY_CSRF_TOKEN` overrides, mirroring `ANTIGRAVITY_DAEMON_URL`.
- `doctor` prints `surface:` / `csrf:` lines with root-aware remediation.
- Watch loop rebuilds the daemon client on port rediscovery (IDE restart =
  new port + new token).
- Tests are stdlib-style — this repo uses NO testify.

## Phase 1 — repeatable `--root`

Root becomes a slice; per-root pipeline unchanged (surface detect → discovery
→ conditional CSRF → client → sidecars written into that root). `--list` /
`doctor` aggregate with a surface column; `--watch` = one loop per root in one
process. Sync-by-id searches roots in order. Explicit roots suppress
discovery.

## Phase 2 — default discovers both stores

When no `--root` and no `ANTIGRAVITY_CLI_ROOT`: include each of
`~/.gemini/antigravity-cli` and `~/.gemini/antigravity` that exists. Do NOT
scan `~/.gemini` generically (it is Gemini CLI's config home; the empty
`antigravity-ide/` dir is a known red herring). CRUX — per-surface soft-fail:
the IDE daemon is routinely down (IDE closed) while the CLI daemon runs; a
store-exists-but-daemon-down root is WAITING, not failing (watch polls
quietly; doctor reports per-surface; exit codes fail only when an explicitly
requested root is unhealthy, or all are). This semantic is what makes an
`--all` flag unnecessary.

## Docs nit to fix while in there

User-facing wording should say "Antigravity 2.0" (the current product) rather
than just "IDE" — the IDE surface was verified on 2.0; the deprecated IDE's
sessions merely share the same store.

## Relationship to agentsview (do not conflate)

agentsview's `feat/antigravity-ide-sidecar` (sidecar-wins in the IDE parser)
is a separate, parked branch — parser-only, rebased onto upstream `3d409f68`,
tip `0fdb40b9`, tests/lint green. It is fallback-safe without sidecars, so the
two ship independently. Its remaining decision (drop the two timing-fill
commits or keep them) is tracked in kata 65kf. Don't import that branch's
review history into this work.

## Verification recipe (live smoke, per surface)

1. `agy-reader doctor` (CLI default root) and
   `agy-reader --root ~/.gemini/antigravity doctor` — expect correct surface,
   reachable daemon (IDE requires the Antigravity app running), csrf found
   for IDE only.
2. `--list` then `--sync <id>` on each surface; sidecars land next to the .db.
3. CLI no-regression: no CSRF header ever sent to a CLI daemon (unit test) +
   live `--list`/`--sync` against the CLI daemon.
