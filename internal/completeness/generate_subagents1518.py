"""Retain the optional running flag on agy 1.5.18 subagent checkpoints."""
from pathlib import Path

from generate_options145 import CORTEX_FILE, apply_additions, one, remove_additions

SNAPSHOT = (Path(__file__).resolve().parents[2] /
            "artifacts/proto-snapshots/agy-1.5.18-7c96ad8c944c/closure.protoset")
SNAPSHOT_HASH = "a97e5d40452a572a8e28edb84c70a10c89e8719a61d15c8e5e3f4df1e7afe86b"


def additions(bundle):
    cortex = one(bundle.file, CORTEX_FILE)
    yield one(cortex.message_type, "SubagentSnapshot"), "field", "running"


def augment(base, snapshot=SNAPSHOT):
    return apply_additions(base, snapshot, SNAPSHOT_HASH, additions)


def strip(bundle):
    return remove_additions(bundle, additions)
