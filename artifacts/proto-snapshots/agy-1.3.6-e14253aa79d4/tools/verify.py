"""Verify this snapshot without agy or agy-reader. Requires protobuf 7.36.2."""
from pathlib import Path
import hashlib, json
from google.protobuf import descriptor_pb2 as d, descriptor_pool, message_factory, json_format
root = Path(__file__).resolve().parents[1]
manifest = json.loads((root / 'extracted/manifest.json').read_text())
raw = d.FileDescriptorSet.FromString((root / 'all-extracted.protoset').read_bytes())
assert len(raw.file) == len(manifest['files'])
for i, record in enumerate(manifest['files']):
    data = (root / 'extracted' / f'{i:04d}.pb').read_bytes()
    assert hashlib.sha256(data).hexdigest() == record['sha256']
    assert raw.file[i] == d.FileDescriptorProto.FromString(data)
a = d.FileDescriptorSet.FromString((root / 'trajectory.protoset').read_bytes())
b = d.FileDescriptorSet.FromString((root / 'roundtrip.protoset').read_bytes())
def normalized(message):
    value = json_format.MessageToDict(message)
    def clean(v):
        if isinstance(v, dict):
            return {k: (sorted((clean(t) for t in x), key=lambda t: t['name']) if k == 'nestedType' else clean(x)) for k, x in v.items() if not (k == 'options' and x == {})}
        if isinstance(v, list):
            return [clean(x) for x in v]
        return v
    return clean(value)
assert {f.name: normalized(f) for f in a.file} == {f.name: normalized(f) for f in b.file}, 'proto roundtrip differs beyond empty options and nested declaration ordering'
files = {f.name: f for f in a.file}; pool = descriptor_pool.DescriptorPool(); loaded = set()
def load(name):
    if name in loaded: return
    for dep in files[name].dependency: load(dep)
    pool.Add(files[name]); loaded.add(name)
for name in files: load(name)
cls = message_factory.GetMessageClass(pool.FindMessageTypeByName('gemini_coder.Trajectory'))
sample = cls(); sample.trajectory_id = 'synthetic-offline-example'; sample.cascade_id = 'synthetic-cascade'
decoded = cls.FromString(sample.SerializeToString())
assert decoded.trajectory_id == sample.trajectory_id
report = {'raw_records_hash_verified': len(raw.file), 'normalized_files_loaded': len(loaded), 'proto_roundtrip_equivalent_except_empty_options_and_nested_order': len(b.file), 'synthetic_offline_decode': json_format.MessageToDict(decoded), 'real_conversations_tested': 0}
print(json.dumps(report, indent=2))
