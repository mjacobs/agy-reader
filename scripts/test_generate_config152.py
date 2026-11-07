"""The 1.5.2 overlay adds choices without removing historical model fields."""
import unittest
from test_generate_config150 import SOURCE, INPUTS, descriptor_pb2
import tempfile
from pathlib import Path
if descriptor_pb2 is not None:
    import generate_config152 as generator

@unittest.skipIf(descriptor_pb2 is None, 'google.protobuf is not installed')
class Config152(unittest.TestCase):
    def test_exact_delta(self):
        with tempfile.TemporaryDirectory() as directory:
            base, _ = generator.prior.generate(*INPUTS, SOURCE, Path(directory)/'base.gz')
        result = generator.augment(base)
        target = generator.prior.one(generator.prior.one(result.file, generator.prior.CORTEX_FILE).message_type, 'SubagentModelOption')
        source = descriptor_pb2.FileDescriptorSet.FromString(generator.SNAPSHOT.read_bytes())
        original = generator.prior.one(generator.prior.one(source.file, generator.prior.CORTEX_FILE).message_type, 'SubagentModelOption')
        for name in ('inherit', 'named_model'):
            field = generator.prior.one(target.field, name)
            self.assertEqual(field, generator.prior.one(original.field, name))
            target.field.remove(field)
        self.assertEqual(target.nested_type[0], original.nested_type[0])
        self.assertEqual(target.oneof_decl[0], original.oneof_decl[0])
        del target.nested_type[:]
        del target.oneof_decl[:]
        self.assertEqual(result, base)
        with tempfile.TemporaryDirectory() as directory:
            bad = Path(directory)/'bad'
            bad.write_bytes(generator.SNAPSHOT.read_bytes()+b'bad')
            with self.assertRaisesRegex(ValueError, 'snapshot hash'):
                generator.augment(base, bad)

    def test_cli_accepts_different_gzip_encoding(self):
        import gzip
        import subprocess
        import sys
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)/'base.gz'
            output = Path(directory)/'output.gz'
            _, data = generator.prior.generate(*INPUTS, SOURCE, base)
            base.write_bytes(gzip.compress(gzip.decompress(data), compresslevel=1, mtime=123))
            subprocess.run([sys.executable, '-B', generator.__file__, str(base), str(output)], check=True)
            expected = generator.augment(descriptor_pb2.FileDescriptorSet.FromString(gzip.decompress(data)))
            self.assertEqual(gzip.decompress(output.read_bytes()), expected.SerializeToString(deterministic=True))
            base.write_bytes(gzip.compress(b'invalid', mtime=0))
            original = output.read_bytes()
            result = subprocess.run([sys.executable, '-B', generator.__file__, str(base), str(output)], capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(output.read_bytes(), original)
