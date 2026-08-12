#!/usr/bin/env python3
"""Validate the committed structure of the v0.7 G0 investigation evidence."""

from __future__ import annotations

import hashlib
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parent


def read(name: str) -> str:
    path = ROOT / name
    if not path.is_file():
        raise SystemExit(f"missing G0 evidence file: {path}")
    return path.read_text(encoding="utf-8")


def require(text: str, needles: list[str], label: str) -> None:
    missing = [needle for needle in needles if needle not in text]
    if missing:
        raise SystemExit(f"{label} missing required text: {missing}")


files = {
    name: read(name)
    for name in (
        "README.md",
        "THREAT_MODEL.md",
        "PROTOCOL_GLOSSARY.md",
        "MISUSE_CASES.md",
        "REFERENCE_VALIDATION.md",
    )
}

require(
    files["THREAT_MODEL.md"],
    [
        "## Overview",
        "## Threat Model, Trust Boundaries, and Assumptions",
        "### Assets and consequences",
        "### Adversaries and capabilities",
        "### Trust boundaries",
        "### Trust roots, enrollment, revocation, and recovery",
        "### Metadata leakage budget",
        "### Audit events",
        "## Attack Surface, Mitigations, and Attacker Stories",
        "## Severity Calibration",
        "### Critical",
        "### High",
        "### Medium",
        "### Low",
    ],
    "threat model",
)

glossary_terms = [
    "Profile",
    "Database",
    "Replica",
    "Peer",
    "Carrier",
    "Snapshot",
    "Envelope",
    "Object",
    "State vector",
    "Advertisement",
    "Request",
]
for term in glossary_terms:
    if not re.search(rf"\| \*\*{re.escape(term)}\*\* \|", files["PROTOCOL_GLOSSARY.md"]):
        raise SystemExit(f"protocol glossary missing normative row: {term}")

misuse_ids = re.findall(r"^\| (M\d{2}) \|", files["MISUSE_CASES.md"], re.MULTILINE)
expected_ids = [f"M{number:02d}" for number in range(1, 31)]
if misuse_ids != expected_ids:
    raise SystemExit(f"misuse cases are not the complete ordered M01-M30 set: {misuse_ids}")

require(
    files["MISUSE_CASES.md"],
    ["G3", "G4", "G5", "G6", "G7", "G8", "G9", "G10", "G11", "G12", "G13", "G14", "G15", "G16", "G17", "G18", "G20"],
    "misuse-case ownership",
)

require(
    files["REFERENCE_VALIDATION.md"],
    [
        "| Candidate / reference | Status observed | License |",
        "crypto/ed25519",
        "golang.org/x/crypto/chacha20poly1305",
        "golang.org/x/crypto/argon2",
        "zalando/go-keyring",
        "flutter_secure_storage",
        "rclone",
        "Subversion",
        "maxpert/marmot",
        "Cachapa",
        "reearth/ygo",
        "Deln0r/ygo",
        "minisign",
        "BSD-3-Clause",
        "MIT",
        "Apache-2.0",
        "Rejected as a protocol/runtime dependency",
    ],
    "reference/dependency matrix",
)

for name, text in files.items():
    if "\x00" in text:
        raise SystemExit(f"NUL byte in {name}")
    if re.search(r"(?i)(password|access_token|private_key)\s*=\s*[^\s]+", text) or "BEGIN PRIVATE KEY" in text:
        raise SystemExit(f"possible secret-shaped assignment in {name}")

require(
    files["README.md"],
    [
        "contains no production",
        "planned unless",
        "Authenticated sync encryption",
        "are not implemented by G0",
    ],
    "working-state disclaimer",
)

print(f"G0 evidence OK: {len(files)} documents, {len(glossary_terms)} glossary terms, {len(misuse_ids)} misuse cases")
for name in sorted(files):
    digest = hashlib.sha256(files[name].encode("utf-8")).hexdigest()
    print(f"{digest}  {name}")
