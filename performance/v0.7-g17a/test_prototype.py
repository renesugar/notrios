#!/usr/bin/env python3
from __future__ import annotations

import copy
from pathlib import Path
import tempfile
import unittest
import zipfile

import prototype


class CanonicalManifestTests(unittest.TestCase):
    def test_canonical_json_is_stable(self) -> None:
        left = prototype.canonical_bytes({"z": 2, "a": [True, None, "å"]})
        right = prototype.canonical_bytes({"a": [True, None, "å"], "z": 2})
        self.assertEqual(left, right)
        self.assertEqual(left, b'{"a":[true,null,"\xc3\xa5"],"z":2}\n')

    def test_canonical_json_rejects_float_and_non_nfc(self) -> None:
        with self.assertRaises(ValueError):
            prototype.canonical_bytes({"value": 1.25})
        with self.assertRaises(ValueError):
            prototype.canonical_bytes({"value": "a\u030a"})
        with self.assertRaises(ValueError):
            prototype.canonical_bytes({"value": 2**63})

    def test_chain_refuses_tamper_swap_and_gap(self) -> None:
        records = prototype.make_chain([
            {"record_type": "artifact", "logical_name": "payload/a", "size_bytes": 1, "sha256": "a" * 64},
            {"record_type": "artifact", "logical_name": "payload/b", "size_bytes": 1, "sha256": "b" * 64},
        ])
        prototype.verify_chain(records)
        tampered = copy.deepcopy(records)
        tampered[0]["payload"]["size_bytes"] = 2
        with self.assertRaises(ValueError):
            prototype.verify_chain(tampered)
        with self.assertRaises(ValueError):
            prototype.verify_chain(list(reversed(copy.deepcopy(records))))
        gap = copy.deepcopy(records)
        gap[1]["sequence"] = 3
        with self.assertRaises(ValueError):
            prototype.verify_chain(gap)


class ContainerValidationTests(unittest.TestCase):
    def test_zip_validation_accepts_crc_clean_and_refuses_traversal(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            good = root / "good.zip"
            with zipfile.ZipFile(good, "w") as archive:
                archive.writestr("README.md", "fixture")
            self.assertTrue(prototype.validate_zip(good)["valid"])
            bad = root / "bad.zip"
            with zipfile.ZipFile(bad, "w") as archive:
                archive.writestr("../escape", "fixture")
            result = prototype.validate_zip(bad)
            self.assertFalse(result["valid"])
            self.assertEqual(result["unsafe_names"], 1)
            for index, member in enumerate(("./alias", "nested//alias"), start=1):
                unsafe = root / f"unsafe-{index}.zip"
                with zipfile.ZipFile(unsafe, "w") as archive:
                    archive.writestr(member, "fixture")
                self.assertFalse(prototype.validate_zip(unsafe)["valid"])

    def test_png_validation_detects_crc_change(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            image = root / "good.png"
            prototype._write_png(image)
            self.assertTrue(prototype.validate_png(image)["valid"])
            data = bytearray(image.read_bytes())
            data[-5] ^= 1
            image.write_bytes(data)
            self.assertFalse(prototype.validate_png(image)["valid"])

    def test_payload_tree_refuses_missing_extra_and_swapped_bytes(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "payload").mkdir()
            first = root / "payload" / "a"
            second = root / "payload" / "b"
            first.write_bytes(b"a")
            second.write_bytes(b"b")
            records = prototype.make_chain([
                {"record_type": "artifact", "logical_name": "payload/a", "size_bytes": 1,
                 "sha256": prototype.sha256_file(first)},
                {"record_type": "artifact", "logical_name": "payload/b", "size_bytes": 1,
                 "sha256": prototype.sha256_file(second)},
            ])
            prototype.verify_payload_tree(records, root)
            second.unlink()
            with self.assertRaises(ValueError):
                prototype.verify_payload_tree(records, root)
            second.write_bytes(b"b")
            (root / "payload" / "extra").write_bytes(b"x")
            with self.assertRaises(ValueError):
                prototype.verify_payload_tree(records, root)
            (root / "payload" / "extra").unlink()
            first.write_bytes(b"b")
            second.write_bytes(b"a")
            with self.assertRaises(ValueError):
                prototype.verify_payload_tree(records, root)


if __name__ == "__main__":
    unittest.main()
