"""Add authenticated agy 1.4.8 subagent state without replacing historical fields."""
import argparse
import gzip
import hashlib
from pathlib import Path
import tempfile
from google.protobuf import descriptor_pb2
import generate_finish147 as prior

GenerationError = prior.GenerationError
require, one = prior.require, prior.one
SNAPSHOT_SHA256 = "964a14ddf6af95b2772cabc41ed25df7d913631779367a2e51c569f43020a133"
TOOLS_FILE = prior.TOOLS_FILE


def augment(base, snapshot):
    data = snapshot.read_bytes()
    require(hashlib.sha256(data).hexdigest() == SNAPSHOT_SHA256,
            "snapshot hash does not match the verified 1.4.8 definitions")
    source = descriptor_pb2.FileDescriptorSet.FromString(data)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    original = one(one(result.file, TOOLS_FILE).message_type, "ManageSubagents")
    updated = one(one(source.file, TOOLS_FILE).message_type, "ManageSubagents")
    historical = descriptor_pb2.DescriptorProto()
    historical.CopyFrom(updated)
    historical.field.remove(one(historical.field, "subagents"))
    historical.nested_type.remove(one(historical.nested_type, "SubagentState"))
    require(historical == original, "historical ManageSubagents declaration differs")
    original.CopyFrom(updated)
    prior.prior.prior.load_pool(result)
    return result


def generate(options145, compaction146, finish147, snapshot, output):
    inputs = [options145, compaction146, finish147, snapshot, prior.prior.DEFAULT_BASE]
    require(output.resolve() not in {p.resolve() for p in inputs},
            "refusing to replace an authenticated input")
    with tempfile.TemporaryDirectory() as directory:
        base, _ = prior.generate(options145, compaction146, finish147,
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
    for name in ("options145", "compaction146", "finish147", "snapshot", "output"):
        parser.add_argument(name, type=Path)
    args = parser.parse_args()
    try:
        result, data = generate(**vars(args))
    except (ValueError, OSError, KeyError) as error:
        parser.exit(1, f"refused: {error}\n")
    print(len(result.file), "files", len(data), "compressed bytes", hashlib.sha256(data).hexdigest())


if __name__ == "__main__":
    main()
