"""Generate the checked descriptor profile from the separately extracted records.
Input is a directory produced by the read-only handoff extractor. No network.
"""
from pathlib import Path
import sys,json,gzip
from google.protobuf import descriptor_pb2
root=Path(sys.argv[1]);out=Path(sys.argv[2])
manifest=json.loads((root/'manifest.json').read_text())
assert manifest['binary_sha256']=='0d0d3eba22daf29504dd290151c7ed9a4d33b0c6aa0acfc5da27bc3b01d2f029'
files={}
for p in root.glob('*.pb'):
 d=descriptor_pb2.FileDescriptorProto.FromString(p.read_bytes());files[d.name]=d
needed=set()
def visit(n):
 if n in needed:return
 needed.add(n)
 if n in files:
  for dep in files[n].dependency:visit(dep)
visit('third_party/gemini_coder/proto/trajectory.proto')
missing=needed-files.keys()
assert missing=={'net/proto2/contrib/validator/annotations.proto','net/proto2/proto/descriptor.proto','storage/datapol/annotations/proto/semantic_annotations.proto','third_party/boundary_proxy/proto_compare/annotation.proto','third_party/golang/protobuf/v2/src/google/protobuf/go_features.proto'}
def strip(m):
 if hasattr(m,'extension'):del m.extension[:]
 for n in getattr(m,'nested_type',[]):strip(n)
set=descriptor_pb2.FileDescriptorSet()
for name in sorted(needed-missing):
 d=set.file.add();d.CopyFrom(files[name]);d.DiscardUnknownFields();strip(d)
 for m in d.message_type:strip(m)
 deps=[n for n in d.dependency if n in files];del d.dependency[:];d.dependency.extend(deps)
 del d.public_dependency[:];del d.weak_dependency[:]
out.write_bytes(gzip.compress(set.SerializeToString(deterministic=True),mtime=0))
print(len(set.file),'files',out.stat().st_size,'compressed bytes')
