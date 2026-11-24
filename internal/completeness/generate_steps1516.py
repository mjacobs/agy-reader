"""Add 1.5.16 planner and prompt steps and policy coverage to the historical decoder."""
import gzip
import hashlib
from pathlib import Path

from google.protobuf import descriptor_pb2
import generate_options145 as helpers

SNAPSHOT = (Path(__file__).resolve().parents[2] /
            "artifacts/proto-snapshots/agy-1.5.16-05245f4346d1/closure.protoset")
SNAPSHOT_HASH = "2701c5443ac07ed56fd702f3a947aa9709e19b25c2ea8a2053e155a37cd8453a"
CHAT = "third_party/jetski/chat_pb/chat.proto"
PREFIX = "learning/genai/api/interactions/proto/"
NEW_FILES = tuple(PREFIX + name + ".proto" for name in
                  ("agent_tools", "elicitation", "user_steps", "steps"))
CONTENT = PREFIX + "content.proto"
HISTORICAL_CONTENT_HASH = "88dd9d08c2a85e2e27b8b94ff80114ab7e7e6a1ac730af057acb4dcc78b2163d"
FIELDS = ((CHAT, "ChatMessagePrompt", "steps"),
          (helpers.CORTEX_FILE, "CortexStepPlannerResponse", "steps"),
          (helpers.CORTEX_FILE, "PolicyGuardianConfig", "coverage"))


def check_historical_content(content):
    digest = hashlib.sha256(content.SerializeToString(deterministic=True)).hexdigest()
    helpers.require(digest == HISTORICAL_CONTENT_HASH, "historical content descriptor mismatch")


def augment(base, snapshot=SNAPSHOT):
    raw = snapshot.read_bytes()
    helpers.require(hashlib.sha256(raw).hexdigest() == SNAPSHOT_HASH, "snapshot hash mismatch")
    source = descriptor_pb2.FileDescriptorSet.FromString(raw)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    # The 1.5.16 content file retains the historical fields and enum values,
    # and includes the full media types reachable from a structured step.
    content = helpers.one(result.file, CONTENT)
    check_historical_content(content)
    updated = helpers.one(source.file, CONTENT)
    # BASH execution remains outside the reader's qualified language set.
    # Keep the historical declaration while updating the surrounding content.
    def languages(file):
        call = helpers.one(file.message_type, "CodeExecutionCallContent")
        arguments = helpers.one(call.nested_type, "CodeExecutionCallArguments")
        return helpers.one(arguments.enum_type, "Language")

    languages(updated).CopyFrom(languages(content))
    content.CopyFrom(updated)
    for name in NEW_FILES:
        added = result.file.add()
        added.CopyFrom(helpers.one(source.file, name))
        if name == PREFIX + "steps.proto":
            call = helpers.one(added.message_type, "CodeExecutionCallStep")
            args = helpers.one(call.nested_type, "CodeExecutionCallStepArguments")
            language = helpers.one(args.enum_type, "Language")
            language.value.remove(helpers.one(language.value, "BASH"))
    for name in (CHAT, helpers.CORTEX_FILE):
        helpers.one(result.file, name).dependency.append(NEW_FILES[-1])
    cortex = helpers.one(result.file, helpers.CORTEX_FILE)
    cortex.message_type.add().CopyFrom(helpers.one(
        helpers.one(source.file, helpers.CORTEX_FILE).message_type, "PolicyGuardianCoverage"))
    for name, message, field in FIELDS:
        target = helpers.one(helpers.one(result.file, name).message_type, message)
        original = helpers.one(helpers.one(source.file, name).message_type, message)
        helpers.append_field(target, original, helpers.one(original.field, field))
    helpers.load_pool(result)
    return result


def strip(bundle, historical=None):
    if historical is None:
        base = Path(__file__).with_name("schema-base-agy134.pb.gz")
        historical = descriptor_pb2.FileDescriptorSet.FromString(gzip.decompress(base.read_bytes()))
    content = helpers.one(historical.file, CONTENT)
    check_historical_content(content)
    helpers.one(bundle.file, CONTENT).CopyFrom(content)
    for name, message, field in FIELDS:
        target = helpers.one(helpers.one(bundle.file, name).message_type, message)
        added = helpers.one(target.field, field)
        if added.HasField("oneof_index"):
            del target.oneof_decl[added.oneof_index]
        target.field.remove(added)
    cortex = helpers.one(bundle.file, helpers.CORTEX_FILE)
    cortex.message_type.remove(helpers.one(cortex.message_type, "PolicyGuardianCoverage"))
    for name in (CHAT, helpers.CORTEX_FILE):
        helpers.one(bundle.file, name).dependency.remove(NEW_FILES[-1])
    for name in NEW_FILES:
        bundle.file.remove(helpers.one(bundle.file, name))
    return bundle
