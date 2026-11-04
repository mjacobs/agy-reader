"""Add only the browser and subagent model configuration observed with agy 1.5.0."""
import argparse
import gzip
import hashlib
from pathlib import Path
import tempfile

from google.protobuf import descriptor_pb2
import generate_parentmsgs1410 as prior
import generate_options145 as helpers

GenerationError = helpers.GenerationError
require, one = helpers.require, helpers.one
CORTEX_FILE = helpers.CORTEX_FILE
SNAPSHOT_SHA256 = '817e08196b0d1e8bc5c206153e1755722eed24d69c6658fcacd6e066e4614bf8'
MESSAGES = ('ModelCatalogKey', 'SubagentModelOption', 'SubagentModelOptions', 'ModelTierConfig')
FIELDS = (
    ('CascadeConfig', 'model_tiers', 15,
     descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE, 'modelTiers', False,
     '.exa.cortex_pb.ModelTierConfig'),
    ('AntigravityBrowserToolConfig', 'enable_browser_subagent_webview', 30,
     descriptor_pb2.FieldDescriptorProto.TYPE_BOOL, 'enableBrowserSubagentWebview', True, ''),
    ('InvokeSubagentToolConfig', 'model_options', 10,
     descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE, 'modelOptions', False,
     '.exa.cortex_pb.SubagentModelOptions'),
)


def augment(base, snapshot):
    data = snapshot.read_bytes()
    require(hashlib.sha256(data).hexdigest() == SNAPSHOT_SHA256,
            'snapshot hash does not match the verified 1.5.0 definitions')
    source = descriptor_pb2.FileDescriptorSet.FromString(data)
    source.DiscardUnknownFields()
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    target, current = one(result.file, CORTEX_FILE), one(source.file, CORTEX_FILE)
    require((target.package, target.syntax) == (current.package, current.syntax),
            'source and target file semantics differ')
    for name in MESSAGES:
        require(not any(m.name == name for m in target.message_type),
                f'target already contains {name}')
        target.message_type.add().CopyFrom(one(current.message_type, name))
    for message, name, number, kind, json_name, optional, type_name in FIELDS:
        original = one(current.message_type, message)
        field = helpers.checked_field(original, name, number, kind, json_name,
                                      optional=optional, type_name=type_name)
        helpers.append_field(one(target.message_type, message), original, field)
    helpers.load_pool(result)
    return result


def generate(options145, compaction146, finish147, subagents148, media149,
             parentmsgs1410, snapshot, output):
    inputs = [options145, compaction146, finish147, subagents148, media149,
              parentmsgs1410, snapshot, helpers.DEFAULT_BASE]
    require(output.resolve() not in {p.resolve() for p in inputs},
            'refusing to replace an authenticated input')
    with tempfile.TemporaryDirectory() as directory:
        base, _ = prior.generate(*inputs[:6], Path(directory) / 'prior.pb.gz')
    result = augment(base, snapshot)
    data = gzip.compress(result.SerializeToString(deterministic=True), mtime=0)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(dir=output.parent, prefix='.schema-', delete=False) as stream:
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
    for name in ('options145', 'compaction146', 'finish147', 'subagents148', 'media149',
                 'parentmsgs1410', 'snapshot', 'output'):
        parser.add_argument(name, type=Path)
    args = parser.parse_args()
    try:
        result, data = generate(**vars(args))
    except (ValueError, OSError, KeyError) as error:
        parser.exit(1, f'refused: {error}\n')
    print(len(result.file), 'files', len(data), 'compressed bytes', hashlib.sha256(data).hexdigest())


if __name__ == '__main__':
    main()
