# agy 1.2.11 compatibility audit and parent-metadata repair

The 2026-09-25 audit fetched and rendered all 242 selected CLI conversations
through the agy 1.2.11 daemon with no failures. Both the SQLite schema and the
trajectory structure match the 1.2.10 baseline. The historical parent stamps
that caused cycle warnings were repaired separately; all 80 native parent
pointers now agree with the reader metadata.

The audited reader commit was `fc975e2`, with a clean working tree before
recording. See the generated [compatibility record](../../COMPATIBILITY.md). The
canonical field/type listing is unchanged from the
[1.2.10 snapshot](agy-1.2.10-sidecar-shape.tsv).

## Fresh corpus and format checks

The live daemon executable matched the installed agy binary byte-for-byte, and
that executable reported version 1.2.11. Every selected conversation was fetched
using `LoadTrajectory` and `GetCascadeTrajectory`, including the internal
cascade-ID fallback where needed. Fresh responses were saved in a new private
corpus and rendered through the reader. No cached sidecar fallback was used.

| Check                                                        |          Result |
| ------------------------------------------------------------ | --------------: |
| Selected / freshly fetched / rendered conversations          | 242 / 242 / 242 |
| Fetch or render failures                                     |               0 |
| SQLite / legacy protobuf sessions                            |        227 / 15 |
| SQLite `PRAGMA quick_check` results                          |  227 / 227 `ok` |
| SQLite schema version / table count, in every SQLite session |           1 / 7 |
| Populated / empty trajectories                               |        220 / 22 |
| Steps / distinct step types                                  |     22,163 / 21 |
| Canonical field/type entries                                 |           1,736 |
| Added / removed entries versus the 1.2.10 snapshot           |           0 / 0 |
| agy-reader unit suite                                        |            PASS |

Schema fingerprint:
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
Sidecar shape fingerprint:
`sha256:3acca61c897bdec898292c1646090faf1913781f0a4d11ca389bc043ba4ec92a`.

The 1.2.11 changelog covers reasoning-effort selection, plugin and custom-agent
discovery, and terminal rendering and copying. It contains no stated changes to
session storage, encryption, trajectory payloads, or the daemon protocol. These
notes raise no format-specific red flags. This audit did not exercise each new
user-interface behavior or run a new subagent scenario.

## Historical parent repair

The reader service and its path trigger were stopped before backing up all 242
CLI sidecars. Repair used a reader built from `fc975e2`, which includes the
metadata-write freshness fix; the installed reader was still at `5cf6aeb`. The
full conversations directory supplied the relationship evidence.

The first `backfill-parent-links --repair` reported:

```text
scanned=242 stamped=1 cleared=5 unchanged=79 unresolved=162 diagnostics=0
```

Six sidecars changed. Four existing parent cycles were eliminated. Comparison
with the backup verified that all daemon-owned payloads, other reader fields,
and all 242 sidecar modification timestamps were preserved. All 80 native
`metadata.parentConversationId` values in the fresh daemon corpus match the
repaired reader stamps. The 162 unresolved entries have no selected parent; they
are not 162 repair failures.

A second repair reported zero stamps, zero clears, and zero diagnostics; every
sidecar remained byte-identical. The service and path trigger were restored. An
ordinary sync then wrote a fresh sidecar without parent-cycle warnings. The
watcher reported 242 up-to-date and zero failed.

## Recording and limits

After inspecting the unchanged fingerprints and changelog, this command
completed successfully and replaced `COMPATIBILITY.md`:

```sh
AGY_SIDECAR_CORPUS=/path/to/fresh-corpus \
  skills/agy-format-audit/scripts/audit_format.sh --record --corpus-swept
```

Both the preflight and recording runs passed the reader unit suite. The explicit
corpus assertion covers all 242 fresh daemon responses. The IDE was not running,
and the separate agentsview consumer suite and downstream re-ingestion were not
exercised.

The installed reader was not replaced. It still embeds the 1.2.10 compatibility
record until rebuilt; the updated source build's CLI `doctor` check passed with
the 1.2.11 record. Private local evidence retains the sidecar backup, repair
verification, fresh corpus, sweep manifest, SQLite results, and audit logs. Raw
conversations and live identifiers are excluded from this report.
