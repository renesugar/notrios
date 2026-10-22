#!/usr/bin/env python3
"""Rewrite docs-site/THEME_PROVENANCE.json from the vendored bytes (J31).

    python3 performance/v1.0-j31/write_provenance.py <upstream-commit>

The counts and the manifest hash are computed the same way G18g's validator
computes them, so the file describes what is on disk rather than what somebody
believed was on disk.
"""
import hashlib
import json
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
THEME = ROOT / "docs-site/themes/hugo-theme-ledger"
PROVENANCE = ROOT / "docs-site/THEME_PROVENANCE.json"


def manifest() -> tuple[int, int, str]:
    rows, total = [], 0
    for path in sorted(p for p in THEME.rglob("*") if p.is_file()):
        data = path.read_bytes()
        rows.append((path.relative_to(THEME).as_posix(), hashlib.sha256(data).hexdigest()))
        total += len(data)
    digest = hashlib.sha256()
    for relative, file_digest in rows:
        digest.update(f"{file_digest}  {relative}\n".encode())
    return len(rows), total, digest.hexdigest()


def main() -> int:
    if len(sys.argv) != 2 or len(sys.argv[1]) != 40:
        print(__doc__, file=sys.stderr)
        return 2
    count, total, digest = manifest()
    record = json.loads(PROVENANCE.read_text(encoding="utf-8"))
    record.update({"upstream_commit": sys.argv[1], "file_count": count,
                   "byte_count": total, "manifest_sha256": digest})
    PROVENANCE.write_text(json.dumps(record, indent=2) + "\n", encoding="utf-8")
    print(f"docs-site/THEME_PROVENANCE.json: {count} files, {total:,} bytes, {digest[:12]}…")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
