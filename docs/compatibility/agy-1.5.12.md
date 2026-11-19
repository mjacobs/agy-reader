# agy 1.5.12 compatibility

The reader verifies the 297-step release-review conversation and its 25-step child.
A 138-step fork also verifies. Its Markdown export identifies the original
conversation and keeps the entire multiline fork notice in a blockquote, including
its visible opening and closing tags.

## Storage and transcript changes

The SQLite schema still has user version 1, seven tables and the two step
indices. The ordinary RPC corpus gained 220 path/type entries relative to the
1.5.11 audit, with no removals. Of these, 218 describe nested planner
configuration in the agent script and static configuration. The other two are
`metadata.agentScript.hidden` and `steps.[].errorMessage.shouldShowModel`.
Optional feature coverage contributes to this comparison.

The [protobuf snapshot](../../artifacts/proto-snapshots/agy-1.5.12-94104f028e69/snapshot.json)
rechecked 405 record byte ranges against the installed executable. All 35
files in the trajectory and tools import closure compiled back to equivalent
descriptors. The comparison with 1.5.11 found 43 wire changes, zero JSON-only
changes and eight descriptor-only changes. In particular, `ResolvedSkill`
and `ResolvedSubagent` reserve their former fields 1–4 and use fields 5–6 for
builtin and custom definitions. Replacing the entire historical decoder would
lose those earlier field definitions. The production decoder retains its
historical definitions and refuses unknown persisted fields.

## Reader verification

A dedicated 1.5.12 daemon fetched and rendered 505 copied conversations.
Native verification against full and ordinary RPC passed for 493, plus ten
with previously declared source-only fields. All 503 verified offline exports
matched the ordinary RPC steps, generator metadata and executor metadata.
The corpus includes the API-key model-selection capture, the headless command
cancellation, the fork and three previously failing timestamped skill reads.

The reader now derives literal skill descriptions with their paragraph
breaks, retains append and overwrite requests on the observed file operations,
and accounts for foreground command cancellation. Subagent lists can carry
actual child step indices, including lists with 15 children. The ordinary
response also derives metadata for multi-task lists, cron schedules without
a daemon flag and timers interrupted by incoming messages.

Two conversations still refuse verification. One copied database has 936
steps and 293,797,435 bytes of executor metadata. Its ordinary RPC returned
zero steps, and the source exceeded the reader's 64 MiB limit. The other is
the historical 86-step conversation with a failed board post documented in
the [1.5.11 audit](agy-1.5.11.md). Both remain outside verified sync.

The CLI listed all four Gemini 4 Neon variants in an isolated API-key home.
Both remote launch flags exited with code 2. The model-selection capture
covers the request and error path. Successful paid generation, consumer
Auto-mode command review, transient network retries and Remote Control
browser interactions remain untested.
