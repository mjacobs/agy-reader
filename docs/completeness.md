# Supported SQLite completeness profile

Conversation labels such as `conversation-001` are consistent aliases across
these reports, not session IDs. Use actual IDs from your own store when
rerunning commands.

The reader only marks a SQLite sidecar verified when the persisted trajectory,
the full raw RPC, and the ordinary RPC's supported projection agree. This
prevents a reachable daemon with an old in-memory cache from certifying a
partial or same-step stale response.

## Supported representation

Profile `sqlite-v1-format0-agy-1.3.4-5b93565c5f04-planner50-permission2-prompt23-tools143-permission1-tools144-skill1-history2-ide225-options145-compaction146-skill146-finish147` supports SQLite
`user_version=1`, the seven known tables and their exact columns, contiguous
zero-based indices, and `step_format=0`. All persisted protobuf fields and enum
values must be known, except the two declared source-only fields below.
Decomposed subtrajectories remain unsupported. The decoder checks
redundant step columns against the complete step payload. In particular,
`steps.render_info` must equal `generic.result.step_render_info`, including
presence (SQL NULL means absent); malformed and unknown content is rejected.
Unknown versions, fields, formats, tables, and columns return unknown; they
cannot create or refresh completeness evidence.

The `tools143` suffix qualifies six modern tool payloads observed with agy
1.4.3. `RunCommand` retains outer metadata and task details and presents the
stored result text as command output; `ReadFile` converts one-based ranges to
zero-based `VIEW_FILE` ranges and keeps the stored result text. `FileEdit`
maps CREATE and EDIT results into `CODE_ACTION`, including empty-file diffs.
`InvokeSubagent` preserves the child specifications and results. `ManageTask`
and `Schedule` remain generic, with metadata derived from their payloads.

These are bounded mappings: completed commands, reads, creates and edits;
completed single-task status/list/kill results; a canceled timer; and a
completed subagent invocation without task mode. Other statuses, argument
names, newly populated payload fields, paths requiring URI normalization,
and unsupported outcomes still return unknown. Full persisted/RPC equality
precedes projection, and every presented field must match. The change does
not make a schema fingerprint or equal step counts sufficient evidence.

The combined `tools143-permission1` suffix means the modern 1.4.3 tool
mappings initially refused wrappers with permission grants or
completed interactions. Their presentation has not been qualified by a native
1.4.3 capture; the `ide225` exception below applies to modern reads only.
Older native-Step permission mappings remain separate.

The `tools144` suffix adds mappings observed with agy 1.4.4. Canceled
background `run_command` steps with render-only results become native command
steps with the stored command and empty output. They retain metadata and task
details, require the observed pending/running/canceled history, background
timestamp and 5000 ms wait argument, and do not invent an exit code. Completed
modern `ManageSubagents` list calls with two conversation IDs remain generic;
their result metadata gains the action and comma-joined IDs from the payload.
Modern subagent invocations also accept `Workspace=inherit` when the stored
arguments match the metadata tool call; child specifications and results remain
unchanged. The `skill1` suffix derived modern reads' `isSkillFile` flag from
the exact enabled-skill entry in the step executor's persisted customization
registry. The `skill146` suffix replaces that rule; see below. The
expected flag must match the ordinary RPC along with every other presented
field. Other workspace selectors, actions, ID counts, argument names,
permissions, completed interactions
and unsupported fields remain unknown. These mappings use the existing 1.3.4
descriptor; no schema definition or source-only allowance was added.

The [1.4.4 audit](compatibility/agy-1.4.4.md) records the source/full/default
qualification and the broad corpus checks. Old attestations must be reverified
under the new profile.

The `history2` suffix adds bounded mappings for historical sessions rechecked
with agy 1.4.4: failed `list_resources`, calls rejected for missing
`toolSummary`, a command whose bridge failed while formatting its result,
legacy resultless skill reads, and payload-free `manage_task send_input`.
These rules use persisted arguments, errors, native payloads and executor
registrations. They preserve the full-source comparison and add no descriptor
fields or source-only exceptions. The detailed bounds appear below.

The `options145` suffix adds the three configuration fields qualified with
agy 1.4.5: browser auto-invoke, per-tool output limits, and skipping plugin
logos in file views. Two idle conversations persisted enabled and explicit
false flags, plus absent, zero, negative and positive per-tool limits. Their
SQLite sources, full RPC and ordinary RPC agreed. These fields remain part of
the compared payload; they are not source-only exceptions. This qualifies
serialization and retention, not the options' effects on executing tools.

The per-tool map is checked before protobuf decoding, which otherwise discards
unknown entry fields and repeated keys. Unknown, malformed, wrongly encoded or
duplicate entry members and duplicate map keys are refused, including keys
split across repeated singular enclosing messages. Missing key/value members
keep protobuf defaults. The separate new `BASH` enum remains unsupported.
See the [qualification report](compatibility/agy-1.4.5-options.md).

The `compaction146` suffix preserves captured checkpoint requests and their
attachments, checkpoint context/media settings, and the customization-kind
map carried by native agy 1.4.6 conversations. Four native conversations with
five manual checkpoints passed source/full/default comparison, offline export,
sync, and saved-attestation checks. The customization map receives the same
strict wire checks as the per-tool map. No projection or source-only allowance
was added. Old attestations require reverification under this profile. See the
[feature qualification and its limits](compatibility/agy-1.4.6-compaction.md);
the separate whole-release 1.4.6 audit failed on one skill-file marker case,
which the `skill146` suffix resolves.

The `skill146` suffix sets a modern read's `isSkillFile` flag from its stored
path alone: the flag is set when the path, with backslashes read as slashes,
ends in `/SKILL.md` or is exactly `SKILL.md`. Case matters, so `skill.md` and
`SKILL.MD` stay unmarked. The executor's skill registry, the filesystem and the
file content play no part. This is what agy 1.4.6 does when it rebuilds a
stored `ReadFile` result as a `VIEW_FILE` step for the ordinary RPC; the
installed 1.4.6 binary's `rehydrateReadFile` calls a path-only `isSkillFile`
check and stores its answer unconditionally. The earlier registry rule
predicted no marker for a read of an unregistered plugin `SKILL.md`, while
agy marked it (agy-reader#xpeh). Across the 405-conversation 1.4.6 corpus,
the path rule agrees with all 76 presented modern reads (6 marked, 70
unmarked); the registry rule disagreed on one. Every preserved native capture
from 1.4.4 and 1.4.6 still verifies. Legacy native `VIEW_FILE` steps keep their
stored flag, and failed legacy reads keep the `history2` registry rule below.
Old attestations require reverification under this profile.

### IDE 2.25.0 file tools

The `ide225` suffix fixes offline export of the file-tool wrappers observed in
Antigravity IDE 2.25.0. On 2026-10-22, a frozen SQLite source with 76 steps matched
the running IDE's full unrehydrated RPC. The extended projection then matched
all fields of its ordinary response. This qualifies these shapes in that
conversation, not every possible IDE tool or outcome:

- Three completed `read_file` wrappers retain their permission grants and
  completed interactions unchanged. Both fields must be present together.
  There is no permission record synthesis or removal; full-source equality and
  presentation equality cover their complete contents. Other modern tools
  still refuse permission-bearing wrappers.
- Ten reads accept `ByteOffset=0`, with all arguments repeated in the retained
  tool call. Nonzero and noncanonical offsets remain unsupported.
- Seven artifact creates and six artifact edits retain `isArtifactFile`.
  Creates also retain the payload's summary, update timestamp and user-facing
  flag. Their `ArtifactMetadata` argument must match the retained tool call and
  payload, with `RequestFeedback=false`. Metadata on edits, missing artifact
  flags and unqualified metadata fields remain unsupported.

The IDE daemon executable's SHA-256 was
`c1deaf112e8d5b634ef5774d063cc763795c5796b98f3cf3151ca553f62339b6`.
Synthetic source/presentation pairs in `projection_ide225_test.go` cover the
mapping and refusals without storing conversation content. The descriptors and
source-only allowances are unchanged. Existing attestations need reverification
under the new profile; offline exports still carry no completeness attestation.

### Declared source-only field: CascadePlannerConfig field 50

Historical sources (78 retained ones, 72 CLI and 6 IDE) store
`exa.cortex_pb.CascadePlannerConfig` field 50, wire type 2, inside
`generator_metadata` and `executor_metadatas` records. The current descriptor
reserves that number, and the daemon's RPC never returns it, so the full RPC
can never equal these sources.

The field's historical definition is known. The official agy CLI 1.0.4 and
1.0.10 release binaries both declare it as `context_engine_config`, of type
`exa.cortex_pb.ContextEngineConfig`
([declarations](../artifacts/historical-schema-recovery/provenance.json),
[descriptor excerpts](../artifacts/historical-schema-recovery/recovered-definitions.json),
[summary](../artifacts/historical-schema-recovery/README.md)). In 1.0.10 that
message has five fields: `enable_prehook` (1), `enable_tools` (2),
`rpc_timeout_seconds` (3), `enable_reflection_agent` (4) and
`reflection_agent_model` (5)
([cortex.proto](../artifacts/historical-schema-recovery/1.0.10-proto/third_party/jetski/cortex_pb/cortex.proto)).
1.0.4 declares only fields 1 to 3, and the IDE 2.0.1 build only field 1;
neither declares field 4, which the accepted shapes below include. Read with
the 1.0.10 definition, each payload the profile accepts is an empty
message or sets `enable_tools` and/or `enable_reflection_agent` explicitly to
false.

The profile still identifies the field by number and wire type and checks its
payload bytes against the observed shapes below. It does not decode the payload
with the recovered type, and the recovered name appears nowhere in the
comparison or the attestation.

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
(`prompt_struct`, of type `genai.Struct`), which the reader compares like any
other field. The current descriptor reserves 23, and the daemon's RPC drops it.

The field's historical definition is known. The official agy CLI 1.0.4 and
1.0.10 release binaries both declare it as `prompt_struct`, of type
`exa.codeium_common_pb.Struct`
([declarations](../artifacts/historical-schema-recovery/provenance.json),
[chat.proto](../artifacts/historical-schema-recovery/1.0.10-proto/third_party/jetski/chat_pb/chat.proto)).
That type is a custom Struct: a list of `Field` entries, each a `name` (1) and
a `Value` (2), where `Value` holds one of null, number, string, bool, nested
Struct, list (members 1 to 6) or `Media` (member 7)
([codeium_common.proto](../artifacts/historical-schema-recovery/1.0.10-proto/third_party/jetski/codeium_common_pb/codeium_common.proto)).
It is not `google.protobuf.Struct`, and it is not the `genai.Struct` of field 25.
A shared field name does not make the encodings interchangeable, so the reader
does not treat field 23 as an older copy of field 25.

The profile still identifies the field by number and wire type, as for field
50, and parses the payload with `google.protobuf.Struct` rather than with the
recovered type. That is a strict reader for the narrow shape the profile
accepts, not a claim about the declared type. Within that shape the two encode
the same way: in both, Struct field 1 holds repeated entries of a string at 1
and a Value at 2, and Value members 1 to 6 have the same numbers and wire types
([google/protobuf/struct.proto](../artifacts/historical-schema-recovery/1.0.10-proto/google/protobuf/struct.proto)).
What the custom type allows beyond that is refused: a `Media` value arrives as
unknown field 7 and fails the no-unknown-field rule, and a repeated name, which
a list allows but a map merges, makes the payload re-encode shorter than
itself.

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

### What the recovered definitions establish, and what they do not

They give each field's authentic name and type in the 1.0.4 and 1.0.10
releases. They do not change any rule above. Both fields stay source-only:
the daemon's RPC drops them, so no definition could let full-raw equality cover
their bytes. Every location, shape, value, step and byte rule still applies.

The sources carry no build stamp, so which binary wrote any one session is
still unproven. The 2026-06-17 field-23 source, for example, was written before
1.0.10 was published.

A separate check decoded the payloads in four retained sessions with the 1.0.10
definitions: all 16 field-50 payloads as `ContextEngineConfig` and all nine
field-23 payloads as `exa.codeium_common_pb.Struct`, with no unknown fields
([results](../artifacts/historical-schema-recovery/recovered-payload-validation.json)).
The four sessions are not a separate sample. They are retained sources
(created 2026-06-02 to 2026-07-16), taken from the older-SQLite compatibility
sample. The nine field-23 payloads are 9 of the 47 occurrences above: six in
`conversation-021` and three in
`conversation-035`. The other 38 were not decoded with the
recovered type. The 16 field-50 payloads come from retained field-50 sources
(`conversation-005`, `conversation-021` and `conversation-079`), so they are part of the 78-source
population too.

So the check shows that the definitions fit the bytes of a subset of the
retained corpus. It is not independent corroboration, it is not a completeness
attestation, and it does not show that every historical trajectory decodes
under one version.

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
binary's SHA-256 that the descriptors came from. Suffixes identify additions
or restrictions to the qualified representation. Saved audit manifests from
an earlier profile cannot certify the new rules. For example, the historical
`permission1` suffix refuses a native `RUN_COMMAND` step whose permission
record has no copy on its wrapper, and `permission2` refuses a native `VIEW_FILE`
step in the same case. The newer combined `tools143-permission1` suffix
refuses permission grants and completed interactions on modern tool wrappers;
it does not replace or relax those historical native-Step rules. The `planner50` and `prompt23` suffixes mark the
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
  Wrapper and native status must match except for the qualified command
  cases. A run-command wrapper `CANCELED` over a native `ERROR` with error
  details, or a native `RUNNING` without an error, is presented with the outer
  `CANCELED` status. Other canceled/native status combinations fail. The
  separate command formatting failure below presents `ERROR` over a stored
  `DONE` command.
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
  code-action, view-file, generate-image, grep-search, MCP-tool,
  read-URL-content, search-web and list-resources. Except for the command
  formatting failure below, that Step must also be `ERROR` and its own error
  must have the same short error. The ordinary response presents the native
  Step with its inner error. The outer error may hold only
  short and full error text; it is dropped from the presentation and still
  covered by full raw equality. A failed `generate_image` call embeds a native
  image Step instead of a `GenerateImage` Any and is qualified the same way,
  with its prompt, image name and aspect ratio arguments. Successful embedded
  image Steps use a separate mapping that requires both statuses to be `DONE`
  and the wrapper's single media item to match the native generated media.
  An outer error over an invoke-subagent Step, or over any other native type,
  fails; a type joins this list only with an observed source, full RPC and
  default RPC triplet.
- A failed `list_resources` call is qualified only as an `ERROR` wrapper over
  an `ERROR` native `LIST_RESOURCES` Step with the same short error. The
  `ServerName` argument must equal the native `server_name`, and all wrapper
  arguments must equal the native metadata's tool call. The native resource
  body may contain only that server name; the wrapper result may contain only
  text, the native payload and an output URI. Task details, permission records,
  interaction records and successful list calls remain unqualified.
- A `run_command` bridge formatting failure stores a successful `DONE` native
  command inside an `ERROR` wrapper. The wrapper's short and full errors must
  match and start with
  `bridged adapter run_command: to string: template execution error: `.
  Its result contains only the native payload. The ordinary response presents
  that payload with status `ERROR`, preserving all
  native command output and metadata while omitting the wrapper's error.
  Qualification requires exit code zero, a blocking sandboxed Bash command,
  a 5000 ms wait, and matching command, proposed command, working directory
  and tool-call arguments. Both steps must record the built-in tool source
  and one `allow` hook; their histories must be `PENDING, RUNNING, ERROR`
  outside and `PENDING, RUNNING` inside. Native errors, task details,
  permission or interaction records, other result fields and other command
  outcomes fail. Full raw equality still checks the omitted formatting error
  and wrapper metadata.
- A wrapper that failed before its tool ran stores arguments and an outer
  `error` but no result. With status `ERROR`, the ordinary response presents
  it unchanged for `write_to_file`, `view_file` without a recorded tool source,
  `find_by_name`, `list_dir` and `schedule`, with arguments bounded per tool.
  `write_to_file` is also accepted with status `INVALID`. A `generate_image`
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
- A resultless `run_command` or `view_file` rejected for missing
  `toolSummary` stays generic. Both error strings must be exactly
  `invalid arguments:\n- missing property 'toolSummary'` (where `\n` is a
  newline), the status must be `INVALID`, the tool source `BUILTIN`, and the
  history `PENDING, INVALID`.
  The arguments must equal the retained metadata tool call. The command shape
  requires `CommandLine`, `Cwd`, `WaitMsBeforeAsync`, `IsDaemon=false`,
  `RunPersistent=false` and `toolAction`; the read shape requires
  `AbsolutePath`, `StartLine`, `EndLine` and `toolAction`. A result,
  `toolSummary`, other arguments, task details, permission or interaction
  records, or another error fails. This mapping preserves the entire wrapper;
  it does not synthesize a native failed command or file read.
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
  error. It was observed once, in conversation `conversation-114`, when agy was
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
  not a standard file`). The arguments must equal the presented tool call,
  which still shows them, and
  are limited to the path and either both lines or neither, with
  `1 <= StartLine <= EndLine`. The path must be absolute, already clean, and
  made only of letters, digits and `/ . _ - @ +`, the characters observed
  copied into the URI unescaped. Another tool source, another status,
  an outer error with more than short and full text, any other argument,
  or another path or line shape fails with
  `unsupported resultless view_file`.

  The `history2` rule derives `is_skill_file` from an exact-path enabled skill
  registration saved by the current executor or a qualified earlier executor.
  To mark a read, the step's execution ID must select a current executor.
  A prior registration
  is usable only when the saved trajectory and cascade IDs and step index
  identify this exact source step, executor end indices strictly increase,
  every earlier executor ends before this step, and the current executor ends
  at or after it. Missing executor binding, no matching registration, or only
  disabled or non-skill registrations leaves the flag unset, including repeated
  registrations; registrations after the current executor cannot mark a read.
  When any enabled skill registration could mark the read, the current
  execution binding must be unique and the registration must be the only
  exact-path match in current or earlier executors. An earlier enabled skill
  registration also requires unique nonempty executor IDs and valid ordering
  through the current executor. Duplicate IDs do not prevent an unmarked read;
  unrelated duplicates do not prevent a marker from the current executor.
  Filenames and the current filesystem do not determine the flag. A different
  flag in the ordinary response makes the result incomplete.

  This resolves the earlier refusal of `conversation-098` (agy-reader#3193):
  its failed read at step 62 has a qualifying registration in a prior executor.
  The 2026-10-21 source/full/default check verifies all 355 steps. The
  [earlier recovery report](compatibility/recovery-rerun-2026-10-07.md)
  records the former limitation. Modern `ReadFile` payloads follow the separate
  `skill146` rule instead (see above): the flag comes from the stored path
  alone, as agy 1.4.6 sets it, and no executor registry is consulted.
- Payload-free `manage_task` and `send_message` steps remain generic, with
  bounded arguments and result fields. Task metadata keys are limited to the
  observed `task_description`, `task_id`, and `tool_name`, except for the
  qualified `send_input` shape below. A failed `manage_task` step with status
  `ERROR` may also carry an outer `error`, which the ordinary response presents
  unchanged. An outer error on any other
  payload-free wrapper fails except for the schedule case below.
- A payload-free `manage_task` with `Action=send_input` remains generic when
  it is `DONE` and has nonempty `TaskId` and `Input` arguments matching the
  retained metadata tool call. Its result metadata must contain exactly
  `input`, `task_id`, `task_description` and `tool_name`: input and task ID
  must repeat the arguments, and the target tool must be `run_command`.
  Qualification requires the built-in tool source and exactly one `allow`
  hook. Result text, output URI and render information are retained. Empty or
  mismatched input, other actions or target tools, extra metadata, outer
  errors, task details, permission or interaction records remain unknown.
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
based on definitions extracted from the agy 1.3.4 binary with SHA-256
`5b93565c5f04ea4f7b79d49d5c3237449bf6ce47b6779e8cf46c1fae97957c98`.
The second root supplies `exa.cortex.core.tools.GenerateImage`, which travels
inside an Any and is absent from the trajectory's ordinary import closure.
The retained base, `internal/completeness/schema-base-agy134.pb.gz`, contains
31 descriptors (137,394 compressed bytes), with SHA-256
`90bd669c32e044ef769763f3e4dc5b27fb187972cc9ff518189ff3933ed8fd92`.
All 343 extracted records were checked against bytes at their manifest offsets
in that binary before generation. The bundle contains schema definitions only,
no conversation payloads or credentials.

The bundle adds exactly five declarations from the verified 1.4.5
binary `8af8e7a2a68f72fa7164e00bb59b8902c018fa43e528fc401ad7db8c2972583c`:
`AntigravityBrowserToolConfig.enable_browser_subagent_auto_invoke`,
`ViewFileToolConfig.skip_plugin_logo`, `ToolOutputConfig.per_tool`, its nested
`PerToolEntry`, and `ToolOutputOverride`. That intermediate bundle has
137,843 compressed bytes with SHA-256
`941e3c1eb080db629d2d627b15c7e268d076ed7f43acb9578bac82b0e5ee3aed`.
Every original definition remains intact, including historical
`PolicyGuardianConfig.use_v2` field 14, which a full 1.4.5 replacement would
remove. BASH and other intervening additions are not enabled.

The `compaction146` overlay adds six declarations from the verified 1.4.6
binary `128780f5776959ba6a991bde1f0a8b699eee37a59ed30deb9926147fedfa56c4`:
`CortexStepCheckpoint.captured_user_requests`, `CapturedUserRequest`,
`CheckpointConfig.context_window`, `CheckpointConfig.max_user_request_media_tokens`,
and `CustomizationDiscoveryConfig.kind_enablement` with its nested map entry.
The customization map is a necessary prerequisite: real 1.4.6 compaction
conversations carry it in planner and executor configuration. It is compared
against both RPC responses, not treated as source-only content. The existing
`Media` and `Blobref` definitions match the snapshot and remain unchanged.

The intermediate 1.4.6 bundle contains 31 descriptors, 137,988 compressed bytes
with SHA-256
`3d9a1ebb2ae55b316ab35cf19875f50d9f9c568ff16d8a04b95c90703ef026fa`.
Every earlier declaration remains intact, including legacy checkpoint
`user_requests`. The new request text and media participate in exact native,
full-RPC, and default-RPC comparison. Native checkpoint steps need no new
projection rule. Both qualified maps are checked before protobuf decoding can
discard unknown entry fields or duplicate keys; unrelated new fields remain
unsupported.

The `finish147` overlay adds only `exa.cortex.core.tools.Finish` from the
verified 1.4.7 snapshot. The current bundle contains 31 descriptors, 138,024
compressed bytes, with SHA-256
`12b303d5dc3cd50630f82a48a3e1616863067b7448a6283b241a55c1434b97b9`.
Every earlier declaration remains intact. The new projection supports completed
Finish calls with a result schema, and the same capture qualifies missing-file
read wrappers. The [offline export follow-up](compatibility/agy-1.4.7.md#offline-export-follow-up)
records their bounds and the remaining command wait-field mismatch; this does
not establish full 1.4.7 compatibility.

Rebuild the current bundle from the pinned base and all three snapshots:

```sh
python3 internal/completeness/generate_finish147.py \
  artifacts/proto-snapshots/agy-1.4.5-8af8e7a2a68f/extracted \
  artifacts/proto-snapshots/agy-1.4.6-128780f57769/extracted \
  artifacts/proto-snapshots/agy-1.4.7-40349135371a/all-extracted.protoset \
  internal/completeness/schema.pb.gz
```

This authenticates the prior inputs, all 380 records in the 1.4.6 snapshot,
and the pinned 1.4.7 descriptor-set hash. It preserves optional-field presence,
loads the resulting pool, and writes atomically. Maintenance tests require
exactly six additions over the intermediate 1.4.5 bundle, then only the Finish
message over that 1.4.6 bundle, with no other changes.
The [native qualification report](compatibility/agy-1.4.6-compaction.md)
records the observed attachment representations and configuration values.

To reproduce only the historical 1.4.5 intermediate bundle, use a separate
output file:

```sh
python3 internal/completeness/generate_options145.py \
  artifacts/proto-snapshots/agy-1.4.5-8af8e7a2a68f/extracted \
  /tmp/schema-options145.pb.gz
```

That generator checks the base and manifest hashes, the manifest's binary
identity, and all 379 extracted record hashes, lengths and names. It copies the
authentic declarations and their optional-field presence, loads the resulting
descriptor pool, and replaces the output atomically. Maintenance tests require
an exact five-addition delta and no unrelated changes.

The original `generate_schema.py` workflow below generates a whole-release
base, not the current augmented bundle. Replacing the bundle with a newer
release requires checking removals and historical decoding as well as additions.
The generator consumes a separate directory of `FileDescriptorProto` records
and a manifest declaring that hash and the hash of each extracted record.
It verifies those hashes, drops five named annotation-only files whether or not
the extractor found them, removes the imports and extension declarations that
refer to them, and writes deterministic gzip output. Any other missing import
stops it.
The descriptor delta from 1.3.0 adds `DefaultAgentConfig.surface` (field 5, enum
`AgentSurface`), `RunCommandToolConfig.enable_fast_output_close` (field 24, bool),
`TaskDetails.moved_to_background_at` (field 8, Timestamp),
`McpServerSpec.plugin_auth_provider` (field 30, string), `CortexStepMcpTool.app_view`
(field 14, McpAppView), `CascadeNUXConfig.session` (field 22, enum),
`HookArgsCommon.caller_context` (11) and `root_conversation_id` (12), and
`StepRenderInfo.TitlePart.agent` (field 5, AgentRef).
These schema definitions are extracted directly from the binary descriptors;
not all additive fields are exercised by the native audit scenarios. The
1.3.4 profile includes the extracted subagents and background task schema additions.

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

## Legacy encrypted protobuf sources

Legacy `.pb` conversations now use native verification through the same fetch,
cache-write, watcher and doctor paths as SQLite. They no longer rely on sidecar
mtime. When both extensions exist for one conversation, SQLite still wins.

The supported envelope is a 12-byte nonce, AES-256-GCM ciphertext and a 16-byte
tag, with no additional authenticated data. The authenticated plaintext is a
serialized `gemini_coder.Trajectory`. Authentication precedes protobuf decoding;
invalid tags, truncation, oversized files, unknown fields and unsupported
projections fail closed. The 64 MiB plaintext and existing recursion limits apply.
The reader never decrypts into a file or changes the encrypted source.

`legacy_key.bin` is the fixed format key distributed in agy, not an account key
or user credential. Static inspection of agy 1.4.4 binary SHA-256
`337dcde668441a2828968644d7982829c2d2a5e0c4144f346e1f3fc6abb3cdc7`
identified the key copy at virtual address `0x8d77ee6` from file offset
`0x5807f90`, passed to `WithEncryptionKey` (`0x853ec40`), and the corresponding
`DiskSaver.loadUnlocked` (`0x8540940`). The compiled reader embeds those 32
format bytes; it does not depend on binary offsets or an installed agy binary
at runtime. Unknown encryption keys/layouts remain unsupported.

`LegacyProfile` prefixes the unchanged SQLite schema/projection profile with
`legacy-aes256gcm-v1-agy-1.4.4-337dcde66844/`. Its source digest covers the profile
and exact encrypted file, including nonce and tag. A rewritten ciphertext
invalidates evidence even if the decoded trajectory is unchanged. SQLite
attestations remain valid under their existing profile. Legacy sidecars without
attestations are re-fetched and verified against full/unhydrated and default
responses from one endpoint, bracketed by native source snapshots.

Audit-sweep manifest version 5 counts both formats and records their distinct
profiles. `verified` and `verifiedSourceOnly` cover both formats; `sqlite` and
`protobuf` count sources. `notVerified` counts failures across both formats,
and `legacyUnverifiable` is the legacy subset of those failures. Audit-qualify
requires version 5 and rechecks each source's current profile and digest.
Legacy watcher checks use the same bounded metadata reuse as SQLite, keyed by
both format profiles. Source or sidecar changes, expiry, and a sibling SQLite
file force another native check. Legacy authentication/schema refusals also
reuse unchanged source metadata within that bound. Doctor and one-shot reads
always check native content. No source-only field allowances were added.

## Direct offline export

`agy-reader export-offline [--format=json|md] SESSION.db` uses the native SQLite
snapshot reader and the same qualified presentation mapping used when comparing
RPC responses. It does not fetch a response or mint an attestation. A successful
export establishes that this reader can decode and project the selected source;
it does not establish a fresh match with agy's daemon.

JSON stores that distinction under `agyReader.offline` (version 1), with the
snapshot profile and digest. Declared source-only records retain their bytes,
locations, counts and hashes under `sourceOnlyFields`. There is no
`agyReader.completeness` record. Markdown has a visible offline notice. These
exports do not change the sync/cache verification rules or profile identifiers.

See the [offline export usage](../README.md#offline-export) for WAL handling,
output protection and scope. An opt-in regression test compares direct exports
with an independently captured agy 1.4.4 audit-sweep corpus:

```bash
AGY_READER_OFFLINE_MANIFEST=/private/sweep/manifest.json \
AGY_READER_OFFLINE_SOURCES=/private/conversations \
  go test ./internal/cli -run '^TestExportOfflineCorpus$' -count=1 -v
```

The test checks every SQLite source's profile and digest against the manifest,
then compares the entire parsed JSON after removing reader provenance and making
an omitted empty `steps` array explicit. It compares Markdown rendered from the
saved daemon JSON with the command output, excluding only the generation time
and offline notice. Both sides render only the selected session.

The [agy 1.4.4 comparison record](compatibility/agy-1.4.4-offline-export.md)
records the corpus coverage and representation differences.
