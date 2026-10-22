"""Add only the three qualified agy 1.4.5 configuration fields.

The 1.3.4 base remains intact, including historical PolicyGuardianConfig field
14. Declarations come from the verified 1.4.5 binary snapshot, not a handwritten
schema. This deliberately excludes its BASH enum and other intervening changes.
No network or binary execution is needed. Requires google.protobuf.
"""

import argparse
import gzip
import hashlib
import io
import json
from pathlib import Path
import tempfile

from google.protobuf import descriptor_pb2, descriptor_pool

BASE_SHA256 = "90bd669c32e044ef769763f3e4dc5b27fb187972cc9ff518189ff3933ed8fd92"
BINARY_SHA256 = "8af8e7a2a68f72fa7164e00bb59b8902c018fa43e528fc401ad7db8c2972583c"
MANIFEST_SHA256 = "5a3fc4d99ca4276ac654e2786a427ef80286f8d510476a7d7a6b3a53e47c12f1"
CORTEX_FILE = "third_party/jetski/cortex_pb/cortex.proto"
DEFAULT_BASE = Path(__file__).with_name("schema-base-agy134.pb.gz")


class GenerationError(ValueError):
    pass


def require(condition, reason):
    if not condition:
        raise GenerationError(reason)


def one(items, name):
    found = [item for item in items if item.name == name]
    require(len(found) == 1, f"expected exactly one declaration: {name}")
    return found[0]


def verified_inputs(base_path, extracted):
    base_bytes = base_path.read_bytes()
    require(hashlib.sha256(base_bytes).hexdigest() == BASE_SHA256,
            "base descriptor hash does not match the qualified 1.3.4 bundle")
    base = descriptor_pb2.FileDescriptorSet.FromString(gzip.decompress(base_bytes))
    manifest_bytes = (extracted / "manifest.json").read_bytes()
    require(hashlib.sha256(manifest_bytes).hexdigest() == MANIFEST_SHA256,
            "manifest hash does not match the verified 1.4.5 snapshot")
    manifest = json.loads(manifest_bytes)
    require(manifest["binary_sha256"] == BINARY_SHA256,
            "descriptor manifest does not identify the qualified binary")
    entries = manifest["files"]
    expected_paths = {f"{i:04d}.pb" for i in range(len(entries))}
    require({p.name for p in extracted.glob("*.pb")} == expected_paths,
            "extracted record inventory does not match the manifest")
    files = {}
    for i, entry in enumerate(entries):
        data = (extracted / f"{i:04d}.pb").read_bytes()
        require(len(data) == entry["length"] and
                hashlib.sha256(data).hexdigest() == entry["sha256"],
                f"descriptor bytes do not match manifest: {entry['name']}")
        file = descriptor_pb2.FileDescriptorProto.FromString(data)
        require(file.name == entry["name"] and file.name not in files,
                f"descriptor name does not match inventory: {entry['name']}")
        # Match the existing production generator's option normalization.
        file.DiscardUnknownFields()
        files[file.name] = file
    return base, files[CORTEX_FILE]


def checked_field(message, name, number, kind, json_name, *, optional=False,
                  repeated=False, type_name=""):
    field = one(message.field, name)
    want_label = (descriptor_pb2.FieldDescriptorProto.LABEL_REPEATED if repeated
                  else descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL)
    require((field.number, field.type, field.label, field.json_name,
             field.type_name, field.proto3_optional) ==
            (number, kind, want_label, json_name, type_name, optional),
            f"unexpected source field definition: {message.name}.{name}")
    require(field.HasField("oneof_index") == optional,
            f"unexpected source presence: {message.name}.{name}")
    if optional:
        require(0 <= field.oneof_index < len(message.oneof_decl),
                f"invalid source oneof: {message.name}.{name}")
        declaration = message.oneof_decl[field.oneof_index]
        require(declaration.name == "_" + name and
                sum(f.HasField("oneof_index") and
                    f.oneof_index == field.oneof_index for f in message.field) == 1,
                f"source field is not a standalone optional: {message.name}.{name}")
    return field


def append_field(target, source, field):
    require(not any(f.name == field.name or f.number == field.number for f in target.field),
            f"target field already exists: {target.name}.{field.name}")
    require(field.name not in target.reserved_name and
            not any(r.start <= field.number < r.end for r in target.reserved_range),
            f"target field is reserved: {target.name}.{field.name}")
    added = target.field.add()
    added.CopyFrom(field)
    if field.HasField("oneof_index"):
        declaration = source.oneof_decl[field.oneof_index]
        require(not any(o.name == declaration.name for o in target.oneof_decl),
                f"target oneof already exists: {declaration.name}")
        added.oneof_index = len(target.oneof_decl)
        target.oneof_decl.add().CopyFrom(declaration)


def augment(base, source):
    """Copy five authenticated declarations, preserving every old definition."""
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    target = one(result.file, CORTEX_FILE)
    require((target.package, target.syntax) == (source.package, source.syntax),
            "source and target file semantics differ")
    boolean = descriptor_pb2.FieldDescriptorProto.TYPE_BOOL
    message_type = descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE
    for message_name, field_name, number, json_name in (
        ("AntigravityBrowserToolConfig", "enable_browser_subagent_auto_invoke", 31,
         "enableBrowserSubagentAutoInvoke"),
        ("ViewFileToolConfig", "skip_plugin_logo", 21, "skipPluginLogo"),
    ):
        original = one(source.message_type, message_name)
        field = checked_field(original, field_name, number, boolean, json_name, optional=True)
        append_field(one(target.message_type, message_name), original, field)

    source_output = one(source.message_type, "ToolOutputConfig")
    target_output = one(target.message_type, "ToolOutputConfig")
    per_tool = checked_field(source_output, "per_tool", 2, message_type, "perTool",
                             repeated=True, type_name=".exa.cortex_pb.ToolOutputConfig.PerToolEntry")
    entry = one(source_output.nested_type, "PerToolEntry")
    require(entry.options.map_entry and len(entry.field) == 2 and not entry.oneof_decl,
            "unexpected per_tool map entry definition")
    checked_field(entry, "key", 1, descriptor_pb2.FieldDescriptorProto.TYPE_STRING, "key")
    checked_field(entry, "value", 2, message_type, "value",
                  type_name=".exa.cortex_pb.ToolOutputOverride")
    override = one(source.message_type, "ToolOutputOverride")
    require(len(override.field) == 1 and len(override.oneof_decl) == 1,
            "unexpected tool output override definition")
    checked_field(override, "max_output_bytes", 1,
                  descriptor_pb2.FieldDescriptorProto.TYPE_INT32, "maxOutputBytes", optional=True)
    require(not any(m.name == entry.name for m in target_output.nested_type) and
            not any(m.name == override.name for m in target.message_type),
            "target already contains an added message")
    append_field(target_output, source_output, per_tool)
    target_output.nested_type.add().CopyFrom(entry)
    target.message_type.add().CopyFrom(override)
    load_pool(result)
    return result


def load_pool(files):
    by_name = {file.name: file for file in files.file}
    require(len(by_name) == len(files.file), "duplicate descriptor file")
    pool = descriptor_pool.DescriptorPool()
    loaded, loading = set(), set()

    def load(name):
        if name in loaded:
            return
        require(name in by_name and name not in loading,
                f"missing or circular descriptor dependency: {name}")
        loading.add(name)
        for dependency in by_name[name].dependency:
            load(dependency)
        pool.Add(by_name[name])
        loaded.add(name)
        loading.remove(name)

    for name in by_name:
        load(name)
    return pool


def generate(extracted, output, base_path=DEFAULT_BASE):
    require(output.resolve() != base_path.resolve(), "refusing to replace the pinned base")
    base, source = verified_inputs(base_path, extracted)
    result = augment(base, source)
    compressed = io.BytesIO()
    with gzip.GzipFile(fileobj=compressed, filename="", mode="wb", mtime=0) as stream:
        stream.write(result.SerializeToString(deterministic=True))
    # All verification completes before opening an output file. Replacement is
    # atomic, so a refusal or write failure cannot leave a partial schema bundle.
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
    parser.add_argument("extracted", type=Path, help="verified 1.4.5 snapshot's extracted directory")
    parser.add_argument("output", type=Path)
    parser.add_argument("--base", type=Path, default=DEFAULT_BASE)
    args = parser.parse_args()
    try:
        result, data = generate(args.extracted, args.output, args.base)
    except (ValueError, OSError, KeyError) as error:
        parser.exit(1, f"refused: {error}\n")
    print(len(result.file), "files", len(data), "compressed bytes", hashlib.sha256(data).hexdigest())


if __name__ == "__main__":
    main()
