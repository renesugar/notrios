#!/usr/bin/env python3
"""Fail when documentation quotes an error message the program no longer produces.

    python3 scripts/check_documented_errors.py [--list]

`docs/troubleshooting.md` tells a reader what to do when they see a particular
message. That advice is only useful while the message exists, and nothing tied
the two together: v1.0 J15-D found two rows quoting errors the importers had
stopped producing since J25 and J26, each with advice those items had made
wrong. Both had passed every gate for weeks.

So each quote below is pinned to the source that must contain it. The check runs
both ways:

- the quoted fragment must still appear in the named source, so a message that
  is reworded fails here rather than in a user's terminal;
- the quote must still appear in the document, so an entry cannot rot into a
  pin on something nobody documents any more.

A quote is a *fragment*, not a whole format string: a message built with `%q`
and `%w` has no single literal a document could show. The fragment is the part a
reader would recognise and the part that would change if the meaning changed.
"""
from __future__ import annotations

import argparse
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]

# (document, quoted fragment, the source that must contain it).
#
# Only messages a document tells somebody to act on are listed. A field name, a
# configuration key or a phrase in prose is not an error message, and pinning
# one here would make this check about something other than what it says.
PINNED = [
    ("docs/troubleshooting.md",
     "no tweets.js, tweets-partN.js or tweet.js found under",
     "internal/importers/twitter/archive.go"),
    ("docs/troubleshooting.md",
     "is neither a ChatGPT export ZIP nor an extracted export folder",
     "internal/importers/chatgpt/archive.go"),
    ("docs/troubleshooting.md",
     "expected a Claude export ZIP",
     "internal/importers/claude/archive.go"),
    ("docs/troubleshooting.md",
     "not a Notrios archive (missing manifest.json)",
     "internal/archive/read.go"),
    ("docs/troubleshooting.md",
     "is in refused range",
     "internal/media/policy.go"),
    ("docs/troubleshooting.md",
     "does not match the claimed content type",
     "internal/media/fetch.go"),
    ("docs/troubleshooting.md",
     "is not a valid address or CIDR range",
     "internal/addressrange/addressrange.go"),
    ("docs/troubleshooting.md",
     "has host bits set",
     "internal/addressrange/addressrange.go"),
    ("docs/troubleshooting.md",
     "has no value and no items",
     "internal/config/security.go"),
]


def normalise(text: str) -> str:
    """Documents write typographic quotes and ellipses; sources do not."""
    for fancy, plain in (("“", '"'), ("”", '"'), ("‘", "'"),
                         ("’", "'"), ("…", "...")):
        text = text.replace(fancy, plain)
    return text


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--list", action="store_true", help="print the pinned quotes and exit")
    arguments = parser.parse_args()
    if arguments.list:
        for document, quote, source in PINNED:
            print(f"{document}: {quote!r} -> {source}")
        return 0

    problems = []
    for document, quote, source in PINNED:
        document_path, source_path = ROOT / document, ROOT / source
        if not source_path.is_file():
            problems.append(f"{source} does not exist, so {quote!r} is pinned to nothing")
            continue
        if quote not in normalise(source_path.read_text(encoding="utf-8")):
            problems.append(
                f"{source} no longer contains {quote!r}, which {document} tells a reader to expect")
        if quote not in normalise(document_path.read_text(encoding="utf-8")):
            problems.append(
                f"{document} no longer quotes {quote!r}; remove the entry from "
                f"scripts/check_documented_errors.py or restore the quote")

    for problem in problems:
        print(problem, file=sys.stderr)
    if problems:
        print(f"\n{len(problems)} documented error message(s) and their sources disagree.",
              file=sys.stderr)
        return 1
    print(f"documented error messages current: {len(PINNED)} quotes still produced by their source")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
