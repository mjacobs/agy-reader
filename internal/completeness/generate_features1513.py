"""Apply the agy 1.5.13 trajectory schema delta to the historical decoder."""
import hashlib
from pathlib import Path

from google.protobuf import descriptor_pb2
import generate_options145 as helpers

SNAPSHOT = (Path(__file__).resolve().parents[2] /
            "artifacts/proto-snapshots/agy-1.5.13-c3fd2c848abf/closure.protoset")
SNAPSHOT_HASH = "e034e570f22bb6527431380dc430e0a740caa3996e1cc67480d5a28e59493875"
COMMON = "third_party/jetski/codeium_common_pb/codeium_common.proto"
FIELDS = {
    "CortexStepAgencyToolCall": "full_output_uri",
    "CortexStepRunCommand": "environment_id",
    "CortexTrajectoryMetadata": "interaction_mode",
    "HooksDiscoveryConfig": "plugin_paths",
    "PermissionInteractionSpec": "once_covers_task",
    "RunCommandToolConfig": "enable_environment_tools",
}


def augment(base, snapshot=SNAPSHOT):
    raw = snapshot.read_bytes()
    helpers.require(hashlib.sha256(raw).hexdigest() == SNAPSHOT_HASH, "snapshot hash mismatch")
    source = descriptor_pb2.FileDescriptorSet.FromString(raw)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    common, original = (helpers.one(b.file, COMMON) for b in (result, source))
    # CliSettings references CliAgentMode, which predates this release but was
    # absent from the historical decoder.
    common.enum_type.add().CopyFrom(helpers.one(original.enum_type, "CliAgentMode"))
    common.message_type.add().CopyFrom(helpers.one(original.message_type, "CliSettings"))
    model, latest = (helpers.one(f.message_type, "ModelFeatures") for f in (common, original))
    for field in latest.field:
        if not field.proto3_optional:
            continue
        target = helpers.one(model.field, field.name)
        helpers.require(target.type == target.TYPE_BOOL and target.number == field.number
                        and not target.HasField("oneof_index"), "incompatible model feature")
        target.proto3_optional = True
        target.oneof_index = len(model.oneof_decl)
        model.oneof_decl.add().CopyFrom(latest.oneof_decl[field.oneof_index])

    cortex, original = (helpers.one(b.file, helpers.CORTEX_FILE) for b in (result, source))
    cortex.enum_type.add().CopyFrom(helpers.one(original.enum_type, "InteractionMode"))
    for name, field_name in FIELDS.items():
        dst, src = (helpers.one(f.message_type, name) for f in (cortex, original))
        helpers.append_field(dst, src, helpers.one(src.field, field_name))
    helpers.load_pool(result)
    return result


def strip(bundle):
    common = helpers.one(bundle.file, COMMON)
    common.enum_type.remove(helpers.one(common.enum_type, "CliAgentMode"))
    common.message_type.remove(helpers.one(common.message_type, "CliSettings"))
    model = helpers.one(common.message_type, "ModelFeatures")
    for field in model.field:
        if field.proto3_optional:
            field.ClearField("proto3_optional")
            field.ClearField("oneof_index")
    del model.oneof_decl[:]
    cortex = helpers.one(bundle.file, helpers.CORTEX_FILE)
    cortex.enum_type.remove(helpers.one(cortex.enum_type, "InteractionMode"))
    for name, field_name in FIELDS.items():
        message = helpers.one(cortex.message_type, name)
        field = helpers.one(message.field, field_name)
        if field.HasField("oneof_index"):
            del message.oneof_decl[field.oneof_index]
        message.field.remove(field)
    return bundle
