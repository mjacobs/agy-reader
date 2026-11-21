"""Check JSON decoding of plan mode and tool settings under the shipped schema."""
import gzip
import unittest

from test_generate_parentmsgs1410 import REPO, descriptor_pb2

if descriptor_pb2 is not None:
    from google.protobuf import json_format, message_factory
    import generate_options145


@unittest.skipIf(descriptor_pb2 is None, "google.protobuf is not installed")
class Config1514(unittest.TestCase):
    def test_decode_native_message_bytes(self):
        production = gzip.decompress((REPO / "internal/completeness/schema.pb.gz").read_bytes())
        reference = (REPO / "artifacts/proto-snapshots/agy-1.5.14-37873b806239/closure.protoset").read_bytes()
        native, reader = (
            generate_options145.load_pool(descriptor_pb2.FileDescriptorSet.FromString(raw))
            for raw in (reference, production)
        )
        for name, payload in (
            ("exa.codeium_common_pb.CliSettings", {"agentMode": "CLI_AGENT_MODE_PLAN"}),
            ("exa.codeium_common_pb.CascadeModelConfigData",
             {"groupedModels": {"example": {"defaultThinkingSuffix": "low"}}}),
            ("exa.cortex_pb.SidecarConfig",
             {"sandbox": {"allowedRpcs": ["example.Read"]}}),
            ("exa.cortex_pb.CascadePlannerConfig", {"responseLanguage": "fr"}),
            ("exa.cortex_pb.RunCommandToolConfig", {"enableToolSummary": True}),
            ("exa.cortex_pb.PolicyGuardianConfig", {"enableConfirmedRepeatCheck": True}),
        ):
            with self.subTest(message=name):
                original, decoded = (
                    message_factory.GetMessageClass(pool.FindMessageTypeByName(name))()
                    for pool in (native, reader)
                )
                json_format.ParseDict(payload, original)
                decoded.ParseFromString(original.SerializeToString())
                self.assertEqual(json_format.MessageToDict(decoded), payload)
