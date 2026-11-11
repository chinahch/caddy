"""Integration checks against the real pinned checkout (CADDY_TEST_UPSTREAM)."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class PrepareTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.dest = Path(self.temp.name) / 'source'

    def run_prepare(self, upstream, *extra):
        return subprocess.run([sys.executable, str(ROOT / 'scripts/prepare_source.py'),
                               '--upstream', str(upstream), '--out', str(self.dest), *extra],
                              text=True, capture_output=True)

    def test_prepares_real_pinned_source_and_rejects_overwrite(self):
        result = self.run_prepare(os.environ['CADDY_TEST_UPSTREAM'])
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        manifest = json.loads((self.dest / 'build-source.json').read_text())
        self.assertEqual(manifest['upstream_commit'], '0a29338e1f102a462363ce3527417029f89bab28')
        self.assertEqual(manifest['features'], ['xhttp', 'dual-ip-strict', 'caddy-cli'])
        before = (self.dest / 'internal/config/config.go').read_bytes()
        result = self.run_prepare(os.environ['CADDY_TEST_UPSTREAM'])
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.dest / 'internal/config/config.go').read_bytes(), before)

    def test_rejects_wrong_checkout_without_partial_output(self):
        other = Path(self.temp.name) / 'other'
        subprocess.run(['git', 'init', str(other)], check=True, capture_output=True)
        result = self.run_prepare(other)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('pinned upstream', result.stderr)
        self.assertFalse(self.dest.exists())


if __name__ == '__main__':
    unittest.main()
