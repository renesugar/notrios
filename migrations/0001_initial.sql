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
    notebook_id TEXT REFERENCES notebooks(id),
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

-- Schema v5: notebooks, tags, and search notebooks (Notrios redesign task R3).
CREATE TABLE IF NOT EXISTS notebooks (
    id TEXT PRIMARY KEY,
    parent_id TEXT REFERENCES notebooks(id),
    name TEXT NOT NULL,
    icon_emoji TEXT NOT NULL DEFAULT '',
    builtin INTEGER NOT NULL DEFAULT 0,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Notebook names are case-insensitively unique among siblings.
CREATE UNIQUE INDEX IF NOT EXISTS notebooks_sibling_name_idx
    ON notebooks(COALESCE(parent_id, ''), name COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS tags (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS tags_name_idx ON tags(name COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS note_tags (
    document_id TEXT NOT NULL REFERENCES documents(id),
    tag_id TEXT NOT NULL REFERENCES tags(id),
    PRIMARY KEY(document_id, tag_id)
);

CREATE INDEX IF NOT EXISTS note_tags_tag_idx ON note_tags(tag_id);

-- Search notebooks are query-backed virtual notebooks; deleting one never
-- deletes notes. sort_anchor is 'first' (All notes), 'normal', or 'last' (Trash).
CREATE TABLE IF NOT EXISTS search_notebooks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    icon_emoji TEXT NOT NULL DEFAULT '',
    query TEXT NOT NULL DEFAULT '',
    builtin INTEGER NOT NULL DEFAULT 0,
    sort_anchor TEXT NOT NULL DEFAULT 'normal',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS search_notebooks_name_idx
    ON search_notebooks(name COLLATE NOCASE);

PRAGMA user_version = 5;

-- Schema v6: source provenance and conversation threads (Notrios redesign task R4).
-- One row per externally-sourced document (Joplin, Obsidian, Twitter/X, ChatGPT,
-- Claude, ...). Local notes have no row; only local notes may be purged from Trash.
CREATE TABLE IF NOT EXISTS document_sources (
    document_id TEXT PRIMARY KEY REFERENCES documents(id),
    source_system TEXT NOT NULL,
    external_id TEXT NOT NULL DEFAULT '',
    author TEXT NOT NULL DEFAULT '',
    author_id TEXT NOT NULL DEFAULT '',
    thread_id TEXT NOT NULL DEFAULT '',
    reply_to TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    published_at TEXT NOT NULL DEFAULT '',
    published_ts INTEGER,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS document_sources_system_external_idx
    ON document_sources(source_system, external_id);
CREATE INDEX IF NOT EXISTS document_sources_thread_idx ON document_sources(thread_id);
CREATE INDEX IF NOT EXISTS document_sources_author_idx ON document_sources(author_id);

PRAGMA user_version = 6;
