# agy-reader

A Unix-style Go CLI that extracts decrypted transcripts from
[Google Antigravity CLI](https://antigravity.google) sessions by talking to the
local language-server daemon Antigravity runs while `agy` is active. Encrypted
conversation `.pb` session files live under
`~/.gemini/antigravity-cli/conversations/`; agy-reader fetches the decrypted
JSON from the daemon, renders Markdown for humans, and writes a
`<uuid>.trajectory.json` sidecar next to each `.pb` file for downstream tools.

The Antigravity IDE's conversations (`~/.gemini/antigravity`) are covered too —
same format, same daemon RPCs — see
[Antigravity IDE sessions](#antigravity-ide-sessions).

By default, the daemon binds a different ephemeral port each `agy` session.
agy-reader automatically discovers this port on the fly (via the session log
files), meaning no manual configuration is required on the happy path.

## Why

The sister project [`agentsview`](https://github.com/mjacobs/agentsview) is a
local web viewer for AI agent sessions. It can list Antigravity CLI sessions but
cannot render assistant turns because they're AES-GCM encrypted at rest.
agy-reader fills that gap by producing a plain JSON sidecar that agentsview will
detect and parse.

**Integration contract is the file format.** No imports, no protocol, no
coupling — just `<uuid>.trajectory.json` sitting next to `<uuid>.pb`.

## Features

- **Port auto-discovery**: parses `cli.log` to discover and verify the daemon's
  ephemeral HTTP port; no manual configuration on the happy path.
- **Rich transcript formatting**: renders `CodeAction` steps as `git`-style
  diffs and converts file URI paths into clickable local links so you can jump
  into your IDE.
- **Sidecar contract** with [agentsview](https://github.com/mjacobs/agentsview):
  every render also writes `<uuid>.trajectory.json` next to the source `.pb` for
  downstream tools to pick up.
- **Antigravity IDE coverage**: reads the IDE's conversations
  (`~/.gemini/antigravity`) through the same daemon RPCs, with automatic port
  and CSRF-token discovery from the IDE's logs.

## Install

```bash
go install github.com/mjacobs/agy-reader@latest
```

This drops `agy-reader` into `$(go env GOBIN)` (or `$(go env GOPATH)/bin`); make
sure that directory is on your `PATH`.

To build from a local checkout instead:

```bash
go build .
```

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
`<uuid>.trajectory.json` next to the source `.pb` whenever it can — that's the
point of the contract.

### Sidecar contract for agentsview

For every syncable `~/.gemini/antigravity-cli/conversations/<uuid>.pb`,
agy-reader writes `<uuid>.trajectory.json` in the same directory. The contents
are the raw `GetCascadeTrajectory` response from the Antigravity daemon — no
schema invented on top. agentsview is expected to ignore unknown step types and
use `metadata.createdAt` for timestamps.

## Antigravity IDE sessions

The Antigravity IDE stores its conversations at `~/.gemini/antigravity` in the
same shape as the CLI (`conversations/<uuid>.db`, identical SQLite schema — see
[`COMPATIBILITY.md`](COMPATIBILITY.md)) and runs the same language-server daemon
with the same RPCs. Point `--root` (or `ANTIGRAVITY_CLI_ROOT`) at the IDE tree
and everything works the same way, sidecars included:

```bash
agy-reader --root ~/.gemini/antigravity --list
agy-reader --root ~/.gemini/antigravity <cascade-id>
agy-reader --root ~/.gemini/antigravity --watch
agy-reader --root ~/.gemini/antigravity doctor
```

Two things differ from the CLI, and both are handled automatically:

- **Port discovery.** The IDE daemon logs its ephemeral HTTP port to
  `~/.config/Antigravity/logs/language_server.log` (the platform config dir on
  macOS/Windows) instead of a `cli.log` inside the session root. agy-reader
  detects an IDE root — no `cli.log`, plus the `antigravity_state.pbtxt` marker
  or the default IDE path — and reads the right log. `ANTIGRAVITY_DAEMON_URL`
  still overrides everything.
- **CSRF.** The IDE launches its daemon with `--csrf_token <token>`, and that
  daemon rejects RPCs missing the matching `x-codeium-csrf-token` header. The
  CLI daemon is launched without a token and must not receive the header. CSRF
  is a launch configuration, not a daemon version: agy-reader reads the token
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

Polls the session root, fetches a trajectory for any `conversations/*.pb` whose
sidecar is missing or older than the `.pb` file, and writes the sidecar
atomically. Daemon errors are non-fatal — connection-refused logs once per
failure streak and the loop retries on the next tick. SIGINT or SIGTERM drains
in-flight work and exits cleanly.

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
  agy version: 1.0.14  (compatible)
  sidecars:    23/23 fresh
  watch:       running

  exit 0 — healthy
```

It checks the following:

- **surface** — whether the root belongs to the CLI (`agy`) or the Antigravity
  IDE, which determines where the daemon's port (and CSRF token) are discovered
  from.
- **daemon** — whether the Antigravity daemon is reachable, resolved the same
  way the CLI does: a pinned `ANTIGRAVITY_DAEMON_URL` if set, otherwise
  auto-discovery from `cli.log` (CLI) or the IDE's `language_server.log`. An
  unpinned daemon that is simply not running is informational (it runs only
  while `agy`/the IDE is open). A pinned override that is unreachable is
  actionable, since a stale pin never self-heals and the CLI would keep using
  it.
- **csrf** (IDE roots only) — whether a CSRF token was discovered for the IDE
  daemon. A reachable IDE daemon with no token is actionable: every RPC would be
  rejected until a token is found or pinned via `ANTIGRAVITY_CSRF_TOKEN`.
- **agy version** — the running `agy --version` compared against the baseline
  recorded in `COMPATIBILITY.md`. A skew means the format audit should re-run.
- **sidecars** — how many `conversations/` sessions have a fresh sidecar versus
  missing/stale, using the same staleness rule as watch mode.
- **watch** — best-effort detection of a separate `agy-reader --watch` process
  (Linux only; reported as `unknown` elsewhere).

**Exit codes:** `0` when nothing is actionable, non-zero when there is — stale
or missing sidecars (run `agy-reader --watch` to refresh them all, or
`agy-reader --sync <cascade-id>` per session), an agy-version skew versus the
recorded baseline, or a pinned `ANTIGRAVITY_DAEMON_URL` that is unreachable (a
stale pin never self-heals, since `agy` rebinds a new port each start). A daemon
that is simply not running with no pin is not by itself an error, so `doctor` is
safe to wire into a health check that runs while `agy` is closed.

## Troubleshooting

**`Auto-discovery failed and ANTIGRAVITY_DAEMON_URL is not set`**

By default, `agy-reader` scans `cli.log` inside the session root directory
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
   The HTTP JSON-RPC endpoint is typically the lower-numbered port. Export it
   manually:
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

Set `ANTIGRAVITY_CLI_ROOT` if your sessions are not at the default
`~/.gemini/antigravity-cli`.

## Configuration

| Env var                  | Purpose                                                       | Default                             |
| ------------------------ | ------------------------------------------------------------- | ----------------------------------- |
| `ANTIGRAVITY_DAEMON_URL` | Daemon base URL override (optional, auto-detected by default) | unset (optional fallback)           |
| `ANTIGRAVITY_CLI_ROOT`   | Override session root dir (CLI or IDE tree)                   | `~/.gemini/antigravity-cli`         |
| `ANTIGRAVITY_CSRF_TOKEN` | CSRF token override for daemons that enforce one              | unset (auto-discovered for the IDE) |
| `AGY_READER_LIVE`        | Enable live daemon smoke test                                 | unset (test skips)                  |
| `AGY_READER_TEST_UUID`   | Cascade id to use in the live test                            | unset                               |

## Running on a schedule

agy-reader **does not ship a daemon installer**. `--watch` is a long-running
loop, so use whatever process manager you already use to keep it alive — the
examples below are starting points, not installation instructions.

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

## Testing

```bash
go test ./...
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

It is read-only by default and prints an updated record; pass `--record` to
overwrite `COMPATIBILITY.md` once you've confirmed the run qualifies. A re-run
reports `UNCHANGED` when the schema fingerprint matches the recorded baseline
and `DRIFT` when it does not.

The helper is also packaged as the `agy-format-audit` agent skill. Its tracked
source lives under [`skills/`](skills/agy-format-audit/); `make install-skills`
symlinks it into the gitignored agent skill directories (`.claude/skills/`,
`.agents/skills/`) so Claude Code and other agents can invoke it.

## Status

Active development. Currently supports:

- Automatic daemon port discovery via `cli.log` (CLI) and the IDE's
  `language_server.log`.
- Antigravity IDE conversations (`--root ~/.gemini/antigravity`), including CSRF
  token discovery for the IDE daemon.
- Single-shot fetch and render.
- Continuous polling sync via `--watch`.
- Interactive formatting (clickable file links, code diffs, clean text layouts).

Offline decryption (direct binary RE) is planned as a future goal.

## License

MIT
