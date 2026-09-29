# agy 1.2.13 format and feature audit

The 2026-09-29 audit fetched and rendered all **243 selected CLI conversations**
through a verified agy 1.2.13 daemon, with zero failures. The SQLite schema and
trajectory shape match the recorded 1.2.11 baseline. A fresh two-turn scenario
also exercised medium-verbosity history replay, artifact-viewer mode hints,
and narrow-terminal help. Two credential-related test-harness gaps were found
and recorded separately; neither reproduced in the authenticated reader.

The reader code under audit was `9a228af`, with a clean working tree before
recording. [COMPATIBILITY.md](../../COMPATIBILITY.md) now records 1.2.13. The
installed local reader was rebuilt with that record and its watcher restarted.
Source changes remain local and uncommitted.

## Daemon and authentication evidence

The pre-existing agy session was running a deleted, replaced executable.
Its executable hash differed from the installed binary. A new tmux session
was started from the installed 1.2.13 executable, with a fresh explicit
`--csrf_token` as its first argument. Both the initial and final serving
processes matched the installed executable byte-for-byte:

```text
sha256:574b0234656c44564f3ff3b013225c86f14ae79b67d0f3b45f6aaf1b1625440e
```

The HTTP listener was resolved from the serving process's own open log.
A read-only RPC returned **401 without the token** and **200 with the matching
`x-codeium-csrf-token` header**. The reader discovered the launch token while
agy was idle, including after reader and agy restarts. The sweep explicitly
pinned that daemon URL and matching `ANTIGRAVITY_CSRF_TOKEN`; its manifest
endpoint agrees with the verified serving process. The older session was left
running and was not used for the audit.

## Changelog-focused scenario

The delta comprises 1.2.12 and 1.2.13. It describes retry/quota handling,
rendering efficiency, history replay, graphics probes in multiplexers,
artifact-viewer hints, narrow-terminal wrapping, and Vim behavior. **There are
no declared changes to storage, encryption, RPC messages, or step payload
types.** These notes raise no explicit format red flags. Retry behavior can
affect which errors appear in a history, so an unchanged fingerprint alone
does not verify the new rate-limit policy.

The fixture used CSV parsing and exact-row deduplication: five input rows,
four unique rows, raw total 24, and unique total 21. agy wrote a standard-library
implementation and three tests, ran twelve separate calculation commands,
deliberately exited one command with code 7, recovered, and created a Markdown
artifact with a Mermaid flowchart and the display equation `24 - 3 = 21`.
No subagents or network tools were used for this scenario.

| Behavior | Observed result | Local evidence file |
| --- | --- | --- |
| Fixture correctness | 3/3 tests passed, including UTF-8 labels `café` and `中`; an independent rerun also passed | `fixture-tests-independent.txt` |
| Tool-heavy history | First turn completed with 62 steps and all twelve calculation commands | `tui-turn-one.txt`, `corpus-summary.json` |
| Medium-verbosity resume, 1.2.12 | A fresh tmux pane showed the completed assistant summary and idle prompt after 0.419 seconds, sampled about every 100 ms | `replay-samples.json`, `tui-replayed-medium.txt` |
| History preservation after resume | Second turn recalled totals 24/21 and exit code 7, re-read the files and artifact, and passed 3/3 tests again | `tui-resume-complete.txt` |
| Artifact hints, 1.2.13 | Viewer rendered the ASCII flowchart; `m` switched between `diagram source` and `diagram ASCII` hints | `artifact-ascii.txt`, `artifact-source.txt` |
| Narrow help, 1.2.13 | At 40 columns, the workspace, log path, conversation ID, and keyboard hints wrapped; the compact tab bar remained visible | `help-40-columns.txt` |
| Multiplexer title, 1.2.12 | The ordinary pane title remained intact after restart; no `Ga=q...` title corruption was observed | `replay-samples.json` |

The final scenario has **73 steps, two user turns, and 20 command steps**.
The intentional failure preserves exit code 7 and `EXPECTED_FAILURE_7` in a
completed command step; the recovery output is present. agy also tried
`git status` in this non-Git fixture, producing exit code 128, and continued.
Neither nonzero command became a trajectory error step.

Watcher samples observed sidecars at 9, 31, 51, and 62 steps during the first
turn. Final database, live sidecar, and fresh sweep response each contain 73
steps. A direct authenticated `GetCascadeTrajectory` response equals the live
sidecar after removing the reader-owned `agyReader` namespace. Reader Markdown
rendering also retained the failure, recovery, and resumed-turn completion
markers. This verifies payload preservation as well as successful parsing.

The replay timing is one local observation on a 62-step fixture, not a
performance comparison against an older release. Low-verbosity replay,
keystrokes during replay, full-screen scrolling, Vim editing, image-mode math,
quota exhaustion, and server-directed retry delays were not tested. Original
high verbosity was restored after the scenario. Model cost was not exposed by
this audit; no worker fleet was launched.

## Corpus and format checks

The strict `audit-sweep` manifest reports `complete: true`. Every syncable
conversation was fetched through `LoadTrajectory` and `GetCascadeTrajectory`
and rendered, including the internal cascade-ID fallback where necessary.
The sweep used no cached sidecar fallback and wrote its corpus outside the
repository. Unsupported implicit traces were excluded.

| Check | Result |
| --- | ---: |
| Selected / freshly fetched / rendered conversations | 243 / 243 / 243 |
| Fetch or render failures | 0 |
| SQLite / legacy protobuf sessions | 228 / 15 |
| SQLite `PRAGMA quick_check` results | 228 / 228 `ok` |
| SQLite schema version / table count in every database | 1 / 7 |
| Populated / empty trajectories | 221 / 22 |
| Steps / distinct step types | 22,236 / 21 |
| Canonical field/type entries | 1,736 |
| Added / removed entries versus the prior snapshot | 0 / 0 |
| Standard `go test ./...` | PASS |
| Audit preflight and recording, without daemon overrides | PASS / PASS |
| Separate agentsview consumer suite / live ingestion | Not run |

Schema fingerprint:
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
Sidecar shape fingerprint:
`sha256:3acca61c897bdec898292c1646090faf1913781f0a4d11ca389bc043ba4ec92a`.
Both are **UNCHANGED**, with `explicit-version-scoped` corpus provenance.
The canonical listing matches the
[1.2.10 snapshot](agy-1.2.10-sidecar-shape.tsv), also used by the 1.2.11 report.
Opaque configuration fields remain outside the shape fingerprint's coverage.

## Test-harness findings and recording

**Open follow-up: `agy-reader#sjfc`, Make format-audit tests handle configured
daemon credentials.** No test or production code was changed during this audit.

1. The opt-in [live daemon test](../../internal/daemon/client_test.go) constructs
   `daemon.NewClient(base)` without setting its `CSRFToken`. Supplying the
   correct URL, token, and session ID therefore still yields HTTP 401.
   `NewClient` intentionally leaves credential configuration to its caller.
   The authenticated strict sweep and raw RPC comparison succeed.
2. `TestReachableDaemonConnectionCarriesTheScannedToken` in
   [doctor_test.go](../../doctor_test.go) sets a token but does not clear an
   inherited `ANTIGRAVITY_DAEMON_URL`. With a live URL exported, this discovery
   test fails because the override wins. The audit helper inherits this
   environment and suppresses test output, reporting only `FAIL`.

The initial credential-pinned helper run correctly withheld a compatibility
record after the unit failure. Re-running over the same verified fresh corpus
with the daemon overrides unset passed. The recording run also passed:

```sh
env -u ANTIGRAVITY_DAEMON_URL -u ANTIGRAVITY_CSRF_TOKEN \
  -u AGY_READER_LIVE -u AGY_READER_TEST_UUID \
  AGY_SIDECAR_CORPUS=/path/to/verified-sweep/corpus \
  skills/agy-format-audit/scripts/audit_format.sh --record --corpus-swept
```

Endpoint pinning remains appropriate for the RPC sweep. The helper's later
schema, corpus, and unit checks do not need those live credentials.

The rebuilt installed reader's final CLI `doctor` returned:

```text
csrf:        token found (sent as x-codeium-csrf-token)
agy version: 1.2.13  (compatible)
sidecars:    243/243 fresh
watch:       running
exit 0 — healthy
```

The CLI agy process and systemd reader watcher remain running. The IDE was
closed and was not audited. No agentsview checkout was available at the usual
local path, so no consumer tests or downstream re-ingestion are claimed.

Private local evidence is retained under
`~/.local/state/agy-reader/format-audits/2026-09-29-agy-1.2.13-154020/`.
It includes the prompts, fixture, process provenance, authentication probe,
watcher samples, sweep manifest/corpus, SQLite checks, shape diff, passing and
failing test logs, roundtrip comparison, recording log, and final service
checks. Raw conversations, tokens, and live session identifiers are excluded
from this report.
