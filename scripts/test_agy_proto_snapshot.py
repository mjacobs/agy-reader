import argparse
import contextlib
import gzip
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import sys
import tarfile
import tempfile
import textwrap
import unittest

try:
    from google.protobuf import descriptor, descriptor_pb2
except ImportError:  # every command needs google.protobuf; skip without it
    descriptor = descriptor_pb2 = None


# Newer descriptor features that older Python protobuf runtimes lack. Each test
# that needs one is skipped by feature, not by version, so the rest still run.
def _has(name):
    return descriptor_pb2 is not None and hasattr(descriptor_pb2, name)


def _loads_edition_2024():
    """Whether this runtime's descriptor pool accepts an Edition 2024 file."""
    if not _has('EDITION_2024'):
        return False
    from google.protobuf import descriptor_pool
    # A message with a field to read: the pool resolves features lazily, per field.
    probe = descriptor_pb2.FileDescriptorProto(name='probe.proto', syntax='editions', edition=descriptor_pb2.EDITION_2024)
    probe.message_type.add(name='Probe').field.add(
        name='n', number=1, type=descriptor_pb2.FieldDescriptorProto.TYPE_INT32,
        label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, json_name='n')
    try:
        pool = descriptor_pool.DescriptorPool()
        pool.AddSerializedFile(probe.SerializeToString())
        # Features resolve lazily, so read one to meet the edition limit.
        pool.FindMessageTypeByName('Probe').fields[0].has_presence
    except Exception:
        return False
    return True


HAS_OPTION_DEPENDENCY = descriptor_pb2 is not None and 'option_dependency' in descriptor_pb2.FileDescriptorProto.DESCRIPTOR.fields_by_name
HAS_EDITIONS = _has('EDITION_2023') and _has('EDITION_2024') and _has('FeatureSet')
HAS_VISIBILITY = _has('VISIBILITY_LOCAL') and _has('SymbolVisibility')
HAS_PRESENCE_ACCESSORS = descriptor_pb2 is not None and all(
    hasattr(descriptor.FieldDescriptor, name) for name in ('has_presence', 'is_packed'))
LOADS_EDITION_2024 = _loads_edition_2024()

needs_option_dependency = unittest.skipUnless(HAS_OPTION_DEPENDENCY, 'this protobuf has no FileDescriptorProto.option_dependency (field 15)')
needs_editions = unittest.skipUnless(HAS_EDITIONS, 'this protobuf has no EDITION_2023/EDITION_2024 or FeatureSet')
needs_visibility = unittest.skipUnless(HAS_VISIBILITY, 'this protobuf has no symbol visibility (VISIBILITY_LOCAL)')
needs_presence_accessors = unittest.skipUnless(HAS_PRESENCE_ACCESSORS, 'this protobuf has no FieldDescriptor.has_presence/is_packed')

spec = importlib.util.spec_from_file_location('agy_proto_snapshot', Path(__file__).with_name('agy_proto_snapshot.py'))
tool = importlib.util.module_from_spec(spec)
spec.loader.exec_module(tool)

REPO = Path(__file__).resolve().parents[1]
SNAPSHOT_136 = REPO / 'artifacts' / 'proto-snapshots' / 'agy-1.3.6-e14253aa79d4'
HISTORICAL = REPO / 'artifacts' / 'historical-schema-recovery'
ROOTS_136 = ['third_party/gemini_coder/proto/trajectory.proto', 'third_party/jetski/cortex/core/tools_pb/tools.proto']
HAVE_PROTOC = importlib.util.find_spec('grpc_tools') is not None
GOOD_TOOLS = {'python': '3', 'python-protobuf': '7.36.2', 'protoc': '35.1', 'protoc_source': 'test', 'printer': {'source': 'test'}}

B_PROTO = 'syntax = "proto3";\n\npackage pkg;\n\nmessage B {\n  int32 n = 1;\n}\n'
A_PROTO = 'syntax = "proto3";\n\npackage pkg;\n\nimport "pkg/b.proto";\n\nmessage A {\n  string value = 1;\n  B b = 2;\n}\n'


def file_b(**kwargs):
    file = descriptor_pb2.FileDescriptorProto(name='pkg/b.proto', package='pkg', syntax='proto3', **kwargs)
    message = file.message_type.add(name='B')
    message.field.add(name='n', number=1, type=descriptor_pb2.FieldDescriptorProto.TYPE_INT32,
                      label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, json_name='n')
    return file


def file_a(extra_dependency=None):
    file = descriptor_pb2.FileDescriptorProto(name='pkg/a.proto', package='pkg', syntax='proto3')
    file.dependency.append('pkg/b.proto')
    if extra_dependency:
        file.dependency.append(extra_dependency)
    message = file.message_type.add(name='A')
    message.field.add(name='value', number=1, type=descriptor_pb2.FieldDescriptorProto.TYPE_STRING,
                      label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, json_name='value')
    message.field.add(name='b', number=2, type=descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE,
                      label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, type_name='.pkg.B', json_name='b')
    return file


def fake_binary(*files):
    """Bytes that embed serialized descriptors between bytes no record absorbs."""
    return b'\x00header\xff' + b'\xff\xff'.join(f.SerializeToString() for f in files) + b'\xff tail'


def fake_printer(directory, files, exit_code=0, qualified=None):
    """An executable standing in for the Go printer.

    It writes files with -relative, and the qualified texts (when given)
    without it, as the Go printer's default is fully qualified.
    """
    script = Path(tempfile.mkdtemp(dir=directory)) / 'printer'
    script.write_text(textwrap.dedent(f'''\
        #!{sys.executable}
        import os, sys
        files = {files!r}
        if '-relative' not in sys.argv and {qualified!r} is not None:
            files = {qualified!r}
        out = sys.argv[-1]
        for name, text in files.items():
            path = os.path.join(out, name)
            os.makedirs(os.path.dirname(path), exist_ok=True)
            open(path, 'w').write(text)
        sys.exit({exit_code})
        '''))
    script.chmod(0o755)
    return script


@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class Toolchain(unittest.TestCase):
    def test_editions_need_new_enough_tools(self):
        old = {'protoc': (3, 19, 6), 'python-protobuf': (7, 36, 2)}
        problems = tool.check_toolchain({'proto3', 'EDITION_2024'}, old)
        self.assertEqual(len(problems), 1)
        self.assertIn('EDITION_2024 needs protoc >= 32.0, found 3.19.6', problems[0])
        new = {'protoc': (35, 1), 'python-protobuf': (7, 36, 2)}
        self.assertEqual(tool.check_toolchain({'proto2', 'proto3', 'EDITION_2023', 'EDITION_2024'}, new), [])
        self.assertIn('python-protobuf >= 6.32', tool.check_toolchain({'EDITION_2024'}, {'protoc': (35, 1), 'python-protobuf': (5, 29)})[0])

    def test_an_edition_outside_the_table_is_refused(self):
        problems = tool.check_toolchain({'EDITION_2026'}, {'protoc': (99, 0), 'python-protobuf': (99, 0)})
        self.assertEqual(len(problems), 1)
        self.assertIn('not in the supported edition table', problems[0])

    @needs_editions
    def test_syntax_label(self):
        self.assertEqual(tool.syntax_label(descriptor_pb2.FileDescriptorProto()), 'proto2')
        self.assertEqual(tool.syntax_label(descriptor_pb2.FileDescriptorProto(syntax='proto3')), 'proto3')
        editions = descriptor_pb2.FileDescriptorProto(syntax='editions', edition=descriptor_pb2.EDITION_2024)
        self.assertEqual(tool.syntax_label(editions), 'EDITION_2024')


@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class Capture(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.base = Path(temp.name)
        self.source = self.base / 'agy'
        self.source.write_bytes(b'binary bytes' * 1000)
        self.sha = hashlib.sha256(self.source.read_bytes()).hexdigest()

    def test_copy_checks_the_expected_hash(self):
        self.assertEqual(tool.stable_copy(self.source, self.base / 'one', self.sha)[0], self.sha)
        self.assertEqual((self.base / 'one').read_bytes(), self.source.read_bytes())
        with self.assertRaisesRegex(tool.SnapshotError, 'binary hash mismatch'):
            tool.stable_copy(self.source, self.base / 'two', '0' * 64)

    def test_copy_refuses_a_source_that_changes(self):
        original = tool.sha256_file
        calls = []

        def changing(path):
            calls.append(path)
            digest = original(path)
            if len(calls) == 1:  # the source changes after it was first hashed
                Path(path).write_bytes(b'replaced while copying')
            return digest

        tool.sha256_file = changing
        try:
            with self.assertRaisesRegex(tool.SnapshotError, 'changed while it was being copied'):
                tool.stable_copy(self.source, self.base / 'copy')
        finally:
            tool.sha256_file = original

    def test_archive_member_needs_the_published_digest(self):
        archive = self.base / 'agy.tar.gz'
        with tarfile.open(archive, 'w:gz') as tar:
            tar.add(self.source, arcname='antigravity')
        digest = hashlib.sha256(archive.read_bytes()).hexdigest()
        self.assertEqual(tool.copy_from_archive(archive, digest, 'antigravity', self.base / 'one')[0], self.sha)
        with self.assertRaisesRegex(tool.SnapshotError, 'archive hash mismatch'):
            tool.copy_from_archive(archive, '1' * 64, 'antigravity', self.base / 'two')
        with self.assertRaisesRegex(tool.SnapshotError, 'no member'):
            tool.copy_from_archive(archive, digest, 'agy', self.base / 'three')


@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class Closure(unittest.TestCase):
    def test_annotation_only_imports_are_stripped(self):
        annotation = 'net/proto2/proto/descriptor.proto'
        a = file_a(extra_dependency=annotation)
        a.message_type[0].extension.add(name='ext', number=100, extendee='.pkg.B')
        closure, details = tool.build_closure([a, file_b()], ['pkg/a.proto'], [annotation])
        self.assertEqual([f.name for f in closure.file], ['pkg/a.proto', 'pkg/b.proto'])
        self.assertEqual(list(closure.file[0].dependency), ['pkg/b.proto'])
        self.assertEqual(len(closure.file[0].message_type[0].extension), 0)
        self.assertEqual(details['annotation_only_absent_from_binary'], [annotation])
        tool.load_pool(closure)

    @needs_option_dependency
    def test_public_imports_stay_and_option_imports_go(self):
        annotation = 'net/proto2/proto/descriptor.proto'
        c = descriptor_pb2.FileDescriptorProto(name='pkg/c.proto', package='pkg', syntax='proto3')
        c.message_type.add(name='C')
        b = descriptor_pb2.FileDescriptorProto(name='pkg/b.proto', package='pkg', syntax='proto3')
        b.dependency.extend([annotation, 'pkg/c.proto'])
        b.public_dependency.append(1)  # re-exports pkg/c.proto
        b.option_dependency.append('pkg/options.proto')
        a = descriptor_pb2.FileDescriptorProto(name='pkg/a.proto', package='pkg', syntax='proto3')
        a.dependency.append('pkg/b.proto')
        a.message_type.add(name='A').field.add(name='c', number=1, type=descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE,
                                               label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, type_name='.pkg.C', json_name='c')
        closure, _ = tool.build_closure([a, b, c], ['pkg/a.proto'], [annotation])
        kept = {f.name: f for f in closure.file}['pkg/b.proto']
        self.assertEqual((list(kept.dependency), list(kept.public_dependency), list(kept.option_dependency)), (['pkg/c.proto'], [0], []))
        tool.load_pool(closure)

    def test_import_marker_without_an_import_is_refused(self):
        b = file_b()
        b.public_dependency.append(0)
        with self.assertRaisesRegex(tool.SnapshotError, r'import marker \[0\] points past its 0 imports'):
            tool.build_closure([b], ['pkg/b.proto'], [])

    def test_unexpected_missing_import_is_refused(self):
        with self.assertRaisesRegex(tool.SnapshotError, r"unexpected missing imports.*pkg/b.proto"):
            tool.build_closure([file_a()], ['pkg/a.proto'], [])

    def test_absent_root_is_refused(self):
        with self.assertRaisesRegex(tool.SnapshotError, 'root not found'):
            tool.build_closure([file_a(), file_b()], ['pkg/new.proto'], [])

    def test_raw_set_keeps_exact_bytes(self):
        blobs = [file_a().SerializeToString(), b'\x0a\x03odd' * 50]
        data = tool.raw_set(blobs)
        self.assertEqual(tool.raw_set_entries(data), blobs)
        self.assertEqual(len(descriptor_pb2.FileDescriptorSet.FromString(data).file), 2)


@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class Snapshot(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.base = Path(temp.name)
        self.out = self.base / 'snapshots'
        self.out.mkdir()
        self.binary = self.base / 'agy'
        self.binary.write_bytes(fake_binary(file_a(), file_b()))
        self.sha = hashlib.sha256(self.binary.read_bytes()).hexdigest()

    def options(self, **overrides):
        values = dict(binary=str(self.binary), archive=None, archive_sha256=None, archive_member=None,
                      expected_sha256=None, version='9.9.9', version_evidence='test fixture', source_url=None,
                      roots=['pkg/a.proto'], annotation_only=None, out_root=str(self.out), protoc=None)
        values.update(overrides)
        if 'printer' not in values:
            values['printer'] = str(fake_printer(self.base, {'pkg/a.proto': A_PROTO, 'pkg/b.proto': B_PROTO}))
        return argparse.Namespace(**values)

    def refuse(self, pattern, options, toolchain=(['false'], GOOD_TOOLS)):
        before = sorted(p.name for p in self.out.iterdir())
        with self.assertRaisesRegex(tool.SnapshotError, pattern):
            tool.snapshot(options, toolchain)
        # Nothing is published and no staging directory is left behind.
        self.assertEqual(sorted(p.name for p in self.out.iterdir()), before)

    def test_refuses_an_existing_destination(self):
        (self.out / f'agy-9.9.9-{self.sha[:12]}').mkdir()
        self.refuse('refusing to overwrite', self.options())
        self.refuse('refusing to overwrite', self.options(expected_sha256=self.sha))

    def test_refuses_a_hash_mismatch(self):
        self.refuse('binary hash mismatch', self.options(expected_sha256='a' * 64))

    def test_refuses_an_unexpected_missing_import(self):
        self.binary.write_bytes(fake_binary(file_a()))
        self.refuse('unexpected missing imports', self.options())

    @needs_editions
    def test_refuses_an_unsupported_edition(self):
        b = file_b()
        b.syntax, b.edition = 'editions', descriptor_pb2.EDITION_2024
        self.binary.write_bytes(fake_binary(file_a(), b))
        old = dict(GOOD_TOOLS, protoc='3.19.6')
        self.refuse('unsupported toolchain or edition: EDITION_2024 needs protoc >= 32.0, found 3.19.6',
                    self.options(), (['protoc'], old))

    @needs_option_dependency
    def test_refuses_an_extractor_that_would_truncate_records(self):
        real = tool.load_extractor

        def outdated():
            module = real()
            module.ALLOWED = {k: v for k, v in module.ALLOWED.items() if k != 15}
            return module

        tool.load_extractor = outdated
        try:
            self.refuse(r'does not accept FileDescriptorProto field\(s\) \[15\]', self.options())
        finally:
            tool.load_extractor = real

    def test_refuses_a_failing_printer(self):
        printer = fake_printer(self.base, {}, exit_code=1)
        self.refuse('printing .proto files failed', self.options(printer=str(printer)))

    def test_cli_reports_a_refusal(self):
        errors = io.StringIO()
        with contextlib.redirect_stderr(errors):
            code = tool.main(['snapshot', '--binary', str(self.binary), '--version', 'bad version', '--version-evidence', 'x',
                              '--root', 'pkg/a.proto', '--out-root', str(self.out)])
        self.assertEqual(code, 2)
        self.assertIn('refused: version must be a plain release label', errors.getvalue())

    @needs_presence_accessors
    @unittest.skipUnless(HAVE_PROTOC, 'grpcio-tools is not installed')
    def test_publishes_a_verified_snapshot(self):
        toolchain = tool.probe_toolchain(None, str(fake_printer(self.base, {'pkg/a.proto': A_PROTO, 'pkg/b.proto': B_PROTO})))
        dest = tool.snapshot(self.options(expected_sha256=self.sha), toolchain)
        self.assertEqual(dest.name, f'agy-9.9.9-{self.sha[:12]}')
        record = json.loads((dest / 'snapshot.json').read_text())
        self.assertEqual(record['status'], 'verified')
        self.assertEqual(record['binary']['sha256'], self.sha)
        self.assertFalse(record['binary']['executed'])
        self.assertEqual(record['inventory']['records_verified_against_binary'], 2)
        self.assertEqual(record['inventory']['unresolved_imports'], [])
        self.assertEqual(record['roundtrip']['readable_type_names'], 'relative')
        self.assertEqual(record['closure']['files'], 2)
        self.assertEqual([p.name for p in self.out.iterdir()], [dest.name])
        summary = tool.verify(dest, self.binary)
        self.assertEqual(summary['byte_ranges_rechecked'], 2)
        report = tool.compare(dest, dest / 'closure.protoset.gz')
        self.assertTrue(report['identical'])
        # A second run refuses rather than replacing the snapshot.
        self.refuse('refusing to overwrite', self.options(), toolchain)
        # Tampering is caught.
        (dest / 'proto' / 'pkg' / 'b.proto').chmod(0o644)
        (dest / 'proto' / 'pkg' / 'b.proto').write_text(B_PROTO.replace('int32', 'int64'))
        with self.assertRaisesRegex(tool.SnapshotError, 'checksum mismatch'):
            tool.verify(dest)

    @unittest.skipUnless(HAVE_PROTOC, 'grpcio-tools is not installed')
    def test_falls_back_to_qualified_names(self):
        # Short references that compile to the wrong type, or do not compile
        # at all, are printed again fully qualified.
        qualified = {'pkg/a.proto': A_PROTO.replace('  B b = 2;', '  .pkg.B b = 2;'), 'pkg/b.proto': B_PROTO}
        for relative in (B_PROTO.replace('int32', 'int64'), B_PROTO.replace('int32 n', 'Missing n')):
            with self.subTest(relative=relative):
                out = self.base / f'out-{len(list(self.base.iterdir()))}'
                out.mkdir()
                printer = fake_printer(self.base, {'pkg/a.proto': A_PROTO, 'pkg/b.proto': relative}, qualified=qualified)
                dest = tool.snapshot(self.options(printer=str(printer), out_root=str(out)), tool.probe_toolchain(None, str(printer)))
                record = json.loads((dest / 'snapshot.json').read_text())
                self.assertEqual(record['roundtrip']['readable_type_names'], 'fully-qualified')
                self.assertIn('.pkg.B b = 2;', (dest / 'proto' / 'pkg' / 'a.proto').read_text())

    @unittest.skipUnless(HAVE_PROTOC, 'grpcio-tools is not installed')
    def test_accepts_a_printer_path_relative_to_the_working_directory(self):
        printer = fake_printer(self.base, {'pkg/a.proto': A_PROTO, 'pkg/b.proto': B_PROTO})
        previous = os.getcwd()
        os.chdir(printer.parent)
        try:
            dest = tool.snapshot(self.options(printer='./printer'), tool.probe_toolchain(None, './printer'))
        finally:
            os.chdir(previous)
        self.assertTrue((dest / 'snapshot.json').is_file())

    @unittest.skipUnless(HAVE_PROTOC, 'grpcio-tools is not installed')
    def test_refuses_a_roundtrip_difference(self):
        printer = fake_printer(self.base, {'pkg/a.proto': A_PROTO, 'pkg/b.proto': B_PROTO.replace('int32', 'int64')})
        toolchain = tool.probe_toolchain(None, str(printer))
        self.refuse('roundtrip differs', self.options(printer=str(printer)), toolchain)


@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class Compare(unittest.TestCase):
    def write(self, name, *files):
        path = Path(self.temp.name) / name
        path.write_bytes(descriptor_pb2.FileDescriptorSet(file=files).SerializeToString())
        return path

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)

    def kinds(self, report):
        return {(c['category'], c['kind'], c['subject']) for c in report['changes']}

    @needs_presence_accessors
    def test_identical_sets(self):
        old = self.write('old.protoset', file_b(), file_a())
        new = Path(self.temp.name) / 'new.protoset.gz'
        new.write_bytes(gzip.compress(old.read_bytes()))
        report = tool.compare(old, new)
        self.assertTrue(report['identical'])
        self.assertEqual(report['summary'], {'wire': 0, 'json': 0, 'descriptor': 0})

    @needs_editions
    @needs_presence_accessors
    def test_changes_are_classified(self):
        old_a, old_b = file_a(), file_b()
        enum = old_b.enum_type.add(name='Kind')
        enum.value.add(name='KIND_UNSPECIFIED', number=0)
        new_a, new_b = file_a(), file_b()
        new_enum = new_b.enum_type.add(name='Kind')
        new_enum.value.add(name='KIND_UNSPECIFIED', number=0)
        new_enum.value.add(name='KIND_NEW', number=1)
        fields = new_a.message_type[0].field
        fields[0].name, fields[0].json_name = 'label', 'label'           # renamed: JSON only
        del fields[1]                                                     # removed and reserved
        new_a.message_type[0].reserved_range.add(start=2, end=3)
        new_a.message_type[0].field.add(name='count', number=3, type=descriptor_pb2.FieldDescriptorProto.TYPE_UINT64,
                                        label=descriptor_pb2.FieldDescriptorProto.LABEL_REPEATED, json_name='count')
        new_b.syntax, new_b.edition = 'editions', descriptor_pb2.EDITION_2023  # explicit presence by default
        report = tool.compare(self.write('old.protoset', old_b, old_a), self.write('new.protoset', new_b, new_a))
        kinds = self.kinds(report)
        self.assertIn(('json', 'field-name', 'pkg.A field 1'), kinds)
        self.assertIn(('json', 'field-json-name', 'pkg.A field 1'), kinds)
        self.assertIn(('wire', 'field-removed', 'pkg.A field 2'), kinds)
        self.assertIn(('wire', 'field-added', 'pkg.A field 3'), kinds)
        self.assertIn(('descriptor', 'message-reserved-numbers', 'pkg.A'), kinds)
        self.assertIn(('wire', 'enum-value-added', 'pkg.Kind value 1'), kinds)
        self.assertIn(('descriptor', 'file-syntax', 'pkg/b.proto'), kinds)
        self.assertIn(('wire', 'field-presence', 'pkg.B field 1'), kinds)
        self.assertFalse(report['identical'])
        text = tool.render(report)
        self.assertIn('Wire changes', text)
        self.assertIn('field-removed: pkg.A field 2 (was singular pkg.B b)', text)

    @needs_presence_accessors
    def test_service_method_changes_are_reported(self):
        def with_service(server_streaming):
            file = file_a()
            method = file.service.add(name='Chat').method.add(name='Start', input_type='.pkg.A', output_type='.pkg.B')
            method.server_streaming = server_streaming
            return file
        report = tool.compare(self.write('old.protoset', file_b(), with_service(True)), self.write('new.protoset', file_b(), with_service(False)))
        [change] = report['changes']
        self.assertEqual((change['category'], change['kind'], change['subject']), ('wire', 'method-signature', 'pkg.Chat.Start'))
        self.assertEqual((change['old'], change['new']), ('(pkg.A) returns (stream pkg.B)', '(pkg.A) returns (pkg.B)'))

    def test_method_types_are_compared_after_resolution(self):
        def schema(input_type):
            file = descriptor_pb2.FileDescriptorProto(name='p/schema.proto', package='p', syntax='proto2')
            file.message_type.add(name='Child')
            file.message_type.add(name='p').nested_type.add(name='Child')
            file.service.add(name='Api').method.add(name='Call', input_type=input_type, output_type='.p.Child')
            return file
        report = tool.compare(self.write('old.protoset', schema('.p.Child')), self.write('new.protoset', schema('p.Child')))
        self.assertEqual([(c['kind'], c['old'], c['new']) for c in report['changes']],
                         [('method-signature', '(p.Child) returns (p.Child)', '(p.p.Child) returns (p.Child)')])

    @needs_presence_accessors
    def test_reordered_proto2_enum_changes_the_absent_default(self):
        def proto2(order):
            file = descriptor_pb2.FileDescriptorProto(name='p/schema.proto', package='p', syntax='proto2')
            enum = file.enum_type.add(name='State')
            for name, number in order:
                enum.value.add(name=name, number=number)
            file.message_type.add(name='Item').field.add(
                name='state', number=1, type=descriptor_pb2.FieldDescriptorProto.TYPE_ENUM,
                label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, type_name='.p.State', json_name='state')
            return file
        report = tool.compare(self.write('old.protoset', proto2([('OFF', 0), ('ON', 1)])),
                              self.write('new.protoset', proto2([('ON', 1), ('OFF', 0)])))
        self.assertEqual([(c['kind'], c['subject'], c['old'], c['new']) for c in report['changes']],
                         [('field-default', 'p.Item field 1', '0', '1')])

    @needs_presence_accessors
    def test_custom_option_changes_are_reported(self):
        # A custom method option is an extension the runtime cannot name here,
        # so it only exists as unrecognized bytes in the options message.
        def with_route(route):
            file = file_a()
            method = file.service.add(name='Api').method.add(name='Call', input_type='.pkg.A', output_type='.pkg.A')
            data = route.encode()
            method.options.MergeFromString(tool.encode_varint(50000 << 3 | 2) + tool.encode_varint(len(data)) + data)
            return file
        report = tool.compare(self.write('old.protoset', file_b(), with_route('/old')), self.write('new.protoset', file_b(), with_route('/new')))
        [change] = report['changes']
        self.assertEqual((change['category'], change['kind'], change['subject']), ('descriptor', 'method-options', 'pkg.Api.Call'))
        self.assertIn('unrecognized options sha256:', change['new'])

    def oneof_file(self, groups, default=None):
        """pkg/o.proto with message O; groups maps a oneof name to its field numbers."""
        file = descriptor_pb2.FileDescriptorProto(name='pkg/o.proto', package='pkg', syntax='proto2')
        message = file.message_type.add(name='O')
        for index, (name, numbers) in enumerate(groups.items()):
            message.oneof_decl.add(name=name)
            for number in numbers:
                message.field.add(name=f'f{number}', number=number, type=descriptor_pb2.FieldDescriptorProto.TYPE_INT32,
                                  label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, json_name=f'f{number}', oneof_index=index)
        field = message.field.add(name='plain', number=9, type=descriptor_pb2.FieldDescriptorProto.TYPE_INT32,
                                  label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, json_name='plain')
        if default is not None:
            field.default_value = default
        return file

    @needs_presence_accessors
    def test_oneof_rename_is_descriptor_only_and_moving_a_field_is_wire(self):
        old = self.write('old.protoset', self.oneof_file({'choice': [1, 2]}))
        renamed = tool.compare(old, self.write('renamed.protoset', self.oneof_file({'pick': [1, 2]})))
        self.assertEqual({(c['category'], c['kind']) for c in renamed['changes']}, {('descriptor', 'field-oneof-name')})
        moved = tool.compare(old, self.write('moved.protoset', self.oneof_file({'choice': [1], 'other': [2]})))
        self.assertEqual({(c['category'], c['kind'], c['subject']) for c in moved['changes']},
                         {('wire', 'field-oneof', 'pkg.O field 1'), ('wire', 'field-oneof', 'pkg.O field 2')})
        grown = tool.compare(old, self.write('grown.protoset', self.oneof_file({'choice': [1, 2, 3]})))
        self.assertEqual([(c['kind'], c['subject']) for c in grown['changes']], [('field-added', 'pkg.O field 3')])

    def test_public_import_changes_are_reported(self):
        def files(public):
            b = descriptor_pb2.FileDescriptorProto(name='pkg/b.proto', package='pkg', syntax='proto3')
            b.message_type.add(name='B')
            a = descriptor_pb2.FileDescriptorProto(name='pkg/a.proto', package='pkg', syntax='proto3')
            a.dependency.append('pkg/b.proto')
            a.message_type.add(name='A')
            if public:
                a.public_dependency.append(0)
            return b, a
        report = tool.compare(self.write('old.protoset', *files(True)), self.write('new.protoset', *files(False)))
        self.assertEqual([(c['category'], c['kind'], c['subject'], c['old']) for c in report['changes']],
                         [('descriptor', 'file-public-imports', 'pkg/a.proto', ['pkg/b.proto'])])

    def test_extension_range_option_changes_are_reported(self):
        def item(flag):
            file = descriptor_pb2.FileDescriptorProto(name='p/schema.proto', package='p', syntax='proto2')
            file.message_type.add(name='Item').extension_range.add(start=100, end=200).options.MergeFromString(b'\x80\xb5\x18' + flag)
            return file
        report = tool.compare(self.write('old.protoset', item(b'\x00')), self.write('new.protoset', item(b'\x01')))
        self.assertEqual([(c['category'], c['kind'], c['subject']) for c in report['changes']],
                         [('descriptor', 'message-extension-ranges', 'p.Item')])

    @needs_editions
    @needs_visibility
    @needs_presence_accessors
    def test_moved_and_hidden_types_are_reported(self):
        def files(home, local):
            a = descriptor_pb2.FileDescriptorProto(name='pkg/a.proto', package='pkg', syntax='editions', edition=descriptor_pb2.EDITION_2024)
            b = descriptor_pb2.FileDescriptorProto(name='pkg/b.proto', package='pkg', syntax='editions', edition=descriptor_pb2.EDITION_2024)
            item = (a if home == 'a' else b).message_type.add(name='Item')
            if local:
                item.visibility = descriptor_pb2.VISIBILITY_LOCAL
            return a, b
        report = tool.compare(self.write('old.protoset', *files('a', False)), self.write('new.protoset', *files('b', True)))
        self.assertEqual([(c['category'], c['kind'], c['old'], c['new']) for c in report['changes']],
                         [('descriptor', 'message-file', 'pkg/a.proto', 'pkg/b.proto'),
                          ('descriptor', 'message-visibility', None, 'VISIBILITY_LOCAL')])

    @needs_presence_accessors
    def test_oneof_option_changes_are_reported(self):
        old_file, new_file = self.oneof_file({'pick': [1, 2]}), self.oneof_file({'pick': [1, 2]})
        old_file.message_type[0].oneof_decl[0].options.MergeFromString(b'\x80\xb5\x18\x00')
        new_file.message_type[0].oneof_decl[0].options.MergeFromString(b'\x80\xb5\x18\x01')
        report = tool.compare(self.write('old.protoset', old_file), self.write('new.protoset', new_file))
        self.assertEqual([(c['category'], c['kind'], c['subject']) for c in report['changes']],
                         [('descriptor', 'oneof-options', 'pkg.O oneof pick')])

    @needs_presence_accessors
    def test_dropping_an_explicit_default_equal_to_the_implicit_one_is_reported(self):
        report = tool.compare(self.write('old.protoset', self.oneof_file({}, default='0')), self.write('new.protoset', self.oneof_file({})))
        self.assertEqual([(c['category'], c['kind'], c['old'], c['new']) for c in report['changes']],
                         [('descriptor', 'field-default-declaration', '0', None)])

    @needs_presence_accessors
    def test_enum_value_option_changes_are_reported(self):
        def with_enum(deprecated):
            file = file_b()
            enum = file.enum_type.add(name='Kind')
            enum.value.add(name='KIND_UNSPECIFIED', number=0)
            value = enum.value.add(name='KIND_OLD', number=1)
            if deprecated:
                value.options.deprecated = True
            return file
        old_b, new_b = with_enum(False), with_enum(True)
        report = tool.compare(self.write('old.protoset', old_b), self.write('new.protoset', new_b))
        self.assertEqual([(c['category'], c['kind'], c['subject']) for c in report['changes']],
                         [('descriptor', 'enum-value-options', 'pkg.Kind value 1')])

    @needs_editions
    @needs_presence_accessors
    def test_delimited_message_encoding_is_a_wire_change(self):
        def editions(delimited):
            file = descriptor_pb2.FileDescriptorProto(name='p/s.proto', package='p', syntax='editions', edition=descriptor_pb2.EDITION_2023)
            file.message_type.add(name='Child')
            field = file.message_type.add(name='Parent').field.add(
                name='child', number=1, type=descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE,
                label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, type_name='.p.Child', json_name='child')
            if delimited:
                field.options.features.message_encoding = descriptor_pb2.FeatureSet.DELIMITED
            return file
        report = tool.compare(self.write('old.protoset', editions(False)), self.write('new.protoset', editions(True)))
        kinds = {(c['category'], c['kind']): c for c in report['changes']}
        self.assertEqual(kinds[('wire', 'field-type')]['new'], 'p.Child (group)')
        self.assertIn(('descriptor', 'field-options'), kinds)

    @needs_presence_accessors
    def test_extension_changes_are_reported(self):
        def with_extension(kind):
            file = descriptor_pb2.FileDescriptorProto(name='p/extra.proto', package='p', syntax='proto2')
            file.message_type.add(name='Item').extension_range.add(start=100, end=200)
            if kind is not None:
                file.extension.add(name='extra', number=100, extendee='.p.Item', type=kind,
                                   label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, json_name='extra')
            return file
        string, int32 = descriptor_pb2.FieldDescriptorProto.TYPE_STRING, descriptor_pb2.FieldDescriptorProto.TYPE_INT32
        report = tool.compare(self.write('old.protoset', with_extension(string)), self.write('new.protoset', with_extension(int32)))
        self.assertEqual([(c['category'], c['kind'], c['subject'], c['old'], c['new']) for c in report['changes']],
                         [('wire', 'field-type', 'p.Item extension 100', 'TYPE_STRING (length-delimited)', 'TYPE_INT32 (varint)'),
                          ('json', 'field-default', 'p.Item extension 100', "''", '0')])
        removed = tool.compare(self.write('a.protoset', with_extension(string)), self.write('b.protoset', with_extension(None)))
        self.assertEqual([(c['kind'], c['subject']) for c in removed['changes']], [('extension-removed', 'p.Item extension 100')])

    @needs_editions
    @needs_presence_accessors
    def test_utf8_validation_change_is_a_wire_change(self):
        def editions(check):
            file = descriptor_pb2.FileDescriptorProto(name='p/s.proto', package='p', syntax='editions', edition=descriptor_pb2.EDITION_2023)
            file.options.features.utf8_validation = check
            file.message_type.add(name='Item').field.add(
                name='text', number=1, type=descriptor_pb2.FieldDescriptorProto.TYPE_STRING,
                label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, json_name='text')
            return file
        report = tool.compare(self.write('old.protoset', editions(descriptor_pb2.FeatureSet.NONE)),
                              self.write('new.protoset', editions(descriptor_pb2.FeatureSet.VERIFY)))
        kinds = {(c['category'], c['kind']): c for c in report['changes']}
        self.assertEqual((kinds[('wire', 'field-utf8-validation')]['old'], kinds[('wire', 'field-utf8-validation')]['new']), ('NONE', 'VERIFY'))

    def test_message_set_wire_format_is_a_wire_change(self):
        def item(message_set):
            file = descriptor_pb2.FileDescriptorProto(name='p/m.proto', package='p', syntax='proto2')
            message = file.message_type.add(name='Item')
            message.extension_range.add(start=4, end=536870912)
            if message_set:
                message.options.message_set_wire_format = True
            return file
        report = tool.compare(self.write('old.protoset', item(False)), self.write('new.protoset', item(True)))
        self.assertIn(('wire', 'message-set-wire-format', 'p.Item'), self.kinds(report))

    @needs_presence_accessors
    def test_field_targets_are_compared_after_resolution(self):
        def schema(type_name):
            file = descriptor_pb2.FileDescriptorProto(name='p/schema.proto', package='p', syntax='proto2')
            file.message_type.add(name='Child').field.add(name='text', number=1, type=descriptor_pb2.FieldDescriptorProto.TYPE_STRING,
                                                          label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, json_name='text')
            parent = file.message_type.add(name='Parent')
            parent.nested_type.add(name='p').nested_type.add(name='Child').field.add(
                name='count', number=1, type=descriptor_pb2.FieldDescriptorProto.TYPE_INT32,
                label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, json_name='count')
            parent.field.add(name='child', number=1, type=descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE,
                             label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, type_name=type_name, json_name='child')
            return file
        report = tool.compare(self.write('old.protoset', schema('.p.Child')), self.write('new.protoset', schema('p.Child')))
        [change] = report['changes']
        self.assertEqual((change['kind'], change['old'], change['new']),
                         ('field-type', 'p.Child (length-delimited)', 'p.Parent.p.Child (length-delimited)'))

    @needs_presence_accessors
    def test_moving_an_extension_to_another_file_is_reported(self):
        def files(home):
            out = []
            for name in ('pkg/a.proto', 'pkg/b.proto'):
                file = descriptor_pb2.FileDescriptorProto(name=name, package='pkg', syntax='proto2')
                if name == 'pkg/a.proto':
                    file.message_type.add(name='Item').extension_range.add(start=100, end=200)
                else:
                    file.dependency.append('pkg/a.proto')
                    if home == 'b':
                        file.extension.add(name='marker', number=100, extendee='.pkg.Item', type=descriptor_pb2.FieldDescriptorProto.TYPE_BOOL,
                                           label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, json_name='marker')
                if name == 'pkg/a.proto' and home == 'a':
                    file.extension.add(name='marker', number=100, extendee='.pkg.Item', type=descriptor_pb2.FieldDescriptorProto.TYPE_BOOL,
                                       label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, json_name='marker')
                out.append(file)
            return out
        report = tool.compare(self.write('old.protoset', *files('a')), self.write('new.protoset', *files('b')))
        self.assertEqual([(c['category'], c['kind'], c['old'], c['new']) for c in report['changes']],
                         [('descriptor', 'extension-file', 'pkg/a.proto', 'pkg/b.proto')])

    def test_moving_a_service_to_another_file_is_reported(self):
        def files(home):
            common = descriptor_pb2.FileDescriptorProto(name='p/common.proto', package='p', syntax='proto3')
            common.message_type.add(name='M')
            out = [common]
            for name in ('p/a.proto', 'p/b.proto'):
                file = descriptor_pb2.FileDescriptorProto(name=name, package='p', syntax='proto3')
                file.dependency.append('p/common.proto')
                if name == home:
                    file.service.add(name='Api').method.add(name='Call', input_type='.p.M', output_type='.p.M')
                out.append(file)
            return out
        report = tool.compare(self.write('old.protoset', *files('p/a.proto')), self.write('new.protoset', *files('p/b.proto')))
        self.assertFalse(report['identical'])
        self.assertEqual([(c['category'], c['kind'], c['subject'], c['old'], c['new']) for c in report['changes']],
                         [('descriptor', 'service-file', 'p.Api', 'p/a.proto', 'p/b.proto')])

    @needs_presence_accessors
    def test_moving_an_extension_changes_its_json_key(self):
        def schema(scope):
            file = descriptor_pb2.FileDescriptorProto(name='p/schema.proto', package='p', syntax='proto2')
            file.message_type.add(name='Item').extension_range.add(start=100, end=200)
            for name in ('Before', 'After'):
                message = file.message_type.add(name=name)
                if name == scope:
                    message.extension.add(name='extra', number=100, extendee='.p.Item', type=descriptor_pb2.FieldDescriptorProto.TYPE_STRING,
                                          label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, json_name='extra')
            return file
        report = tool.compare(self.write('old.protoset', schema('Before')), self.write('new.protoset', schema('After')))
        self.assertEqual([(c['category'], c['kind'], c['old'], c['new']) for c in report['changes']],
                         [('json', 'extension-scope', 'p.Before', 'p.After')])

    @needs_presence_accessors
    def test_type_change_shows_the_wire_encoding(self):
        new_b = file_b()
        new_b.message_type[0].field[0].type = descriptor_pb2.FieldDescriptorProto.TYPE_UINT32
        report = tool.compare(self.write('old.protoset', file_b()), self.write('new.protoset', new_b))
        [change] = report['changes']
        self.assertEqual((change['kind'], change['old'], change['new']), ('field-type', 'TYPE_INT32 (varint)', 'TYPE_UINT32 (varint)'))

    def test_directory_without_a_snapshot_record_needs_an_explicit_file(self):
        with self.assertRaisesRegex(tool.SnapshotError, 'name the descriptor set file explicitly'):
            tool.load_set(self.temp.name)

    def cli(self, *args):
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            code = tool.main(['compare', *map(str, args)])
        return code, out.getvalue(), err.getvalue()

    def assertRefused(self, result, reason):
        code, out, err = result
        self.assertEqual(code, 2)
        self.assertEqual(out, '')
        self.assertEqual(len(err.splitlines()), 1, err)
        self.assertIn('refused: ', err)
        self.assertIn(reason, err)

    @needs_presence_accessors
    def test_cli_exit_status_is_0_identical_and_1_different(self):
        old = self.write('old.protoset', file_b())
        new = self.write('new.protoset', file_b(), file_a())
        code, out, err = self.cli(old, old)
        self.assertEqual((code, err), (0, ''))
        self.assertIn('identical: no schema differences', out)
        code, out, err = self.cli(old, new)
        self.assertEqual((code, err), (1, ''))
        self.assertIn('summary: ', out)

    def test_cli_refuses_a_missing_input(self):
        old = self.write('old.protoset', file_b())
        missing = Path(self.temp.name) / 'typo.protoset'
        self.assertRefused(self.cli(old, missing), f'cannot read {missing}')
        self.assertRefused(self.cli(missing, old), f'cannot read {missing}')

    def test_cli_refuses_an_input_that_is_not_a_descriptor_set(self):
        old = self.write('old.protoset', file_b())
        text = Path(self.temp.name) / 'README.md'
        text.write_text('# agy-reader\n\nNot a descriptor set.\n')
        self.assertRefused(self.cli(old, text), f'{text} is not a descriptor set')
        empty = Path(self.temp.name) / 'empty.protoset'
        empty.write_bytes(b'')
        self.assertRefused(self.cli(old, empty), 'holds no descriptors')
        bad_gzip = Path(self.temp.name) / 'bad.protoset.gz'
        bad_gzip.write_bytes(gzip.compress(old.read_bytes())[:12])
        self.assertRefused(self.cli(old, bad_gzip), 'is not a descriptor set')

    @needs_presence_accessors
    def test_cli_refuses_an_existing_json_path_before_comparing(self):
        old = self.write('old.protoset', file_b())
        new = self.write('new.protoset', file_b(), file_a())
        delta = Path(self.temp.name) / 'delta.json'
        code, out, err = self.cli(old, new, '--json', delta)
        self.assertEqual((code, err), (1, ''))
        self.assertIn('summary: ', out)
        self.assertFalse(json.loads(delta.read_text())['identical'])
        before = delta.read_bytes()
        # The rerun refuses up front: nothing is compared, the file is kept.
        real, calls = tool.compare, []
        tool.compare = lambda *a: calls.append(a) or real(*a)
        self.addCleanup(setattr, tool, 'compare', real)
        self.assertRefused(self.cli(old, new, '--json', delta), f'--json output already exists: {delta}')
        self.assertEqual(calls, [])
        self.assertEqual(delta.read_bytes(), before)
        self.assertRefused(self.cli(old, new, '--json', Path(self.temp.name) / 'no-such-dir' / 'delta.json'),
                           '--json directory does not exist')

    @needs_presence_accessors
    def test_cli_refuses_when_the_report_cannot_be_written(self):
        old = self.write('caf\u00e9.protoset', file_b())
        out = io.TextIOWrapper(io.BytesIO(), encoding='ascii')
        err = io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            code = tool.main(['compare', str(old), str(old)])
        self.assertEqual(code, 2)
        self.assertEqual(len(err.getvalue().splitlines()), 1, err.getvalue())
        self.assertIn('refused: cannot write the report to stdout: UnicodeEncodeError', err.getvalue())

    @unittest.skipUnless(os.path.exists('/dev/full'), 'needs /dev/full')
    @needs_presence_accessors
    def test_cli_refuses_with_status_2_when_stdout_is_full(self):
        import subprocess
        old = self.write('old.protoset', file_b())
        with open('/dev/full', 'w') as full:
            result = subprocess.run([sys.executable, '-B', str(Path(tool.__file__)), 'compare', str(old), str(old)],
                                    stdout=full, stderr=subprocess.PIPE, text=True)
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertEqual(result.stderr.splitlines(), ['refused: cannot write the report to stdout: OSError: [Errno 28] No space left on device'])

    def test_cli_reports_an_unexpected_error_as_a_refusal(self):
        old = self.write('old.protoset', file_b())
        real = tool.compare
        def crash(*args):
            raise ValueError('boom')
        tool.compare = crash
        self.addCleanup(setattr, tool, 'compare', real)
        self.assertRefused(self.cli(old, old), 'compare failed unexpectedly: ValueError: boom')


@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
@unittest.skipUnless(SNAPSHOT_136.is_dir() and HISTORICAL.is_dir(), 'committed snapshots are not present')
@unittest.skipUnless(LOADS_EDITION_2024, 'this protobuf cannot load the Edition 2024 descriptors in the committed evidence')
class CommittedEvidence(unittest.TestCase):
    """The committed 1.0.10 and 1.3.6 evidence, rebuilt and compared offline."""

    def rebuild(self, extracted, roots):
        manifest = json.loads((extracted / 'manifest.json').read_text())
        records = [file for _, file in tool.read_records(extracted, manifest)]
        closure, _ = tool.build_closure(records, roots, tool.DEFAULT_ANNOTATION_ONLY)
        return closure.SerializeToString(deterministic=True)

    def test_records_rebuild_the_committed_closures(self):
        self.assertEqual(self.rebuild(SNAPSHOT_136 / 'extracted', ROOTS_136), (SNAPSHOT_136 / 'trajectory.protoset').read_bytes())
        self.assertEqual(self.rebuild(HISTORICAL / '1.0.10-extracted', ROOTS_136[:1]),
                         (HISTORICAL / '1.0.10-trajectory.protoset').read_bytes())

    @needs_presence_accessors
    def test_committed_closure_matches_itself(self):
        report = tool.compare(SNAPSHOT_136 / 'trajectory.protoset', SNAPSHOT_136 / 'trajectory.protoset.gz')
        self.assertTrue(report['identical'])

    @needs_presence_accessors
    def test_historical_delta_includes_the_recovered_fields(self):
        report = tool.compare(HISTORICAL / '1.0.10-trajectory.protoset', SNAPSHOT_136 / 'trajectory.protoset')
        changes = {(c['kind'], c['subject']): c for c in report['changes']}
        self.assertEqual(changes[('field-removed', 'exa.chat_pb.ChatMessagePrompt field 23')]['old'],
                         'singular exa.codeium_common_pb.Struct prompt_struct')
        self.assertEqual(changes[('field-added', 'exa.chat_pb.ChatMessagePrompt field 25')]['new'], 'singular genai.Struct prompt_struct')
        self.assertEqual(changes[('field-removed', 'exa.cortex_pb.CascadePlannerConfig field 50')]['old'],
                         'singular exa.cortex_pb.ContextEngineConfig context_engine_config')
        self.assertIn(('message-removed', 'exa.cortex_pb.ContextEngineConfig'), changes)
        self.assertIn([23, 24], [list(r) for r in changes[('message-reserved-numbers', 'exa.chat_pb.ChatMessagePrompt')]['new']])


if __name__ == '__main__':
    unittest.main()
