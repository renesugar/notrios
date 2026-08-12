-- Schema v19: explicit sync enrollment and the transactionally complete local
-- operation journal. These tables do not enable sync by themselves. Until the
-- sync_local_journal singleton exists, every capture trigger is inert.

CREATE TABLE IF NOT EXISTS sync_replicas (
    replica_id TEXT PRIMARY KEY,
    database_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('local', 'peer')),
    status TEXT NOT NULL CHECK (status IN ('active', 'retired', 'revoked')),
    enrolled_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    retired_at TEXT,
    capabilities_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(capabilities_json))
);

CREATE INDEX IF NOT EXISTS sync_replicas_database_status_idx
    ON sync_replicas(database_id, status, replica_id);
CREATE UNIQUE INDEX IF NOT EXISTS sync_replicas_one_active_local_idx
    ON sync_replicas(database_id)
    WHERE role = 'local' AND status = 'active';

CREATE TABLE IF NOT EXISTS sync_snapshot_boundaries (
    id TEXT PRIMARY KEY,
    replica_id TEXT NOT NULL REFERENCES sync_replicas(replica_id),
    sequence INTEGER NOT NULL CHECK (sequence >= 0),
    state_vector_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(state_vector_json)),
    reason TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sync_local_journal (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    replica_id TEXT NOT NULL UNIQUE REFERENCES sync_replicas(replica_id),
    last_sequence INTEGER NOT NULL DEFAULT 0 CHECK (last_sequence >= 0),
    snapshot_boundary_id TEXT NOT NULL REFERENCES sync_snapshot_boundaries(id),
    enabled_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sync_operations (
    replica_id TEXT NOT NULL REFERENCES sync_replicas(replica_id),
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    operation_id TEXT NOT NULL UNIQUE,
    kind TEXT NOT NULL,
    record_type TEXT NOT NULL,
    record_id TEXT NOT NULL,
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (replica_id, sequence)
);

CREATE INDEX IF NOT EXISTS sync_operations_record_idx
    ON sync_operations(record_type, record_id, replica_id, sequence);

CREATE TABLE IF NOT EXISTS sync_operation_dependencies (
    operation_replica_id TEXT NOT NULL,
    operation_sequence INTEGER NOT NULL,
    dependency_replica_id TEXT NOT NULL,
    dependency_sequence INTEGER NOT NULL CHECK (dependency_sequence > 0),
    PRIMARY KEY (
        operation_replica_id,
        operation_sequence,
        dependency_replica_id,
        dependency_sequence
    ),
    FOREIGN KEY (operation_replica_id, operation_sequence)
        REFERENCES sync_operations(replica_id, sequence) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS sync_operation_dependencies_missing_idx
    ON sync_operation_dependencies(dependency_replica_id, dependency_sequence);

CREATE TABLE IF NOT EXISTS sync_state_vectors (
    observer_replica_id TEXT NOT NULL,
    subject_replica_id TEXT NOT NULL,
    contiguous_sequence INTEGER NOT NULL DEFAULT 0 CHECK (contiguous_sequence >= 0),
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (observer_replica_id, subject_replica_id)
);

CREATE TABLE IF NOT EXISTS sync_state_gaps (
    observer_replica_id TEXT NOT NULL,
    subject_replica_id TEXT NOT NULL,
    start_sequence INTEGER NOT NULL CHECK (start_sequence > 0),
    end_sequence INTEGER NOT NULL CHECK (end_sequence >= start_sequence),
    recorded_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (observer_replica_id, subject_replica_id, start_sequence)
);

CREATE TABLE IF NOT EXISTS sync_peer_acknowledgements (
    peer_replica_id TEXT NOT NULL,
    subject_replica_id TEXT NOT NULL,
    contiguous_sequence INTEGER NOT NULL DEFAULT 0 CHECK (contiguous_sequence >= 0),
    acknowledged_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (peer_replica_id, subject_replica_id)
);

CREATE TABLE IF NOT EXISTS sync_pending_admissions (
    replica_id TEXT NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    operation_id TEXT NOT NULL,
    encoded_operation BLOB NOT NULL,
    encoded_size INTEGER NOT NULL CHECK (
        encoded_size = length(encoded_operation) AND encoded_size <= 1048576
    ),
    reason TEXT NOT NULL,
    received_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (replica_id, sequence),
    UNIQUE (operation_id)
);

CREATE INDEX IF NOT EXISTS sync_pending_admissions_received_idx
    ON sync_pending_admissions(received_at, replica_id, sequence);

CREATE TABLE IF NOT EXISTS sync_audit_events (
    id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    replica_id TEXT,
    subject_id TEXT NOT NULL DEFAULT '',
    details_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(details_json)),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS sync_audit_events_created_idx
    ON sync_audit_events(created_at, id);

-- Canonical-table triggers write one row here. The after-insert trigger below
-- is the single journal seam: it allocates the sequence, inserts the immutable
-- operation, advances the local state vector, and removes the transient row.
-- All four writes are part of the canonical caller's transaction.
CREATE TABLE IF NOT EXISTS sync_journal_capture (
    kind TEXT NOT NULL,
    record_type TEXT NOT NULL,
    record_id TEXT NOT NULL,
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json))
);

CREATE TRIGGER IF NOT EXISTS sync_journal_capture_append
AFTER INSERT ON sync_journal_capture
BEGIN
    UPDATE sync_local_journal
       SET last_sequence = last_sequence + 1
     WHERE singleton = 1;

    INSERT INTO sync_operations(
        replica_id, sequence, operation_id, kind, record_type, record_id, payload_json
    )
    SELECT replica_id,
           last_sequence,
           replica_id || ':' || printf('%020d', last_sequence),
           NEW.kind,
           NEW.record_type,
           NEW.record_id,
           NEW.payload_json
      FROM sync_local_journal
     WHERE singleton = 1;

    INSERT INTO sync_state_vectors(
        observer_replica_id, subject_replica_id, contiguous_sequence
    )
    SELECT replica_id, replica_id, last_sequence
      FROM sync_local_journal
     WHERE singleton = 1
    ON CONFLICT(observer_replica_id, subject_replica_id) DO UPDATE SET
        contiguous_sequence = excluded.contiguous_sequence,
        updated_at = CURRENT_TIMESTAMP;

    DELETE FROM sync_journal_capture WHERE rowid = NEW.rowid;
END;

-- Record/field vocabulary. User-editable scalar records carry their complete
-- new register state. Immutable revisions and resources carry stable IDs and
-- metadata references. Memberships are independent elements. Derived FTS,
-- links/blocks, projections, reports, jobs, and importer checkpoints have no
-- triggers and therefore never become sync operations.

CREATE TRIGGER IF NOT EXISTS sync_capture_collections_insert
AFTER INSERT ON collections
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.create', 'collection', NEW.id,
        json_object('name', NEW.name, 'description', COALESCE(NEW.description, ''))
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_collections_update
AFTER UPDATE OF name, description ON collections
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND (OLD.name IS NOT NEW.name OR OLD.description IS NOT NEW.description)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.update', 'collection', NEW.id,
        json_object('name', NEW.name, 'description', COALESCE(NEW.description, ''))
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_collections_delete
AFTER DELETE ON collections
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES('record.delete', 'collection', OLD.id, '{}');
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_documents_insert
AFTER INSERT ON documents
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.create', 'document', NEW.id,
        json_object(
            'collection_id', NEW.collection_id,
            'notebook_id', COALESCE(NEW.notebook_id, ''),
            'title', NEW.title,
            'body_mime_type', NEW.body_mime_type,
            'current_revision_id', COALESCE(NEW.current_revision_id, ''),
            'deleted_at', COALESCE(NEW.deleted_at, '')
        )
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_documents_update
AFTER UPDATE OF collection_id, notebook_id, title, body_mime_type, current_revision_id, deleted_at ON documents
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND (
    OLD.collection_id IS NOT NEW.collection_id OR
    OLD.notebook_id IS NOT NEW.notebook_id OR
    OLD.title IS NOT NEW.title OR
    OLD.body_mime_type IS NOT NEW.body_mime_type OR
    OLD.current_revision_id IS NOT NEW.current_revision_id OR
    OLD.deleted_at IS NOT NEW.deleted_at
 )
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        CASE
            WHEN OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN 'document.trash'
            WHEN OLD.deleted_at IS NOT NULL AND NEW.deleted_at IS NULL THEN 'document.restore'
            ELSE 'record.update'
        END,
        'document', NEW.id,
        json_object(
            'collection_id', NEW.collection_id,
            'notebook_id', COALESCE(NEW.notebook_id, ''),
            'title', NEW.title,
            'body_mime_type', NEW.body_mime_type,
            'current_revision_id', COALESCE(NEW.current_revision_id, ''),
            'deleted_at', COALESCE(NEW.deleted_at, '')
        )
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_documents_delete
AFTER DELETE ON documents
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'document.purge', 'document', OLD.id,
        json_object('current_revision_id', COALESCE(OLD.current_revision_id, ''))
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_revisions_insert
AFTER INSERT ON document_revisions
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.create', 'revision', NEW.id,
        json_object(
            'document_id', NEW.document_id,
            'title', NEW.title,
            'body_mime_type', NEW.body_mime_type,
            'message', COALESCE(NEW.message, '')
        )
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_notebooks_insert
AFTER INSERT ON notebooks
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.create', 'notebook', NEW.id,
        json_object(
            'parent_id', COALESCE(NEW.parent_id, ''), 'name', NEW.name,
            'icon_emoji', NEW.icon_emoji, 'position', NEW.position
        )
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_notebooks_update
AFTER UPDATE OF parent_id, name, icon_emoji, position ON notebooks
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND (
    OLD.parent_id IS NOT NEW.parent_id OR OLD.name IS NOT NEW.name OR
    OLD.icon_emoji IS NOT NEW.icon_emoji OR OLD.position IS NOT NEW.position
 )
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.update', 'notebook', NEW.id,
        json_object(
            'parent_id', COALESCE(NEW.parent_id, ''), 'name', NEW.name,
            'icon_emoji', NEW.icon_emoji, 'position', NEW.position
        )
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_notebooks_delete
AFTER DELETE ON notebooks
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES('record.delete', 'notebook', OLD.id, '{}');
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_tags_insert
AFTER INSERT ON tags
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.create', 'tag', NEW.id, json_object('name', NEW.name)
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_tags_update
AFTER UPDATE OF name ON tags
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND OLD.name IS NOT NEW.name
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.update', 'tag', NEW.id, json_object('name', NEW.name)
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_tags_delete
AFTER DELETE ON tags
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES('record.delete', 'tag', OLD.id, '{}');
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_note_tags_insert
AFTER INSERT ON note_tags
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'membership.add', 'document_tag', NEW.document_id || ':' || NEW.tag_id,
        json_object('document_id', NEW.document_id, 'tag_id', NEW.tag_id)
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_note_tags_delete
AFTER DELETE ON note_tags
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'membership.remove', 'document_tag', OLD.document_id || ':' || OLD.tag_id,
        json_object('document_id', OLD.document_id, 'tag_id', OLD.tag_id)
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_search_notebooks_insert
AFTER INSERT ON search_notebooks
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.create', 'search_notebook', NEW.id,
        json_object('name', NEW.name, 'icon_emoji', NEW.icon_emoji, 'query', NEW.query)
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_search_notebooks_update
AFTER UPDATE OF name, icon_emoji, query ON search_notebooks
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND (OLD.name IS NOT NEW.name OR OLD.icon_emoji IS NOT NEW.icon_emoji OR OLD.query IS NOT NEW.query)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.update', 'search_notebook', NEW.id,
        json_object('name', NEW.name, 'icon_emoji', NEW.icon_emoji, 'query', NEW.query)
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_search_notebooks_delete
AFTER DELETE ON search_notebooks
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES('record.delete', 'search_notebook', OLD.id, '{}');
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_resources_insert
AFTER INSERT ON resources
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.create', 'resource', NEW.id,
        json_object(
            'collection_id', NEW.collection_id, 'blob_sha256', NEW.blob_sha256,
            'filename', COALESCE(NEW.filename, ''), 'mime_type', NEW.mime_type,
            'metadata_json', NEW.metadata_json
        )
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_resources_update
AFTER UPDATE OF collection_id, blob_sha256, filename, mime_type, metadata_json ON resources
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND (
    OLD.collection_id IS NOT NEW.collection_id OR OLD.blob_sha256 IS NOT NEW.blob_sha256 OR
    OLD.filename IS NOT NEW.filename OR OLD.mime_type IS NOT NEW.mime_type OR
    OLD.metadata_json IS NOT NEW.metadata_json
 )
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.update', 'resource', NEW.id,
        json_object(
            'collection_id', NEW.collection_id, 'blob_sha256', NEW.blob_sha256,
            'filename', COALESCE(NEW.filename, ''), 'mime_type', NEW.mime_type,
            'metadata_json', NEW.metadata_json
        )
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_resources_delete
AFTER DELETE ON resources
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.delete', 'resource', OLD.id,
        json_object('blob_sha256', OLD.blob_sha256)
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_resource_refs_insert
AFTER INSERT ON document_resource_refs
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'membership.add', 'document_resource',
        NEW.document_id || ':' || NEW.resource_id || ':' || NEW.relation_type || ':' || NEW.ordinal,
        json_object(
            'document_id', NEW.document_id, 'resource_id', NEW.resource_id,
            'relation_type', NEW.relation_type, 'ordinal', NEW.ordinal,
            'anchor_json', NEW.anchor_json
        )
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_resource_refs_update
AFTER UPDATE OF anchor_json ON document_resource_refs
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND OLD.anchor_json IS NOT NEW.anchor_json
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'membership.update', 'document_resource',
        NEW.document_id || ':' || NEW.resource_id || ':' || NEW.relation_type || ':' || NEW.ordinal,
        json_object(
            'document_id', NEW.document_id, 'resource_id', NEW.resource_id,
            'relation_type', NEW.relation_type, 'ordinal', NEW.ordinal,
            'anchor_json', NEW.anchor_json
        )
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_resource_refs_delete
AFTER DELETE ON document_resource_refs
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'membership.remove', 'document_resource',
        OLD.document_id || ':' || OLD.resource_id || ':' || OLD.relation_type || ':' || OLD.ordinal,
        json_object(
            'document_id', OLD.document_id, 'resource_id', OLD.resource_id,
            'relation_type', OLD.relation_type, 'ordinal', OLD.ordinal
        )
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_sources_insert
AFTER INSERT ON document_sources
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.create', 'document_source', NEW.document_id,
        json_object(
            'source_system', NEW.source_system, 'external_id', NEW.external_id,
            'author', NEW.author, 'author_id', NEW.author_id, 'thread_id', NEW.thread_id,
            'reply_to', NEW.reply_to, 'source_url', NEW.source_url,
            'published_at', NEW.published_at, 'metadata_json', NEW.metadata_json
        )
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_sources_update
AFTER UPDATE OF source_system, external_id, author, author_id, thread_id, reply_to, source_url, published_at, metadata_json ON document_sources
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND (
    OLD.source_system IS NOT NEW.source_system OR OLD.external_id IS NOT NEW.external_id OR
    OLD.author IS NOT NEW.author OR OLD.author_id IS NOT NEW.author_id OR
    OLD.thread_id IS NOT NEW.thread_id OR OLD.reply_to IS NOT NEW.reply_to OR
    OLD.source_url IS NOT NEW.source_url OR OLD.published_at IS NOT NEW.published_at OR
    OLD.metadata_json IS NOT NEW.metadata_json
 )
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.update', 'document_source', NEW.document_id,
        json_object(
            'source_system', NEW.source_system, 'external_id', NEW.external_id,
            'author', NEW.author, 'author_id', NEW.author_id, 'thread_id', NEW.thread_id,
            'reply_to', NEW.reply_to, 'source_url', NEW.source_url,
            'published_at', NEW.published_at, 'metadata_json', NEW.metadata_json
        )
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_sources_delete
AFTER DELETE ON document_sources
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES('record.delete', 'document_source', OLD.document_id, '{}');
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_source_bundles_insert
AFTER INSERT ON source_bundle_items
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.create', 'source_bundle_item',
        json_array(NEW.source_system, NEW.source_key, NEW.collection_id, NEW.item_key),
        json_object(
            'source_system', NEW.source_system, 'source_key', NEW.source_key,
            'collection_id', NEW.collection_id, 'item_key', NEW.item_key,
            'item_type', NEW.item_type, 'external_id', NEW.external_id,
            'relative_path', NEW.relative_path, 'sha256', NEW.sha256,
            'size_bytes', NEW.size_bytes,
            'property_order_json', NEW.property_order_json
        )
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_source_bundles_update
AFTER UPDATE OF item_type, external_id, relative_path, sha256, size_bytes, property_order_json ON source_bundle_items
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND (
    OLD.item_type IS NOT NEW.item_type OR OLD.external_id IS NOT NEW.external_id OR
    OLD.relative_path IS NOT NEW.relative_path OR OLD.sha256 IS NOT NEW.sha256 OR
    OLD.size_bytes IS NOT NEW.size_bytes OR OLD.property_order_json IS NOT NEW.property_order_json
 )
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.update', 'source_bundle_item',
        json_array(NEW.source_system, NEW.source_key, NEW.collection_id, NEW.item_key),
        json_object(
            'source_system', NEW.source_system, 'source_key', NEW.source_key,
            'collection_id', NEW.collection_id, 'item_key', NEW.item_key,
            'item_type', NEW.item_type, 'external_id', NEW.external_id,
            'relative_path', NEW.relative_path, 'sha256', NEW.sha256,
            'size_bytes', NEW.size_bytes,
            'property_order_json', NEW.property_order_json
        )
    );
END;

CREATE TRIGGER IF NOT EXISTS sync_capture_source_bundles_delete
AFTER DELETE ON source_bundle_items
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.delete', 'source_bundle_item',
        json_array(OLD.source_system, OLD.source_key, OLD.collection_id, OLD.item_key),
        json_object('sha256', OLD.sha256)
    );
END;

PRAGMA user_version = 19;
