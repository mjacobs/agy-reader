"""Add the authenticated agy 1.4.10 CortexStepCheckpoint.captured_parent_messages field."""
import argparse
import gzip
import hashlib
from pathlib import Path
import tempfile

from google.protobuf import descriptor_pb2
import generate_media149 as prior

GenerationError = prior.GenerationError
require, one = prior.require, prior.one
SNAPSHOT_SHA256 = "801110a449d02729f2a2ae23d466215e954839521e6b2b519ce4fc450ec1419e"
CORTEX_FILE = prior.CORTEX_FILE
MESSAGE_NAME = "CortexStepCheckpoint"
FIELD_NAME = "captured_parent_messages"
FIELD_NUMBER = 31
FIELD_TYPE_NAME = ".exa.cortex_pb.CapturedUserRequest"


def augment(base, snapshot):
    data = snapshot.read_bytes()
    require(hashlib.sha256(data).hexdigest() == SNAPSHOT_SHA256,
            "snapshot hash does not match the verified 1.4.12 definitions")
    source = descriptor_pb2.FileDescriptorSet.FromString(data)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    target, current = one(result.file, CORTEX_FILE), one(source.file, CORTEX_FILE)
    require((target.package, target.syntax) == (current.package, current.syntax),
            "source and target file semantics differ")
    original = one(target.message_type, MESSAGE_NAME)
    field = one(one(current.message_type, MESSAGE_NAME).field, FIELD_NAME)
    require(field.number == FIELD_NUMBER and field.type == field.TYPE_MESSAGE and
            field.type_name == FIELD_TYPE_NAME and not field.HasField("oneof_index") and
            not field.proto3_optional and field.label == field.LABEL_REPEATED and
            field.json_name == "capturedParentMessages",
            f"unexpected {MESSAGE_NAME}.{FIELD_NAME} definition")
    require(not any(f.name == FIELD_NAME or f.number == FIELD_NUMBER for f in original.field),
            f"target already contains {MESSAGE_NAME}.{FIELD_NAME}")
    original.field.add().CopyFrom(field)
    prior.prior.prior.prior.prior.load_pool(result)
    return result


def generate(options145, compaction146, finish147, subagents148, media149, snapshot, output):
    inputs = [options145, compaction146, finish147, subagents148, media149, snapshot,
              prior.prior.prior.prior.DEFAULT_BASE]
    require(output.resolve() not in {p.resolve() for p in inputs},
            "refusing to replace an authenticated input")
    with tempfile.TemporaryDirectory() as directory:
        base, _ = prior.generate(options145, compaction146, finish147, subagents148, media149,
                                 Path(directory) / "prior.pb.gz")
    result = augment(base, snapshot)
    data = gzip.compress(result.SerializeToString(deterministic=True), mtime=0)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(dir=output.parent, prefix=".schema-", delete=False) as stream:
            temporary = Path(stream.name)
            stream.write(data)
        temporary.chmod(0o644)
        temporary.replace(output)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)
    return result, data


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("options145", "compaction146", "finish147", "subagents148", "media149", "snapshot", "output"):
        parser.add_argument(name, type=Path)
    args = parser.parse_args()
    try:
        result, data = generate(**vars(args))
    except (ValueError, OSError, KeyError) as error:
        parser.exit(1, f"refused: {error}\n")
    print(len(result.file), "files", len(data), "compressed bytes", hashlib.sha256(data).hexdigest())


if __name__ == "__main__":
    main()
