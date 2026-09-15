#!/usr/bin/env python3
"""Digest what a library holds as notes, independent of schema version.

    python3 performance/v1.0-j7/content_digest.py library.sqlite [more.sqlite ...]

J7 compares libraries written or migrated by different versions. A table-by-
table comparison (performance/v1.0-j17/j17_compare.py) fails on those by
design, because a v27 and a v28 library differ in private tables such as
documents_fts_rowid. This digests only what a user's notes consist of, in
columns every compared version has:

  documents   id, collection, notebook, title, MIME type, trashed or not
  bodies      each document's current revision body
  notebooks   id, parent, name
  tags        name, and which documents carry it
  links       source, target document or resource, relation, raw target
  resources   id, filename, MIME type, content SHA-256
  sources     document, source system, external id

Row identifiers that are random per library (revision ids, database and replica
identity) and every timestamp are left out. Each section prints its row count
and SHA-256, so a difference names the section. Libraries are opened read-only.
"""
import hashlib
import sqlite3
import sys

SECTIONS = {
    "documents": """SELECT id, collection_id, notebook_id, title, body_mime_type,
                           CASE WHEN deleted_at IS NULL OR deleted_at = '' THEN 0 ELSE 1 END
                    FROM documents""",
    "bodies": """SELECT d.id, r.body FROM documents d
                 JOIN document_revisions r ON r.id = d.current_revision_id""",
    "notebooks": "SELECT id, coalesce(parent_id, ''), name FROM notebooks",
    "tags": """SELECT t.name, nt.document_id FROM note_tags nt JOIN tags t ON t.id = nt.tag_id""",
    "links": """SELECT source_document_id, coalesce(target_document_id, ''),
                       coalesce(target_resource_id, ''), relation_type, raw_target
                FROM document_links""",
    "resources": "SELECT id, filename, mime_type, blob_sha256 FROM resources",
    "sources": "SELECT document_id, source_system, external_id FROM document_sources",
}


def digest(path):
    conn = sqlite3.connect(f"file:{path}?mode=ro", uri=True)
    try:
        version = conn.execute("PRAGMA user_version").fetchone()[0]
        result = {}
        for name, sql in SECTIONS.items():
            try:
                rows = sorted(repr(tuple("" if v is None else v for v in row)) for row in conn.execute(sql))
            except sqlite3.Error as error:
                result[name] = (None, f"unreadable: {error}")
                continue
            result[name] = (len(rows), hashlib.sha256("\n".join(rows).encode()).hexdigest())
    finally:
        conn.close()
    return version, result


def main():
    if len(sys.argv) < 2:
        print(__doc__.strip(), file=sys.stderr)
        return 2
    whole = {}
    for path in sys.argv[1:]:
        version, sections = digest(path)
        overall = hashlib.sha256("".join(f"{k}:{v[0]}:{v[1]}\n" for k, v in sorted(sections.items())).encode()).hexdigest()
        whole[path] = overall
        print(f"{overall}  user_version={version}  {path}")
        for name, (count, value) in sections.items():
            print(f"    {name:10} rows={count}  {value}")
    if len(set(whole.values())) > 1:
        print("content differs")
        return 1
    if len(whole) > 1:
        print("content identical")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
