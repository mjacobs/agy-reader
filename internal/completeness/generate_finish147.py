"""Add only the agy 1.4.7 Finish payload to the qualified historical schema.

Rebuild the prior overlays from their authenticated inputs, then copy the exact
Finish declaration from the pinned 1.4.7 snapshot. Requires google.protobuf.
"""

import argparse
import gzip
import hashlib
import io
from pathlib import Path
import tempfile

from google.protobuf import descriptor_pb2
import generate_compaction146 as prior

GenerationError = prior.GenerationError
require, one = prior.require, prior.one
SNAPSHOT_SHA256 = "b3b02aee80c64f17550fde3aa0a998c2e3eb41fb6c5a6eb7e14e0b2cb4fe4ec2"
TOOLS_FILE = "third_party/jetski/cortex/core/tools_pb/tools.proto"


def augment(base, snapshot):
    data = snapshot.read_bytes()
    require(hashlib.sha256(data).hexdigest() == SNAPSHOT_SHA256,
            "snapshot hash does not match the verified 1.4.7 definitions")
    source = one(descriptor_pb2.FileDescriptorSet.FromString(data).file, TOOLS_FILE)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    target = one(result.file, TOOLS_FILE)
    require((target.package, target.syntax) == (source.package, source.syntax),
            "source and target file semantics differ")
    finish = one(source.message_type, "Finish")
    require(len(finish.field) == 2 and not finish.nested_type and not finish.enum_type
            and not finish.oneof_decl and not finish.extension,
            "unexpected Finish definition")
    prior.prior.checked_field(finish, "arguments_json", 1,
                              descriptor_pb2.FieldDescriptorProto.TYPE_STRING, "argumentsJson")
    prior.prior.checked_field(finish, "has_result_schema", 2,
                              descriptor_pb2.FieldDescriptorProto.TYPE_BOOL, "hasResultSchema")
    require(not any(m.name == "Finish" for m in target.message_type),
            "target already contains Finish")
    target.message_type.add().CopyFrom(finish)
    prior.prior.load_pool(result)
    return result


def generate(options145, compaction146, snapshot, output, base_path=prior.DEFAULT_BASE):
    require(output.resolve() not in {base_path.resolve(), snapshot.resolve()},
            "refusing to replace an authenticated input")
    base, source145 = prior.prior.verified_inputs(base_path, options145)
    base = prior.augment(prior.prior.augment(base, source145), prior.verified_source(compaction146))
    result = augment(base, snapshot)
    compressed = io.BytesIO()
    with gzip.GzipFile(fileobj=compressed, filename="", mode="wb", mtime=0) as stream:
        stream.write(result.SerializeToString(deterministic=True))
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(dir=output.parent, prefix=".schema-", delete=False) as stream:
            temporary = Path(stream.name)
            stream.write(compressed.getvalue())
        temporary.chmod(0o644)
        temporary.replace(output)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)
    return result, compressed.getvalue()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("options145", "compaction146", "snapshot", "output"):
        parser.add_argument(name, type=Path)
    args = parser.parse_args()
    try:
        result, data = generate(args.options145, args.compaction146, args.snapshot, args.output)
    except (ValueError, OSError, KeyError) as error:
        parser.exit(1, f"refused: {error}\n")
    print(len(result.file), "files", len(data), "compressed bytes", hashlib.sha256(data).hexdigest())


if __name__ == "__main__":
    main()
