---
name: agy-format-audit
description: Guide for auditing Antigravity-CLI session storage formats (SQLite .db, legacy .pb) and decrypted JSON schema (trajectory.json) for compatibility with agy-reader and agentsview when new agy versions are released.
---

# Antigravity CLI Session Format Audit

Every new release of the Antigravity CLI (`agy`) may introduce changes to the local trace database format or JSON-RPC protocol. 

This skill provides a deterministic automated auditing workflow alongside a manual verification fallback to validate compatibility with `agy-reader` and `agentsview`.

> **Source & install.** This skill's tracked source of truth lives at
> `skills/agy-format-audit/`. The agent skill dirs (`.claude/skills/`,
> `.agents/skills/`) are gitignored install targets — run `make install-skills`
> from the repo root to (re)create the symlinks that point them at this source.

---

## ⚡ Automated Audit (Recommended)

To run a fast, deterministic schema check and regression test suite, execute the bundled audit helper script:

```bash
skills/agy-format-audit/scripts/audit_format.sh
```

### What the script checks:
1. **Session Storage Detection**: Identifies whether the active session uses SQLite (`.db`) or legacy Protocol Buffers (`.pb`).
2. **Schema Verification**: Confirms SQLite database matches `user_version = 1` and contains the expected 7 tables.
3. **Trajectory Check**: Parses the decrypted sidecar (`.trajectory.json`) and verifies step counts.
4. **Integration Diagnostics**: Runs the `agy-reader` unit test suite. The `agentsview` consumer suite is **opt-in** — it runs only when `AGENTSVIEW_DIR` points at a local agentsview checkout (e.g. `AGENTSVIEW_DIR=~/dev/projects/agentsview skills/agy-format-audit/scripts/audit_format.sh`); by default it's skipped, since agentsview is a separate codebase with its own lifecycle.
5. **Changelog Delta**: Runs `agy changelog` and prints every release note **newer than the version recorded in `COMPATIBILITY.md`, up to the live `agy`** (the changelog is newest-first; the delta is every block above the recorded version's `X.Y.Z:` header). This is read-only and never gates the audit — it's the human-/agent-readable companion to the deterministic fingerprint: the fingerprint tells you *whether* the on-disk schema moved, the changelog tells you *what the release claims to have changed* and *why*. It prints even when the audit has findings (a breaking release is exactly when you want the notes). When there are no new versions it says so; when it can't bound the delta (recorded version missing from the changelog window) it falls back to the latest block only.
6. **Compatibility Record**: Computes a deterministic schema fingerprint (a `sha256` over every `CREATE` statement plus `user_version`) and emits a copy-paste record — agy version, date, agy-reader commit (with a clean/dirty flag), table/index summary, and the fingerprint.

### Recording compatibility (`COMPATIBILITY.md`)

The script is **read-only by default**: a passing run just *prints* the record. You decide whether the run qualifies as a citable verification — note the clean/dirty warning, since a dirty tree means the recorded commit doesn't capture your working changes.

The record is the full contents of [`COMPATIBILITY.md`](../../COMPATIBILITY.md) at the repo root — a single-purpose file, so updating it is a wholesale replace with no README to search-and-edit. Two ways to update it once you've confirmed the run is good:

```bash
# Read-only: audit + print the record block (paste it in yourself if you like)
skills/agy-format-audit/scripts/audit_format.sh

# Record: same audit, then overwrite COMPATIBILITY.md (the only write it performs)
skills/agy-format-audit/scripts/audit_format.sh --record
```

On every run the script compares the live fingerprint to the recorded one and reports **`UNCHANGED`** (format identical — the fast path after an `agy` upgrade that didn't touch the schema), **`DRIFT`** (fingerprint differs — inspect before recording), or **`NEW`** (no baseline yet). The README's `## Compatibility` section points readers at the recorded file.

> The fingerprint is stable across sessions and machines for a given `agy` version, so any `.db` from that version reproduces it — that, not a one-off session UUID, is what makes a verification reproducible.

### Reviewing the changelog delta for compatibility red flags

When you (the agent) run this skill, **don't just relay the changelog delta — read it for compatibility risk and call out anything relevant.** The fingerprint already tells you if the schema *did* change; the changelog tells you whether a release *intended* to change something the reader depends on, which is your early warning even on an `UNCHANGED` run (a note can describe a sidecar/payload change the schema fingerprint won't catch).

Flag a release note as a **red flag** when it touches anything `agy-reader` / `agentsview` parse or rely on:

- **Storage format / DB**: SQLite schema, `user_version` bumps, added/removed/renamed tables, columns, or indices; migrations; a switch between SQLite and legacy Protocol Buffers (`.pb`); changes to the conversations directory layout or file naming.
- **Trajectory sidecar**: changes to `.trajectory.json` shape, generation, or location; new/renamed/removed step types or step payload fields; anything that affects how steps deserialize into `internal/daemon/types.go`.
- **Encryption / encoding**: changes to how the sidecar is decrypted or encoded, key handling, or compression.
- **Protocol**: JSON-RPC / daemon protocol or message-shape changes the reader speaks.

Treat as **likely benign** (mention briefly, don't alarm): pure TUI/keybinding/rendering tweaks, auth/browser sign-in flows, permission-prompt UX, and unrelated CLI ergonomics — none of which touch the on-disk format the reader consumes.

Report the verdict explicitly, e.g. *"changelog delta is TUI/permissions-only — no format-affecting notes"* or *"⚠️ 1.0.x note mentions a new step payload type — verify it deserializes before recording."* If a note is a genuine red flag, prefer inspecting a fresh session DB / sidecar and the parsing code before treating the run as a citable verification, even when the fingerprint says `UNCHANGED`.

---

## 🔍 Manual Audit Fallback (High Freedom)

If the automated script is unavailable or custom validation is required:

### 1. SQLite Database Schema Audit
Verify if the database structure matches the baseline. Run:
```bash
# Locate active CLI root
export AGY_ROOT="${ANTIGRAVITY_CLI_ROOT:-$HOME/.gemini/antigravity-cli}"

# Inspect user_version and schema
sqlite3 "$AGY_ROOT/conversations/<session-id>.db" "PRAGMA user_version;"
sqlite3 "$AGY_ROOT/conversations/<session-id>.db" ".schema"
```

**Baseline Tables:**
Ensure the following 7 tables are present:
* `trajectory_meta`
* `steps` (indices: `idx_steps_status`, `idx_steps_step_type`)
* `gen_metadata`
* `executor_metadata`
* `parent_references`
* `trajectory_metadata_blob`
* `battle_mode_infos`

### 2. Sidecar JSON Validation
Verify that the generated JSON sidecar maps properly to `internal/daemon/types.go`:
```bash
cat "$AGY_ROOT/conversations/<session-id>.trajectory.json"
```
Ensure newly added step payloads parse without breaking client deserialization.

### 3. Verification Tests
Execute package unit tests:
```bash
# From agy-reader:
go test ./...

# From agentsview:
cd ~/dev/projects/agentsview && go test ./...
```

### 4. Changelog Review
Read what the new `agy` release(s) claim to have changed and scan for the
compatibility red flags listed above (storage/DB, trajectory sidecar,
encryption, protocol). `agy changelog` is newest-first; stop at the version
recorded in `COMPATIBILITY.md`:
```bash
agy changelog        # newest-first; everything above the recorded X.Y.Z: is the delta
```

