"""Verify the declarative overlay chain and tamper detection across all post-1.4.5 steps."""

from dataclasses import replace
import gzip
import hashlib
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

try:
    from google.protobuf import descriptor_pb2
except ImportError:
    descriptor_pb2 = None

REPO = Path(__file__).resolve().parents[1]
COMPLETENESS_DIR = REPO / "internal/completeness"
if str(COMPLETENESS_DIR) not in sys.path:
    sys.path.insert(0, str(COMPLETENESS_DIR))

import apply_overlays  # noqa: E402


EXPECTED_MODULES = (
    "generate_options145",
    "generate_compaction146",
    "generate_finish147",
    "generate_subagents148",
    "generate_media149",
    "generate_parentmsgs1410",
    "generate_config150",
    "generate_config152",
    "generate_tools153",
    "generate_tools156",
    "generate_config157",
    "generate_planexecute157",
    "generate_tools158",
    "generate_workflow159",
    "generate_schedule1510",
    "generate_policy1510",
    "generate_refs1511",
    "generate_features1513",
)


class OverlayChainMetadataTests(unittest.TestCase):
    def test_declarative_table_covers_ordered_chain(self):
        self.assertEqual(
            tuple(step.module_name for step in apply_overlays.OVERLAY_CHAIN),
            EXPECTED_MODULES,
        )
        apply_overlays.validate_chain_definition()

    def test_pinned_base_and_production_digests(self):
        base_gz = apply_overlays.DEFAULT_BASE.read_bytes()
        prod_gz = apply_overlays.DEFAULT_PRODUCTION.read_bytes()
        self.assertEqual(len(base_gz), apply_overlays.BASE_COMPRESSED_BYTES)
        self.assertEqual(apply_overlays.sha256_hex(base_gz), apply_overlays.BASE_COMPRESSED_SHA256)
        self.assertEqual(
            apply_overlays.sha256_hex(gzip.decompress(base_gz)),
            apply_overlays.BASE_RAW_SHA256,
        )
        final_step = apply_overlays.OVERLAY_CHAIN[-1]
        self.assertEqual(len(prod_gz), final_step.compressed_bytes)
        self.assertEqual(apply_overlays.sha256_hex(prod_gz), final_step.compressed_sha256)
        self.assertEqual(
            apply_overlays.sha256_hex(gzip.decompress(prod_gz)),
            final_step.raw_sha256,
        )

    def test_all_snapshot_inputs_verify_and_tampering_is_refused(self):
        for step in apply_overlays.OVERLAY_CHAIN:
            with self.subTest(step=step.name):
                verified = apply_overlays.verify_snapshot(step, repo_root=REPO)
                self.assertTrue(verified.exists())
                with tempfile.TemporaryDirectory() as directory:
                    tampered_path = Path(directory) / "tampered"
                    if step.snapshot_kind == "extracted":
                        shutil.copytree(verified, tampered_path)
                        first_pb = tampered_path / "0000.pb"
                        first_pb.write_bytes(first_pb.read_bytes() + b"\x00")
                    else:
                        tampered_path.write_bytes(verified.read_bytes() + b"\x00")
                    with self.assertRaises(apply_overlays.GenerationError):
                        apply_overlays.verify_snapshot(
                            step, repo_root=REPO, snapshot_override=tampered_path
                        )


@unittest.skipIf(descriptor_pb2 is None, "google.protobuf is not installed")
class ApplyOverlaysChainTests(unittest.TestCase):
    def test_verify_chain_from_production_reproduces_exact_gzip(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "rebuilt.pb.gz"
            summary = apply_overlays.verify_chain_from_production(output_path=output)
            self.assertEqual(summary["steps_verified"], len(EXPECTED_MODULES))
            self.assertTrue(summary["raw_match"])
            self.assertTrue(summary["compressed_match"])
            expected_gz = apply_overlays.DEFAULT_PRODUCTION.read_bytes()
            self.assertEqual(output.read_bytes(), expected_gz)
            self.assertEqual(
                apply_overlays.sha256_hex(gzip.decompress(expected_gz)),
                summary["rebuilt_raw_sha256"],
            )
            self.assertEqual(
                apply_overlays.sha256_hex(expected_gz),
                summary["rebuilt_compressed_sha256"],
            )
            self.assertFalse(list(Path(directory).glob(".schema-*")))

    def test_refuses_to_overwrite_authenticated_inputs(self):
        protected = [
            apply_overlays.DEFAULT_PRODUCTION,
            apply_overlays.DEFAULT_BASE,
        ]
        for step in apply_overlays.OVERLAY_CHAIN:
            snap = REPO / step.snapshot_path
            protected.append(snap)
            if step.snapshot_kind == "extracted":
                protected.extend([snap / "manifest.json", snap / "0000.pb"])
        for path in protected:
            with self.subTest(path=path):
                with self.assertRaisesRegex(apply_overlays.GenerationError, "authenticated input"):
                    apply_overlays.verify_chain_from_production(output_path=path)

        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            # Overridden protoset and overridden extracted record must also be protected.
            protoset_copy = root / "override.protoset"
            shutil.copy2(REPO / apply_overlays.OVERLAY_CHAIN[2].snapshot_path, protoset_copy)
            saved_proto = protoset_copy.read_bytes()
            with self.assertRaisesRegex(apply_overlays.GenerationError, "authenticated input"):
                apply_overlays.verify_chain_from_production(
                    output_path=protoset_copy,
                    snapshot_overrides={apply_overlays.OVERLAY_CHAIN[2].name: protoset_copy},
                )
            self.assertEqual(protoset_copy.read_bytes(), saved_proto)

            extracted_copy = root / "extracted-145"
            shutil.copytree(REPO / apply_overlays.OVERLAY_CHAIN[0].snapshot_path, extracted_copy)
            record = extracted_copy / "0000.pb"
            saved_record = record.read_bytes()
            with self.assertRaisesRegex(apply_overlays.GenerationError, "authenticated input"):
                apply_overlays.verify_chain_from_production(
                    output_path=record,
                    snapshot_overrides={apply_overlays.OVERLAY_CHAIN[0].name: extracted_copy},
                )
            self.assertEqual(record.read_bytes(), saved_record)

    def test_refuses_to_overwrite_symlinked_snapshot_records(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            snapshot = root / "snapshot"
            shutil.copytree(REPO / apply_overlays.OVERLAY_CHAIN[0].snapshot_path, snapshot)
            external_record = root / "record.pb"
            symlink_record = snapshot / "0000.pb"
            saved = symlink_record.read_bytes()
            shutil.move(symlink_record, external_record)
            symlink_record.symlink_to(external_record)

            # Writing to the symlink inside the snapshot dir must be refused.
            with self.assertRaisesRegex(apply_overlays.GenerationError, "authenticated input"):
                apply_overlays.verify_chain_from_production(
                    output_path=symlink_record,
                    snapshot_overrides={"options145": snapshot},
                )
            self.assertTrue(symlink_record.is_symlink())
            self.assertEqual(external_record.read_bytes(), saved)

            # Writing directly to the external symlink target must also be refused.
            with self.assertRaisesRegex(apply_overlays.GenerationError, "authenticated input"):
                apply_overlays.verify_chain_from_production(
                    output_path=external_record,
                    snapshot_overrides={"options145": snapshot},
                )
            self.assertEqual(external_record.read_bytes(), saved)

            # Writing via an external symlink pointing to a protected bundle must be refused.
            link_to_prod = root / "prod-link.pb.gz"
            link_to_prod.symlink_to(apply_overlays.DEFAULT_PRODUCTION)
            with self.assertRaisesRegex(apply_overlays.GenerationError, "authenticated input"):
                apply_overlays.verify_chain_from_production(output_path=link_to_prod)
            self.assertTrue(link_to_prod.is_symlink())

    def test_overridden_unreadable_original_snapshot_does_not_block_output(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            alt_repo = root / "alt-repo"
            for step in apply_overlays.OVERLAY_CHAIN:
                src = REPO / step.snapshot_path
                dst = alt_repo / step.snapshot_path
                dst.parent.mkdir(parents=True, exist_ok=True)
                if step.snapshot_kind == "extracted":
                    shutil.copytree(src, dst, dirs_exist_ok=True)
                else:
                    shutil.copy2(src, dst)
            override = root / "readable-options145"
            shutil.copytree(REPO / apply_overlays.OVERLAY_CHAIN[0].snapshot_path, override)
            unused_original = alt_repo / apply_overlays.OVERLAY_CHAIN[0].snapshot_path
            unused_original.chmod(0o111)
            output = root / "rebuilt.pb.gz"
            try:
                summary = apply_overlays.verify_chain_from_production(
                    repo_root=alt_repo,
                    output_path=output,
                    snapshot_overrides={"options145": override},
                )
            finally:
                unused_original.chmod(0o755)
            self.assertTrue(summary["raw_match"])
            self.assertEqual(
                gzip.decompress(output.read_bytes()),
                gzip.decompress(apply_overlays.DEFAULT_PRODUCTION.read_bytes()),
            )

    def test_snapshot_override_and_alternate_repo_root_work_for_all_steps(self):
        import generate_tools153

        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            alt_repo = root / "alt-repo"
            tools153_override = root / "relocated-tools153.protoset"
            for step in apply_overlays.OVERLAY_CHAIN:
                src = REPO / step.snapshot_path
                if step.name == "tools153":
                    shutil.copy2(src, tools153_override)
                    continue
                dst = alt_repo / step.snapshot_path
                dst.parent.mkdir(parents=True, exist_ok=True)
                if step.snapshot_kind == "extracted":
                    shutil.copytree(src, dst, dirs_exist_ok=True)
                else:
                    shutil.copy2(src, dst)
            output = root / "rebuilt.pb.gz"
            saved_snapshot = generate_tools153.SNAPSHOT
            generate_tools153.SNAPSHOT = root / "missing-canonical.protoset"
            try:
                summary = apply_overlays.verify_chain_from_production(
                    repo_root=alt_repo,
                    output_path=output,
                    snapshot_overrides={"tools153": tools153_override},
                )
            finally:
                generate_tools153.SNAPSHOT = saved_snapshot
            self.assertTrue(summary["raw_match"])
            self.assertEqual(
                gzip.decompress(output.read_bytes()),
                gzip.decompress(apply_overlays.DEFAULT_PRODUCTION.read_bytes()),
            )

    def test_overridden_symlinked_original_record_remains_protected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            alt_repo = root / "alt-repo"
            for step in apply_overlays.OVERLAY_CHAIN:
                src = REPO / step.snapshot_path
                dst = alt_repo / step.snapshot_path
                dst.parent.mkdir(parents=True, exist_ok=True)
                if step.snapshot_kind == "extracted":
                    shutil.copytree(src, dst, dirs_exist_ok=True)
                else:
                    shutil.copy2(src, dst)
            shared_record = root / "shared-record.pb"
            canonical_record = alt_repo / apply_overlays.OVERLAY_CHAIN[0].snapshot_path / "0000.pb"
            saved = canonical_record.read_bytes()
            shutil.move(canonical_record, shared_record)
            canonical_record.symlink_to(shared_record)

            override = root / "override"
            shutil.copytree(REPO / apply_overlays.OVERLAY_CHAIN[0].snapshot_path, override)
            with self.assertRaisesRegex(apply_overlays.GenerationError, "authenticated input"):
                apply_overlays.verify_chain_from_production(
                    repo_root=alt_repo,
                    snapshot_overrides={"options145": override},
                    output_path=shared_record,
                )
            self.assertEqual(shared_record.read_bytes(), saved)

    def test_overridden_circular_symlink_in_original_snapshot_does_not_block_output(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            alt_repo = root / "alt-repo"
            for step in apply_overlays.OVERLAY_CHAIN:
                src = REPO / step.snapshot_path
                dst = alt_repo / step.snapshot_path
                dst.parent.mkdir(parents=True, exist_ok=True)
                if step.snapshot_kind == "extracted":
                    shutil.copytree(src, dst, dirs_exist_ok=True)
                else:
                    shutil.copy2(src, dst)
            circular = alt_repo / apply_overlays.OVERLAY_CHAIN[0].snapshot_path / "0000.pb"
            circular.unlink()
            circular.symlink_to("0000.pb")

            override = root / "override"
            shutil.copytree(REPO / apply_overlays.OVERLAY_CHAIN[0].snapshot_path, override)
            output = root / "rebuilt.pb.gz"

            orig_resolve = Path.resolve

            def py312_resolve(path_self, strict=False):
                if path_self == circular:
                    raise RuntimeError(f"Symlink loop from {path_self}")
                return orig_resolve(path_self, strict=strict)

            with mock.patch.object(Path, "resolve", py312_resolve):
                summary = apply_overlays.verify_chain_from_production(
                    repo_root=alt_repo,
                    output_path=output,
                    snapshot_overrides={"options145": override},
                )
            self.assertTrue(summary["raw_match"])
            self.assertEqual(
                gzip.decompress(output.read_bytes()),
                gzip.decompress(apply_overlays.DEFAULT_PRODUCTION.read_bytes()),
            )

    def test_tamper_detection_snapshot_at_every_step(self):
        prod = descriptor_pb2.FileDescriptorSet.FromString(
            gzip.decompress(apply_overlays.DEFAULT_PRODUCTION.read_bytes())
        )
        base = apply_overlays.strip_chain(prod)
        for step in apply_overlays.OVERLAY_CHAIN:
            with self.subTest(step=step.name):
                with tempfile.TemporaryDirectory() as directory:
                    root = Path(directory)
                    output = root / "output.pb.gz"
                    output.write_bytes(b"untouched")
                    tampered = root / "tampered-snapshot"
                    src = REPO / step.snapshot_path
                    if step.snapshot_kind == "extracted":
                        shutil.copytree(src, tampered)
                        manifest = tampered / "manifest.json"
                        manifest.write_bytes(manifest.read_bytes() + b" ")
                    else:
                        tampered.write_bytes(src.read_bytes() + b"tampered")
                    with self.assertRaises(apply_overlays.GenerationError):
                        apply_overlays.verify_chain_from_production(
                            output_path=output,
                            snapshot_overrides={step.name: tampered},
                        )
                    self.assertEqual(output.read_bytes(), b"untouched")
                    with self.assertRaises(apply_overlays.GenerationError):
                        apply_overlays.apply_chain(
                            base,
                            snapshot_overrides={step.name: tampered},
                        )
                    self.assertFalse(list(root.glob(".schema-*")))

    def test_tamper_detection_hashes_and_intermediates_at_every_step(self):
        prod = descriptor_pb2.FileDescriptorSet.FromString(
            gzip.decompress(apply_overlays.DEFAULT_PRODUCTION.read_bytes())
        )
        chain = apply_overlays.OVERLAY_CHAIN
        # Walk backwards once to collect every authentic intermediate bundle.
        intermediates = {}
        current = prod
        for step in reversed(chain):
            intermediates[step.name] = current
            current = apply_overlays.strip_step(step, current)
        base = current

        bad_digest = "0" * 64
        for idx, step in enumerate(chain):
            with self.subTest(step=step.name):
                # 1. Tampered expected raw_sha256 in declarative table fails verification.
                bad_chain = list(chain)
                bad_chain[idx] = replace(step, raw_sha256=bad_digest)
                with self.assertRaises(apply_overlays.GenerationError):
                    apply_overlays.verify_chain_from_production(chain=tuple(bad_chain))

                # 2. Tampered expected base_raw_sha256 fails chain validation.
                bad_base_chain = list(chain)
                bad_base_chain[idx] = replace(step, base_raw_sha256=bad_digest)
                with self.assertRaises(apply_overlays.GenerationError):
                    apply_overlays.verify_chain_from_production(chain=tuple(bad_base_chain))

                # 3. Mutated intermediate bundle is rejected at both strip_step and apply_step.
                step_output = descriptor_pb2.FileDescriptorSet()
                step_output.CopyFrom(intermediates[step.name])
                step_output.file[0].name = "tampered.proto"
                with self.assertRaisesRegex(apply_overlays.GenerationError, "strip input hash mismatch"):
                    apply_overlays.strip_step(step, step_output)

                prior_bundle = base if idx == 0 else intermediates[chain[idx - 1].name]
                for field, value in (
                    ("compressed_sha256", bad_digest),
                    ("compressed_bytes", step.compressed_bytes + 1),
                ):
                    with self.subTest(pin=field):
                        with self.assertRaisesRegex(
                            apply_overlays.GenerationError, f"{step.name}: compressed output pin mismatch"
                        ):
                            apply_overlays.apply_step(replace(step, **{field: value}), prior_bundle)

                bad_prior = descriptor_pb2.FileDescriptorSet()
                bad_prior.CopyFrom(prior_bundle)
                bad_prior.file[0].name = "tampered.proto"
                with self.assertRaisesRegex(apply_overlays.GenerationError, "base hash mismatch"):
                    apply_overlays.apply_step(step, bad_prior)

    def test_refuses_to_publish_when_intermediate_gzip_pin_differs(self):
        chain = apply_overlays.OVERLAY_CHAIN
        step = chain[1]
        # A different header preserves the raw descriptor and gzip length.
        bad_step = replace(step, gzip_os=3)
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "output.pb.gz"
            output.write_bytes(b"untouched")
            with self.assertRaisesRegex(
                apply_overlays.GenerationError, f"{step.name}: compressed output pin mismatch"
            ):
                apply_overlays.verify_chain_from_production(
                    chain=(chain[0], bad_step, *chain[2:]), output_path=output
                )
            self.assertEqual(output.read_bytes(), b"untouched")
            self.assertFalse(list(Path(directory).glob(".schema-*")))

    def test_tamper_detection_production_and_base_files(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bad_prod = root / "bad-prod.pb.gz"
            bad_prod.write_bytes(apply_overlays.DEFAULT_PRODUCTION.read_bytes() + b"x")
            with self.assertRaisesRegex(apply_overlays.GenerationError, "production compressed hash mismatch"):
                apply_overlays.verify_chain_from_production(production_path=bad_prod)

            bad_base = root / "bad-base.pb.gz"
            bad_base.write_bytes(apply_overlays.DEFAULT_BASE.read_bytes() + b"x")
            with self.assertRaisesRegex(apply_overlays.GenerationError, "base compressed hash mismatch"):
                apply_overlays.verify_chain_from_production(base_path=bad_base)

    def test_cli_verify_and_output(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            output = root / "schema.pb.gz"
            res = subprocess.run(
                [sys.executable, "-B", str(COMPLETENESS_DIR / "apply_overlays.py"), "--output", str(output)],
                capture_output=True,
                text=True,
                check=True,
            )
            self.assertIn(f"verified {len(EXPECTED_MODULES)} overlays", res.stdout)
            self.assertIn("compressed_match=True", res.stdout)
            self.assertEqual(output.read_bytes(), apply_overlays.DEFAULT_PRODUCTION.read_bytes())
            bad = root / "bad.pb.gz"
            bad.write_bytes(gzip.compress(b"not-valid", mtime=0))
            fail = subprocess.run(
                [sys.executable, "-B", str(COMPLETENESS_DIR / "apply_overlays.py"), "--production", str(bad)],
                capture_output=True,
                text=True,
            )
            self.assertNotEqual(fail.returncode, 0)
            self.assertIn("refused:", fail.stderr)


if __name__ == "__main__":
    unittest.main()
