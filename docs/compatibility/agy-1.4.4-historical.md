# Historical conversation compatibility with agy 1.4.4

The eight SQLite conversations refused by the initial 1.4.4 reader now pass
native verification. Together they contain **2,990 steps**. Each database was
captured alongside full unhydrated and default RPC responses from the installed
agy 1.4.4 executable, with unchanged database and nonempty WAL content during
capture. No model prompt was submitted.

The `history2` profile adds five bounded behaviors observed in those captures:

- Completed payload-free `manage_task send_input` records remain generic when
  the input and task ID agree with result metadata and the stored allow hook.
- A failed `list_resources` wrapper presents its embedded native step when
  both errors and the server name agree.
- Calls rejected for a missing `toolSummary` remain generic INVALID steps.
- A successful command whose bridge fails while formatting its result presents
  the native command payload with ERROR status. The exact source-derived shape
  is checked, including the bridge error class and matching command arguments.
- Failed legacy reads derive the skill marker from an exact enabled skill
  registration in the current or a bounded earlier executor. Execution identity,
  source step identity and ordering must agree. Duplicate, conflicting, disabled
  or future-only registrations cannot establish a marker. Unregistered paths
  stay unmarked regardless of their filename. When no enabled exact skill
  registration exists, disabled and non-skill registrations, repeated entries
  and duplicate executor identities preserve that unmarked behavior.

The modern `skill1` rule still uses only the matching current executor. The
retained 73-step modern conversation passes after the legacy changes. The
embedded descriptor, declared source-only allowances and permission restrictions
are unchanged. Older profile attestations and cached refusals, including
those under the initial `history1` profile, require fresh verification. Review
identified that the first registry rule also refused ordinary failed reads of
non-skill customization files. The `history2` correction preserves those reads;
positive skill markers retain the strict identity, registration and ordering
checks. Regression tests cover both sides of that boundary.

`TestNativeEvidence` passes for all eight captures, including attestation
rechecks. Synthetic tests reject unsupported fields, permissions, inconsistent
arguments and metadata, ambiguous registrations, and altered full/default
responses. [Sanitized native evidence](agy-1.4.4-historical-evidence.json)
records capture hashes and counts without private conversation contents.

The fresh full sweep from clean commit
`6a8c63d2e2ca7415e26037a8f0324c8848c83e6d` fetched and rendered **396/396**
conversations without failure. All **381/381 SQLite** conversations verified:
306 normally and 75 with the existing declared source-only fields. There are
zero unsupported SQLite conversations in this corpus. The 15 legacy protobuf
conversations remain unverifiable for completeness.

The expanded required gate passed **13/13**, covering these eight and the five
previous 1.4.4 qualification conversations. Ordinary sync also wrote verified
sidecars for disposable copies of all eight repaired sources. Both read-only
and recording runs of the format-audit skill passed; the generated
[compatibility record](../../COMPATIBILITY.md) names the clean implementation
commit. Schema and sidecar shape fingerprints are unchanged from the initial
1.4.4 qualification.

Formatting, vet, lint, Go package tests and build passed. Python maintenance
checks passed with **134 tests run, 17 skipped**. The protobuf-snapshot skill
reverified all **378 descriptor byte ranges** against the installed executable;
the **34-file closure** still loads and roundtrips. No snapshot or descriptor
change was needed for these historical projection gaps.

These results establish compatibility with the observed stored corpus. They
do not exercise new model-generated steps, the closed IDE daemon or downstream
AgentsView ingestion. Installation and live-service receipts are maintained
separately from this audit.
