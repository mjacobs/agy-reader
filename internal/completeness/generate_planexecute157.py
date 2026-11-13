"""Add authenticated plan/execute configuration without replacing historical definitions."""
import argparse
import gzip
import hashlib
from pathlib import Path
import tempfile

from google.protobuf import descriptor_pb2
import generate_options145 as helpers

SNAPSHOT = Path(__file__).resolve().parents[2] / "artifacts/proto-snapshots/agy-1.5.7-5df525a2d690/closure.protoset"
SNAPSHOT_HASH = "f3de51c7f2a7317d073cabcea13cd42a7f7f1adf3986c3bf5b3f07035b120d46"
BASE_HASH = "e603b335cff88609a63b131cf92491c8af64f9c402e8282d967b060d61a6336a"


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
    planner, original = (helpers.one(file.message_type, "CascadePlannerConfig")
                         for file in (target, current))
    field = helpers.one(original.field, "plan_execute_config")
    helpers.require(field.number == 60, "unexpected plan_execute_config field number")
    helpers.require(not any(f.name == field.name or f.number == field.number for f in planner.field),
                    "plan_execute_config already declared")
    helpers.require(not any(m.name == "PlanExecuteConfig" for m in target.message_type),
                    "PlanExecuteConfig already declared")
    target.message_type.add().CopyFrom(helpers.one(current.message_type, "PlanExecuteConfig"))
    planner.field.add().CopyFrom(field)
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
