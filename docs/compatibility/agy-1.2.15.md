# agy 1.2.15 format and feature audit

The initial 2026-10-02 audit found that **agy 1.2.15 can complete the new WebP/resize
scenario, but the reader cannot sync it**. Its persisted content falls outside
the reader's completeness profile. The reader correctly refuses to certify or
write a sidecar. `COMPATIBILITY.md` remains at **1.2.14**; this report is a
failed qualification, not a new passing baseline. No reader code was changed
during that audit. The subsequent [1.2.16 reader qualification](agy-1.2.16.md#reader-qualification-follow-up)
incorporates the 1.2.15 fix and verifies this frozen 12-step case again.

The broad sweep fetched and rendered **269/269 conversations**, and the
explicit-corpus shell audit exited **0**, despite that normal-sync failure.
This exposes a gap in the audit workflow: its success does not establish that
the current reader can persist the conversations being tested.

The [sanitized evidence record](agy-1.2.15-evidence.json) contains exact counts,
failure output, fingerprints, and payload diagnostics. The
[canonical shape snapshot](agy-1.2.15-sidecar-shape.tsv) is an observation,
not an accepted compatibility baseline.

## Process and scenario provenance

The tested reader's tracked source was
`87dda0492d8d969efb8cd41d0aa5f07b4453e4ec`. The build checkout had only a
pre-existing untracked handoff, and the tested binary reports
`vcs.modified=true`; both shell audits ran in a clean worktree of that commit.
The existing `scripts/agy-pair.py start` helper
launched a new named tmux pair, `audit-1215`, with a generated launch token.
It passed the matching `ANTIGRAVITY_CSRF_TOKEN` to both children and pinned
the reader's endpoint to the fresh agy listener. The process owning that
listener and the installed `agy --version` executable matched this SHA-256:

```text
5f9c16b286895f8f7fdecd423883ca256a85077b8acf9a6bc1111761d34df164
```

Direct `GetCascadeTrajectory` returned **401** without the token and **200**
with it. Full-verbosity RPC also returned **200**. No installed service was
restarted. The pair watches the normal CLI store and can update historical
sidecars; its working directory isolates the fixture, not conversation storage.
The pre-existing watcher was left running.

The changelog delta is 1.2.15 only. It declares no SQLite, encryption, or RPC
format change. Large-WebP handling deserves payload inspection; permission
denials and stream reconnection deserve behavioral checks. The remaining
platform, terminal, quota-display, customization, and OAuth notes do not
explicitly declare storage changes. Those notes do not rule out wire drift.

The fixture was an **8000 × 6000 WebP**, 216,310 bytes, generated with four
colored quadrants: red `ALPHA 3`, green `BETA 5`, blue `GAMMA 8`, and yellow
`DELTA 13`. The prompt supplied the absolute file path but did not reveal the
labels, colors, or numbers. It requested native `view_file`, a timed local
command, and an image-derived sum. Gemini 3.8 Flash Low ran with permission
auto-approval, so this does **not** test the permission-denial fix.

The timed command was:

```sh
python3 -c "import time; [(print('RESIZE_TICK_'+str(i), flush=True), time.sleep(1)) for i in range(1,21)]; print('RESIZE_DONE_1215')"
```

During its background execution, 18 successful tmux resize requests cycled
through 92×28, 150×44, and 72×22 at approximately 180 ms intervals; the final
window was restored to 120×35. The TUI displayed completion and all four
correct image descriptions, reported sum **29**, and subsequently displayed
`FOLLOWUP_1215 29` from a second command. This is a short-session observation,
not a reproduction of the long-conversation freeze or proof of a reconnect.

The stored image artifact decodes as **JPEG 4096 × 3072**, 323,054 bytes.
Scaling therefore occurred. Both RPC MIME fields still say `image/webp` and
the media filename ends in `.webp`: an observed upstream encoding/metadata
mismatch, without an observed model failure. Other formats and platforms were
not tested.

## Why ordinary sync fails

The idle scenario has **12 steps** in its SQLite backup, full RPC, ordinary
RPC, and sweep response. Ordinary RPC and the sweep payload are structurally
equal. There is no live sidecar to compare: both watcher and one-shot sync
refuse this source. The opt-in native test independently fails on the backup:

```text
=== RUN   TestNativeEvidence
    native_test.go:19: unknown persisted protobuf fields
--- FAIL: TestNativeEvidence (0.02s)
FAIL
```

Run it with disposable evidence files, not live DB paths:

```sh
AGY_READER_SCRATCH_DB="$evidence/scenario.db" \
AGY_READER_SCRATCH_RAW="$evidence/raw.json" \
AGY_READER_SCRATCH_PRESENTATION="$evidence/presentation.json" \
  go test ./internal/completeness -run '^TestNativeEvidence$' -v -count=1
```

Both JSON inputs retain the RPC `trajectory` envelope. Three profile limits
are relevant; only the first is reached by this final native-test execution:

| Limit | Evidence and consequence |
| --- | --- |
| Unrecognized persisted metadata | `executor_metadata[0]` and `[2]` contain `AgentConfigResolution` field **3**, wire type **2**, with a zero-length message. Full RPC names it `contribVariantMetadata: {}`. The pinned 1.2.14 descriptor only defines fields 1 and 2, so snapshot decoding rejects it. |
| Image result projection | Raw step **2** has `generic.result.media`, which the result allowlist in `internal/completeness/projection.go` excludes. The watcher reported `unsupported generic result` before the final metadata failure appeared. This media field already exists in the pinned descriptor. |
| Background command projection | Raw step **4** has outer `taskDetails`, which the generic-wrapper allowlist excludes. Code inspection shows that resolving the preceding limits alone would still leave this wrapper unsupported. This was not an independently passing/failing modified-fixture test. |

The previous 245-conversation 1.2.14 corpus had no `contribVariantMetadata`,
and its pinned descriptor lacks field 3. This establishes a difference from
the qualified baseline; it does not establish which release or feature first
introduced it. The profile should remain strict until the new representation
and both projections are qualified.

The image step also exposes a rendering gap. Default RPC retains
`viewFile.mediaData`, but the typed `ViewFile` model drops it. Rendering this
payload directly succeeds while producing a file link, `Lines 0-799`, and an
empty text fence. The later assistant description makes the transcript look
useful but does not repair the image step. The previous corpus already had
21 image-media steps, so this is existing reader coverage debt.

## Corpus results and audit limits

| Check | Result |
| --- | ---: |
| Strict sweep selected / fetched / rendered | 269 / 269 / 269 |
| Fetch or render failures | 0 |
| SQLite / legacy protobuf files | 254 / 15 |
| SQLite `quick_check` / version 1 with seven tables | 254/254 / 254/254 |
| SQLite step counts matching ordinary sweep RPC | 254/254 |
| Populated / empty trajectories | 245 / 24 |
| Total steps / distinct step types | 24,319 / 22 |
| Standard `go test ./...` through the helper | PASS |
| Native scenario test / ordinary sync | FAIL / FAIL |
| Default helper, watcher grace disabled | Exit 1: latest sidecar missing |
| Explicit-corpus helper with `--corpus-swept` | Exit 0, offers a record |
| Separate agentsview suite / live ingestion | Not run / not verified |

The sweep manifest is `complete: true`: every selected session was freshly
fetched through the verified daemon, with no cached-sidecar fallback. The
SQLite count and integrity checks were separate read-only queries. Equal
counts do not prove equal persisted content, and neither these queries nor
the sweep provide an atomic store snapshot.

Schema fingerprint, **UNCHANGED**:
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
Observed sidecar fingerprint, **DRIFT**:
`sha256:4ffe53d3dd8d6a9349d40e8602d324f6348f9cf235e6895e45c2b00cdf6f387d`.
The fresh corpus has `explicit-version-scoped` provenance and **1,756**
canonical entries: **15 added, zero removed** versus the 1.2.14 snapshot.
Additions cover checkpoint data, background-command flags, file append, and
skill logos. They are newly observed coverage, not all proven release changes.

Two audit boundaries explain the misleading green result:

1. `audit_sweep_cmd.go` directly fetches ordinary RPC and renders it; it
   bypasses `fetchSession`'s persisted/full-RPC/default-RPC verification.
   The shell helper accepts this explicit corpus even when the latest live
   sidecar is missing. It displays sidecar counts without checking DB equality.
2. Shape hashing intentionally prunes `executorMetadatas` and generic payloads.
   The new field that blocks native decoding is invisible to that fingerprint.
   The current shape drift is not a detector for this failure.

At the recorded doctor check, the CLI root had **254 SQLite sessions: 58 with
verified sidecars, zero incomplete, and 196 unknown/missing**, plus 15 legacy
sidecars fresh by mtime.
Doctor exited **1**. These historical unknowns have multiple possible profile
limits; they are not all attributed to 1.2.15. Authentication and watcher
liveness therefore do not imply root health.

## Recorded follow-ups and retained evidence

| Tracker | Next verification |
| --- | --- |
| `agy-reader#eecj` — Qualify 1.2.15 metadata and media/background projections | Add bounded descriptor/projection support with rejection tests, then require native verification, normal sync, and a verified watcher sidecar for this scenario. |
| `agy-reader#8rst` — Make format audits detect unsupported normal sync | Separate broad fetch/render inspection from native qualification; prevent this explicit-corpus green result from being mistaken for a qualified reader baseline. |
| `agy-reader#gntb` — Render image-view media details | Verify the image step itself exposes meaningful media information instead of an empty text fence. |

Raw prompts, process provenance, SQLite backup, RPC envelopes, media diagnostic,
TUI captures, sweep corpus, and audit logs remain private under
`~/.local/state/agy-reader/format-audits/2026-10-02-agy-1.2.15-121351/`.
Tokens and conversation identifiers are excluded from committed evidence.
The new pair remains running; use `scripts/agy-pair.py attach --session
audit-1215` or `scripts/agy-pair.py stop --session audit-1215` from the checkout.

The agy scenario made six model responses. Reported counters total 53,986
input, 696 output, and 65,112 cache-read tokens; these are separate counters,
not a billing calculation. One research worker assisted the audit, and the
repository's required commit reviewers are additional work. No dollar-cost
telemetry is available for the scenario or worker fleet.
