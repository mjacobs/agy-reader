# agy 1.5.5 compatibility

All four release-exercise conversations now pass native verification against
full and displayed daemon responses. Their offline JSON matches displayed RPC
exactly, and ordinary sync writes complete sidecars in an isolated store. The
coordinator has 270 steps, the swarm worker seven, and both workspace children
six. The renderer nests all three children beneath their matching creation
steps. No production reader was installed or restarted.

## Reader changes

The `tools155` profile adds the captured completed swarm calls: `swarm` status,
models, title, add and kill; `read_board` with either since or limit;
`post_message`; and `swarm_wait`. Their entire generic wrapper, result text,
output reference and render information remain unchanged. Unsupported actions,
argument sets, workspace modes, errors and extra payload fields still fail
closed. Retained tool-call arguments must agree with stored generic arguments.

Subagent invocations now accept `Workspace=share` and `Workspace=branch` as
well as `inherit`, retaining returned workspace URIs rather than deriving paths.
Two-child list and kill results now derive the same metadata as the existing
single-child mapping. IDs must be unique and match the ordered child states;
list states must carry index -1, and kill states must omit it. Kill arguments
must name the same ordered IDs. Other counts remain unqualified.

After those blockers were removed, the coordinator exposed an empty task list
and two timestamp-prefixed skill reads. The empty list adds only `action: list`
metadata. Skill name and description are derived from saved numbered plain
frontmatter after two valid timestamp lines; the original output stays intact.
No current skill file or incoming RPC value supplies that metadata. Historical
presentation alternatives remain available. No descriptor or source-only
exception changed; old attestations must be reverified under the new profile.

Child placement now uses returned conversation IDs for native invocations and
successful swarm-add results. It only places children already linked by the
resolver. Multiple returned children can nest under one invocation. Explicit
IDs are reserved before the legacy chronological fallback runs; unmatched
children remain visible in the trailing section. The fallback is disabled for
parents containing swarm calls, preventing a worker from being attached to an
unrelated legacy invocation.

## Qualification and regression evidence

| Conversation | Role | Initial cached / saved steps | Qualified synced steps |
| --- | --- | ---: | ---: |
| `ee9e2816-0bf0-4025-ba13-e65355a90694` | Coordinator | 0 / 270 | 270 |
| `ac82d526-5252-490a-b617-c9a1245b0393` | Swarm worker | 2 / 7 | 7 |
| `dad40f65-f688-4484-a6ce-7ba83caea0df` | Share workspace child | 6 / 6 | 6 |
| `6329c59f-93cf-4d62-bf6e-e6398404e3c1` | Branch workspace child | 6 / 6 | 6 |

For each source, `TestNativeEvidence` passes full/default comparison and saved
attestation validation. `export-offline --format=json` matches the complete
parsed displayed RPC after removing reader provenance. Ordinary `--sync` and
`audit-qualify` pass on the private store. A rendered coordinator check finds
each child under its matching creation step (162, 166 and 168, one-based).
The coordinator's later tool output quotes an old truncated reader transcript;
those quoted details are retained as source content, not new child expansions.

The fresh sweep fetched and rendered 499/499 conversations with zero failures.
Native sync verified 489/499. The ten remaining historical source/RPC mismatches
have exactly the same IDs and reasons as the retained 1.5.4 sweep and remain
tracked in agy-reader#cpqn. All four required release scenarios pass the format
helper's current-source gate. A fully verified historical corpus is not claimed.

Focused synthetic tests cover the captured mappings, altered source/result
content, unknown arguments, unsupported actions and states, mismatched workspace
arguments, duplicate or reordered child IDs, wrong state counts and indices,
invalid timestamp/frontmatter forms, and safe child placement with reordered,
missing, repeated and legacy children. Package tests, race tests, vet and lint
pass.

SQLite remains at `user_version=1`, seven tables and two explicit indices.
The unchanged schema fingerprint is
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
The fresh corpus adds 30 shape paths and removes none compared with the retained
1.5.4 path listing: workspace branch/share fields, message delivery strategy,
environment ID, and agent configuration fields. These are observed corpus
coverage additions, not proof that all were introduced in 1.5.5. Raw sidecars
preserve them; the exercised sources need no new embedded declarations.

## Binary provenance and exercise limits

The [1.5.5 snapshot](../../artifacts/proto-snapshots/agy-1.5.5-89f16bdb87f6/snapshot.json)
verifies 398/398 embedded byte ranges, 34 closure files and 34 round-trip files.
The installed version and isolated daemon startup log both report 1.5.5; the
serving executable matches SHA-256
`89f16bdb87f62e6d07484c52e6bafe76a81b0fd13d740110c77dc3c3c8d8d8b6`.
Compared with 1.5.4, the snapshot adds two wire fields and has zero JSON-facing
or descriptor-only changes: browser `enable_eval_browser_use` (field 34) and
recording `recording_name` (field 5). Names do not establish runtime behavior.

Changelog verdict: swarms and workspace-copy changes are relevant and exercised.
Both child transcripts show propagation of the untracked snapshot directory;
this does not verify staged/unstaged tracked changes, non-Git copies, size limits
or all workspace semantics. Workspace selectors predate this release. Plugin
import/hooks, teamwork state relocation, ripgrep fallback, headless shutdown,
gateway/auth behavior, model availability and board TUI behavior remain outside
this trajectory qualification. The supplied ripgrep/headless attempt failed at
sandbox startup. AgentsView tests and live downstream re-ingestion were not run.

The initial audit on `339c682` failed 2/4 required sessions with unsupported
generic results. It also found earlier coordinator presentation mismatches;
the timestamp-prefixed skill reads explain the two isolated step differences
in the fresh capture. Existing child parent metadata already linked all three
children. Single-child `ConversationIds` kills already worked; the unsupported
case was the two-child payload, not the argument name itself.

Private source copies were hash-checked before and after copying, including
nonempty WALs. A separate daemon served only copied stores; the existing pair
was neither restarted nor stopped. Original audit evidence remains under
ignored `tmp/agy-1.5.5-audit/`; fresh captures, manifests, comparisons and test
logs are under `tmp/agy-1.5.5-support/`. These contain private conversation data
and must stay private. Recording compatibility changes the checkout baseline
only; installing or restarting the deployed reader is a separate operation.
