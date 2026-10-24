"""Exact preservation, reproducibility and refusal checks for checkpoint support."""

import gzip
import hashlib
import json
from pathlib import Path
import shutil
import sys
import tempfile
import unittest
from unittest import mock

try:
    from google.protobuf import descriptor_pb2, message_factory
except ImportError:
    descriptor_pb2 = message_factory = None

REPO = Path(__file__).resolve().parents[1]
SOURCE = REPO / "artifacts/proto-snapshots/agy-1.4.6-128780f57769/extracted"
PRIOR_SOURCE = REPO / "artifacts/proto-snapshots/agy-1.4.5-8af8e7a2a68f/extracted"
if descriptor_pb2 is not None:
    sys.path.insert(0, str(REPO / "internal/completeness"))
    import generate_compaction146 as generator


@unittest.skipIf(descriptor_pb2 is None, "google.protobuf is not installed")
class GenerateCompaction146(unittest.TestCase):
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

    def refused(self, source=SOURCE, base=None, prior=PRIOR_SOURCE, message=""):
        with self.assertRaisesRegex(generator.GenerationError, message):
            generator.generate(prior, source, self.output, base or generator.DEFAULT_BASE)
        self.assertEqual(self.output.read_bytes(), self.original_output)
        self.assertFalse(list(self.root.glob(".schema-*")))

    def prior_bundle(self):
        prior = generator.prior
        base, source = prior.verified_inputs(prior.DEFAULT_BASE, PRIOR_SOURCE)
        return prior.augment(base, source)

    def test_deterministic_exact_overlay_preserves_every_old_declaration(self):
        result, first = generator.generate(PRIOR_SOURCE, SOURCE, self.output)
        _, second = generator.generate(PRIOR_SOURCE, SOURCE, self.output)
        self.assertEqual(first, second)
        self.assertEqual(first, self.output.read_bytes())
        # The committed bundle may have been compressed by another zlib version.
        production = descriptor_pb2.FileDescriptorSet.FromString(gzip.decompress(
            (REPO / "internal/completeness/schema.pb.gz").read_bytes()))
        # The later Finish overlay has its own exact-preservation test.
        tools = generator.one(production.file, "third_party/jetski/cortex/core/tools_pb/tools.proto")
        tools.message_type.remove(generator.one(tools.message_type, "Finish"))
        self.assertEqual(result, production)
        self.assertEqual(len(result.file), 31)
        source = generator.verified_source(SOURCE)[generator.prior.CORTEX_FILE]
        target = generator.one(result.file, generator.prior.CORTEX_FILE)
        captured = generator.one(target.message_type, "CapturedUserRequest")
        self.assertEqual(captured, generator.one(source.message_type, captured.name))
        target.message_type.remove(captured)
        checkpoint = generator.one(target.message_type, "CortexStepCheckpoint")
        field = generator.one(checkpoint.field, "captured_user_requests")
        self.assertEqual(field, generator.one(generator.one(source.message_type, checkpoint.name).field, field.name))
        checkpoint.field.remove(field)
        config = generator.one(target.message_type, "CheckpointConfig")
        source_config = generator.one(source.message_type, config.name)
        for name in ("max_user_request_media_tokens", "context_window"):
            field = generator.one(config.field, name)
            original = generator.one(source_config.field, name)
            self.assertEqual(config.oneof_decl[field.oneof_index], source_config.oneof_decl[original.oneof_index])
            # Remapping a synthetic oneof index is the only source change.
            field.oneof_index = original.oneof_index
            self.assertEqual(field, original)
            config.field.remove(field)
            config.oneof_decl.remove(generator.one(config.oneof_decl, "_" + name))
        discovery = generator.one(target.message_type, "CustomizationDiscoveryConfig")
        original = generator.one(source.message_type, discovery.name)
        enablement = generator.one(discovery.field, "kind_enablement")
        entry = generator.one(discovery.nested_type, "KindEnablementEntry")
        self.assertEqual(enablement, generator.one(original.field, enablement.name))
        self.assertEqual(entry, generator.one(original.nested_type, entry.name))
        discovery.field.remove(enablement)
        discovery.nested_type.remove(entry)
        self.assertEqual(result, self.prior_bundle())

    def test_optional_presence_and_old_oneofs_survive(self):
        result, _ = generator.generate(PRIOR_SOURCE, SOURCE, self.output)
        pool = generator.prior.load_pool(result)
        descriptor = pool.FindMessageTypeByName("exa.cortex_pb.CheckpointConfig")
        cls = message_factory.GetMessageClass(descriptor)
        for name, value in (("context_window", ""), ("max_user_request_media_tokens", 0),
                            ("max_user_request_media_tokens", -1), ("enable_fallback", False)):
            with self.subTest(field=name, value=value):
                message = cls()
                self.assertFalse(message.HasField(name))
                setattr(message, name, value)
                restored = cls.FromString(message.SerializeToString())
                self.assertTrue(restored.HasField(name))
                self.assertEqual(getattr(restored, name), value)

    def test_changed_base_and_prior_snapshot_are_refused(self):
        path = self.root / "base.gz"
        path.write_bytes(generator.DEFAULT_BASE.read_bytes() + b"changed")
        self.refused(base=path, message="base descriptor hash")
        prior = self.root / "prior"
        shutil.copytree(PRIOR_SOURCE, prior)
        (prior / "0000.pb").write_bytes(b"changed")
        self.refused(prior=prior, message="descriptor bytes do not match")

    def test_changed_manifest_and_binary_identity_are_refused(self):
        source = self.copied_source()
        manifest = source / "manifest.json"
        data = json.loads(manifest.read_text())
        data["binary_sha256"] = "0" * 64
        manifest.write_text(json.dumps(data))
        self.refused(source, message="manifest hash")
        with mock.patch.object(generator, "MANIFEST_SHA256", hashlib.sha256(manifest.read_bytes()).hexdigest()):
            self.refused(source, message="does not identify the qualified binary")

    def test_changed_missing_and_extra_records_are_refused(self):
        source = self.copied_source()
        record = source / "0000.pb"
        original = record.read_bytes()
        record.write_bytes(original + b"\x00")
        self.refused(source, message="descriptor bytes do not match")
        record.unlink()
        self.refused(source, message="inventory does not match")
        record.write_bytes(original)
        (source / "extra.pb").write_bytes(original)
        self.refused(source, message="inventory does not match")

    def test_output_cannot_replace_pinned_base(self):
        with self.assertRaisesRegex(generator.GenerationError, "refusing to replace the pinned base"):
            generator.generate(PRIOR_SOURCE, SOURCE, generator.DEFAULT_BASE)

    def test_existing_target_presence_and_media_changes_are_refused(self):
        base = self.prior_bundle()
        files = generator.verified_source(SOURCE)
        augmented = generator.augment(base, files)
        with self.assertRaisesRegex(generator.GenerationError, "already contains an added message"):
            generator.augment(augmented, files)
        source = files[generator.prior.CORTEX_FILE]
        field = generator.one(generator.one(source.message_type, "CheckpointConfig").field, "context_window")
        field.proto3_optional = False
        with self.assertRaisesRegex(generator.GenerationError, "unexpected source field definition"):
            generator.augment(base, files)
        field.proto3_optional = True
        media = generator.one(files[generator.COMMON_FILE].message_type, "Media")
        media.field[0].number = 99
        with self.assertRaisesRegex(generator.GenerationError, "existing media declaration differs"):
            generator.augment(base, files)


if __name__ == "__main__":
    unittest.main()
