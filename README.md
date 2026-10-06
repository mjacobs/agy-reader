# agy-reader

A Unix-style Go CLI that extracts decrypted transcripts from
[Google Antigravity CLI](https://antigravity.google) sessions by talking to the
local language-server daemon Antigravity runs while `agy` is active. Encrypted
conversation SQLite `.db` and legacy `.pb` session files live under
`~/.gemini/antigravity-cli/conversations/`; agy-reader fetches the decrypted
JSON from the daemon, renders Markdown for humans, and writes a
`<uuid>.trajectory.json` sidecar next to each session file for downstream tools.

Antigravity 2.0 (the IDE) stores its conversations at `~/.gemini/antigravity`
in the same format with the same daemon RPCs, and is covered too: a bare
invocation operates on **every default store that exists on disk**, so IDE
sessions are listed, synced, and watched automatically alongside CLI ones —
see [Antigravity 2.0 (IDE) sessions](#antigravity-20-ide-sessions).

By default, the daemon binds a different ephemeral port each `agy` session.
agy-reader discovers the live CLI endpoint and its CSRF token from local
processes on Linux, with session logs as a fallback for older versions.

## Why

The sister project [`agentsview`](https://github.com/mjacobs/agentsview) is a
local web viewer for AI agent sessions. It can list Antigravity CLI sessions but
cannot render assistant turns because they're AES-GCM encrypted at rest.
agy-reader fills that gap by producing a plain JSON sidecar that agentsview will
detect and parse.

**Integration contract is the file format.** No imports, no protocol, no
coupling — just `<uuid>.trajectory.json` sitting next to `<uuid>.db` or `<uuid>.pb`.

## Features

- **CLI connection discovery**: on Linux, matches the active `agy` process's
  HTTP listener to its explicit launch token or credentials exported to tools.
  Older daemons fall back to `cli.log`.
- **Rich transcript formatting**: renders `CodeAction` steps as `git`-style
  diffs and converts file URI paths into clickable local links so you can jump
  into your IDE.
- **Sidecar contract** with [agentsview](https://github.com/mjacobs/agentsview):
  every render also writes `<uuid>.trajectory.json` next to the source session for
  downstream tools to pick up.
- **Antigravity 2.0 (IDE) coverage**: reads the IDE's conversations
  (`~/.gemini/antigravity`) through the same daemon RPCs, with automatic port
  and CSRF-token discovery from the IDE's logs.
- **Multi-root by default**: with no `--root`, every default store that exists
  is covered in one invocation — one `--list`, one `--watch` process, one
  `doctor` for both surfaces. A surface whose daemon is down (e.g. the IDE is
  closed) just waits; it never fails the others.

## Install

Building requires Go 1.25 or newer. The reader binary has no Python, SQLite
CLI, or agent-harness dependency. Fetching transcripts requires a running
Antigravity daemon with access to the selected sessions; see the
[CSRF setup below](#watch-mode) if authentication fails.

```bash
go install github.com/mjacobs/agy-reader@latest
```

This drops `agy-reader` into `$(go env GOBIN)` (or `$(go env GOPATH)/bin`); make
sure that directory is on your `PATH`.

To build from a local checkout instead:

```bash
go build .
```

For the supplied Linux user service, install into its expected location:

```bash
make install PREFIX="$HOME/.local"
```

This creates `~/.local/bin` if needed. If you use `go install` instead, set the
service's `ExecStart` to the actual installed binary path. See
[event-driven service installation](docs/event-driven-startup.md#install-recommended-event-driven).

## Quick start

List syncable conversation sessions (does not contact the daemon):

```bash
agy-reader --list
```

Antigravity also writes background `implicit/` trajectory files. They are not
ordinary chat transcripts and are not syncable through the current daemon API,
so they are hidden by default. To inspect them while debugging:

```bash
agy-reader --list --include-implicit
```

Pick a cascade id from that list and render it to stdout as Markdown:

```bash
agy-reader <cascade-id>
```

Render and save:

```bash
agy-reader --format md  --out transcript.md  <cascade-id>
agy-reader --format json --out trajectory.json <cascade-id>
```

Sync the sidecar without printing anything (for agentsview consumption):

```bash
agy-reader --sync <cascade-id>
```

Even in the default (Markdown) mode, agy-reader writes the sidecar
`<uuid>.trajectory.json` next to the source `.db` or `.pb` whenever it can — that's the
point of the contract.

### Sidecar contract for agentsview

For every syncable `~/.gemini/antigravity-cli/conversations/<uuid>.db` or `.pb`,
agy-reader writes `<uuid>.trajectory.json` in the same directory. When both
`<uuid>.db` and `<uuid>.pb` exist, as agy 1.2.17 leaves them for a forked
conversation (the `.pb` from the moment of the fork, the `.db` once the fork
continues), they are one conversation and the `.db` is its source, because it
holds the later, complete history; the `.pb` is not read. The contents
preserve the raw `GetCascadeTrajectory` trajectory object from the Antigravity
daemon. Unknown fields, nested values, and large JSON numbers survive the
fetch/write boundary. Object formatting and key order are not contractual.
agentsview is expected to ignore unknown step types and use
`metadata.createdAt` for timestamps.

#### Reader metadata contract v1

agy-reader owns one optional top-level namespace that is deliberately separate
from the daemon schema:

```json
"agyReader": {
  "parentCascadeId": "<bare cascade UUID>"
}
```

- The block is omitted for roots and sessions whose immediate parent cannot be
  resolved unambiguously; an empty parent value is never written.
- `parentCascadeId` names only the immediate parent. Deeper trees are recovered
  by following parent pointers, so there is no depth/level field.
- Old sidecars without `agyReader` remain valid. Unknown sibling fields inside
  `agyReader` are preserved when the parent pointer is stamped or updated.
- The stored ID is deliberately bare. A consumer using qualified session IDs
  must add its own source prefix (for agentsview,
  `antigravity-cli:<parentCascadeId>`) and set the relationship to `subagent`
  explicitly.
- The sidecar-shape fingerprint excludes the entire `agyReader` subtree. Format
  audits therefore continue to measure daemon-owned schema only.

Normal one-shot sync and watch use two passes: first fetch/write every eligible
sidecar in the batch, then resolve relationships across the sibling corpus and
stamp unambiguous children atomically. Existing sidecars can be migrated
offline, without a running daemon:

```bash
agy-reader backfill-parent-links
agy-reader backfill-parent-links --root ~/.gemini/antigravity-cli
agy-reader backfill-parent-links --root ~/.gemini/antigravity-cli --root ~/.gemini/antigravity
```

Backfill is idempotent. Missing parent sidecars do not fail the scan; malformed,
stale, or conflicting evidence is reported diagnostically and never guessed or
silently overwritten.

The resolver prefers the daemon's `metadata.parentConversationId` and
`invokeSubagent.results[].conversationId` fields. Older agent-definition paths
and prompt-matched invocation responses are fallbacks. Messages between agents
do not establish ancestry. Cyclic links are diagnosed before new stamps are
written and excluded from the reader's rendered tree.

To repair existing incorrect stamps, first stop writers to the selected store
and back up its sidecars, then run:

```bash
agy-reader backfill-parent-links --repair --root ~/.gemini/antigravity-cli
```

Repair recomputes every parent from the available directional evidence,
replaces stale pointers, and clears unsupported, conflicting, or cyclic ones.
Use the complete conversations directory: a partial copy may omit the evidence
needed to retain a valid link. Daemon-owned payloads and other `agyReader`
fields are preserved. Re-ingest affected sessions in downstream consumers
afterward. Ordinary sync/watch does not replace historical stamps automatically.

Use the [repair and rollback runbook](docs/metadata-repair.md) for the complete
stop → backup → repair → verify → restart workflow. Its Python helper checks
payload preservation, freshness timestamps, and parent cycles. The repair
command itself needs only the reader binary.

## Antigravity 2.0 (IDE) sessions

Antigravity 2.0 stores its conversations at `~/.gemini/antigravity` in the
same shape as the CLI (`conversations/<uuid>.db`, identical SQLite schema — see
[`COMPATIBILITY.md`](COMPATIBILITY.md)) and runs the same language-server daemon
with the same RPCs. (Sessions from the deprecated pre-2.0 IDE share the same
store.)

**No configuration is needed**: when the store exists, a bare invocation covers
it automatically. `--list` gains a surface column (`cli`/`ide`) whenever more
than one root is in play, `--watch` runs one loop per root in a single process,
and `doctor` reports each root as its own block.

To pin the roots explicitly, pass `--root` — it is repeatable, and explicit
roots suppress store discovery entirely (as does `ANTIGRAVITY_CLI_ROOT`, which
pins a single root):

```bash
agy-reader --root ~/.gemini/antigravity --list           # IDE store only
agy-reader --root ~/.gemini/antigravity <cascade-id>
agy-reader --root ~/.gemini/antigravity-cli --root ~/.gemini/antigravity --watch
agy-reader --root ~/.gemini/antigravity doctor
```

When a cascade id is fetched with multiple roots active, the roots are searched
in order and the daemon, CSRF configuration, and sidecar location all follow
the root that holds the session. An id that is on no root's disk (e.g. one the
daemon holds only in memory) is probed against each root's daemon in order.

Two things differ from the CLI, and both are handled automatically:

- **Port discovery.** The IDE daemon logs its ephemeral HTTP port to
  `~/.config/Antigravity/logs/language_server.log` (the platform config dir on
  macOS/Windows) instead of a `cli.log` inside the session root. agy-reader
  detects an IDE root — no `cli.log`, plus the `antigravity_state.pbtxt` marker
  or the default IDE path — and reads the right log. `ANTIGRAVITY_DAEMON_URL`
  still overrides everything.
- **CSRF.** The IDE launches its daemon with `--csrf_token <token>`, and that
  daemon rejects RPCs missing the matching `x-codeium-csrf-token` header. The
  CLI in agy 1.2.4 also requires a token; older CLI daemons may not. For the IDE,
  agy-reader reads the token
  from the newest daemon spawn command recorded in the IDE's `main.log` (a fresh
  token is minted on every IDE restart) and attaches the header only when a
  token was found. Set `ANTIGRAVITY_CSRF_TOKEN` to override discovery.

As with the CLI, the daemon only runs while the IDE is open; sidecars written
earlier remain readable after it closes.

## Watch mode

```bash
agy-reader --watch                       # 30s interval (default)
agy-reader --watch --watch-interval=10s  # custom interval
```

Polls every session root, fetches a trajectory for any conversation `.db` or
legacy `.pb` whose sidecar is missing or stale, and writes the sidecar
atomically. After writing the batch it resolves and stamps unambiguous
immediate-parent links across the sibling sidecars. With multiple roots (the
default when both stores exist) one
process runs one loop per root, with log lines labeled by surface; each loop
discovers and re-discovers its own daemon independently, so an IDE restart on
one surface never disturbs the other. Daemon errors are non-fatal —
connection-refused logs once per failure streak and the loop retries on the
next tick; a root whose daemon is down (e.g. the IDE is closed) simply waits.
SIGINT or SIGTERM drains in-flight work and exits cleanly. With
`--watch-idle-timeout` set, the process exits only once **every** root's
daemon has been idle that long.

On Linux, CLI discovery checks the daemon's open session log and owned listening
socket before using credentials. By default, discovery uses the
`ANTIGRAVITY_LS_ADDRESS` and `ANTIGRAVITY_CSRF_TOKEN` values agy exports to a
live tool process. A process environment is readable only by its own user, so
the token stays private to you. A running watcher keeps that token in memory
after the tool exits. A fresh watcher that you start yourself, such as the
systemd service, cannot authenticate until it catches another tool process;
commands shorter than its polling interval may be missed entirely.

**Single-user machines only:** for reliable discovery even while agy is idle or
the watcher restarts, launch agy with a fresh token:

```sh
agy --csrf_token="$(uuidgen)"
```

Keep `--csrf_token` first, before any other agy arguments. This launch flag was
verified with agy 1.2.5 but is omitted from its help output. agy-reader reads it
from the live process's arguments and pairs it only with the HTTP listener
recorded in that process's open log. It does not change the token of an
already-running agy session.

A command's arguments are world-readable on Linux through
`/proc/<pid>/cmdline`, unlike its environment. On a host you share with other
Unix accounts, anyone with a local login can therefore read a launch token,
find the loopback listener, and make authenticated RPCs to the daemon —
including listing and reading decrypted conversations. Do not use the launch
flag there; stay on the exported-environment path above, which keeps the token
out of other users' reach, or use the test-pair helper's `no-flag` or `shim`
mode ([below](#running-a-test-pair)), which pair a watcher with agy without
putting the token in any process's arguments. On a shared host, mounting
`/proc` with `hidepid=invisible` (or `hidepid=2`) also hides other accounts'
processes, including their arguments. agy-reader itself never logs the token or
writes it to disk.

Both discovery paths require readable `/proc` entries in the same Linux network
namespace and a daemon log under the selected root (`cli.log` or `log/cli-*.log`).
This avoids stale `cli.log` ports left by short-lived CLI invocations. Other
platforms, restricted `/proc` access, and custom log locations can use
`ANTIGRAVITY_DAEMON_URL` and `ANTIGRAVITY_CSRF_TOKEN` overrides.
Authentication rejection stops the batch
with one `authentication failed; sync blocked` message per poll instead of
repeating the same failure for every session.

### Running a test pair

[`scripts/agy-pair.py`](scripts/agy-pair.py) starts `agy` and this checkout's
reader together. It reads agy's HTTP port from that launch's own log, confirms
the token with an authenticated RPC, then starts the reader with
`ANTIGRAVITY_DAEMON_URL` and `ANTIGRAVITY_CSRF_TOKEN` in its environment, so the
watcher stays with that `agy` process even if other sessions are running.

`--token-mode` chooses how agy gets its token:

| Mode | How agy gets the token | Sync starts | Token in process arguments |
| --- | --- | --- | --- |
| `argv` (default) | a fresh token as agy's first `--csrf_token` argument | at once, even while agy is idle | yes, readable by every local account |
| `no-flag` | agy picks its own; the helper reads it from the environment of agy's first tool process | after agy runs its first tool | no |
| `shim` (opt-in) | a preload library swaps a placeholder argument for the token before agy reads its arguments | at once | no; arguments show `--pair_token_slot=1` |

- **argv** is for single-user machines only, because of the process-argument
  exposure described above.
- **no-flag** works on Linux only. The helper polls this agy's own process tree
  every 20 ms for a process whose exported address matches the logged port, and
  confirms the token before starting the reader. A token it has found stays a
  candidate until agy accepts it or refuses it with 401 or 403, so a slow or
  failing first check does not lose it when the tool that exported it has
  already exited. `--startup-timeout` bounds only the wait for agy's listener;
  the wait for the first tool has no limit, and the helper says on stderr that
  sync is pending. A session with no tool calls is not synced during that run.
  Each watch tick fetches every conversation whose sidecar is missing, so a
  later authenticated watcher, such as the next pair in any mode, picks such a
  session up. The helper must catch the token itself: it gives agy a private log
  file, and the reader's own discovery skips an agy without an open log in the
  session root.
- **shim** needs `make shim` (a C compiler) and works only on Linux with glibc.
  It depends on agy's hidden `--csrf_token` flag in a closed-source binary that
  updates often; it was verified with agy 1.2.17.

  `--agy` must name the agy executable itself. Every process started with the
  library loads it, so a wrapper script or a launcher such as a mise shim would
  receive the real flag and pass it to agy in its arguments. The helper
  therefore refuses, before starting anything, an `--agy` that is not a native
  (ELF) executable containing the flag's name, and the library swaps in the
  token only in a process running that exact file. In any other process it
  clears the token, so agy reached through a wrapper rejects the placeholder.
  Point `--agy` at the real binary instead. One case remains: if agy re-ran its
  own executable with its arguments, the new process's arguments would hold the
  token. That did not happen with `--sandbox`. Once agy accepts the token, the
  helper checks the arguments of agy and every process under it, and stops the
  pair if the token is there.

  Before agy starts any tool, the library removes `LD_PRELOAD`,
  `ANTIGRAVITY_PAIR_TOKEN` and `ANTIGRAVITY_PAIR_EXE` from agy's environment, so
  the tools agy runs inherit none of them. agy's own `/proc/<pid>/environ` shows
  the environment agy started with, and removing a variable does not change it.
  The library zeroes the token's bytes there, so the token is gone, but
  `LD_PRELOAD` with the library's path, `ANTIGRAVITY_PAIR_EXE` with agy's path,
  and the empty `ANTIGRAVITY_PAIR_TOKEN=` name stay visible to your own user.
  The helper sets agy's `LD_PRELOAD` to the library alone: an `LD_PRELOAD` you
  already had does not apply to agy or its tools in this mode, and the helper
  says so on stderr. The reader keeps it.

  It fails closed: the helper refuses to start without the library, and if the
  library does not load, agy rejects the placeholder flag and exits with
  status 2. A watcher you start separately, such as the systemd service, cannot
  authenticate to an idle agy launched this way, because discovery reads launch
  tokens only from process arguments.

In every mode the reader receives the token through its environment, and agy
hands its token to every tool it runs. Processes running as your own user can
read both. The `no-flag` and `shim` modes protect against other Unix accounts
and anything that records process arguments, such as monitoring agents, audit
logs, or pasted `ps` output.

The helper requires Python 3.10+; detached mode also requires tmux. Build the
reader first:

```bash
make build

# Foreground: use agy normally; exiting agy stops its reader.
scripts/agy-pair.py run
scripts/agy-pair.py run -- --continue
scripts/agy-pair.py run --token-mode=no-flag
make shim && scripts/agy-pair.py run --token-mode=shim

# Detached: each named pair has its own tmux session.
scripts/agy-pair.py start --session audit
scripts/agy-pair.py attach --session audit
# Detach with Ctrl-b d; both processes keep running.
scripts/agy-pair.py status --session audit
scripts/agy-pair.py restart --session audit
scripts/agy-pair.py stop --session audit
```

`run` and `start` accept the same launch options; put agy's arguments after
`--`. For example, `run --watch-interval=2s -- --conversation ID` resumes a
conversation with a two-second watcher interval. `--reader /path/to/agy-reader`
selects another build. Restart reuses the original arguments, token mode, and
working directory, with a new token and log directory; `--continue` or `--conversation`
is needed if you want agy to resume a conversation on every restart.

The helper prints the private per-run directory containing `agy.log` and
`reader.log`; use `tail -f /path/to/reader.log` to inspect sync activity. Logs
remain after shutdown. `--log-dir /path/to/logs` selects their parent directory.
The helper does not print or save its token separately. `status` reports whether
the detached supervisor is running or has exited; the authenticated startup
message and reader log show readiness and sync results. An exited tmux pane
remains available for inspection until you stop or restart it.

The watcher polls every second by default and writes sidecars in the normal CLI
store, `~/.gemini/antigravity-cli`. `--root` changes only the store the reader
watches; it does not relocate agy's data or isolate conversations. Inherited
`ANTIGRAVITY_*` endpoint, token, and reader-root overrides are cleared for the
pair. Existing agy sessions and installed watcher services are left running.

If either process exits, the helper stops its partner. Startup times out after
30 seconds (`--startup-timeout`; in `no-flag` mode it covers only agy's
listener); shutdown allows five seconds before killing
an unresponsive child (`--stop-timeout`). `stop` and `restart` wait up to 15
seconds for cleanup; increase their `--timeout` if you selected a longer stop
timeout. A foreground run preserves agy's exit status; startup or watcher
failure returns nonzero. Very short print-mode runs may exit before the pair
is ready or before the final watcher tick, so a successful launch alone does
not guarantee a final sidecar.

## Doctor

`agy-reader doctor` is a self-check that reports whether the integration with
agentsview is healthy. It is the first thing to run when agentsview shows the
"install agy-reader" hint or a session is stuck in summary mode.

```bash
agy-reader doctor
```

```text
  surface:     cli
  daemon:      reachable (http://127.0.0.1:51847)
  agy version: 1.2.14  (compatible)
  sidecars:    SQLite 23 verified, 0 incomplete, 0 unknown (supported daemon projection)
               legacy .pb: 0/0 fresh by mtime (completeness unknown)
  watch:       running

  exit 0 — healthy
```

It checks the following:

- **surface** — whether the root belongs to the CLI (`agy`) or the Antigravity
  IDE, which determines where the daemon's port (and CSRF token) are discovered
  from.
- **daemon** — whether the Antigravity daemon is reachable, resolved the same
  way the CLI does: a pinned `ANTIGRAVITY_DAEMON_URL` if set, otherwise
  CLI process discovery (with `cli.log` fallback) or the IDE's
  `language_server.log`. An
  unpinned daemon that is simply not running is informational (it runs only
  while `agy`/the IDE is open). A pinned override that is unreachable is
  actionable, since a stale pin never self-heals and the CLI would keep using
  it.
- **csrf** — whether a CSRF token was discovered. CLI roots show this line when
  a token is available; IDE roots always report it. A reachable IDE
  daemon with no token is actionable: every RPC would be
  rejected until a token is found or pinned via `ANTIGRAVITY_CSRF_TOKEN`.
- **auth** — a reachable CLI daemon that rejects a read-only RPC is reported as
  `rejected — sync blocked`, and fails doctor even if the IDE root is healthy.
- **agy version** — the running `agy --version` compared against the baseline
  recorded in `COMPATIBILITY.md`. A skew means the format audit should re-run.
- **sidecars** — SQLite sessions are verified, incomplete, or unknown, using
  the same content checks as watch mode. Legacy encrypted `.pb` sessions retain
  mtime freshness checks, which cannot establish content completeness.
- **watch** — best-effort detection of a separate `agy-reader --watch` process
  covering the reported root, resolved the way the watcher itself would: its
  explicit `--root` flags, else its own `ANTIGRAVITY_CLI_ROOT` pin, else the
  default stores under its own `HOME` (both read from the watcher's process
  environment). Linux only; reported as `unknown` elsewhere, or when a
  watcher's environment cannot be read (a different user's process).

With multiple roots (the default when both stores exist), `doctor` prints one
block per root, each headed by its root path, and a single exit line.

**Exit codes:** `0` when nothing is actionable, non-zero when there is — stale
or missing sidecars (run `agy-reader --watch` to refresh them all, or
`agy-reader --sync <cascade-id>` per session), an agy-version skew versus the
recorded baseline, or a pinned `ANTIGRAVITY_DAEMON_URL` that is unreachable (a
stale pin never self-heals, since `agy` rebinds a new port each start). A daemon
that is simply not running with no pin is not by itself an error, so `doctor` is
safe to wire into a health check that runs while `agy` is closed.

With multiple roots, explicitly requested roots (`--root` or
`ANTIGRAVITY_CLI_ROOT`) are hard requirements: any unhealthy one fails the run.
A discovered root with an unavailable daemon can report *waiting* while another
root is healthy, provided it has no incomplete or unknown SQLite sidecars.
Missing SQLite sidecars and older sidecars without completeness evidence are
unknown, so they fail doctor even when their daemon is closed and another root
is healthy. Legacy `.pb` work can remain waiting without failing the aggregate
check. All-roots-unhealthy and CLI authentication rejection also exit non-zero.

## SQLite completeness checks

A successful daemon response can still contain an old in-memory trajectory.
For supported SQLite sessions, the reader compares a full raw RPC response
against a read-only database snapshot, including committed WAL data. It also
checks the ordinary RPC response against the supported display projection and
rechecks the source before writing. Equal step counts and matching timestamps
alone cannot pass these checks.

The sidecar keeps the ordinary daemon response, including unknown JSON fields
and large numbers. `agyReader.completeness` records the checked source and
payload digests; existing reader-owned parent links survive refreshes. It
certifies the supported daemon display projection, not an archive of generic
wrapper fields that the ordinary RPC omits. A WAL checkpoint with unchanged
content does not force a rewrite. Old sidecars without evidence are unknown
until a successful verification replaces them.

On a content mismatch, the reader tries at most eight other existing daemon
endpoints admitted by the same root ownership and credential checks. It never
starts or stops a daemon and never overrides `ANTIGRAVITY_DAEMON_URL`. If none
matches, watch leaves the sidecar unchanged and retries later. One-shot reads
may return an existing cache with a warning; that fallback cannot refresh its
contents or freshness evidence.

Unknown source fields, schemas, enum values, or projections fail closed and
need a compatibility update. When the source itself causes the refusal (it
cannot be read, lies outside the supported profile, or has a projection the
profile does not support), watch logs it once and does not fetch that
conversation again until its source content or modification time, its
sidecar, or the daemon endpoint changes, or the watcher restarts; it still
counts as skipped on each tick. A refusal caused by the daemon's response,
such as malformed JSON or an enum name outside the profile, is retried on
every tick, as is an incomplete (stale) daemon response. See
[the supported profile](docs/completeness.md)
for its exact limits and schema provenance. These checks do not install a new
watcher or repair historical sidecars until the updated reader is run.

## Troubleshooting

**`Auto-discovery failed and ANTIGRAVITY_DAEMON_URL is not set`**

On Linux, `agy-reader` first looks for the active CLI process and its launch
token or exported tool credentials. As a fallback, it scans `cli.log` inside the
session root directory
(`~/.gemini/antigravity-cli/` or `$ANTIGRAVITY_CLI_ROOT`) to locate the active
HTTP port. For an IDE root (`~/.gemini/antigravity`) it scans
`~/.config/Antigravity/logs/language_server.log` instead — that log only exists
once the IDE has run on this machine.

If the log file is missing, empty, or the server is unresponsive, auto-discovery
will fail. You can troubleshoot by:

1. Ensuring `agy` is running, as the daemon is only active during an active
   session.
1. Manually overriding the port if necessary. Find the port using:
   ```bash
   ss -tlnp 2>/dev/null | grep agy            # Linux
   lsof -iTCP -sTCP:LISTEN -anP | grep agy    # macOS
   ```
   Use the port explicitly logged as `for HTTP`, not the HTTPS/gRPC port.
   For CLI versions that require CSRF, also supply the matching token exported
   to a CLI tool process. Export the HTTP endpoint manually:
   ```bash
   export ANTIGRAVITY_DAEMON_URL=http://127.0.0.1:<port>
   ```

**`connection refused`**

The daemon only listens while `agy` (Antigravity CLI) is running, and the port
changes each time `agy` restarts. Ensure `agy` is running. If you are manually
specifying `ANTIGRAVITY_DAEMON_URL`, remember to update the port to match the
new session.

**`daemon error 5xx` or unknown cascade id**

The daemon only knows about sessions it has loaded. Calling `LoadTrajectory`
first (which `agy-reader` does automatically) usually solves this, but the
daemon will refuse if the session truly doesn't exist or its key is unavailable.
The current daemon `LoadTrajectory` request accepts only a cascade ID and
resolves it under `conversations/`; `implicit/` files are hidden by default and
are not synced by watch mode. Use `--list --include-implicit` only when
debugging those unsupported background traces.

**`no sessions found`**

Set `ANTIGRAVITY_CLI_ROOT` (or pass `--root`) if your sessions are not at the
default locations `~/.gemini/antigravity-cli` / `~/.gemini/antigravity`.

## Configuration

| Env var                  | Purpose                                                       | Default                             |
| ------------------------ | ------------------------------------------------------------- | ----------------------------------- |
| `ANTIGRAVITY_DAEMON_URL` | Daemon base URL override (optional, auto-detected by default) | unset (optional fallback)           |
| `ANTIGRAVITY_CLI_ROOT`   | Pin a single session root (suppresses store discovery)        | unset (each default store that exists) |
| `ANTIGRAVITY_CSRF_TOKEN` | CSRF token override for daemons that enforce one              | unset (auto-discovered for Linux CLI and the IDE) |
| `AGY_READER_LIVE`        | Enable live daemon smoke test                                 | unset (test skips)                  |
| `AGY_READER_TEST_UUID`   | Cascade id to use in the live test                            | unset                               |

## Running on a schedule

agy-reader **does not ship a daemon installer**. `--watch` is a long-running
loop that can run under your process manager. Linux users can follow the
[event-driven systemd installation guide](docs/event-driven-startup.md), using
the tracked [service and path units](deploy/systemd/). They start the reader
when Antigravity writes session data and stop it after both daemons go idle.
The examples below are alternatives for an always-on watcher.

### systemd (user service, Linux)

`~/.config/systemd/user/agy-reader.service`:

```ini
[Unit]
Description=agy-reader sync loop
After=default.target

[Service]
Type=simple
ExecStart=%h/.local/bin/agy-reader --watch
Restart=on-failure
RestartSec=30

[Install]
WantedBy=default.target
```

Enable with `systemctl --user enable --now agy-reader`.
This example assumes installation with `make install PREFIX="$HOME/.local"`.
Do not combine it with the event-driven service configuration.

### launchd (macOS)

`~/Library/LaunchAgents/dev.mjacobs.agy-reader.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
        "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
    <dict>
        <key>Label</key>
        <string>dev.mjacobs.agy-reader</string>
        <key>ProgramArguments</key>
        <array>
            <string>/usr/local/bin/agy-reader</string>
            <string>--watch</string>
        </array>
        <key>RunAtLoad</key>
        <true/>
        <key>KeepAlive</key>
        <true/>
        <key>StandardOutPath</key>
        <string>/tmp/agy-reader.out</string>
        <key>StandardErrorPath</key>
        <string>/tmp/agy-reader.err</string>
    </dict>
</plist>
```

Load with
`launchctl load -w ~/Library/LaunchAgents/dev.mjacobs.agy-reader.plist`.
Replace `/usr/local/bin/agy-reader` with your installed binary's absolute path.

## Testing

The Linux audit-script tests also need Bash and Python 3. `make check` runs
formatting, Go vet/tests, the Python maintenance tests, and a build.

```bash
go test ./...
make maintenance-test # Python 3: repair backup/verification tests
```

The daemon smoke test is gated. To exercise it against a live `agy`:

```bash
AGY_READER_LIVE=1 AGY_READER_TEST_UUID=<some-cascade-id> go test ./internal/daemon
```

## Compatibility

The Antigravity CLI (`agy`) can change its local session format between
releases. The last `agy` version agy-reader was verified against — along with a
deterministic schema fingerprint and the commit it was verified at — is recorded
in [`COMPATIBILITY.md`](COMPATIBILITY.md).

To re-verify after an `agy` upgrade, run the audit helper directly:

```bash
skills/agy-format-audit/scripts/audit_format.sh          # read-only: print a record
skills/agy-format-audit/scripts/audit_format.sh --record # overwrite COMPATIBILITY.md
```

These commands require a repository checkout, Go 1.25+, Bash, Python 3,
Git, and the `sqlite3` CLI; `jq` is optional. Python uses only its standard
library. File selection and hashing do not require GNU `find` or `sha256sum`.
The optional agentsview tests require a separate checkout selected with
`AGENTSVIEW_DIR`. See the [full audit workflow](docs/format-audits.md).

For a broad audit, collect fresh responses with the strict sweep first:

```bash
go build -o bin/agy-reader .
bin/agy-reader audit-sweep --out /path/to/new-private-audit
AGY_SIDECAR_CORPUS=/path/to/new-private-audit/corpus \
  skills/agy-format-audit/scripts/audit_format.sh --corpus-swept
```

The sweep selects one store (CLI by default), fetches and renders every
conversation, and writes `corpus/` plus `manifest.json` in a new private
directory. Any fetch or render failure exits nonzero and leaves an incomplete
manifest. It never uses cached sidecars or overwrites live sidecars. It also
records whether ordinary sync could verify each SQLite conversation; rendering
is not syncing. The helper refuses a record unless the newest conversation,
and any listed in `AGY_QUALIFY_SESSIONS`, passes that reader-sync check. After verifying
the serving daemon's version and inspecting the results, add `--record`.

`COMPATIBILITY.md` is embedded in the binary at build time. Recording updates
the checkout only. Rebuild/reinstall the reader and restart its watcher to
update an installed binary's `doctor` baseline; record with a clean source
tree when you need the commit to identify exactly what was tested.

If the newest database has just been created, the helper waits up to 35
seconds for agy-reader's watcher to write its first sidecar. If the sidecar
shape still cannot be computed, the audit exits with a finding and will not
print or overwrite a compatibility record.

It is read-only by default and prints an updated record; pass `--record` to
overwrite `COMPATIBILITY.md` once you've confirmed the run qualifies. A re-run
reports `UNCHANGED` when the schema fingerprint matches the recorded baseline
and `DRIFT` when it does not.

The [agy 1.2.10 audit](docs/compatibility/agy-1.2.10.md) records a live
three-worker scenario and 240 freshly serialized conversations. Storage and
raw-payload checks passed; bidirectional subagent messages exposed the reader
parent-link cycle addressed by directional evidence and explicit repair above.

The [agy 1.2.5 baseline report](docs/compatibility/agy-1.2.5.md) records coverage
from 229 freshly serialized conversations and retains the canonical field/type
list behind its fingerprint. Use its comparison procedure for a broad baseline
after an upgrade. The newest paired sidecar is a spot check; an empty trace does
not exercise the step payloads that a broad corpus covers.

The helper is also packaged as the `agy-format-audit` agent skill. Its tracked
source lives under [`skills/`](skills/agy-format-audit/); `make install-skills`
symlinks it into the gitignored agent skill directories (`.claude/skills/`,
`.agents/skills/`) so Claude Code and other agents can invoke it.

## Status

Active development. Currently supports:

- CLI port and CSRF discovery from Linux processes, with `cli.log` fallback;
  IDE discovery from `language_server.log` and `main.log`.
- Antigravity 2.0 (IDE) conversations, discovered automatically alongside the
  CLI store, including CSRF token discovery for the IDE daemon.
- Repeatable `--root` for explicit multi-root operation.
- Single-shot fetch and render.
- Continuous polling sync via `--watch` — one loop per root in one process.
- Interactive formatting (clickable file links, code diffs, clean text layouts).

Offline decryption (direct binary RE) is planned as a future goal.

## License

MIT
