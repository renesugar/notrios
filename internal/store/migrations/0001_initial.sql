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

CREATE INDEX IF NOT EXISTS documents_collection_state_updated_idx
    ON documents(collection_id, deleted_at, updated_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS documents_trash_deleted_idx
    ON documents(deleted_at DESC, id DESC)
    WHERE deleted_at IS NOT NULL;

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
    unreferenced_at TEXT,
    unreferenced_reason TEXT NOT NULL DEFAULT '',
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
    error_text TEXT,
    next_attempt_at TEXT
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
CREATE INDEX IF NOT EXISTS note_tags_tag_document_idx ON note_tags(tag_id, document_id);

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

-- Schema v7: media-policy hardening (v0.3 task H1). User-managed domain and
-- hash rules complement the remote_media config lists; resource_hashes holds
-- additional (including future perceptual) hashes per stored blob. The
-- media_policy_decisions table above gains quarantine-state columns via the
-- v7 upgrade shim (ALTERs are not idempotent, so they cannot live here).
CREATE TABLE IF NOT EXISTS media_domain_rules (
    id TEXT PRIMARY KEY,
    pattern TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('allow', 'block', 'review')),
    note TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS media_domain_rules_pattern_idx
    ON media_domain_rules(pattern COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS media_hash_rules (
    algo TEXT NOT NULL,
    hash TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('exact', 'perceptual')),
    action TEXT NOT NULL CHECK (action IN ('block', 'review')),
    reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (algo, hash)
);

CREATE TABLE IF NOT EXISTS resource_hashes (
    blob_sha256 TEXT NOT NULL,
    algo TEXT NOT NULL,
    hash TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (blob_sha256, algo)
);

CREATE INDEX IF NOT EXISTS resource_hashes_hash_idx ON resource_hashes(algo, hash);

PRAGMA user_version = 7;

-- Schema v8: retention-aware resource garbage collection (v0.3 task H6).
-- Upgrade backfill and the index are applied by ensureSchemaV8 so existing
-- databases and fresh databases share one idempotent path.
PRAGMA user_version = 8;

-- Schema v9: scalable keyset traversal and filter-supporting indexes
-- (v0.3 task H7).
PRAGMA user_version = 9;

-- Schema v10: resumable importer state and exact source-bundle manifests
-- (v0.3 task H8). Source bytes remain in the asset store; only their
-- content-addressed paths and hashes live in SQLite.
CREATE TABLE IF NOT EXISTS import_checkpoints (
    source_system TEXT NOT NULL,
    source_key TEXT NOT NULL,
    collection_id TEXT NOT NULL,
    inventory_fingerprint TEXT NOT NULL,
    phase TEXT NOT NULL,
    next_index INTEGER NOT NULL DEFAULT 0,
    total_items INTEGER NOT NULL DEFAULT 0,
    processed_items INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    report_json TEXT NOT NULL DEFAULT '{}',
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TEXT,
    PRIMARY KEY(source_system, source_key, collection_id)
);

CREATE TABLE IF NOT EXISTS import_item_states (
    source_system TEXT NOT NULL,
    source_key TEXT NOT NULL,
    collection_id TEXT NOT NULL,
    item_key TEXT NOT NULL,
    item_type TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    target_id TEXT,
    action TEXT NOT NULL,
    processed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(source_system, source_key, collection_id, item_key)
);

CREATE INDEX IF NOT EXISTS import_item_states_type_idx
    ON import_item_states(source_system, source_key, collection_id, item_type, item_key);

CREATE TABLE IF NOT EXISTS source_bundle_items (
    source_system TEXT NOT NULL,
    source_key TEXT NOT NULL,
    collection_id TEXT NOT NULL,
    item_key TEXT NOT NULL,
    item_type TEXT NOT NULL,
    external_id TEXT NOT NULL,
    relative_path TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    storage_path TEXT NOT NULL,
    property_order_json TEXT NOT NULL DEFAULT '[]',
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(source_system, source_key, collection_id, item_key)
);

CREATE INDEX IF NOT EXISTS source_bundle_items_hash_idx
    ON source_bundle_items(sha256);

PRAGMA user_version = 10;

-- Schema v12: stable logical database identity and per-writable-copy replica
-- identity. Archive v2 preserves database_id for in-universe restores while
-- restore/clone workflows always mint a new replica_id.
CREATE TABLE IF NOT EXISTS database_identity (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    database_id TEXT NOT NULL UNIQUE,
    replica_id TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    replica_created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

PRAGMA user_version = 12;

-- Schema v13: the restore-in-progress marker. An archive-v2 restore commits
-- many bounded transactions, so an interruption leaves committed rows behind;
-- this row is what stops that partial library passing as a complete one.
CREATE TABLE IF NOT EXISTS restore_state (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    snapshot_id TEXT NOT NULL,
    commit_sha256 TEXT NOT NULL,
    intent TEXT NOT NULL,
    started_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

PRAGMA user_version = 13;

-- Schema v14: content-addressed note blocks. A block's ID derives from its
-- text (PROJECT_DECISIONS.md 17), so moving a block keeps its identity and
-- editing its text mints a new one. Rows are derived state, rebuilt in the same
-- transaction as the note save that produced them.
CREATE TABLE IF NOT EXISTS document_blocks (
    id TEXT NOT NULL,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL,
    kind TEXT NOT NULL,
    heading_level INTEGER NOT NULL DEFAULT 0,
    marker TEXT,
    content_sha256 TEXT NOT NULL,
    start_byte INTEGER NOT NULL,
    end_byte INTEGER NOT NULL,
    heading_slug TEXT,
    PRIMARY KEY (document_id, id)
);
CREATE INDEX IF NOT EXISTS document_blocks_document_idx ON document_blocks(document_id, ordinal);
CREATE INDEX IF NOT EXISTS document_blocks_marker_idx ON document_blocks(document_id, marker) WHERE marker IS NOT NULL;
CREATE INDEX IF NOT EXISTS document_blocks_slug_idx ON document_blocks(document_id, heading_slug) WHERE heading_slug IS NOT NULL;

-- Schema v15: heading slugs. A `#section-title` anchor resolves against this
-- column; block rows deliberately store no heading text, so without it a
-- heading anchor has nothing to compare against (PROJECT_DECISIONS.md 19).
PRAGMA user_version = 15;
