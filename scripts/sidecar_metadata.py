#!/usr/bin/env python3
"""Back up sidecars and verify metadata-only repairs. Python 3 standard library."""
import argparse
import decimal
import hashlib
import json
import os
import re
from pathlib import Path
import shutil
import sys


SUFFIX = '.trajectory.json'


def files(directory):
    result = {}
    for path in sorted(directory.glob('*' + SUFFIX)):
        if path.is_symlink() or not path.is_file():
            raise ValueError(f'expected a regular sidecar: {path}')
        result[path.name] = path
    if not result:
        raise ValueError(f'no sidecars in {directory}')
    return result


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load(path):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError(f'duplicate JSON key in {path}')
            result[key] = value
        return result
    def invalid(value):
        raise ValueError(f'non-JSON number in {path}')
    doc = json.loads(path.read_text(), parse_float=decimal.Decimal,
                     parse_constant=invalid, object_pairs_hook=unique)
    if not isinstance(doc, dict):
        raise ValueError(f'expected a trajectory object: {path}')
    return doc


def equal(a, b):
    # Python's ordinary equality equates true with 1; a repair must not.
    if type(a) is not type(b):
        return False
    if isinstance(a, dict):
        return a.keys() == b.keys() and all(equal(a[k], b[k]) for k in a)
    if isinstance(a, list):
        return len(a) == len(b) and all(equal(x, y) for x, y in zip(a, b))
    return a == b


def without_parent(doc):
    doc = dict(doc)
    reader = doc.pop('agyReader', {})
    if not isinstance(reader, dict):
        raise ValueError('agyReader must be an object')
    reader = dict(reader)
    reader.pop('parentCascadeId', None)
    return doc, reader


def check_graph(docs):
    links = {}
    for name, doc in docs.items():
        cid = doc.get('cascadeId')
        if not isinstance(cid, str) or not re.fullmatch(r'[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}', cid):
            raise ValueError(f'missing cascadeId: {name}')
        cid = cid.lower()
        if cid in links:
            raise ValueError(f'duplicate cascadeId: {name}')
        reader = doc.get('agyReader', {})
        if not isinstance(reader, dict):
            raise ValueError(f'invalid agyReader: {name}')
        parent = reader.get('parentCascadeId')
        if parent is not None and (not isinstance(parent, str) or not re.fullmatch(r'[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}', parent)):
            raise ValueError(f'invalid parentCascadeId: {name}')
        links[cid] = parent.lower() if parent else None
    for start in links:
        seen = set()
        node = start
        while node in links:
            if node in seen:
                raise ValueError(f'parent cycle involving {start}')
            seen.add(node)
            node = links[node]
    return sum(parent is not None for parent in links.values())


def backup(root, dest):
    source = files(root / 'conversations')
    dest.mkdir(mode=0o700)  # Never overwrite a previous backup.
    target = dest / 'conversations'
    target.mkdir(mode=0o700)
    manifest = {'version': 1, 'root': str(root.resolve()), 'files': {}}
    for name, path in source.items():
        load(path)  # Reject unreadable JSON before claiming a usable backup.
        stat = path.stat()
        before = digest(path)
        shutil.copy2(path, target / name)
        if before != digest(target / name) or before != digest(path) or stat.st_mtime_ns != path.stat().st_mtime_ns:
            raise ValueError(f'sidecar changed while backing up: {name}; stop writers')
        manifest['files'][name] = {'sha256': before, 'mtimeNs': stat.st_mtime_ns}
    if set(source) != set(files(root / 'conversations')):
        raise ValueError('sidecar set changed during backup; stop writers')
    for name, path in source.items():
        record = manifest['files'][name]
        if digest(path) != record['sha256'] or path.stat().st_mtime_ns != record['mtimeNs']:
            raise ValueError(f'sidecar changed during backup: {name}; stop writers')
    # Write the manifest last. An interrupted backup is not usable evidence.
    path = dest / 'manifest.json'
    with path.open('x') as stream:
        json.dump(manifest, stream, indent=2)
        stream.write('\n')
    os.chmod(path, 0o600)
    return {'backedUp': len(source), 'backup': str(dest)}


def verify(root, dest, unchanged=False):
    manifest = json.loads((dest / 'manifest.json').read_text())
    if manifest.get('version') != 1 or manifest.get('root') != str(root.resolve()):
        raise ValueError('backup manifest version or source root mismatch')
    old = files(dest / 'conversations')
    current = files(root / 'conversations')
    if set(old) != set(current) or set(old) != set(manifest['files']):
        raise ValueError('sidecar set changed; verification requires the same complete corpus')
    changed = []
    docs = {}
    for name, path in current.items():
        record = manifest['files'][name]
        if digest(old[name]) != record['sha256']:
            raise ValueError(f'backup content changed: {name}')
        before, after = load(old[name]), load(path)
        a, ar = without_parent(before)
        b, br = without_parent(after)
        if not equal(a, b) or not equal(ar, br):
            raise ValueError(f'non-parent payload changed: {name}')
        if path.stat().st_mtime_ns != record['mtimeNs']:
            raise ValueError(f'freshness timestamp changed: {name}')
        different = digest(path) != record['sha256']
        if unchanged and different:
            raise ValueError(f'second repair changed bytes: {name}')
        if different:
            changed.append(name)
        docs[name] = after
    links = check_graph(docs)
    return {'verified': len(current), 'changed': len(changed), 'changedFiles': changed, 'parentLinks': links,
            'cycles': 0, 'payloadsPreserved': True, 'timestampsPreserved': True}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=['backup', 'verify'])
    parser.add_argument('--root', required=True, type=Path, help='store containing conversations/')
    parser.add_argument('--backup', required=True, type=Path, help='new backup directory for backup; existing one for verify')
    parser.add_argument('--unchanged', action='store_true', help='verify byte-for-byte idempotence as well')
    args = parser.parse_args()
    if args.command == 'backup' and args.unchanged:
        parser.error('--unchanged applies only to verify')
    try:
        result = backup(args.root, args.backup) if args.command == 'backup' else verify(args.root, args.backup, args.unchanged)
    except (OSError, ValueError, KeyError, TypeError) as exc:
        print(f'sidecar-metadata: {exc}', file=sys.stderr)
        return 1
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == '__main__':
    sys.exit(main())
