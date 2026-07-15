-- Initial schema for the Step 4 runnable persistence slice and planned MVP tables.
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS collections (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,
    collection_id TEXT NOT NULL REFERENCES collections(id),
    title TEXT NOT NULL,
    body_mime_type TEXT NOT NULL DEFAULT 'text/markdown',
    current_revision_id TEXT,
    deleted_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS document_revisions (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id),
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    body_mime_type TEXT NOT NULL DEFAULT 'text/markdown',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    message TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
    document_id UNINDEXED,
    collection_id UNINDEXED,
    title,
    body,
    tokenize = 'unicode61'
);

CREATE TABLE IF NOT EXISTS blobs (
    sha256 TEXT PRIMARY KEY,
    storage_path TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    mime_type TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS resources (
    id TEXT PRIMARY KEY,
    collection_id TEXT NOT NULL REFERENCES collections(id),
    blob_sha256 TEXT NOT NULL REFERENCES blobs(sha256),
    filename TEXT,
    mime_type TEXT NOT NULL,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS resources_collection_idx ON resources(collection_id);
CREATE INDEX IF NOT EXISTS resources_blob_idx ON resources(blob_sha256);

CREATE TABLE IF NOT EXISTS document_resource_refs (
    document_id TEXT NOT NULL REFERENCES documents(id),
    resource_id TEXT NOT NULL REFERENCES resources(id),
    relation_type TEXT NOT NULL,
    ordinal INTEGER NOT NULL DEFAULT 0,
    anchor_json TEXT NOT NULL DEFAULT '{}',
    PRIMARY KEY(document_id, resource_id, relation_type, ordinal)
);

CREATE INDEX IF NOT EXISTS document_resource_refs_document_idx ON document_resource_refs(document_id);
CREATE INDEX IF NOT EXISTS document_resource_refs_resource_idx ON document_resource_refs(resource_id);

CREATE TABLE IF NOT EXISTS document_links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_document_id TEXT NOT NULL REFERENCES documents(id),
    target_document_id TEXT REFERENCES documents(id),
    target_resource_id TEXT REFERENCES resources(id),
    target_uri TEXT,
    relation_type TEXT NOT NULL,
    source_format TEXT NOT NULL,
    raw_target TEXT NOT NULL,
    display_text TEXT,
    anchor_type TEXT,
    anchor_value TEXT,
    context TEXT,
    source_start_byte INTEGER,
    source_end_byte INTEGER,
    source_line INTEGER,
    source_column INTEGER,
    resolution_status TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS document_links_source_idx ON document_links(source_document_id);
CREATE INDEX IF NOT EXISTS document_links_target_document_idx ON document_links(target_document_id);
CREATE INDEX IF NOT EXISTS document_links_target_resource_idx ON document_links(target_resource_id);

CREATE TABLE IF NOT EXISTS media_policy_decisions (
    id TEXT PRIMARY KEY,
    document_id TEXT REFERENCES documents(id),
    original_url TEXT NOT NULL,
    final_url TEXT,
    decision TEXT NOT NULL,
    reason TEXT,
    exact_hash TEXT,
    perceptual_hash TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS index_outbox (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    object_type TEXT NOT NULL,
    object_id TEXT NOT NULL,
    operation TEXT NOT NULL,
    projection_path TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    error_text TEXT
);

PRAGMA user_version = 4;
