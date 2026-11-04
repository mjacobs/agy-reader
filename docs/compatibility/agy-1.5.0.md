# agy 1.5.0 compatibility

The agy 1.5.0 audit fetched and rendered **499/499 conversations (19,618
steps)**. Native verification passed for **490/499**, including all three
required 1.5.0 scenarios. Nine older conversations remain unverified; this
record does not claim universal coverage of the retained store.

The reader now understands three persisted configuration fields and the
plain skill name/description metadata observed in the new release. The
historical schema stays intact. Unknown fields and mismatched full/default
RPC responses still prevent verification.

## Release changes and qualification

The changelog has compatibility risks: ACP image/file prompts and resume,
remote conversations, background-task timeout behavior, and tool-capable
`/btw` questions. These can affect stored history or transcript presentation.
Local-project management may change workspace metadata. Plugin sign-in,
plugin template variables, and terminal rendering are primarily configuration
or UI changes. No SQLite migration or encryption change is announced.

The supplied running pair used binary SHA-256
`b9043860a06a3bfa70b68f06c3d4527abc16207767f14ae9d1e06e3026081862`, matching
the installed `agy --version` result, 1.5.0. The new
[protobuf snapshot](../../artifacts/proto-snapshots/agy-1.5.0-b9043860a06a/snapshot.json)
verified all **394 embedded records** against the binary, and all **34 closure
files** compiled back to matching descriptors. Its
[delta from 1.4.12](agy-1.5.0-proto-delta.json) contains **32 additive wire
changes, zero JSON-facing changes, and three deprecation annotations**.

The runtime overlay copies only the observed configuration additions:

- Browser `enable_browser_subagent_webview` (field 30).
- Subagent `model_options` (field 10), with its supporting model messages.
- Cascade `model_tiers` (field 15), with `ModelTierConfig`.

The browser field predates 1.5.0's descriptor delta but appeared in the
1.5.0 native capture. All seven added declarations come from the authenticated
1.5.0 closure. Reproduction and exact-preservation tests are documented in
[completeness.md](../completeness.md#schema-provenance-and-maintenance).

Default RPC also adds `viewFile.skillMetadata` for modern skill reads. The
new whole-trajectory presentation alternative derives name and description
from the saved numbered read result, limited to the observed four-line
frontmatter form with plain scalar values. It never consults today's file or
uses incoming RPC values as evidence. More complex frontmatter remains
outside this rule. The old presentation remains supported.

## Native evidence

| Required conversation | Steps | Result |
| --- | ---: | --- |
| `46e1fd20` | 207 | Verified; main feature-exercise conversation |
| `383583b8` | 1 | Verified; minimal coverage |
| `0c2deaaa` | 2 | Verified; minimal coverage |

The main conversation contains 22 file reads, 74 commands, three task calls,
and two edits. Its two skill reads exercise the new metadata presentation.
An isolated SQLite backup plus independently captured full/default RPCs also
passed `TestNativeEvidence` for all 207 steps and cached-attestation checking.
The short conversations do not establish broad ACP coverage. No successful
remote-agent, image-prompt, or interactive `/btw` test is claimed.

The supplied pair was interrupted with SIGINT after its ongoing writes
prevented a stable capture. A separate 1.5.0 process, left idle at the folder
trust prompt, served the final sweep. Its executable hash matched the snapshot.
No live sidecars were written by the sweeps. The selected store changed from
500 to 499 databases while the supplied pair was exercising functionality;
the final sweep covers the 499 it selected, not the initial inventory.

## Fingerprints and limits

SQLite remains `user_version=1`, seven tables, with schema fingerprint
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
The fresh, fully swept sidecar corpus has shape fingerprint
`sha256:75f6dd0645b3029bc39abd6618b569e2170a157b19a8763af46c981752b1f21a`,
which differs from the recorded 1.4.12 baseline.

The original 1.4.12 path inventory was unavailable, so an exact baseline
path delta could not be recovered. As a diagnostic only, the fresh union adds **zero paths** and drops **329
paths** relative to the current cached sidecar union, mostly old nested
agent configuration.
The cached digest is `sha256:d4ddd0a34874ee519b24c9d5c02f826286c1c15881258011f068db7cdd4ad871`,
not the recorded baseline. Cache age and corpus turnover prevent attributing
those removals solely to 1.5.0. The new baseline deliberately uses only the
fresh daemon responses, never cached sidecars to hide missing coverage.

Nine retained conversations do not verify: two unsupported canceled background
commands, one unsupported failed file read, and six presentation differences.
Exact IDs and results are in [the evidence summary](agy-1.5.0-evidence.json).
Follow-up `agy-reader#t5zw` tracks their qualification; existing background
command evidence under `agy-reader#z0wh` must be consulted before expanding
those rules. None of the nine is a required new-release scenario.

## Checks and retained evidence

- `go test ./...` passes.
- Python maintenance suite: 171 tests, 17 skipped, no failures.
- Snapshot verification rechecked 394 byte ranges and 34 roundtrip files.
- The format audit passes with all three scenario IDs required.
- The agentsview consumer suite and IDE surface were not tested.

Private source/RPC evidence, the sweep corpus and manifest, shape inventories,
and command logs are retained in the ignored `tmp/agy-1.5.0-audit/` directory.
Only schema definitions and a content-free summary are committed. Recording
updates the checkout baseline; it does not install or restart a deployed reader.
