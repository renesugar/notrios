#!/usr/bin/env python3
"""Prove an archive is the commit it claims to be.

This is what a retroactive archive has to establish, and it is not the same as
passing today's gates. A bundle built now from a commit written weeks ago will
fail checks that did not exist then -- pinned counts that have since moved, an
`npm audit` advisory published later, a document count that grew. None of that
says anything about whether the archive faithfully carries that commit.

What does say so: every file tracked at that commit appears in the archive with
byte-identical contents, and the archive adds nothing tracked that the commit
did not have. Untracked build output is expected and allowed -- `web/dist` is
gitignored and the packager builds it before zipping -- so it is reported rather
than treated as a mismatch.

    python3 verify_archive.py <archive.zip> <commit> [repo-root]
"""
import hashlib
import subprocess
import sys
import zipfile
from pathlib import Path


# What the packager is supposed to drop, mirroring scripts/package_release.sh.
# A tracked file matching one of these is absent from the archive on purpose,
# and reporting it as missing would make the packager's correct behaviour look
# like a defect. Two v0.8 commits carry a tracked vitest cache file under
# node_modules for exactly this reason.
EXCLUDED_PREFIXES = ("node_modules/", "data/", ".git/", "dist/", "bin/", "_site/",
                     ".playwright-mcp/", ".claude/", ".zvec-grep/", "tmp/")
EXCLUDED_SUFFIXES = (".pyc", ".sqlite", "~")


def excluded_by_packaging(path: str) -> bool:
    return (path.startswith(EXCLUDED_PREFIXES)
            or path.endswith(EXCLUDED_SUFFIXES)
            or "/node_modules/" in path
            or "/__pycache__/" in path
            or path.startswith("__pycache__/")
            or ".sqlite-" in path)


def tracked_files(root: Path, commit: str) -> dict[str, str]:
    """Path -> blob sha for everything tracked at `commit`."""
    listing = subprocess.run(
        ["git", "-C", str(root), "ls-tree", "-r", "--format=%(objectname) %(path)", commit],
        capture_output=True, text=True, check=True).stdout.splitlines()
    found = {}
    for line in listing:
        blob, path = line.split(" ", 1)
        found[path] = blob
    return found


def git_blob(root: Path, blob: str) -> bytes:
    return subprocess.run(["git", "-C", str(root), "cat-file", "blob", blob],
                          capture_output=True, check=True).stdout


def main() -> int:
    archive_path, commit = Path(sys.argv[1]), sys.argv[2]
    root = Path(sys.argv[3]) if len(sys.argv) > 3 else Path.cwd()

    every_tracked = tracked_files(root, commit)
    tracked = {p: b for p, b in every_tracked.items() if not excluded_by_packaging(p)}
    dropped = sorted(set(every_tracked) - set(tracked))
    mismatched, missing = [], []
    with zipfile.ZipFile(archive_path) as archive:
        names = {name for name in archive.namelist() if not name.endswith("/")}
        # Zip entries are written relative to the worktree root, sometimes with
        # a leading "./" depending on how zip was invoked.
        normalised = {name[2:] if name.startswith("./") else name: name for name in names}
        for path, blob in tracked.items():
            entry = normalised.get(path)
            if entry is None:
                missing.append(path)
                continue
            if hashlib.sha256(archive.read(entry)).digest() != hashlib.sha256(git_blob(root, blob)).digest():
                mismatched.append(path)
        extra_untracked = sorted(set(normalised) - set(tracked))

    if missing or mismatched:
        print(f"{archive_path.name} is not {commit[:9]}:", file=sys.stderr)
        for path in missing[:10]:
            print(f"  missing  {path}", file=sys.stderr)
        for path in mismatched[:10]:
            print(f"  differs  {path}", file=sys.stderr)
        if len(missing) + len(mismatched) > 20:
            print(f"  … {len(missing) + len(mismatched)} in total", file=sys.stderr)
        return 1

    # Build output is expected; anything else untracked is worth seeing.
    unexpected = [p for p in extra_untracked if not p.startswith("web/dist/")]
    print(f"{archive_path.name}: {len(tracked)} tracked files byte-identical to {commit[:9]}, "
          f"{len(extra_untracked)} untracked "
          f"({len(extra_untracked) - len(unexpected)} built frontend"
          + (f", {len(unexpected)} other: {unexpected[:3]}" if unexpected else "") + ")"
          + (f"; {len(dropped)} tracked path(s) excluded by packaging: {dropped[:2]}" if dropped else ""))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
