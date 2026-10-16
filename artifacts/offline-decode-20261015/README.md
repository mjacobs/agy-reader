# Offline trajectory experiment and schema observations

Five real CLI trajectories decoded with the saved agy 1.3.6 schema, Python
protobuf 7.36.2, and Python SQLite. No agy daemon calls or agy-reader process
were involved. The experiment used the repository's storage mapping as a
reference, not its Go decoder or presentation implementation.

## Result

| Conversation prefix | Stored / sidecar steps | Directly equal steps | Equal after bounded presentation |
| --- | ---: | ---: | ---: |
| 0ab4e2e5 | 2 / 2 | 2 | 2 |
| 1a512cb5 | 86 / 86 | 46 | 86 |
| 31003b9e | 119 / 119 | 71 | 119 |
| f7cfba81 | 646 / 646 | 422 | 646 |
| 21657389 | 632 / 632 | 338 | 632 |
| Total | 1,485 / 1,485 | 879 | 1,485 |

All non-step trajectory fields match after typed JSON normalization. One of five
whole raw trajectories matches directly. In the other four, 606 stored GENERIC
steps differ from the sidecar's named tool steps. Their embedded Any payloads
contain the native Step: 570 match directly; the other 36 differ in taskDetails,
with six also differing in status. Selecting the embedded Step and carrying over
the outer taskDetails/status makes every step's decoded fields match. Nothing
else is dropped from those selected steps for this comparison.

That presentation transformation is deliberately bounded to this sample. It
omits the generic wrapper, which can contain additional stored information. It
is not a lossless archival conversion or permission to accept every wrapper in
a production completeness verifier. Keep the raw records as well as any view.

`results.json` records per-file SHA-256 values, exact comparisons, differences,
unknown-field checks, Any payload checks, and source stability. `probe.py` is the
reproducible experiment. Run with Python protobuf 7.36.2; alternatively:

```sh
UV_CACHE_DIR=/tmp/agy-uv-cache uv run --with grpcio-tools==1.84.0 python artifacts/offline-decode-20261015/probe.py
```

The five samples were selected from recent idle DB/sidecar pairs: minimal chat,
file/command use, subagent invocation, and two conversations with checkpoints.
They were not selected randomly and their original producer binary versions
were not established. Success shows that this schema decodes these historical
records; it does not establish release-wide compatibility.

## Preservation and boundaries

The script reads original DB, WAL and sidecar bytes, copies them into a private
temporary directory, and checks originals again before and after decoding.
SQLite opens only the copies with mode=ro and query_only, in a read transaction.
All original content hashes remained unchanged. The temporary copies are deleted
on exit. No private prompt/output text is retained in this report or JSON results.

The copies passed SQLite quick_check. Step indices, metadata indices, storage
format, redundant step headers, metadata/error/permission/task columns,
render-info presence/content, and generator-metadata byte lengths were checked.
All five selected WAL files were empty, so this run does not test nonempty WAL
recovery, concurrent writers, encrypted legacy files, or decomposed steps.

A sidecar is an existing comparison artifact, not an independent oracle for
all semantics. This test compares decoded known fields and checks for unknown
wire data; it does not establish exact original provider requests or model behavior.

## Non-obvious schema observations

These describe the agy agent harness and its provider interfaces. They do not
reveal Gemini's neural architecture, training, or private reasoning. Field
presence establishes representational capability, not that a feature is enabled.
Paths below are relative to the saved snapshot's `proto/` directory.

1. **A visible tool call can be the endpoint of a repair loop.**
   `third_party/jetski/cortex_pb/cortex.proto:2484` separates model-output retries
   from API retries. The former can retry invalid arguments and either forbid
   tool use or force a named tool on the last attempt. The latter has backoff and
   error-feedback settings. Thus an apparent change of strategy might come from
   harness instructions during recovery, rather than a spontaneous model choice.
   This is an inference about supported behavior, not a measured retry in these
   samples. Populated retryInfos does not by itself count actual retries.

2. **The transcript is not necessarily the prompt sequence.**
   `third_party/jetski/chat_pb/chat.proto:213` has step_idx and merged_step_idxs;
   `third_party/jetski/cortex_pb/cortex.proto:1526` permits tool-result wrapping,
   appending ephemeral messages to tool results, and system-message merging.
   `cortex.proto:5992` defines heuristic-controlled ephemeral messages and their
   persistence level. A viewer cannot assume one displayed step equals one model
   message. Nonempty mergedStepIdxs occurs in two of the sampled conversations.

3. **Compaction is a configurable state transfer, with its own failure modes.**
   `cortex.proto:3691` records summary failure/fallback, retained user requests,
   subagent/task snapshots, artifact snapshots and accumulated sections.
   `cortex.proto:3747` allows a checkpoint model, using the last planner model,
   output limits, per-category limits, retries, and fallback. Context loss could
   therefore reflect summarization policy or fallback, not only a context-window
   limit. Two checkpoint steps in this sample match the sidecars directly; the
   experiment does not establish that fallback was used.

4. **Reasoning-related text and continuation data are distinct.**
   `chat.proto:213` distinguishes thinking, raw_thinking, thinking_signature,
   thinking_redacted, passback_context and side_channels. The useful conclusion
   is that displayed text alone may not preserve all provider-facing state.
   Signatures are present in these samples, but their cryptographic meaning and
   replay requirements are not established by the field names. A raw_thinking
   field is not evidence of access to a model's complete internal reasoning.

5. **Model identity has more than one layer.**
   `cortex.proto:1093` separates model, model_display_name, response_model and
   response_model_full, alongside usage, timing and retry records. A learning
   view could distinguish a selected label from a response-reported identity.
   responseModel is populated in all five samples; this does not demonstrate
   that routing or fallback changed the model.

6. **Instructions compete for explicit budgets.**
   `cortex.proto:3114` defines customization_token_budget and rules_token_budget.
   The same planner configuration contains loop-detection controls and a
   loop_error_model_message. This suggests useful investigation questions about
   omitted instructions and harness-mediated loop recovery, but establishes
   neither the allocation algorithm nor the defaults or actual activation.
