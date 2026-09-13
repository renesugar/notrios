-- Schema v28: stop scanning the full-text index on every document write.
--
-- `documents_fts` declares `document_id UNINDEXED`, so FTS5 builds no index on
-- it and `DELETE FROM documents_fts WHERE document_id = ?` plans as
-- `SCAN documents_fts VIRTUAL TABLE`. Every document write therefore scanned
-- the whole index. Measured in v1.0 J18: 156 ms per write at 60 notes, 490 ms
-- at 103,349 and 2,087 ms at 382,206, of which the scan alone was 2,068 ms.
-- Deleting by rowid on the same library takes 19 ms.
--
-- FTS5 cannot be given an index on a column, so the rowid it assigns is
-- recorded here instead. This table is a private index, not protocol state: it
-- carries nothing a peer sends or receives and nothing a user wrote, and it can
-- be rebuilt from `documents_fts` at any time by the statement below.
CREATE TABLE IF NOT EXISTS documents_fts_rowid (
    document_id TEXT PRIMARY KEY,
    fts_rowid   INTEGER NOT NULL
);

-- One scan, once, at migration time -- against a scan on every write forever.
-- On the 382,206-note library this reads the index the same way a single write
-- used to.
INSERT OR REPLACE INTO documents_fts_rowid (document_id, fts_rowid)
    SELECT document_id, rowid FROM documents_fts;

CREATE INDEX IF NOT EXISTS documents_fts_rowid_idx
    ON documents_fts_rowid (fts_rowid);

PRAGMA user_version = 28;
