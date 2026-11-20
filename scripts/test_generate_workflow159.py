"""Authenticate the single workflow title field and preserve all historical declarations."""
import gzip
import hashlib
from pathlib import Path
import tempfile
import unittest

from test_generate_parentmsgs1410 import descriptor_pb2, REPO

if descriptor_pb2 is not None:
    import generate_workflow159 as generator
    import generate_schedule1510
    import generate_features1513
    import generate_refs1511
    import generate_policy1510


@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class Workflow159(unittest.TestCase):
    def production(self):
        bundle = descriptor_pb2.FileDescriptorSet.FromString(gzip.decompress(
            (REPO / 'internal/completeness/schema.pb.gz').read_bytes()))
        return generate_schedule1510.strip(generate_policy1510.strip(generate_refs1511.strip(generate_features1513.strip(bundle))))

    def base(self):
        result = generator.strip(self.production())
        self.assertEqual(hashlib.sha256(result.SerializeToString(deterministic=True)).hexdigest(), generator.BASE_HASH)
        return result

    def test_exact_authenticated_delta(self):
        base = self.base()
        result = generator.augment(base)
        self.assertEqual(result, self.production())
        source = descriptor_pb2.FileDescriptorSet.FromString(generator.SNAPSHOT.read_bytes())
        source.DiscardUnknownFields()
        def title(bundle):
            message = generator.helpers.one(generator.helpers.one(bundle.file, generator.helpers.CORTEX_FILE).message_type, 'ToolSourceMetadata')
            return generator.helpers.one(message.field, 'title')
        self.assertEqual(title(result), title(source))
        self.assertEqual(generator.strip(result), base)
        with self.assertRaisesRegex(ValueError, 'already declared'):
            generator.augment(self.production())

    def test_generation_and_tampering(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            previous, output = root / 'previous.gz', root / 'output.gz'
            previous.write_bytes(gzip.compress(self.base().SerializeToString(deterministic=True), compresslevel=1, mtime=42))
            generator.generate(previous, output)
            self.assertEqual(gzip.decompress(output.read_bytes()), self.production().SerializeToString(deterministic=True))
            saved = output.read_bytes()
            previous.write_bytes(gzip.compress(b'changed'))
            with self.assertRaisesRegex(ValueError, 'base hash'):
                generator.generate(previous, output)
            self.assertEqual(output.read_bytes(), saved)
            bad = root / 'bad.protoset'
            bad.write_bytes(generator.SNAPSHOT.read_bytes() + b'changed')
            with self.assertRaisesRegex(ValueError, 'snapshot hash'):
                generator.augment(self.base(), bad)
            self.assertEqual(output.read_bytes(), saved)
            self.assertFalse(list(root.glob('.schema-*')))
        with self.assertRaisesRegex(ValueError, 'authenticated input'):
            generator.generate(generator.SNAPSHOT, generator.SNAPSHOT)
