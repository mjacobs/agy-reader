"""Add authenticated MCP and DefineSubagent declarations without replacing history."""
import argparse
import gzip
import hashlib
from pathlib import Path
import tempfile

from google.protobuf import descriptor_pb2
import generate_options145 as helpers

SNAPSHOT = Path(__file__).resolve().parents[2] / 'artifacts/proto-snapshots/agy-1.5.8-9db59b637110/closure.protoset'
SNAPSHOT_HASH = '82b4b9b0d3949c99528ab0d597bd2e82a0956cea9617c61b115ab8f996b66250'
BASE_HASH = 'f05ed42c8ceb5eea6cb003f354b1fffbe8f4169cdce2a297049849624bde4680'
TOOLS_FILE = 'third_party/jetski/cortex/core/tools_pb/tools.proto'


def augment(base, snapshot=SNAPSHOT):
    data = snapshot.read_bytes()
    helpers.require(hashlib.sha256(data).hexdigest() == SNAPSHOT_HASH, 'snapshot hash mismatch')
    source = descriptor_pb2.FileDescriptorSet.FromString(data)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    target, current = (helpers.one(bundle.file, TOOLS_FILE) for bundle in (result, source))
    helpers.require((target.package, target.syntax) == (current.package, current.syntax),
                    'source and target file semantics differ')
    message, original = (helpers.one(file.message_type, 'DefineSubagent') for file in (target, current))
    for name, number in [('description', 4), ('agent_config', 5)]:
        field = helpers.one(original.field, name)
        helpers.require(field.number == number, 'unexpected field number')
        helpers.require(not any(f.name == name or f.number == number for f in message.field), 'field already declared')
        message.field.add().CopyFrom(field)
    helpers.require(not any(m.name == 'McpTool' for m in target.message_type), 'McpTool already declared')
    target.message_type.add().CopyFrom(helpers.one(current.message_type, 'McpTool'))
    helpers.load_pool(result)
    return result


def strip(bundle):
    message = helpers.one(helpers.one(bundle.file, TOOLS_FILE).message_type, 'DefineSubagent')
    for name in ['description', 'agent_config']:
        message.field.remove(helpers.one(message.field, name))
    target = helpers.one(bundle.file, TOOLS_FILE)
    target.message_type.remove(helpers.one(target.message_type, 'McpTool'))
    return bundle


def generate(previous, output):
    helpers.require(output.resolve() not in {previous.resolve(), SNAPSHOT.resolve()}, 'refusing to replace an authenticated input')
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
