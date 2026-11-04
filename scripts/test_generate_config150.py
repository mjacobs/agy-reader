"""Verify the exact authenticated config overlay and preserve historical definitions."""
import gzip
from pathlib import Path
import tempfile
import unittest
from test_generate_parentmsgs1410 import INPUTS, REPO, SNAPSHOTS, descriptor_pb2

if descriptor_pb2 is not None:
    import generate_config150 as generator

SOURCE = SNAPSHOTS / 'agy-1.5.0-b9043860a06a/closure.protoset'


@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class GenerateConfig150(unittest.TestCase):
    def test_exact_overlay_and_reproduction(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'schema.pb.gz'
            old, _ = generator.prior.generate(*INPUTS, output)
            result, first = generator.generate(*INPUTS, SOURCE, output)
            _, second = generator.generate(*INPUTS, SOURCE, output)
            self.assertEqual(first, second)
            self.assertEqual(gzip.decompress(first), gzip.decompress(
                (REPO / 'internal/completeness/schema.pb.gz').read_bytes()))
            source = descriptor_pb2.FileDescriptorSet.FromString(SOURCE.read_bytes())
            source.DiscardUnknownFields()
            target = generator.one(result.file, generator.CORTEX_FILE)
            current = generator.one(source.file, generator.CORTEX_FILE)
            for name in generator.MESSAGES:
                message = generator.one(target.message_type, name)
                self.assertEqual(message, generator.one(current.message_type, name))
                target.message_type.remove(message)
            for message_name, name, *_ in generator.FIELDS:
                message = generator.one(target.message_type, message_name)
                original = generator.one(current.message_type, message_name)
                field = generator.one(message.field, name)
                source_field = generator.one(original.field, name)
                if field.HasField('oneof_index'):
                    declaration = message.oneof_decl[field.oneof_index]
                    self.assertEqual(declaration, original.oneof_decl[source_field.oneof_index])
                    message.oneof_decl.remove(declaration)
                    field.oneof_index = source_field.oneof_index
                self.assertEqual(field, source_field)
                message.field.remove(field)
            self.assertEqual(result, old)

    def test_tampering_preserves_output(self):
        with tempfile.TemporaryDirectory() as directory:
            output, source = Path(directory) / 'output', Path(directory) / 'bad'
            output.write_bytes(b'existing')
            source.write_bytes(SOURCE.read_bytes() + b'tampered')
            with self.assertRaisesRegex(generator.GenerationError, 'snapshot hash'):
                generator.generate(*INPUTS, source, output)
            self.assertEqual(output.read_bytes(), b'existing')
            self.assertFalse(list(Path(directory).glob('.schema-*')))

    def test_refuses_input_as_output(self):
        for source in [*INPUTS, SOURCE, generator.helpers.DEFAULT_BASE]:
            with self.subTest(source=source):
                with self.assertRaisesRegex(generator.GenerationError, 'authenticated input'):
                    generator.generate(*INPUTS, SOURCE, source)
