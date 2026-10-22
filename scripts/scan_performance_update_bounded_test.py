import datetime as dt
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec=importlib.util.spec_from_file_location('bounded',Path(__file__).with_name('scan-performance-update-bounded.py'))
bounded=importlib.util.module_from_spec(spec);spec.loader.exec_module(bounded)


class RetentionTests(unittest.TestCase):
    def manifest(self,root,deadline):
        data=json.dumps(dict(retention_deadline=deadline.isoformat(),entries=[])).encode()
        (root/'manifest.json').write_bytes(data)
        return hashlib.sha256(data).hexdigest()

    def test_expired_input_stops_before_any_seed_access(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            identity=self.manifest(root,dt.datetime.now(dt.timezone.utc)-dt.timedelta(seconds=1))
            original=(root/'manifest.json').read_bytes()
            with patch.object(bounded,'COHORT',identity):
                with self.assertRaisesRegex(RuntimeError,'expired'):
                    bounded.verify(root)
            self.assertFalse((root/'seed').exists())
            self.assertEqual((root/'manifest.json').read_bytes(),original)

    def test_run_budget_cannot_extend_existing_deadline(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            identity=self.manifest(root,dt.datetime.now(dt.timezone.utc)+dt.timedelta(seconds=30))
            with patch.object(bounded,'COHORT',identity):
                with self.assertRaisesRegex(RuntimeError,'expired'):
                    bounded.valid_manifest(root,reserve_seconds=600)

    def test_manifest_identity_is_required_before_reading_entries(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            self.manifest(root,dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1))
            with self.assertRaisesRegex(RuntimeError,'identity_mismatch'):
                bounded.verify(root)


if __name__=='__main__':unittest.main()
