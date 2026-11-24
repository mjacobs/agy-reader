"""Verify the authenticated policy fields and restore the exact previous bundle."""
import gzip
import hashlib
import unittest

from test_generate_parentmsgs1410 import descriptor_pb2, REPO

if descriptor_pb2 is not None:
    import generate_config1514
    import generate_snapshot1515
    import generate_steps1516
    import generate_features1513
    import generate_refs1511
    import generate_policy1510 as generator


@unittest.skipIf(descriptor_pb2 is None, "google.protobuf is not installed")
class Policy1510(unittest.TestCase):
    def test_authenticated_delta_and_round_trip(self):
        production = gzip.decompress((REPO / "internal/completeness/schema.pb.gz").read_bytes())
        bundle = generate_snapshot1515.strip(generate_steps1516.strip(descriptor_pb2.FileDescriptorSet.FromString(production)))
        result = generate_refs1511.strip(generate_features1513.strip(generate_config1514.strip(bundle)))
        production = result.SerializeToString(deterministic=True)
        source = descriptor_pb2.FileDescriptorSet.FromString(generator.SNAPSHOT.read_bytes())
        source.DiscardUnknownFields()
        target, original = (generator.policy(bundle) for bundle in (result, source))
        for name, _, _, present in generator.FIELDS:
            actual = generator.helpers.one(target.field, name)
            expected = generator.helpers.one(original.field, name)
            if present:
                self.assertEqual(target.oneof_decl[actual.oneof_index], original.oneof_decl[expected.oneof_index])
                expected.oneof_index = actual.oneof_index
            self.assertEqual(actual, expected)

        base = generator.strip(result)
        self.assertEqual(hashlib.sha256(base.SerializeToString(deterministic=True)).hexdigest(),
                         "f142a51963d6af1df06072e4c35d92be4a75dce1c545eeb421a7d4837b9483a2")
        self.assertEqual(generator.augment(base).SerializeToString(deterministic=True), production)
