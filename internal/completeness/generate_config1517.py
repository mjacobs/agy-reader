"""Add 1.5.17 command environments, model icons and guardian settings."""
from pathlib import Path

from generate_features1513 import COMMON
from generate_options145 import CORTEX_FILE, apply_additions, one, remove_additions

SNAPSHOT = (Path(__file__).resolve().parents[2] /
            "artifacts/proto-snapshots/agy-1.5.17-f3f384bd2c15/closure.protoset")
SNAPSHOT_HASH = "6adfd4ce8a28380ce9ce091f54d0f89d99df48af0b3127d3c5f8dc8c7d2c54e0"


def additions(bundle):
    common = one(bundle.file, COMMON)
    yield common, "message_type", "IconDetails"
    yield one(common.message_type, "ClientModelConfig"), "field", "icon_details"
    cortex = one(bundle.file, CORTEX_FILE)
    command = one(cortex.message_type, "RunCommandToolConfig")
    yield command, "nested_type", "EnvEntry"
    yield command, "field", "env"
    policy = one(cortex.message_type, "PolicyGuardianConfig")
    yield policy, "enum_type", "SkillScanMode"
    yield policy, "field", "enable_proctor_read_budget"
    yield policy, "field", "proctor_skill_scan_mode"
    coverage = one(cortex.message_type, "PolicyGuardianCoverage")
    for field in ("url_data_exfil", "destructive_action", "remote_code_exec",
                  "dependency_install", "skill_supply_chain", "mcp_tool_use", "recon",
                  "user_coercion", "post_exec_tampering", "network_exposure"):
        yield coverage, "field", field


def augment(base, snapshot=SNAPSHOT):
    return apply_additions(base, snapshot, SNAPSHOT_HASH, additions)


def strip(bundle):
    return remove_additions(bundle, additions)
