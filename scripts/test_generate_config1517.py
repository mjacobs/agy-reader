"""Read the new configuration fields from bytes emitted by the native 1.5.17 schema."""
import gzip
import unittest

from test_generate_parentmsgs1410 import REPO, descriptor_pb2

if descriptor_pb2 is not None:
    from google.protobuf import json_format, message_factory
    import agy_proto_snapshot as snapshots
    import generate_config1517
    import generate_options145


@unittest.skipIf(descriptor_pb2 is None, "google.protobuf is not installed")
class Config1517(unittest.TestCase):
    def test_overlay_matches_native_release_delta(self):
        previous = generate_config1517.SNAPSHOT.parents[1] / "agy-1.5.16-05245f4346d1"
        native = snapshots.compare(previous, generate_config1517.SNAPSHOT)
        self.assertEqual(native["summary"], {"wire": 17, "json": 0, "descriptor": 0})

        production = snapshots.load_set(REPO / "internal/completeness/schema.pb.gz")
        current = snapshots.schema_model(production)
        previous = snapshots.schema_model(generate_config1517.strip(production))
        self.assertEqual(snapshots.diff_models(previous, current), native["changes"])

    def test_native_config_fields_round_trip(self):
        native, reader = (
            generate_options145.load_pool(descriptor_pb2.FileDescriptorSet.FromString(raw))
            for raw in (generate_config1517.SNAPSHOT.read_bytes(),
                        gzip.decompress((REPO / "internal/completeness/schema.pb.gz").read_bytes()))
        )
        areas = ("urlDataExfil", "destructiveAction", "remoteCodeExec", "dependencyInstall",
                 "skillSupplyChain", "mcpToolUse", "recon", "userCoercion", "postExecTampering",
                 "networkExposure")
        for name, fields in (
            ("exa.cortex_pb.RunCommandToolConfig", {"env": {"FIRST": "one", "SECOND": ""}}),
            ("exa.codeium_common_pb.ClientModelConfig", {"iconDetails": {"uri": "a", "darkUri": "b"}}),
            ("exa.cortex_pb.PolicyGuardianConfig", {
                "enableProctorReadBudget": False, "proctorSkillScanMode": "SKILL_SCAN_MODE_AUDIT",
                "coverage": {area: {"proctor": "MODE_SHADOW"} for area in areas},
            }),
        ):
            with self.subTest(message=name):
                original, decoded = (
                    message_factory.GetMessageClass(pool.FindMessageTypeByName(name))()
                    for pool in (native, reader)
                )
                json_format.ParseDict(fields, original)
                decoded.ParseFromString(original.SerializeToString())
                self.assertEqual(json_format.MessageToDict(decoded), fields)
