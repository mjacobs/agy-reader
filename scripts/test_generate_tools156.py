"""Authenticate one enum value without moving the historical schema base."""
import gzip
import hashlib
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

from test_generate_planexecute157 import strip_planexecute

from test_generate_config150 import descriptor_pb2, REPO

if descriptor_pb2 is not None:
    import generate_tools156 as generator


@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class Tools156(unittest.TestCase):
    def production156(self):
        bundle = descriptor_pb2.FileDescriptorSet.FromString(gzip.decompress(
            (REPO / 'internal/completeness/schema.pb.gz').read_bytes()))
        strip_planexecute(bundle)
        message = generator.helpers.one(generator.helpers.one(bundle.file, generator.helpers.CORTEX_FILE).message_type, 'PromptSectionCustomizationConfig')
        message.field.remove(generator.helpers.one(message.field, 'template_overrides'))
        message.nested_type.remove(generator.helpers.one(message.nested_type, 'TemplateOverridesEntry'))
        return bundle

    def base(self):
        bundle = self.production156()
        enum = generator.helpers.one(generator.helpers.one(bundle.file, generator.helpers.CORTEX_FILE).enum_type,
                                     'ExecutorTerminationReason')
        enum.value.remove(generator.helpers.one(enum.value, 'EXECUTOR_TERMINATION_REASON_CHAINED'))
        self.assertEqual(hashlib.sha256(bundle.SerializeToString(deterministic=True)).hexdigest(), generator.BASE_HASH)
        return bundle

    def test_exact_delta_and_reproduction(self):
        base = self.base()
        result = generator.augment(base)
        self.assertEqual(result.SerializeToString(deterministic=True), self.production156().SerializeToString(deterministic=True))
        enum = generator.helpers.one(generator.helpers.one(result.file, generator.helpers.CORTEX_FILE).enum_type,
                                     'ExecutorTerminationReason')
        value = generator.helpers.one(enum.value, 'EXECUTOR_TERMINATION_REASON_CHAINED')
        self.assertEqual(value.number, 12)
        source = descriptor_pb2.FileDescriptorSet.FromString(generator.SNAPSHOT.read_bytes())
        original = generator.helpers.one(generator.helpers.one(source.file, generator.helpers.CORTEX_FILE).enum_type,
                                         'ExecutorTerminationReason')
        self.assertEqual(value, generator.helpers.one(original.value, value.name))
        enum.value.remove(value)
        self.assertEqual(result, base)
        with self.assertRaisesRegex(ValueError, 'already declared'):
            generator.augment(generator.augment(base))

    def test_tampering_and_compression(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            previous, output = root / 'previous.gz', root / 'output.gz'
            previous.write_bytes(gzip.compress(self.base().SerializeToString(deterministic=True), compresslevel=1, mtime=42))
            generator.generate(previous, output)
            self.assertEqual(gzip.decompress(output.read_bytes()), self.production156().SerializeToString(deterministic=True))
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

    def test_input_overwrite_refused(self):
        with self.assertRaisesRegex(ValueError, 'authenticated input'):
            generator.generate(generator.SNAPSHOT, generator.SNAPSHOT)
