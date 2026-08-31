#!/usr/bin/env python3
"""Offline, fail-closed dependency/license inventory validator for G20."""
import hashlib, json, re, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
INVENTORY = ROOT / "performance/v0.7-g20/DEPENDENCY_LICENSES.json"
ALLOWED = {"0BSD", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "BlueOak-1.0.0", "CC0-1.0", "ISC", "MIT", "MIT-0", "MPL-2.0", "Python-2.0"}
FORBIDDEN = re.compile(r"(?:^|[-+ ])(?:AGPL|GPL|SSPL)(?:[-+ ]|$)|(?:^|[-+ ])UNKNOWN(?:$|[-+ ])", re.I)

class LicenseError(ValueError): pass
def fail(msg): raise LicenseError(msg)
def sha(path): return hashlib.sha256(path.read_bytes()).hexdigest()
def load(path):
    try: return json.loads(path.read_text())
    except Exception as e: fail(f"invalid JSON {path}: {e}")
def check_license(name, lic):
    if not isinstance(lic, str) or not lic or lic not in ALLOWED or FORBIDDEN.search(lic): fail(f"{name}: unapproved or unknown license {lic!r}")

def go_modules():
    found = set()
    for line in (ROOT / "go.mod").read_text().splitlines():
        m = re.match(r"^\s*([^\s/]+(?:/[^\s]+)+)\s+(v\S+)(?:\s+//.*)?$", line)
        if m: found.add(m.group(1) + "@" + m.group(2))
    return found

def npm_lockfiles():
    return {
        path.relative_to(ROOT).as_posix()
        for path in ROOT.rglob("package-lock.json")
        if "node_modules" not in path.parts
    }

def validate():
    inv = load(INVENTORY)
    if inv.get("schema") != "notrios.g20.dependency-licenses.v1": fail("wrong inventory schema")
    if set(inv.get("policy", {}).get("allowed_spdx", [])) != ALLOWED: fail("policy allowed SPDX set drift")
    g = inv.get("go", {}); src = ROOT / "go.mod"
    if g.get("source") != "go.mod" or g.get("sha256") != sha(src): fail("go.mod hash is stale")
    entries = g.get("modules", {}); declared = go_modules()
    if set(entries) != declared: fail(f"Go coverage mismatch: inventory={len(entries)} declared={len(declared)}")
    for name, lic in entries.items(): check_license(name, lic)
    npm = inv.get("npm", {}).get("lockfiles", {})
    if not npm: fail("no npm lockfiles")
    discovered = npm_lockfiles()
    if set(npm) != discovered:
        fail(f"npm lockfile coverage mismatch: inventory={sorted(npm)} discovered={sorted(discovered)}")
    for rel, meta in npm.items():
        path = ROOT / rel
        if not path.is_file(): fail(f"missing npm lockfile {rel}")
        if meta.get("sha256") != sha(path): fail(f"{rel}: lockfile hash is stale")
        packages = load(path).get("packages", {})
        actual = len(packages) - (1 if "" in packages else 0)
        if meta.get("packages") != actual: fail(f"{rel}: package count {actual}, inventory says {meta.get('packages')}")
        for key, package in packages.items():
            if key != "": check_license(f"{rel}:{key}@{package.get('version')}", package.get("license"))
    return len(entries), sum(int(m["packages"]) for m in npm.values())

if __name__ == "__main__":
    try:
        g, n = validate(); print(f"G20 dependency licenses valid: {g} Go modules, {n} npm packages; offline and fail-closed")
    except LicenseError as e:
        print(f"G20 dependency licenses invalid: {e}", file=sys.stderr); sys.exit(1)
