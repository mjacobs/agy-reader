# agy 1.4.12 file URI paths

The reader now verifies and exports conversations whose file tools touched a
file named with spaces, punctuation or non-ASCII characters, for example
`Project Notes — Phase 1 & 2.md` (pull request #4, issue #3). Before, it
refused them because it accepted only letters, digits and `/ . _ - @ +` in
those paths. A native capture showed which characters agy copies into its file
URIs unchanged. Every character tried was copied unchanged except the
backslash, which the reader keeps refusing.

This does not re-record `COMPATIBILITY.md`, which still names agy 1.4.11, and
it does not install the reader.

## Native case

| Item | Value |
| --- | --- |
| agy | 1.4.12, SHA-256 `1edfb30477447825f0aaa05fd9a91d82f9adf26686f41cd4f5c70b90abe383e8` |
| Setup | Sandbox `HOME`, print mode, model `gemini-3.8-flash-high`, a `PreToolUse` hook allowing only `read_file`, `edit_file` and `write_file` inside the workspace |
| Files | Dummy files with two lines of filler text: `plain.md`, `Project Notes — Phase 1 & 2.md`, `Notes, draft.md`, `C# notes.md`, `50% done.md`, `what?.md`, `a%20b.md` (a literal `%20`), `back\slash.md`, `café decomposed.md` (the accent as a separate combining character), and a PNG named `Diagram #1 50%?.png` |
| Reads (`e1f0b49b`) | One `read_file` per file, the image, and a missing `missing #2 50%?.md` |
| Writes (`f74d8112`) | `edit_file` on seven files, `write_file` creating `created #3 50% new?.md`, `write_file` overwriting `Notes, draft.md`, and an `edit_file` whose target text was absent |
| Backslash (`38e01201`, `9f0daf94`) | The same two prompts with `back\slash.md` included |
| RPC | `LoadTrajectory`, default `GetCascadeTrajectory` and full unrehydrated `GetCascadeTrajectory` from an idle daemon started after the conversations were written |

## What the daemon presents

In the default RPC, `view_file.absolute_path_uri` and the `code_action` URIs
were `"file://" + path`, byte for byte, for every file above except the
backslash one. That covers spaces, `&`, `,`, `#`, `%`, `?`, a literal `%20`, an
em dash and a decomposed accent, across text reads, the image read, the failed
read, edits, the create, the overwrite and the failed edit. The full RPC keeps
each call as a generic step and has no file URI. The hook allowed every call;
none was denied.

A backslash is different. The stored argument was `back\slash.md`, but the
default RPC showed `file:///…/back/slash.md`. Both the read and the edit failed
with a does-not-exist error although the file existed, so agy treats the
backslash as a directory separator.

## Verdicts

`audit-sweep` against one fresh daemon, and `export-offline` on copies of the
stored sources:

| Conversation | Reader at `main` | Pull request as submitted | With backslashes refused |
| --- | --- | --- | --- |
| Reads (`e1f0b49b`) | Not verified: `unsupported 1.4.3 tool projection`; offline export fails | Verified | Verified; offline URIs match the daemon (10 of 10) |
| Writes (`f74d8112`) | Not verified, same reason | Verified | Verified; offline URIs match the daemon (19 of 19) |
| Reads with backslash (`38e01201`) | Not verified, same reason | `incomplete: daemon presentation differs`; offline export writes `back\slash.md` | Not verified: `unsupported failed read_file projection`; offline export refuses |
| Writes with backslash (`9f0daf94`) | Not verified, same reason | `incomplete: daemon presentation differs` | Not verified: `unsupported failed edit_file projection` |

The pull request as submitted accepted backslashes. It therefore blamed the
daemon for a presentation the reader had predicted wrongly, and exported a URI
the daemon never shows. Refusing the backslash keeps those conversations
unsupported, as they were before.

A fifth sandbox conversation, a confinement check, is refused by every build
because the hook denied one of its reads (`agy-reader#gs51`).

## Not tested

- agy 1.4.10. The contributor reported the same presentation there; no capture
  from that version was retained.
- Other punctuation such as `"`, `*`, `:`, `<`, `>` and `|`. The rule accepts
  it because no character tried was escaped.
- Markdown rendering. Markdown transcripts link these files with the URI as
  written, so a name with a space or `#` produces a link that Markdown readers
  do not follow. JSON export is unaffected. Tracked as `agy-reader#hwb3`.
- macOS and Windows hosts, symbolic links, and skill files with such names.

## Validation

On the merged tree: `go test -count=1 ./...` passes, `go vet ./...` passes and
`golangci-lint run` reports `0 issues`. With the backslash check removed, the
five new backslash checks fail, including the native-shaped failed read, which
reads as `incomplete` instead of `unknown`. Private captures (stored sources,
all three RPC responses, sweep manifests, offline exports, hook logs and the
scripts) are in
`~/.local/state/agy-reader/audits/2026-10-31-pr4-path-capture/`, outside the
repository; committed fixtures are synthetic.
