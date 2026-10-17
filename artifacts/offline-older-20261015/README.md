# Older SQLite compatibility sample

Conversation labels such as `conversation-001` are consistent aliases across
these reports, not session IDs. Use actual IDs from your own store when
rerunning commands.

The saved agy 1.3.6 schema decoded all 12 selected older CLI SQLite trajectories (1,474 steps). Every known step field matches the existing sidecar after the same bounded presentation transformation used in the recent-session experiment. All non-step known fields also match. Four sessions contain fields that the current schema cannot interpret.

## Sample and results

Selected 12 evenly spaced entries from 152 eligible older CLI DB/sidecar pairs: creation before October 2, at least 10 steps, sidecar at most 40 MB. This is a chronological sample, not random. Dates below are America/Los_Angeles, derived from sidecar creation metadata. The exact producer binaries are unknown. DB modification dates remain near those historical dates, but are not definitive producer-version evidence.

| Created | Conversation alias | Steps matched / decoded | Undefined stored fields |
| --- | --- | ---: | --- |
| 2026-06-02 | conversation-005 | 172 / 172 | CascadePlannerConfig field_50 (3) |
| 2026-06-22 | conversation-021 | 214 / 214 | ChatMessagePrompt field_23 (6); CascadePlannerConfig field_50 (11) |
| 2026-07-06 | conversation-079 | 21 / 21 | CascadePlannerConfig field_50 (2) |
| 2026-07-16 | conversation-035 | 88 / 88 | ChatMessagePrompt field_23 (3) |
| 2026-08-14 | conversation-100 | 52 / 52 | None detected |
| 2026-08-20 | conversation-131 | 39 / 39 | None detected |
| 2026-08-28 | conversation-199 | 186 / 186 | None detected |
| 2026-09-05 | conversation-129 | 369 / 369 | None detected |
| 2026-09-05 | conversation-122 | 97 / 97 | None detected |
| 2026-09-16 | conversation-073 | 153 / 153 | None detected |
| 2026-09-29 | conversation-176 | 73 / 73 | None detected |
| 2026-10-01 | conversation-102 | 10 / 10 | None detected |

## What backward compatibility means here

Eight samples had no detected unknown fields, including recursively decoded Any payloads. The four June/July samples contain 16 occurrences of CascadePlannerConfig field 50 and 9 occurrences of ChatMessagePrompt field 23, both length-delimited. These match the identities of the historical source-only exceptions documented in docs/completeness.md; this experiment identifies their numbers and wire types, but does not independently qualify every exception payload shape.

Known JSON fields matching is weaker than complete interpretation. The current descriptor reserves those numbers and assigns no meaning to their values. Protobuf parsing retains the unknown data: a serialize/parse roundtrip preserved message equality including unknown fields in all 12 samples. JSON conversion does not carry those unknown fields. A historical-schema archive therefore still matters, even when a current-schema transcript looks correct.

The presentation rule selects a stored GENERIC result's embedded native Step and carries over outer taskDetails/status. It makes the sample's known step fields match; it is not a general production compatibility rule or lossless replacement for the original wrapper.

## Preservation and limitations

No daemon calls; no agy-reader process. Only private copies were opened by SQLite. Original DB, WAL and sidecar hashes were unchanged before/after each run. Copies were deleted at exit. All selected WAL files were empty. No transcript text was written to the report.

The experiment reuses the prior Python decoder and saved descriptor set unchanged. results.json contains exact counts, unknown-field identities and preservation checks. Private source hashes and exact source timestamps are omitted; JSON creation dates retain UTC day precision. probe-base.py preserves the original comparison implementation; run.py selects one case at a time and records failures without private content. selection.json describes the selection.

This covers existing older SQLite data and existing sidecars, not encrypted legacy .pb files, all historical agy releases, migrated-vs-original writer provenance, or active nonempty WAL behavior. The public report uses aliases; it cannot select files from a real store.

To rerun, supply a private selection JSON with a `samples` array. Each entry
needs an actual session UUID in `id`, plus `created_at` and `db_mtime_utc`
timestamps. Save the raw results outside the checkout: they contain private
identifiers, hashes and timestamps. Run from the repository root with Python
protobuf 7.36.2:

```sh
UV_CACHE_DIR=/tmp/agy-uv-cache uv run --with grpcio-tools==1.84.0 python artifacts/offline-older-20261015/run.py \
  --selection /tmp/private-selection.json --output /tmp/private-older-results.json
```
