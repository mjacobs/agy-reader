import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

try:
    from google.protobuf import descriptor_pb2
except ImportError:  # the extractor needs google.protobuf; skip without it
    descriptor_pb2 = None

# Field 15 (option_dependency) is newer than some installed runtimes; the
# extractor tolerates its absence, so only the test that builds it is skipped.
HAS_OPTION_DEPENDENCY = descriptor_pb2 is not None and 'option_dependency' in descriptor_pb2.FileDescriptorProto.DESCRIPTOR.fields_by_name

if descriptor_pb2 is not None:
    spec = importlib.util.spec_from_file_location('extract_descriptors', Path(__file__).with_name('extract_descriptors.py'))
    extractor = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(extractor)


def descriptor(name, package, message, dependency=()):
    file = descriptor_pb2.FileDescriptorProto(name=name, package=package, syntax='proto3')
    file.dependency.extend(dependency)
    msg = file.message_type.add(name=message)
    msg.field.add(name='value', number=1, type=descriptor_pb2.FieldDescriptorProto.TYPE_STRING,
                  label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL, json_name='value')
    return file.SerializeToString()


@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class ExtractDescriptors(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)

    def test_finds_embedded_records_with_offsets(self):
        short = descriptor('pkg/short.proto', 'pkg', 'Short')
        # A name of 128 bytes or more takes a two-byte length prefix.
        long_name = 'pkg/' + 'l' * 130 + '.proto'
        long = descriptor(long_name, 'pkg', 'Long', ['pkg/short.proto'])
        # 0xFF ends each record: it is not a valid FileDescriptorProto tag.
        binary = (b'\x00junk pkg/decoy.proto text\xff' + short + b'\xff' * 7 + long + b'\xff tail')
        path = self.base / 'agy'
        path.write_bytes(binary)

        manifest = extractor.extract(path, self.base / 'out')

        self.assertEqual(manifest['binary_sha256'], hashlib.sha256(binary).hexdigest())
        self.assertEqual([f['name'] for f in manifest['files']], sorted([long_name, 'pkg/short.proto']))
        for index, entry in enumerate(manifest['files']):
            record = (self.base / 'out' / f'{index:04d}.pb').read_bytes()
            self.assertEqual(record, binary[entry['offset']:entry['offset'] + entry['length']])
            self.assertEqual(entry['sha256'], hashlib.sha256(record).hexdigest())
            self.assertIn(record, (short, long))
        written = json.loads((self.base / 'out' / 'manifest.json').read_text())
        self.assertEqual(written, manifest)

    def test_keeps_a_record_whose_tail_looks_like_one_more_field(self):
        # 0x22 0x01 0xff is a well-formed message_type tag and length whose
        # single byte is not a valid DescriptorProto, so a greedy scan that
        # swallowed it would make the whole record unparseable (rv-q2yj.1).
        short = descriptor('pkg/short.proto', 'pkg', 'Short')
        found = extractor.find_descriptors(short + b'\x22\x01\xff\xff')
        self.assertEqual({name: blob for name, (_, blob) in found.items()}, {'pkg/short.proto': short})
        long_name = 'pkg/' + 'l' * 130 + '.proto'
        long = descriptor(long_name, 'pkg', 'Long', ['pkg/short.proto'])
        path = self.base / 'agy'
        path.write_bytes(b'\xff' + short + b'\x22\x01\xff\xff' + long + b'\x22\x01\xff tail')
        manifest = extractor.extract(path, self.base / 'out')
        self.assertEqual([f['name'] for f in manifest['files']], sorted([long_name, 'pkg/short.proto']))
        self.assertEqual([f['length'] for f in manifest['files']], [len(long), len(short)])

    @unittest.skipUnless(HAS_OPTION_DEPENDENCY, 'this protobuf has no FileDescriptorProto.option_dependency (field 15)')
    def test_keeps_option_imports_at_the_end_of_a_record(self):
        # option_dependency (field 15) follows every other field; a record that
        # has it must not be cut short before it.
        file = descriptor_pb2.FileDescriptorProto.FromString(descriptor('pkg/opt.proto', 'pkg', 'Opt'))
        file.option_dependency.extend(['pkg/options.proto', 'pkg/more.proto'])
        record = file.SerializeToString()
        self.assertTrue(record.endswith(b'pkg/more.proto'))
        found = extractor.find_descriptors(b'\xff' + record + b'\xff')
        self.assertEqual(found['pkg/opt.proto'][1], record)
        # Trailing bytes that would parse as an empty or non-.proto option
        # import are not part of the record.
        short = descriptor('pkg/short.proto', 'pkg', 'Short')
        for tail in (b'\x7a\x00', b'\x7a\x03abc'):
            found = extractor.find_descriptors(short + tail + b'\xff')
            self.assertEqual(found['pkg/short.proto'][1], short, tail)
        # Nor are public or weak import markers with no import to mark. The
        # record has no syntax field (12), so fields 10 and 11 may follow it.
        bare = descriptor_pb2.FileDescriptorProto(name='pkg/bare.proto', package='pkg')
        bare.message_type.add(name='Bare')
        bare = bare.SerializeToString()
        for tail in (b'\x50\x00', b'\x58\x00', b'\x50\x52'):
            self.assertEqual(extractor.boundaries(bare + tail, 0)[-1], len(bare) + 2, tail)  # the scan does reach it
            found = extractor.find_descriptors(bare + tail + b'\xff')
            self.assertEqual(found['pkg/bare.proto'][1], bare, tail)

    def test_does_not_absorb_a_trailing_field_out_of_order(self):
        # 0x1a 0x03 abc is a well-formed dependency "abc". A marshalled
        # descriptor lists dependencies (3) before message types (4), so after
        # the record's message type it cannot be part of the record
        # (rv-yct0.1); nor can a second syntax field (12) after the first.
        short = descriptor('pkg/short.proto', 'pkg', 'Short')
        for tail in (b'\x1a\x03abc', b'\x62\x06proto3', b'\x62\x05abcde'):
            found = extractor.find_descriptors(short + tail + b'\xff')
            self.assertEqual({name: blob for name, (_, blob) in found.items()}, {'pkg/short.proto': short}, tail)
            parsed = descriptor_pb2.FileDescriptorProto.FromString(found['pkg/short.proto'][1])
            self.assertEqual(list(parsed.dependency), [])

    def test_does_not_absorb_an_invalid_syntax_after_a_record_without_one(self):
        # A proto2 record leaves syntax (12) unset, so a trailing syntax field
        # is in order and not a repeat; only the check of its value against
        # the known syntaxes keeps "abcde" out of the record (roborev 18556).
        file = descriptor_pb2.FileDescriptorProto(name='pkg/short.proto', package='pkg')
        file.message_type.add(name='Short')
        short = file.SerializeToString()
        found = extractor.find_descriptors(short + b'\x62\x05abcde\xff')
        self.assertEqual({name: blob for name, (_, blob) in found.items()}, {'pkg/short.proto': short})

    def test_does_not_absorb_a_trailing_duplicate_message(self):
        # A second "Short" message after the record is a well-formed, in-order
        # message_type field, but no file declares a top-level name twice
        # (rv-rh9e.1); the bundle built from such a record would not load.
        short = b'\x0a\x0fpkg/short.proto\x12\x03pkg\x22\x07\x0a\x05Short'
        found = extractor.find_descriptors(short + b'\x22\x07\x0a\x05Short\xff')
        self.assertEqual({name: blob for name, (_, blob) in found.items()}, {'pkg/short.proto': short})

    def test_ignores_names_without_a_descriptor(self):
        path = self.base / 'agy'
        path.write_bytes(b'\x0a\x0fpkg/alone.proto\xff only a name, no package or types')
        self.assertEqual(extractor.extract(path, self.base / 'out')['files'], [])

    def test_refuses_existing_output(self):
        path = self.base / 'agy'
        path.write_bytes(b'')
        (self.base / 'out').mkdir()
        with self.assertRaises(FileExistsError):
            extractor.extract(path, self.base / 'out')


if __name__ == '__main__':
    unittest.main()
