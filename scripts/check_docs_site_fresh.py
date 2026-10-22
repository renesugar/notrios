#!/usr/bin/env python3
"""Say whether _site/ was built after the documentation it renders.

_site/ is git-ignored and rebuilt only by `make docs` or `make g18g-validate`,
and the latter is reached only by `scripts/package_release.sh`. So a checkout
can carry months-old rendered documentation while every gate is green -- which
is exactly what happened: the owner found a `_site` whose newest page was
thirteen days older than the documents it was built from, with `make validate`
passing throughout.

This reports rather than fails by default. A stale git-ignored build directory
is not a reason to refuse a commit, and a gate that blocked on it would be
turned off within a week. `--fail-stale` is there for a caller that wants the
opposite, and `scripts/package_release.sh` does not need it because it rebuilds
the site outright.

Modification times are the only signal available. They are not a content hash
and this does not pretend otherwise: a `touch` on every file under docs/ would
report staleness that is not real, and a rebuild followed by an editor writing
the same bytes back would be reported as fresh. What it catches is the case that
actually occurs -- documents edited, site never rebuilt -- and it names the
newest offending document so the answer is checkable rather than a verdict.
"""
from __future__ import annotations

import argparse
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
DOCS = ROOT / "docs"
SITE = ROOT / "_site"


def newest(directory: pathlib.Path, suffixes: tuple[str, ...]) -> tuple[float, pathlib.Path | None]:
    newest_time = 0.0
    newest_path: pathlib.Path | None = None
    for path in directory.rglob("*"):
        if not path.is_file() or (suffixes and path.suffix not in suffixes):
            continue
        stamp = path.stat().st_mtime
        if stamp > newest_time:
            newest_time, newest_path = stamp, path
    return newest_time, newest_path


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--fail-stale", action="store_true",
                        help="exit non-zero when the site is older than the documents")
    arguments = parser.parse_args()

    if not SITE.is_dir():
        print("_site/ has not been built; run: make docs")
        return 1 if arguments.fail_stale else 0

    source_time, source_path = newest(DOCS, (".md",))
    built_time, _ = newest(SITE, (".html",))
    if source_path is None:
        print("no documents under docs/, which is itself wrong")
        return 1
    if built_time == 0.0:
        print("_site/ holds no rendered pages; run: make docs")
        return 1 if arguments.fail_stale else 0

    if built_time >= source_time:
        print(f"_site/ is newer than every document under docs/ "
              f"(newest document: {source_path.relative_to(ROOT)})")
        return 0

    print(f"_site/ is stale: {source_path.relative_to(ROOT)} is "
          f"{interval(source_time - built_time)} newer than the newest rendered page. "
          "Run: make docs")
    return 1 if arguments.fail_stale else 0


def interval(seconds: float) -> str:
    """A gap in the largest unit that does not round to zero.

    "0.0 day(s)" was the first thing this printed, for a document edited a
    minute after the build -- true, useless, and the kind of number a reader
    stops believing.
    """
    for size, unit in ((86400, "day"), (3600, "hour"), (60, "minute")):
        if seconds >= size:
            value = seconds / size
            return f"{value:.1f} {unit}s" if value >= 2 else f"{value:.1f} {unit}"
    return f"{seconds:.0f} seconds"


if __name__ == "__main__":
    raise SystemExit(main())
