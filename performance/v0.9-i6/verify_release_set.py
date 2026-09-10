#!/usr/bin/env python3
"""Verify a release set offline, the way somebody who did not build it would.

    python3 performance/v0.9-i6/verify_release_set.py [--set dist/release-set]

Everything here is recomputed rather than read back. A checksum file that agrees
with itself proves nothing; what matters is that it agrees with the bytes, that
it covers every file in the set, and that the SBOM agrees with an inventory
derived from the repository by different code -- G20's licence gate, which reads
go.mod and the lockfiles for its own reasons.

Cross-checking two independently produced records is the point. A generator and
a verifier written from the same assumption fail together and look like
agreement.
"""
import argparse
import hashlib
import json
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
INVENTORY = ROOT / "performance/v0.7-g20/DEPENDENCY_LICENSES.json"
TOKEN = re.compile(r"[A-Za-z0-9.\-+]+")


class ReleaseSetError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise ReleaseSetError(message)


def sha256(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1 << 20):
            digest.update(chunk)
    return digest.hexdigest()


def licence_ids(expression: str) -> list[str]:
    """The SPDX identifiers inside a licence expression.

    npm lockfiles carry expressions such as "(MPL-2.0 OR Apache-2.0)", which is
    a licence choice rather than an unknown licence. Rejecting anything that is
    not a bare identifier would refuse a dependency whose terms are acceptable
    twice over.
    """
    return [token for token in TOKEN.findall(expression) if token.upper() not in {"OR", "AND", "WITH"}]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--set", dest="release_set", type=pathlib.Path,
                        default=ROOT / "dist" / "release-set")
    arguments = parser.parse_args()
    root = arguments.release_set
    require(root.is_dir(), f"no release set at {root}")

    sums = root / "SHA256SUMS"
    require(sums.is_file(), "the set has no SHA256SUMS")
    listed = {}
    for line in sums.read_text(encoding="ascii").splitlines():
        if not line.strip():
            continue
        digest, _, name = line.partition("  ")
        listed[name] = digest

    present = {path.name for path in root.iterdir() if path.name != "SHA256SUMS"}
    require(present == set(listed),
            f"SHA256SUMS does not cover the set exactly: {sorted(present ^ set(listed))}")
    for name, digest in sorted(listed.items()):
        actual = sha256(root / name)
        require(actual == digest, f"{name}: recorded {digest[:16]} but the bytes hash to {actual[:16]}")

    sbom = json.loads((root / "SBOM.cdx.json").read_text(encoding="utf-8"))
    require(sbom["bomFormat"] == "CycloneDX", "the SBOM is not CycloneDX")
    require(sbom["specVersion"] == "1.5", "unexpected CycloneDX spec version")

    inventory = json.loads(INVENTORY.read_text(encoding="utf-8"))
    counted = {"go": 0}
    for lockfile in inventory["npm"]["lockfiles"]:
        counted[lockfile] = 0
    for component in sbom["components"]:
        require(component.get("purl"), f"{component.get('name')}: no purl")
        if component["purl"].startswith("pkg:golang/"):
            counted["go"] += 1
        else:
            source = next((p["value"] for p in component.get("properties", [])
                           if p["name"] == "notrios:lockfile"), None)
            require(source in counted, f"{component['name']}: unattributed to a lockfile")
            counted[source] += 1

    require(counted["go"] == len(inventory["go"]["modules"]),
            f"SBOM lists {counted['go']} Go modules, the licence inventory has "
            f"{len(inventory['go']['modules'])}")
    for lockfile, data in inventory["npm"]["lockfiles"].items():
        require(counted[lockfile] == data["packages"],
                f"SBOM lists {counted[lockfile]} packages for {lockfile}, the inventory has "
                f"{data['packages']}")

    allowed = set(inventory["policy"]["allowed_spdx"])
    forbidden = [family.upper() for family in inventory["policy"]["forbidden_families"]]
    undeclared = 0
    for component in sbom["components"]:
        expressions = [entry["license"]["id"] for entry in component.get("licenses", [])]
        if not expressions:
            undeclared += 1
            continue
        for expression in expressions:
            for family in forbidden:
                require(family not in expression.upper(),
                        f"{component['name']}: licence {expression} is in a forbidden family")
            for identifier in licence_ids(expression):
                require(identifier in allowed,
                        f"{component['name']}: licence {identifier} is not in the allowed set")

    provenance = json.loads((root / "PROVENANCE.json").read_text(encoding="utf-8"))
    require(provenance["_type"] == "https://in-toto.io/Statement/v1", "not an in-toto statement")
    subjects = provenance["subject"]
    require(subjects, "the provenance names no subject")
    for subject in subjects:
        require(subject["name"] in listed, f"provenance names {subject['name']}, not in the set")
        require(subject["digest"]["sha256"] == listed[subject["name"]],
                f"provenance digest for {subject['name']} disagrees with SHA256SUMS")

    signing = provenance["notrios:signing"]
    require(signing["state"] in ("unsigned", "signed"), "unknown signing state")
    if signing["state"] == "unsigned":
        require(not any(name.endswith((".asc", ".sig")) for name in listed),
                "the set carries signatures but records itself as unsigned")
    else:
        require(any(name.endswith((".asc", ".sig")) for name in listed),
                "the set records itself as signed and carries no signature")

    print(f"release set verified: {len(listed)} artifacts, "
          f"{len(sbom['components'])} SBOM components matching the licence inventory "
          f"({undeclared} without a declared licence), provenance bound to the bytes, "
          f"state {signing['state']}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except ReleaseSetError as error:
        print(f"release set invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
