# Event-driven startup (systemd)

How to run `agy-reader --watch` so it starts in lockstep with its daemons
instead of polling 24/7 — using only stock systemd, portable to any
systemd-based Linux.

A bare `agy-reader --watch` covers **every default session store that exists**:
the agy CLI's (`~/.gemini/antigravity-cli`) and Antigravity 2.0's
(`~/.gemini/antigravity`). Everything below therefore treats "activity" as
coming from either surface.

## The problem

Each surface's language-server daemon only listens while its host program
(`agy`, or the Antigravity 2.0 IDE) is running, and binds a **different
ephemeral port each launch**. A user service that runs `agy-reader --watch`
therefore has two failure modes at the edges of that lifecycle:

1. **Boot before agy.** If the unit starts (e.g. at login) before any agy daemon
   exists, the old code errored out of `requireDaemonURL` and exited non-zero —
   systemd marked the unit failed and never recovered.
2. **Idle forever.** A persistent poll keeps a process resident even when agy has
   been closed for hours, reacting no faster than `--watch-interval`.

Problem 1 is fixed unconditionally: `--watch` now starts in a *pending* state
when `ANTIGRAVITY_DAEMON_URL` is unset and the daemon isn't up yet, and
auto-discovers the port on a later tick (see `runWatch`/`watcher.tick`). The rest
of this doc addresses problem 2 with an **event-driven** deployment.

## Mechanism: systemd `.path` activation

A `.path` unit watches a filesystem location via inotify and starts a same-named
`.service` when it changes. It's present on every systemd host and needs no
machine-specific glue — exactly the portable trigger we want.

**Why not socket activation?** Socket activation requires systemd to *own the
listening socket* and pass the fd to the service. Here `agy` owns an ephemeral
port it chooses itself, and `agy-reader` is a *client*. Socket activation is
structurally the wrong tool.

### What signals "a daemon is running"

| Surface | Signal | Path | Fires when |
|---------|--------|------|-----------|
| CLI | New log file | `~/.gemini/antigravity-cli/log/` (dir) | agy's daemon starts — a fresh `cli-<timestamp>.log` appears and `cli.log` is re-pointed at it |
| CLI | Session writes | `~/.gemini/antigravity-cli/conversations/` (dir) | a transcript is being written (`.db`/`.db-wal`) — implies the daemon is up |
| IDE | Daemon logs | `$XDG_CONFIG_HOME/Antigravity/logs/` (dir) | Antigravity 2.0 starts its language server (`language_server.log` and friends are written) |
| IDE | Session writes | `~/.gemini/antigravity/conversations/` (dir) | an IDE transcript is being written |

For the CLI we watch the `log/` **directory** (not the `cli.log` symlink — a
replaced symlink is the unreliable inotify case) for "agy started", plus
`conversations/` so a sync re-arms on new transcript data after the service has
idle-exited. The IDE rows mirror the same two signals from Antigravity 2.0's
locations; on machines without the IDE those paths never exist and the watches
sit idle.

## The lifecycle knob: `--watch-idle-timeout`

```
--watch-idle-timeout DURATION   Exit --watch after the daemon stays unreachable
                                this long (0 = run forever; default 0)
```

When set, the watch loop exits cleanly (status 0) after **every watched root's**
daemon has been unreachable (or never discovered) for that long. Each root has
its own idle clock that **resets on any reachable tick**, so an active session
of any length never trips it — and one live surface keeps the process running:
with the IDE open and agy closed, the watcher stays resident (it is still
syncing IDE sessions; the agy root just waits). An idle-expired root is never
retired — it keeps polling and picks its daemon back up if it returns before
the other roots go idle. Combined with the `.path` trigger this gives a true
on-demand lifecycle: the service runs only while agy or the IDE is up (plus
the grace period), then exits and is relaunched on the next activity from
either. With the default `0` the loop runs forever, preserving the classic
always-on behavior.

## Install (recommended: event-driven)

Unit files live in [`deploy/systemd/`](../deploy/systemd). They use `%h`/`%E`
so they're host-agnostic; the only assumption is the default store layout
(`~/.gemini/antigravity-cli`, `~/.gemini/antigravity`, and the IDE's logs under
the user config dir), which is identical across machines.

```bash
# 1. Build/install the binary with the idle-timeout flag.
make build && install -m755 bin/agy-reader ~/.local/bin/agy-reader

# 2. Install the units.
install -Dm644 deploy/systemd/agy-reader.service ~/.config/systemd/user/agy-reader.service
install -Dm644 deploy/systemd/agy-reader.path    ~/.config/systemd/user/agy-reader.path
systemctl --user daemon-reload

# 3. Switch from an always-on service (if you had one) to the path trigger.
systemctl --user disable --now agy-reader.service   # stop the persistent poll
systemctl --user enable  --now agy-reader.path      # arm the trigger
```

`agy-reader.service` deliberately has **no `[Install]` section** — it is started
by `agy-reader.path`, not at boot.

### Verify

```bash
systemctl --user status agy-reader.path        # active (waiting)
# start an agy session (or the Antigravity 2.0 IDE), then:
journalctl --user -u agy-reader.service -f      # spins up, discovers daemon(s), syncs
# quit agy AND the IDE; ~5 min later the service logs the all-roots-idle line
# and exits 0.
```

### Rollback to always-on

```bash
systemctl --user disable --now agy-reader.path
# Drop --watch-idle-timeout from the ExecStart (or set it to 0), re-add
# [Install] WantedBy=default.target, then:
systemctl --user daemon-reload
systemctl --user enable --now agy-reader.service
```

## Options & trade-offs

| Option | Idle process? | Reaction | Stops with agy/IDE? | How |
|--------|--------------|----------|---------------------|-----|
| Always-on `--watch` | always | ≤ poll interval | no | service `WantedBy=default.target`, no idle-timeout |
| **`.path` → `--watch --watch-idle-timeout` (recommended)** | only while a surface is active (+grace) | instant on start | yes, once both are closed (clean exit) | the units in `deploy/systemd/` |
| `.path` → oneshot per event | never | instant per event | n/a | would need a `--watch-once` flag (not implemented) |

## Edge cases & notes

- **Trigger fires before the port is live.** The `.path` reacts to a log-file
  write that can precede the socket accepting connections. Handled: the watch
  loop retries discovery for several ticks before the idle timeout elapses, so
  pick `--watch-idle-timeout` as a comfortable multiple of `--watch-interval`
  (the default 5m vs 30s = 10 retries).
- **Chatty directories.** `log/` and `conversations/` see frequent writes during
  an active session; these are no-ops because systemd won't restart an
  already-running unit. After agy quits the dirs go quiet, so there's no busy
  loop (we use edge-triggered `PathModified`, not level-triggered `PathExists`).
- **agy restarts / new port.** Each restart writes a new `log/` file and rebinds
  a port; the trigger re-fires and the loop re-discovers — idempotent.
- **Fresh machine / no IDE.** If a watched path (`~/.gemini/antigravity-cli/log`,
  the IDE's logs dir, …) doesn't exist yet, systemd watches the nearest existing
  ancestor and triggers once it's created. Machines without Antigravity 2.0 just
  never fire the IDE triggers.
- **Chatty IDE logs.** The IDE writes its logs dir continuously while open —
  harmless for the same reason as the other chatty dirs: an already-running
  unit is not restarted, and the service is resident while the IDE is open
  anyway (its root is not idle).
- **Pinned URL.** Setting `ANTIGRAVITY_DAEMON_URL` disables auto-discovery; the
  idle-timeout still applies, but the event-driven units intentionally leave it
  unset.
