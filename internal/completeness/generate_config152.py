"""Preserve historical model choices and add the authenticated 1.5.2 choices."""
import gzip
import hashlib
from pathlib import Path
import generate_config150 as prior
from google.protobuf import descriptor_pb2

SNAPSHOT = Path(__file__).resolve().parents[2] / "artifacts/proto-snapshots/agy-1.5.2-da1751e76584/closure.protoset"
SNAPSHOT_HASH = "2a61655fb6cd313634b55a3f7b04eef86b2f64dcd15df03baafeb33da1af2c3d"

def augment(base, snapshot=SNAPSHOT):
    data = snapshot.read_bytes()
    prior.require(hashlib.sha256(data).hexdigest() == SNAPSHOT_HASH, "snapshot hash mismatch")
    source = descriptor_pb2.FileDescriptorSet.FromString(data)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    target = prior.one(prior.one(result.file, prior.CORTEX_FILE).message_type, "SubagentModelOption")
    original = prior.one(prior.one(source.file, prior.CORTEX_FILE).message_type, "SubagentModelOption")
    prior.require(not target.oneof_decl and not target.nested_type, "already augmented model choices")
    target.nested_type.add().CopyFrom(prior.one(original.nested_type, "NamedModel"))
    target.oneof_decl.add().CopyFrom(prior.one(original.oneof_decl, "kind"))
    for name in ("inherit", "named_model"):
        target.field.add().CopyFrom(prior.one(original.field, name))
    prior.helpers.load_pool(result)
    return result

if __name__ == "__main__":
    import sys
    source, output = map(Path, sys.argv[1:])
    base = descriptor_pb2.FileDescriptorSet.FromString(gzip.decompress(source.read_bytes()))
    result = augment(base)
    prior.require(source.resolve() != output.resolve(), "refusing to overwrite input")
    prior.require(hashlib.sha256(gzip.decompress(source.read_bytes())).hexdigest() == "8579717d67c37459b6f61b0815650ae18412f5afbf522cdf81b9604ebe06bdd7", "base hash mismatch")
    import tempfile
    with tempfile.NamedTemporaryFile(dir=output.parent, delete=False) as stream:
        temporary = Path(stream.name)
        stream.write(gzip.compress(result.SerializeToString(deterministic=True), mtime=0))
    temporary.chmod(0o644)
    temporary.replace(output)
