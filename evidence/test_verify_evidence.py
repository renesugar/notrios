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


class CustodyChainTests(unittest.TestCase):
    """The custody chain is only a chain if something walks it."""

    @staticmethod
    def _events(count: int) -> list[dict]:
        import hashlib
        records, previous = [], ev.ZERO_HASH
        for index in range(count):
            event = {"schema": "notrios.evidence.custody-event.v1",
                     "action": "create-and-verify-reserve",
                     "volume_id": f"NTR-EV-{index + 1:04d}",
                     "previous_event_sha256": previous}
            records.append({"payload": {"custody_event": event}})
            previous = hashlib.sha256(ev.canonical_bytes(event)).hexdigest()
        return records

    def test_walks_a_good_chain_and_refuses_a_broken_one(self) -> None:
        records = self._events(3)
        ev.verify_custody_chain(records)

        broken = copy.deepcopy(records)
        broken[2]["payload"]["custody_event"]["previous_event_sha256"] = ev.ZERO_HASH
        with self.assertRaises(ev.EvidenceError):
            ev.verify_custody_chain(broken)

        # The failure the sealer actually had: a later event copied a field
        # nothing writes, so every link fell back to all zeroes.
        zeroed = copy.deepcopy(records)
        for record in zeroed:
            record["payload"]["custody_event"]["previous_event_sha256"] = ev.ZERO_HASH
        with self.assertRaises(ev.EvidenceError):
            ev.verify_custody_chain(zeroed)

        edited = copy.deepcopy(records)
        edited[0]["payload"]["custody_event"]["action"] = "something-else"
        with self.assertRaises(ev.EvidenceError):
            ev.verify_custody_chain(edited)

        with self.assertRaises(ev.EvidenceError):
            ev.verify_custody_chain([{"payload": {}}])


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
