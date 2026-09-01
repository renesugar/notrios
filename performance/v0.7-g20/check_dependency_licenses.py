#!/usr/bin/env python3
"""Offline, fail-closed dependency/license inventory validator for G20."""
import hashlib, json, re, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
INVENTORY = ROOT / "performance/v0.7-g20/DEPENDENCY_LICENSES.json"
ALLOWED = {"0BSD", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "BlueOak-1.0.0", "CC0-1.0", "ISC", "MIT", "MIT-0", "MPL-2.0", "Python-2.0", "Unlicense"}
FORBIDDEN = re.compile(r"(?:^|[-+ ])(?:AGPL|GPL|SSPL)(?:[-+ ]|$)|(?:^|[-+ ])UNKNOWN(?:$|[-+ ])", re.I)

# Three reviewed decisions taken for v0.8 H2, recorded here rather than in a
# looser matcher, because each is a specific judgement about a specific package
# and a general relaxation would hide the next real problem.
#
# 1. Unlicense joins ALLOWED above. It is public-domain equivalent and imposes
#    no conditions; the project already redistributes public-domain code in the
#    vendored SQLite amalgamation. robust-predicates@3.0.3 is the dependency
#    that raised it.
#
# 2. An SPDX "OR" expression is satisfied when every branch is allowed, since a
#    redistributor may pick any branch. dompurify ships "(MPL-2.0 OR
#    Apache-2.0)" and both halves are already allowed individually; the old
#    exact-set test refused it purely because it is a compound string. An "AND"
#    expression is deliberately not resolved: it requires satisfying every
#    branch, and it has not come up.
#
# 3. NO_METADATA_LICENSE names packages that ship a licence file but omit the
#    field from package.json, so a metadata scanner reports nothing. The entry
#    records what the file actually says and its SHA-256, so this cannot drift
#    into a blanket exemption: if the package changes its licence text the hash
#    stops matching and the review has to happen again.
NO_METADATA_LICENSE = {
    "khroma@2.1.0": {
        "license": "MIT",
        "license_file": "license",
        "sha256": "66b333b0f66759a0b710459e03f7029abe17f4358114a128d2c972e642961b49",
        "reviewed": "v0.8 H2a",
    },
}


def resolve_disjunction(lic):
    """Return the branches of an SPDX OR expression, or None if it is not one."""
    if not isinstance(lic, str):
        return None
    text = lic.strip()
    if not (text.startswith("(") and text.endswith(")")):
        return None
    inner = text[1:-1]
    if " AND " in inner.upper():
        return None
    parts = [part.strip() for part in re.split(r"\s+OR\s+", inner, flags=re.I)]
    return parts if len(parts) > 1 else None

class LicenseError(ValueError): pass
def fail(msg): raise LicenseError(msg)
def sha(path): return hashlib.sha256(path.read_bytes()).hexdigest()
def load(path):
    try: return json.loads(path.read_text())
    except Exception as e: fail(f"invalid JSON {path}: {e}")
def verify_no_metadata_overrides(project_root):
    """Re-hash the licence file behind every recorded no-metadata review.

    A recorded hash that nothing checks is decoration. If the installed package
    is absent the check is skipped, because the gate must stay runnable without
    node_modules; when it is present the text has to be the reviewed text.
    """
    for key, override in NO_METADATA_LICENSE.items():
        name = key.rsplit("@", 1)[0]
        candidate = project_root / "node_modules" / name / override["license_file"]
        if not candidate.is_file():
            continue
        if sha(candidate) != override["sha256"]:
            fail(f"{key}: licence file changed since the {override['reviewed']} review; re-review it")


def check_license(name, lic):
    # A package with no metadata licence is refused unless it has a recorded,
    # hash-pinned review. The identity is the trailing "pkg@version" of the
    # caller's label.
    if lic is None:
        key = name.rsplit(":", 1)[-1].lstrip("node_modules/")
        override = NO_METADATA_LICENSE.get(key)
        if override is None:
            fail(f"{name}: no license in metadata and no recorded review")
        lic = override["license"]
    branches = resolve_disjunction(lic)
    if branches is not None:
        for branch in branches:
            if branch not in ALLOWED or FORBIDDEN.search(branch):
                fail(f"{name}: license expression {lic!r} has unapproved branch {branch!r}")
        return
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
        verify_no_metadata_overrides(path.parent)
    return len(entries), sum(int(m["packages"]) for m in npm.values())

if __name__ == "__main__":
    try:
        g, n = validate(); print(f"G20 dependency licenses valid: {g} Go modules, {n} npm packages; offline and fail-closed")
    except LicenseError as e:
        print(f"G20 dependency licenses invalid: {e}", file=sys.stderr); sys.exit(1)
