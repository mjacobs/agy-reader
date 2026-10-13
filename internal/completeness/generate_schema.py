"""Generate the checked descriptor profile from separately extracted records.

Input is a directory produced by the read-only extractor,
scripts/extract_descriptors.py. No network.
"""

import gzip
import hashlib
import io
import json
from pathlib import Path
import sys

from google.protobuf import descriptor_pb2, descriptor_pool

BINARY_SHA256 = "5b93565c5f04ea4f7b79d49d5c3237449bf6ce47b6779e8cf46c1fae97957c98"
ROOTS = (
    "third_party/gemini_coder/proto/trajectory.proto",
    # GenerateImage is carried in an Any, so trajectory.proto does not import it.
    "third_party/jetski/cortex/core/tools_pb/tools.proto",
)
# Annotation-only files: imports that declare options and extensions but no type
# the trajectory uses. They are dropped when the extractor finds them and may be
# absent from the import closure; any other missing import is an error.
ANNOTATION_ONLY = {
    "net/proto2/contrib/validator/annotations.proto",
    "net/proto2/proto/descriptor.proto",
    "storage/datapol/annotations/proto/semantic_annotations.proto",
    "third_party/boundary_proxy/proto_compare/annotation.proto",
    "third_party/golang/protobuf/v2/src/google/protobuf/go_features.proto",
}

root, out = map(Path, sys.argv[1:])
manifest = json.loads((root / "manifest.json").read_text())
if manifest["binary_sha256"] != BINARY_SHA256:
    sys.exit("descriptor manifest does not identify the qualified binary")
manifest_records = {(f["name"], f["sha256"]) for f in manifest["files"]}
files = {}
for path in sorted(root.glob("*.pb")):
    data = path.read_bytes()
    descriptor = descriptor_pb2.FileDescriptorProto.FromString(data)
    if (descriptor.name, hashlib.sha256(data).hexdigest()) not in manifest_records:
        sys.exit(f"descriptor does not match manifest: {descriptor.name}")
    if descriptor.name in files:
        sys.exit(f"duplicate extracted descriptor: {descriptor.name}")
    if descriptor.name in ANNOTATION_ONLY:
        continue
    files[descriptor.name] = descriptor

needed = set()


def visit(name):
    if name in needed:
        return
    needed.add(name)
    if name in files:
        for dependency in files[name].dependency:
            visit(dependency)


for name in ROOTS:
    visit(name)
missing = needed - files.keys()
if not missing <= ANNOTATION_ONLY:
    sys.exit(f"unexpected missing descriptors: {sorted(missing - ANNOTATION_ONLY)}")


def strip_extensions(message):
    if hasattr(message, "extension"):
        del message.extension[:]
    for nested in getattr(message, "nested_type", []):
        strip_extensions(nested)


descriptors = descriptor_pb2.FileDescriptorSet()
for name in sorted(needed - missing):
    descriptor = descriptors.file.add()
    descriptor.CopyFrom(files[name])
    descriptor.DiscardUnknownFields()
    strip_extensions(descriptor)
    for message in descriptor.message_type:
        strip_extensions(message)
    dependencies = [name for name in descriptor.dependency if name in files]
    del descriptor.dependency[:]
    descriptor.dependency.extend(dependencies)
    del descriptor.public_dependency[:]
    del descriptor.weak_dependency[:]

# Prove the bundle loads before writing it: a duplicate or dangling symbol
# would otherwise surface only when the reader's tests fail to open the schema.
pool = descriptor_pool.DescriptorPool()
loaded = set()


def load(descriptor):
    if descriptor.name in loaded:
        return
    for dependency in descriptor.dependency:
        load(by_name[dependency])
    pool.Add(descriptor)
    loaded.add(descriptor.name)


by_name = {d.name: d for d in descriptors.file}
for descriptor in descriptors.file:
    try:
        load(descriptor)
    except Exception as error:  # the pool reports the duplicate or missing symbol
        sys.exit(f"generated bundle does not load: {descriptor.name}: {error}")

# GzipFile keeps the header independent of the output name, clock, and platform.
compressed = io.BytesIO()
with gzip.GzipFile(fileobj=compressed, filename="", mode="wb", mtime=0) as stream:
    stream.write(descriptors.SerializeToString(deterministic=True))
out.write_bytes(compressed.getvalue())
print(len(descriptors.file), "files", out.stat().st_size, "compressed bytes")
