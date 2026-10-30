# agy 1.4.11 compatibility

The reader now syncs and exports every conversation in the local agy 1.4.11
store. Before this audit, the newest real session was refused because it
overwrote existing files with `write_file` and had one failed `write_file`.
Conversations captured while a command is still running were refused too.
Markdown transcripts now say when a turn ended early.

The final sweep fetched, rendered and natively verified **499 of 499
conversations (21,403 steps)**, with no source-only exceptions. The SQLite
schema fingerprint and the sidecar shape fingerprint are both unchanged from
1.4.9. agy 1.4.10 was never installed on this machine; its changes are covered
only as far as 1.4.11 still carries them.

## Review of the Gemini research

| Finding | Assessment and change |
| --- | --- |
| Scratch files created without `Overwrite` fail projection | Confirmed, and already fixed in `72ae24a` before this audit. A real scratch write in the research session verifies against its daemon. The fix had not changed the completeness profile; the new `files1411` profile now covers it. |
| Running `run_command` steps are treated as canceled background commands | Confirmed. Two mid-run captures showed the daemon presenting the running step unchanged. The reader now accepts that shape, so a conversation syncs while a command runs instead of waiting for it to finish. |
| A terminating `PreInvocation` hook leaves the database and sidecar out of step | Not a hook defect. A fresh hook scenario reproduced a sidecar with zero steps for about four seconds after creation, while the database already had two. The next watcher pass wrote both steps. The attestation correctly reported the stale sidecar as changed source. The divergence persists only when the reader's daemon holds a stale copy (below). |
| Markdown omits turns that ended early | Confirmed. Transcripts now end with a note for canceled, hook-stopped, failed and limit-reached turns, from the executor's termination reason. Normal endings are not reported. |

The Markdown that the research quoted for the hook scenario does not match the
renderer's actual output. The real transcript uses a `Tool Execution Error`
warning block with collapsed model detail. That heading is unchanged.

The research's changelog summary is accurate. `finished_reason` is printed
only on stdout and is not stored in the database or sidecar.

## Gaps the research missed

- **Overwrites and failed writes.** The newest session had 24 `write_file`
  calls over existing files (payload kind `OVERWRITE`) and one call refused
  because the file existed. The daemon presents overwrites as file creations
  with `overwrite: true`, and the failed call as a creation with the error.
  Both are now qualified.
- **`IsDaemon: false` on completed commands.** The model passed this argument
  in the command scenario, and the modern command mapping refused it. It is
  now admitted only as `false`, repeated in the retained call.
- **Stale daemon copies.** An agy daemon loads each conversation once and keeps
  that copy. When a second agy process writes the conversation, the reader's
  daemon keeps serving the old copy, and verification refuses it. This caused
  the research session's sweep failure (358 steps served against 399 stored).
  It also caused the command scenario's stuck zero-step sidecar. Offline export
  is unaffected. Tracked as `agy-reader#3cwb`.
- **New stored fields.** The protobuf delta adds
  `CortexStepCheckpoint.captured_parent_messages`, from the 1.4.10 subagent
  compaction fix, plus `HookSystemMessage.render_info` and
  `RunCommand.trusted_cli_control_outputs`. None occurs in the 499
  conversations. A conversation that populates one will fail closed until it
  is qualified. Tracked as `agy-reader#7v0k`.
- **agy deletes old conversation databases.** The store held 500 databases at
  the start of the audit and 499 at the end. While two conversations were
  created and one agy started, the three oldest databases, from 2026-09-05,
  disappeared. Their sidecars remain, with source digests equal to the
  baseline sweep's, so the last verified transcripts survive; without a
  database they can no longer be re-verified.
  The store has 705 sidecars for 499 databases, so this pruning predates
  1.4.11. The cap and its trigger are inferred, not documented by agy. The
  [audit runbook](../format-audits.md) now warns about it.

## Native scenarios

Scenarios ran the installed agy 1.4.11 executable, SHA-256
`4341b80b43fc3b4437ff65041673f0f71c950e49d69a6256e3b6f212c4694ed4`, with
`agy -p` in disposable workspaces. The model executed the tools; no source
records or RPC responses were edited.

| Scenario | Steps | Observed result |
| --- | ---: | --- |
| `PreInvocation` hook returning `terminationBehavior: "terminate"` and an `errorMessage` | 2 | Exit code 3, `finished_reason: "error"`, executor reason `TERMINAL_CUSTOM_HOOK`. Prompt and error stored; verified. |
| Long foreground command, captured twice while running | 3, then 12 | The step stayed a generic `RUNNING` wrapper; task details appeared once agy moved the command to the background after 10 seconds. Both snapshots verified against the running daemon. |
| The same command after completion | 21 | Verified after the `IsDaemon` change. Before it, offline export refused the step. |

The research conversations served as further evidence: a `--print-timeout`
run (one stored step, executor reason `USER_CANCELED`), an earlier hook
scenario, a plain one-turn prompt, the 399-step research session, and the
488-step follow-up session with the overwrites. All seven conversations are in
the record's required set and verify in the final sweep.

That sweep used a freshly started agy daemon that had loaded none of these
conversations, so no stale copies were involved. The two daemons from the
research sessions were idle and left running.

Not exercised: a hook-injected system message with render information,
subagent compaction, `finished_reason: "interrupted"`, image downscaling, and the plugin and sign-in changes. No more scenarios were
run once the database deletions were noticed.

## Changelog review

The 1.4.10 and 1.4.11 notes contain three items that affect stored transcripts:
hook termination and injected errors, subagent compaction keeping parent
messages, and image size limits for requests. The first was exercised. The
second adds an unexercised checkpoint field. The third changes images sent to
the model; whether stored attachments change was not tested. Streaming, memory,
billing, plugin and `/browser` changes do not touch the stored format.

## Schema and validation

The [snapshot](../../artifacts/proto-snapshots/agy-1.4.11-4341b80b43fc/snapshot.json)
contains 390 extracted records and a 34-file verified closure and roundtrip.
The [1.4.9-to-1.4.11 delta](agy-1.4.11-proto-delta.json) has 13 additive wire
changes, no JSON-facing changes, and 11 descriptor-only deprecations. The
production schema is unchanged: no new field was observed in stored data, and
`overwrite` on file creation already existed. Earlier attestations require
reverification under the `files1411` profile.

Validation: `go test ./...`, `go test -race` on the changed packages, `go vet`,
`golangci-lint` (0 issues), and 161 maintenance tests (17 pidfd subtests
skipped, as in the 1.4.9 audit). New tests cover each qualified shape and its
offline export, altered presentations, and refusal of unobserved variants:
`Overwrite` false or missing, artifact overwrites, failed writes with
`Overwrite`, `IsDaemon: true`, and running commands with result text or extra
arguments.

Automatic review raised three findings. Two were fixed. The running-command
rule did not refuse `IsDaemon: true` when both copies of the argument agreed;
it now does, as the completed-command rule already did, and a test changes
both copies. Markdown also claimed that a failed overwrite had replaced the
file. The third finding, about a verification cache carried across an upgrade
within one profile, was dismissed: no deployed build ever held the looser rule.
A follow-up change makes Markdown name the file each `write_file` wrote, which
it previously did only for edit commands.

`COMPATIBILITY.md` was generated by the format-audit helper from clean commit
`5dc188d`, which contains all of these changes. Its sweep ran on verification
code identical to that commit's and verified all 499 conversations. Before
recording, the helper confirmed that each required conversation's stored
source still matched the sweep. The installed reader and its service still
run the previous build and profile until rebuilt and restarted. The AgentsView
consumer suite and live downstream ingestion were not tested. Raw RPC
captures, database backups and transcripts stay outside the repository; the
committed fixtures use synthetic values in the captured shapes.

## Follow-up: the scenarios the audit left untested

On 2026-10-30 three of the four behaviours from the changelog review were run
natively with agy 1.4.11 under a throwaway `HOME` (a symlinked credential and a
copy of `settings.json`), so that agy's pruning could not touch the real store.
The real store held 499 databases before and after. Each conversation was read
from its stored source and from both RPC responses of a daemon that had not
loaded it, and the reader's ordinary sync verification ran on the sandbox root
with `audit-sweep`. The fourth, subagent compaction, was not exercised.

| Behaviour | Captured | Reader verdict | Result |
| --- | --- | --- | --- |
| Hook injecting system messages with render information | A one-shot `PreInvocation` hook injected two system messages, one with `renderInfo` and one with `render_info` | Verified, no change | Neither render field is stored. The step keeps only `message` and `eventType` (`hook_json_PreInvocation`) in the source and both responses |
| `-p --output-format json` interrupted with SIGINT | Interrupted during generation, and interrupted during a `sleep` command | Both verified, no change | Result JSON has `status: "ERROR"`, `error: "interrupted"`, `finished_reason: "interrupted"`, exit code 1. The stored executor reason is `USER_CANCELED`, as for `--print-timeout`; the Markdown shows "Turns That Ended Early" |
| Image over 2000 pixels per side | A 3200x2400 PNG | Refused: `unsupported image read projection` | Stored as a 2000x1500 `image/jpeg` while the result text still names `image/png`. Qualified under `media1410` (see `docs/completeness.md`) |
| JPEG saved with a `.png` name | An 800x600 JPEG | Verified, no change | Stored unchanged as `image/jpeg`; text and media both name the JPEG |
| Subagent context compaction | Not run | Not tested | No cheap trigger found; see below |

What the hook result does not show: agy accepted the hook (the injected text is
stored), but nothing in the stored data, either response, the agy logs or the
conversation's `brain/` directory shows whether it parsed the render fields or
dropped them. Only the two key spellings above were tried. The finding is
"not stored", not "not parsed".

The interrupt that landed before any model output stored only the user step,
with executor reason `USER_CANCELED`; the one that landed during a command
stored the planner response and no tool step. A command that was already
running when it was interrupted was not captured.

Subagent compaction: `agy --help`, `settings.json` keys, the changelog and the
binary's strings show `/compact` as the only user-facing trigger and no setting
that lowers the automatic threshold (a `compaction_threshold` field exists in
the harness configuration but no setting key reaches it). Reaching the
automatic threshold would take a very large context, and `/compact` needs an
interactive session with a subagent focused. Neither was tried, so
`CortexStepCheckpoint.captured_parent_messages` stays unqualified.

Two lessons for the next capture. A hook that injects on every invocation makes
the model answer each injection: the first attempt ran 29 invocations and
91 steps, and the unconstrained agent read files outside the sandbox
(read-only; nothing was written). Make hooks inject once, and confine the
agent with a `PreToolUse` denial hook. That run's conversation is also refused,
for an unrelated reason: a `run_command` that moved to the background and was
then ended with `manage_task` kill is stored with status `CANCELED`, which no
rule qualifies. It is tracked separately.

Validation: `go test ./...`, `go vet ./...`, `golangci-lint run` (0 issues) and
`make maintenance-test` (161 tests, 91 skipped) pass with the `media1410` change.
`COMPATIBILITY.md` is not re-recorded: that needs a sweep of the real store
through a freshly started real-`HOME` agy, which can cost another old database.
