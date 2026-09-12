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
import os
import pathlib
import re
import subprocess
import sys
import tempfile

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


def signing_key_fingerprint(public_key: pathlib.Path) -> str:
    """The primary fingerprint of the key a verifier is told to trust."""
    with tempfile.TemporaryDirectory(prefix="notrios-relkey-") as directory:
        home = pathlib.Path(directory)
        home.chmod(0o700)
        environment = dict(os.environ, GNUPGHOME=str(home))
        subprocess.run(["gpg", "--batch", "--no-autostart", "--quiet", "--import",
                        str(public_key)], env=environment, check=True,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        listing = subprocess.run(["gpg", "--batch", "--no-autostart", "--with-colons",
                                  "--fingerprint", "--list-keys"], env=environment,
                                 capture_output=True, text=True, check=True).stdout
    for line in listing.splitlines():
        if line.startswith("fpr:"):
            return line.split(":")[9]
    raise ReleaseSetError(f"{public_key} holds no key")


def verify_signature(signature: pathlib.Path, datum: pathlib.Path,
                     public_key: pathlib.Path) -> str:
    """Check a detached signature against one key, in a keyring of its own.

    A clean keyring rather than the caller's: verifying against whatever keys a
    machine happens to trust answers a different question than "was this signed
    by the key this repository publishes". The evidence verifier does the same
    thing for the same reason, and the fingerprint is returned rather than
    merely checked so the caller can hold it against the committed key.
    """
    with tempfile.TemporaryDirectory(prefix="notrios-relsig-") as directory:
        home = pathlib.Path(directory)
        home.chmod(0o700)
        environment = dict(os.environ, GNUPGHOME=str(home))
        subprocess.run(["gpg", "--batch", "--no-autostart", "--quiet", "--import",
                        str(public_key)], env=environment, check=True,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        result = subprocess.run(
            ["gpg", "--batch", "--no-autostart", "--status-fd", "1", "--trust-model", "always",
             "--verify", str(signature), str(datum)],
            env=environment, capture_output=True, text=True)
    fields = next((line.split() for line in result.stdout.splitlines()
                   if line.startswith("[GNUPG:] VALIDSIG ")), None)
    if fields is None or len(fields) < 12:
        raise ReleaseSetError(
            f"{signature.name} is not a good signature over {datum.name} by the published key")
    return fields[-1]


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
    parser.add_argument("--public-key", type=pathlib.Path,
                        default=ROOT / "keys" / "release-public.asc",
                        help="the key a signed set must verify against")
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

    # SHA256SUMS cannot list itself, and cannot list its own signature either:
    # the signature is made over the finished file, so hashing it would require
    # the file to contain a hash of something derived from it. Same closure
    # boundary as the reserve's -- a volume cannot contain its own final hash.
    # Everything else in the set must be covered.
    uncovered = {"SHA256SUMS", "SHA256SUMS.asc"}
    present = {path.name for path in root.iterdir() if path.name not in uncovered}
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
    # From the directory, not from SHA256SUMS: SHA256SUMS.asc is deliberately
    # absent from the file it signs, so a list derived from that file would miss
    # exactly the signature that matters most.
    signatures = sorted(path.name for path in root.iterdir()
                        if path.name.endswith(".asc") and not path.name.endswith("-public.asc"))
    if signing["state"] == "unsigned":
        require(not signatures, "the set carries signatures but records itself as unsigned")
    else:
        # Noticing a signature exists is not verifying it. A set could carry a
        # .asc of anything -- another project's, an old one, empty -- and the
        # earlier check would have passed it, which is a check that reads as
        # protection and is not.
        require(signatures, "the set records itself as signed and carries no signature")
        public_key = arguments.public_key
        require(public_key.is_file(), f"no published key at {public_key}")
        expected = signing_key_fingerprint(public_key)
        for name in signatures:
            subject = name[:-len(".asc")]
            require((root / subject).is_file(),
                    f"{name} signs {subject}, which is not in the set")
            fingerprint = verify_signature(root / name, root / subject, public_key)
            require(fingerprint == expected,
                    f"{name} was signed by {fingerprint}, not the published key {expected}")
        # The two that must be signed, named rather than inferred: the artifact
        # itself, and the file everything else is checked against.
        for required in (provenance["subject"][0]["name"], "SHA256SUMS"):
            require(f"{required}.asc" in signatures,
                    f"a signed set must carry a signature over {required}")

    verified = len(signatures) if signing["state"] == "signed" else 0
    print(f"release set verified: {len(listed)} artifacts, {verified} signatures checked, "
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
