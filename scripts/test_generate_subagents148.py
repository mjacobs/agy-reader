"""Prove the authenticated overlay preserves all historical declarations."""
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
    import generate_subagents148 as generator
INPUTS = [SNAPSHOTS / p for p in (
    'agy-1.4.5-8af8e7a2a68f/extracted', 'agy-1.4.6-128780f57769/extracted',
    'agy-1.4.7-40349135371a/all-extracted.protoset',
    'agy-1.4.8-70b1d27f1812/closure.protoset')]

@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class GenerateSubagents148(unittest.TestCase):
    def test_exact_overlay_and_reproduction(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'schema.pb.gz'
            old, _ = generator.prior.generate(*INPUTS[:3], output)
            result, first = generator.generate(*INPUTS, output)
            _, second = generator.generate(*INPUTS, output)
            self.assertEqual(first, second)
            self.assertEqual(gzip.decompress(first), gzip.decompress(
                (REPO / 'internal/completeness/schema.pb.gz').read_bytes()))
            target = generator.one(generator.one(result.file, generator.TOOLS_FILE).message_type, 'ManageSubagents')
            source = descriptor_pb2.FileDescriptorSet.FromString(INPUTS[3].read_bytes())
            source.DiscardUnknownFields()
            self.assertEqual(target, generator.one(generator.one(source.file, generator.TOOLS_FILE).message_type, 'ManageSubagents'))
            target.field.remove(generator.one(target.field, 'subagents'))
            target.nested_type.remove(generator.one(target.nested_type, 'SubagentState'))
            self.assertEqual(result, old)

    def test_tampering_preserves_output(self):
        with tempfile.TemporaryDirectory() as directory:
            output, source = Path(directory) / 'output', Path(directory) / 'bad'
            output.write_bytes(b'existing')
            source.write_bytes(INPUTS[3].read_bytes() + b'tampered')
            with self.assertRaisesRegex(generator.GenerationError, 'snapshot hash'):
                generator.generate(*INPUTS[:3], source, output)
            self.assertEqual(output.read_bytes(), b'existing')
