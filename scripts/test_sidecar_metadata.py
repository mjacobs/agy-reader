import copy
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('sidecar_metadata', Path(__file__).with_name('sidecar_metadata.py'))
helper = importlib.util.module_from_spec(spec)
spec.loader.exec_module(helper)


class RepairVerification(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        base = Path(self.temp.name)
        self.root = base / 'store'
        (self.root / 'conversations').mkdir(parents=True)
        self.dest = base / 'backup'
        self.doc = {'cascadeId': '11111111-1111-1111-1111-111111111111', 'steps': [], 'large': 9007199254740993123456789,
                    'agyReader': {'parentCascadeId': '22222222-2222-2222-2222-222222222222', 'other': True}}
        self.path = self.root / 'conversations' / 'parent.trajectory.json'
        self.path.write_text(json.dumps(self.doc))
        self.child = self.root / 'conversations' / 'child.trajectory.json'
        self.child.write_text(json.dumps({'cascadeId': '22222222-2222-2222-2222-222222222222', 'steps': [], 'agyReader': {'parentCascadeId': '11111111-1111-1111-1111-111111111111'}}))
        helper.backup(self.root, self.dest)

    def write(self, doc):
        mtime = self.path.stat().st_mtime_ns
        self.path.write_text(json.dumps(doc))
        os.utime(self.path, ns=(mtime, mtime))

    def repaired(self):
        doc = copy.deepcopy(self.doc)
        del doc['agyReader']['parentCascadeId']
        return doc

    def test_repair_and_idempotence(self):
        self.write(self.repaired())
        report = helper.verify(self.root, self.dest)
        self.assertEqual(report['verified'], 2)
        self.assertEqual(report['changed'], 1)
        self.assertEqual(report['cycles'], 0)
        second = self.dest.with_name('after')
        helper.backup(self.root, second)
        self.assertEqual(helper.verify(self.root, second, unchanged=True)['changed'], 0)
        with self.assertRaises(ValueError):
            helper.verify(self.root, self.dest, unchanged=True)

    def test_cycle_rejected(self):
        with self.assertRaisesRegex(ValueError, 'cycle'):
            helper.verify(self.root, self.dest)

    def test_payload_and_other_reader_fields_preserved(self):
        for key in ['large', 'other', 'boolean']:
            with self.subTest(key=key):
                doc = self.repaired()
                if key == 'large':
                    doc['large'] += 1
                else:
                    doc['agyReader']['other'] = False if key == 'other' else 1
                self.write(doc)
                with self.assertRaisesRegex(ValueError, 'payload changed'):
                    helper.verify(self.root, self.dest)

    def test_timestamp_change_rejected(self):
        self.write(self.repaired())
        mtime = self.path.stat().st_mtime_ns + 1000000000
        os.utime(self.path, ns=(mtime, mtime))
        with self.assertRaisesRegex(ValueError, 'timestamp'):
            helper.verify(self.root, self.dest)

    def test_incomplete_corpus_rejected(self):
        self.child.unlink()
        with self.assertRaisesRegex(ValueError, 'set changed'):
            helper.verify(self.root, self.dest)

    def test_backup_tampering_and_overwrite_rejected(self):
        with self.assertRaises(FileExistsError):
            helper.backup(self.root, self.dest)
        (self.dest / 'conversations' / self.path.name).write_text('{}')
        with self.assertRaisesRegex(ValueError, 'backup content changed'):
            helper.verify(self.root, self.dest)

    def test_symlink_rejected(self):
        self.child.unlink()
        self.child.symlink_to(self.path)
        with self.assertRaisesRegex(ValueError, 'regular sidecar'):
            helper.verify(self.root, self.dest)

    def test_duplicate_json_keys_rejected(self):
        self.path.write_text('{"cascadeId":"parent","steps":[],"steps":[1]}')
        with self.assertRaisesRegex(ValueError, 'duplicate JSON'):
            helper.backup(self.root, self.dest.with_name('bad'))


if __name__ == '__main__':
    unittest.main()
