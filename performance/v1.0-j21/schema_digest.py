#!/usr/bin/env python3
"""Print a digest of each library's schema, so two builds can be compared.

    python3 performance/v1.0-j21/schema_digest.py a.sqlite [b.sqlite ...]

The digest covers every row of sqlite_master (type, name, table, SQL text)
and user_version. It leaves out rootpage, which depends on the order pages
were allocated in, not on what the schema is. Libraries are opened read-only.
"""
import hashlib
import sqlite3
import sys


def schema(path):
    conn = sqlite3.connect(f"file:{path}?mode=ro", uri=True)
    try:
        rows = conn.execute(
            "SELECT type, name, tbl_name, coalesce(sql, '') FROM sqlite_master "
            "ORDER BY type, name").fetchall()
        version = conn.execute("PRAGMA user_version").fetchone()[0]
    finally:
        conn.close()
    return rows, version


def main():
    if len(sys.argv) < 2:
        print(__doc__.strip(), file=sys.stderr)
        return 2
    for path in sys.argv[1:]:
        rows, version = schema(path)
        text = "\n".join("\x1f".join(row) for row in rows) + f"\nuser_version={version}\n"
        digest = hashlib.sha256(text.encode()).hexdigest()
        print(f"{digest}  objects={len(rows)}  user_version={version}  {path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
