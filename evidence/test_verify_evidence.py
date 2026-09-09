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


class ApprovedMediaTypeTests(unittest.TestCase):
    """A type is only approved because its container can be proved intact."""

    @staticmethod
    def _bundle() -> bytes:
        import hashlib
        pack = b"PACK" + (2).to_bytes(4, "big") + (0).to_bytes(4, "big")
        return (b"# v2 git bundle\n" + b"a" * 40 + b" refs/heads/main\n" + b"\n"
                + pack + hashlib.sha1(pack).digest())

    @staticmethod
    def _deb(members: list[tuple[bytes, bytes]]) -> bytes:
        out = b"!<arch>\n"
        for name, body in members:
            out += (name.ljust(16) + b"0".ljust(12) + b"0".ljust(6) + b"0".ljust(6)
                    + b"100644".ljust(8) + str(len(body)).encode().ljust(10) + b"`\n")
            out += body + (b"\n" if len(body) % 2 else b"")
        return out

    def test_git_bundle_accepts_a_bundle_and_refuses_damage(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "history.bundle"
            good = self._bundle()
            path.write_bytes(good)
            result = ev.validate_git_bundle(path)
            self.assertTrue(result["valid"], result)
            self.assertEqual(result["refs"], 1)

            # One flipped byte inside the pack must break its checksum.
            damaged = bytearray(good)
            damaged[len(good) - 25] ^= 1
            path.write_bytes(bytes(damaged))
            self.assertFalse(ev.validate_git_bundle(path)["valid"])

            for broken in (good.replace(b"# v2 git bundle", b"# v9 git bundle"),
                           good[: len(good) - 4],
                           good.replace(b"a" * 40, b"z" * 40)):
                path.write_bytes(broken)
                self.assertFalse(ev.validate_git_bundle(path)["valid"], broken[:20])

    def test_deb_accepts_a_package_and_refuses_damage(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "package.deb"
            good = self._deb([(b"debian-binary", b"2.0\n"),
                              (b"control.tar.zst", b"c" * 9),
                              (b"data.tar.zst", b"d" * 4)])
            path.write_bytes(good)
            result = ev.validate_deb(path)
            self.assertTrue(result["valid"], result)
            self.assertEqual(result["names"], ["debian-binary", "control.tar.zst", "data.tar.zst"])

            # A size field that runs past the end, a missing data member, a
            # wrong format version, and a truncated file all have to refuse.
            for broken in (good.replace(b"9".ljust(10), b"999".ljust(10)),
                           self._deb([(b"debian-binary", b"2.0\n"), (b"control.tar.zst", b"c")]),
                           self._deb([(b"debian-binary", b"3.0\n"), (b"control.tar.zst", b"c"),
                                      (b"data.tar.zst", b"d")]),
                           good[:-3], b"not an archive"):
                path.write_bytes(broken)
                self.assertFalse(ev.validate_deb(path)["valid"], broken[:16])

    def test_the_sealer_approves_exactly_what_can_be_validated(self) -> None:
        import importlib.util
        spec = importlib.util.spec_from_file_location(
            "seal_volume", Path(__file__).resolve().parents[1] / "scripts" / "seal_volume.py")
        sealer = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(sealer)
        for suffix, expected in sealer.APPROVED_MEDIA_TYPES.items():
            self.assertEqual(sealer.media_type(Path("x" + suffix)), expected)
            # Every approved type must reach a real validator, never the
            # sha256-only fallback that would approve any bytes at all.
            with tempfile.TemporaryDirectory() as directory:
                probe = Path(directory) / ("x" + suffix)
                probe.write_bytes(b"definitely not a valid container")
                self.assertFalse(ev.structural_validation(probe, expected)["valid"], suffix)
        with self.assertRaises(ev.EvidenceError):
            sealer.media_type(Path("notes.txt"))


if __name__ == "__main__":
    unittest.main()
