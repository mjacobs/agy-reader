"""Add the authenticated workflow child title without replacing historical definitions."""
import argparse
import gzip
import hashlib
from pathlib import Path
import tempfile

from google.protobuf import descriptor_pb2
import generate_options145 as helpers

SNAPSHOT = Path(__file__).resolve().parents[2] / 'artifacts/proto-snapshots/agy-1.5.9-39816d49d5ab/closure.protoset'
SNAPSHOT_HASH = '2134e03f83ba5d97a3fd8b5fa7574154be0cb71294e373a90a5cad3fd7e481d2'
BASE_HASH = '6708a33891214c32c2a4b85fab1c93e2974cc25e69038d488e369414f700f799'


def augment(base, snapshot=SNAPSHOT):
    data = snapshot.read_bytes()
    helpers.require(hashlib.sha256(data).hexdigest() == SNAPSHOT_HASH, 'snapshot hash mismatch')
    source = descriptor_pb2.FileDescriptorSet.FromString(data)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    target, current = (helpers.one(bundle.file, helpers.CORTEX_FILE) for bundle in (result, source))
    helpers.require((target.package, target.syntax) == (current.package, current.syntax),
                    'source and target file semantics differ')
    message, original = (helpers.one(file.message_type, 'ToolSourceMetadata') for file in (target, current))
    field = helpers.one(original.field, 'title')
    helpers.require(field.number == 4, 'unexpected title field number')
    helpers.require(not any(f.name == field.name or f.number == field.number for f in message.field),
                    'title already declared')
    message.field.add().CopyFrom(field)
    helpers.load_pool(result)
    return result


def strip(bundle):
    message = helpers.one(helpers.one(bundle.file, helpers.CORTEX_FILE).message_type, 'ToolSourceMetadata')
    message.field.remove(helpers.one(message.field, 'title'))
    return bundle


def generate(previous, output):
    helpers.require(output.resolve() not in {previous.resolve(), SNAPSHOT.resolve()},
                    'refusing to replace an authenticated input')
    data = gzip.decompress(previous.read_bytes())
    helpers.require(hashlib.sha256(data).hexdigest() == BASE_HASH, 'base hash mismatch')
    result = augment(descriptor_pb2.FileDescriptorSet.FromString(data))
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(dir=output.parent, prefix='.schema-', delete=False) as stream:
            temporary = Path(stream.name)
            stream.write(gzip.compress(result.SerializeToString(deterministic=True), mtime=0))
        temporary.chmod(0o644)
        temporary.replace(output)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('previous', type=Path)
    parser.add_argument('output', type=Path)
    args = parser.parse_args()
    try:
        generate(args.previous, args.output)
    except (ValueError, OSError) as error:
        parser.exit(1, f'refused: {error}\n')
