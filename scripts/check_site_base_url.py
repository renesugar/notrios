#!/usr/bin/env python3
"""Fail when a tracked file names a documentation address the build does not serve.

The site is built for exactly one address: `baseURL` in `docs-site/hugo.toml`.
A build made for the apex emits root-absolute `/css/…` and is wrong under a
path, and a build made for a project path is wrong at the root. Nothing local
can test the deployment, so a sentence naming the other address survives every
gate -- v1.0 J31 found four such statements, left from the GitHub Pages era,
including the browser smoke's own default base URL, which is why the smoke had
stopped working: served under the stale path, the theme's JavaScript 404s.

So this compares what the repository says against what the build is configured
to serve:

- The apex build is served at `/`. A mention of this repository's GitHub Pages
  address, or of its `/<repo>/` path used as a base, contradicts it.
- A project-path build (`…github.io/<repo>/`) is the mirror image: a mention of
  the apex host as the served address contradicts that.

It reads the configuration; it never reaches the network. It can say the
repository agrees with itself, not what the live site serves.

A historical mention is legitimate -- the roadmap records the move, and this
file describes both shapes -- so each one is listed in HISTORY with the reason
it is not a claim about today. An unlisted mention fails, naming the file, the
line and the address the build actually serves.

    python3 scripts/check_site_base_url.py [--list]
"""
from __future__ import annotations

import argparse
import pathlib
import re
import subprocess
import sys
from urllib.parse import urlsplit

ROOT = pathlib.Path(__file__).resolve().parents[1]
HUGO_CONFIG = pathlib.Path("docs-site/hugo.toml")

# Text files worth scanning: prose, evidence scripts and configuration. Built
# output is git-ignored and never tracked, so `git ls-files` excludes it.
SUFFIXES = {".md", ".mjs", ".js", ".py", ".sh", ".toml", ".json", ".yaml", ".yml", ".html", ".go"}

# Mentions that are history rather than a claim about the served address. Each
# entry is (path or path prefix ending in "/", the text that must appear on the
# line or "*" for any, and why it is not a claim about today).
#
# Whole directories are listed where every file in them is a dated record: an
# archived plan and a milestone's evidence say what was true when they were
# written, and rewriting them to match today would destroy the record. A live
# tool that happens to live in such a directory is not covered by that -- it
# has to be right, which is why performance/v0.7-g18g/browser_smoke.mjs was
# corrected rather than listed here.
HISTORY = [
    ("plans/", "*", "archived plan documents record the address of their milestone"),
    ("performance/v0.7-", "*", "v0.7 evidence, recorded when the project path was the address"),
    ("performance/v0.9-", "*", "v0.9 evidence, recorded around the move to the apex"),
    ("performance/v1.0-j31/", "*", "records the stale subpath the browser smoke could not serve"),
    ("performance/v1.0-j35/", "*", "records this correction and what it measured"),
    ("ROADMAP.md", "*", "records the v0.9 I10 move, the fallback address, and how each build shape is previewed"),
    ("PLAN.md", "*", "v1.0 J31 and J35 record the stale subpath and its correction"),
    ("CODING_CLIENT_HANDOFF.md", "/notrios/", "records what the v0.7 G18b slice measured at the time"),
    ("agent/PLAN_STATUS.md", "/notrios/", "records what the v0.7 G18b slice measured at the time"),
    ("scripts/check_site_base_url.py", "*", "this check names both shapes in order to compare them"),
    ("scripts/test_check_site_base_url.py", "*", "the test of this check names both shapes"),
]

class Finding(tuple):
    """(path, line number, line, why it contradicts the served address)."""


def served_base(root: pathlib.Path) -> str:
    """The address the site is built for, from the one file that decides it."""
    text = (root / HUGO_CONFIG).read_text(encoding="utf-8")
    match = re.search(r'^\s*baseURL\s*=\s*"([^"]+)"', text, re.MULTILINE)
    if not match:
        raise SystemExit(f"{HUGO_CONFIG} states no baseURL")
    return match.group(1)


def repository_slug(root: pathlib.Path) -> tuple[str, str]:
    """The owner and repository name, from the origin remote."""
    url = subprocess.run(["git", "-C", str(root), "remote", "get-url", "origin"],
                         capture_output=True, text=True, check=True).stdout.strip()
    match = re.search(r"[:/]([^/:]+)/([^/]+?)(?:\.git)?$", url)
    if not match:
        raise SystemExit(f"cannot read an owner and repository from {url!r}")
    return match.group(1), match.group(2)


def custom_domain_of(root: pathlib.Path) -> str:
    """The custom domain the site is published under, if it declares one."""
    cname = root / "docs-site/static/CNAME"
    return cname.read_text(encoding="utf-8").strip() if cname.is_file() else ""


def contradictions(base: str, owner: str, repo: str, custom_domain: str = "") -> list[tuple[re.Pattern[str], str]]:
    """Patterns that contradict `base`, each with what the mention would mean.

    Only address-shaped mentions count. The repository name appears in
    filesystem paths (`~/.config/notrios/`, `cmd/notrios/`) many times a page,
    and none of those says anything about where the site is served.
    """
    split = urlsplit(base)
    name = re.escape(repo)
    if split.path.strip("/"):
        # A project-path build. What contradicts it is the custom domain, named
        # by the site's CNAME, presented as the address being served.
        if not custom_domain:
            return []
        host = re.escape(custom_domain)
        return [(re.compile(rf"https?://{host}(?![\w.-])"),
                 f"the build is made for {base}, a project path, so {custom_domain} is not what it serves")]
    return [
        # The Pages address, in any host spelling: owner.github.io/<repo>.
        (re.compile(rf"[\w.-]*\.github\.io/{name}"),
         f"the build is made for {base}; the Pages address is not what it serves"),
        # Any http(s) URL whose path starts with the repository name, which is
        # what a project-path base looks like, including a local preview.
        (re.compile(rf"https?://[^\s\"\'`)\]]+/{name}/"),
         f"the build is made for {base}, which is served at the root"),
        # The bare path in prose or a value, as `/<repo>/` or "/<repo>/".
        (re.compile(rf"[`\"\']/{name}/[`\"\']"),
         f"the build is made for {base}, so pages use root-absolute paths"),
    ]


def allowed(path: str, line: str) -> bool:
    for where, text, _ in HISTORY:
        matches_path = path.startswith(where) if where.endswith("/") or where.startswith("performance/v") else path == where
        if matches_path and (text == "*" or text in line):
            return True
    return False


def tracked_files(root: pathlib.Path) -> list[str]:
    listing = subprocess.run(["git", "-C", str(root), "ls-files"],
                             capture_output=True, text=True, check=True).stdout.splitlines()
    return [name for name in listing if pathlib.Path(name).suffix in SUFFIXES]


def scan(root: pathlib.Path) -> tuple[str, list[Finding]]:
    base = served_base(root)
    owner, repo = repository_slug(root)
    patterns = contradictions(base, owner, repo, custom_domain_of(root))
    findings: list[Finding] = []
    for name in tracked_files(root):
        try:
            text = (root / name).read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            continue
        for number, line in enumerate(text.splitlines(), start=1):
            for pattern, why in patterns:
                if pattern.search(line) and not allowed(name, line):
                    findings.append(Finding((name, number, line.strip()[:160], why)))
                    break
    return base, findings


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--list", action="store_true", help="print the allowed historical mentions and exit")
    arguments = parser.parse_args()
    if arguments.list:
        for where, text, why in HISTORY:
            print(f"{where}: {text!r} — {why}")
        return 0
    base, findings = scan(ROOT)
    for name, number, line, why in findings:
        print(f"{name}:{number}: {why}\n  {line}", file=sys.stderr)
    if findings:
        print(f"\n{len(findings)} mention(s) contradict the address the site is built for ({base}).\n"
              "Correct them, or add the line to HISTORY in scripts/check_site_base_url.py with the\n"
              "reason it is history rather than a claim about what is served.", file=sys.stderr)
        return 1
    print(f"documentation addresses agree with the built address {base}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
