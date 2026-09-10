#!/usr/bin/env python3
"""Re-record the frozen surfaces after a deliberate change.

    python3 performance/v0.9-i8/build_freeze.py

Run this when a surface changes on purpose, and say in the commit what moved.
The gate's job is to make sure that sentence gets written.
"""
import json
import pathlib
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import surfaces  # noqa: E402

frozen = json.loads((HERE / "FROZEN.json").read_text(encoding="utf-8"))
before = {name: data["sha256"] for name, data in frozen["surfaces"].items()}
frozen["surfaces"] = surfaces.inventory()
frozen["content_commit"] = subprocess.run(["git", "rev-parse", "HEAD"],
                                          capture_output=True, text=True).stdout.strip()
(HERE / "FROZEN.json").write_text(json.dumps(frozen, indent=2, sort_keys=True) + "\n",
                                  encoding="utf-8")
for name, data in frozen["surfaces"].items():
    if before.get(name) != data["sha256"]:
        print(f"changed: {name} ({data['count']} members)")
print("FROZEN.json rewritten")
