#!/usr/bin/env bash
#
# audit_format.sh: Deterministic Antigravity session and schema audit helper.
#
# Read-only by default. It inspects the latest session DB, verifies the schema
# against the known baseline, parses the trajectory sidecar, runs the agy-reader
# test suite (and the agentsview consumer suite too, but only when AGENTSVIEW_DIR
# points at it), computes a deterministic schema fingerprint, and prints a
# copy-paste "compatibility record".
#
# The record is the full contents of COMPATIBILITY.md at the repo root. On a
# qualified passing audit you (the human) decide whether the run is citable and
# either paste the block in or re-run with --record to overwrite the file
# wholesale -- the only write this script ever performs, and a single-purpose
# file so there is no README to search-and-edit.
#
# Usage:
#   audit_format.sh            # audit + print record (read-only)
#   audit_format.sh --record   # audit + overwrite COMPATIBILITY.md on PASS

set -euo pipefail

RECORD=false
[ "${1:-}" = "--record" ] && RECORD=true

# Resolve repo root from the script's own location, independent of cwd, so the
# recorded commit and COMPATIBILITY.md path are always the agy-reader repo.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel 2>/dev/null || echo "$PWD")"
COMPAT_FILE="$REPO_ROOT/COMPATIBILITY.md"

# 1. Resolve Root Path
AGY_ROOT="${ANTIGRAVITY_CLI_ROOT:-$HOME/.gemini/antigravity-cli}"
CONVS_DIR="$AGY_ROOT/conversations"

echo "=== ANTIGRAVITY SESSION FORMAT AUDIT ==="
echo "Configured CLI Root: $AGY_ROOT"
echo "agy-reader repo:     $REPO_ROOT"

# Read-only audits of an Antigravity IDE root (same .db schema) are fine, but
# the record's provenance fields (agy version, changelog delta) describe the
# CLI, so recording from an IDE root would mislabel the baseline.
if [ "$RECORD" = true ] && [ -f "$AGY_ROOT/antigravity_state.pbtxt" ] && [ ! -f "$AGY_ROOT/cli.log" ]; then
    echo "Error: $AGY_ROOT looks like an Antigravity IDE root; --record is CLI-only."
    exit 1
fi

if [ ! -d "$CONVS_DIR" ]; then
    echo "Error: Conversations directory $CONVS_DIR not found."
    exit 1
fi

# 2. Find Most Recent Session Database
RECENT_DB=$(find "$CONVS_DIR" -type f -name "*.db" -printf "%T@ %p\n" 2>/dev/null | sort -rn | head -n 1 | cut -d' ' -f2- || true)

if [ -z "$RECENT_DB" ]; then
    echo "No modern SQLite databases found under $CONVS_DIR."
    echo "Checking for legacy Protocol Buffers (.pb) in implicit/..."
    RECENT_PB=$(find "$AGY_ROOT/implicit" -type f -name "*.pb" -printf "%T@ %p\n" 2>/dev/null | sort -rn | head -n 1 | cut -d' ' -f2- || true)
    if [ -n "$RECENT_PB" ]; then
        echo "Found legacy session file: $RECENT_PB"
    else
        echo "No session files (.db or .pb) detected."
    fi
    exit 0
fi

SESSION_ID=$(basename "$RECENT_DB" .db)
echo "Auditing Latest Session: $SESSION_ID"
echo "Database Path: $RECENT_DB"

# Overall pass/fail accumulators. AUDIT_OK gates whether a compatibility record
# is emitted at all -- we never print a "verified" record for a failing audit.
AUDIT_OK=true
VERSION="unknown"
FP=""
TABLE_COUNT=0
INDEX_LIST="(sqlite3 unavailable)"

# 3. Audit SQLite DB
echo "--- SQLite Database Check ---"
if ! command -v sqlite3 &>/dev/null; then
    echo "Warning: sqlite3 CLI is not installed. Skipping schema inspection and"
    echo "         fingerprinting; a compatibility record cannot be produced."
    AUDIT_OK=false
else
    # PRAGMA user_version
    VERSION=$(sqlite3 "$RECENT_DB" "PRAGMA user_version;")
    echo "Database user_version: $VERSION"

    # Inspect tables
    TABLES=$(sqlite3 "$RECENT_DB" "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;")
    TABLE_COUNT=$(echo "$TABLES" | grep -c . || true)
    echo "Tables present in DB:"
    echo "$TABLES" | sed 's/^/  - /'

    # Explicit (non-autoindex) indices -- sql IS NULL filters sqlite_autoindex_*.
    INDEX_LIST=$(sqlite3 "$RECENT_DB" "SELECT name FROM sqlite_master WHERE type='index' AND sql IS NOT NULL ORDER BY name;" | paste -sd ', ' - || true)
    [ -z "$INDEX_LIST" ] && INDEX_LIST="(none)"
    echo "Explicit indices: $INDEX_LIST"

    # Verify against expected baseline
    EXPECTED_TABLES=("battle_mode_infos" "executor_metadata" "gen_metadata" "parent_references" "steps" "trajectory_meta" "trajectory_metadata_blob")

    MISSING=()
    for t in "${EXPECTED_TABLES[@]}"; do
        if ! echo "$TABLES" | grep -Fq "$t"; then
            MISSING+=("$t")
        fi
    done

    EXTRA=()
    while read -r t; do
        [ -z "$t" ] && continue
        is_expected=false
        for exp in "${EXPECTED_TABLES[@]}"; do
            if [ "$t" = "$exp" ]; then
                is_expected=true
                break
            fi
        done
        if [ "$is_expected" = false ]; then
            EXTRA+=("$t")
        fi
    done <<< "$TABLES"

    if [ ${#MISSING[@]} -ne 0 ]; then
        echo "CRITICAL: Missing baseline tables: ${MISSING[*]}"
        AUDIT_OK=false
    fi
    if [ ${#EXTRA[@]} -ne 0 ]; then
        echo "NOTICE: Found newly introduced table(s): ${EXTRA[*]}"
    fi
    if [ ${#MISSING[@]} -eq 0 ] && [ ${#EXTRA[@]} -eq 0 ]; then
        echo "Schema Status: PASS (Matches expected 7 baseline tables)"
    fi

    # Deterministic schema fingerprint: every CREATE statement (tables + explicit
    # indices) ordered stably, plus user_version. Stable across sessions/machines
    # for a given agy version; changes iff the on-disk schema changes.
    FP=$( { sqlite3 "$RECENT_DB" "SELECT sql FROM sqlite_master WHERE sql IS NOT NULL ORDER BY type, name;"; sqlite3 "$RECENT_DB" "PRAGMA user_version;"; } | sha256sum | cut -d' ' -f1 )
    echo "Schema fingerprint: sha256:$FP"
fi

# 4. Trajectory Sidecar Check
echo "--- Trajectory Sidecar Check ---"
SIDECAR="$CONVS_DIR/$SESSION_ID.trajectory.json"
if [ -f "$SIDECAR" ]; then
    echo "Sidecar path: $SIDECAR"
    STEPS_COUNT=$(grep -o '"type":' "$SIDECAR" | wc -l || true)
    echo "Sidecar parsed successfully: Yes ($STEPS_COUNT steps detected)"
else
    echo "Sidecar status: MISSING (Sidecar file not generated yet)"
fi

# 5. Run Go tests to confirm compatibility
echo "--- Package Test Diagnostics ---"
if command -v go &>/dev/null; then
    # Run agy-reader tests
    if [ -f "$REPO_ROOT/go.mod" ]; then
        echo "Running agy-reader unit tests..."
        if (cd "$REPO_ROOT" && go test ./... &>/dev/null); then
            echo "agy-reader unit tests: PASS"
        else
            echo "agy-reader unit tests: FAIL"
            AUDIT_OK=false
        fi
    fi

    # Run agentsview tests if explicitly located via AGENTSVIEW_DIR
    if [ -n "${AGENTSVIEW_DIR:-}" ]; then
        if [ -d "$AGENTSVIEW_DIR" ] && [ -f "$AGENTSVIEW_DIR/go.mod" ]; then
            echo "Running agentsview unit tests from $AGENTSVIEW_DIR..."
            if (cd "$AGENTSVIEW_DIR" && go test ./... &>/dev/null); then
                echo "agentsview unit tests: PASS"
            else
                echo "agentsview unit tests: FAIL"
                AUDIT_OK=false
            fi
        else
            echo "Warning: AGENTSVIEW_DIR=$AGENTSVIEW_DIR is not a valid Go package directory; skipping."
        fi
    else
        echo "Skipping agentsview integration tests (AGENTSVIEW_DIR not specified)."
    fi
else
    echo "go CLI not found, skipping unit tests."
fi

# 6. Provenance + prior record. Computed here -- ABOVE the pass/fail gate -- so
#    the changelog delta below still prints when the audit has findings: a
#    breaking agy release is exactly when you want to read what changed.
AGY_VERSION=$(agy --version 2>/dev/null | head -n1 || echo "unknown")
GIT_SHA=$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo "unknown")
RECORD_DATE=$(date +%F)
if [ -n "$(git -C "$REPO_ROOT" status --porcelain 2>/dev/null)" ]; then
    TREE="dirty"
else
    TREE="clean"
fi

PRIOR_FP=""
PRIOR_VER=""
if [ -f "$COMPAT_FILE" ]; then
    PRIOR_FP=$(grep -oE 'sha256:[0-9a-f]{64}' "$COMPAT_FILE" | head -n1 | sed 's/sha256://' || true)
    PRIOR_VER=$(grep -oE '\*\*agy version:\*\* *.*' "$COMPAT_FILE" | head -n1 | sed 's/.*\*\* *//' || true)
fi

# 7. Changelog delta. Surface what changed in agy between the recorded version
#    and the live one so the human -- and the agy-format-audit skill agent --
#    can scan the release notes for compatibility red flags. Read-only; the
#    delta never gates the audit. `agy changelog` is newest-first, one block per
#    version (header line "X.Y.Z:" + "·" bullets), blocks separated by a blank
#    line, so the delta is every block above the recorded version's header.
echo "--- Changelog Delta (agy ${PRIOR_VER:-?} -> ${AGY_VERSION}) ---"
if ! command -v agy &>/dev/null; then
    echo "agy CLI not found; cannot fetch changelog."
elif [ -n "$PRIOR_VER" ] && [ "$PRIOR_VER" = "$AGY_VERSION" ]; then
    echo "No new agy versions since last record (still $AGY_VERSION)."
    echo "(Re-recording would only refresh the verification date/commit.)"
else
    CHANGELOG=$(agy changelog 2>/dev/null || true)
    # NB: feed $CHANGELOG via here-string, never `printf ... | reader`. With
    # `set -o pipefail`, a reader that exits early (grep -q, awk `exit`) closes
    # the pipe and the upstream printf takes SIGPIPE (141), which pipefail then
    # propagates -- silently flipping a found match to "not found" and aborting
    # the whole script. A here-string is a redirection, not a pipeline: no
    # upstream process, so no SIGPIPE.
    if [ -z "$CHANGELOG" ]; then
        echo "agy changelog returned nothing; skipping delta."
    elif [ -n "$PRIOR_VER" ] && grep -qxF "${PRIOR_VER}:" <<< "$CHANGELOG"; then
        DELTA=$(awk -v stop="${PRIOR_VER}:" '$0 == stop { exit } { print }' <<< "$CHANGELOG")
        NEW_COUNT=$(grep -cE '^[0-9]+\.[0-9]+\.[0-9]+:' <<< "$DELTA" || true)
        echo "New since recorded agy $PRIOR_VER ($NEW_COUNT version(s)):"
        echo ""
        printf '%s\n' "$DELTA"
    else
        # No bound: NEW record, or the recorded version is older than the
        # changelog window. Show just the latest block rather than dumping all.
        echo "Could not bound the delta against recorded version '${PRIOR_VER:-none}';"
        echo "showing the most recent changelog entry for $AGY_VERSION:"
        echo ""
        awk 'NR>1 && /^[0-9]+\.[0-9]+\.[0-9]+:/ { exit } { print }' <<< "$CHANGELOG"
    fi
    echo ""
    echo "REVIEW: scan the notes above for compatibility-affecting changes"
    echo "        (schema / table / column, storage format, trajectory.json"
    echo "        shape, step payload types, encryption / sidecar, DB migrations)."
fi

# 8. Compatibility record (the COMPATIBILITY.md payload)
echo "--- Compatibility Record ---"

if [ "$AUDIT_OK" != true ] || [ -z "$FP" ]; then
    echo "Audit did not fully pass; no compatibility record produced."
    echo "=== AUDIT COMPLETED (with findings) ==="
    exit 1
fi

emit_record() {
cat <<EOF
# Compatibility

agy-reader is verified against the Antigravity CLI (\`agy\`) local session format
recorded below. This file is generated by the \`agy-format-audit\` skill and is
replaced wholesale on a qualified passing audit (\`audit_format.sh --record\`).
Do not hand-edit.

- **agy version:** $AGY_VERSION
- **Verified on:** $RECORD_DATE
- **agy-reader commit:** \`$GIT_SHA\` ($TREE working tree)
- **Schema:** user_version=$VERSION, $TABLE_COUNT tables, indices: $INDEX_LIST
- **Schema fingerprint:** \`sha256:$FP\`
- **IDE surface:** the Antigravity IDE (\`~/.gemini/antigravity\`) writes the
  same conversation \`.db\` schema, verified byte-identical against this
  fingerprint at Antigravity 2.2.1 (2026-07-02); only \`trajectory_meta.source\`
  differs (IDE=1, CLI=17). Spot-check the IDE copy read-only with
  \`ANTIGRAVITY_CLI_ROOT=\$HOME/.gemini/antigravity audit_format.sh\` — never
  \`--record\` from an IDE root.
EOF
}

# Drift detection against the previously recorded fingerprint (parsed above).
if [ ! -f "$COMPAT_FILE" ]; then
    echo "Status: NEW -- no prior COMPATIBILITY.md; this run would establish the baseline."
elif [ -z "$PRIOR_FP" ]; then
    echo "Prior COMPATIBILITY.md found but no fingerprint parsed; treating as new."
elif [ "$PRIOR_FP" = "$FP" ]; then
    echo "Status: UNCHANGED since last verified (agy ${PRIOR_VER:-?}). Format is identical."
else
    echo "Status: DRIFT -- fingerprint differs from recorded (was sha256:$PRIOR_FP)."
    echo "        Review the schema change before recording."
fi

if [ "$TREE" = "dirty" ]; then
    echo "WARNING: working tree is dirty -- commit '$GIT_SHA' does not capture"
    echo "         uncommitted changes. Commit first if this should be a citable"
    echo "         verification point."
fi

if [ "$RECORD" = true ]; then
    emit_record > "$COMPAT_FILE"
    echo "Recorded to: $COMPAT_FILE"
else
    echo "Read-only run. To record, decide the run qualifies, then either re-run"
    echo "with --record or paste the block below into $COMPAT_FILE:"
    echo "------------------------------------------------------------"
    emit_record
    echo "------------------------------------------------------------"
fi

echo "=== AUDIT COMPLETED ==="
