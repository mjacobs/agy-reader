# agy 1.2.17 format and feature audit

Conversation labels such as `conversation-001` are consistent aliases across
these reports, not session IDs. Use actual IDs from your own store when
rerunning commands.

## Status on 2026-10-07

The findings below describe the reader as it was on 2026-10-05. Every issue
this audit filed has since been closed. On 2026-10-07 the reader on `main`
(commit `8789ae6`) verifies all 16 scenario conversations: `audit-qualify`
accepts each one's sidecar attestation against its current source.
`COMPATIBILITY.md` still records 1.2.16, and no `--record` has been run.

A rerun on 2026-10-07 covered the two cases this audit could not exercise:

- **A rejected permission request fails reader sync.** A user rejected one
  command in a parent conversation and one in a subagent, and a headless run
  denied a subagent's command on its own. All three conversations are
  refused; `agy-reader#xyfe` covers the step shape. See
  [Rejected permission requests](#rejected-permission-requests-2026-10-07).
- **`Send Immediately` is still untested.** It needs an interactive session;
  the steps are in
  [Send Immediately: steps to run by hand](#send-immediately-steps-to-run-by-hand).

[Where the scenarios stand](#where-the-scenarios-stand) lists every scenario
with its result, and the conversations to name in the next `--record`.

## Findings on 2026-10-05

The 2026-10-05 audit found that **the reader cannot sync or render any
conversation written by agy 1.2.17**. Every new conversation stores one
configuration field that the reader's schema does not define, and the reader
refuses a source with an undefined field. `COMPATIBILITY.md` stays at
**1.2.16**. This is a failed qualification; no reader code was changed.

The audit also ran scenarios drawn from the changelog back to 1.2.5. A
diagnostic build with the 1.2.17 schema shows what remains once the schema is
refreshed: 7 of the 16 scenario conversations verify, and 9 fail on five kinds
of step. One scenario, forking a conversation, stops `audit-sweep` from running
on the store at all.

The [sanitized evidence](agy-1.2.17-evidence.json) records exact results. The
[shape snapshot](agy-1.2.17-sidecar-shape.tsv) is an observation, not an
accepted baseline. Raw conversations, prompts, fixtures and logs are private,
in `~/.local/state/agy-reader/audits/2026-10-05-agy-1.2.17/`.

## What needed action on 2026-10-05

All of these issues are now closed; see the status above.

| Finding | Effect today | Issue |
| --- | --- | --- |
| New configuration field `RunCommandToolConfig.enable_plugin_auth` | Every 1.2.17 conversation is refused; transcripts stop updating | `agy-reader#8mnx` |
| `/fork` leaves a `.pb` and a `.db` for one conversation | `audit-sweep` refuses the store; the watcher retries the fork on every tick | `agy-reader#3g5w` |
| Command that ran after a recorded permission request | Conversation refused | `agy-reader#pzg7` |
| Hook-blocked command; image request with invalid arguments | Conversation refused | `agy-reader#azya` |
| Answered `ask_question` step | Conversation refused; transcript omits questions and answers | `agy-reader#3pq4`, `agy-reader#vyb0` |
| Failed `view_file` on a pipe or device | Conversation refused | `agy-reader#2xqw` |
| agy stopped while a command ran | Conversation refused | `agy-reader#vx0f` |

The last five are masked until the first is fixed.

## The schema change

The helper's SQLite check passes: user version 1, seven tables, two step
indexes, fingerprint unchanged. The drift is inside the protobuf records.
Comparing the 31 descriptor files the reader embeds, as extracted from the
1.2.16 and 1.2.17 binaries, gives two added fields and nothing removed or
changed:

- `exa.cortex_pb.RunCommandToolConfig.enable_plugin_auth`, field 23, optional
  bool. agy stores it as `false` in each conversation's generator and executor
  metadata, at `planner_config.tool_config.run_command`.
- `exa.cortex_pb.WorkflowSpec.depot_path`, field 19, string. No conversation
  carried it.

A survey of all 281 SQLite conversations found field 23 in the 16 written
after the update and in none before. A conversation with no tool calls has it
too, so the refusal does not depend on what the session did.

The live effect is a transcript that stops partway. A watcher syncs the first
steps while a turn runs, then refuses the source once the metadata record
appears. Seven of the 16 scenario conversations have such a partial sidecar,
for example 1 of 4 steps for a headless run and 3 of 17 for the interactive
one; the other nine have none. `agy-reader <id>` exits 1 with `completeness
unknown: unknown persisted protobuf fields`. `doctor` reports 134 unknown SQLite sources and
exits 1.

## Changelog review and scenarios

`agy changelog` for 1.2.17 declares no storage, encryption or RPC change. Only
the TIFF note concerns stored step content; the announcement cards, Windows
sandbox and table alignment notes are display or platform changes. The audit
also covered notes from earlier releases that previous audits listed as
untested.

"Verified" below means the diagnostic 1.2.17-schema build verified the
persisted source against full and ordinary RPC. The current reader refuses all
of them.

| Release note | Scenario | What agy did | Reader with 1.2.17 schema |
| --- | --- | --- | --- |
| 1.2.17: `.tiff` no longer mislabelled or converted to PNG | View a 1200 × 800 TIFF whose labels the prompt did not reveal | Read all four labels. Step media type is `image/tiff`; the stored media file is still a TIFF of the fixture's size | Verified; Markdown shows the media type |
| 1.2.17: table alignment with emoji and combining characters | Response table with ⚠️, 👍🏽 and Devanagari text | Stored and rendered unchanged | Verified (same conversation) |
| 1.2.17: announcement cards | Interactive start | A deprecation card appeared; left undismissed. No stored content observed | Not applicable |
| 1.2.16: killed background commands report an exit code | Two background tasks killed from outside agy, with SIGKILL and SIGABRT | Reported exit 137 and 134 and continued | Verified |
| 1.2.16: manifests in parent `.agents/` directories | `skills.json` and `rules.json` at a project root; session started two directories below | Loaded the skill and the rule | Verified |
| 1.2.16: image requests go to the `image-generator` subagent | Image with aspect ratio 37:1 | Tool validation error; no retry | Parent verified; child refused, status `INVALID` with no result |
| 1.2.16: queued messages | Two messages queued behind a running turn | Delivered as one user input holding both lines | Step accepted; rendered correctly |
| 1.2.15: a denied request is respected, not worked around | Workspace hook denies one command, in a parent and in a subagent | Reported the denial and moved on | Refused: failed `run_command` with no result |
| 1.2.15: permission requests | Workspace hook forces a request; the `always-proceed` setting approved it | Command ran | Refused: step carries `permissions` and `completed_interactions` |
| 1.2.14: pipes, devices and truncated media rejected up front | `view_file` on a FIFO, `/dev/null` and a truncated MP4 | Three immediate errors | Pipe and device refused; MP4 accepted |
| 1.2.8 fork fix, 1.2.6 rewind fix | `/fork`, then `/rewind` in the fork to before a file edit, then a new message | Fork went from 25 steps to 13, then 15; the edited file was removed | Refused on the copied `ask_question` step |
| 1.2.7: `ask_question` prompt | Two questions; one option chosen, one written in | Answers returned to the agent | Refused; transcript shows no details |
| 1.2.6: diffs over 1 MiB keep statistics only | Replace one token inside a single 1.7 MiB line | Edit succeeded. The step is 3.5 MB and keeps the content, although the configured `maxStoredDiffBytes` is 1,048,576 | Verified; the Markdown is 3.5 MB |
| 1.2.5: commands killed by an outside signal finish as canceled | SIGTERM to agy while a foreground command ran | Last step is `CANCELED` over a native step still `RUNNING` | Refused |
| Control | Prompt with no tool calls | Replied | Verified |

Two behaviors outside the changelog window turned up while testing. `/btw`
asks a side question during a turn; agy stores it as a separate conversation
of type `CORTEX_TRAJECTORY_TYPE_SIDE_QUESTION` holding a copy of the history
so far plus the question and answer. Forks and side questions record their
origin in `parentReferences` with a `FORK` reference type. The reader keeps
those fields in the sidecar and does not use them, so each appears downstream
as an independent conversation that repeats its parent's history.

## Forks break the sweep

agy wrote `<id>.pb` when the fork was created and `<id>.db` when the fork
continued, and left both. The reader lists each file as a session.
`audit-sweep` rejects a duplicate ID before fetching anything, so it exited 1
on the real store. Three running watchers each logged a failed sidecar write
for the fork on every tick.

To get corpus results, the sweep ran over a symlinked view of the store
without that one `.pb`. This is a workaround for the audit, not a claim that
the store sweeps cleanly.

## Corpus results and fingerprints

Over that view, both sweeps selected, fetched and rendered **296
conversations, 26,083 steps, zero failures**.

| Reader | Verified | Verified with source-only fields | Not verified | Legacy |
| --- | ---: | ---: | ---: | ---: |
| Current (commit `d493936`) | 98 | 49 | 134 | 15 |
| Diagnostic, 1.2.17 schema | 105 | 49 | 127 | 15 |

The helper, given all 16 scenario conversations in `AGY_QUALIFY_SESSIONS`,
exited 1 with `Reader sync: FAIL` and produced no record. The reader unit
suite passed.

- Schema: `sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`
  (unchanged).
- Shape: `sha256:3f53bfb844d0e3f3c1454df8767494c33175885487ced51aaaf2422e03014cc8`
  over 296 responses.

Against the recorded 1.2.16 snapshot the shape union adds 61 path/type entries
and removes none. They cover fork references, permission requests and hook
results on steps, and configuration paths including `enablePluginAuth`. Most
come from features these scenarios exercised for the first time, so the delta
does not by itself show daemon drift.

## Process and provenance

Every agy process was started by `scripts/agy-pair.py` in its default token
mode, which generates the CSRF token, confirms it with an authenticated RPC
and pins the paired reader to that agy. Ten headless runs and one interactive
session used `gemini-3.8-flash-high` at high effort, in disposable
directories. No permission-bypass flag was passed; the machine's agy settings
already use `toolPermission: always-proceed` with the terminal sandbox on.

The installed agy, the interactive scenario process and the inspection process
all have SHA-256
`c54ef90651a8646ae67334d39212c81f5946feec373ad6aa335f9ef401662bc5`. The
inspection pair was started after every scenario had finished. Its endpoint
returned 401 without the token and 200 with it.

The reader was built from commit
`d49393666bb5b6aed9080aadabb8738f7ac668c9`; the only untracked path was the
pre-existing `docs/handoffs/`. The diagnostic build replaced the embedded
schema through a Go build overlay and used a profile name marked as
diagnostic. It ran `audit-sweep`, which writes no sidecars, and package tests.
It was not installed. The descriptor extractor was checked against the
retained 1.2.16 binary: it reproduces the embedded bundle's files, fields and
enum values, though 4 of 31 files differ in other bytes.

Paired readers watched the normal CLI store, as did a pre-existing pair and
the installed service, which were left running. agy added the interactive
scenario's directory to its trusted workspaces; that entry was removed and the
settings file is byte-identical to its state before the audit.

## Not exercised

- A permission request that the user rejects, or one a subagent cannot ask
  for. Both need a `toolPermission` setting other than `always-proceed`, which
  is global. Exercised on 2026-10-07; see
  [Rejected permission requests](#rejected-permission-requests-2026-10-07).
- The `Send Immediately` option for queued messages, also a global setting.
  Still untested; see
  [Send Immediately: steps to run by hand](#send-immediately-steps-to-run-by-hand).
- Automatic context compaction, Remote Control, Windows behavior, quota and
  credit errors, and dismissing an announcement card.
- The agentsview consumer suite and live downstream ingestion.
- Full raw RPC responses for the scenario conversations were compared by the
  sweep but not retained. A fixture needs a fresh fetch from a 1.2.17 daemon;
  the conversations are still in the store.

The headless runs reported 288,873 input, 20,835 output and 496,491 cache-read
tokens in total; the interactive session's usage was not captured and cost is
not exposed. No audit subagents were launched.

## Rejected permission requests (2026-10-07)

agy respects a rejected request, but the reader refuses every conversation
that holds one. The rejected command is stored with no result and with a
record of the request and its refusal, and the completeness check accepts
neither. `agy-reader#xyfe` covers the shape. A parent conversation whose own
steps are clean still verifies.

### How it was run

- agy 1.3.1, the installed version (SHA-256
  `ce1bdaed3201bb84f35d69d2773caec4f18af52af00e8c25f6cace07e4359615`); the
  1.2.17 binary was no longer installed. The reader was built from `main` at
  `8789ae6`, profile
  `sqlite-v1-format0-agy-1.3.0-19be6af38f7b-planner50-permission2-prompt23`.
- Every agy process ran with `HOME` set to a throwaway directory and was
  started by `scripts/agy-pair.py run` in its default token mode, inside a
  private tmux server for the interactive parts. That directory held a copy
  of the normal `settings.json` with only `toolPermission` changed, from
  `always-proceed` to `request-review`. agy rewrote the copy without the key,
  since `request-review` is the default, and added the scenario directory to
  its trusted workspaces when the trust prompt was answered. Copying the
  normal `cache/onboarding.json` skipped the first-run screens. No hooks,
  plugins, MCP servers or credentials were copied; agy signed in through the
  system keyring.
- The normal agy configuration did not change. The SHA-256 and modification
  time of `~/.gemini/antigravity-cli/settings.json` (`2278d0636ca4…`), its
  `mcp_config.json`, `~/.gemini/settings.json`, `~/.gemini/trustedFolders.json`
  and every file directly in `~/.gemini/config/` were the same before and
  after.
- Each scenario was frozen after agy exited. Only then was a fresh
  inspection pair, an idle agy and its watcher in the same throwaway
  directory, started: once after the interactive run and once after the
  headless run. Each answered 401 without the token and 200 with it, and
  each logged version 1.3.1. SQLite, full RPC and default RPC agreed on every
  scenario conversation's step count, and the live databases still matched
  the frozen copies after the first inspection pair.
- Model `gemini-3.8-flash-high` at high effort.

### Scenarios

| Scenario | Conversation | What agy did |
| --- | --- | --- |
| Interactive parent | `conversation-188`, 12 steps | Asked to run `echo AUDIT_REJECT_PARENT_1`; the user chose `No, cancel`. agy showed `User declined the tool call` and ended the turn with `Interrupted · What should Antigravity CLI do instead?`. A follow-up message asked for the subagent step. When the subagent finished, the parent did not resume on its own; a third message got a report quoting both denials. |
| Interactive subagent | `conversation-017`, 3 steps | Asked to run `echo AUDIT_REJECT_SUB_2`. The TUI showed `self needs approval for Bash`; the user opened it with `Alt+J` and chose `No, deny`. The subagent stopped without retrying. |
| Headless parent | `conversation-196`, 4 steps | `-p` run. Invoked one subagent, ended its turn with "Waiting for its report", and the run exited 0 without relaying a result. |
| Headless subagent | `conversation-110`, 3 steps | Asked to run `echo AUDIT_HEADLESS_SUB_3`. Nobody could answer: agy's log records `print mode, soft-denying` for step 2 as the run ended. The step is stored as a user denial. |
| Control | `conversation-151`, headless, no tools | Replied `ok`. |

The denial text agy returns to the model tells it not to work around the
denial, and no workaround was attempted, as the 1.2.15 change intends.

### Reader results

| Conversation | `TestNativeEvidence` on the frozen copy | Fresh watcher | `--sync` | Sidecar equals default RPC | `audit-qualify` |
| --- | --- | --- | --- | --- | --- |
| `conversation-188` | fails, `unsupported generic wrapper` | skipped, same reason | exit 1 | no sidecar | not verified |
| `conversation-017` | fails, `unsupported generic wrapper` | skipped, same reason | exit 1 | no sidecar | not verified |
| `conversation-110` | fails, `unsupported generic wrapper` | skipped, same reason | exit 1 | no sidecar | not verified |
| `conversation-196` | passes, 4 steps | synced and verified | exit 0 | equal, for the watcher's and for `--sync`'s | verified, from the sweep manifest and from the live attestation |
| `conversation-151` | not run | synced and verified | not run | not compared | verified by the sweep |

`audit-sweep` over the throwaway store fetched and rendered all six
conversations without failure (manifest complete) and verified 2 of 6.

### The refused step

Projecting each step on its own shows one refused step per conversation,
step index 2 in all three; every other step is accepted. The three refused
steps have the same shape:

- A `run_command` call stored as a generic wrapper with status `ERROR`, its
  arguments, an outer error and no result. The error starts
  `permission check failed for command "<command>": user denied permission to
  run command:` and tells the model not to circumvent the denial.
- A `completed_interactions` record holding the request (resource `command`
  with the command as target, trigger `TYPE_DEFAULT_FALLBACK`, a suggested
  `echo` pattern, shell `bash`) and an answer with no `allow` and empty
  `persistGrants`. The step has no `permissions` grant.
- One `allow` pre-tool hook result, although no hook was configured.
- The default RPC presents it as a `RUN_COMMAND` step built from the
  arguments, keeping the status, the error and the interaction record.

`agy-reader#azya` accepts a similar step for a command that a hook denied,
but only with the hook's error text, one `deny` hook result and no
interaction record. `agy-reader#pzg7` accepts an interaction record only for
an approved request raised by a `force_ask` hook, and refuses a rejected
answer by design. Neither covers this shape; `agy-reader#xyfe` does, with the
full field list.

An aborted first attempt left one more conversation,
`conversation-114`. agy was stopped while a permission
prompt was open (hidden behind the first-run screens), and the `run_command`
is stored as a `CANCELED` wrapper with arguments only. The reader refuses it
with `unsupported generic fields`. `agy-reader#xyfe` records it as a related
shape.

### Rendering

Because the reader refuses these conversations, `agy-reader <id>` prints
nothing and exits 1. Rendering the default RPC response directly, through an
audit-only test overlay, shows each rejected step as a failed `RUN_COMMAND`
with the denial as a warning, so the behavior will show on the step once the
shape is accepted. Two gaps remain:

- The step shows `Exit Code: unknown` and an `Output` block holding `{}`,
  which reads as output from a command that never ran. Hook-denied steps,
  accepted today, render the same way. `agy-reader#q6qb` covers this.
- The request and its answer are not shown. The README records this as
  deliberate: the transcript's step view does not carry the interaction
  record.

The headless parent `conversation-196` renders its subagent call with the child's
conversation ID.

Raw evidence (prompts, frozen databases, RPC responses, logs, per-step
diagnostics and rendered Markdown) is private, in
`~/.local/state/agy-reader/audits/2026-10-07-agy-1.3.1-4yv9/`. The headless
runs reported 39,285 input and 848 output tokens in total; the interactive
session's usage was not captured.

## Send Immediately: steps to run by hand

With `Queued Messages` set to `Send Immediately`, a message typed during a
turn should reach the agent before the turn ends. agy 1.2.16 says a message
sent while earlier ones are still waiting goes to the agent together with
them, in order. agy 1.3.1 fixed such messages waiting for the end of the turn
when the agent had already written a task list or plan in that turn. The
check is that the messages arrive mid-turn in the order typed, and that the
stored user-input steps keep that order in the sidecar and in Markdown.

Setup, in a terminal. It uses its own throwaway `HOME`, so the normal
configuration is not touched:

```bash
cd ~/dev/projects/agy-reader && make build
T=~/.local/state/agy-reader/audits/send-immediately-$(date +%F)
mkdir -p "$T/home/.gemini/antigravity-cli/cache" "$T/ws" "$T/logs"
cp ~/.gemini/antigravity-cli/settings.json "$T/home/.gemini/antigravity-cli/"
cp ~/.gemini/antigravity-cli/cache/onboarding.json "$T/home/.gemini/antigravity-cli/cache/"
chmod -R go= "$T"
sha256sum ~/.gemini/antigravity-cli/settings.json
cd "$T/ws"
env HOME="$T/home" /usr/bin/python3 ~/dev/projects/agy-reader/scripts/agy-pair.py run \
  --log-dir "$T/logs" -- --model gemini-3.8-flash-high --effort high
```

`/usr/bin/python3` bypasses the mise shim, which refuses to run under another
`HOME`. Copying `onboarding.json` skips the first-run color and terms screens,
which the normal profile has already completed.

In the TUI:

1. At the trust prompt, choose `Yes, I trust this folder`. The banner should
   show `Antigravity CLI 1.3.1` or later; note the version. Dismiss any
   announcement card with `Esc`.
2. Type `/config`, move to `Queued Messages`, press `→` until it reads
   `Send Immediately`, then press `Esc`. In a second terminal, with the same
   `T`, `jq -r .queuedMessages "$T/home/.gemini/antigravity-cli/settings.json"`
   should print `send-immediately`.
3. Send this prompt as one line:

   ```text
   Queue test. First write a short task list artifact with the three steps below. Then run each step as a separate foreground shell command and wait for it to finish, updating the task list after each: 1) sleep 30; echo SI_STEP_1 2) sleep 30; echo SI_STEP_2 3) sleep 30; echo SI_STEP_3. If a message from me arrives while you work, follow it at once. End with the line SI_DONE.
   ```

   Expect a task list to appear, then the first command to start.
4. While the first command is still running, send these three messages one
   after another, pressing Enter after each, within about ten seconds:

   ```text
   SI_MSG_A: include the word ALPHA in your next reply
   SI_MSG_B: include the word BRAVO in your next reply
   SI_MSG_C: skip step 3 and finish after step 2
   ```

5. Watch where they go. With `Send Immediately` working:
   - None of the three waits in the queue above the prompt until the turn
     ends. They appear in the conversation in the order A, B, C before the
     second command starts, as one user message with three lines or as three
     messages. The changelog does not say whether the running command is
     interrupted or allowed to finish; note which happens.
   - The agent's next reply or thought mentions ALPHA and BRAVO, step 2 runs,
     step 3 does not, and the turn ends with `SI_DONE`.
   - If the messages stay queued and appear only after the turn ends, or
     step 3 runs, `Send Immediately` did not apply. Note which.
6. Exit with `/exit` and copy the conversation ID from the
   `agy --conversation=<id>` line.
7. Run `sha256sum ~/.gemini/antigravity-cli/settings.json` again. It should
   print the same hash as during setup.

Report the conversation ID, the version in the banner, where A, B and C
appeared relative to the commands and whether they were grouped, whether
step 3 ran, and anything left in the queue.

Afterwards, with agy exited, the conversation gets the same reader checks as
the rejected requests: freeze `<id>.db` from
`$T/home/.gemini/antigravity-cli/conversations/`, start a fresh idle pair in
the same `HOME`, then run `TestNativeEvidence` on the copy, `audit-sweep`,
`--sync`, a fresh watcher, the sidecar comparison with the default RPC and
`audit-qualify`. In the sidecar and in Markdown, the user input holding A, B
and C must appear in that order, after the first command step and before the
reply that mentions ALPHA.

## Where the scenarios stand

This combines the 2026-10-05 audit, the 2026-10-07 rerun and the reader on
`main` at `8789ae6`. The 1.2.17 conversations were checked on 2026-10-07 with
`audit-qualify` against their live sidecar attestations in the normal store;
the rerun's conversations went through the fresh-daemon checks above.

Pass:

| Scenario | Conversations |
| --- | --- |
| A hook denies a command, in a parent and in a subagent (1.2.15) | `conversation-117`; `conversation-156` and its subagent `conversation-067` |
| A hook forces a permission request, which is approved (1.2.15) | `conversation-117` and its subagent `conversation-068` |
| Image request with an invalid aspect ratio (1.2.16) | parent `conversation-183`, `image-generator` child `conversation-094` |
| Queued messages in the default `Queue` mode (1.2.16) | `conversation-168` |
| Manifests in a parent `.agents/` directory (1.2.16) | `conversation-186` |
| Background commands killed from outside with SIGKILL and SIGABRT (1.2.16) | `conversation-019` |
| Headless parent whose subagent's request was denied | `conversation-196`, in the rerun's throwaway store |

Covered by an open qualification issue, refused today:

- A rejected request, in a parent, in a subagent, and denied automatically
  for a headless subagent: `conversation-188`, `conversation-017` and `conversation-110`, all
  `agy-reader#xyfe`.
- A command canceled while its permission prompt was open: `conversation-114`,
  recorded in `agy-reader#xyfe` as a related shape.
- The `{}` output block on a command that never ran, a rendering gap only:
  `agy-reader#q6qb`.

Untested:

- `Send Immediately`, with the steps above.
- A `permissions.deny` rule in `settings.json`. The audit denied commands
  with a workspace hook instead, and the rerun changed only `toolPermission`.
- The other items under [Not exercised](#not-exercised).

### Conversations for the next `--record`

Use the actual session IDs corresponding to these aliases in
`AGY_QUALIFY_SESSIONS` for the next `audit_format.sh --record`
over the normal store: the passing scenario conversations above, plus the
agy 1.2.16 image parent `conversation-096` and its
`image-generator` child `conversation-203`, which also
verify. Pass the list on the helper's own command line, as below; a separate
unexported assignment does not reach the helper, which would then gate the
record on the newest conversation alone. `$work` is the fresh `audit-sweep`
output described in [the audit runbook](../format-audits.md).

```bash
AGY_QUALIFY_SESSIONS="<actual-scenario-id> <actual-child-id> ..." \
AGY_SIDECAR_CORPUS="$work/sweep/corpus" \
  skills/agy-format-audit/scripts/audit_format.sh --record --corpus-swept
```

The rerun's conversations, including the passing headless parent, live in
its throwaway store, which a record of the normal store cannot name. So
would a `Send Immediately` conversation run with the steps above. Covering
them takes a record made over that store, or a rerun in the normal store.
