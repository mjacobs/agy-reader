"""Add the two authenticated agy 1.4.9 fields without replacing historical definitions."""
import argparse
import gzip
import hashlib
from pathlib import Path
import tempfile

from google.protobuf import descriptor_pb2
import generate_subagents148 as prior

GenerationError = prior.GenerationError
require, one = prior.require, prior.one
SNAPSHOT_SHA256 = "1db2f7b0fd919476541e2dc0ec7aadc9b3f7e56ba5f8cf26c06663ad1eaa0f68"
CORTEX_FILE = "third_party/jetski/cortex_pb/cortex.proto"
CONTENT_FILE = "learning/genai/api/interactions/proto/content.proto"
ADDITIONS = (
    ("CortexStepPlannerResponse", "media", 20, ".exa.codeium_common_pb.Media", True),
    ("GenericStepResult", "result_value", 11, ".genai.Value", False),
)


def augment(base, snapshot):
    data = snapshot.read_bytes()
    require(hashlib.sha256(data).hexdigest() == SNAPSHOT_SHA256,
            "snapshot hash does not match the verified 1.4.9 definitions")
    source = descriptor_pb2.FileDescriptorSet.FromString(data)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    target, current = one(result.file, CORTEX_FILE), one(source.file, CORTEX_FILE)
    require((target.package, target.syntax) == (current.package, current.syntax),
            "source and target file semantics differ")
    for message_name, field_name, number, type_name, repeated in ADDITIONS:
        original = one(target.message_type, message_name)
        field = one(one(current.message_type, message_name).field, field_name)
        require(field.number == number and field.type == field.TYPE_MESSAGE and
                field.type_name == type_name and not field.HasField("oneof_index") and
                not field.proto3_optional and
                field.label == (field.LABEL_REPEATED if repeated else field.LABEL_OPTIONAL),
                f"unexpected {message_name}.{field_name} definition")
        require(not any(f.name == field_name or f.number == number for f in original.field),
                f"target already contains {message_name}.{field_name}")
        original.field.add().CopyFrom(field)
    # Value already exists in the historical bundle; cortex newly references it.
    # Do not replace its dependencies or enable any unrelated newer fields.
    require(CONTENT_FILE in current.dependency, "missing authenticated genai import")
    require(CONTENT_FILE not in target.dependency, "target already imports genai content")
    require(one(one(result.file, CONTENT_FILE).message_type, "Value") ==
            one(one(source.file, CONTENT_FILE).message_type, "Value"),
            "historical genai.Value declaration differs")
    target.dependency.append(CONTENT_FILE)
    prior.prior.prior.prior.load_pool(result)
    return result


def generate(options145, compaction146, finish147, subagents148, snapshot, output):
    inputs = [options145, compaction146, finish147, subagents148, snapshot,
              prior.prior.prior.DEFAULT_BASE]
    require(output.resolve() not in {p.resolve() for p in inputs},
            "refusing to replace an authenticated input")
    with tempfile.TemporaryDirectory() as directory:
        base, _ = prior.generate(options145, compaction146, finish147, subagents148,
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
    for name in ("options145", "compaction146", "finish147", "subagents148", "snapshot", "output"):
        parser.add_argument(name, type=Path)
    args = parser.parse_args()
    try:
        result, data = generate(**vars(args))
    except (ValueError, OSError, KeyError) as error:
        parser.exit(1, f"refused: {error}\n")
    print(len(result.file), "files", len(data), "compressed bytes", hashlib.sha256(data).hexdigest())


if __name__ == "__main__":
    main()
