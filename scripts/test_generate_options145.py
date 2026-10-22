"""Reproducibility and refusal checks for the bounded configuration overlay."""

import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest import mock

try:
    from google.protobuf import descriptor_pb2, message_factory
except ImportError:
    descriptor_pb2 = message_factory = None

REPO = Path(__file__).resolve().parents[1]
SOURCE = REPO / "artifacts/proto-snapshots/agy-1.4.5-8af8e7a2a68f/extracted"

if descriptor_pb2 is not None:
    spec = importlib.util.spec_from_file_location(
        "generate_options145", REPO / "internal/completeness/generate_options145.py")
    generator = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(generator)
    spec = importlib.util.spec_from_file_location(
        "agy_proto_snapshot_options145", Path(__file__).with_name("agy_proto_snapshot.py"))
    snapshot = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(snapshot)


@unittest.skipIf(descriptor_pb2 is None, "google.protobuf is not installed")
class GenerateOptions145(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.output = self.root / "schema.pb.gz"
        self.original_output = b"existing output must survive a refusal"
        self.output.write_bytes(self.original_output)

    def copied_source(self):
        target = self.root / "extracted"
        shutil.copytree(SOURCE, target)
        return target

    def refused(self, source=SOURCE, base=None, message=""):
        with self.assertRaisesRegex(generator.GenerationError, message):
            generator.generate(source, self.output, base or generator.DEFAULT_BASE)
        self.assertEqual(self.output.read_bytes(), self.original_output)
        self.assertFalse(list(self.root.glob(".schema-*")))

    def test_rebuild_is_deterministic_and_only_adds_five_declarations(self):
        result, first = generator.generate(SOURCE, self.output)
        _, second = generator.generate(SOURCE, self.output)
        self.assertEqual(first, second)
        self.assertEqual(self.output.read_bytes(), first)
        self.assertEqual(gzip.decompress(first), gzip.decompress(
            (REPO / "internal/completeness/schema.pb.gz").read_bytes()))
        self.assertEqual(len(result.file), 31)
        report = snapshot.compare(generator.DEFAULT_BASE, self.output)
        self.assertEqual(report["summary"], {"wire": 5, "json": 0, "descriptor": 0})
        self.assertEqual({(c["kind"], c["subject"]) for c in report["changes"]}, {
            ("field-added", "exa.cortex_pb.AntigravityBrowserToolConfig field 31"),
            ("field-added", "exa.cortex_pb.ToolOutputConfig field 2"),
            ("field-added", "exa.cortex_pb.ViewFileToolConfig field 21"),
            ("message-added", "exa.cortex_pb.ToolOutputConfig.PerToolEntry"),
            ("message-added", "exa.cortex_pb.ToolOutputOverride"),
        })
        pool = generator.load_pool(result)
        # Historical decoding survives; unrelated newer definitions stay absent.
        self.assertEqual(pool.FindMessageTypeByName("exa.cortex_pb.PolicyGuardianConfig")
                         .fields_by_number[14].name, "use_v2")
        self.assertNotIn(2, pool.FindEnumTypeByName(
            "genai.CodeExecutionCallContent.CodeExecutionCallArguments.Language").values_by_number)
        self.assertNotIn(30, pool.FindMessageTypeByName(
            "exa.cortex_pb.AntigravityBrowserToolConfig").fields_by_number)

    def test_optional_presence_and_existing_oneofs_are_preserved(self):
        result, _ = generator.generate(SOURCE, self.output)
        pool = generator.load_pool(result)
        for message_name, name in (
            ("AntigravityBrowserToolConfig", "enable_browser_subagent_auto_invoke"),
            ("ViewFileToolConfig", "skip_plugin_logo"),
            ("ToolOutputOverride", "max_output_bytes"),
        ):
            with self.subTest(message=message_name):
                descriptor = pool.FindMessageTypeByName("exa.cortex_pb." + message_name)
                message = message_factory.GetMessageClass(descriptor)()
                field = descriptor.fields_by_name[name]
                self.assertTrue(field.has_presence)
                self.assertEqual(field.containing_oneof.name, "_" + name)
                self.assertFalse(message.HasField(name))
                setattr(message, name, 0)
                self.assertTrue(message.HasField(name))
                encoded = message.SerializeToString()
                self.assertTrue(encoded)
                restored = type(message).FromString(encoded)
                self.assertTrue(restored.HasField(name))

    def test_map_accepts_signed_override_and_preserves_zero_presence(self):
        result, _ = generator.generate(SOURCE, self.output)
        descriptor = generator.load_pool(result).FindMessageTypeByName("exa.cortex_pb.ToolOutputConfig")
        cls = message_factory.GetMessageClass(descriptor)
        message = cls()
        for name, value in (("negative", -1), ("zero", 0), ("positive", 4096)):
            message.per_tool[name].max_output_bytes = value
        restored = cls.FromString(message.SerializeToString())
        self.assertEqual({k: v.max_output_bytes for k, v in restored.per_tool.items()},
                         {"negative": -1, "zero": 0, "positive": 4096})
        self.assertTrue(restored.per_tool["zero"].HasField("max_output_bytes"))

    def test_changed_base_is_refused_without_replacing_output(self):
        path = self.root / "base.gz"
        path.write_bytes(generator.DEFAULT_BASE.read_bytes() + b"changed")
        self.refused(base=path, message="base descriptor hash")

    def test_changed_manifest_is_refused_without_replacing_output(self):
        source = self.copied_source()
        manifest = source / "manifest.json"
        manifest.write_bytes(manifest.read_bytes() + b"\n")
        self.refused(source, message="manifest hash")

    def test_changed_record_is_refused_without_replacing_output(self):
        source = self.copied_source()
        record = source / "0000.pb"
        record.write_bytes(record.read_bytes() + b"\x00")
        self.refused(source, message="descriptor bytes do not match")

    def test_missing_and_extra_records_are_refused(self):
        source = self.copied_source()
        record = source / "0000.pb"
        original = record.read_bytes()
        record.unlink()
        self.refused(source, message="inventory does not match")
        record.write_bytes(original)
        (source / "extra.pb").write_bytes(original)
        self.refused(source, message="inventory does not match")

    def test_output_cannot_replace_pinned_base(self):
        with self.assertRaisesRegex(generator.GenerationError, "refusing to replace the pinned base"):
            generator.generate(SOURCE, generator.DEFAULT_BASE)
        self.assertEqual(hashlib.sha256(generator.DEFAULT_BASE.read_bytes()).hexdigest(),
                         generator.BASE_SHA256)

    def test_different_binary_identity_is_refused_even_with_matching_manifest_hash(self):
        source = self.copied_source()
        manifest = source / "manifest.json"
        data = json.loads(manifest.read_text())
        data["binary_sha256"] = "0" * 64
        manifest.write_text(json.dumps(data))
        with mock.patch.object(generator, "MANIFEST_SHA256", hashlib.sha256(manifest.read_bytes()).hexdigest()):
            self.refused(source, message="does not identify the qualified binary")

    def test_source_presence_mismatch_and_existing_target_are_refused(self):
        base, source = generator.verified_inputs(generator.DEFAULT_BASE, SOURCE)
        browser = generator.one(source.message_type, "AntigravityBrowserToolConfig")
        field = generator.one(browser.field, "enable_browser_subagent_auto_invoke")
        field.proto3_optional = False
        with self.assertRaisesRegex(generator.GenerationError, "unexpected source field definition"):
            generator.augment(base, source)
        field.proto3_optional = True
        augmented = generator.augment(base, source)
        with self.assertRaisesRegex(generator.GenerationError, "target field already exists"):
            generator.augment(augmented, source)


if __name__ == "__main__":
    unittest.main()
