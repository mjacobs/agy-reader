# Legacy conversation verification with agy 1.4.4

Legacy encrypted conversations now receive the same native completeness checks
as SQLite conversations before the reader writes or trusts a sidecar. A recent
sidecar timestamp alone no longer certifies a legacy conversation.

The reader authenticates the encrypted file, decodes its complete trajectory,
and compares it with fresh full and default responses from agy. Unknown keys,
unknown fields, damaged ciphertext, oversized files and a migration to SQLite
fail closed. Evidence binds to the exact encrypted bytes and a separate legacy
profile; the SQLite profile is unchanged.

## Qualification on 2026-10-21

Implementation commit `6eaaf99` was checked against an owned agy 1.4.4 process
whose executable SHA-256 was
`337dcde668441a2828968644d7982829c2d2a5e0c4144f346e1f3fc6abb3cdc7`.
No model prompt was submitted.

- All 15 legacy conversations passed native/full/default comparison and cached
  attestation validation: 2,610 steps. Their source hashes stayed unchanged.
- A fresh sweep covered all 396 CLI conversations: 381 SQLite and 15 legacy.
  There were 306 fully projected successes and 90 successes with declared
  source-only fields, including all 15 legacy conversations. There were zero
  unverified conversations. Source-only fields remain covered by the native
  digest and existing allowlist; this change adds no allowances.
- The format audit passed with 16 gating conversations: the newest conversation
  plus all 15 legacy conversations. Schema and sidecar shape were unchanged.
  `COMPATIBILITY.md` records this clean implementation commit.
- The protobuf snapshot check passed: 378 byte ranges, 34 closure descriptors
  and 34 roundtrips. No new schema snapshot was needed.
- `make vet lint test build` passed. The maintenance Python suite passed on the
  host: 134 tests run, 17 skipped. The sandbox attempt could not complete its
  process-dependent checks; it is not counted as a passing run.
- AgentsView integration tests were not run because no consumer checkout was
  selected.

Raw trajectories, source copies and process provenance are private audit
artifacts, not repository fixtures. Synthetic encrypted fixtures cover
failure cases without disclosing conversation contents. Deployment and live
service verification have separate receipts; this audit alone does not prove
that a running reader was replaced.
