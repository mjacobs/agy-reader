"""Add the authenticated Schedule timestamp without replacing historical definitions."""
import argparse
import gzip
import hashlib
from pathlib import Path
import tempfile

from google.protobuf import descriptor_pb2
import generate_options145 as helpers
from generate_tools158 import TOOLS_FILE

SNAPSHOT = Path(__file__).resolve().parents[2] / 'artifacts/proto-snapshots/agy-1.5.10-20d6f74672ce/closure.protoset'
SNAPSHOT_HASH = '5e2adf25951e87fe151b3fd097ab2c772990ecfb167f3b2025cab40119aae1b1'
BASE_HASH = 'bfcb3c2e926ced1cef32c2eb5bcf329554b9ad3d35183f16eba6749bb9a3ff6c'
TIMESTAMP = 'google/protobuf/timestamp.proto'


def augment(base, snapshot=SNAPSHOT):
    data = snapshot.read_bytes()
    helpers.require(hashlib.sha256(data).hexdigest() == SNAPSHOT_HASH, 'snapshot hash mismatch')
    source = descriptor_pb2.FileDescriptorSet.FromString(data)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    target, current = (helpers.one(bundle.file, TOOLS_FILE) for bundle in (result, source))
    helpers.require((target.package, target.syntax, target.edition) == (current.package, current.syntax, current.edition),
                    'source and target file semantics differ')
    message, original = (helpers.one(file.message_type, 'Schedule') for file in (target, current))
    field = helpers.one(original.field, 'fire_time')
    helpers.require(field.number == 5 and field.type == field.TYPE_MESSAGE and
                    field.type_name == '.google.protobuf.Timestamp', 'unexpected Schedule field')
    helpers.require(not any(f.name == field.name or f.number == field.number for f in message.field),
                    'fire_time already declared')
    helpers.require(TIMESTAMP in current.dependency and TIMESTAMP not in target.dependency,
                    'unexpected Timestamp import')
    target.dependency.append(TIMESTAMP)
    message.field.add().CopyFrom(field)
    helpers.load_pool(result)
    return result


def strip(bundle):
    target = helpers.one(bundle.file, TOOLS_FILE)
    message = helpers.one(target.message_type, 'Schedule')
    message.field.remove(helpers.one(message.field, 'fire_time'))
    target.dependency.remove(TIMESTAMP)
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
