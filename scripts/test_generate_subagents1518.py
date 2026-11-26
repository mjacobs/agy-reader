"""Verify the agy 1.5.18 decoder delta against both extracted release schemas."""
import unittest

from test_generate_parentmsgs1410 import REPO, SNAPSHOTS, descriptor_pb2

if descriptor_pb2 is not None:
    import agy_proto_snapshot as snapshots
    import generate_subagents1518 as generator


@unittest.skipIf(descriptor_pb2 is None, "google.protobuf is not installed")
class Subagents1518(unittest.TestCase):
    def test_overlay_matches_release_wire_and_json_changes(self):
        native = snapshots.compare(SNAPSHOTS / "agy-1.5.17-f3f384bd2c15", generator.SNAPSHOT)
        self.assertEqual(native["summary"], {"wire": 1, "json": 0, "descriptor": 1})
        production = snapshots.load_set(REPO / "internal/completeness/schema.pb.gz")
        updated = snapshots.schema_model(production)
        original = snapshots.schema_model(generator.strip(production))
        # HeuristicPrompt gained a deprecation notice, which has no effect on decoding.
        self.assertEqual(snapshots.diff_models(original, updated),
                         [change for change in native["changes"] if change["category"] != "descriptor"])
