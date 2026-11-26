"""Compare the shipped agy 1.5.13 declarations with their verified source."""
import gzip
import unittest

from test_generate_parentmsgs1410 import REPO, descriptor_pb2

if descriptor_pb2 is not None:
    import generate_config1514
    import generate_snapshot1515
    import generate_steps1516
    import generate_config1517
    import generate_subagents1518
    import generate_features1513 as generator
    from google.protobuf import message_factory


@unittest.skipIf(descriptor_pb2 is None, "google.protobuf is not installed")
class Features1513(unittest.TestCase):
    def test_snapshot_declarations_and_explicit_false(self):
        production = gzip.decompress((REPO / "internal/completeness/schema.pb.gz").read_bytes())
        bundle = generate_subagents1518.strip(descriptor_pb2.FileDescriptorSet.FromString(production))
        bundle = generate_config1517.strip(bundle)
        bundle = generate_snapshot1515.strip(generate_steps1516.strip(bundle))
        actual = generate_config1514.strip(bundle)
        source = descriptor_pb2.FileDescriptorSet.FromString(generator.SNAPSHOT.read_bytes())
        source.DiscardUnknownFields()
        one = generator.helpers.one
        common, reference = (one(b.file, generator.COMMON) for b in (actual, source))
        for name in ("ModelFeatures", "CliSettings"):
            self.assertEqual(one(common.message_type, name), one(reference.message_type, name))
        self.assertEqual(one(common.enum_type, "CliAgentMode"), one(reference.enum_type, "CliAgentMode"))

        target, original = (one(b.file, generator.helpers.CORTEX_FILE) for b in (actual, source))
        self.assertEqual(one(target.enum_type, "InteractionMode"), one(original.enum_type, "InteractionMode"))
        for message_name, field_name in generator.FIELDS.items():
            dst, src = (one(f.message_type, message_name) for f in (target, original))
            copied, expected = (one(m.field, field_name) for m in (dst, src))
            if expected.HasField("oneof_index"):
                self.assertEqual(dst.oneof_decl[copied.oneof_index], src.oneof_decl[expected.oneof_index])
                expected.oneof_index = copied.oneof_index
            self.assertEqual(copied, expected)

        pool = generator.helpers.load_pool(actual)
        model = message_factory.GetMessageClass(pool.FindMessageTypeByName("exa.codeium_common_pb.ModelFeatures"))
        flags = model(supports_tool_calls=False)
        decoded = model.FromString(flags.SerializeToString())
        self.assertTrue(decoded.HasField("supports_tool_calls"))
        self.assertFalse(decoded.supports_tool_calls)
        self.assertFalse(decoded.HasField("supports_thinking"))
