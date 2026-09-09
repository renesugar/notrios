#!/usr/bin/env python3
"""Verify the vendored SQLite amalgamation against its provenance manifest.

The manifest records what was downloaded, from where, and with which hashes.
This checks the tree still matches, that the build options the manifest
advertises are the ones sqlite_cgo.go actually passes, and that nobody has
started editing third-party sources in place.

It is offline: it re-hashes local files and never contacts sqlite.org. Run it
in the scaffold gate and before any release.
"""

from __future__ import annotations

import hashlib
import json
import os
import re
import sys

VENDOR_DIR = os.path.join("internal", "store", "csqlite")
MANIFEST = os.path.join(VENDOR_DIR, "PROVENANCE.json")
BUILD_OWNER = os.path.join("internal", "store", "sqlite_cgo.go")
SHIM = os.path.join("internal", "store", "sqlite3_amalgamation.c")


def digest(path: str) -> str:
    hasher = hashlib.sha256()
    with open(path, "rb") as stream:
        for chunk in iter(lambda: stream.read(1 << 20), b""):
            hasher.update(chunk)
    return hasher.hexdigest()


def main() -> int:
    problems: list[str] = []

    try:
        with open(MANIFEST, encoding="utf-8") as stream:
            manifest = json.load(stream)
    except (OSError, ValueError) as error:
        print(f"cannot read {MANIFEST}: {error}", file=sys.stderr)
        return 1

    # 1. The vendored files are exactly what was recorded.
    for name, expected in manifest["files"].items():
        path = os.path.join(VENDOR_DIR, name)
        if not os.path.exists(path):
            problems.append(f"{path} is missing")
            continue
        actual = digest(path)
        if actual != expected["sha256"]:
            problems.append(
                f"{path} sha256 {actual} does not match the manifest "
                f"{expected['sha256']}; third-party sources must not be edited"
            )
        size = os.path.getsize(path)
        if size != expected["bytes"]:
            problems.append(f"{path} is {size} bytes, manifest says {expected['bytes']}")

    # 2. Nothing extra crept into the vendor directory.
    permitted = set(manifest["files"]) | {"PROVENANCE.json", "NOTICE"}
    for entry in sorted(os.listdir(VENDOR_DIR)):
        if entry not in permitted:
            problems.append(f"unexpected file in {VENDOR_DIR}: {entry}")

    # 3. The advertised compile options are the ones actually passed. A
    #    manifest that documents options the build does not use is worse than
    #    no manifest, because it is believed.
    try:
        with open(BUILD_OWNER, encoding="utf-8") as stream:
            build = stream.read()
    except OSError as error:
        problems.append(f"cannot read {BUILD_OWNER}: {error}")
        build = ""
    declared = set(re.findall(r"#cgo CFLAGS: -D([A-Za-z0-9_=]+)", build))
    for option in manifest["compile_options"]:
        if option not in declared:
            problems.append(f"{BUILD_OWNER} does not define {option}")
    for option in sorted(declared - set(manifest["compile_options"])):
        problems.append(f"{BUILD_OWNER} defines undocumented option {option}")

    # 4. Hidden visibility and static linkage are claims the build must back.
    if manifest["linkage"]["visibility"] == "hidden" and "-fvisibility=hidden" not in build:
        problems.append(f"{BUILD_OWNER} does not pass -fvisibility=hidden")
    if "pkg-config" in build:
        problems.append(f"{BUILD_OWNER} still references pkg-config; linkage must be static")

    # 5. No file outside the vendor directory may link a system SQLite again.
    for root, _, files in os.walk("internal"):
        for name in files:
            if not name.endswith(".go"):
                continue
            path = os.path.join(root, name)
            with open(path, encoding="utf-8", errors="replace") as stream:
                content = stream.read()
            if "pkg-config: sqlite3" in content:
                problems.append(f"{path} links a system SQLite via pkg-config")
            if "#include <sqlite3.h>" in content:
                problems.append(f"{path} includes a system <sqlite3.h> instead of the vendored header")

    # 6. The shim that makes cgo compile the amalgamation is still present.
    if not os.path.exists(SHIM):
        problems.append(f"{SHIM} is missing; cgo would not compile the amalgamation")

    if problems:
        for problem in problems:
            print(f"sqlite provenance: {problem}", file=sys.stderr)
        return 1

    print(
        f"SQLite provenance valid: {manifest['component']} {manifest['version']}, "
        f"{len(manifest['files'])} unmodified files, "
        f"{len(manifest['compile_options'])} documented compile options, "
        f"static hidden linkage"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
