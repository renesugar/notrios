#!/usr/bin/env python3
"""Refuse a workflow that trusts a moving tag or takes more permission than it needs.

Two properties, both of which decay silently.

**Every action is pinned to a commit digest.** `actions/checkout@v4` is a tag,
and a tag is a pointer somebody else can move: whoever controls it chooses what
runs in this repository's CI, with this repository's token. Pinning to a digest
means an upgrade is a commit here, reviewable like any other. The tag is kept as
a trailing comment, because a bare digest tells a reader nothing about which
version they are looking at.

**Every workflow declares its permissions.** A workflow with no `permissions:`
block inherits the repository default, which can be read/write across every
scope -- so a job that only builds and tests would hold a token that can push to
the repository. Declaring `contents: read` costs a line and removes that.

Read as text rather than parsed as YAML, so `make validate` gains no dependency.
"""
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / ".github" / "workflows"
USES = re.compile(r"^\s*-?\s*uses:\s*(\S+)")
DIGEST = re.compile(r"^[^@]+@[0-9a-f]{40}$")


def main() -> int:
    problems: list[str] = []
    if not WORKFLOWS.is_dir():
        print("no workflows to check")
        return 0
    files = sorted(WORKFLOWS.glob("*.y*ml"))
    pinned = 0
    for path in files:
        name = path.relative_to(ROOT).as_posix()
        text = path.read_text(encoding="utf-8")

        for line in text.splitlines():
            match = USES.match(line)
            if not match:
                continue
            ref = match.group(1)
            if ref.startswith("./") or ref.startswith("docker://"):
                continue  # a local action, or an image pinned by its own digest rules
            if not DIGEST.match(ref):
                problems.append(f"{name}: {ref} is not pinned to a commit digest; a tag is a "
                                f"pointer somebody else can move")
            else:
                pinned += 1

        if not re.search(r"^permissions:", text, re.MULTILINE) and "permissions:" not in text:
            problems.append(f"{name}: declares no permissions, so it inherits the repository "
                            f"default, which can be read/write on every scope")
        if re.search(r"permissions:\s*write-all", text):
            problems.append(f"{name}: permissions: write-all is not least privilege")

    if problems:
        for problem in problems:
            print(f"workflow hardening: {problem}", file=sys.stderr)
        return 1
    print(f"workflow hardening passed: {len(files)} workflows, {pinned} actions pinned by digest, "
          f"permissions declared")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
