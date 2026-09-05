"""The interface's control signature: what would change the control inventory.

The crawl is a report and not a build gate, for reasons recorded in PLAN.md
under H15 -- it needs Playwright, a browser, built assets and a running service,
and `make validate` has to stay runnable offline. That decision leaves one hole,
and it is the hole the crawl exists to close: somebody adds a button, never runs
the crawl, and the pinned inventory keeps passing because it is a committed file
describing an interface that has moved.

This closes it without a browser. It is not a second measurement of the
interface -- it cannot be, since it never runs anything -- it is a signature of
the things that decide what the crawl would find: test ids, interactive
elements, and the roles that make a non-element behave as one. Prose, styling
and comments do not move it, so an ordinary edit does not force a re-crawl,
while adding, removing or renaming a control does.

The signature is recorded by the crawl at the moment it measured, and checked
here. A mismatch is never a claim about what changed; it is an instruction to
run the crawl again, which is the only thing that can say.
"""
import hashlib
import pathlib
import re

# Written once and copied verbatim into gui_controls.mjs. Two implementations of
# one definition is the risk here, and it is taken deliberately: the alternative
# is a browser in `make validate`. The failure mode is safe -- an implementation
# that drifts produces a mismatch, which stops the build and asks for a crawl,
# rather than a quiet pass.
TOKENS = re.compile(
    r'data-testid\s*=\s*(?:"[^"]*"|\'[^\']*\'|\{[^}]*\})'
    r'|<button|<input|<select|<textarea|<a\s'
    r'|role="button"|role="tab"')


def interface_files(web_src: pathlib.Path):
    """The interface sources a control can appear in, tests excluded."""
    found = []
    for path in sorted(web_src.rglob("*")):
        if path.suffix not in (".ts", ".tsx") or not path.is_file():
            continue
        if "__tests__" in path.parts:
            continue
        found.append(path)
    return found


def interface_signature(web_src: pathlib.Path) -> str:
    lines = []
    for path in interface_files(web_src):
        text = path.read_text(encoding="utf-8")
        relative = path.relative_to(web_src.parent.parent).as_posix()
        lines.append(relative + "|" + "|".join(TOKENS.findall(text)))
    return hashlib.sha256("\n".join(lines).encode("utf-8")).hexdigest()
