"""Retain three observed PolicyGuardianConfig fields from the agy 1.5.10 snapshot."""
import hashlib

from google.protobuf import descriptor_pb2
import generate_options145 as helpers
from generate_schedule1510 import SNAPSHOT, SNAPSHOT_HASH

FIELDS = (
    ("deployment", 15, descriptor_pb2.FieldDescriptorProto.TYPE_STRING, False),
    ("enable_irreversible_vetting", 18, descriptor_pb2.FieldDescriptorProto.TYPE_BOOL, True),
    ("enable_reversibility_adjudication", 19, descriptor_pb2.FieldDescriptorProto.TYPE_BOOL, True),
)


def policy(bundle):
    cortex = helpers.one(bundle.file, helpers.CORTEX_FILE)
    return helpers.one(cortex.message_type, "PolicyGuardianConfig")


def augment(base, snapshot=SNAPSHOT):
    raw = snapshot.read_bytes()
    helpers.require(hashlib.sha256(raw).hexdigest() == SNAPSHOT_HASH, "snapshot hash mismatch")
    source = descriptor_pb2.FileDescriptorSet.FromString(raw)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    target_file, source_file = (helpers.one(bundle.file, helpers.CORTEX_FILE) for bundle in (result, source))
    helpers.require(target_file.package == source_file.package and
                    target_file.syntax == source_file.syntax == "proto3",
                    "source and target file semantics differ")
    target, original = (policy(bundle) for bundle in (result, source))

    for name, number, kind, has_presence in FIELDS:
        field = helpers.one(original.field, name)
        helpers.require((field.number, field.type, field.proto3_optional, field.HasField("oneof_index")) ==
                        (number, kind, has_presence, has_presence), f"unexpected {name} declaration")
        helpers.require(not any(f.name == name or f.number == number for f in target.field),
                        f"{name} already declared")
        added = target.field.add()
        added.CopyFrom(field)
        if has_presence:
            oneof = original.oneof_decl[field.oneof_index]
            helpers.require(oneof.name == "_" + name, f"unexpected {name} presence declaration")
            added.oneof_index = len(target.oneof_decl)
            target.oneof_decl.add().CopyFrom(oneof)

    helpers.load_pool(result)
    return result


def strip(bundle):
    message = policy(bundle)
    for name, _, _, has_presence in reversed(FIELDS):
        message.field.remove(helpers.one(message.field, name))
        if has_presence:
            helpers.require(message.oneof_decl[-1].name == "_" + name, f"unexpected {name} presence declaration")
            del message.oneof_decl[-1]
    return bundle
