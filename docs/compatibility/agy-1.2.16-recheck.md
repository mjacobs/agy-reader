# agy 1.2.16 fresh-session qualification

On 2026-10-03 Pacific time (2026-10-04 UTC), a fresh headless background-task
scenario passed on reader commit `bbfa1cece8b2f4c2c5e668f9d05f8c3ace98bf70`.
The reader verified and synced all eight steps after a fresh daemon loaded the
completed conversation. The qualified audit updates `COMPATIBILITY.md` to
1.2.16. This is a scenario-qualified baseline, not proof that every historical
conversation can sync.

The [sanitized evidence](agy-1.2.16-recheck-evidence.json) records counts,
timestamps, provenance, and token usage. The
[shape snapshot](agy-1.2.16-recheck-sidecar-shape.tsv) captures the full fresh
corpus union. This supplements the [initial audit](agy-1.2.16.md) and its reader
qualification follow-up; it does not replace their original observations.

## Scenario and observed behavior

The existing `scripts/agy-pair.py` launched a new agy/reader pair with a generated
`ANTIGRAVITY_CSRF_TOKEN`, pinned authenticated endpoint, and the checkout's
`make build` reader. agy used `gemini-3.8-flash-high`, high effort, and permission
auto-approval in a disposable workspace. The watcher used an initially empty
private root; agy itself wrote to the normal CLI store. No installed watcher
was restarted.

The prompt requested one background task:

```sh
python3 -c 'import time,sys; print("AUDIT_RECHECK_STARTED",flush=True); time.sleep(20); sys.exit(137)'
```

It instructed agy to end its first response with `WAITING_RECHECK`, without
polling, then run `printf "AUDIT_RECHECK_RECOVERED\n"` after the completion
notification and finish with `RECHECK_DONE`.

The first response completed at `04:41:53.454452943Z`. The task completed at
`04:42:10.957630048Z`, about 17.50 seconds later. The stream explicitly reported
waiting for one background task. The notification carried exit 137; the recovery
command exited 0, and the final response completed at `04:42:16.467526292Z`.
Headless agy exited 0. This exercises late headless completion and nonzero
background completion, not an actual signal kill, OOM, or exit 134.

The final SQLite backup passes `PRAGMA integrity_check` and has eight steps.
Fresh full/default RPC and native persisted-source verification agree, as shown
by the sweep's `readerSync: verified`. A fresh watcher wrote eight steps with
profile `sqlite-v1-format0-agy-a759ce7c7a23`. Ordinary `--sync` and subsequent
`audit-qualify` against that sidecar both exit 0. Rendered command sections show
exit 137, recovery exit 0, and recovery output; the final response includes
`RECHECK_DONE`.

## The stale-daemon control failed safely

An inspection pair was opened while the headless run was still finishing. It
warned that the conversation was already open. That daemon subsequently returned
seven steps while the completed SQLite backup held eight. The watcher refused
to write a sidecar: `persisted and RPC trajectory content differ`.

The first strict sweep fetched and rendered all 277 conversations but marked the
scenario `incomplete`. The shell audit, explicitly given the scenario ID in
`AGY_QUALIFY_SESSIONS`, exited 1 and produced no compatibility record, even though
unit tests passed. Restarting only the inspection pair after completion restored
an eight-step response and verified sync. The temporal overlap is a plausible
cause of the stale cache; this run does not isolate daemon cache internals.

This validates an important skill distinction: successful rendering and matching
schema fingerprints cannot substitute for persisted-source/full/default-RPC
qualification. Capture the scenario after completion and start the inspection
daemon afterward. No skill implementation change was needed.

## Corpus, fingerprints, and record

Both serving executable checks matched the installed 1.2.16 binary SHA-256:
`a759ce7c7a235d9b6c281a25ead97cbbf2e92314a3ffd224e2f9144f3fae7a86`.
The first inspection endpoint returned 401 without its token and 200 with it;
the restarted pair also passed the helper's authenticated startup check.

The fresh sweep completed with **277 fetched/rendered, zero failures, 25,110
steps**. Native qualification verified **85/262 SQLite conversations**;
**177 historical SQLite conversations were not verified**, and **15 legacy
conversations were unverifiable**. The scenario was also the newest conversation
selected by the shell audit. Historical failures were explicitly not gating.
Raw corpora stay outside the repository; sweep loads may update daemon store
state, but the sweep itself writes no live sidecars.

The schema remains user_version 1, seven tables, and two step indexes:

- Schema: `sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
- Shape: `sha256:3b3cde304c93c8d8787a6e5ed677e682480158838cb72300294182db80b06c82`.

The comparable fresh corpus adds 35 path/type entries and removes none versus
1.2.14. Against the previous 1.2.16 observation, only the MCP configuration object
and its `nonBlockingTurnLoad` boolean are added. Other additions since 1.2.14
cover image-parent artifact configuration, checkpoint/task metadata, and optional
edit/skill fields already observed by the earlier audit. No removed render path
was observed. These union differences include coverage and configuration changes;
they do not independently prove breaking daemon drift.

The read-only fresh audit and qualified `--record --corpus-swept` audit pass,
including the reader unit suite. The record reports a dirty tree because the
pre-existing untracked `docs/handoffs/` directory was preserved. Tracked reader
code was unchanged from the recorded commit. No downstream agentsview tests or
live re-ingestion were run, and the installed reader baseline was not rebuilt.

## Changelog verdict and limits

The 1.2.15–1.2.16 delta declares no SQLite, encryption, or explicit RPC schema
change. Image generation through a built-in subagent is a compatibility red flag
for lineage, tool payloads, and artifacts. Background completion changes affect
notifications and lifecycle assumptions; this scenario exercises those paths.
Image generation was not rerun here; its prior evidence remains in the initial
audit. TUI/config/selection changes are likely format-benign. Queued messages,
parent customization discovery, permission UX, and long-history responsiveness
remain untested in this run.

The CLI result reported 24,426 input tokens, 1,571 output tokens, and 48,836 cache
read tokens; thinking tokens are a subset, not an additional charge. Monetary
cost was not exposed. No audit worker subagents were launched.

Both temporary pairs were stopped with `agy-pair.py stop` after collection.
Private raw evidence remains at `/tmp/agy-audit-20261003-recheck/`; this temporary
location is not a durable archive. The committed sanitized evidence and shape
snapshot are the durable record. No token or raw conversation is committed.
