# agy 1.5.8 compatibility

The 1.5.8 update verifies all 14 supplied exercise conversations and the
407-step audit conversation against an isolated daemon. Ordinary sync and
Markdown output pass for all 15. A broad copied-store sweep fetches and renders
500/500 conversations and natively verifies 499/500: 489 ordinary verifications
and ten with existing declared source-only legacy model-choice fields. Offline
export matches fresh RPC steps, generator metadata and executor metadata for
all 499 verified conversations.

The remaining historical agy 1.5.7 source contains a failed `post_message`
after its swarm member was killed. It remains refused as unsupported and is
tracked separately in agy-reader#07da. It does not qualify for a new attestation.
The audit record gates the 499 verified sources, including every new scenario;
it does not claim that fetching and rendering the remaining source makes it
syncable.

## Corrections established by the audit

| Persisted behavior | Reader support |
| --- | --- |
| Redundant `steps.render_info` and `steps.permissions` columns are NULL | The complete step payload supplies both records. Any populated redundant column must still agree. |
| `gen_metadata.size` is zero | Zero and the actual byte length are accepted; other markers are refused. Metadata is still decoded and compared in full. |
| `DefineSubagent` uses fields 4 and 5 and PascalCase arguments | The two declarations are added without removing historical fields. The default RPC's derived definition and result name are reconstructed from saved arguments. Agent configuration remains compared. |
| MCP results use a dedicated `McpTool` payload | Completed string results become native MCP steps with matching call identity, server, tool and argument JSON. |
| An MCP ask rule records an approval | Both permission fields must describe the call's exact resource and one effective-grant allowing interaction. The modern projection preserves them. The earlier report's embedded-native-Step mapping was not the actual captured form. |
| An MCP deny rule prevents execution | The captured resultless error remains generic, including its resource-specific denial text. |
| Failed reads and background cancellation appear in the audit conversation | Missing-file, inverted-range, malformed hook-result and completed hook-free 10000 ms cancellation forms have separate bounded rules. |

The original storage refusals hid the modern MCP payload, derived subagent
fields and further read/cancellation differences. Before correction, the same
500-source corpus verified 484 sources (474 ordinary, ten source-only) and
refused 16. Nine sanitized full/default step pairs now cover these forms,
with argument, status, approval and payload rejection tests. NULL duplicate
columns, empty populated columns, malformed bytes and changed payloads have
separate checks. The profile suffix is `tools158`; there is no new source-only
exemption and no bypass for CLI or offline rendering of unsupported sources.

## Schema and release evidence

The retained [snapshot](../../artifacts/proto-snapshots/agy-1.5.8-9db59b637110/snapshot.json)
was reverified against the installed binary, SHA-256
`9db59b6371107b939befa9c507c0a24989fda974e75d81d8529f3689ef7322e5`:
402 embedded byte ranges, 35 closure files and 35 round-trip files. The isolated
daemon's startup log identifies 1.5.8. Comparing with 1.5.7 gives 28 wire,
zero JSON-facing and five descriptor-only changes. Removals include historical
subagent fields and a marketplace flag; a wholesale schema replacement would
lose them. Only the two subagent fields and MCP message needed by the new
captures enter the production bundle. Its exact authenticated delta and
historical preservation are tested.

All 27 changelog entries were read. MCP permission rules, model additions,
ACP usage updates, gateway web-fetch behavior, BOM handling and resumed
structured output are compatibility red flags to inspect even though the
SQLite schema is unchanged. The supplied captures cover approved/denied MCP,
subagent definition/invocation, Hydrogen Low, ACP sessions, BOM settings and
reads, hooks, and resumed structured Finish output. One hook experiment used
an invalid hook-result JSON field; its recorded decode failure is preserved,
not treated as proof of a successful hook. Windows-specific behavior, Carbon,
enterprise gateway/admin policy, remote transport and authentication changes
were not independently exercised. TUI rendering, shell escape output, rewind
and ephemeral side questions do not by themselves establish a storage change.

SQLite remains `user_version=1`, seven tables and two explicit indices, with
fingerprint
`sha256:1ca98426f561fe73223c8620a238405030fdb3014444d970e1300a6009f72f43`.
Compared with the retained 506-source 1.5.7 corpus, the 500-source 1.5.8 corpus
adds seven shape paths and lacks 25: experiment/default-ask configuration is
added; optional checkpoint/media, permission-description and error-display
coverage is absent. This is an inspected coverage difference, not proof that
1.5.8 removed those fields. Every sidecar in the new corpus was freshly
serialized by 1.5.8; no cached sidecars supplied sweep evidence.

Private database copies, full/default RPC captures, manifests, export checks,
shape paths and check logs remain under ignored `tmp/compat158/`. Collection
and sync write only the disposable store. This update does not install or
restart the active reader or agy-pair. The generated `COMPATIBILITY.md` records
the passing audit's fingerprints and native qualification.

## Validation

`make check` passes Go formatting, vet, lint, tests and the build. `go test
-race ./...` passes. The maintenance suite also passes under the snapshot's
protobuf/protoc environment: 183 tests reported, 166 run and 17 skipped. The
system Python lacks protobuf, so its separate default maintenance run skips
109 tests; that run alone is not the schema-validation evidence. Exact overlay
reproduction, historical preservation, tampered-input refusal and snapshot
verification are covered by the protobuf-enabled run.
