"""Add directory snapshot and Orchestrator metadata to the historical decoder."""
import hashlib
from pathlib import Path

from google.protobuf import descriptor_pb2
import generate_options145 as helpers

SNAPSHOT = (Path(__file__).resolve().parents[2] /
            "artifacts/proto-snapshots/agy-1.5.15-7b004d26c3fd/closure.protoset")
SNAPSHOT_HASH = "aeda8b78c29c4e9e28718cb4dfc7261270128edec8429ba933d9df0a970adc71"


FIELDS = (
    ("SnapshotMetadata", "x20_directory"),
    ("DefaultAgentConfig", "plan_execute"),
)


def messages(bundle):
    return helpers.one(bundle.file, helpers.CORTEX_FILE).message_type


def augment(base, snapshot=SNAPSHOT):
    raw = snapshot.read_bytes()
    helpers.require(hashlib.sha256(raw).hexdigest() == SNAPSHOT_HASH, "snapshot hash mismatch")
    source = descriptor_pb2.FileDescriptorSet.FromString(raw)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    target, original = messages(result), messages(source)
    target.add().CopyFrom(helpers.one(original, "X20DirectorySnapshot"))
    for message_name, field_name in FIELDS:
        metadata = helpers.one(target, message_name)
        source_metadata = helpers.one(original, message_name)
        helpers.append_field(metadata, source_metadata, helpers.one(source_metadata.field, field_name))
    helpers.load_pool(result)
    return result


def strip(bundle):
    target = messages(bundle)
    for message_name, field_name in FIELDS:
        metadata = helpers.one(target, message_name)
        field = helpers.one(metadata.field, field_name)
        del metadata.oneof_decl[field.oneof_index]
        metadata.field.remove(field)
    target.remove(helpers.one(target, "X20DirectorySnapshot"))
    return bundle
