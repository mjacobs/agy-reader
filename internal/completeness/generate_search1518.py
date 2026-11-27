"""Add the typed SearchWeb result observed in agy 1.5.18 ACP conversations."""
from generate_options145 import apply_additions, one, remove_additions
from generate_subagents1518 import SNAPSHOT, SNAPSHOT_HASH
from generate_tools158 import TOOLS_FILE


def additions(bundle):
    yield one(bundle.file, TOOLS_FILE), "message_type", "SearchWeb"


def augment(base, snapshot=SNAPSHOT):
    return apply_additions(base, snapshot, SNAPSHOT_HASH, additions)


def strip(bundle):
    return remove_additions(bundle, additions)
