#!/usr/bin/env python3
"""Build the pinned G18b Hugo/Ledger prototype from repository-owned inputs."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
PROTOTYPE = HERE / "prototype"
INVENTORY = ROOT / "performance/v0.7-g18a/INVENTORY.json"


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def document_inventory() -> list[dict[str, object]]:
    return json.loads(INVENTORY.read_text(encoding="utf-8"))["documents"]


def stage_source(destination: Path) -> list[tuple[Path, Path]]:
    """Stage the prototype and byte-copy all 15 canonical docs into it."""
    shutil.copytree(PROTOTYPE, destination, dirs_exist_ok=True)
    copied: list[tuple[Path, Path]] = []
    for item in document_inventory():
        source = ROOT / str(item["path"])
        if digest(source) != item["sha256"]:
            raise RuntimeError(f"G18a document hash drifted: {source.relative_to(ROOT)}")
        relative = source.relative_to(ROOT / "docs")
        if relative.as_posix() == "index.md":
            relative = Path("_index.md")
        target = destination / "content" / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(source.read_bytes())
        copied.append((source, target))
    # 15 -> 16 in v0.8 H14 slice B: docs/features.md.
    if len(copied) != 17:
        raise RuntimeError(f"expected 17 documentation pages, got {len(copied)}")
    return copied


def run(command: list[str], *, env: dict[str, str]) -> None:
    print("+", " ".join(command), flush=True)
    subprocess.run(command, check=True, env=env)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--hugo", default="hugo")
    parser.add_argument("--pagefind", default="pagefind")
    args = parser.parse_args()

    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env.update({"LC_ALL": "C.UTF-8", "TZ": "UTC", "SOURCE_DATE_EPOCH": "0"})

    with tempfile.TemporaryDirectory(prefix="notrios-g18b-") as temporary:
        source = Path(temporary) / "site"
        stage_source(source)
        run(
            [
                args.hugo,
                "--source",
                str(source),
                "--destination",
                str(output),
                "--cleanDestinationDir",
                "--gc",
                "--minify",
                "--environment",
                "production",
            ],
            env=env,
        )
        run([args.pagefind, "--site", str(output)], env=env)

    print(f"G18b prototype built at {output}")


if __name__ == "__main__":
    main()
