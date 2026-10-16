#!/usr/bin/env python3
"""Extract embedded FileDescriptorProto records from an agy binary.

usage: extract_descriptors.py AGY_BINARY OUT_DIR

Read-only on the binary. OUT_DIR must not exist. Writes one NNNN.pb per
descriptor and a manifest.json (binary SHA-256, and each record's name,
SHA-256, offset and length) in the layout that
internal/completeness/generate_schema.py consumes. Needs google.protobuf.
docs/completeness.md, "Schema provenance and maintenance", gives the full
refresh procedure.
"""
import hashlib
import json
import mmap
from pathlib import Path
import re
import sys

from google.protobuf import descriptor_pb2

# FileDescriptorProto top-level field numbers and their wire types.
ALLOWED = {1: 2, 2: 2, 3: 2, 4: 2, 5: 2, 6: 2, 7: 2, 8: 2, 9: 2, 10: (0, 2), 11: (0, 2), 12: 2, 13: 2, 14: 0}
# Fields that may repeat. A marshalled descriptor lists its fields in ascending
# number order, so a field numbered below the one before it, or a repeat of a
# singular field, is the start of something else, not more of this record.
REPEATED = {3, 4, 5, 6, 7, 10, 11}
SYNTAXES = {"", "proto2", "proto3", "editions"}
NAME_PATTERN = re.compile(rb"[A-Za-z0-9_./\-]{3,200}\.proto")


def varint(data, pos):
    value = shift = 0
    while True:
        if pos >= len(data) or shift > 63:
            return None, pos
        byte = data[pos]
        pos += 1
        value |= (byte & 0x7F) << shift
        if not byte & 0x80:
            return value, pos
        shift += 7


def boundaries(data, start):
    """Offsets just after each top-level field of the descriptor at start.

    The scan is greedy: bytes that follow the record can look like one more
    field (0x22 0x01 0xff is a one-byte message_type), so the caller tries the
    longest prefix first and backs off to an earlier boundary when the longer
    bytes do not parse. Field order bounds what the scan can absorb: a field
    numbered below the previous one (0x1a, a dependency, after message types)
    or a repeated singular field ends the record.
    """
    pos, last, ends = start, 0, []
    while pos < len(data):
        tag, after = varint(data, pos)
        if tag is None:
            break
        number, wire = tag >> 3, tag & 7
        allowed = ALLOWED.get(number)
        if allowed is None or (wire != allowed if isinstance(allowed, int) else wire not in allowed):
            break
        if number < last or (number == last and number not in REPEATED):
            break
        last = number
        if wire == 0:
            value, after = varint(data, after)
            if value is None:
                break
        else:
            length, after = varint(data, after)
            if length is None or after + length > len(data):
                break
            after += length
        pos = after
        ends.append(pos)
    return ends


def parse_at(data, header, candidate):
    """The longest prefix starting at header that parses as a descriptor named candidate."""
    for end in reversed(boundaries(data, header)):
        blob = bytes(data[header:end])
        try:
            descriptor = descriptor_pb2.FileDescriptorProto.FromString(blob)
        except Exception:
            continue
        if descriptor.name.encode() == candidate and descriptor.syntax in SYNTAXES and not repeats(descriptor):
            return descriptor, blob
    return None, None


def repeats(descriptor):
    """True when a top-level name is declared twice, which no real file does.

    Trailing bytes that happen to form one more message_type would otherwise
    be absorbed as a second declaration and poison the generated bundle.
    """
    names = [m.name for m in descriptor.message_type] + [e.name for e in descriptor.enum_type] + [s.name for s in descriptor.service]
    return len(names) != len(set(names))


def find_descriptors(data):
    """Map each descriptor name to (offset, serialized bytes), longest record winning."""
    found = {}
    for match in NAME_PATTERN.finditer(data):
        name = match.group()
        # The name may have matched extra leading bytes; try each suffix whose
        # length prefix and field-1 tag sit immediately before it.
        for skip in range(len(name) - 6):
            candidate = name[skip:]
            start = match.start() + skip
            if len(candidate) < 128:
                header = start - 2
                if header < 0 or data[header] != 0x0A or data[header + 1] != len(candidate):
                    continue
            else:
                header = start - 3
                if header < 0:
                    continue
                length, _ = varint(data, header + 1)
                if data[header] != 0x0A or length != len(candidate):
                    continue
            descriptor, blob = parse_at(data, header, candidate)
            if descriptor is None or not (
                descriptor.message_type or descriptor.enum_type or descriptor.service or descriptor.dependency or descriptor.package
            ):
                continue
            previous = found.get(descriptor.name)
            if previous is None or len(blob) > len(previous[1]):
                found[descriptor.name] = (header, blob)
            break
    return found


def extract(binary, out):
    binary, out = Path(binary), Path(out)
    out.mkdir(parents=True, exist_ok=False)
    sha = hashlib.sha256()
    with binary.open("rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            sha.update(block)
        with mmap.mmap(handle.fileno(), 0, access=mmap.ACCESS_READ) as data:
            found = find_descriptors(data)
    files = []
    for index, (name, (offset, blob)) in enumerate(sorted(found.items())):
        (out / f"{index:04d}.pb").write_bytes(blob)
        files.append({"name": name, "sha256": hashlib.sha256(blob).hexdigest(), "offset": offset, "length": len(blob)})
    manifest = {"binary_sha256": sha.hexdigest(), "files": files}
    (out / "manifest.json").write_text(json.dumps(manifest, indent=1))
    return manifest


def main(argv):
    if len(argv) != 3:
        sys.exit(__doc__.strip().splitlines()[2])
    manifest = extract(argv[1], argv[2])
    print(len(manifest["files"]), "descriptors", manifest["binary_sha256"])


if __name__ == "__main__":
    main(sys.argv)
