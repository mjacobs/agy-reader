# agy 1.2.14 format and feature audit

The 2026-09-29 audit fetched and rendered all **245 selected CLI conversations**
through a verified agy 1.2.14 daemon, with zero fetch or render failures. The
SQLite schema is unchanged. The trajectory fingerprint adds five paths from
new structured-output coverage, with no removed paths. Fresh file-rejection
and resume scenarios passed, but inspection found two Markdown rendering gaps:
failed file views omit their error details, and structured-output `FINISH`
steps omit their result payload. Both payloads survive in raw sidecars.

The reader code under audit was `fd9df25`, with a clean working tree before
recording. [COMPATIBILITY.md](../../COMPATIBILITY.md) records 1.2.14 and the
expanded corpus fingerprint. This qualifies the storage, authenticated RPC,
and raw-payload checks; it does **not** assert complete Markdown rendering.
No reader or test code was changed. The local installed reader was rebuilt
with the new baseline and its systemd watcher restarted.

## Daemon and authentication

The older agy session was stopped with the user's permission. A fresh tmux
session launched the installed binary with an explicit `--csrf_token` as its
first argument. Both initial and resumed serving processes matched the
installed executable's SHA-256:

```text
0d0d3eba22daf29504dd290151c7ed9a4d33b0c6aa0acfc5da27bc3b01d2f029
```

The endpoint came from the serving process's own open log and listener.
`GetCascadeTrajectory` for the scenario returned **401 without CSRF** and
**200 with the matching header**. Explicit
`ANTIGRAVITY_DAEMON_URL` / `ANTIGRAVITY_CSRF_TOKEN` values bound the sync,
roundtrip, and strict sweep to the verified daemon. The sweep manifest's
endpoint agrees with that process. The watcher also discovered the launch
token while agy was idle, including after restart.

The initial harness placed the daemon log outside the session root, so
automatic discovery reported no daemon. Moving the open log into the CLI
root's supported `log/cli-*.log` location restored discovery. The resumed
process used agy's default log location. No persistent endpoint or token
override was added to the watcher service.

## Changelog-focused scenarios

The delta is 1.2.14 only. **No note declares a SQLite schema, encryption, or RPC
format change.** File rejection, missing-step resume repair, attachment-based
titles, and JSON-schema handling deserve payload inspection because they
affect persisted input, errors, or completion output. Queue configuration,
remote-control startup, sign-in messaging, and cursor rendering are mostly
CLI behavior; long-history loading is a performance claim.

The main scenario used an isolated text fixture containing `café`, `中`, and
`👩‍💻`, values 3/5/8, a FIFO with no writer, and truncated media. It ran with
Gemini 3.8 Flash at low effort, without subagents or network tools. Initial
relative file references caused agy to run broad filesystem searches; those
tasks were canceled, and later prompts supplied exact absolute fixture paths.

| Check | Observed result | Private evidence file |
| --- | --- | --- |
| FIFO view | Rejected as a device, pipe, or socket; subsequent command returned `RECOVERED_1214 16` | `tui-initial.txt`, `raw-final.json` |
| Truncated MP4 | A recognizable 128-byte MP4 was rejected for a truncated `moov` box; the next command returned `MEDIA_RECOVERY_1214 16` | `tui-media.txt`, `raw-final.json` |
| Resume after process restart | Recalled the sum, Unicode labels, and both errors; ran another successful command | `tui-resumed.txt` |
| History preservation | All 44 pre-restart steps remain identical within the final 51-step trajectory | `roundtrip-results.json` |
| Reader roundtrip | Direct RPC equals the live sidecar after removing reader-owned metadata; database, sidecar, and sweep each contain 51 steps | `roundtrip-results.json`, `corpus-summary.json` |
| Invalid JSON schemas | Plain text, bare type name, missing file, and non-object root each fail at startup with exit 1 and a schema-specific error | `schema-startup-results.json` |
| Valid object schema | Exit 0; `structured_output` is exactly `{"marker":"SCHEMA_VALID_1214","sum":16}` | `schema-valid-output.json` |

The first 20-byte media fixture was not recognizable as MP4 and was read as
text. It is **not** evidence for the media-rejection fix. The replacement was
the first 128 bytes of a genuine ffmpeg-produced MP4; `file --mime-type`
identified it as `video/mp4` before the view attempt.

A follow-up typed during the first turn appeared queued and ran after that
turn finished. This is an observation of default queue behavior, not a
controlled comparison with `Send Immediately`. Missing-step corruption was
not injected, so ordinary resume preservation does not prove the gap-repair
fix. Thousands-of-steps CLI replay performance, attachment-only titles,
socket/device views, MOV/M4A rejection, cursor editing, remote control, and
sign-in errors were not tested. No dollar cost was exposed and no worker
fleet was launched.

## Corpus and format results

The strict sweep manifest has `complete: true`. Every selected conversation
was fetched with `LoadTrajectory` / `GetCascadeTrajectory` and rendered,
without cached-sidecar fallback. Unsupported implicit traces were excluded.
Raw corpora remain outside the repository.

| Check | Result |
| --- | ---: |
| Selected / freshly fetched / rendered conversations | 245 / 245 / 245 |
| Fetch or render failures | 0 |
| SQLite / legacy protobuf files | 230 / 15 |
| SQLite `quick_check` | 230 / 230 `ok` |
| Schema version / table count in every SQLite file | 1 / 7 |
| Populated / empty trajectories | 223 / 22 |
| Steps / distinct step types | 22,292 / 22 |
| Canonical path/type entries | 1,741 |
| Added / removed paths versus the prior snapshot | 5 / 0 |
| Standard `go test ./...` | PASS |
| Audit preflight / recording | PASS / PASS |
| Separate agentsview tests / live ingestion | Not run; local checkout absent |

Schema fingerprint, **UNCHANGED**:
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
Sidecar fingerprint, **DRIFT**, with `explicit-version-scoped` provenance:
`sha256:c60cd3ee72dfdeb7e76f95c46f8c2b6a4ad04112ec963666789e7372cc18fb4a`.

The [new canonical snapshot](agy-1.2.14-sidecar-shape.tsv) differs from the
[previous snapshot](agy-1.2.10-sidecar-shape.tsv), also used for 1.2.13, by:

```text
steps.[].finish                                      object
steps.[].finish.output                               object
steps.[].finish.output.arguments_json                string
steps.[].finish.outputString                         string
steps.[].userInput.userAgentSpec.builtinAgent.defaultAgent.finishToolSchema string
```

These additions come from the valid JSON-schema scenario. They are new to
this corpus, not proven new in 1.2.14. The helper correctly signals structural
drift, but its explanatory sentence that the daemon "reshaped" JSON is more
definite than this evidence permits. Corpus feature coverage remains part
of interpreting drift. Opaque configuration subtrees remain excluded.

## Findings from using the audit skill

The schema check alone missed the added paths. The shape check caught them,
but a successful strict sweep only proves that rendering returns no error.
It does not prove that each step's useful content is rendered. Inspecting
specific failed and completion steps exposed these open follow-ups:

| Tracker | Finding and verification needed |
| --- | --- |
| `agy-reader#qd5q` — Show daemon errors on failed file-view steps in Markdown | FIFO steps 21/33 and media step 41 carry top-level `error.shortError`, preserved in raw JSON. Markdown instead says `File Viewed` with an empty fence. Render the error and test a failure without a later assistant summary. |
| `agy-reader#kfzg` — Render structured-output FINISH step payloads in Markdown | The schema scenario's `finish.output.arguments_json` and `finish.outputString` contain the final object. Markdown prints `No additional details stored for this step type`. Render that result and test it without duplicated planner text. |
| `agy-reader#sjfc` — Make format-audit tests handle configured daemon credentials | The opt-in live Go test still returns HTTP 401 despite a matching environment token: it constructs a client without assigning CSRF. The authenticated RPC, sync, and strict sweep succeed. |

The [typed step model](../../internal/daemon/types.go) does not model either
top-level step errors or `finish`; the [Markdown renderer](../../internal/render/markdown.go)
therefore cannot display them at those steps. Error phrases and completion
markers survive elsewhere in the main transcript because the assistant
repeated them. Those whole-transcript marker checks passed, but are weaker
than checking the failed step's own content. These are reader coverage gaps,
not evidence that agy introduced incompatible storage.

The helper's standard tests passed with daemon overrides unset. Its preflight
and recording used the same verified fresh corpus. Separately, the opt-in
live test reproduced the known 401 failure and remains failing; it is not
counted as a passing gate. No test-harness fix is claimed by this audit.

The rebuilt local reader's final CLI `doctor` returned:

```text
csrf:        token found (sent as x-codeium-csrf-token)
agy version: 1.2.14  (compatible)
sidecars:    245/245 fresh
watch:       running
exit 0 — healthy
```

The test agy session (`tmux attach -t agy-audit-1214`) and reader watcher remain
running. The IDE was closed and was not audited. Private evidence is retained
under `~/.local/state/agy-reader/format-audits/2026-09-29-agy-1.2.14-225655/`.
It contains prompts, fixtures, process hashes, auth probes, raw RPC responses,
the sweep manifest/corpus, canonical diffs, rendered transcripts, audit logs,
and the known live-test failure. Tokens and live conversation identifiers
are excluded from the committed report and shape snapshot.
