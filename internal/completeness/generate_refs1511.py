"""Preserve the observed agy 1.5.11 fields in transcript render references."""
import hashlib
from pathlib import Path

from google.protobuf import descriptor_pb2
import generate_options145 as helpers

SNAPSHOT = (Path(__file__).resolve().parents[2] /
            "artifacts/proto-snapshots/agy-1.5.11-01b063665007/closure.protoset")
SNAPSHOT_HASH = "f770d88ff74023cb4b38f9692c0d59774b87f48025200af392f2a185fdcded9d"
FIELDS = {"FileRef": ("total_lines", "total_bytes"), "AgentRef": ("log_uri", "workspace_uris")}


def reference(bundle, name):
    cortex = helpers.one(bundle.file, helpers.CORTEX_FILE)
    return helpers.one(helpers.one(cortex.message_type, "StepRenderInfo").nested_type, name)


def augment(base, snapshot=SNAPSHOT):
    raw = snapshot.read_bytes()
    helpers.require(hashlib.sha256(raw).hexdigest() == SNAPSHOT_HASH, "snapshot hash mismatch")
    source = descriptor_pb2.FileDescriptorSet.FromString(raw)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    for message, names in FIELDS.items():
        original, target = (reference(bundle, message) for bundle in (source, result))
        for name in names:
            field = helpers.one(original.field, name)
            helpers.require(all(f.name != name and f.number != field.number for f in target.field),
                            f"{message}.{name} already declared")
            added = target.field.add()
            added.CopyFrom(field)
            if field.HasField("oneof_index"):
                oneof = original.oneof_decl[field.oneof_index]
                helpers.require(field.proto3_optional and oneof.name == "_" + name,
                                f"unexpected {message}.{name} presence declaration")
                added.oneof_index = len(target.oneof_decl)
                target.oneof_decl.add().CopyFrom(oneof)
    helpers.load_pool(result)
    return result


def strip(bundle):
    for message, names in FIELDS.items():
        target = reference(bundle, message)
        for name in reversed(names):
            field = helpers.one(target.field, name)
            if field.HasField("oneof_index"):
                helpers.require(target.oneof_decl[-1].name == "_" + name,
                                f"unexpected {message}.{name} presence declaration")
                del target.oneof_decl[-1]
            target.field.remove(field)
    return bundle
