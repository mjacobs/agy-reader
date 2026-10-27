# agy 1.4.8 offline export follow-up

Offline export of a session containing subagent tests failed with `unknown
persisted protobuf fields`. The missing declaration was field 3, `subagents`,
of `exa.cortex.core.tools.ManageSubagents`, including its nested `SubagentState`
message and optional `step_index`. The existing 1.4.8 schema snapshot had these
definitions; the production decoder's historical schema did not.

The fix adds those exact definitions without replacing historical fields.
It also qualifies the additional presentations that blocked this session:

- A resultless `define_subagent` error rejecting a colon in the agent name.
- A single-child `manage_subagents` list carrying step index `-1`, and a
  single-child kill with no step index. Both preserve the payload and reproduce
  the daemon's additional result metadata.
- An overwritten file marked as an artifact without artifact metadata, where
  all wrapper arguments match the retained tool call and the path matches the
  payload. The artifact flag survives reconstruction.

MCP payloads now survive typed decoding and appear as JSON in Markdown's
fallback renderer, including server, tool, arguments and result. This avoids
silently replacing those details with “No step details available.”

## Review of the reported findings

| Finding | Assessment |
| --- | --- |
| Failed calls hit the resultless-tool whitelist | Partly correct. The rejected agent definition does. The tested MCP error instead has an embedded native MCP step, already supported by completeness projection. The report's blanket claim about all failed MCP calls is incorrect. |
| MCP details disappear from Markdown | Confirmed and fixed. Raw trajectory JSON already preserved them; typed rendering did not. |
| Subagent state and list/kill calls fail | Confirmed. New protobuf field 3 caused decoding failure before projection. The legacy projection allowed only two-child lists. This fix adds the captured one-child list/kill shapes. Other counts, actions and step-index values remain unqualified. |
| Shared/branched workspace invocations fail qualification | Confirmed code restriction, not a demonstrated new regression. The test used a normal research subagent, without exercising either workspace mode. |
| Goal state is absent from the typed transcript | A coverage concern, not evidence that raw JSON loses it or that fork cancellation is incorrect. No active-goal fork was tested. Goal behavior needs its own capture and comparison. |

The successful ordinary subagent launch does not test plugin-qualified names.
The colon-name `define_subagent` call actually failed validation. The changelog
concerns custom agent names containing a colon; the report does not establish
that this definition path is the same path as a plugin-qualified custom agent.

Killing a child and observing that its brain directory is absent does not test
the release's parent-conversation deletion fix. It also does not establish when
that directory disappeared, whether saved conversation databases were removed,
or whether shared/branched workspaces were cleaned up. No destructive lifecycle
tests were performed for this follow-up.

## Evidence and limits

The saved 1.4.8 snapshot was verified against the installed binary: all 387
extracted records matched their byte ranges, with 34 closure and roundtrip
files. The production overlay retains 31 historical descriptors and adds only
the subagent state declarations. Its generator authenticates every input and
checks that removing those additions restores the previous declaration.

The reported SQLite session failed on the unchanged reader. With this fix,
JSON and Markdown offline exports succeed for all 303 steps. The source digest
remained stable across both RPC reads. Comparison of the persisted
source against both full and ordinary live RPC responses returns `verified`.
The tests use synthetic content and cover exact metadata, rejected unqualified
shapes, nested unknown protobuf fields, artifact preservation and MCP rendering.
Private captures and exports are not included in the repository.

This is a targeted session qualification, not a new all-conversation audit.
`COMPATIBILITY.md` retains its earlier audit record. The profile change makes
older attestations unknown until reverified. The installed reader and watcher
must be upgraded separately to use the new decoder.
