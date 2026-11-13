"""Authenticate the exact plan/execute delta and preserve historical definitions."""
import gzip
import hashlib
from pathlib import Path
import tempfile
import unittest

from test_generate_parentmsgs1410 import descriptor_pb2, REPO

if descriptor_pb2 is not None:
    import generate_planexecute157 as generator


def strip_planexecute(bundle):
    cortex = generator.helpers.one(bundle.file, generator.helpers.CORTEX_FILE)
    planner = generator.helpers.one(cortex.message_type, 'CascadePlannerConfig')
    planner.field.remove(generator.helpers.one(planner.field, 'plan_execute_config'))
    cortex.message_type.remove(generator.helpers.one(cortex.message_type, 'PlanExecuteConfig'))
    return bundle


@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class PlanExecute157(unittest.TestCase):
    def production(self):
        return descriptor_pb2.FileDescriptorSet.FromString(gzip.decompress(
            (REPO / 'internal/completeness/schema.pb.gz').read_bytes()))

    def base(self):
        result = strip_planexecute(self.production())
        self.assertEqual(hashlib.sha256(result.SerializeToString(deterministic=True)).hexdigest(), generator.BASE_HASH)
        return result

    def test_exact_authenticated_delta(self):
        base = self.base()
        result = generator.augment(base)
        self.assertEqual(result, self.production())
        target = generator.helpers.one(result.file, generator.helpers.CORTEX_FILE)
        source = descriptor_pb2.FileDescriptorSet.FromString(generator.SNAPSHOT.read_bytes())
        source.DiscardUnknownFields()
        original = generator.helpers.one(source.file, generator.helpers.CORTEX_FILE)
        planner = generator.helpers.one(target.message_type, 'CascadePlannerConfig')
        source_planner = generator.helpers.one(original.message_type, planner.name)
        field = generator.helpers.one(planner.field, 'plan_execute_config')
        self.assertEqual(field, generator.helpers.one(source_planner.field, field.name))
        message = generator.helpers.one(target.message_type, 'PlanExecuteConfig')
        self.assertEqual(message, generator.helpers.one(original.message_type, message.name))
        self.assertEqual(strip_planexecute(result), base)
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
