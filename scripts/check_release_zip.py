#!/usr/bin/env python3
from pathlib import Path
import sys
import zipfile

if len(sys.argv) != 2:
    print("usage: check_release_zip.py <zip-path>", file=sys.stderr)
    sys.exit(2)

zip_path = Path(sys.argv[1])
if not zip_path.exists():
    print(f"zip not found: {zip_path}", file=sys.stderr)
    sys.exit(1)

required_exact = {
    "README.md",
    "PLAN.md",
    "ROADMAP.md",
    "API_SPEC.md",
    "DATABASE_SCHEMA.md",
    "PACKAGING.md",
    "SECURITY_REVIEW.md",
    "plans/mvp/MVP_RELEASE_REPORT.md",
    "cmd/notriosd/main.go",
    "cmd/notriosctl/main.go",
    "web/dist/index.html",
    "docs-site/hugo.toml",
    "docs-site/package-lock.json",
    "docs-site/themes/hugo-theme-ledger/LICENSE",
    "performance/v0.7-g18g/REPORT.json",
    "plans/v0.7/041-hugo-ledger-production-site.md",
}
# Anchored at the archive root: these directories only ever exist there.
forbidden_prefixes = (
    "web/node_modules/",
    ".git/",
    ".claude/",
    ".playwright-mcp/",
    "_site/",
    "bin/",
    "dist/",
)

# Matched at ANY depth. A runtime `data/` is created wherever the service or a
# test happens to be running, so `internal/service/data/` and
# `cmd/notriosctl/data/` are just as real as the one at the root — and a
# root-anchored check passed them straight through. `.gitignore` already treats
# `data/` this way, which is why git ignored the same directories the archive
# was shipping.
forbidden_segments = (
    "node_modules/",
    "data/",
    "quarantine/",
    "search-index/",
    "projections/",
)


def has_forbidden_segment(name: str) -> bool:
    return any(("/" + name).find("/" + segment) >= 0 for segment in forbidden_segments)

with zipfile.ZipFile(zip_path) as zf:
    names = set(zf.namelist())
    missing = sorted(required_exact - names)
    if missing:
        print("missing required zip entries:")
        for name in missing:
            print(f"  - {name}")
        sys.exit(1)
    if not any(name.startswith("web/dist/assets/") for name in names):
        print("missing web/dist/assets/ entries")
        sys.exit(1)
    forbidden = sorted(
        name
        for name in names
        if name.startswith(forbidden_prefixes)
        or has_forbidden_segment(name)
        or "__pycache__/" in name
        or name.endswith((".pyc", ".sqlite", ".zip", "~"))
    )
    if forbidden:
        print("forbidden zip entries present:")
        for name in forbidden[:50]:
            print(f"  - {name}")
        if len(forbidden) > 50:
            print(f"  ... and {len(forbidden)-50} more")
        sys.exit(1)

print(f"release zip looks complete: {zip_path} ({zip_path.stat().st_size} bytes, {len(names)} entries)")
