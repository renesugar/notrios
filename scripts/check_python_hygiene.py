#!/usr/bin/env python3
"""Check that the repository runs Python without leaving bytecode behind.

Two different checks, deliberately separated, because one is a fact about
tracked files and the other is a fact about this machine right now.

`--sources` is a gate. Every tracked shell script that invokes Python must
export PYTHONDONTWRITEBYTECODE and PYTHONUNBUFFERED, and so must the Makefile
and every workflow. Twenty scripts were edited by hand to add that line, which
is exactly the situation where the twenty-first is forgotten -- so the
requirement is checked rather than remembered.

`--byproducts` is a report, and reports rather than fails on purpose. Leftover
__pycache__ is a byproduct in a git-ignored path: it is untidy, not wrong, and a
gate that refused a commit over it would be switched off inside a week. It is
also not the hazard it looks like. An orphaned __pycache__ entry whose source
has been deleted is not importable at all (PEP 3147), and a cache whose source
changed is invalidated and recompiled -- both measured before this was written.
What remains is tidiness, which `make clean` already handles.

    python3 scripts/check_python_hygiene.py --sources      # gate
    python3 scripts/check_python_hygiene.py --byproducts   # report
"""
from __future__ import annotations

import argparse
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
REQUIRED = ("PYTHONDONTWRITEBYTECODE", "PYTHONUNBUFFERED")

# A script that mentions python without running it -- in a comment, or in a
# message telling somebody else what to run -- is not invoking it. The pattern
# looks for a command position rather than the word.
INVOKES = re.compile(r"(?:^|[;&|(]|\$\(|\bexec\s+|\bthen\s+|\belse\s+|\bdo\s+)\s*python3?\b", re.M)


def tracked(pattern: str) -> list[pathlib.Path]:
    out = subprocess.run(["git", "ls-files", pattern], cwd=ROOT,
                         capture_output=True, text=True, check=True).stdout
    return [ROOT / line for line in out.split() if line]


def check_sources() -> list[str]:
    problems: list[str] = []

    for path in tracked("*.sh"):
        text = path.read_text(encoding="utf-8")
        # Comments are stripped before asking whether Python is invoked, so a
        # script that only *documents* a python command is not required to
        # export anything.
        code = "\n".join(re.sub(r"(?<!\$)#.*$", "", line) for line in text.splitlines())
        if not INVOKES.search(code):
            continue
        missing = [name for name in REQUIRED if f"export {name}" not in text
                   and not re.search(rf"export [A-Z0-9_= ]*\b{name}=", text)]
        if missing:
            problems.append(f"{path.relative_to(ROOT)} invokes Python without exporting "
                            + ", ".join(missing))

    makefile = (ROOT / "Makefile").read_text(encoding="utf-8")
    for name in REQUIRED:
        if not re.search(rf"^export {name}\s*[:?]?=", makefile, re.M):
            problems.append(f"Makefile does not export {name}, so no recipe inherits it")

    for path in tracked(".github/workflows/*.yml"):
        text = path.read_text(encoding="utf-8")
        for name in REQUIRED:
            if name not in text:
                problems.append(f"{path.relative_to(ROOT)} does not set {name}")

    return problems


def check_byproducts() -> int:
    directories = [p for p in ROOT.rglob("__pycache__")
                   if p.is_dir() and ".git" not in p.parts and "node_modules" not in p.parts]
    files = [p for p in ROOT.rglob("*.pyc")
             if ".git" not in p.parts and "node_modules" not in p.parts]
    if not directories and not files:
        print("no bytecode in the tree")
        return 0
    print(f"bytecode present: {len(directories)} __pycache__ director"
          f"{'y' if len(directories) == 1 else 'ies'} and {len(files)} .pyc file(s).")
    for directory in sorted(directories)[:5]:
        print(f"  {directory.relative_to(ROOT)}")
    if len(directories) > 5:
        print(f"  ... and {len(directories) - 5} more")
    print("Left by a Python run that did not have PYTHONDONTWRITEBYTECODE set -- an editor,")
    print("an IDE, or a bare `python3 scripts/...`. `make clean` removes them, and")
    print("`python3 scripts/check_python_hygiene.py --sources` says whether the repository's")
    print("own entry points are the cause.")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sources", action="store_true",
                        help="fail if an entry point can write bytecode")
    parser.add_argument("--byproducts", action="store_true",
                        help="report bytecode currently in the tree")
    arguments = parser.parse_args()
    if not arguments.sources and not arguments.byproducts:
        parser.error("choose --sources or --byproducts")

    status = 0
    if arguments.sources:
        problems = check_sources()
        for problem in problems:
            print(problem, file=sys.stderr)
        if problems:
            print(f"\n{len(problems)} entry point(s) can write bytecode. Add this after the "
                  "`set -` line:\n"
                  "  export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1", file=sys.stderr)
            status = 1
        else:
            print("every Python entry point exports PYTHONDONTWRITEBYTECODE and PYTHONUNBUFFERED")
    if arguments.byproducts:
        status = check_byproducts() or status
    return status


if __name__ == "__main__":
    raise SystemExit(main())
