# agy 1.4.12 compatibility

agy-reader now records agy 1.4.12 as verified. The final sweep fetched,
rendered and natively verified **499 of 499 conversations (21,365 steps)** in
the local CLI store, with no source-only exceptions. The SQLite schema
fingerprint and the sidecar shape fingerprint are both unchanged from 1.4.11.
No reader change was needed beyond the subagent-compaction qualification that
landed just before this audit.

## What 1.4.12 changed

| Source | Change | Effect on the reader |
| --- | --- | --- |
| Changelog | Plugin `auth.json` API key providers can be marked `"optional": true` | None. Plugin configuration, not conversation data |
| Changelog | Markdown tables in long streaming replies reach the terminal scrollback complete | None. Terminal display only |
| Protobuf snapshot | New text field `PluginThemeSeeds.code_foreground` (field 5) | None. The reader does not store or render plugin themes |

The protobuf comparison is
[`agy-1.4.12-proto-delta.json`](agy-1.4.12-proto-delta.json), between the
committed 1.4.11 and 1.4.12 snapshots. For this audit the 1.4.12 snapshot was
re-verified against the installed binary (SHA-256
`1edfb30477447825f0aaa05fd9a91d82f9adf26686f41cd4f5c70b90abe383e8`): all 390
embedded records matched their byte ranges, and the comparison was rerun and
matched the committed delta.

## Coverage

The record's required set is the three conversations agy 1.4.12 wrote in this
store. agy's logs show 1.4.12 first ran at 14:52 that day, and these are the
only databases with content written after that time:

| Conversation | Steps | Result |
| --- | ---: | --- |
| `c1b6f876` (newest) | 182 | Verified |
| `21908929` | 19 | Verified |
| `1d00702a` | 0 | Verified, but empty, so it covers no step types |

The other 496 conversations were written by earlier releases and verify under
1.4.12's daemon. Subagent compaction, which stores a field added in 1.4.10,
was qualified separately in a throwaway store; see [agy-1.4.12-subagent-compaction.md](agy-1.4.12-subagent-compaction.md).
Its conversations are not in this store, so they are not in the required set.

## Method

- An idle interactive agy 1.4.12, started for the audit with its own token and
  left at the folder-trust prompt, served the sweep. It had loaded none of the
  conversations beforehand, which avoids the stale-copy refusals described in
  the [1.4.11 report](agy-1.4.11.md).
- No scenario conversations were created. The store held 499 databases before
  and after the sweep, so agy deleted none during this audit. A private copy of
  the store was taken first.
- The reader unit suite passed inside the audit helper. The agentsview consumer
  suite was not run.

## Not covered

- The optional plugin API key. It changes how a plugin's commands run, not what
  agy stores, and no plugin conversation was captured.
- `RunCommand.trusted_cli_control_outputs` and the two 1.4.10 configuration
  fields remain unseen in real data. A conversation that stores one will be
  refused until it is qualified (`agy-reader#7v0k`).
- The installed reader was not rebuilt or restarted, so its `doctor` still
  reports 1.4.11 until it is reinstalled.
