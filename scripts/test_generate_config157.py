"""Authenticate the exact template-map delta and reject altered inputs."""
import gzip
import hashlib
from pathlib import Path
import tempfile
import unittest

from test_generate_planexecute157 import strip_planexecute

from test_generate_config150 import descriptor_pb2, REPO

if descriptor_pb2 is not None:
    import generate_config157 as generator


@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class Config157(unittest.TestCase):
    def production(self):
        return strip_planexecute(descriptor_pb2.FileDescriptorSet.FromString(gzip.decompress(
            (REPO / 'internal/completeness/schema.pb.gz').read_bytes())))

    def base(self):
        result = self.production()
        message = generator.helpers.one(generator.helpers.one(result.file, generator.helpers.CORTEX_FILE).message_type,
                                        'PromptSectionCustomizationConfig')
        message.field.remove(generator.helpers.one(message.field, 'template_overrides'))
        message.nested_type.remove(generator.helpers.one(message.nested_type, 'TemplateOverridesEntry'))
        self.assertEqual(hashlib.sha256(result.SerializeToString(deterministic=True)).hexdigest(), generator.BASE_HASH)
        return result

    def test_exact_authenticated_delta(self):
        base = self.base()
        result = generator.augment(base)
        self.assertEqual(result, self.production())
        target = generator.helpers.one(generator.helpers.one(result.file, generator.helpers.CORTEX_FILE).message_type,
                                       'PromptSectionCustomizationConfig')
        source = descriptor_pb2.FileDescriptorSet.FromString(generator.SNAPSHOT.read_bytes())
        source.DiscardUnknownFields()
        original = generator.helpers.one(generator.helpers.one(source.file, generator.helpers.CORTEX_FILE).message_type,
                                         target.name)
        for members, name in [('field', 'template_overrides'), ('nested_type', 'TemplateOverridesEntry')]:
            field = generator.helpers.one(getattr(target, members), name)
            self.assertEqual(field, generator.helpers.one(getattr(original, members), name))
            getattr(target, members).remove(field)
        self.assertEqual(result, base)
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
