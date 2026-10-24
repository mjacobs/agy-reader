"""Authenticate Finish and prove that no historical declaration changed."""
import gzip
from pathlib import Path
import sys
import tempfile
import unittest

try:
    from google.protobuf import descriptor_pb2
except ImportError:
    descriptor_pb2 = None

REPO = Path(__file__).resolve().parents[1]
SNAPSHOTS = REPO / "artifacts/proto-snapshots"
SOURCE145 = SNAPSHOTS / "agy-1.4.5-8af8e7a2a68f/extracted"
SOURCE146 = SNAPSHOTS / "agy-1.4.6-128780f57769/extracted"
SOURCE147 = SNAPSHOTS / "agy-1.4.7-40349135371a/all-extracted.protoset"
if descriptor_pb2 is not None:
    sys.path.insert(0, str(REPO / "internal/completeness"))
    import generate_finish147 as generator


@unittest.skipIf(descriptor_pb2 is None, "google.protobuf is not installed")
class GenerateFinish147(unittest.TestCase):
    def test_exact_overlay_and_reproducibility(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "schema.pb.gz"
            old, _ = generator.prior.generate(SOURCE145, SOURCE146, output)
            result, first = generator.generate(SOURCE145, SOURCE146, SOURCE147, output)
            _, second = generator.generate(SOURCE145, SOURCE146, SOURCE147, output)
            self.assertEqual(first, second)
            self.assertEqual(gzip.decompress(first), gzip.decompress(
                (REPO / "internal/completeness/schema.pb.gz").read_bytes()))
            tools = generator.one(result.file, generator.TOOLS_FILE)
            finish = generator.one(tools.message_type, "Finish")
            source = descriptor_pb2.FileDescriptorSet.FromString(SOURCE147.read_bytes())
            source.DiscardUnknownFields()
            self.assertEqual(finish, generator.one(
                generator.one(source.file, generator.TOOLS_FILE).message_type, "Finish"))
            tools.message_type.remove(finish)
            self.assertEqual(result, old)

    def test_tampered_source_preserves_output(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            output, source = root / "schema.pb.gz", root / "tampered.protoset"
            output.write_bytes(b"existing output")
            source.write_bytes(SOURCE147.read_bytes() + b"tampered")
            with self.assertRaisesRegex(generator.GenerationError, "snapshot hash"):
                generator.generate(SOURCE145, SOURCE146, source, output)
            self.assertEqual(output.read_bytes(), b"existing output")
            self.assertFalse(list(root.glob(".schema-*")))


if __name__ == "__main__":
    unittest.main()
