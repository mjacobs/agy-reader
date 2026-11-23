"""Declarative overlay chain and round-trip verifier for internal/completeness/schema.pb.gz.

Lists every post-1.4.5 overlay generator in order, pins its snapshot path/hash
and expected intermediate/final uncompressed and compressed SHA-256 digests, and
proves exact reproduction by stripping schema.pb.gz back to the 1.3.4 base and
re-applying all overlays forward. Requires google.protobuf for verification.
"""

from __future__ import annotations

import argparse
from dataclasses import dataclass
import gzip
import hashlib
import importlib
import json
from pathlib import Path
import sys
import tempfile
import threading
import zlib

try:
    from google.protobuf import descriptor_pb2
except ImportError:
    descriptor_pb2 = None

COMPLETENESS_DIR = Path(__file__).resolve().parent
REPO_ROOT = COMPLETENESS_DIR.parents[1]

if descriptor_pb2 is not None:
    if str(COMPLETENESS_DIR) not in sys.path:
        sys.path.insert(0, str(COMPLETENESS_DIR))
    import generate_options145 as _helpers

    GenerationError = _helpers.GenerationError
else:
    _helpers = None

    class GenerationError(ValueError):
        pass


DEFAULT_BASE = COMPLETENESS_DIR / "schema-base-agy134.pb.gz"
DEFAULT_PRODUCTION = COMPLETENESS_DIR / "schema.pb.gz"

BASE_COMPRESSED_SHA256 = "90bd669c32e044ef769763f3e4dc5b27fb187972cc9ff518189ff3933ed8fd92"
BASE_COMPRESSED_BYTES = 137394
BASE_RAW_SHA256 = "53885fee2728fe79ff9d4931bf19004f42f85521a93281f73a6a625f84e82777"

CORTEX_FILE = "third_party/jetski/cortex_pb/cortex.proto"
TOOLS_FILE = "third_party/jetski/cortex/core/tools_pb/tools.proto"
CONTENT_FILE = "learning/genai/api/interactions/proto/content.proto"
_TOOLS153_LOCK = threading.Lock()


@dataclass(frozen=True)
class OverlayStep:
    """Declarative specification for one sequential descriptor overlay step."""

    name: str
    module_name: str
    snapshot_path: str
    snapshot_kind: str
    snapshot_sha256: str
    binary_sha256: str | None
    base_raw_sha256: str
    raw_sha256: str
    compressed_sha256: str
    compressed_bytes: int
    gzip_os: int = 255  # Gzip header OS byte: 255 (unknown) or 3 (Unix).


OVERLAY_CHAIN: tuple[OverlayStep, ...] = (
    OverlayStep(
        name="options145",
        module_name="generate_options145",
        snapshot_path="artifacts/proto-snapshots/agy-1.4.5-8af8e7a2a68f/extracted",
        snapshot_kind="extracted",
        snapshot_sha256="5a3fc4d99ca4276ac654e2786a427ef80286f8d510476a7d7a6b3a53e47c12f1",
        binary_sha256="8af8e7a2a68f72fa7164e00bb59b8902c018fa43e528fc401ad7db8c2972583c",
        base_raw_sha256=BASE_RAW_SHA256,
        raw_sha256="0c1b792443534ba6d8cb160c09583eedeb1798c2273308f7f3bc9387bdec5a9e",
        compressed_sha256="941e3c1eb080db629d2d627b15c7e268d076ed7f43acb9578bac82b0e5ee3aed",
        compressed_bytes=137843,
    ),
    OverlayStep(
        name="compaction146",
        module_name="generate_compaction146",
        snapshot_path="artifacts/proto-snapshots/agy-1.4.6-128780f57769/extracted",
        snapshot_kind="extracted",
        snapshot_sha256="3c44a5d8a5e2bc951c1f45d6eebabef811f08bcf8ca2e7d551bef3a7c2f377e0",
        binary_sha256="128780f5776959ba6a991bde1f0a8b699eee37a59ed30deb9926147fedfa56c4",
        base_raw_sha256="0c1b792443534ba6d8cb160c09583eedeb1798c2273308f7f3bc9387bdec5a9e",
        raw_sha256="65e19ac6a3b0b51971f1c7a02c16d0ff53c3969f874ed9a3930d1e6587a8b8b0",
        compressed_sha256="3d9a1ebb2ae55b316ab35cf19875f50d9f9c568ff16d8a04b95c90703ef026fa",
        compressed_bytes=137988,
    ),
    OverlayStep(
        name="finish147",
        module_name="generate_finish147",
        snapshot_path="artifacts/proto-snapshots/agy-1.4.7-40349135371a/all-extracted.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="b3b02aee80c64f17550fde3aa0a998c2e3eb41fb6c5a6eb7e14e0b2cb4fe4ec2",
        binary_sha256=None,
        base_raw_sha256="65e19ac6a3b0b51971f1c7a02c16d0ff53c3969f874ed9a3930d1e6587a8b8b0",
        raw_sha256="5e3c6914bd036d1a8d1ce96fd3566e78d57e0350696b01fa903d4e3d0dd6919a",
        compressed_sha256="12b303d5dc3cd50630f82a48a3e1616863067b7448a6283b241a55c1434b97b9",
        compressed_bytes=138024,
    ),
    OverlayStep(
        name="subagents148",
        module_name="generate_subagents148",
        snapshot_path="artifacts/proto-snapshots/agy-1.4.8-70b1d27f1812/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="964a14ddf6af95b2772cabc41ed25df7d913631779367a2e51c569f43020a133",
        binary_sha256=None,
        base_raw_sha256="5e3c6914bd036d1a8d1ce96fd3566e78d57e0350696b01fa903d4e3d0dd6919a",
        raw_sha256="ff8de137c541a33d2fb9a3a08c8d9e79d0d2035185f41caa13d09153a8042bd4",
        compressed_sha256="c0044f05838b41be3a0329070bf0e6e5ec07fcff49f27d8cdf97c96ce616dea8",
        compressed_bytes=138089,
    ),
    OverlayStep(
        name="media149",
        module_name="generate_media149",
        snapshot_path="artifacts/proto-snapshots/agy-1.4.9-1c9102d0a361/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="1db2f7b0fd919476541e2dc0ec7aadc9b3f7e56ba5f8cf26c06663ad1eaa0f68",
        binary_sha256=None,
        base_raw_sha256="ff8de137c541a33d2fb9a3a08c8d9e79d0d2035185f41caa13d09153a8042bd4",
        raw_sha256="5426f0d4e3cf1170f23c3f09808071e72cbc418aea1fdb5a7a63add6925e5902",
        compressed_sha256="ca24422544d1d5570edaa1528cb26b71bc488415b18ddefcc5c0e26b67431123",
        compressed_bytes=138147,
        gzip_os=3,
    ),
    OverlayStep(
        name="parentmsgs1410",
        module_name="generate_parentmsgs1410",
        snapshot_path="artifacts/proto-snapshots/agy-1.4.12-1edfb3047744/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="801110a449d02729f2a2ae23d466215e954839521e6b2b519ce4fc450ec1419e",
        binary_sha256=None,
        base_raw_sha256="5426f0d4e3cf1170f23c3f09808071e72cbc418aea1fdb5a7a63add6925e5902",
        raw_sha256="33432f5ae6f2f58fc910fe647e220b0fd25a33191720bb8c22fc126fd01b400c",
        compressed_sha256="675f67f23a03f46f183a2df72d8321d2083b994d22e54865b69552265880330c",
        compressed_bytes=138179,
        gzip_os=3,
    ),
    OverlayStep(
        name="config150",
        module_name="generate_config150",
        snapshot_path="artifacts/proto-snapshots/agy-1.5.0-b9043860a06a/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="817e08196b0d1e8bc5c206153e1755722eed24d69c6658fcacd6e066e4614bf8",
        binary_sha256=None,
        base_raw_sha256="33432f5ae6f2f58fc910fe647e220b0fd25a33191720bb8c22fc126fd01b400c",
        raw_sha256="8579717d67c37459b6f61b0815650ae18412f5afbf522cdf81b9604ebe06bdd7",
        compressed_sha256="c812976330495d3df357080872b420aaf21bd7b3c987d5ff77e8060ad650d0d7",
        compressed_bytes=138407,
        gzip_os=3,
    ),
    OverlayStep(
        name="config152",
        module_name="generate_config152",
        snapshot_path="artifacts/proto-snapshots/agy-1.5.2-da1751e76584/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="2a61655fb6cd313634b55a3f7b04eef86b2f64dcd15df03baafeb33da1af2c3d",
        binary_sha256=None,
        base_raw_sha256="8579717d67c37459b6f61b0815650ae18412f5afbf522cdf81b9604ebe06bdd7",
        raw_sha256="55ef33aef1b9dbe728ce698bf1db0088fd78016c9c526d9edb0dfe5aa0ce19d4",
        compressed_sha256="13eeb9592877fc93b33b098abcd7737b963d48eb9c544b0ee478862e2b7a9831",
        compressed_bytes=138479,
        gzip_os=3,
    ),
    OverlayStep(
        name="tools153",
        module_name="generate_tools153",
        snapshot_path="artifacts/proto-snapshots/agy-1.5.3-7ca212748ca0/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="af90876809de9cbb670c7e8d3dd973aaa81542698c114480c7c1a8a1fb23f1e2",
        binary_sha256=None,
        base_raw_sha256="55ef33aef1b9dbe728ce698bf1db0088fd78016c9c526d9edb0dfe5aa0ce19d4",
        raw_sha256="a7b3b707bfbe9da7c4961fe925211b338e71717ae3b8ef0cd0224a29fd4a96c6",
        compressed_sha256="a19794bc09715060f9e3d4e49f62d27e2295dce58b52fe8588f0dbf1601171dd",
        compressed_bytes=138499,
        gzip_os=3,
    ),
    OverlayStep(
        name="tools156",
        module_name="generate_tools156",
        snapshot_path="artifacts/proto-snapshots/agy-1.5.6-7e5bbdcbf689/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="e44e34f43926494831dd395d43d51c3afaf45e596ec71850fb980ef754880346",
        binary_sha256=None,
        base_raw_sha256="a7b3b707bfbe9da7c4961fe925211b338e71717ae3b8ef0cd0224a29fd4a96c6",
        raw_sha256="834fd630ac230fb84421e94ad773da9bbf6aa5f0cd277f3a7dc4343157a9818b",
        compressed_sha256="6055d1577da39c0c66115c18e4d9a96f2902fb41d597b91df75109774cb372f4",
        compressed_bytes=138520,
        gzip_os=3,
    ),
    OverlayStep(
        name="runtime157",
        module_name="generate_config157",
        snapshot_path="artifacts/proto-snapshots/agy-1.5.7-5df525a2d690/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="f3de51c7f2a7317d073cabcea13cd42a7f7f1adf3986c3bf5b3f07035b120d46",
        binary_sha256=None,
        base_raw_sha256="834fd630ac230fb84421e94ad773da9bbf6aa5f0cd277f3a7dc4343157a9818b",
        raw_sha256="e603b335cff88609a63b131cf92491c8af64f9c402e8282d967b060d61a6336a",
        compressed_sha256="2ce6f4cebef928bca35f94365bf9abaf14067fc9ab841bf6eed2d001b58f25ab",
        compressed_bytes=138560,
        gzip_os=3,
    ),
    OverlayStep(
        name="planexecute157",
        module_name="generate_planexecute157",
        snapshot_path="artifacts/proto-snapshots/agy-1.5.7-5df525a2d690/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="f3de51c7f2a7317d073cabcea13cd42a7f7f1adf3986c3bf5b3f07035b120d46",
        binary_sha256=None,
        base_raw_sha256="e603b335cff88609a63b131cf92491c8af64f9c402e8282d967b060d61a6336a",
        raw_sha256="f05ed42c8ceb5eea6cb003f354b1fffbe8f4169cdce2a297049849624bde4680",
        compressed_sha256="a98ccbf848a2ec82dd489484dfdbf9684c171014d366c6a1db8fbb3d41bff0ec",
        compressed_bytes=138615,
        gzip_os=3,
    ),
    OverlayStep(
        name="tools158",
        module_name="generate_tools158",
        snapshot_path="artifacts/proto-snapshots/agy-1.5.8-9db59b637110/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="82b4b9b0d3949c99528ab0d597bd2e82a0956cea9617c61b115ab8f996b66250",
        binary_sha256=None,
        base_raw_sha256="f05ed42c8ceb5eea6cb003f354b1fffbe8f4169cdce2a297049849624bde4680",
        raw_sha256="6708a33891214c32c2a4b85fab1c93e2974cc25e69038d488e369414f700f799",
        compressed_sha256="baa0efc42f14d2187a785414ebb348a5bc0763bf3c458d4ad5dc1acfc87910d1",
        compressed_bytes=138787,
        gzip_os=3,
    ),
    OverlayStep(
        name="workflow159",
        module_name="generate_workflow159",
        snapshot_path="artifacts/proto-snapshots/agy-1.5.9-39816d49d5ab/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="2134e03f83ba5d97a3fd8b5fa7574154be0cb71294e373a90a5cad3fd7e481d2",
        binary_sha256=None,
        base_raw_sha256="6708a33891214c32c2a4b85fab1c93e2974cc25e69038d488e369414f700f799",
        raw_sha256="bfcb3c2e926ced1cef32c2eb5bcf329554b9ad3d35183f16eba6749bb9a3ff6c",
        compressed_sha256="89c0877cfe610ac674038826080672e1a692be4c5d5cb0dd550495e9c575da3a",
        compressed_bytes=138790,
    ),
    OverlayStep(
        name="schedule1510",
        module_name="generate_schedule1510",
        snapshot_path="artifacts/proto-snapshots/agy-1.5.10-20d6f74672ce/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="5e2adf25951e87fe151b3fd097ab2c772990ecfb167f3b2025cab40119aae1b1",
        binary_sha256=None,
        base_raw_sha256="bfcb3c2e926ced1cef32c2eb5bcf329554b9ad3d35183f16eba6749bb9a3ff6c",
        raw_sha256="f142a51963d6af1df06072e4c35d92be4a75dce1c545eeb421a7d4837b9483a2",
        compressed_sha256="4740772b198b004b434a41b16f5dcead44445df336f5ff8cac23611521c093f3",
        compressed_bytes=138810,
        gzip_os=3,
    ),
    OverlayStep(
        name="policy1510",
        module_name="generate_policy1510",
        snapshot_path="artifacts/proto-snapshots/agy-1.5.10-20d6f74672ce/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="5e2adf25951e87fe151b3fd097ab2c772990ecfb167f3b2025cab40119aae1b1",
        binary_sha256=None,
        base_raw_sha256="f142a51963d6af1df06072e4c35d92be4a75dce1c545eeb421a7d4837b9483a2",
        raw_sha256="07b862d0d5e8a4ed511f2d3e4d703148f87e2b29c72d32d055259b0a7316fd11",
        compressed_sha256="d4d50760ceaff4e64e79c14d1b3f37ea8f5a8559574bbdddfd556c212e15fa23",
        compressed_bytes=138900,
        gzip_os=3,
    ),
    OverlayStep(
        name="refs1511",
        module_name="generate_refs1511",
        snapshot_path="artifacts/proto-snapshots/agy-1.5.11-01b063665007/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="f770d88ff74023cb4b38f9692c0d59774b87f48025200af392f2a185fdcded9d",
        binary_sha256=None,
        base_raw_sha256="07b862d0d5e8a4ed511f2d3e4d703148f87e2b29c72d32d055259b0a7316fd11",
        raw_sha256="c7f51417b9dc7b9b0d2dc1627ee49734d5c85d011e37f7e555c0e9eadf8ffae8",
        compressed_sha256="9b115da2ddbceb0a1de9df7ea1918b6ce237e1dd7717ff3da300cd7f36b9fcc6",
        compressed_bytes=138957,
        gzip_os=3,
    ),
    OverlayStep(
        name="features1513",
        module_name="generate_features1513",
        snapshot_path="artifacts/proto-snapshots/agy-1.5.13-c3fd2c848abf/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="e034e570f22bb6527431380dc430e0a740caa3996e1cc67480d5a28e59493875",
        binary_sha256=None,
        base_raw_sha256="c7f51417b9dc7b9b0d2dc1627ee49734d5c85d011e37f7e555c0e9eadf8ffae8",
        raw_sha256="053e01bf1e9a0f79424425b59661487a07c98d63504e2a4d65772b63357fa8d5",
        compressed_sha256="280549d833c482fd2f9c0de1b7fa6b6b552aea6a2f7efb56fa56393d56555248",
        compressed_bytes=139420,
        gzip_os=3,
    ),
    OverlayStep(
        name="config1514",
        module_name="generate_config1514",
        snapshot_path="artifacts/proto-snapshots/agy-1.5.14-37873b806239/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="a48906e588f470c2d37db003a5ed41a7a9989bfa7bdfb562d4b698f60af038dd",
        binary_sha256=None,
        base_raw_sha256="053e01bf1e9a0f79424425b59661487a07c98d63504e2a4d65772b63357fa8d5",
        raw_sha256="c2f1a393acdcec6b65a5f3e85a75182cd81c5ddbb32e5e65213b39a323b8fbf7",
        compressed_sha256="aa9e2945a59d93e87fd5fa67fbf696e81d03df5eadad58b72f1b75e768ae08f3",
        compressed_bytes=140116,
        gzip_os=3,
    ),
    OverlayStep(
        name="snapshot1515",
        module_name="generate_snapshot1515",
        snapshot_path="artifacts/proto-snapshots/agy-1.5.15-7b004d26c3fd/closure.protoset",
        snapshot_kind="protoset",
        snapshot_sha256="aeda8b78c29c4e9e28718cb4dfc7261270128edec8429ba933d9df0a970adc71",
        binary_sha256=None,
        base_raw_sha256="c2f1a393acdcec6b65a5f3e85a75182cd81c5ddbb32e5e65213b39a323b8fbf7",
        raw_sha256="8feb9424e0716c0800f5097e0b36eacb0c31451f64d3219334ae3f3e4ccf3f46",
        compressed_sha256="dbb8676695134c3b883d5537ebd7d3ad20609cdb48dbd3554958427d2588692b",
        compressed_bytes=140187,
        gzip_os=3,
    ),
)


def require(condition: bool, reason: str) -> None:
    if not condition:
        raise GenerationError(reason)


def _require_protobuf() -> None:
    require(descriptor_pb2 is not None, "google.protobuf is required to verify or apply overlays")


def sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def deterministic_bytes(bundle) -> bytes:
    return bundle.SerializeToString(deterministic=True)


def compress_deterministic(step: OverlayStep, raw: bytes) -> bytes:
    # zlib emits a gzip member with no name and mtime=0. Set the recorded
    # OS byte explicitly so Python and platform header defaults cannot change it.
    compressor = zlib.compressobj(level=9, wbits=31)
    compressed = compressor.compress(raw) + compressor.flush()
    return compressed[:9] + bytes([step.gzip_os]) + compressed[10:]


def validate_chain_definition(chain: tuple[OverlayStep, ...] = OVERLAY_CHAIN) -> None:
    require(bool(chain), "overlay chain must not be empty")
    require(chain[0].base_raw_sha256 == BASE_RAW_SHA256, "overlay chain does not start at pinned 1.3.4 base")
    for previous, current in zip(chain, chain[1:]):
        require(
            current.base_raw_sha256 == previous.raw_sha256,
            f"overlay chain discontinuity between {previous.name} and {current.name}",
        )


def verify_snapshot(
    step: OverlayStep,
    repo_root: Path = REPO_ROOT,
    snapshot_override: Path | None = None,
) -> Path:
    snapshot = snapshot_override if snapshot_override is not None else repo_root / step.snapshot_path
    if step.snapshot_kind == "extracted":
        manifest_path = snapshot / "manifest.json"
        require(manifest_path.is_file(), f"{step.name}: missing extracted manifest: {manifest_path}")
        manifest_bytes = manifest_path.read_bytes()
        require(
            sha256_hex(manifest_bytes) == step.snapshot_sha256,
            f"{step.name}: manifest snapshot hash mismatch",
        )
        manifest = json.loads(manifest_bytes)
        require(
            manifest.get("binary_sha256") == step.binary_sha256,
            f"{step.name}: binary identity mismatch in manifest",
        )
        entries = manifest["files"]
        expected_paths = {f"{i:04d}.pb" for i in range(len(entries))}
        require(
            {p.name for p in snapshot.glob("*.pb")} == expected_paths,
            f"{step.name}: extracted record inventory mismatch",
        )
        for i, entry in enumerate(entries):
            data = (snapshot / f"{i:04d}.pb").read_bytes()
            require(
                len(data) == entry["length"] and sha256_hex(data) == entry["sha256"],
                f"{step.name}: extracted record mismatch for {entry['name']}",
            )
        return snapshot
    if step.snapshot_kind == "protoset":
        require(snapshot.is_file(), f"{step.name}: missing snapshot protoset: {snapshot}")
        data = snapshot.read_bytes()
        require(
            sha256_hex(data) == step.snapshot_sha256,
            f"{step.name}: snapshot hash mismatch",
        )
        return snapshot
    raise GenerationError(f"{step.name}: unknown snapshot kind: {step.snapshot_kind}")


def _remove_optional_field(message, field_name: str) -> None:
    field = _helpers.one(message.field, field_name)
    require(field.HasField("oneof_index"), f"expected optional oneof on {message.name}.{field_name}")
    index = field.oneof_index
    declaration = message.oneof_decl[index]
    require(declaration.name == "_" + field_name, f"unexpected oneof name: {declaration.name}")
    message.field.remove(field)
    del message.oneof_decl[index]
    for other in message.field:
        if other.HasField("oneof_index"):
            require(other.oneof_index != index, f"shared oneof index on {message.name}.{other.name}")
            if other.oneof_index > index:
                other.oneof_index -= 1


def _strip_options145(bundle) -> None:
    cortex = _helpers.one(bundle.file, CORTEX_FILE)
    _remove_optional_field(_helpers.one(cortex.message_type, "AntigravityBrowserToolConfig"), "enable_browser_subagent_auto_invoke")
    _remove_optional_field(_helpers.one(cortex.message_type, "ViewFileToolConfig"), "skip_plugin_logo")
    output = _helpers.one(cortex.message_type, "ToolOutputConfig")
    output.field.remove(_helpers.one(output.field, "per_tool"))
    output.nested_type.remove(_helpers.one(output.nested_type, "PerToolEntry"))
    cortex.message_type.remove(_helpers.one(cortex.message_type, "ToolOutputOverride"))


def _strip_compaction146(bundle) -> None:
    cortex = _helpers.one(bundle.file, CORTEX_FILE)
    discovery = _helpers.one(cortex.message_type, "CustomizationDiscoveryConfig")
    discovery.field.remove(_helpers.one(discovery.field, "kind_enablement"))
    discovery.nested_type.remove(_helpers.one(discovery.nested_type, "KindEnablementEntry"))
    config = _helpers.one(cortex.message_type, "CheckpointConfig")
    _remove_optional_field(config, "max_user_request_media_tokens")
    _remove_optional_field(config, "context_window")
    cortex.message_type.remove(_helpers.one(cortex.message_type, "CapturedUserRequest"))
    checkpoint = _helpers.one(cortex.message_type, "CortexStepCheckpoint")
    checkpoint.field.remove(_helpers.one(checkpoint.field, "captured_user_requests"))


def _strip_finish147(bundle) -> None:
    tools = _helpers.one(bundle.file, TOOLS_FILE)
    tools.message_type.remove(_helpers.one(tools.message_type, "Finish"))


def _strip_subagents148(bundle) -> None:
    tools = _helpers.one(bundle.file, TOOLS_FILE)
    manage = _helpers.one(tools.message_type, "ManageSubagents")
    manage.field.remove(_helpers.one(manage.field, "subagents"))
    manage.nested_type.remove(_helpers.one(manage.nested_type, "SubagentState"))


def _strip_media149(bundle) -> None:
    cortex = _helpers.one(bundle.file, CORTEX_FILE)
    require(CONTENT_FILE in cortex.dependency, f"missing expected dependency: {CONTENT_FILE}")
    cortex.dependency.remove(CONTENT_FILE)
    result = _helpers.one(cortex.message_type, "GenericStepResult")
    result.field.remove(_helpers.one(result.field, "result_value"))
    response = _helpers.one(cortex.message_type, "CortexStepPlannerResponse")
    response.field.remove(_helpers.one(response.field, "media"))


def _strip_parentmsgs1410(bundle) -> None:
    cortex = _helpers.one(bundle.file, CORTEX_FILE)
    checkpoint = _helpers.one(cortex.message_type, "CortexStepCheckpoint")
    checkpoint.field.remove(_helpers.one(checkpoint.field, "captured_parent_messages"))


def _strip_config150(bundle) -> None:
    cortex = _helpers.one(bundle.file, CORTEX_FILE)
    invoke = _helpers.one(cortex.message_type, "InvokeSubagentToolConfig")
    invoke.field.remove(_helpers.one(invoke.field, "model_options"))
    _remove_optional_field(_helpers.one(cortex.message_type, "AntigravityBrowserToolConfig"), "enable_browser_subagent_webview")
    cascade = _helpers.one(cortex.message_type, "CascadeConfig")
    cascade.field.remove(_helpers.one(cascade.field, "model_tiers"))
    for name in ("ModelTierConfig", "SubagentModelOptions", "SubagentModelOption", "ModelCatalogKey"):
        cortex.message_type.remove(_helpers.one(cortex.message_type, name))


def _strip_config152(bundle) -> None:
    cortex = _helpers.one(bundle.file, CORTEX_FILE)
    option = _helpers.one(cortex.message_type, "SubagentModelOption")
    for name in ("named_model", "inherit"):
        option.field.remove(_helpers.one(option.field, name))
    option.oneof_decl.remove(_helpers.one(option.oneof_decl, "kind"))
    option.nested_type.remove(_helpers.one(option.nested_type, "NamedModel"))


def _strip_tools153(bundle) -> None:
    cortex = _helpers.one(bundle.file, CORTEX_FILE)
    spec = _helpers.one(cortex.message_type, "SubagentSpec")
    spec.field.remove(_helpers.one(spec.field, "model_option"))


def _strip_tools156(bundle) -> None:
    cortex = _helpers.one(bundle.file, CORTEX_FILE)
    enum = _helpers.one(cortex.enum_type, "ExecutorTerminationReason")
    enum.value.remove(_helpers.one(enum.value, "EXECUTOR_TERMINATION_REASON_CHAINED"))


def _strip_runtime157(bundle) -> None:
    cortex = _helpers.one(bundle.file, CORTEX_FILE)
    prompt = _helpers.one(cortex.message_type, "PromptSectionCustomizationConfig")
    prompt.field.remove(_helpers.one(prompt.field, "template_overrides"))
    prompt.nested_type.remove(_helpers.one(prompt.nested_type, "TemplateOverridesEntry"))


def _strip_planexecute157(bundle) -> None:
    cortex = _helpers.one(bundle.file, CORTEX_FILE)
    planner = _helpers.one(cortex.message_type, "CascadePlannerConfig")
    planner.field.remove(_helpers.one(planner.field, "plan_execute_config"))
    cortex.message_type.remove(_helpers.one(cortex.message_type, "PlanExecuteConfig"))


_STRIPPERS = {
    "options145": _strip_options145,
    "compaction146": _strip_compaction146,
    "finish147": _strip_finish147,
    "subagents148": _strip_subagents148,
    "media149": _strip_media149,
    "parentmsgs1410": _strip_parentmsgs1410,
    "config150": _strip_config150,
    "config152": _strip_config152,
    "tools153": _strip_tools153,
    "tools156": _strip_tools156,
    "runtime157": _strip_runtime157,
    "planexecute157": _strip_planexecute157,
}


def strip_step(step: OverlayStep, bundle):
    """Strip a single overlay step backward and verify input/output raw hashes."""
    _require_protobuf()
    actual_in = sha256_hex(deterministic_bytes(bundle))
    require(actual_in == step.raw_sha256, f"{step.name}: strip input hash mismatch")
    stripped = descriptor_pb2.FileDescriptorSet()
    stripped.CopyFrom(bundle)
    if step.name in ("tools158", "workflow159", "schedule1510", "policy1510", "refs1511", "features1513", "config1514", "snapshot1515"):
        module = importlib.import_module(step.module_name)
        stripped = module.strip(stripped)
    elif step.name in _STRIPPERS:
        _STRIPPERS[step.name](stripped)
    else:
        raise GenerationError(f"unknown overlay step: {step.name}")
    _helpers.load_pool(stripped)
    actual_out = sha256_hex(deterministic_bytes(stripped))
    require(actual_out == step.base_raw_sha256, f"{step.name}: stripped base hash mismatch")
    return stripped


def apply_step(
    step: OverlayStep,
    bundle,
    *,
    repo_root: Path = REPO_ROOT,
    snapshot_override: Path | None = None,
):
    """Authenticate the snapshot and verify both descriptor and gzip output pins."""
    _require_protobuf()
    actual_in = sha256_hex(deterministic_bytes(bundle))
    require(actual_in == step.base_raw_sha256, f"{step.name}: base hash mismatch")
    snapshot = verify_snapshot(step, repo_root=repo_root, snapshot_override=snapshot_override)
    module = importlib.import_module(step.module_name)

    if step.module_name == "generate_options145":
        manifest = json.loads((snapshot / "manifest.json").read_text())
        files = {}
        for i, entry in enumerate(manifest["files"]):
            record = descriptor_pb2.FileDescriptorProto.FromString((snapshot / f"{i:04d}.pb").read_bytes())
            require(record.name == entry["name"] and record.name not in files, f"{step.name}: bad record {entry['name']}")
            record.DiscardUnknownFields()
            files[record.name] = record
        result = module.augment(bundle, files[CORTEX_FILE])
    elif step.module_name == "generate_compaction146":
        result = module.augment(bundle, module.verified_source(snapshot))
    elif step.module_name == "generate_tools153":
        # generate_tools153.augment reads module.SNAPSHOT directly; bind it to the
        # authenticated snapshot path for the call so repo_root and snapshot_override work.
        with _TOOLS153_LOCK:
            original_snapshot = module.SNAPSHOT
            module.SNAPSHOT = snapshot
            try:
                result = module.augment(bundle)
            finally:
                module.SNAPSHOT = original_snapshot
    else:
        result = module.augment(bundle, snapshot)

    raw = deterministic_bytes(result)
    require(sha256_hex(raw) == step.raw_sha256, f"{step.name}: output hash mismatch")
    compressed = compress_deterministic(step, raw)
    digest = sha256_hex(compressed)
    require(
        digest == step.compressed_sha256 and len(compressed) == step.compressed_bytes,
        f"{step.name}: compressed output pin mismatch with zlib {zlib.ZLIB_RUNTIME_VERSION}: "
        f"expected {step.compressed_bytes} bytes and SHA-256 {step.compressed_sha256}, "
        f"got {len(compressed)} bytes and SHA-256 {digest}",
    )
    return result


def strip_chain(bundle, chain: tuple[OverlayStep, ...] = OVERLAY_CHAIN):
    validate_chain_definition(chain)
    current = bundle
    for step in reversed(chain):
        current = strip_step(step, current)
    return current


def apply_chain(
    base_bundle,
    chain: tuple[OverlayStep, ...] = OVERLAY_CHAIN,
    *,
    repo_root: Path = REPO_ROOT,
    snapshot_overrides: dict[str, Path] | None = None,
):
    validate_chain_definition(chain)
    current = base_bundle
    overrides = snapshot_overrides or {}
    for step in chain:
        current = apply_step(
            step,
            current,
            repo_root=repo_root,
            snapshot_override=overrides.get(step.name),
        )
    return current


def _resolve_best_effort(path: Path, fallback: Path | None = None) -> Path:
    try:
        return path.resolve()
    except (OSError, RuntimeError):
        return fallback if fallback is not None else path.absolute()


def _path_variants(path: Path) -> tuple[Path, Path]:
    """Return both the directory-entry path (preserving final symlink) and resolved target."""
    entry_path = _resolve_best_effort(path.parent) / path.name
    return entry_path, _resolve_best_effort(path, fallback=entry_path)


def _is_protected_output(output: Path, protected_files: set[Path], protected_dirs: set[Path]) -> bool:
    for candidate in _path_variants(output):
        if candidate in protected_files or candidate in protected_dirs:
            return True
        if any(candidate.is_relative_to(directory) for directory in protected_dirs):
            return True
    return False


def _write_atomic(
    output: Path,
    data: bytes,
    protected_files: set[Path],
    protected_dirs: set[Path],
) -> None:
    require(
        not _is_protected_output(output, protected_files, protected_dirs),
        "refusing to replace an authenticated input",
    )
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(dir=output.parent, prefix=".schema-", delete=False) as stream:
            temporary = Path(stream.name)
            stream.write(data)
        temporary.chmod(0o644)
        temporary.replace(output)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def _protected_inputs(
    production_path: Path,
    base_path: Path,
    chain: tuple[OverlayStep, ...],
    repo_root: Path,
    snapshot_overrides: dict[str, Path] | None,
) -> tuple[set[Path], set[Path]]:
    protected_files: set[Path] = set()
    for path in (base_path, production_path, DEFAULT_BASE, DEFAULT_PRODUCTION):
        protected_files.update(_path_variants(path))
    protected_dirs: set[Path] = set()
    overrides = snapshot_overrides or {}
    for step in chain:
        canonical = repo_root / step.snapshot_path
        active = overrides.get(step.name, canonical)
        candidates = [canonical] if active == canonical else [canonical, active]
        for candidate in candidates:
            entry_path, resolved = _path_variants(candidate)
            is_dir = step.snapshot_kind == "extracted"
            if not is_dir:
                try:
                    is_dir = candidate.is_dir() or resolved.is_dir()
                except OSError:
                    is_dir = False
            if is_dir:
                protected_dirs.update((entry_path, resolved))
                for directory in (entry_path, resolved):
                    try:
                        if directory.is_dir():
                            for record in directory.iterdir():
                                protected_files.update(_path_variants(record))
                    except (OSError, RuntimeError):
                        # An unused overridden canonical directory may lack list permission
                        # or contain symlink loops; keep its directory protected without
                        # blocking active verification.
                        if candidate == active:
                            raise
            else:
                protected_files.update((entry_path, resolved))
    return protected_files, protected_dirs


def verify_chain_from_production(
    production_path: Path = DEFAULT_PRODUCTION,
    base_path: Path = DEFAULT_BASE,
    chain: tuple[OverlayStep, ...] = OVERLAY_CHAIN,
    *,
    repo_root: Path = REPO_ROOT,
    output_path: Path | None = None,
    snapshot_overrides: dict[str, Path] | None = None,
) -> dict[str, object]:
    """Strip schema.pb.gz back to 1.3.4 base and re-apply all overlays forward."""
    _require_protobuf()
    validate_chain_definition(chain)

    protected_files: set[Path] = set()
    protected_dirs: set[Path] = set()
    if output_path is not None:
        protected_files, protected_dirs = _protected_inputs(
            production_path, base_path, chain, repo_root, snapshot_overrides
        )
        require(
            not _is_protected_output(output_path, protected_files, protected_dirs),
            "refusing to replace an authenticated input",
        )

    prod_gz = production_path.read_bytes()
    prod_gz_sha256 = sha256_hex(prod_gz)
    require(
        prod_gz_sha256 == chain[-1].compressed_sha256,
        f"production compressed hash mismatch: expected {chain[-1].compressed_sha256}, got {prod_gz_sha256}",
    )
    prod_raw = gzip.decompress(prod_gz)
    prod_raw_sha256 = sha256_hex(prod_raw)
    require(
        prod_raw_sha256 == chain[-1].raw_sha256,
        f"production uncompressed hash mismatch: expected {chain[-1].raw_sha256}, got {prod_raw_sha256}",
    )
    production = descriptor_pb2.FileDescriptorSet.FromString(prod_raw)
    require(deterministic_bytes(production) == prod_raw, "production bundle is not canonically serialized")

    base_gz = base_path.read_bytes()
    base_gz_sha256 = sha256_hex(base_gz)
    require(
        base_gz_sha256 == BASE_COMPRESSED_SHA256,
        f"base compressed hash mismatch: expected {BASE_COMPRESSED_SHA256}, got {base_gz_sha256}",
    )
    base_raw = gzip.decompress(base_gz)
    base_raw_sha256 = sha256_hex(base_raw)
    require(
        base_raw_sha256 == BASE_RAW_SHA256,
        f"base uncompressed hash mismatch: expected {BASE_RAW_SHA256}, got {base_raw_sha256}",
    )
    base = descriptor_pb2.FileDescriptorSet.FromString(base_raw)

    stripped_base = strip_chain(production, chain)
    require(
        deterministic_bytes(stripped_base) == base_raw and stripped_base == base,
        "stripped production bundle does not match pinned 1.3.4 base",
    )

    rebuilt_from_stripped = apply_chain(
        stripped_base,
        chain,
        repo_root=repo_root,
        snapshot_overrides=snapshot_overrides,
    )
    rebuilt_raw = deterministic_bytes(rebuilt_from_stripped)
    require(
        rebuilt_raw == prod_raw and rebuilt_from_stripped == production,
        "re-applied overlay chain does not reproduce production descriptor bytes",
    )

    rebuilt_from_base = apply_chain(
        base,
        chain,
        repo_root=repo_root,
        snapshot_overrides=snapshot_overrides,
    )
    require(
        deterministic_bytes(rebuilt_from_base) == prod_raw and rebuilt_from_base == production,
        "forward build from pinned 1.3.4 base does not reproduce production descriptor bytes",
    )

    rebuilt_gz = compress_deterministic(chain[-1], rebuilt_raw)
    require(gzip.decompress(rebuilt_gz) == prod_raw, "recompressed bundle fails round-trip decompression")
    require(rebuilt_gz == prod_gz, "re-applied overlay chain does not reproduce production gzip bytes")

    if output_path is not None:
        _write_atomic(output_path, rebuilt_gz, protected_files, protected_dirs)

    return {
        "steps_verified": len(chain),
        "base_raw_sha256": base_raw_sha256,
        "base_compressed_sha256": base_gz_sha256,
        "production_raw_sha256": prod_raw_sha256,
        "production_compressed_sha256": prod_gz_sha256,
        "rebuilt_raw_sha256": sha256_hex(rebuilt_raw),
        "rebuilt_compressed_sha256": sha256_hex(rebuilt_gz),
        "raw_match": rebuilt_raw == prod_raw,
        "compressed_match": rebuilt_gz == prod_gz,
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--production", type=Path, default=DEFAULT_PRODUCTION)
    parser.add_argument("--base", type=Path, default=DEFAULT_BASE)
    parser.add_argument("--output", type=Path, default=None, help="optional path to write rebuilt schema.pb.gz")
    args = parser.parse_args()
    try:
        summary = verify_chain_from_production(
            production_path=args.production,
            base_path=args.base,
            output_path=args.output,
        )
    except (ValueError, OSError, KeyError) as error:
        parser.exit(1, f"refused: {error}\n")
    print(
        f"verified {summary['steps_verified']} overlays "
        f"(raw={summary['rebuilt_raw_sha256']}, "
        f"compressed_match={summary['compressed_match']})"
    )


if __name__ == "__main__":
    main()
