# agy 1.2.16 format and feature audit

The initial 2026-10-02 audit found that **agy 1.2.16 completes the new image-subagent
and background-task scenarios, but the reader cannot sync them**. The prior,
uncommitted 1.2.15 fix is also insufficient. `COMPATIBILITY.md` remains at
**1.2.14**. That initial audit was a failed qualification; no reader implementation,
installed binary, or service was updated during it.

The subsequent [reader qualification](#reader-qualification-follow-up) passes
the frozen cases through native verification, ordinary sync, and a fresh
watcher. The audit observations below remain the original results.

The strict sweep fetched and rendered **274/274 conversations**, containing
**24,615 steps**, and the explicit-corpus shell audit exited **0**. Separately,
all three frozen test conversations failed native verification and ordinary
sync on both reader builds. Successful fetching and rendering do not establish
that the reader can certify and persist those conversations.

The [sanitized evidence](agy-1.2.16-evidence.json) records exact results. The
[shape snapshot](agy-1.2.16-sidecar-shape.tsv) is an observation, not an accepted
compatibility baseline. Raw conversations, images, terminal captures, helper
scripts, and logs are retained privately outside the repository.

## Changelog risks and selected scenarios

The installed CLI's `agy changelog` reports no SQLite, encryption, or explicit
RPC-format change in 1.2.16. Three notes nevertheless affect reader coverage:

| Release behavior | Scenario and observed result |
| --- | --- |
| Image requests now use the built-in `image-generator` subagent and save artifacts | Requested one blue owl on a cream background. The parent invoked the built-in child, which generated an image artifact; both conversations were captured independently. |
| Killed background commands now finish with an exit code instead of stalling the agent | A disposable Python process printed a marker, slept eight seconds, and sent SIGKILL only to itself. The task reported exit **137**, followed by a successful foreground recovery command. |
| Headless `-p` runs wait for background work after the first turn | An 18-second background command remained running after the first response. Completion triggered another execution, a recovery command, and a final response before process exit **0**. |

The killed-command fixture used:

```sh
python3 -c 'import os,signal,time; print("AUDIT_1216_CRASH_STARTED",flush=True); time.sleep(8); os.kill(os.getpid(),signal.SIGKILL)'
```

The child made one visible image-generation call. Its parent/root identifiers,
the invocation result, and the parent's received message agree. Its config
sets `saveToParentArtifactDirectory=true`, and the returned artifact is in
the parent's artifact directory. The file decodes as **JPEG 1024 × 1024**,
**362,630 bytes**, SHA-256
`d1f075e575f66610ab7b160f0e5c133be4ecae7ba993907aef8443d6fc6964a7`.
No visible generation retry or child image-view call occurred. Server-internal
retries and image-composition quality were not measured.

The killed-command output also contains a sandbox read-denial warning; this
was not an error-free sandbox test. Exit 134 was not exercised.

The headless scenario deliberately ended its first response with `WAITING_1216`
and did not poll. Persisted timestamps show that response completing at
`2026-10-03T04:21:40.824357107Z`; the background task completed at
`04:21:47.292511368Z`, **6.468 seconds later**. The completion notification was
created at `04:21:47.479681808Z`, the recovery command completed at
`04:21:54.726306977Z`, and the final response completed at
`04:21:58.232613289Z`. The stream also explicitly reported waiting for one
background task. Its `num_turns: 1` result counter does not describe these
execution boundaries; event emission order alone also differs from step time.

Twelve terminal resize requests cycled through four sizes, followed by a
successful `RESIZE_FOLLOWUP_1216` command. A separate post-sweep resume of the
completed headless conversation printed `RESUME_1216_OK`, recalled exit 0,
and exited 0. That resumed database has 13 steps; the original eight-step
fixture and sweep remain unchanged. These are short-session smoke checks,
not evidence for long-conversation responsiveness or altscreen performance.

The remaining notes concern config shortcuts, mouse selection, permission
suggestions, queued-message ordering, customization discovery, and terminal
or settings handling. They declare no storage change. Queued-message ordering,
permission suggestions/denials, parent customization manifests, Windows BOM
handling, and long-history performance were not exercised. Permission
auto-approval was enabled for these controlled fixtures.

## Process and reader provenance

The existing `scripts/agy-pair.py` launched fresh authenticated agy/reader pairs
using `ANTIGRAVITY_CSRF_TOKEN`, with root model `gemini-3.8-flash-high` and high
effort. The serving interactive executable matched the installed 1.2.16 binary:

```text
a759ce7c7a235d9b6c281a25ead97cbbf2e92314a3ffd224e2f9144f3fae7a86
```

The retained daemon returned **401** without its token and **200** with it.
Readers watched private copied roots. agy itself still wrote conversations
into its normal CLI store; the helper's `--root` only selects the reader store.
The broad sweep reads that normal store and can cause daemon loads, but does
not write live sidecars. No installed watcher was restarted.

The baseline reader was built from clean commit
`0827ee961219ada4f64ff22070d7f205660c8163`, whose reader code is unchanged from
`87dda0492d8d969efb8cd41d0aa5f07b4453e4ec`. A second build used the six-file,
uncommitted 1.2.15 fix with profile `sqlite-v1-format0-agy-5f9c16b28689`.
Its exact patch hash is in the evidence record. It was a comparison build,
not the committed baseline or an installed reader.

## Native qualification failures

All frozen databases pass SQLite integrity checks. Database, full RPC, ordinary
RPC, and sweep counts agree:

| Conversation | Steps | Baseline native test / ordinary sync | Prior 1.2.15 fix |
| --- | ---: | --- | --- |
| Image parent, killed task, resize follow-up | 21 | `unsupported decomposed step representation` | Same failure |
| Image-generator child | 6 | `unsupported decomposed step representation` | Same failure |
| Headless late completion | 8 | `unknown persisted protobuf fields` | Same failure |

Both native test variants exit 1 for each case. Both normal-sync variants exit
1 and write no sidecar. The baseline watcher and a separate fresh watcher using
the prior fix each report three failed sources and write zero sidecars. The
headless pair's watcher exits with its CLI; the final headless source was
therefore tested independently through the retained authenticated daemon,
avoiding confusion between that lifecycle and reader rejection.

Two confirmed boundaries explain the initial failures:

- **Decomposed render metadata:** parent rows 6 and 10 and child rows 2 and 4
  have nonempty `steps.render_info`. `readSteps` rejects any such value before
  inspecting the payload. These rows still use `step_format=0` and have
  `has_subtrajectory=false`; this is not a schema or format-number bump.
  agy's diagnosis and an independent decode confirm that each column is
  byte-identical to the nested `generic.result.step_render_info`, not a
  top-level Step field. A fix must verify that redundancy rather than ignore it.
- **Descriptor drift:** the actual 1.2.16 descriptor closure adds seven fields
  and one message versus the prior fix, with no removed or changed field
  signatures and no enum changes. A diagnostic Go overlay replacing only that
  fix's descriptor blob verifies all **eight headless steps**. This isolates
  the headless failure to descriptor coverage without changing tracked source.
  Its unknown executor fields include `CascadeToolConfig.list_plugin_accounts`
  (field 57) and loaded customization `version` (field 8).

The seven additions include image generation's
`save_to_parent_artifact_directory`, render metadata's `show_on_error`,
`CascadeToolConfig.list_plugin_accounts` and its `enabled` field, loaded
customization `version`/`installed_from`, and plugin `skills`. The 30-file
closure also contains 68 deprecation-only field-option changes versus 1.2.15.
Not every new field was exercised by these scenarios.

The image workflow also exposes projection cases beyond the first rejection:
`invoke_subagent`, `manage_task`, and `send_message` arguments are outside the
current allowlist. Some generic results have render information and no native
Step payload. Image generation carries an
`exa.cortex.core.tools.GenerateImage` Any payload rather than a
`gemini_coder.Step`. That image message is absent from the current 30-file,
trajectory-only descriptor closure and needs explicit inclusion for Any
resolution. Merely regenerating that closure does not qualify the image case.
These observations identify required work; they are not passing tests of a
new implementation.

## Rendered image details are missing

Actual flat Markdown output from the frozen RPC payloads reveals a separate
loss that the sweep's render-success check does not detect. The child's image
step renders only:

> *(No additional details stored for this step type)*

The native payload contains the prompt, model, aspect ratio, image name, and
media URI/MIME. None appears in that step's output. The parent's invocation
section similarly contains only a generic delegation paragraph, without the
stored child identity, type, role, or result-log URI. Artifact text elsewhere
in the transcript can conceal these omissions in a whole-transcript check.
These observations cover flat Markdown, not tree child resolution or downstream
agentsview ingestion. They are separate from the existing image-view rendering
issue documented in the 1.2.15 audit.

## Corpus results and follow-up work

The sweep's selection was captured before the later diagnostic and resume
work. All 274 selected sources were freshly serialized through the verified
1.2.16 daemon and rendered. The shell helper passed the reader unit suite;
the separate agentsview consumer suite and live downstream re-ingestion were
not run.

SQLite retains user version 1, seven tables, and its two explicit indices.
Its fingerprint remains:

```text
sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43
```

The observed sidecar shape is:

```text
sha256:a61acb40bdb6f3483aae5825a3ceea9241471606a25219857dec59a87975f5b7
```

Compared with the 1.2.15 audit snapshot, 18 path/type entries were added and
none removed. They include the image child's parent-artifact configuration and
other agent configuration paths. Corpus coverage and configuration can change
this union; the delta alone does not establish a breaking render-contract
change. Opaque executor/generator metadata remains outside shape hashing.

| Issue | Recorded work |
| --- | --- |
| `agy-reader#vj2t` — Qualify agy 1.2.16 render metadata and image-agent projections | Recover the new descriptors, validate decomposed render metadata, and qualify each observed generic projection while retaining strict rejection and full raw equality. Require native, sync, and watcher checks for parent, child, and headless cases. |
| `agy-reader#8rst` — Make format audits detect unsupported normal reader sync | The shell helper again offered a passing record while all three native cases failed. Preserve broad fetch/render inspection but gate qualification on actual reader verification. |
| `agy-reader#9re0` — Render generated-image artifacts and subagent identity | Render the native image and invocation details and test each step independently of later assistant text. |

No `--record` run was performed. The compatibility record and prior uncommitted
fix remain unchanged. This audit does not claim complete 1.2.16 support.

## Reader qualification follow-up

The reader now qualifies the three frozen 1.2.16 cases with profile
`sqlite-v1-format0-agy-a759ce7c7a23`. This incorporates the earlier 1.2.15
metadata, media, and background-task fixes. `COMPATIBILITY.md` remains at
1.2.14: this bounded qualification does not promote the fetch/render sweep
into proof of compatibility for the full corpus.

The updated descriptor closure includes the image tool's Any payload type.
The decoder validates the redundant render column against the nested generic
result, including presence and unknown fields. Projection rules separately
qualify native subagent steps, unchanged task/message generic steps, and the
image tool's native presentation. See [the supported profile](../completeness.md)
for the exact bounds and intentionally unsupported shapes.

Verification used disposable copies of the frozen sources and the retained
1.2.16 authenticated daemon. Its executable hash was rechecked against the
hash above; ordinary and full RPC responses still equalled the frozen inputs.

| Case | Native steps verified | Normal sync | Fresh watcher |
| --- | ---: | --- | --- |
| Image parent | 21 | equal ordinary RPC, valid attestation | equal ordinary RPC, valid attestation |
| Image child | 6 | equal ordinary RPC, valid attestation | equal ordinary RPC, valid attestation |
| Headless background task | 8 | equal ordinary RPC, valid attestation | equal ordinary RPC, valid attestation |
| Prior 1.2.15 WebP/background case | 12 | not rerun in this qualification | not rerun in this qualification |

The new watcher started with an empty copied root, then detected all three
new database files and wrote three sidecars with zero failures. Doctor checked
three verified SQLite sources, zero incomplete, and zero unknown in each root.
Its overall exit remained 1 because the recorded compatibility version is
still 1.2.14. All 30 retained frozen evidence files kept their original hashes.
No installed reader, normal watcher, or live conversation sidecar was changed.

`go test -count=1 ./...` and `go vet ./...` pass. Synthetic regressions cover
render-column contradictions and missing/unknown content; unsupported tools,
arguments, and Any payloads; changed generic text/media; image projections,
lineage and artifact ownership; and conflicting background task details.
Private native inputs and runtime logs remain outside the repository.
