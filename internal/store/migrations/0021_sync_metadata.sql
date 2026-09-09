-- Schema v21: durable hybrid logical timestamps and the deterministic G6
-- metadata convergence projection. G9 will authenticate operation/certificate
-- bytes; these tables do not create a transport or a public sync surface.

UPDATE sync_operations
   SET hlc_wall_ms = MAX(1, CAST(strftime('%s', created_at) AS INTEGER) * 1000)
 WHERE hlc_wall_ms = 0;

CREATE TABLE IF NOT EXISTS sync_hlc_clock (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    wall_ms INTEGER NOT NULL CHECK (wall_ms > 0 AND wall_ms <= 253402300799999),
    logical INTEGER NOT NULL CHECK (logical >= 0 AND logical <= 2147483647)
);

INSERT INTO sync_hlc_clock(singleton, wall_ms, logical)
SELECT 1, MAX(1, COALESCE(MAX(hlc_wall_ms), 0)), 0 FROM sync_operations
WHERE 1
ON CONFLICT(singleton) DO UPDATE SET wall_ms=MAX(sync_hlc_clock.wall_ms, excluded.wall_ms);

CREATE TABLE IF NOT EXISTS sync_metadata_baseline_records (
    record_type TEXT NOT NULL,
    record_id TEXT NOT NULL,
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
    active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    PRIMARY KEY(record_type, record_id)
);

CREATE TABLE IF NOT EXISTS sync_metadata_baseline_memberships (
    element_id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL,
    tag_id TEXT NOT NULL,
    present INTEGER NOT NULL DEFAULT 1 CHECK (present IN (0, 1))
);

CREATE TABLE IF NOT EXISTS sync_metadata_baseline_floors (
    replica_id TEXT PRIMARY KEY,
    sequence INTEGER NOT NULL CHECK (sequence >= 0)
);

CREATE TABLE IF NOT EXISTS sync_field_registers (
    record_type TEXT NOT NULL,
    record_id TEXT NOT NULL,
    field_name TEXT NOT NULL,
    value_json TEXT NOT NULL CHECK (json_valid(value_json)),
    hlc_wall_ms INTEGER NOT NULL,
    hlc_logical INTEGER NOT NULL,
    replica_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    PRIMARY KEY(record_type, record_id, field_name)
);

CREATE TABLE IF NOT EXISTS sync_lifecycle_registers (
    record_type TEXT NOT NULL,
    record_id TEXT NOT NULL,
    active INTEGER NOT NULL CHECK (active IN (0, 1)),
    hlc_wall_ms INTEGER NOT NULL,
    hlc_logical INTEGER NOT NULL,
    replica_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    PRIMARY KEY(record_type, record_id)
);

CREATE TABLE IF NOT EXISTS sync_membership_registers (
    element_id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL,
    tag_id TEXT NOT NULL,
    present INTEGER NOT NULL CHECK (present IN (0, 1)),
    hlc_wall_ms INTEGER NOT NULL,
    hlc_logical INTEGER NOT NULL,
    replica_id TEXT NOT NULL,
    sequence INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sync_death_certificates (
    document_id TEXT PRIMARY KEY,
    signer_replica_id TEXT NOT NULL,
    signer_sequence INTEGER NOT NULL,
    signature TEXT NOT NULL CHECK (length(signature) > 0 AND length(signature) <= 2048),
    hlc_wall_ms INTEGER NOT NULL,
    hlc_logical INTEGER NOT NULL,
    acknowledged_by_all INTEGER NOT NULL DEFAULT 0 CHECK (acknowledged_by_all IN (0, 1))
);

CREATE TABLE IF NOT EXISTS sync_repair_events (
    id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    details_json TEXT NOT NULL CHECK (json_valid(details_json)),
    hlc_wall_ms INTEGER NOT NULL,
    hlc_logical INTEGER NOT NULL,
    replica_id TEXT NOT NULL,
    sequence INTEGER NOT NULL
);

-- Canonical writes performed by the deterministic applier must not create a
-- second local operation. The guard lives in the same transaction and is
-- always empty outside reconciliation.
CREATE TABLE IF NOT EXISTS sync_apply_guard (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1)
);

DROP TRIGGER IF EXISTS sync_journal_capture_suppress;
CREATE TRIGGER sync_journal_capture_suppress
BEFORE INSERT ON sync_journal_capture
WHEN EXISTS (SELECT 1 FROM sync_apply_guard WHERE singleton = 1)
BEGIN
    SELECT RAISE(IGNORE);
END;

-- A synchronized document may only be permanently deleted through a signed
-- death-certificate operation. G9 supplies signing/verification; G6 therefore
-- refuses the ordinary local purge path while enrollment is active.
DROP TRIGGER IF EXISTS sync_documents_purge_requires_certificate;
CREATE TRIGGER sync_documents_purge_requires_certificate
BEFORE DELETE ON documents
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND NOT EXISTS (SELECT 1 FROM sync_apply_guard WHERE singleton = 1)
BEGIN
    SELECT RAISE(ABORT, 'sync purge requires signed death certificate');
END;

DROP TRIGGER sync_journal_capture_append;
CREATE TRIGGER sync_journal_capture_append
AFTER INSERT ON sync_journal_capture
BEGIN
    UPDATE sync_hlc_clock
       SET logical = CASE
               WHEN wall_ms >= CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER)
                    AND logical < 2147483647 THEN logical + 1
               ELSE 0
           END,
           wall_ms = CASE
               WHEN wall_ms >= CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER)
                    AND logical >= 2147483647 THEN wall_ms + 1
               ELSE MAX(wall_ms, CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER))
           END
     WHERE singleton = 1;

    UPDATE sync_local_journal
       SET last_sequence = last_sequence + 1
     WHERE singleton = 1;

    INSERT INTO sync_operations(
        replica_id, sequence, operation_id, kind, record_type, record_id,
        payload_json, hlc_wall_ms, hlc_logical
    )
    SELECT journal.replica_id,
           journal.last_sequence,
           journal.replica_id || ':' || printf('%020d', journal.last_sequence),
           NEW.kind,
           NEW.record_type,
           NEW.record_id,
           NEW.payload_json,
           clock.wall_ms,
           clock.logical
      FROM sync_local_journal AS journal
      JOIN sync_hlc_clock AS clock ON clock.singleton = 1
     WHERE journal.singleton = 1;

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

-- G4 used complete-record update payloads because it had no merge semantics.
-- G6 replaces the mutable scalar update triggers with sparse field payloads so
-- a transaction writes only registers it actually changed.
DROP TRIGGER sync_capture_collections_update;
CREATE TRIGGER sync_capture_collections_update
AFTER UPDATE OF name, description ON collections
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND (OLD.name IS NOT NEW.name OR OLD.description IS NOT NEW.description)
BEGIN
    INSERT INTO sync_journal_capture VALUES('record.update', 'collection', NEW.id,
        json_patch(
            CASE WHEN OLD.name IS NOT NEW.name THEN json_object('name', NEW.name) ELSE '{}' END,
            CASE WHEN OLD.description IS NOT NEW.description THEN json_object('description', COALESCE(NEW.description, '')) ELSE '{}' END
        ));
END;

DROP TRIGGER sync_capture_documents_update;
CREATE TRIGGER sync_capture_documents_update
AFTER UPDATE OF collection_id, notebook_id, title, body_mime_type, current_revision_id, deleted_at ON documents
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND (OLD.collection_id IS NOT NEW.collection_id OR OLD.notebook_id IS NOT NEW.notebook_id OR
      OLD.title IS NOT NEW.title OR OLD.body_mime_type IS NOT NEW.body_mime_type OR
      OLD.current_revision_id IS NOT NEW.current_revision_id OR OLD.deleted_at IS NOT NEW.deleted_at)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        CASE WHEN OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN 'document.trash'
             WHEN OLD.deleted_at IS NOT NULL AND NEW.deleted_at IS NULL THEN 'document.restore'
             ELSE 'record.update' END,
        'document', NEW.id,
        json_patch(json_patch(json_patch(json_patch(json_patch(json_patch('{}',
            CASE WHEN OLD.collection_id IS NOT NEW.collection_id THEN json_object('collection_id', NEW.collection_id) ELSE '{}' END),
            CASE WHEN OLD.notebook_id IS NOT NEW.notebook_id THEN json_object('notebook_id', COALESCE(NEW.notebook_id, '')) ELSE '{}' END),
            CASE WHEN OLD.title IS NOT NEW.title THEN json_object('title', NEW.title) ELSE '{}' END),
            CASE WHEN OLD.body_mime_type IS NOT NEW.body_mime_type THEN json_object('body_mime_type', NEW.body_mime_type) ELSE '{}' END),
            CASE WHEN OLD.current_revision_id IS NOT NEW.current_revision_id THEN json_object('current_revision_id', COALESCE(NEW.current_revision_id, '')) ELSE '{}' END),
            CASE WHEN OLD.deleted_at IS NOT NEW.deleted_at THEN json_object('deleted_at', COALESCE(NEW.deleted_at, '')) ELSE '{}' END)
    );
END;

DROP TRIGGER sync_capture_notebooks_update;
CREATE TRIGGER sync_capture_notebooks_update
AFTER UPDATE OF parent_id, name, icon_emoji, position ON notebooks
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND (OLD.parent_id IS NOT NEW.parent_id OR OLD.name IS NOT NEW.name OR OLD.icon_emoji IS NOT NEW.icon_emoji OR OLD.position IS NOT NEW.position)
BEGIN
    INSERT INTO sync_journal_capture VALUES('record.update', 'notebook', NEW.id,
        json_patch(json_patch(json_patch(json_patch('{}',
            CASE WHEN OLD.parent_id IS NOT NEW.parent_id THEN json_object('parent_id', COALESCE(NEW.parent_id, '')) ELSE '{}' END),
            CASE WHEN OLD.name IS NOT NEW.name THEN json_object('name', NEW.name) ELSE '{}' END),
            CASE WHEN OLD.icon_emoji IS NOT NEW.icon_emoji THEN json_object('icon_emoji', NEW.icon_emoji) ELSE '{}' END),
            CASE WHEN OLD.position IS NOT NEW.position THEN json_object('position', NEW.position) ELSE '{}' END)
    );
END;

DROP TRIGGER sync_capture_search_notebooks_update;
CREATE TRIGGER sync_capture_search_notebooks_update
AFTER UPDATE OF name, icon_emoji, query ON search_notebooks
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND (OLD.name IS NOT NEW.name OR OLD.icon_emoji IS NOT NEW.icon_emoji OR OLD.query IS NOT NEW.query)
BEGIN
    INSERT INTO sync_journal_capture VALUES('record.update', 'search_notebook', NEW.id,
        json_patch(json_patch(json_patch('{}',
            CASE WHEN OLD.name IS NOT NEW.name THEN json_object('name', NEW.name) ELSE '{}' END),
            CASE WHEN OLD.icon_emoji IS NOT NEW.icon_emoji THEN json_object('icon_emoji', NEW.icon_emoji) ELSE '{}' END),
            CASE WHEN OLD.query IS NOT NEW.query THEN json_object('query', NEW.query) ELSE '{}' END)
    );
END;

DROP TRIGGER sync_capture_sources_update;
CREATE TRIGGER sync_capture_sources_update
AFTER UPDATE OF source_system, external_id, author, author_id, thread_id, reply_to, source_url, published_at, metadata_json ON document_sources
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
 AND (OLD.source_system IS NOT NEW.source_system OR OLD.external_id IS NOT NEW.external_id OR
      OLD.author IS NOT NEW.author OR OLD.author_id IS NOT NEW.author_id OR OLD.thread_id IS NOT NEW.thread_id OR
      OLD.reply_to IS NOT NEW.reply_to OR OLD.source_url IS NOT NEW.source_url OR
      OLD.published_at IS NOT NEW.published_at OR OLD.metadata_json IS NOT NEW.metadata_json)
BEGIN
    INSERT INTO sync_journal_capture VALUES('record.update', 'document_source', NEW.document_id,
        json_patch(json_patch(json_patch(json_patch(json_patch(json_patch(json_patch(json_patch(json_patch('{}',
            CASE WHEN OLD.source_system IS NOT NEW.source_system THEN json_object('source_system', NEW.source_system) ELSE '{}' END),
            CASE WHEN OLD.external_id IS NOT NEW.external_id THEN json_object('external_id', NEW.external_id) ELSE '{}' END),
            CASE WHEN OLD.author IS NOT NEW.author THEN json_object('author', NEW.author) ELSE '{}' END),
            CASE WHEN OLD.author_id IS NOT NEW.author_id THEN json_object('author_id', NEW.author_id) ELSE '{}' END),
            CASE WHEN OLD.thread_id IS NOT NEW.thread_id THEN json_object('thread_id', NEW.thread_id) ELSE '{}' END),
            CASE WHEN OLD.reply_to IS NOT NEW.reply_to THEN json_object('reply_to', NEW.reply_to) ELSE '{}' END),
            CASE WHEN OLD.source_url IS NOT NEW.source_url THEN json_object('source_url', NEW.source_url) ELSE '{}' END),
            CASE WHEN OLD.published_at IS NOT NEW.published_at THEN json_object('published_at', NEW.published_at) ELSE '{}' END),
            CASE WHEN OLD.metadata_json IS NOT NEW.metadata_json THEN json_object('metadata_json', NEW.metadata_json) ELSE '{}' END)
    );
END;

PRAGMA user_version = 21;
