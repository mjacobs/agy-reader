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
4. **Sidecar Shape Fingerprint**: Computes a deterministic digest over the *daemon-owned structure* of the trajectory JSON (field names + value types, values excluded) — the content agentsview renders. The reader-owned top-level `agyReader` namespace is removed entirely before walking. See [Sidecar shape fingerprint](#sidecar-shape-fingerprint) below. This catches a blind spot the schema fingerprint cannot: the sidecar stores the bare trajectory payload from `GetCascadeTrajectory`, which the daemon can reshape *without* touching the `.db` schema, so the schema check reports `UNCHANGED` while agentsview rendering silently degrades.
5. **Integration Diagnostics**: Runs the `agy-reader` unit test suite. The `agentsview` consumer suite is **opt-in** — it runs only when `AGENTSVIEW_DIR` points at a local agentsview checkout (e.g. `AGENTSVIEW_DIR=~/dev/projects/agentsview skills/agy-format-audit/scripts/audit_format.sh`); by default it's skipped, since agentsview is a separate codebase with its own lifecycle.
6. **Changelog Delta**: Runs `agy changelog` and prints every release note **newer than the version recorded in `COMPATIBILITY.md`, up to the live `agy`** (the changelog is newest-first; the delta is every block above the recorded version's `X.Y.Z:` header). This is read-only and never gates the audit — it's the human-/agent-readable companion to the deterministic fingerprint: the fingerprint tells you *whether* the on-disk schema moved, the changelog tells you *what the release claims to have changed* and *why*. It prints even when the audit has findings (a breaking release is exactly when you want the notes). When there are no new versions it says so; when it can't bound the delta (recorded version missing from the changelog window) it falls back to the latest block only.
7. **Compatibility Record**: Computes a deterministic schema fingerprint (a `sha256` over every `CREATE` statement plus `user_version`) plus the sidecar shape fingerprint (item 4), and emits a copy-paste record — agy version, date, agy-reader commit (with a clean/dirty flag), table/index summary, and both fingerprints.

### Recording compatibility (`COMPATIBILITY.md`)

The script is **read-only by default**: a passing run just *prints* the record. You decide whether the run qualifies as a citable verification — note the clean/dirty warning, since a dirty tree means the recorded commit doesn't capture your working changes.

The record is the full contents of [`COMPATIBILITY.md`](../../COMPATIBILITY.md) at the repo root — a single-purpose file, so updating it is a wholesale replace with no README to search-and-edit. Two ways to update it once you've confirmed the run is good:

```bash
# Read-only: audit + print the record block (paste it in yourself if you like)
skills/agy-format-audit/scripts/audit_format.sh

# Record: same audit, then overwrite COMPATIBILITY.md (the only write it performs)
skills/agy-format-audit/scripts/audit_format.sh --record
```

On every successful audit run that reaches compatibility-record generation, the script compares **both** live fingerprints to their recorded values and reports, on two independent status lines:

- **Schema** — `UNCHANGED` (format identical — the fast path after an `agy` upgrade that didn't touch the schema), `DRIFT` (fingerprint differs — inspect before recording), or `NEW` (no baseline yet).
- **Sidecar shape** — the same states, plus **`not recorded`** when the recorded `COMPATIBILITY.md` predates this check, and **`not comparable`** when the recorded corpus scope is absent or differs from the live run. Neither state is drift or failure: inspect the live structure, then let the next qualified `--record` capture the fingerprint and its corpus provenance. A comparable `DRIFT` with an `UNCHANGED` schema is exactly the blind spot this check exists to catch, so triage it against the changelog delta.

**Corpus scope decides comparability**, so the script never claims provenance it cannot verify. Pointing `AGY_SIDECAR_CORPUS` at a directory says nothing about which `agy` version wrote those files, and a union over stale sidecars can mask a path the new version *removed* — the digest reads `UNCHANGED` when coverage is simply absent. The three scope labels:

| Scope | When | Comparable across versions |
| --- | --- | --- |
| `latest-paired-sidecar` | no `AGY_SIDECAR_CORPUS` — the sidecar paired with the newest DB | yes |
| `partial-<version>-scoped` | `AGY_SIDECAR_CORPUS` set, sweep **not** asserted | no — the version is baked in, so the next bump reports `not comparable` rather than a falsely reassuring `UNCHANGED` |
| `explicit-version-scoped` | `AGY_SIDECAR_CORPUS` set **and** `--corpus-swept` passed | yes |

`--corpus-swept` is an operator assertion that *every* sidecar in the corpus was re-serialized by the version under audit (see the re-sync sweep in the corpus caveat below). Only assert it when you actually did the sweep — it is the one thing standing between a partial corpus and a baseline that future audits trust.

Like schema drift, sidecar-shape drift **reports but does not gate** `--record` — you decide whether the change is benign. The README's `## Compatibility` section points readers at the recorded file.

> The schema fingerprint is stable across sessions and machines for a given `agy` version, so any `.db` from that version reproduces it — that, not a one-off session UUID, is what makes a verification reproducible. The sidecar-shape fingerprint is stable for a given `agy` version *given a broad enough version-scoped sidecar corpus* (see the caveat in [Sidecar shape fingerprint](#sidecar-shape-fingerprint)).

### Sidecar shape fingerprint

The schema fingerprint covers the `.db` only. The `.trajectory.json` sidecar stores the daemon's bare trajectory payload (the client unwraps the outer `{"trajectory": ...}` RPC envelope) without imposing a reader-defined schema on daemon-owned values (`internal/cache/sidecar.go`; volatile fields are `json.RawMessage` in `internal/daemon/types.go`), plus an optional reader-owned `agyReader` metadata block. Google can rename, restructure, or drop a step-payload field **without touching the DB schema**. The schema fingerprint stays `UNCHANGED`, agentsview's tolerant parsers don't complain, and rendering silently degrades. The sidecar-shape fingerprint closes that gap while subtracting `agyReader` so reader metadata can never look like daemon drift.

**Algorithm** — the authoritative, precisely-documented implementation is the package doc of [`internal/shapefp`](../../internal/shapefp/shapefp.go); it is deliberately simple so **agentsview can reimplement it byte-identically**. In brief, it is a `sha256` over a sorted, order-insensitive listing of the JSON's key-structure:

1. Unwrap an RPC envelope when present, remove the root `agyReader` member entirely, then walk each sidecar; every remaining node emits one `path → type` entry (`type` ∈ `object|array|string|number|boolean|null`; **values excluded**).
2. `path` is dot-joined segments from the root. An array element is the segment `[]` (so all elements share one path and their types **union**). An object member contributes its key — unless the key is **volatile** (a `file://` URI, a filesystem path, a UUID, a pure integer, or a long hex id), which normalizes to `*` (so maps keyed by *data*, e.g. `workspaceUrisToRelativePaths`, don't explode the fingerprint).
3. A fixed set of **opaque subtrees** — config/passthrough blobs agy-reader stores as `json.RawMessage` and never structurally models, with open-ended data-dependent keys, and which agentsview does not schema-render (`generatorMetadata`, `executorMetadatas`, `steps.[].generic`, `steps.[].userInput.userConfig`/`activeUserState`, `steps.[].metadata.internalMetadata`/`sourceTrajectoryStepInfo`, `metadata.mendelExperimentIds`) — are recorded as a leaf but **not recursed**. (Empirically these are ~78% of raw paths and are pure noise: config that flips on every settings/version change, not render-contract shape.)
4. Entries **union across every supplied sidecar** and every occurrence of a path, so a field present in one session and absent in another does not flip the digest. The audit defaults to the sidecar paired with the newest database so historical sessions from older agy versions cannot mask removed fields. For better optional-feature coverage, set `AGY_SIDECAR_CORPUS` to a curated file or directory known to have been generated by the version under audit.

Compute or inspect it directly:

```bash
go run . shape-fingerprint <dir-or-sidecar>...          # print sha256:<hex>
go run . shape-fingerprint --paths <dir-or-sidecar>...  # print the canonical path/type lines (diff these on a DRIFT)

# Audit a curated corpus instead of the newest sidecar only. Without
# --corpus-swept this records as partial-<version>-scoped (not comparable).
AGY_SIDECAR_CORPUS=/path/to/curated-sidecars skills/agy-format-audit/scripts/audit_format.sh

# Assert every sidecar in the corpus was re-serialized by the version under
# audit, earning the comparable explicit-version-scoped label.
AGY_SIDECAR_CORPUS=/path/to/curated-sidecars \
  skills/agy-format-audit/scripts/audit_format.sh --record --corpus-swept
```

**False-DRIFT tradeoffs (honest limits).** The union-across-sidecars defense is only as complete as the corpus you compute over:

- **Optional-feature paths.** A path that no session in the corpus exercises can't appear in the union. Two machines whose sidecars exercise different optional features (e.g. one used directory-attachment context, another didn't) can produce **different fingerprints at the same `agy` version**. Empirically (89 CLI sidecars, mid-2026) the residual after pruning is tiny — a couple of optional attachment/edit fields — but it is nonzero. **Mitigation:** record the baseline over a broad, representative corpus, and treat a `DRIFT` as a *signal to inspect* (via `--paths`), not an automatic failure — same posture as schema drift.
- **New volatile-key styles.** The `*` normalization only collapses keys matching the volatile patterns. A future map keyed by a *new* style of id (not a UUID/path/int/hex) would show through as distinct keys until the pattern set is extended.
- **Opaque-subtree churn is invisible by design.** Anything under a pruned subtree (§3) is not fingerprinted; a genuinely render-relevant field appearing *there* would be missed. That set is intentionally limited to non-rendered config; revisit it if agy starts rendering from those blobs.

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

Then check the **structural** shape against the recorded baseline (this is what the automated audit does — the DB schema check will not catch a reshaped RPC response):
```bash
# from the agy-reader repo; use the newest session sidecar or a curated corpus
# known to have been generated by the version under audit:
go run . shape-fingerprint "$AGY_ROOT/conversations/<session-id>.trajectory.json"
go run . shape-fingerprint --paths "$AGY_ROOT/conversations/<session-id>.trajectory.json"
```

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
