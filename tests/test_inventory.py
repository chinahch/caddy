import importlib.util
import io
from pathlib import Path
import tarfile
import tempfile
import unittest

spec=importlib.util.spec_from_file_location('inventory',Path(__file__).resolve().parents[1]/'scripts/release_inventory.py')
inventory=importlib.util.module_from_spec(spec)

class InventoryTests(unittest.TestCase):
    def test_sources_and_notices_are_both_included_and_unlicensed_module_refused(self):
        spec.loader.exec_module(inventory)
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp);(root/'file.go').write_text('package fixture')
            with self.assertRaises(ValueError): inventory.module_files(root)
            (root/'LICENSE').write_text('Fixture notice')
            (root/'nested').mkdir();(root/'nested/NOTICE').write_text('Embedded attribution')
            files=inventory.module_files(root)
            self.assertIn(root/'file.go',files);self.assertIn(root/'nested/NOTICE',files)
            stream=io.BytesIO()
            with tarfile.open(fileobj=stream,mode='w') as tar: inventory.add_tree(tar,root,'module',files)
            stream.seek(0)
            with tarfile.open(fileobj=stream) as tar:
                self.assertEqual(tar.extractfile('module/file.go').read(),b'package fixture')
                self.assertEqual(tar.getmember('module/LICENSE').mtime,0)

if __name__=='__main__':unittest.main()
