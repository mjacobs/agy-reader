# agy 1.4.12 subagent-compaction qualification

The reader now verifies and exports a subagent that agy has compacted. When agy
compacts a subagent it stores the parent agent's recent messages in the
checkpoint (`CortexStepCheckpoint.captured_parent_messages`, field 31). Before
this change the reader refused such a conversation for sync and offline export
because it did not know the field. One native conversation was captured, refused
by the previous reader, and verifies with the new profile suffix
`parentmsgs1410`.

This closes the subagent-compaction scenario left open in the
[1.4.11 follow-up](agy-1.4.11.md). It does not re-record `COMPATIBILITY.md`,
which still names agy 1.4.11, and it does not install the reader. Fields
`trusted_cli_control_outputs` and the two 1.4.10 configuration fields remain
unseen (`agy-reader#7v0k` stays open).

## Native case

| Item | Value |
| --- | --- |
| agy | 1.4.12, SHA-256 `1edfb30477447825f0aaa05fd9a91d82f9adf26686f41cd4f5c70b90abe383e8` |
| Scenario | A parent agent (model `gemini-3.8-flash-high`, sandbox `HOME`) started one subagent and sent it three more messages with `send_message`; the subagent answered each |
| Compaction | `--conversation <subagent> -p "/compact"` in print mode, first and only attempt |
| Subagent steps | 8 before, 11 after: a user input, a type-101 step and a checkpoint (`CORTEX_STEP_TYPE_CHECKPOINT`) were appended |
| `capturedParentMessages` | 4 entries, text only, none with media. Each is one string of the form `[Message] timestamp=… sender=<parent id> priority=MESSAGE_PRIORITY_HIGH content=<the message>` |
| `capturedUserRequests` | One entry: `/compact` |
| Default and full RPC | Identical files (SHA-256 `a2ae87c8…`), both carrying the four messages. The stored source decodes to the same four |

The messages arrived at the subagent as `PARENT-MSG-1` to `PARENT-MSG-4`; the
first is the task text the parent gave it, so the checkpoint keeps the whole
exchange. Because only four messages existed, the reported cap of ten recent
messages was not observed.

## Verdicts

Reader at `main` before this change, run through `audit-sweep` against a daemon
that had not loaded the conversation, and `export-offline` on a copy of the
stored source:

| Conversation | Before | After (`parentmsgs1410`) |
| --- | --- | --- |
| Compacted subagent (`7941bc23`) | Not verified: `unknown persisted protobuf fields`; offline export fails with the same reason | Verified; offline JSON keeps 4 parent messages; Markdown shows them under "Captured Parent Messages" |
| Its parent (`07e99ce2`) | Not verified: `unsupported generic result` | Unchanged, for a reason unrelated to compaction (next section) |
| Same scenario, rerun, uncompacted (`c80589bb` parent, `5aa123e2` subagent) | Verified | Verified |

The qualification adds the field to the embedded schema, a `Checkpoint` field
and a Markdown section, and no projection rule: the stored source, default RPC
and full RPC present the field the same way. The overlay generator is
`internal/completeness/generate_parentmsgs1410.py`, chained from
`generate_media149.py` and pinned to the 1.4.12 snapshot; its delta over the
previous bundle is exactly that one field. Attestations minted under the
previous profile read as unknown.

## What the sandbox itself caused

The parent `07e99ce2` was refused only because the confinement hook used for the
capture denied one `manage_task` call (step 4, a `GENERIC` step with status
`ERROR`). The rerun with that tool allowed verified. A denied `read_file` call
in the smoke-test conversation (`a9debf4e`) is refused the same way. Denying
those tools with a `PreToolUse` hook is something real users can do, so it is
tracked as `agy-reader#gs51` and is not part of this change. `agy-reader#azya`
already covers a denied `run_command`.

## What the capture showed about the method

- **Trigger.** `/compact` sent to a subagent's conversation with `--conversation`
  works in print mode. The tmux and RPC fallbacks were not needed. agy's reply
  to the command was a replay of the subagent's earlier answers, not a statement
  that it compacted, so the stored `steps` table is the evidence.
- **Argument order.** On 1.4.12, `-p` takes the next argument as the prompt, so
  `agy -p --output-format json "…"` fails; put `-p "<prompt>"` last.
- **Confinement.** `allowNonWorkspaceAccess: false` did not stop a sandboxed
  agent from reading `/etc/hostname`. A `PreToolUse` hook that allows only
  subagent and message tools did. The hook, not the setting, is what confined the
  capture.
- **Quota.** The larger worker model returned HTTP 429 ("Individual quota
  reached") for full-size prompts while accepting one-word prompts; the
  qualification code was drafted by the smaller model after three retries.

## Not tested

- Parent messages that carry media. The type permits it and synthetic tests
  cover it, but no native conversation had any.
- More than four captured messages, including the stated cap of ten.
- Automatic compaction (agy compacting a subagent on its own when the context
  grows). Only the manual `/compact` command was used.
- Whether the field appears for any conversation other than a subagent.
- Other differences between 1.4.11 and 1.4.12. The snapshot delta
  ([`agy-1.4.12-proto-delta.json`](agy-1.4.12-proto-delta.json)) has one wire
  change, a new `code_foreground` text field on `PluginThemeSeeds`
  (field 5), which the reader does not store anywhere.

## Validation

In the worktree that became `main`: `go test -count=1 ./...` passes,
`go vet ./...` passes, `golangci-lint run` reports `0 issues`, and
`make maintenance-test` passes (164 tests; 94 skipped under the system
`python3`, 17 with the protobuf environment). The six new Go tests all fail
against the previous `schema.pb.gz` and pass against the new one. Private
captures (stored sources, both RPC responses and their hashes, sweep output, the
coordination log) are in
`~/.local/state/agy-reader/audits/2026-10-30-agy-1.4.12-subagent-compaction/`,
outside the repository; committed fixtures are synthetic.
