"""Retain agy 1.5.14 additions to messages in the historical decoder."""
from pathlib import Path

import generate_options145 as helpers
from generate_features1513 import COMMON

SNAPSHOT = (Path(__file__).resolve().parents[2] /
            "artifacts/proto-snapshots/agy-1.5.14-37873b806239/closure.protoset")
SNAPSHOT_HASH = "a48906e588f470c2d37db003a5ed41a7a9989bfa7bdfb562d4b698f60af038dd"


def additions(bundle):
    common = helpers.one(bundle.file, COMMON)
    for enum, value in (
        ("CascadeNUXTrigger", "CASCADE_NUX_TRIGGER_SIDEBAR_INBOX_MODE_DEFAULT"),
        ("CliAgentMode", "CLI_AGENT_MODE_PLAN"),
    ):
        yield helpers.one(common.enum_type, enum), "value", value

    for name in ("SupportedThinkingLevel", "ModelVariantList"):
        yield common, "message_type", name
    models = helpers.one(common.message_type, "CascadeModelConfigData")
    yield models, "nested_type", "GroupedModelsEntry"
    yield models, "field", "grouped_models"

    cortex = helpers.one(bundle.file, helpers.CORTEX_FILE)
    for name in ("CustomizationScope", "CustomizationInheritanceMode"):
        yield cortex, "enum_type", name

    yield cortex, "message_type", "SidecarSandboxConfig"
    yield helpers.one(cortex.message_type, "SidecarConfig"), "field", "sandbox"

    command = helpers.one(cortex.message_type, "RunCommandToolConfig")
    proxy = helpers.one(command.nested_type, "SandboxProxy")
    yield proxy, "nested_type", "Rpc"
    yield proxy, "field", "rpc"

    for message, field in (
        ("CascadePlannerConfig", "response_language"),
        ("CortexMemory", "customization_scope"),
        ("CustomizationDiscoveryConfig", "inheritance_mode"),
        ("FlowSpec", "customization_scope"),
        ("WorkflowSpec", "customization_scope"),
        ("McpToolConfig", "enable_tool_summary"),
        ("PolicyGuardianConfig", "enable_user_approval_check"),
        ("PolicyGuardianConfig", "enable_confirmed_repeat_check"),
        ("RunCommandToolConfig", "enable_tool_summary"),
    ):
        yield helpers.one(cortex.message_type, message), "field", field


def augment(base, snapshot=SNAPSHOT):
    return helpers.apply_additions(base, snapshot, SNAPSHOT_HASH, additions)


def strip(bundle):
    return helpers.remove_additions(bundle, additions)
