"""Add the bounded agy 1.4.6 checkpoint overlay to the historical options145 schema.

Both prior inputs and the new snapshot are authenticated before atomic output.
Requires google.protobuf; no binary execution or network access is needed.
"""

import argparse
import gzip
import hashlib
import io
import json
from pathlib import Path
import tempfile

from google.protobuf import descriptor_pb2
import generate_options145 as prior

GenerationError = prior.GenerationError
require, one = prior.require, prior.one
BINARY_SHA256 = "128780f5776959ba6a991bde1f0a8b699eee37a59ed30deb9926147fedfa56c4"
MANIFEST_SHA256 = "3c44a5d8a5e2bc951c1f45d6eebabef811f08bcf8ca2e7d551bef3a7c2f377e0"
COMMON_FILE = "third_party/jetski/codeium_common_pb/codeium_common.proto"
DEFAULT_BASE = prior.DEFAULT_BASE


def verified_source(extracted):
    manifest_bytes = (extracted / "manifest.json").read_bytes()
    require(hashlib.sha256(manifest_bytes).hexdigest() == MANIFEST_SHA256,
            "manifest hash does not match the verified 1.4.6 snapshot")
    manifest = json.loads(manifest_bytes)
    require(manifest["binary_sha256"] == BINARY_SHA256,
            "descriptor manifest does not identify the qualified binary")
    entries = manifest["files"]
    require({p.name for p in extracted.glob("*.pb")} ==
            {f"{i:04d}.pb" for i in range(len(entries))},
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
        file.DiscardUnknownFields()
        files[file.name] = file
    return files


def augment(base, files):
    """Copy six declarations without changing any prior declaration."""
    result = descriptor_pb2.FileDescriptorSet()
    result.CopyFrom(base)
    target, source = one(result.file, prior.CORTEX_FILE), files[prior.CORTEX_FILE]
    require((target.package, target.syntax) == (source.package, source.syntax),
            "source and target file semantics differ")
    # These existing dependencies must remain byte-identical to the snapshot.
    common = one(result.file, COMMON_FILE)
    for name in ("Media", "Blobref"):
        require(one(common.message_type, name) == one(files[COMMON_FILE].message_type, name),
                f"existing media declaration differs: {name}")
    message_type = descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE
    string_type = descriptor_pb2.FieldDescriptorProto.TYPE_STRING
    checkpoint = one(source.message_type, "CortexStepCheckpoint")
    field = prior.checked_field(checkpoint, "captured_user_requests", 30, message_type,
                                "capturedUserRequests", repeated=True,
                                type_name=".exa.cortex_pb.CapturedUserRequest")
    captured = one(source.message_type, "CapturedUserRequest")
    require(len(captured.field) == 2 and not captured.oneof_decl and not captured.nested_type
            and not captured.enum_type and not captured.extension,
            "unexpected captured request definition")
    prior.checked_field(captured, "text", 1, string_type, "text")
    prior.checked_field(captured, "media", 2, message_type, "media", repeated=True,
                        type_name=".exa.codeium_common_pb.Media")
    require(not any(m.name == captured.name for m in target.message_type),
            "target already contains an added message")
    prior.append_field(one(target.message_type, checkpoint.name), checkpoint, field)
    target.message_type.add().CopyFrom(captured)
    config = one(source.message_type, "CheckpointConfig")
    for name, number, kind, json_name in (
        ("context_window", 36, string_type, "contextWindow"),
        ("max_user_request_media_tokens", 37, descriptor_pb2.FieldDescriptorProto.TYPE_INT32,
         "maxUserRequestMediaTokens"),
    ):
        field = prior.checked_field(config, name, number, kind, json_name, optional=True)
        prior.append_field(one(target.message_type, config.name), config, field)
    # Native 1.4.6 startup configuration persists this map in planner metadata.
    discovery = one(source.message_type, "CustomizationDiscoveryConfig")
    target_discovery = one(target.message_type, discovery.name)
    enablement = prior.checked_field(
        discovery, "kind_enablement", 12, message_type, "kindEnablement", repeated=True,
        type_name=".exa.cortex_pb.CustomizationDiscoveryConfig.KindEnablementEntry")
    entry = one(discovery.nested_type, "KindEnablementEntry")
    require(entry.options.map_entry and len(entry.field) == 2 and not entry.oneof_decl,
            "unexpected kind enablement map entry definition")
    prior.checked_field(entry, "key", 1, string_type, "key")
    prior.checked_field(entry, "value", 2, descriptor_pb2.FieldDescriptorProto.TYPE_BOOL, "value")
    require(not any(m.name == entry.name for m in target_discovery.nested_type),
            "target already contains the kind enablement map entry")
    prior.append_field(target_discovery, discovery, enablement)
    target_discovery.nested_type.add().CopyFrom(entry)
    prior.load_pool(result)
    return result


def generate(options145, extracted, output, base_path=DEFAULT_BASE):
    require(output.resolve() != base_path.resolve(), "refusing to replace the pinned base")
    base, source145 = prior.verified_inputs(base_path, options145)
    result = augment(prior.augment(base, source145), verified_source(extracted))
    compressed = io.BytesIO()
    with gzip.GzipFile(fileobj=compressed, filename="", mode="wb", mtime=0) as stream:
        stream.write(result.SerializeToString(deterministic=True))
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
    parser.add_argument("options145", type=Path, help="verified 1.4.5 extracted directory")
    parser.add_argument("extracted", type=Path, help="verified 1.4.6 extracted directory")
    parser.add_argument("output", type=Path)
    parser.add_argument("--base", type=Path, default=DEFAULT_BASE)
    args = parser.parse_args()
    try:
        result, data = generate(args.options145, args.extracted, args.output, args.base)
    except (ValueError, OSError, KeyError) as error:
        parser.exit(1, f"refused: {error}\n")
    print(len(result.file), "files", len(data), "compressed bytes", hashlib.sha256(data).hexdigest())


if __name__ == "__main__":
    main()
