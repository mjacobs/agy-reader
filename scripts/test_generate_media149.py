"""Authenticate the 1.4.9 overlay and prove that historical definitions survive."""
import gzip
from pathlib import Path
import sys
import tempfile
import unittest

try:
    from google.protobuf import descriptor_pb2
except ImportError:
    descriptor_pb2 = None

REPO = Path(__file__).resolve().parents[1]
SNAPSHOTS = REPO / 'artifacts/proto-snapshots'
if descriptor_pb2 is not None:
    sys.path.insert(0, str(REPO / 'internal/completeness'))
    import generate_media149 as generator

INPUTS = [SNAPSHOTS / p for p in (
    'agy-1.4.5-8af8e7a2a68f/extracted', 'agy-1.4.6-128780f57769/extracted',
    'agy-1.4.7-40349135371a/all-extracted.protoset',
    'agy-1.4.8-70b1d27f1812/closure.protoset',
    'agy-1.4.9-1c9102d0a361/closure.protoset')]


@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class GenerateMedia149(unittest.TestCase):
    def test_exact_overlay_and_reproduction(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'schema.pb.gz'
            old, _ = generator.prior.generate(*INPUTS[:4], output)
            result, first = generator.generate(*INPUTS, output)
            _, second = generator.generate(*INPUTS, output)
            self.assertEqual(first, second)
            self.assertEqual(gzip.decompress(first), gzip.decompress(
                (REPO / 'internal/completeness/schema.pb.gz').read_bytes()))
            source = descriptor_pb2.FileDescriptorSet.FromString(INPUTS[4].read_bytes())
            source.DiscardUnknownFields()
            target = generator.one(result.file, generator.CORTEX_FILE)
            current = generator.one(source.file, generator.CORTEX_FILE)
            for message_name, field_name, *_ in generator.ADDITIONS:
                message = generator.one(target.message_type, message_name)
                field = generator.one(message.field, field_name)
                self.assertEqual(field, generator.one(
                    generator.one(current.message_type, message_name).field, field_name))
                message.field.remove(field)
            target.dependency.remove(generator.CONTENT_FILE)
            self.assertEqual(result, old)

    def test_tampering_preserves_output(self):
        with tempfile.TemporaryDirectory() as directory:
            output, source = Path(directory) / 'output', Path(directory) / 'bad'
            output.write_bytes(b'existing')
            source.write_bytes(INPUTS[4].read_bytes() + b'tampered')
            with self.assertRaisesRegex(generator.GenerationError, 'snapshot hash'):
                generator.generate(*INPUTS[:4], source, output)
            self.assertEqual(output.read_bytes(), b'existing')
            self.assertFalse(list(Path(directory).glob('.schema-*')))

    def test_refuses_authenticated_input_as_output(self):
        for source in INPUTS[2:]:
            with self.subTest(source=source):
                with self.assertRaisesRegex(generator.GenerationError, 'authenticated input'):
                    generator.generate(*INPUTS, source)
