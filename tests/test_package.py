import importlib.util
import io
from pathlib import Path
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class PackageTests(unittest.TestCase):
    def test_archive_reproducible_and_contains_only_named_executables(self):
        spec = importlib.util.spec_from_file_location('builder', ROOT / 'scripts/build.py')
        builder = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(builder)
        with tempfile.TemporaryDirectory() as temp:
            folder = Path(temp)
            (folder / 'caddy').write_bytes(b'node-fixture')
            (folder / 'caddyctl').write_bytes(b'cli-fixture')
            (folder / 'secret.env').write_bytes(b'never-package')
            first = builder.archive(folder, 1700000000)
            second = builder.archive(folder, 1700000000)
            self.assertEqual(first, second)
            with tarfile.open(fileobj=io.BytesIO(first)) as package:
                self.assertEqual(package.getnames(), ['caddy', 'caddyctl'])
                self.assertEqual(package.extractfile('caddy').read(), b'node-fixture')
                self.assertEqual(package.getmember('caddy').mode, 0o755)
                self.assertEqual(package.getmember('caddy').uid, 0)


if __name__ == '__main__':
    unittest.main()
