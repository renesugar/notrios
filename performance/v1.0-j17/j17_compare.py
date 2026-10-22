#!/usr/bin/env python3
"""Compare two Notrios libraries table by table, ignoring only timestamp columns.
A mismatched table is re-compared column by column, so the report names what
differs and a reader can judge whether it is volatile (a random ID) or real."""
import hashlib
import sqlite3
import sys

a_path, b_path = sys.argv[1], sys.argv[2]
SKIP_TABLE_PREFIXES = ("sqlite_", "documents_fts_")  # FTS shadow storage depends on rowid order

def open_ro(path):
    return sqlite3.connect(f"file:{path}?mode=ro", uri=True)

a, b = open_ro(a_path), open_ro(b_path)

def tables(conn):
    return {r[0] for r in conn.execute("select name from sqlite_master where type='table'")}

def columns(conn, table):
    return [r[1] for r in conn.execute(f"select * from pragma_table_info('{table}')")]

def digest(conn, table, cols):
    q = ",".join(f'"{c}"' for c in cols)
    rows = sorted(repr(r) for r in conn.execute(f'select {q} from "{table}"'))
    return len(rows), hashlib.sha256("\n".join(rows).encode()).hexdigest()

only = tables(a) ^ tables(b)
if only:
    print("tables in only one library:", sorted(only))
failures = 0
for table in sorted(tables(a) & tables(b)):
    if table.startswith(SKIP_TABLE_PREFIXES) and table != "documents_fts_rowid":
        continue
    cols = [c for c in columns(a, table) if not c.endswith("_at") and c != "processed_at"]
    if table == "documents_fts_rowid":
        cols = ["document_id"]
    try:
        na, da = digest(a, table, cols)
        nb, db = digest(b, table, cols)
    except sqlite3.Error as err:
        print(f"{table}: skipped ({err})")
        continue
    if na == 0 and nb == 0:
        continue
    if da == db:
        print(f"same  {table} ({na} rows)")
        continue
    failures += 1
    differing = [c for c in cols if digest(a, table, [c]) != digest(b, table, [c])]
    print(f"DIFF  {table} ({na} vs {nb} rows) columns: {differing}")
print("failures:", failures)
