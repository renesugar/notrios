from __future__ import annotations

import copy
import tempfile
from pathlib import Path
import unittest
import zipfile

from evidence import verify_evidence as ev


class CanonicalTests(unittest.TestCase):
    def test_stable_and_rejects_float_non_nfc(self) -> None:
        self.assertEqual(ev.canonical_bytes({"z": 1, "a": "å"}),
                         b'{"a":"\xc3\xa5","z":1}\n')
        with self.assertRaises(ev.EvidenceError):
            ev.canonical_bytes({"float": 1.5})
        with self.assertRaises(ev.EvidenceError):
            ev.canonical_bytes({"text": "a\u030a"})

    def test_chain_refuses_tamper_and_swap(self) -> None:
        previous = ev.ZERO_HASH
        records = []
        for sequence, name in enumerate(("a", "b"), start=1):
            core = {"schema": ev.ENTRY_SCHEMA, "sequence": sequence,
                    "previous_entry_sha256": previous,
                    "payload": {"record_type": "artifact", "logical_name": name}}
            record = dict(core)
            import hashlib
            previous = hashlib.sha256(ev.canonical_bytes(core)).hexdigest()
            record["entry_sha256"] = previous
            records.append(record)
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "manifest.jsonl"
            path.write_bytes(b"".join(ev.canonical_bytes(record) for record in records))
            ev.load_chain(path)
            changed = copy.deepcopy(records)
            changed[0]["payload"]["logical_name"] = "x"
            path.write_bytes(b"".join(ev.canonical_bytes(record) for record in changed))
            with self.assertRaises(ev.EvidenceError):
                ev.load_chain(path)


class ContainerTests(unittest.TestCase):
    def test_zip_path_and_crc_validation(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            good = root / "good.zip"
            with zipfile.ZipFile(good, "w") as archive:
                archive.writestr("README.md", "fixture")
            self.assertTrue(ev.validate_zip(good)["valid"])
            bad = root / "bad.zip"
            with zipfile.ZipFile(bad, "w") as archive:
                archive.writestr("../escape", "fixture")
            self.assertFalse(ev.validate_zip(bad)["valid"])


if __name__ == "__main__":
    unittest.main()
