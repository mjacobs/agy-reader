"""Add authenticated prompt-template overrides without replacing historical definitions."""
import argparse
import gzip
import hashlib
from pathlib import Path
import tempfile

from google.protobuf import descriptor_pb2
import generate_options145 as helpers

SNAPSHOT = Path(__file__).resolve().parents[2] / "artifacts/proto-snapshots/agy-1.5.7-5df525a2d690/closure.protoset"
SNAPSHOT_HASH = "f3de51c7f2a7317d073cabcea13cd42a7f7f1adf3986c3bf5b3f07035b120d46"
BASE_HASH = "834fd630ac230fb84421e94ad773da9bbf6aa5f0cd277f3a7dc4343157a9818b"


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
    target, current = (helpers.one(file.message_type, "PromptSectionCustomizationConfig")
                       for file in (target, current))
    field = helpers.one(current.field, "template_overrides")
    helpers.require(field.number == 5, "unexpected template_overrides field number")
    helpers.require(not any(f.name == field.name or f.number == field.number for f in target.field),
                    "template_overrides already declared")
    helpers.require(not any(m.name == "TemplateOverridesEntry" for m in target.nested_type),
                    "template map entry already declared")
    target.nested_type.add().CopyFrom(helpers.one(current.nested_type, "TemplateOverridesEntry"))
    target.field.add().CopyFrom(field)
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
