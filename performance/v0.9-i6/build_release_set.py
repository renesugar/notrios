#!/usr/bin/env python3
"""Assemble the artifacts and evidence a release would carry, and nothing else.

No SBOM tool is installed here, and none is added. H6a's rule was the smallest
maintainable toolchain, and the inputs for a dependency SBOM are already in the
repository: `go.mod`/`go.sum` resolved into G20's license inventory, and the two
npm lockfiles. Generating from those is deterministic, runs offline, and cannot
disagree with the licence gate -- because `verify_release_set.py` checks it
against exactly that inventory.

    python3 performance/v0.9-i6/build_release_set.py [--out dist/release-set]

Nothing here signs anything. I5 decided what signing means and deliberately
created no key; the signature and timestamp lines are the shape a release will
take, recorded in the provenance as absent rather than implied.
"""
import argparse
import hashlib
import json
import pathlib
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
INVENTORY = ROOT / "performance/v0.7-g20/DEPENDENCY_LICENSES.json"


def sha256(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1 << 20):
            digest.update(chunk)
    return digest.hexdigest()


def git(*args: str) -> str:
    return subprocess.run(["git", "-C", str(ROOT), *args],
                          capture_output=True, text=True, check=True).stdout.strip()


def tool_version(*command: str) -> str | None:
    try:
        result = subprocess.run(command, capture_output=True, text=True, check=True)
    except (OSError, subprocess.CalledProcessError):
        return None
    return (result.stdout or result.stderr).strip().splitlines()[0]


def npm_components(lockfile: pathlib.Path) -> list[dict]:
    """Components from an npm lockfile, named and versioned rather than counted."""
    data = json.loads(lockfile.read_text(encoding="utf-8"))
    components = []
    for path, package in sorted(data.get("packages", {}).items()):
        if not path:
            continue  # the root project is the subject, not one of its components
        name = package.get("name") or path.split("node_modules/")[-1]
        version = package.get("version")
        if not version:
            continue
        entry = {"type": "library", "name": name, "version": version,
                 "purl": f"pkg:npm/{name}@{version}",
                 "properties": [{"name": "notrios:lockfile",
                                 "value": lockfile.relative_to(ROOT).as_posix()}]}
        if package.get("license"):
            entry["licenses"] = [{"license": {"id": package["license"]}}]
        components.append(entry)
    return components


def build_sbom(subject: dict) -> dict:
    inventory = json.loads(INVENTORY.read_text(encoding="utf-8"))
    components: list[dict] = []
    for module, licence in sorted(inventory["go"]["modules"].items()):
        name, _, version = module.rpartition("@")
        components.append({
            "type": "library", "name": name, "version": version,
            "purl": f"pkg:golang/{name}@{version}",
            "licenses": [{"license": {"id": licence}}],
            "properties": [{"name": "notrios:ecosystem", "value": "go"}],
        })
    for lockfile in sorted(inventory["npm"]["lockfiles"]):
        components.extend(npm_components(ROOT / lockfile))
    return {
        "bomFormat": "CycloneDX", "specVersion": "1.5", "version": 1,
        "metadata": {
            "component": {"type": "application", "name": "notrios",
                          "version": subject["version"]},
            "properties": [
                {"name": "notrios:generator",
                 "value": "performance/v0.9-i6/build_release_set.py"},
                {"name": "notrios:derived-from",
                 "value": "performance/v0.7-g20/DEPENDENCY_LICENSES.json and the npm lockfiles"},
            ],
        },
        "components": components,
    }


def build_provenance(subjects: list[dict], version: str) -> dict:
    """An in-toto statement describing how the subjects were produced."""
    return {
        "_type": "https://in-toto.io/Statement/v1",
        "subject": subjects,
        "predicateType": "https://slsa.dev/provenance/v1",
        "predicate": {
            "buildDefinition": {
                "buildType": "https://notrios.com/buildtypes/deb/v1",
                "externalParameters": {"version": version,
                                       "script": "scripts/build_deb.sh"},
                "resolvedDependencies": [{
                    "uri": "git+https://github.com/renesugar/notrios",
                    "digest": {"gitCommit": git("rev-parse", "HEAD")},
                }],
                "internalParameters": {
                    "go": tool_version("go", "version"),
                    "node": tool_version("node", "--version"),
                    "dpkg-deb": tool_version("dpkg-deb", "--version"),
                },
            },
            "runDetails": {
                "builder": {"id": "https://notrios.com/builders/local-workstation"},
                "metadata": {"invocationId": git("rev-parse", "HEAD")},
            },
        },
        "notrios:signing": {
            "state": "unsigned",
            "why": "v0.9 I5 decided the signing policy and deliberately created no key. A "
                   "release built from this set is signed at the point a key exists, under "
                   "performance/v0.9-i5/POLICY.json; recording the absence here keeps an "
                   "unsigned set from being mistaken for a signed one.",
            "policy": "performance/v0.9-i5/POLICY.json",
        },
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--deb", type=pathlib.Path)
    parser.add_argument("--out", type=pathlib.Path, default=ROOT / "dist" / "release-set")
    arguments = parser.parse_args()

    deb = arguments.deb
    if deb is None:
        candidates = sorted((ROOT / "dist" / "deb").glob("*.deb"))
        if not candidates:
            print("no package in dist/deb; run `make deb` first", file=sys.stderr)
            return 1
        deb = candidates[-1]

    out = arguments.out
    out.mkdir(parents=True, exist_ok=True)
    version = subprocess.run(["dpkg-deb", "-f", str(deb), "Version"],
                             capture_output=True, text=True, check=True).stdout.strip()

    payload = out / deb.name
    payload.write_bytes(deb.read_bytes())
    subject = {"name": deb.name, "digest": {"sha256": sha256(payload)}, "version": version}

    (out / "SBOM.cdx.json").write_text(
        json.dumps(build_sbom(subject), indent=2, sort_keys=True) + "\n", encoding="utf-8")
    (out / "PROVENANCE.json").write_text(
        json.dumps(build_provenance(
            [{"name": subject["name"], "digest": subject["digest"]}], version),
            indent=2, sort_keys=True) + "\n", encoding="utf-8")

    # Checksums last, over everything else, so the file that verifies the set is
    # the file a reader checks first.
    lines = [f"{sha256(path)}  {path.name}\n"
             for path in sorted(out.iterdir()) if path.name != "SHA256SUMS"]
    (out / "SHA256SUMS").write_text("".join(lines), encoding="ascii")

    print(f"release set at {out.relative_to(ROOT)}: {len(lines)} artifacts, version {version}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
