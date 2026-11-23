"""Verify that the timer overlay changes only the authenticated field and import."""
import gzip
import hashlib
from pathlib import Path
import tempfile
import unittest

from test_generate_parentmsgs1410 import descriptor_pb2, REPO

if descriptor_pb2 is not None:
    import generate_schedule1510 as generator
    import generate_config1514
    import generate_snapshot1515
    import generate_features1513
    import generate_refs1511
    import generate_policy1510


@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class Schedule1510(unittest.TestCase):
    def test_authenticated_delta_and_round_trip(self):
        production = gzip.decompress((REPO / 'internal/completeness/schema.pb.gz').read_bytes())
        bundle = generate_snapshot1515.strip(descriptor_pb2.FileDescriptorSet.FromString(production))
        result = generate_policy1510.strip(generate_refs1511.strip(generate_features1513.strip(generate_config1514.strip(bundle))))
        production = result.SerializeToString(deterministic=True)
        source = descriptor_pb2.FileDescriptorSet.FromString(generator.SNAPSHOT.read_bytes())
        source.DiscardUnknownFields()

        def fire_time(bundle):
            tools = generator.helpers.one(bundle.file, generator.TOOLS_FILE)
            self.assertIn(generator.TIMESTAMP, tools.dependency)
            schedule = generator.helpers.one(tools.message_type, 'Schedule')
            return generator.helpers.one(schedule.field, 'fire_time')

        self.assertEqual(fire_time(result), fire_time(source))
        with self.assertRaisesRegex(ValueError, 'already declared'):
            generator.augment(result)
        base = generator.strip(result)
        self.assertEqual(hashlib.sha256(base.SerializeToString(deterministic=True)).hexdigest(), generator.BASE_HASH)
        self.assertEqual(generator.augment(base).SerializeToString(deterministic=True), production)
        with tempfile.TemporaryDirectory() as directory:
            bad = Path(directory) / 'tampered.protoset'
            bad.write_bytes(generator.SNAPSHOT.read_bytes() + b'changed')
            with self.assertRaisesRegex(ValueError, 'snapshot hash'):
                generator.augment(base, bad)
