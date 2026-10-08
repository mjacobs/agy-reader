# Supported SQLite completeness profile

The reader only marks a SQLite sidecar verified when the persisted trajectory,
the full raw RPC, and the ordinary RPC's supported projection agree. This
prevents a reachable daemon with an old in-memory cache from certifying a
partial or same-step stale response.

## Supported representation

Profile `sqlite-v1-format0-agy-1.3.0-19be6af38f7b-planner50-permission2-prompt23` supports SQLite
`user_version=1`, the seven known tables and their exact columns, contiguous
zero-based indices, and `step_format=0`. All persisted protobuf fields and enum
values must be known, except the two declared source-only fields below.
Decomposed subtrajectories remain unsupported. The decoder checks
redundant step columns against the complete step payload. In particular,
`steps.render_info` must equal `generic.result.step_render_info`, including
presence (SQL NULL means absent); malformed and unknown content is rejected.
Unknown versions, fields, formats, tables, and columns return unknown; they
cannot create or refresh completeness evidence.

### Declared source-only field: CascadePlannerConfig field 50

Historical sources (78 retained ones, 72 CLI and 6 IDE) store
`exa.cortex_pb.CascadePlannerConfig` field 50, wire type 2, inside
`generator_metadata` and `executor_metadatas` records. The current descriptor
reserves that number, and the daemon's RPC never returns it, so the full RPC
can never equal these sources. The profile accepts the field by number and
wire type only. It assigns no name or type: no authentic definition for these
bytes is known (the IDE 2.0.1 build's `context_engine_config` declares a
different nested field).

- Accepted only at the two locations observed in the retained sources:
  `CortexStepGeneratorMetadata.planner_config` and
  `ExecutorMetadata.cascade_config.planner_config`. It may occur once per
  message, with no other unknown field beside it.
  `ExecutorMetadata.request_override_config.planner_config` was never observed
  carrying it and stays unknown.
- Its payload must be one of the observed shapes: empty, or varint fields 2
  and/or 4, each once, in that order, with value zero.
- Any other field number, wire type, location or nested content is unknown.
  That includes another message, trajectory metadata, parent references,
  battle-mode infos, a step, an embedded Any, and every `CascadePlannerConfig`
  nested deeper inside a record. The descriptor nests planners recursively (an
  agent definition in a planner's customization settings, or in
  `ExecutorMetadata.agent_config_resolution`, carries its own cascade and
  planner configuration), so the allowance names exact paths rather than "any
  planner in the record". A test walks the embedded descriptor from both
  record types, entering each message type at most twice on one path, places
  the field on every planner it reaches, and checks that exactly these two
  paths accept it. It also checks that no step or trajectory message is
  reachable from either record outside an embedded Any.

### Declared source-only field: ChatMessagePrompt field 23

15 retained CLI sources, written between 2026-06-17 and 2026-07-20, store
`exa.chat_pb.ChatMessagePrompt` field 23, wire type 2, 47 times. It is a
structured copy of the results of the `grep_search` step that the tool-result
prompt answers. Later builds write the same content to field 25
(`prompt_struct`), which the reader compares like any other field. The current
descriptor reserves 23, and the daemon's RPC drops it. As with field 50, the
profile accepts it by number and wire type only, with no name or type: no
descriptor from a build that wrote it has been read, so the exact producer
build and the field's name are unknown.

- Accepted only on an element of
  `CortexStepGeneratorMetadata.chat_model.message_prompts`, at most once per
  prompt, with no other unknown field beside it. A test walks the descriptor
  from generator and executor records and from a step, places the field on
  every `ChatMessagePrompt` it reaches, and checks that only this path accepts
  it.
- The prompt must be a tool result: `source` is `CHAT_MESSAGE_SOURCE_TOOL`,
  `tool_call_id` and `step_idx` are set, and `prompt_struct` is not.
- The payload must parse as `google.protobuf.Struct` with no unknown field at
  any depth, and re-encode to its own length (so nothing was dropped or
  merged, such as a repeated key). Its shape must be the observed one: a
  `results` list of 1 to 50 objects, either all `{path}` or all
  `{path, line_content, line_number}`, with strings where strings belong and
  a whole `line_number` of at least 1; and optionally a whole, positive
  `num_elided_results`. Nothing else.
- The values must be derived from the step that `step_idx` names, which must
  be a `GREP_SEARCH` step whose tool call ID equals the prompt's
  `tool_call_id`. The list has as many entries as the step has results, in the
  same order. Entry *i*'s `path` equals result *i*'s relative or absolute path;
  its `line_content` equals result *i*'s `content`; its `line_number` equals
  result *i*'s `line_number` + 1. Path-only entries belong to a search with
  `match_per_line` false, and line entries to one with it true.
  `num_elided_results` is present exactly when `total_results` is larger than
  the number of results, and equals the difference.
- Any other location, wire type, prompt, shape or value is unknown. The step
  check runs when the whole source has been read, so a field-23 payload that
  does not match its step refuses the source like an unknown field.

### Source-only bytes in the comparison and the attestation

The source digest keeps the bytes of both fields. The full-raw and presentation comparisons
use a copy with them removed. A verified attestation for such a source carries
the reason `verified except declared source-only fields absent from RPC` and a
`sourceOnly` record with one entry per declared field present (field 50
first, then field 23): the declared path, the number of occurrences, the
SHA-256 of their stored encodings in traversal order, and each occurrence's
location and payload. The sidecar therefore keeps the bytes under
`agyReader.completeness`. A recheck requires that record to match the current
source; a changed record is incomplete, like a changed payload.

Each output that reports a verification result shows such a source apart
from plain verified: `audit-qualify` prints `VERIFIED WITH SOURCE-ONLY FIELDS`
with the reason, `doctor` counts it as `verified with source-only fields`, an
`audit-sweep` manifest records the reason and counts it in
`verifiedSourceOnly` (its `verified` count holds plain results only), watch
and `--sync` add the reason to their `synced` and `wrote` lines, and the
format-audit helper's summary and record count it. It is otherwise a verified
result: fresh, not re-synced, and qualifying a format-audit record.

Changing the profile identifier makes every attestation minted under the
previous profile read as unknown until the next live sync or watch re-verifies
it. The identifier names the agy release and the first 12 hex digits of the
binary's SHA-256 that the descriptors came from. A suffix after the hash marks a projection rule that refuses sources an
earlier profile accepted, so saved audit manifests from before the rule cannot
certify them: `permission1` refuses a native `RUN_COMMAND` step whose permission
record has no copy on its wrapper, and `permission2` refuses a native `VIEW_FILE`
step in the same case. The `planner50` and `prompt23` suffixes mark the
declared source-only fields; `prompt23` also adds a second kind of
`sourceOnly` entry, so it replaced
`sqlite-v1-format0-agy-1.3.0-19be6af38f7b-planner50-permission2`. The 1.3.0 profile replaced
`sqlite-v1-format0-agy-a759ce7c7a23-planner50` (the 1.2.16 descriptors), under
which the CLI store held 154 attestations on 2026-10-06: 105 standard and 49
with source-only fields. That profile had replaced
`sqlite-v1-format0-agy-a759ce7c7a23` (84 standard attestations at the time).

The full RPC uses `CLIENT_TRAJECTORY_VERBOSITY_FULL` and
`disableRehydration=true`. It must equal the reconstructed persisted protobuf.
The default response must equal its supported display projection:

- Ordinary steps pass through unchanged.
- Generic wrappers containing recognized run-command, code-action, view-file,
  or invoke-subagent native Steps are replaced with the embedded Step. Outer
  task details are preserved; conflicting or inner-only task details fail.
  Wrapper and native status must match, with one exception: a run-command
  wrapper `CANCELED` over a native `ERROR` that carries error details is
  presented with the outer `CANCELED` status. A canceled wrapper over any
  other native status still fails.
- A run-command or view-file step whose call asked for permission and was approved
  stores the record on the wrapper and on the native Step: `permissions` (the
  grant) and `completed_interactions` (the request and its answer). The
  ordinary response presents the native Step's copy, with the pre-tool hook
  results in its metadata, and drops the wrapper's. Both copies must be present
  and equal, both steps `DONE`, and the record must be the shape observed in
  the agy 1.2.17 audit: one allowing `command` grant whose target, and resource
  target, is the command that ran; one permission request for that command,
  raised by a `TYPE_FORCE_ASK_HOOK` trigger and answered with `allow`, with an
  empty `persist_grants`. Any other field in the grant, request or answer, a
  rejected request, another trigger, a record on one copy only, or a wrapper
  record for any other tool or over any other native type fails. A rejected
  request leaves a different step, with no result and no grant; see the
  resultless wrappers below.
  A view-file step (agy 1.2.16, agy-reader#axnr) carries the same two copies:
  one allowing `read_file` grant whose target, and resource target, is the
  `AbsolutePath` argument; one permission request for that file with only a
  resource and an action description (no trigger), answered with `allow` and an
  empty `persist_grants`. The same equality, `DONE` and unknown-field rules apply.
- A view-file `ContentOffset` argument is accepted only when it is a canonical
  decimal and the presented native Step carries the same value, both in its
  tool-call arguments and in `view_file.content_offset`. The ordinary response
  therefore still shows the offset that the dropped argument recorded.
- A `write_to_file` `Append` argument is accepted only when it is `true` or
  `false` and the presented native code-action Step records the same value
  twice: as the same JSON literal in its tool-call arguments, and in
  `code_action.action_spec.create_file.append` (set for true, unset for
  false). Only `true` was observed, in one retained agy 1.2.16 session. Any
  other value, a call to another tool, a native Step without a `create_file`
  spec, or a tool call that lacks or changes the value fails with
  `unsupported write_to_file append`.
- A failed wrapper with status `ERROR` may carry an outer `error` over an
  embedded native Step only for the types observed with one: run-command,
  code-action, view-file and generate-image. That Step must also be `ERROR`
  and its own error must have the same short error. The ordinary response
  presents the native Step with its inner error. The outer error may hold only
  short and full error text; it is dropped from the presentation and still
  covered by full raw equality. A failed `generate_image` call embeds a native
  image Step instead of a `GenerateImage` Any and is qualified the same way,
  with its prompt, image name and aspect ratio arguments. An embedded image
  Step on a call that did not fail still fails. An outer error over an
  invoke-subagent Step, or over any other native type, fails; a type joins
  this list only with an observed source, full RPC and default RPC triplet.
- A wrapper that failed before its tool ran stores arguments and an outer
  `error` but no result. With status `ERROR`, the ordinary response presents
  it unchanged for `write_to_file`, `view_file`, `find_by_name`, `list_dir`
  and `schedule`, with arguments bounded per tool. `write_to_file` is also
  accepted with status `INVALID`. Two more were observed. A `generate_image`
  call whose arguments the tool rejected has status `INVALID` only, an outer
  error starting `invalid arguments:` and the arguments `AspectRatio`,
  `ImageName` and `Prompt`; the ordinary response presents it unchanged. A
  `run_command` call that a `PreToolUse` hook denied has status `ERROR`, an
  outer error starting `tool call denied by pre-tool hook: ` and the arguments
  `CommandLine`, `Cwd` and `WaitMsBeforeAsync`; its metadata must record
  exactly one hook result, a `deny` whose reason the error repeats. The
  ordinary response presents it as a `RUN_COMMAND` step that keeps the status,
  metadata and error, with the command line (also as the proposed command
  line), working directory, a blocking call and empty combined output taken
  from the arguments. Both outer errors may hold only short and full error
  text. A resultless wrapper without an outer error (other than the canceled
  prompt below), with extra wrapper fields, or for any other tool fails.
- A `run_command` call whose permission request was rejected (agy 1.3.1,
  agy-reader#xyfe) is also resultless. agy writes it when a user in review mode
  answers "No, cancel" (or "No, deny" for a subagent's command), and when it
  soft-denies a request from a subagent of a headless `-p` run, which nobody
  can answer; the three observed samples were alike. The wrapper has status
  `ERROR`, the arguments `CommandLine`, `Cwd` and `WaitMsBeforeAsync` (the
  headless child also `BypassSandbox` `"false"`), no result and no
  `permissions` grant. Its metadata records exactly one pre-tool hook result,
  `{decision: allow}`, with no reason (agy writes it with no hook
  configured). Its outer error holds only short and full text, both exactly
  `permission check failed for command "<command>": user denied permission to
  run command:`, the command, and agy's advice not to circumvent the denial,
  on three lines. It keeps the completed request in `completed_interactions`:
  one permission request for that `command` with only a resource, a suggested
  persist type and pattern, an action description, the shell name and a
  `TYPE_DEFAULT_FALLBACK` trigger with no detail, answered with no `allow`
  field and an empty `persist_grants`. The ordinary response presents it as a
  `RUN_COMMAND` step built like the hook-denied one, keeping the status,
  metadata, error and completed request, so a transcript shows the command
  with the rejection as its error. The dropped arguments must repeat the
  presented tool call. The quoted command in the error is taken to be Go
  `%q` quoting; only plain commands were observed, and a different quoting
  fails closed. An answer that allows, another trigger, a changed error, a
  missing or second interaction, a grant, a hook result other than the one
  `allow`, another status, `BypassSandbox` `"true"`, or any other field
  fails.
- A `run_command` call canceled while its permission prompt was still open
  (agy 1.3.1, agy-reader#3cxt) is the one resultless wrapper without an outer
  error. It was observed once, in conversation `92bd61e6`, when agy was
  stopped with the prompt unanswered. The wrapper has status `CANCELED`, the
  arguments `CommandLine`, `Cwd` and `WaitMsBeforeAsync`, no result, no
  error, no `permissions` grant and no interaction record, pending or
  completed. Its metadata records exactly one pre-tool hook result,
  `{decision: allow}`, as on the rejected request, and a status history whose
  last three entries are `WAITING`, `RUNNING`, `CANCELED`: the step waited for
  the answer and was canceled when it resumed, with no answer recorded. The
  rejected requests end `WAITING`, `RUNNING`, `ERROR` the same way, and their
  commands did not run. The ordinary response presents it as a `RUN_COMMAND`
  step built like the hook-denied one, keeping the status and metadata, so a
  transcript shows a canceled command with no exit code and no output. The
  dropped arguments must repeat the presented tool call. An outer error, a
  grant, an interaction record, task details, `BypassSandbox` or any other
  argument, a hook result other than the one `allow`, a status history that
  does not end that way, another status or another tool fails.
- A resultless `view_file` with status `ERROR` that records the `BUILTIN`
  tool source is presented instead as a `VIEW_FILE` step built from its
  arguments: the wrapper's status, metadata and error, no generic arguments,
  and a `view_file` with `absolute_path_uri` `file://` plus the path and the
  one-based `StartLine` and `EndLine` as zero-based `start_line` and
  `end_line`. agy produces it when it rejects a view before running it, for
  example of a named pipe or `/dev/null` (`path is a device, pipe, or socket,
  not a standard file`). The daemon also sets `is_skill_file` when it takes the
  path for a skill file, which the source does not record. In 3,270 presented
  view-file Steps in retained copies that flag was never set for a path
  without `skill` in it, while the same skill path was marked in one session
  and not in another. So the step is built without the flag, and accepted,
  only when the path does not contain `skill` in any letter case. The
  arguments must equal the presented tool call, which still shows them, and
  are limited to the path and either both lines or neither, with
  `1 <= StartLine <= EndLine`. The path must be absolute, already clean, and
  made only of letters, digits and `/ . _ - @ +`, the characters observed
  copied into the URI unescaped. A possible skill file, another tool source,
  another status, an outer error with more than short and full text, any
  other argument, or another path or line shape fails with
  `unsupported resultless view_file`. A presented `is_skill_file` that the
  source did not predict makes the result incomplete, not verified.

  One retained conversation is refused for this reason and is recorded here as
  unsupported on purpose: `7b3888f6-9f76-4504-969a-187c95705773`
  (agy-reader#3193). Its step 62 (zero-based) is a failed `view_file` call with
  no result, naming a `SKILL.md` file; it is the only step refused when each of
  the 355 steps is projected on its own. The 2026-10-03 ordinary RPC capture
  presents that step with `isSkillFile: true`, but the source does not record
  the flag and the daemon set it for the same skill path in one session and not
  in another, so the flag cannot be predicted from the source. The reader keeps
  refusing the conversation: it accepts neither value of the flag for this
  shape, so no sidecar is written and the conversation has no standard
  attestation. The stored source is complete (the reader reaches the projection
  only after the full raw RPC equals it); it stays retained and unchanged. All
  it has is the one-off export audited by hand on 2026-10-03, which is not a
  normally verified transcript. Accepting this shape would need a decision on
  the flag in a separate change; see
  `docs/compatibility/recovery-rerun-2026-10-07.md`.
- Payload-free `manage_task` and `send_message` steps remain generic, with
  bounded arguments and result fields. Task metadata keys are limited to the
  observed `task_description`, `task_id`, and `tool_name`. A failed
  `manage_task` step with status `ERROR` may also carry an outer `error`, which
  the ordinary response presents unchanged. An outer error on any other
  payload-free wrapper fails.
- Payload-free `schedule`, `manage_subagents`, and `define_subagent` steps also
  remain generic and are presented unchanged. Each is bounded to its observed
  arguments (`DurationSeconds`, `Prompt`, `TimerCondition`; `Action`; either
  `definition` alone or `description`, `enable_mcp_tools`,
  `enable_subagent_tools`, `enable_write_tools`, `name`, `system_prompt`), to
  the result text, output URI, and render information, and to status `DONE`.
  Only `schedule` may carry outer task details. As the one exception to the
  outer-error rule above, a `schedule` step may also fail with status `ERROR`
  and an outer error that holds only short and full text. A `manage_subagents`
  result may carry only the `killed_conversation_ids` metadata key. A
  payload-free step for any other tool still fails.
- A payload-free `list_permissions` step also remains generic and is presented
  unchanged. It is bounded to no arguments, a result of only text and render
  information (no output URI, metadata, media, or payload), no outer task
  details or error, and status `DONE`.
- A payload-free `ask_permission` step also remains generic and is presented
  unchanged. It is bounded to exactly the `Action`, `Reason` and `Target`
  arguments, a result of only text and render information (no output URI,
  metadata, media, or payload), no outer task details, error or permission
  records, and status `DONE`.
- The `generate_image` tool's `exa.cortex.core.tools.GenerateImage` Any becomes
  a native image step. It preserves outer metadata and maps prompt, image name,
  aspect ratio, model name, and the single returned media item. The media URI
  must agree with the payload's image URI. Unqualified payload fields, missing
  or multiple media items, and other Any types fail closed.

Arguments and wrapper/result fields are bounded separately for each projection.
Wrapper result text, media, output URI, arguments, and metadata omitted by the
ordinary RPC still have to match the full raw RPC. Image byte count is also
checked by raw equality even though the ordinary image step omits it. Unknown
RPC-only JSON fields remain in the saved payload and its digest.

The source digest includes the complete reconstructed protobuf and profile.
The payload digest includes all ordinary RPC fields except the reader-owned
`agyReader` namespace, with exact JSON numbers and canonical object order.
A matching existing attestation can be rechecked offline. Missing evidence is
unknown; changed source or payload is incomplete. Content can change after any
snapshot; the next watch/doctor check detects it without trusting mtime.

## Schema provenance and maintenance

`internal/completeness/schema.pb.gz` contains a transitive descriptor set for
`gemini_coder.Trajectory` plus `third_party/jetski/cortex/core/tools_pb/tools.proto`,
extracted from the agy 1.3.0 binary with SHA-256
`19be6af38f7beeaa0db415df9297e314ab3d33fdd6f853434d49f88819bc68e4`.
The second root supplies `exa.cortex.core.tools.GenerateImage`, which travels
inside an Any and is absent from the trajectory's ordinary import closure.
The closure contains 31 descriptors (131,880 compressed bytes), with SHA-256
`c52f0a2af19d559d0c0555ca56a29b3a6207452c1651a0648982f299333e36eb`.
All 322 extracted records were checked against bytes at their manifest offsets
in that binary before generation (a first extraction found 316: the scanner
swallowed trailing bytes that looked like one more field and dropped six
records, none of them in the closure; it now backs off to the longest prefix
that parses). The bundle contains schema definitions only,
no conversation payloads or credentials.
The generator consumes a separate directory of `FileDescriptorProto` records
and a manifest declaring that hash and the hash of each extracted record.
It verifies those hashes, drops five named annotation-only files whether or not
the extractor found them, removes the imports and extension declarations that
refer to them, and writes deterministic gzip output. Any other missing import
stops it.
The descriptor delta from 1.2.14 adds `AgentConfigResolution.contrib_variant_metadata`
(message `.exa.cortex_pb.ContribVariantMetadata`), additive `.exa.codeium_common_pb.Media`
fields (`start_page`, `end_page`, `start_time_ms`, `end_time_ms`, `fps`, and
`media_resolution`), and `.exa.cortex_pb.ViewFileToolConfig.enable_open_document`.
These schema definitions are extracted directly from the binary descriptors;
not all additive fields are exercised by the native audit scenarios. The
1.2.16 profile also includes the extracted image-agent and plugin metadata
schema additions. agy 1.2.17 added `RunCommandToolConfig.enable_plugin_auth`
(field 23), which every 1.2.17 and 1.3.0 conversation stores as `false`, and
`WorkflowSpec.depot_path` (field 19). agy 1.3.0 added
`CheckpointConfig.media_tokens_per_second` (34),
`CheckpointConfig.use_model_reported_token_count` (35) and
`CascadeNUXConfig.g1_redirect_url` (21), and marked six existing fields
deprecated without changing their wire format
([agy 1.3.0 notes](compatibility/agy-1.3.0.md)). Descriptor availability alone
does not qualify new generic tool projections.

To refresh the schema for a new agy release, copy the installed binary so it
cannot change underneath you, then run:

```sh
python3 scripts/extract_descriptors.py AGY_COPY tmp/descriptors   # prints count and binary SHA-256
# set BINARY_SHA256 in internal/completeness/generate_schema.py to that hash
python3 internal/completeness/generate_schema.py tmp/descriptors internal/completeness/schema.pb.gz
```

The extractor is read-only on the binary and needs `google.protobuf`;
`make maintenance-test` runs its tests. Then change `Profile` in
`internal/completeness/schema.go`, update this section, and requalify as
described below.

The generator is a maintenance aid, not a runtime dependency. It needs Python
and `google.protobuf`; ordinary builds need Go 1.25 or later. Runtime decoding
uses the pinned Go protobuf and pure-Go SQLite modules in `go.mod`, without
Python or a SQLite CLI. Updating descriptors alone does not authorize a new
projection: qualify the source format and both RPC representations, extend
synthetic rejection tests, and update the profile identifier before accepting
new content.

The normal test suite uses synthetic SQLite and HTTP fixtures. The opt-in
`TestNativeEvidence` accepts explicitly supplied disposable DB/raw/default RPC
files through `AGY_READER_SCRATCH_DB`, `AGY_READER_SCRATCH_RAW`, and
`AGY_READER_SCRATCH_PRESENTATION`; it does not discover live profiles or daemons.
