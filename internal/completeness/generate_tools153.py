"""Add the authenticated SubagentSpec.model_option used by agy 1.5.3."""
import argparse
import gzip
import hashlib
from pathlib import Path
import tempfile

from google.protobuf import descriptor_pb2
import generate_options145 as helpers

SNAPSHOT = Path(__file__).resolve().parents[2] / "artifacts/proto-snapshots/agy-1.5.3-7ca212748ca0/closure.protoset"
SNAPSHOT_HASH = "af90876809de9cbb670c7e8d3dd973aaa81542698c114480c7c1a8a1fb23f1e2"
BASE_HASH = "55ef33aef1b9dbe728ce698bf1db0088fd78016c9c526d9edb0dfe5aa0ce19d4"


def augment(base):
    data = SNAPSHOT.read_bytes()
    helpers.require(hashlib.sha256(data).hexdigest() == SNAPSHOT_HASH, "snapshot hash mismatch")
    source = descriptor_pb2.FileDescriptorSet.FromString(data)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    target, current = (helpers.one(bundle.file, helpers.CORTEX_FILE) for bundle in (result, source))
    helpers.require((target.package, target.syntax) == (current.package, current.syntax),
                    "source and target file semantics differ")
    original = helpers.one(current.message_type, "SubagentSpec")
    field = helpers.checked_field(original, "model_option", 12,
                                  descriptor_pb2.FieldDescriptorProto.TYPE_STRING, "modelOption")
    helpers.append_field(helpers.one(target.message_type, "SubagentSpec"), original, field)
    helpers.load_pool(result)
    return result


def generate(previous, output):
    helpers.require(output.resolve() not in {previous.resolve(), SNAPSHOT.resolve()},
                    "refusing to replace an authenticated input")
    data = gzip.decompress(previous.read_bytes())
    helpers.require(hashlib.sha256(data).hexdigest() == BASE_HASH, "base hash mismatch")
    result = augment(descriptor_pb2.FileDescriptorSet.FromString(data))
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(dir=output.parent, prefix=".schema-", delete=False) as stream:
            temporary = Path(stream.name)
            stream.write(gzip.compress(result.SerializeToString(deterministic=True), mtime=0))
        temporary.chmod(0o644)
        temporary.replace(output)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("previous", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    try:
        generate(args.previous, args.output)
    except (ValueError, OSError) as error:
        parser.exit(1, f"refused: {error}\n")
