#!/usr/bin/env python3
"""Save, verify and compare versioned protobuf snapshots of an agy binary.

usage:
  agy_proto_snapshot.py snapshot --binary PATH --version V --version-evidence TEXT
                                 --root PROTO [--root PROTO ...] [options]
  agy_proto_snapshot.py verify SNAPSHOT_DIR [--binary PATH]
  agy_proto_snapshot.py compare OLD NEW [--json PATH]

`snapshot` reads an agy binary (never runs it), copies it to a private file,
extracts the embedded FileDescriptorProto records, checks each record against
the copied bytes, builds the import closure of the named roots, prints readable
.proto files, compiles them back with protoc and checks that they describe the
same schema. Only when every step passes does it publish
OUT_ROOT/agy-<version>-<sha256 prefix>/ with a snapshot.json whose status is
"verified". A failed run publishes nothing.

`verify` rechecks a published snapshot offline: checksums, record hashes, the
closure loading in a descriptor pool and the saved roundtrip. With --binary it
also rechecks every record against its byte range in that binary.

`compare` reports the differences between two descriptor closures (snapshot
directories or .protoset[.gz] files), split into wire, JSON-facing and
descriptor-only changes. It exits 0 when they match, 1 when they differ and
2 when it refuses: an input that cannot be read or is not a descriptor set, or
a --json path that already exists (checked before comparing).

Needs Python protobuf (google.protobuf) for every command; `snapshot` also
needs protoc (grpcio-tools by default) and the Go proto printer in
scripts/proto-printer. skills/agy-proto-snapshot/SKILL.md is the runbook.
"""
import argparse
import gzip
import hashlib
import importlib.util
import io
import json
import mmap
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
import zlib

try:
    import google.protobuf
    from google.protobuf import descriptor_pb2, descriptor_pool, json_format, text_format
    from google.protobuf.message import DecodeError
except ImportError:  # reported by require_protobuf() so tests can import this module
    google = descriptor_pb2 = descriptor_pool = json_format = text_format = None
    DecodeError = None

SCRIPTS = Path(__file__).resolve().parent
PRINTER_SOURCE = SCRIPTS / "proto-printer"
FORMAT = "agy-proto-snapshot/1"
CLOSURE = "closure.protoset"

# Imports that only declare options and extensions, never a type a decoder
# needs. The closure drops them; their absence from the binary is allowed.
# This is the set internal/completeness/generate_schema.py uses.
DEFAULT_ANNOTATION_ONLY = (
    "net/proto2/contrib/validator/annotations.proto",
    "net/proto2/proto/descriptor.proto",
    "storage/datapol/annotations/proto/semantic_annotations.proto",
    "third_party/boundary_proxy/proto_compare/annotation.proto",
    "third_party/golang/protobuf/v2/src/google/protobuf/go_features.proto",
)

NORMALIZATION = (
    "drop the annotation-only imports listed in closure.annotation_only",
    "discard unknown fields in every descriptor",
    "remove extension declarations at file and message level",
    "remove imports that are not in the closure; public and weak markers stay on the imports that remain",
    "remove option imports (Edition 2024 option_dependency), which only supply options",
)
ROUNDTRIP_IGNORES = ("presence of empty options messages", "declaration order of nested messages")

# Oldest release that handles each edition. Python protobuf N.M pairs with
# protoc M. Anything not listed here is refused until someone checks it.
EDITION_MINIMUMS = {
    "EDITION_2023": {"protoc": (27, 0), "python-protobuf": (5, 27)},
    "EDITION_2024": {"protoc": (32, 0), "python-protobuf": (6, 32)},
}
CLASSIC_SYNTAXES = {"proto2", "proto3"}

VERSION_PATTERN = re.compile(r"^[0-9A-Za-z][0-9A-Za-z.+_-]{0,63}$")
SHA_PATTERN = re.compile(r"^[0-9a-f]{64}$")


class SnapshotError(Exception):
    """A refusal: the run stops and publishes nothing."""


def require_protobuf():
    if descriptor_pb2 is None:
        raise SnapshotError("missing dependency: Python protobuf (google.protobuf); install protobuf>=6.32")


def load_extractor():
    spec = importlib.util.spec_from_file_location("extract_descriptors", SCRIPTS / "extract_descriptors.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def sha256_file(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def sha256_bytes(data):
    return hashlib.sha256(data).hexdigest()


def version_tuple(text):
    numbers = [int(n) for n in re.findall(r"\d+", text)[:3]]
    if not numbers:
        raise SnapshotError(f"cannot read a version number from {text!r}")
    return tuple(numbers)


def syntax_label(file):
    """proto2, proto3 or the edition name (EDITION_2024) of one FileDescriptorProto."""
    if file.syntax == "editions":
        return descriptor_pb2.Edition.Name(file.edition) if file.edition in descriptor_pb2.Edition.values() else f"EDITION_{file.edition}"
    return file.syntax or "proto2"


def check_toolchain(labels, versions):
    """Problems that stop these tools handling these syntaxes; empty when supported.

    versions maps "protoc" and "python-protobuf" to version tuples.
    """
    problems = []
    for label in sorted(labels):
        if label in CLASSIC_SYNTAXES:
            continue
        minimums = EDITION_MINIMUMS.get(label)
        if minimums is None:
            problems.append(f"{label} is not in the supported edition table; check the tools handle it, then add it to EDITION_MINIMUMS")
            continue
        for tool, minimum in sorted(minimums.items()):
            have = versions.get(tool)
            if have is None or tuple(have) < minimum:
                shown = ".".join(map(str, have)) if have else "missing"
                problems.append(f"{label} needs {tool} >= {'.'.join(map(str, minimum))}, found {shown}")
    return problems


# --- Capture ---------------------------------------------------------------


def stable_copy(source, dest, expected_sha256=None):
    """Copy source to dest and refuse when it changed while being read.

    Returns (sha256, size). The source is hashed before and after the copy; the
    copy is hashed while it is written. All three must agree, as must the
    source's size and modification time.
    """
    source, dest = Path(source), Path(dest)
    if not source.is_file():
        raise SnapshotError(f"binary is not a regular file: {source}")
    before = os.stat(source)
    first = sha256_file(source)
    digest = hashlib.sha256()
    with open(source, "rb") as reader, open(dest, "xb") as writer:
        for block in iter(lambda: reader.read(1 << 20), b""):
            digest.update(block)
            writer.write(block)
    after = os.stat(source)
    second = sha256_file(source)
    copied = digest.hexdigest()
    if not (first == copied == second and (before.st_size, before.st_mtime_ns) == (after.st_size, after.st_mtime_ns)):
        raise SnapshotError(f"binary changed while it was being copied: {source}")
    os.chmod(dest, 0o444)
    if expected_sha256 is not None and copied != expected_sha256:
        raise SnapshotError(f"binary hash mismatch: expected {expected_sha256}, got {copied}")
    return copied, after.st_size


def copy_from_archive(archive, archive_sha256, member, dest):
    """Copy one member of a published .tar.gz after checking the archive digest."""
    archive = Path(archive)
    before = sha256_file(archive)
    if before != archive_sha256:
        raise SnapshotError(f"archive hash mismatch: expected {archive_sha256}, got {before}")
    with tarfile.open(archive, "r:gz") as tar:
        try:
            info = tar.getmember(member)
        except KeyError:
            raise SnapshotError(f"archive has no member {member!r}") from None
        if not info.isreg():
            raise SnapshotError(f"archive member is not a regular file: {member!r}")
        with tar.extractfile(info) as reader, open(dest, "xb") as writer:
            shutil.copyfileobj(reader, writer, 1 << 20)
    if sha256_file(archive) != before:
        raise SnapshotError(f"archive changed while it was being read: {archive}")
    os.chmod(dest, 0o444)
    return sha256_file(dest), os.stat(dest).st_size


def verify_byte_ranges(binary, extracted_dir, manifest):
    """Check every saved record against its manifest hash and its bytes in binary."""
    with open(binary, "rb") as handle, mmap.mmap(handle.fileno(), 0, access=mmap.ACCESS_READ) as data:
        for index, entry in enumerate(manifest["files"]):
            record = (Path(extracted_dir) / f"{index:04d}.pb").read_bytes()
            if sha256_bytes(record) != entry["sha256"] or len(record) != entry["length"]:
                raise SnapshotError(f"saved record does not match its manifest entry: {entry['name']}")
            if data[entry["offset"]:entry["offset"] + entry["length"]] != record:
                raise SnapshotError(f"saved record does not match the binary bytes at offset {entry['offset']}: {entry['name']}")
    return len(manifest["files"])


def read_records(extracted_dir, manifest):
    """The saved records as (raw bytes, parsed FileDescriptorProto) pairs."""
    records = []
    for index, entry in enumerate(manifest["files"]):
        data = (Path(extracted_dir) / f"{index:04d}.pb").read_bytes()
        if sha256_bytes(data) != entry["sha256"]:
            raise SnapshotError(f"saved record does not match its manifest hash: {entry['name']}")
        file = descriptor_pb2.FileDescriptorProto.FromString(data)
        if file.name != entry["name"]:
            raise SnapshotError(f"saved record {index:04d} is named {file.name!r}, manifest says {entry['name']!r}")
        records.append((data, file))
    return records


def encode_varint(value):
    out = bytearray()
    while True:
        byte = value & 0x7F
        value >>= 7
        if value:
            out.append(byte | 0x80)
        else:
            out.append(byte)
            return bytes(out)


def raw_set(blobs):
    """A FileDescriptorSet holding these exact record bytes.

    Built on the wire rather than by reserializing parsed records, so the
    inventory keeps the original bytes whatever the protobuf runtime knows.
    """
    return b"".join(b"\x0a" + encode_varint(len(blob)) + blob for blob in blobs)


def raw_set_entries(data):
    """The record bytes inside a FileDescriptorSet written by raw_set."""
    entries, pos = [], 0
    while pos < len(data):
        if data[pos] != 0x0A:
            raise SnapshotError(f"unexpected tag {data[pos]:#x} at offset {pos} of a descriptor set")
        length, pos = varint(data, pos + 1)
        if length is None or pos + length > len(data):
            raise SnapshotError("truncated descriptor set")
        entries.append(bytes(data[pos:pos + length]))
        pos += length
    return entries


def varint(data, pos):
    value = shift = 0
    while pos < len(data) and shift <= 63:
        byte = data[pos]
        pos += 1
        value |= (byte & 0x7F) << shift
        if not byte & 0x80:
            return value, pos
        shift += 7
    return None, pos


# --- Closure -----------------------------------------------------------------


def strip_extensions(message):
    if hasattr(message, "extension"):
        del message.extension[:]
    for nested in getattr(message, "nested_type", []):
        strip_extensions(nested)


def build_closure(records, roots, annotation_only):
    """The normalized import closure of roots, as (FileDescriptorSet, details).

    Follows internal/completeness/generate_schema.py, so the same records
    and roots give byte-identical output, except that public and weak import
    markers are kept (re-indexed) instead of dropped, and option imports are
    removed. No agy trajectory closure so far has either.
    """
    by_name = {}
    for file in records:
        if file.name in by_name:
            raise SnapshotError(f"duplicate extracted descriptor: {file.name}")
        by_name[file.name] = file
    annotation_only = set(annotation_only)
    files = {name: file for name, file in by_name.items() if name not in annotation_only}
    absent_roots = [root for root in roots if root not in files]
    if absent_roots:
        raise SnapshotError(f"root not found in this binary: {absent_roots}; choose roots that exist rather than adding them")
    needed = set()

    def visit(name):
        if name in needed:
            return
        needed.add(name)
        if name in files:
            for dependency in files[name].dependency:
                visit(dependency)

    for root in roots:
        visit(root)
    missing = needed - files.keys()
    unexpected = sorted(missing - annotation_only)
    if unexpected:
        raise SnapshotError(f"unexpected missing imports in the closure: {unexpected}")
    closure = descriptor_pb2.FileDescriptorSet()
    for name in sorted(needed - missing):
        descriptor = closure.file.add()
        descriptor.CopyFrom(files[name])
        descriptor.DiscardUnknownFields()
        strip_extensions(descriptor)
        for message in descriptor.message_type:
            strip_extensions(message)
        # A public import re-exports its types, so dropping the marker would
        # leave references through it unresolved; keep markers on the imports
        # that remain, at their new positions.
        count = len(descriptor.dependency)
        bad = [i for i in list(descriptor.public_dependency) + list(descriptor.weak_dependency) if not 0 <= i < count]
        if bad:
            raise SnapshotError(f"{name}: public or weak import marker {bad} points past its {count} imports")
        public = {descriptor.dependency[i] for i in descriptor.public_dependency}
        weak = {descriptor.dependency[i] for i in descriptor.weak_dependency}
        dependencies = [dependency for dependency in descriptor.dependency if dependency in files]
        del descriptor.dependency[:]
        descriptor.dependency.extend(dependencies)
        del descriptor.public_dependency[:]
        descriptor.public_dependency.extend(i for i, dependency in enumerate(dependencies) if dependency in public)
        del descriptor.weak_dependency[:]
        descriptor.weak_dependency.extend(i for i, dependency in enumerate(dependencies) if dependency in weak)
        if hasattr(descriptor, "option_dependency"):
            del descriptor.option_dependency[:]
    details = {
        "roots": list(roots),
        "annotation_only": sorted(annotation_only),
        # Imported by the closure and dropped; split by whether the binary has them.
        "annotation_only_stripped": sorted(name for name in missing if name in by_name),
        "annotation_only_absent_from_binary": sorted(name for name in missing if name not in by_name),
        "files": len(closure.file),
    }
    return closure, details


def load_pool(file_set):
    """Load every file into a fresh DescriptorPool, dependencies first."""
    by_name = {file.name: file for file in file_set.file}
    pool = descriptor_pool.DescriptorPool()
    loaded = set()

    def load(name, chain=()):
        if name in loaded:
            return
        if name in chain:
            raise SnapshotError(f"import cycle through {name}")
        if name not in by_name:
            raise SnapshotError(f"descriptor set is missing import {name!r}")
        for dependency in by_name[name].dependency:
            load(dependency, chain + (name,))
        try:
            pool.Add(by_name[name])
        except Exception as error:  # the pool names the duplicate or unsupported construct
            raise SnapshotError(f"descriptor does not load: {name}: {error}") from None
        loaded.add(name)

    for name in sorted(by_name):
        load(name)
    return pool


def deterministic_gzip(data):
    # An empty name and mtime 0 keep the header free of the output name and
    # clock. The compressed bytes still depend on the zlib build, so checks
    # compare the decompressed content, never the .gz bytes.
    buffer = io.BytesIO()
    with gzip.GzipFile(fileobj=buffer, filename="", mode="wb", mtime=0) as stream:
        stream.write(data)
    return buffer.getvalue()


def roundtrip_normalized(file):
    """A comparable form of one descriptor that ignores ROUNDTRIP_IGNORES only."""

    def clean(value):
        if isinstance(value, dict):
            return {
                key: (sorted((clean(t) for t in item), key=lambda t: t["name"]) if key == "nestedType" else clean(item))
                for key, item in value.items()
                if not (key == "options" and item == {})
            }
        if isinstance(value, list):
            return [clean(item) for item in value]
        return value

    return clean(json_format.MessageToDict(file))


def roundtrip_differences(expected, compiled):
    want = {file.name: roundtrip_normalized(file) for file in expected.file}
    got = {file.name: roundtrip_normalized(file) for file in compiled.file}
    problems = []
    for name in sorted(want.keys() | got.keys()):
        if name not in got:
            problems.append(f"{name}: not produced by protoc")
        elif name not in want:
            problems.append(f"{name}: produced by protoc but not in the closure")
        elif want[name] != got[name]:
            problems.append(f"{name}: compiled descriptor differs")
    return problems


# --- Tools -------------------------------------------------------------------


def run_tool(command, what, cwd=None, env=None):
    try:
        result = subprocess.run(command, cwd=cwd, env=env, capture_output=True, text=True)
    except FileNotFoundError:
        raise SnapshotError(f"missing dependency: {command[0]} ({what})") from None
    if result.returncode != 0:
        tail = (result.stderr or result.stdout).strip().splitlines()[-5:]
        raise SnapshotError(f"{what} failed (exit {result.returncode}): " + " | ".join(tail))
    return result.stdout


def protoc_command(protoc):
    if protoc:
        return [protoc]
    if importlib.util.find_spec("grpc_tools") is None:
        raise SnapshotError("missing dependency: protoc; install grpcio-tools or pass --protoc")
    return [sys.executable, "-m", "grpc_tools.protoc"]


def probe_toolchain(protoc, printer):
    """Versions of the tools a snapshot needs; refuses when one is missing."""
    require_protobuf()
    command = protoc_command(protoc)
    protoc_version = run_tool(command + ["--version"], "protoc --version").strip()
    tools = {
        "python": platform.python_version(),
        "python-protobuf": google.protobuf.__version__,
        "protoc": protoc_version.removeprefix("libprotoc ").strip(),
        "protoc_source": "--protoc" if protoc else "grpcio-tools",
        "printer": printer_details(printer),
    }
    if not printer:
        tools["go"] = run_tool(["go", "env", "GOVERSION"], "go env GOVERSION (needed to build the proto printer; or pass --printer)").strip()
    return command, tools


def printer_details(printer):
    if printer:
        path = Path(printer)
        if not path.is_file():
            raise SnapshotError(f"missing dependency: proto printer {printer}")
        return {"source": "prebuilt (--printer)", "sha256": sha256_file(path)}
    requires = re.findall(r"^\s*(\S+) (v\S+)$", (PRINTER_SOURCE / "go.mod").read_text(), re.M)
    return {
        "source": "scripts/proto-printer",
        "main_go_sha256": sha256_file(PRINTER_SOURCE / "main.go"),
        "go_sum_sha256": sha256_file(PRINTER_SOURCE / "go.sum"),
        "modules": {name: version for name, version in requires if "/" in name},
    }


def build_printer(directory):
    out = Path(directory) / "proto-printer"
    run_tool(["go", "build", "-buildvcs=false", "-o", str(out), "."], "building scripts/proto-printer", cwd=PRINTER_SOURCE)
    return out


def tool_revision():
    """The agy-reader commit these tools ran from, and whether the tree was dirty."""
    try:
        commit = subprocess.run(["git", "-C", str(SCRIPTS), "rev-parse", "HEAD"], capture_output=True, text=True, check=True).stdout.strip()
        dirty = subprocess.run(["git", "-C", str(SCRIPTS), "status", "--porcelain", "--", str(SCRIPTS)], capture_output=True, text=True, check=True).stdout.strip()
    except (OSError, subprocess.CalledProcessError):
        return {"commit": None, "scripts_dirty": None}
    return {"commit": commit, "scripts_dirty": bool(dirty)}


# --- Snapshot ----------------------------------------------------------------


def print_and_compile(printer, command, closure, staging, relative):
    """Print closure.protoset to proto/, compile it back, and list differences."""
    proto, roundtrip = staging / "proto", staging / "roundtrip.protoset"
    shutil.rmtree(proto, ignore_errors=True)
    roundtrip.unlink(missing_ok=True)
    run_tool([str(printer)] + (["-relative"] if relative else []) + [str(staging / CLOSURE), str(proto)], "printing .proto files")
    printed = sorted(p.relative_to(proto).as_posix() for p in proto.rglob("*.proto"))
    if printed != sorted(file.name for file in closure.file):
        raise SnapshotError("printed .proto files do not match the closure file list")
    run_tool(command + ["-I", str(proto), "--include_imports", f"--descriptor_set_out={roundtrip}"] + printed,
             "compiling the printed .proto files")
    compiled = descriptor_pb2.FileDescriptorSet.FromString(roundtrip.read_bytes())
    return compiled, roundtrip_differences(closure, compiled)


def write_checksums(directory):
    directory = Path(directory)
    lines = []
    for path in sorted(p for p in directory.rglob("*") if p.is_file() and p.name != "SHA256SUMS"):
        lines.append(f"{sha256_file(path)}  {path.relative_to(directory).as_posix()}\n")
    (directory / "SHA256SUMS").write_text("".join(lines))
    return len(lines)


def snapshot(options, toolchain=None):
    """Create one verified snapshot directory and return its path.

    toolchain lets tests supply (protoc command, tool versions) instead of
    probing the machine.
    """
    require_protobuf()
    if not VERSION_PATTERN.match(options.version):
        raise SnapshotError(f"version must be a plain release label such as 1.3.6, not {options.version!r}")
    if not options.roots:
        raise SnapshotError("name at least one --root; roots are never guessed")
    expected = options.expected_sha256.lower() if options.expected_sha256 else None
    if expected is not None and not SHA_PATTERN.match(expected):
        raise SnapshotError("--expected-sha256 must be 64 hexadecimal characters")
    if bool(options.archive) != bool(options.archive_sha256) or bool(options.archive) != bool(options.archive_member):
        raise SnapshotError("--archive, --archive-sha256 and --archive-member go together")
    if options.archive and options.binary:
        raise SnapshotError("give --binary or --archive, not both")
    if not options.archive and not options.binary:
        raise SnapshotError("give --binary, or --archive with its published digest")
    out_root = Path(options.out_root)
    if not out_root.is_dir():
        raise SnapshotError(f"output root does not exist: {out_root}")

    def destination(sha):
        return out_root / f"agy-{options.version}-{sha[:12]}"

    if expected is not None and destination(expected).exists():
        raise SnapshotError(f"refusing to overwrite existing snapshot: {destination(expected)}")
    command, tools = toolchain or probe_toolchain(options.protoc, options.printer)
    tools = dict(tools)

    workspace = Path(tempfile.mkdtemp(prefix="agy-proto-snapshot-"))
    staging = Path(tempfile.mkdtemp(prefix=".incomplete-", dir=out_root))
    try:
        copy = workspace / "agy"
        provenance = {"source_url": options.source_url}
        if options.archive:
            archive_sha = options.archive_sha256.lower()
            binary_sha, size = copy_from_archive(options.archive, archive_sha, options.archive_member, copy)
            provenance["archive"] = {"name": Path(options.archive).name, "sha256": archive_sha, "published_digest_verified": True, "member": options.archive_member}
            if expected is not None and binary_sha != expected:
                raise SnapshotError(f"binary hash mismatch: expected {expected}, got {binary_sha}")
            source_name = options.archive_member
        else:
            binary_sha, size = stable_copy(options.binary, copy, expected)
            source_name = Path(options.binary).name
        dest = destination(binary_sha)
        if dest.exists():
            raise SnapshotError(f"refusing to overwrite existing snapshot: {dest}")

        extractor = load_extractor()
        # The extractor stops a record at the first field it does not know, so
        # a field the runtime knows but the extractor does not would silently
        # truncate every record that uses it.
        unknown = sorted(f.number for f in descriptor_pb2.FileDescriptorProto.DESCRIPTOR.fields if f.number not in extractor.ALLOWED)
        if unknown:
            raise SnapshotError(f"extract_descriptors.py does not accept FileDescriptorProto field(s) {unknown}; records using them would be truncated")
        manifest = extractor.extract(copy, staging / "extracted")
        if manifest["binary_sha256"] != binary_sha:
            raise SnapshotError("extractor read different bytes from the private copy")
        if not manifest["files"]:
            raise SnapshotError("no descriptors found; this does not look like an agy binary")
        verified = verify_byte_ranges(copy, staging / "extracted", manifest)

        pairs = read_records(staging / "extracted", manifest)
        records = [file for _, file in pairs]
        (staging / "all-extracted.protoset").write_bytes(raw_set(blob for blob, _ in pairs))
        record_names = {file.name for file in records}
        inventory_labels = {}
        for file in records:
            label = syntax_label(file)
            inventory_labels[label] = inventory_labels.get(label, 0) + 1

        annotation_only = options.annotation_only if options.annotation_only is not None else DEFAULT_ANNOTATION_ONLY
        closure, closure_details = build_closure(records, options.roots, annotation_only)
        closure_labels = {}
        for file in closure.file:
            label = syntax_label(file)
            closure_labels[label] = closure_labels.get(label, 0) + 1
        versions = {"protoc": version_tuple(tools["protoc"]), "python-protobuf": version_tuple(tools["python-protobuf"])}
        problems = check_toolchain(closure_labels, versions)
        if problems:
            raise SnapshotError("unsupported toolchain or edition: " + "; ".join(problems))
        load_pool(closure)
        closure_bytes = closure.SerializeToString(deterministic=True)
        (staging / CLOSURE).write_bytes(closure_bytes)
        (staging / (CLOSURE + ".gz")).write_bytes(deterministic_gzip(closure_bytes))

        # A path, never a bare name: subprocess would search PATH for "printer".
        printer = Path(options.printer).resolve() if options.printer else build_printer(workspace)
        # Short type names read best but a nested message can capture one meant
        # for an outer message, so the result may not compile or may compile to
        # something else. Either way, print again fully qualified; refuse only
        # if that fails too.
        type_names = "relative"
        try:
            compiled, differences = print_and_compile(printer, command, closure, staging, relative=True)
        except SnapshotError:
            differences = ["relative names did not compile"]
        if differences:
            type_names = "fully-qualified"
            compiled, differences = print_and_compile(printer, command, closure, staging, relative=False)
        if differences:
            raise SnapshotError("roundtrip differs beyond the documented normalization: " + "; ".join(differences[:5]))

        record = {
            "format": FORMAT,
            "status": "verified",
            "created_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "agy": {"version": options.version, "version_evidence": options.version_evidence},
            "binary": {
                "sha256": binary_sha,
                "size": size,
                "source_name": source_name,
                "expected_sha256_verified": expected is not None,
                "capture": ("member of an archive whose published digest matched; the archive was unchanged while read"
                            if options.archive else "private copy; the source hash, size and mtime were unchanged across the copy"),
                "executed": False,
            },
            "provenance": provenance,
            "extraction": {
                "revision": tool_revision(),
                "extractor_sha256": sha256_file(SCRIPTS / "extract_descriptors.py"),
                "snapshot_tool_sha256": sha256_file(Path(__file__).resolve()),
            },
            "toolchain": tools,
            "inventory": {
                "records": len(records),
                "records_verified_against_binary": verified,
                "syntax": dict(sorted(inventory_labels.items())),
                "unresolved_imports": sorted({dep for file in records for dep in file.dependency if dep not in record_names}),
                "note": "all-extracted.protoset is every record found; it is not claimed to be a loadable closure",
            },
            "closure": dict(closure_details, syntax=dict(sorted(closure_labels.items())), loaded=len(closure.file), normalization=list(NORMALIZATION)),
            "roundtrip": {"compiled_files": len(compiled.file), "equivalent_ignoring": list(ROUNDTRIP_IGNORES), "readable_type_names": type_names},
            "scope": "Schema definitions only. This does not update COMPATIBILITY.md, the reader's schema or verification profile, or qualify any reader and daemon pairing.",
        }
        (staging / "snapshot.json").write_text(json.dumps(record, indent=1, sort_keys=False) + "\n")
        write_checksums(staging)
        if dest.exists():
            raise SnapshotError(f"refusing to overwrite existing snapshot: {dest}")
        os.rename(staging, dest)
        return dest
    finally:
        shutil.rmtree(workspace, ignore_errors=True)
        if staging.exists():
            shutil.rmtree(staging, ignore_errors=True)


# --- Verify ------------------------------------------------------------------


def verify(directory, binary=None):
    """Recheck a published snapshot offline and return a summary dict."""
    require_protobuf()
    directory = Path(directory)
    record_path = directory / "snapshot.json"
    if not record_path.is_file():
        raise SnapshotError(f"{directory} has no snapshot.json; verify works on snapshots made by this tool")
    record = json.loads(record_path.read_text())
    if record.get("format") != FORMAT or record.get("status") != "verified":
        raise SnapshotError("snapshot.json is not a verified snapshot record")
    sums = (directory / "SHA256SUMS").read_text().splitlines()
    listed = set()
    for line in sums:
        digest, name = line.split("  ", 1)
        listed.add(name)
        if sha256_file(directory / name) != digest:
            raise SnapshotError(f"checksum mismatch: {name}")
    present = {p.relative_to(directory).as_posix() for p in directory.rglob("*") if p.is_file() and p.name != "SHA256SUMS"}
    if present != listed:
        raise SnapshotError(f"files not covered by SHA256SUMS: {sorted(present ^ listed)}")
    manifest = json.loads((directory / "extracted" / "manifest.json").read_text())
    if manifest["binary_sha256"] != record["binary"]["sha256"]:
        raise SnapshotError("extracted manifest names a different binary than snapshot.json")
    pairs = read_records(directory / "extracted", manifest)
    records = [file for _, file in pairs]
    if raw_set_entries((directory / "all-extracted.protoset").read_bytes()) != [blob for blob, _ in pairs]:
        raise SnapshotError("all-extracted.protoset does not hold the saved record bytes")
    closure = descriptor_pb2.FileDescriptorSet.FromString((directory / CLOSURE).read_bytes())
    if gzip.decompress((directory / (CLOSURE + ".gz")).read_bytes()) != (directory / CLOSURE).read_bytes():
        raise SnapshotError("closure.protoset.gz does not match closure.protoset")
    rebuilt, _ = build_closure(records, record["closure"]["roots"], record["closure"]["annotation_only"])
    if rebuilt.SerializeToString(deterministic=True) != (directory / CLOSURE).read_bytes():
        raise SnapshotError("closure.protoset does not match a rebuild from the saved records")
    load_pool(closure)
    compiled = descriptor_pb2.FileDescriptorSet.FromString((directory / "roundtrip.protoset").read_bytes())
    differences = roundtrip_differences(closure, compiled)
    if differences:
        raise SnapshotError("saved roundtrip differs: " + "; ".join(differences[:5]))
    summary = {"snapshot": directory.name, "records": len(records), "closure_files": len(closure.file), "roundtrip_files": len(compiled.file), "byte_ranges_rechecked": 0}
    if binary is not None:
        if sha256_file(binary) != manifest["binary_sha256"]:
            raise SnapshotError("binary hash mismatch: --binary is not the binary this snapshot was taken from")
        summary["byte_ranges_rechecked"] = verify_byte_ranges(binary, directory / "extracted", manifest)
    return summary


# --- Compare -----------------------------------------------------------------

CATEGORIES = ("wire", "json", "descriptor")
CATEGORY_TITLES = {
    "wire": "Wire changes (how bytes are read or written, including RPC methods)",
    "json": "JSON-facing changes (names and absent-field defaults; same bytes)",
    "descriptor": "Descriptor-only changes (files, imports, editions, options, reservations)",
}


def load_set(path):
    """A FileDescriptorSet from a snapshot directory or a .protoset[.gz] file."""
    require_protobuf()
    path = Path(path)
    if path.is_dir():
        if not (path / "snapshot.json").is_file():
            candidates = sorted(p.name for p in path.glob("*.protoset*"))
            raise SnapshotError(f"{path} is not a snapshot made by this tool; name the descriptor set file explicitly (found: {candidates})")
        path = path / CLOSURE
    try:
        data = path.read_bytes()
    except OSError as error:
        raise SnapshotError(f"cannot read {path}: {error.strerror or error}") from None
    try:
        if data[:2] == b"\x1f\x8b":
            data = gzip.decompress(data)
        descriptors = descriptor_pb2.FileDescriptorSet.FromString(data)
    except (OSError, EOFError, zlib.error, DecodeError) as error:
        raise SnapshotError(f"{path} is not a descriptor set: {error}") from None
    if not descriptors.file:
        raise SnapshotError(f"{path} is not a descriptor set: it holds no descriptors")
    return descriptors


def options_text(options):
    """Options as text, plus a digest of any the runtime cannot name.

    Custom options are extensions; without their definitions they parse as
    unknown fields, which text output omits, so their bytes are hashed.
    """
    text = text_format.MessageToString(options, as_one_line=True)
    raw = options.SerializeToString(deterministic=True)
    known = type(options)()
    known.CopyFrom(options)
    known.DiscardUnknownFields()
    if raw != known.SerializeToString(deterministic=True):
        text += f" [unrecognized options sha256:{sha256_bytes(raw)[:16]}]"
    return text


def schema_model(file_set):
    """Effective schema facts, keyed for comparison."""
    pool = load_pool(file_set)
    model = {"files": {}, "messages": {}, "enums": {}, "services": {}, "extensions": {}}
    type_names = descriptor_pb2.FieldDescriptorProto.Type

    def visibility(proto):
        """Edition 2024 export/local marker, or None where the runtime lacks it."""
        if not hasattr(proto, "visibility") or not proto.HasField("visibility"):
            return None
        return descriptor_pb2.SymbolVisibility.Name(proto.visibility)

    def enum_entry(proto, full_name, file_name):
        found = pool.FindEnumTypeByName(full_name)
        values, value_options = {}, {}
        for value in proto.value:
            values.setdefault(value.number, []).append(value.name)
            value_options.setdefault(value.number, []).append(options_text(value.options))
        model["enums"][full_name] = {
            "file": file_name,
            "visibility": visibility(proto),
            "values": values,
            "value_options": value_options,
            "closed": bool(getattr(found, "is_closed", False)),
            "reserved_ranges": sorted((r.start, r.end) for r in proto.reserved_range),
            "reserved_names": sorted(proto.reserved_name),
            "options": options_text(proto.options),
        }

    def field_facts(field, field_proto):
        """One field or extension, as the pool resolves it for this edition."""
        repeated = field.is_repeated if hasattr(field, "is_repeated") else field.label == field.LABEL_REPEATED
        required = field.is_required if hasattr(field, "is_required") else field.label == field.LABEL_REQUIRED
        if repeated:
            cardinality = "map" if field.message_type is not None and field.message_type.GetOptions().map_entry else "repeated"
        else:
            cardinality = "required" if required else "singular"
        # A real oneof (not the synthetic one behind proto3 optional): its
        # members, by number, decide mutual exclusion on the wire; its name
        # is only descriptor text.
        oneof = members = None
        if field.containing_oneof is not None and not field_proto.proto3_optional:
            oneof = field.containing_oneof.name
            members = sorted(f.number for f in field.containing_oneof.fields)
        return {
            "name": field_proto.name,
            "json_name": field.json_name,
            # The effective type: an Editions message field with DELIMITED
            # encoding is a group on the wire although declared as a message.
            "type": type_names.Name(field.type),
            # The resolved target: a relative type_name can name a nested
            # message where the absolute one names an outer message.
            "type_name": (field.message_type or field.enum_type).full_name if (field.message_type or field.enum_type) else "",
            "cardinality": cardinality,
            "utf8_validation": utf8_validation(field),
            "presence": bool(field.has_presence),
            "packed": bool(field.is_packed) if repeated else None,
            "oneof": oneof,
            "oneof_members": members,
            # The value a reader sees when the field is absent. For a proto2 or
            # closed enum with no explicit default it is the first declared
            # value, so reordering the enum can change it.
            "default": None if repeated or field.message_type is not None else repr(field.default_value),
            "explicit_default": field_proto.default_value if field_proto.HasField("default_value") else None,
            "options": options_text(field_proto.options),
        }

    def utf8_validation(field):
        """VERIFY or NONE for a string field: whether invalid UTF-8 fails to decode."""
        if field.type != field.TYPE_STRING:
            return None
        try:  # resolved features; private but present in upb and pure Python
            features = field._GetFeatures()
        except AttributeError:
            return None
        return descriptor_pb2.FeatureSet.Utf8Validation.Name(features.utf8_validation)

    def extension_entries(protos, scope, file_name):
        for field_proto in protos:
            field = pool.FindExtensionByName(f"{scope}{field_proto.name}")
            facts = field_facts(field, field_proto)
            facts["scope"] = scope.rstrip(".")
            facts["file"] = file_name
            model["extensions"][f"{field.containing_type.full_name} extension {field.number}"] = facts

    def message_entry(proto, full_name, file_name):
        found = pool.FindMessageTypeByName(full_name)
        fields = {field_proto.number: field_facts(found.fields_by_number[field_proto.number], field_proto) for field_proto in proto.field}
        extension_entries(proto.extension, full_name + ".", file_name)
        model["messages"][full_name] = {
            "file": file_name,
            "visibility": visibility(proto),
            "fields": fields,
            "reserved_ranges": sorted((r.start, r.end) for r in proto.reserved_range),
            "reserved_names": sorted(proto.reserved_name),
            "extension_ranges": sorted((r.start, r.end, options_text(r.options)) for r in proto.extension_range),
            "message_set_wire_format": proto.options.message_set_wire_format,
            "oneof_options": {oneof.name: options_text(oneof.options) for oneof in proto.oneof_decl},
            "options": options_text(proto.options),
        }
        for nested in proto.nested_type:
            message_entry(nested, f"{full_name}.{nested.name}", file_name)
        for nested in proto.enum_type:
            enum_entry(nested, f"{full_name}.{nested.name}", file_name)

    for file in file_set.file:
        prefix = f"{file.package}." if file.package else ""
        model["files"][file.name] = {
            "package": file.package,
            "syntax": syntax_label(file),
            "imports": sorted(file.dependency),
            "public_imports": sorted(file.dependency[i] for i in file.public_dependency),
            "weak_imports": sorted(file.dependency[i] for i in file.weak_dependency),
            "option_imports": sorted(getattr(file, "option_dependency", [])),
            "options": options_text(file.options),
        }
        for message in file.message_type:
            message_entry(message, prefix + message.name, file.name)
        for enum in file.enum_type:
            enum_entry(enum, prefix + enum.name, file.name)
        extension_entries(file.extension, prefix, file.name)
        for service in file.service:
            # Request and response types as the pool resolves them, as for fields.
            resolved = pool.FindServiceByName(prefix + service.name).methods_by_name
            model["services"][prefix + service.name] = {
                "file": file.name,
                "methods": {
                    method.name: {
                        "input": resolved[method.name].input_type.full_name,
                        "output": resolved[method.name].output_type.full_name,
                        "client_streaming": method.client_streaming,
                        "server_streaming": method.server_streaming,
                        "options": options_text(method.options),
                    }
                    for method in service.method
                },
                "options": options_text(service.options),
            }
    return model


WIRE_ENCODING = {
    "TYPE_INT32": "varint", "TYPE_INT64": "varint", "TYPE_UINT32": "varint", "TYPE_UINT64": "varint",
    "TYPE_BOOL": "varint", "TYPE_ENUM": "varint", "TYPE_SINT32": "zigzag varint", "TYPE_SINT64": "zigzag varint",
    "TYPE_FIXED32": "32-bit", "TYPE_SFIXED32": "32-bit", "TYPE_FLOAT": "32-bit",
    "TYPE_FIXED64": "64-bit", "TYPE_SFIXED64": "64-bit", "TYPE_DOUBLE": "64-bit",
    "TYPE_STRING": "length-delimited", "TYPE_BYTES": "length-delimited", "TYPE_MESSAGE": "length-delimited",
    "TYPE_GROUP": "group",
}


def type_summary(field):
    """The declared type and its wire encoding, so a reader can judge a type change."""
    return f"{field['type_name'] or field['type']} ({WIRE_ENCODING.get(field['type'], 'unknown')})"


def field_summary(field):
    kind = field["type"].removeprefix("TYPE_").lower()
    if field["type_name"]:
        kind = field["type_name"]
    return f"{field['cardinality']} {kind} {field['name']}"


def oneof_summary(name, members):
    if name is None:
        return "not in a oneof"
    return f"oneof {name} with fields {members}"


def method_summary(method):
    def side(streaming, name):
        return f"stream {name}" if streaming else name
    return f"({side(method['client_streaming'], method['input'])}) returns ({side(method['server_streaming'], method['output'])})"


def diff_models(old, new):
    changes = []

    def add(category, kind, subject, old_value=None, new_value=None):
        changes.append({"category": category, "kind": kind, "subject": subject, "old": old_value, "new": new_value})

    def type_placement(name, a, b, noun):
        # Which file declares a type, and whether other files may use it,
        # decide which imports reach it; the bytes do not change.
        if a["file"] != b["file"]:
            add("descriptor", f"{noun}-file", name, a["file"], b["file"])
        if a["visibility"] != b["visibility"]:
            add("descriptor", f"{noun}-visibility", name, a["visibility"], b["visibility"])

    def compare_fields(subject, fa, fb, common, noun="field"):
        """common: field numbers on both sides, for oneof membership."""
        if fa is None:
            add("wire", f"{noun}-added", subject, None, field_summary(fb))
            return
        if fb is None:
            add("wire", f"{noun}-removed", subject, field_summary(fa), None)
            return
        if (fa["type"], fa["type_name"]) != (fb["type"], fb["type_name"]):
            add("wire", "field-type", subject, type_summary(fa), type_summary(fb))
        for key, category, kind in (("cardinality", "wire", "field-cardinality"), ("presence", "wire", "field-presence"),
                                    ("packed", "wire", "field-packed"),
                                    ("name", "json", "field-name"), ("json_name", "json", "field-json-name"),
                                    ("default", "json", "field-default"), ("options", "descriptor", "field-options")):
            if fa[key] != fb[key]:
                add(category, kind, subject, fa[key], fb[key])
        # Only between two string fields; a type change is reported above.
        if None not in (fa["utf8_validation"], fb["utf8_validation"]) and fa["utf8_validation"] != fb["utf8_validation"]:
            add("wire", "field-utf8-validation", subject, fa["utf8_validation"], fb["utf8_validation"])
        if fa.get("file") != fb.get("file"):
            # Files that import the old one lose the extension.
            add("descriptor", "extension-file", subject, fa.get("file"), fb.get("file"))
        if fa.get("scope") != fb.get("scope"):
            # An extension's JSON key is its full name, which includes the scope.
            add("json", "extension-scope", subject, fa.get("scope"), fb.get("scope"))
        # Mutual exclusion with fields that exist on both sides; a field
        # new to the oneof is already reported as added.
        ma = None if fa["oneof_members"] is None else sorted(set(fa["oneof_members"]) & common)
        mb = None if fb["oneof_members"] is None else sorted(set(fb["oneof_members"]) & common)
        if ma != mb:
            add("wire", "field-oneof", subject, oneof_summary(fa["oneof"], ma), oneof_summary(fb["oneof"], mb))
        elif fa["oneof"] != fb["oneof"]:
            add("descriptor", "field-oneof-name", subject, fa["oneof"], fb["oneof"])
        if fa["default"] == fb["default"] and fa["explicit_default"] != fb["explicit_default"]:
            add("descriptor", "field-default-declaration", subject, fa["explicit_default"], fb["explicit_default"])

    for name in sorted(old["files"].keys() | new["files"].keys()):
        a, b = old["files"].get(name), new["files"].get(name)
        if a is None:
            add("descriptor", "file-added", name, None, b["syntax"])
            continue
        if b is None:
            add("descriptor", "file-removed", name, a["syntax"], None)
            continue
        if a["package"] != b["package"]:
            add("descriptor", "file-package", name, a["package"], b["package"])
        if a["syntax"] != b["syntax"]:
            add("descriptor", "file-syntax", name, a["syntax"], b["syntax"])
        if a["imports"] != b["imports"]:
            add("descriptor", "file-imports", name, sorted(set(a["imports"]) - set(b["imports"])), sorted(set(b["imports"]) - set(a["imports"])))
        if a["options"] != b["options"]:
            add("descriptor", "file-options", name, a["options"], b["options"])
        for key, kind in (("public_imports", "file-public-imports"), ("weak_imports", "file-weak-imports"), ("option_imports", "file-option-imports")):
            if a[key] != b[key]:
                add("descriptor", kind, name, sorted(set(a[key]) - set(b[key])), sorted(set(b[key]) - set(a[key])))

    for name in sorted(old["messages"].keys() | new["messages"].keys()):
        a, b = old["messages"].get(name), new["messages"].get(name)
        if a is None:
            add("wire", "message-added", name, None, f"{len(b['fields'])} fields")
            continue
        if b is None:
            add("wire", "message-removed", name, f"{len(a['fields'])} fields", None)
            continue
        type_placement(name, a, b, "message")
        if a["message_set_wire_format"] != b["message_set_wire_format"]:
            add("wire", "message-set-wire-format", name, a["message_set_wire_format"], b["message_set_wire_format"])
        for oneof in sorted(a["oneof_options"].keys() & b["oneof_options"].keys()):
            if a["oneof_options"][oneof] != b["oneof_options"][oneof]:
                add("descriptor", "oneof-options", f"{name} oneof {oneof}", a["oneof_options"][oneof], b["oneof_options"][oneof])
        for key, kind in (("reserved_ranges", "message-reserved-numbers"), ("reserved_names", "message-reserved-names"),
                          ("extension_ranges", "message-extension-ranges"), ("options", "message-options")):
            if a[key] != b[key]:
                add("descriptor", kind, name, a[key], b[key])
        common = a["fields"].keys() & b["fields"].keys()
        for number in sorted(a["fields"].keys() | b["fields"].keys()):
            compare_fields(f"{name} field {number}", a["fields"].get(number), b["fields"].get(number), common)

    # Extensions are fields declared outside the message they extend; the
    # snapshot closure strips them, but a supplied descriptor set may have them.
    for subject in sorted(old["extensions"].keys() | new["extensions"].keys()):
        compare_fields(subject, old["extensions"].get(subject), new["extensions"].get(subject), set(), "extension")

    for name in sorted(old["enums"].keys() | new["enums"].keys()):
        a, b = old["enums"].get(name), new["enums"].get(name)
        if a is None:
            add("wire", "enum-added", name, None, f"{len(b['values'])} values")
            continue
        if b is None:
            add("wire", "enum-removed", name, f"{len(a['values'])} values", None)
            continue
        type_placement(name, a, b, "enum")
        if a["closed"] != b["closed"]:
            add("wire", "enum-closed", name, a["closed"], b["closed"])
        for key, kind in (("reserved_ranges", "enum-reserved-numbers"), ("reserved_names", "enum-reserved-names"), ("options", "enum-options")):
            if a[key] != b[key]:
                add("descriptor", kind, name, a[key], b[key])
        for number in sorted(a["values"].keys() | b["values"].keys()):
            subject = f"{name} value {number}"
            va, vb = a["values"].get(number), b["values"].get(number)
            if va is None:
                add("wire", "enum-value-added", subject, None, vb)
            elif vb is None:
                add("wire", "enum-value-removed", subject, va, None)
            else:
                if va != vb:
                    add("json", "enum-value-name", subject, va, vb)
                if a["value_options"][number] != b["value_options"][number]:
                    add("descriptor", "enum-value-options", subject, a["value_options"][number], b["value_options"][number])

    for name in sorted(old["services"].keys() | new["services"].keys()):
        a, b = old["services"].get(name), new["services"].get(name)
        if a is None:
            add("wire", "service-added", name, None, f"{len(b['methods'])} methods")
            continue
        if b is None:
            add("wire", "service-removed", name, f"{len(a['methods'])} methods", None)
            continue
        if a["file"] != b["file"]:
            # Reflection clients find a service through the file that declares it.
            add("descriptor", "service-file", name, a["file"], b["file"])
        if a["options"] != b["options"]:
            add("descriptor", "service-options", name, a["options"], b["options"])
        for method in sorted(a["methods"].keys() | b["methods"].keys()):
            subject = f"{name}.{method}"
            ma, mb = a["methods"].get(method), b["methods"].get(method)
            if ma is None:
                add("wire", "method-added", subject, None, method_summary(mb))
            elif mb is None:
                add("wire", "method-removed", subject, method_summary(ma), None)
            else:
                if method_summary(ma) != method_summary(mb):
                    add("wire", "method-signature", subject, method_summary(ma), method_summary(mb))
                if ma["options"] != mb["options"]:
                    add("descriptor", "method-options", subject, ma["options"], mb["options"])

    order = {category: index for index, category in enumerate(CATEGORIES)}
    changes.sort(key=lambda c: (order[c["category"]], c["subject"], c["kind"]))
    return changes


def compare(old_path, new_path):
    old_set, new_set = load_set(old_path), load_set(new_path)
    changes = diff_models(schema_model(old_set), schema_model(new_set))
    summary = {category: sum(1 for c in changes if c["category"] == category) for category in CATEGORIES}
    return {
        "old": {"source": Path(old_path).name, "files": len(old_set.file)},
        "new": {"source": Path(new_path).name, "files": len(new_set.file)},
        "identical": not changes,
        "summary": summary,
        "changes": changes,
        "note": "Field and message names do not establish runtime behavior; added messages and fields are additive on the wire, removals and type changes are not.",
    }


def render(report):
    lines = [f"old: {report['old']['source']} ({report['old']['files']} files)", f"new: {report['new']['source']} ({report['new']['files']} files)"]
    if report["identical"]:
        lines.append("identical: no schema differences")
        return "\n".join(lines) + "\n"
    lines.append("summary: " + ", ".join(f"{report['summary'][c]} {c}" for c in CATEGORIES))
    for category in CATEGORIES:
        entries = [c for c in report["changes"] if c["category"] == category]
        if not entries:
            continue
        lines.append("")
        lines.append(f"{CATEGORY_TITLES[category]}: {len(entries)}")
        for change in entries:
            detail = []
            if change["old"] not in (None, [], ""):
                detail.append(f"was {change['old']}")
            if change["new"] not in (None, [], ""):
                detail.append(f"now {change['new']}")
            lines.append(f"  {change['kind']}: {change['subject']}" + (f" ({'; '.join(map(str, detail))})" if detail else ""))
    lines.append("")
    lines.append(report["note"])
    return "\n".join(lines) + "\n"


# --- CLI ---------------------------------------------------------------------


def parser():
    top = argparse.ArgumentParser(description=__doc__.split("\n\n")[0], formatter_class=argparse.RawDescriptionHelpFormatter)
    commands = top.add_subparsers(dest="command", required=True)

    make = commands.add_parser("snapshot", help="extract, verify and publish one snapshot")
    make.add_argument("--binary", help="agy binary to read (copied first; never executed)")
    make.add_argument("--archive", help="official release .tar.gz to take the binary from")
    make.add_argument("--archive-sha256", help="the archive's published SHA-256 digest")
    make.add_argument("--archive-member", help="the binary's path inside the archive")
    make.add_argument("--expected-sha256", help="refuse unless the binary has this SHA-256")
    make.add_argument("--version", required=True, help="agy release label, such as 1.3.6")
    make.add_argument("--version-evidence", required=True, help="how the version was established, such as 'agy --version printed 1.3.6'")
    make.add_argument("--source-url", help="release or asset URL, for provenance")
    make.add_argument("--root", dest="roots", action="append", default=[], help="descriptor file whose import closure to keep (repeatable, required)")
    make.add_argument("--annotation-only", action="append", default=None,
                      help="import that may be dropped from the closure (repeatable; replaces the default set)")
    make.add_argument("--out-root", default=str(SCRIPTS.parent / "artifacts" / "proto-snapshots"), help="directory that holds snapshots")
    make.add_argument("--protoc", help="protoc executable (default: python -m grpc_tools.protoc)")
    make.add_argument("--printer", help="prebuilt scripts/proto-printer binary (default: build it with go)")

    check = commands.add_parser("verify", help="recheck a published snapshot")
    check.add_argument("snapshot")
    check.add_argument("--binary", help="the original binary, to recheck every record's byte range")

    diff = commands.add_parser("compare", help="compare two descriptor closures")
    diff.add_argument("old")
    diff.add_argument("new")
    diff.add_argument("--json", help="also write the report as JSON to this new file")
    return top


def main(argv=None):
    options = parser().parse_args(argv)
    try:
        if options.command == "snapshot":
            dest = snapshot(options)
            record = json.loads((dest / "snapshot.json").read_text())
            print(f"verified snapshot: {dest}")
            print(f"records: {record['inventory']['records']}, closure files: {record['closure']['files']}, "
                  f"unresolved inventory imports: {record['inventory']['unresolved_imports']}")
            return 0
        if options.command == "verify":
            print(json.dumps(verify(options.snapshot, options.binary), indent=1))
            return 0
        return run_compare(options)
    except SnapshotError as error:
        print(f"refused: {error}", file=sys.stderr)
        return 2


def check_json_path(path):
    """Refuse a --json path that cannot be created, before any comparing."""
    path = Path(path)
    if os.path.lexists(path):
        raise SnapshotError(f"--json output already exists: {path}; choose a new file or remove it")
    parent = path.parent
    if not parent.is_dir():
        raise SnapshotError(f"--json directory does not exist: {parent}")
    if not os.access(parent, os.W_OK | os.X_OK):
        raise SnapshotError(f"--json directory is not writable: {parent}")


def discard_stdout():
    """Point stdout at the null device after a failed write.

    Otherwise Python flushes the unwritten buffer again at exit, fails the same
    way and replaces status 2 with 120.
    """
    try:
        null = os.open(os.devnull, os.O_WRONLY)
        try:
            os.dup2(null, sys.stdout.fileno())
        finally:
            os.close(null)
    except (OSError, ValueError):  # no real file descriptor, as under test capture
        pass


def run_compare(options):
    """Exit 0 identical, 1 different; anything else is a refusal (2), never 1."""
    try:
        if options.json:
            check_json_path(options.json)
        report = compare(options.old, options.new)
        text = render(report)
        if options.json:
            try:
                with open(options.json, "x") as handle:
                    handle.write(json.dumps(report, indent=1) + "\n")
            except OSError as error:
                raise SnapshotError(f"cannot write --json output {options.json}: {error.strerror or error}") from None
        try:
            sys.stdout.write(text)
            sys.stdout.flush()
        except (OSError, UnicodeError) as error:
            discard_stdout()
            raise SnapshotError(f"cannot write the report to stdout: {type(error).__name__}: {error}") from None
    except SnapshotError:
        raise
    except Exception as error:  # a crash must not read as "schemas differ"
        raise SnapshotError(f"compare failed unexpectedly: {type(error).__name__}: {error}") from error
    return 0 if report["identical"] else 1


if __name__ == "__main__":
    sys.exit(main())
