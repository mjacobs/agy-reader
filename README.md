# agy-reader

A Unix-style Go CLI that extracts decrypted transcripts from
[Google Antigravity CLI](https://antigravity.google) sessions by talking to the
local language-server daemon Antigravity runs while `agy` is active. Encrypted
`.pb` session files live under `~/.gemini/antigravity-cli/{conversations,implicit}/`;
agy-reader fetches the decrypted JSON from the daemon, renders Markdown for
humans, and writes a `<uuid>.trajectory.json` sidecar next to each `.pb` file
for downstream tools. The daemon binds a different ephemeral port each `agy`
session, so you must export `ANTIGRAVITY_DAEMON_URL` first
(see [Troubleshooting](#troubleshooting)).

## Why

The sister project [`agentsview`](https://github.com/mjacobs/agentsview) is a
local web viewer for AI agent sessions. It can list Antigravity CLI sessions
but cannot render assistant turns because they're AES-GCM encrypted at rest.
agy-reader fills that gap by producing a plain JSON sidecar that agentsview
will detect and parse.

**Integration contract is the file format.** No imports, no protocol, no
coupling — just `<uuid>.trajectory.json` sitting next to `<uuid>.pb`.

## Quick start

Build:

```bash
go build ./cmd/agy-reader
```

`ANTIGRAVITY_DAEMON_URL` is required for any command that hits the daemon —
see [Troubleshooting](#troubleshooting) for the one-liner that finds the
current port.

List discovered sessions (does not contact the daemon):

```bash
agy-reader --list
```

Render one to stdout as Markdown:

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
`<uuid>.trajectory.json` next to the source `.pb` whenever it can — that's
the point of the contract.

### Sidecar contract for agentsview

For every `~/.gemini/antigravity-cli/{conversations,implicit}/<uuid>.pb`,
agy-reader writes `<uuid>.trajectory.json` in the same directory. The contents
are the raw `GetCascadeTrajectory` response from the Antigravity daemon — no
schema invented on top. agentsview is expected to ignore unknown step types
and use `metadata.createdAt` for timestamps.

## Watch mode

```bash
agy-reader --watch                       # 30s interval (default)
agy-reader --watch --watch-interval=10s  # custom interval
```

Polls the session root, fetches a trajectory for any `.pb` whose sidecar is
missing or older than the `.pb` file, and writes the sidecar atomically.
Daemon errors are non-fatal — connection-refused logs once per failure
streak and the loop retries on the next tick. SIGINT or SIGTERM drains
in-flight work and exits cleanly.

## Troubleshooting

**`ANTIGRAVITY_DAEMON_URL is not set`**

agy-reader has no built-in default URL because the agy daemon binds a
different ephemeral port every session. Find the current port with:

```bash
ss -tlnp 2>/dev/null | grep agy            # Linux
lsof -iTCP -sTCP:LISTEN -anP | grep agy    # macOS
```

The lower-numbered port is the JSON-RPC endpoint (the higher one is an
internal sidecar). Export it:

```bash
export ANTIGRAVITY_DAEMON_URL=http://127.0.0.1:<port>
```

Auto-discovery via `ss`/`lsof` is planned for v0.1.

**`connection refused`**

The daemon only listens while `agy` (Antigravity CLI) is running, and the
port changes each time `agy` restarts. Start `agy` again, look up the new
port with the snippet above, and re-export `ANTIGRAVITY_DAEMON_URL`.
agy-reader does not start, supervise, or restart the daemon itself — it's
borrowing a process you already have running.

**`daemon error 5xx` or unknown cascade id**

The daemon only knows about sessions it has loaded. Calling `LoadTrajectory`
first (which `agy-reader` does automatically) usually solves this, but the
daemon will refuse if the session truly doesn't exist or its key is
unavailable.

**`no sessions found`**

Set `ANTIGRAVITY_CLI_ROOT` if your sessions are not at the default
`~/.gemini/antigravity-cli`.

## Configuration

| Env var                  | Purpose                                                | Default                     |
|--------------------------|--------------------------------------------------------|-----------------------------|
| `ANTIGRAVITY_DAEMON_URL` | Daemon base URL (REQUIRED — port changes each session) | unset (error if missing)    |
| `ANTIGRAVITY_CLI_ROOT`   | Override session root dir                              | `~/.gemini/antigravity-cli` |
| `AGY_READER_LIVE`        | Enable live daemon smoke test                          | unset (test skips)          |
| `AGY_READER_TEST_UUID`   | Cascade id to use in the live test                     | unset                       |

## Running on a schedule

agy-reader **does not ship a daemon installer**. Pick whatever process manager
you already use — the examples below are starting points, not installation
instructions. Cron is fine too.

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

Load with `launchctl load -w ~/Library/LaunchAgents/dev.mjacobs.agy-reader.plist`.

### cron

Cron is perfectly fine if you don't want a long-running process:

```cron
*/5 * * * * ANTIGRAVITY_DAEMON_URL=http://127.0.0.1:PORT /usr/local/bin/agy-reader --watch --watch-interval=1m 2>/dev/null
```

Because the daemon port changes per `agy` session, cron is less convenient
than a long-running `--watch` invocation kicked off after `agy` starts.

## Testing

```bash
go test ./...
```

The daemon smoke test is gated. To exercise it against a live `agy`:

```bash
AGY_READER_LIVE=1 AGY_READER_TEST_UUID=<some-cascade-id> go test ./internal/daemon
```

## Status

v0 — single-shot fetch and render, plus a polling `--watch` loop.
`ANTIGRAVITY_DAEMON_URL` must be set manually; v0.1 will add auto-discovery
via `ss`/`lsof`. Offline decryption (direct binary RE) is a v1 stretch goal.

## License

MIT
