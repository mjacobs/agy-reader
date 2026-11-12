"""Add the authenticated executor CHAINED termination reason captured with agy 1.5.6."""
import argparse
import gzip
import hashlib
from pathlib import Path
import tempfile

from google.protobuf import descriptor_pb2
import generate_options145 as helpers

SNAPSHOT = Path(__file__).resolve().parents[2] / "artifacts/proto-snapshots/agy-1.5.6-7e5bbdcbf689/closure.protoset"
SNAPSHOT_HASH = "e44e34f43926494831dd395d43d51c3afaf45e596ec71850fb980ef754880346"
BASE_HASH = "a7b3b707bfbe9da7c4961fe925211b338e71717ae3b8ef0cd0224a29fd4a96c6"


def augment(base, snapshot=SNAPSHOT):
    data = snapshot.read_bytes()
    helpers.require(hashlib.sha256(data).hexdigest() == SNAPSHOT_HASH, "snapshot hash mismatch")
    source = descriptor_pb2.FileDescriptorSet.FromString(data)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    target, current = (helpers.one(bundle.file, helpers.CORTEX_FILE) for bundle in (result, source))
    helpers.require((target.package, target.syntax) == (current.package, current.syntax),
                    "source and target file semantics differ")
    original = helpers.one(helpers.one(current.enum_type, "ExecutorTerminationReason").value,
                           "EXECUTOR_TERMINATION_REASON_CHAINED")
    helpers.require(original.number == 12, "unexpected CHAINED enum number")
    enum = helpers.one(target.enum_type, "ExecutorTerminationReason")
    helpers.require(not any(v.name == original.name or v.number == original.number for v in enum.value),
                    "termination reason already declared")
    enum.value.add().CopyFrom(original)
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
