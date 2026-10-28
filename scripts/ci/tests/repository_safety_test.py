#!/usr/bin/env python3
import importlib.util
import io
from pathlib import Path
import stat
import tempfile
from types import SimpleNamespace
import unittest
from contextlib import redirect_stdout, redirect_stderr
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
(ROOT / '.tmp').mkdir(exist_ok=True)
spec = importlib.util.spec_from_file_location('repository_safety', ROOT / 'scripts/ci/check_repository_safety.py')
scanner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scanner)


class RepositorySafetyTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir=ROOT / '.tmp')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def scan(self):
        output, error = io.StringIO(), io.StringIO()
        with redirect_stdout(output), redirect_stderr(error):
            result = scanner.scan(self.root)
        return result, output.getvalue() + error.getvalue()

    def test_untracked_plaintext_credential_is_found_without_echoing_value(self):
        fake = 'AK' + 'IA' + 'Z' * 16
        (self.root / 'new-untracked.txt').write_text(fake)
        result, message = self.scan()
        self.assertEqual(result, 1)
        self.assertIn('new-untracked.txt', message)
        self.assertNotIn(fake, message)

    def test_private_key_header_is_found(self):
        fake = '-----BEGIN ' + 'PRIVATE KEY-----'
        self.assertTrue(scanner.contains_credential(fake.encode()))

    def test_forbidden_state_is_rejected_even_without_credentials(self):
        (self.root / 'terraform.tfstate').write_text('{}')
        result, message = self.scan()
        self.assertEqual(result, 1)
        self.assertIn('Forbidden', message)

    def test_generated_cache_is_excluded_but_source_is_scanned(self):
        cache = self.root / '.tmp'
        cache.mkdir()
        (cache / 'terraform.tfstate').write_text('{}')
        (self.root / 'source.py').write_text('print("synthetic")')
        result, message = self.scan()
        self.assertEqual(result, 0)
        self.assertIn('1 current files', message)

    def test_cloud_placeholder_is_not_substituted_with_old_git_content(self):
        path = self.root / 'cloud.txt'
        path.write_text('current bytes must be present')
        original = Path.lstat
        def lstat(current):
            return SimpleNamespace(st_mode=stat.S_IFREG, st_flags=0x40000000) if current == path else original(current)
        with patch.object(Path, 'lstat', lstat):
            result, message = self.scan()
        self.assertEqual(result, 1)
        self.assertIn('scan incomplete', message)

    def test_external_symlink_is_not_followed(self):
        (self.root / 'link').symlink_to('/outside-this-repository')
        result, message = self.scan()
        self.assertEqual(result, 1)
        self.assertIn('External symlink', message)


if __name__ == '__main__':
    unittest.main()
