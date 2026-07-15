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
    "MVP_RELEASE_REPORT.md",
    "cmd/notesd/main.go",
    "cmd/notesctl/main.go",
    "web/dist/index.html",
}
forbidden_prefixes = (
    "web/node_modules/",
    "data/",
    ".git/",
)

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
    forbidden = sorted(name for name in names if name.startswith(forbidden_prefixes))
    if forbidden:
        print("forbidden zip entries present:")
        for name in forbidden[:50]:
            print(f"  - {name}")
        if len(forbidden) > 50:
            print(f"  ... and {len(forbidden)-50} more")
        sys.exit(1)

print(f"release zip looks complete: {zip_path} ({zip_path.stat().st_size} bytes, {len(names)} entries)")
