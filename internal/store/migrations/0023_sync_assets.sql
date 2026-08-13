-- Schema v23: resource metadata that converges before its bytes do.
--
-- The `availability` column on blobs is added by Go before this file runs,
-- because ALTER TABLE has no IF NOT EXISTS and a partially upgraded database
-- must be able to resume.

-- A blob row may now exist without a file. That is the whole point of lazy
-- materialization: a note can reference an attachment this replica knows the
-- identity, size, and type of but has not downloaded. What must never happen
-- is a row claiming bytes that are not there, so the two facts are tied
-- together and enforced rather than described.
--
-- The enforcement is a trigger rather than a CHECK constraint because SQLite
-- cannot add a CHECK to an existing table, and a rule that only applied to
-- databases created after v23 would be the rule least likely to hold.
DROP TRIGGER IF EXISTS blobs_availability_matches_storage_insert;
CREATE TRIGGER blobs_availability_matches_storage_insert
BEFORE INSERT ON blobs
WHEN (NEW.availability = 'local') <> (TRIM(NEW.storage_path) <> '')
BEGIN
    SELECT RAISE(ABORT, 'blob availability must match whether it has a storage path');
END;

DROP TRIGGER IF EXISTS blobs_availability_matches_storage_update;
CREATE TRIGGER blobs_availability_matches_storage_update
BEFORE UPDATE OF availability, storage_path ON blobs
WHEN (NEW.availability = 'local') <> (TRIM(NEW.storage_path) <> '')
BEGIN
    SELECT RAISE(ABORT, 'blob availability must match whether it has a storage path');
END;

CREATE INDEX IF NOT EXISTS blobs_availability_idx ON blobs(availability);

-- How an object is divided for transfer, and what the whole thing must hash to.
-- `manifest_sha256` is what travels inside a bounded operation payload: 16,384
-- chunk hashes would not fit in one, and a digest lets the manifest be fetched
-- by another route and still be verified on arrival.
CREATE TABLE IF NOT EXISTS sync_blob_manifests (
    blob_sha256 TEXT PRIMARY KEY,
    byte_length INTEGER NOT NULL CHECK (byte_length >= 0),
    mime_type TEXT NOT NULL DEFAULT '',
    chunk_count INTEGER NOT NULL CHECK (chunk_count >= 1 AND chunk_count <= 16384),
    chunk_bytes INTEGER NOT NULL CHECK (chunk_bytes >= 0),
    manifest_sha256 TEXT NOT NULL,
    complete INTEGER NOT NULL DEFAULT 0 CHECK (complete IN (0, 1)),
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- One row per transfer segment. `fetched` is what makes a resumed download
-- resume: an interrupted transfer keeps every chunk it verified and asks only
-- for the rest.
CREATE TABLE IF NOT EXISTS sync_blob_chunks (
    blob_sha256 TEXT NOT NULL,
    ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
    chunk_sha256 TEXT NOT NULL,
    byte_length INTEGER NOT NULL CHECK (byte_length >= 0),
    fetched INTEGER NOT NULL DEFAULT 0 CHECK (fetched IN (0, 1)),
    PRIMARY KEY (blob_sha256, ordinal)
);

CREATE INDEX IF NOT EXISTS sync_blob_chunks_pending_idx
    ON sync_blob_chunks(blob_sha256, fetched, ordinal);

-- Which peers said they hold an object. A replica keeps every advertisement it
-- has heard, because the answer to "nobody has it right now" is to wait and ask
-- again, not to forget who used to.
CREATE TABLE IF NOT EXISTS sync_blob_sources (
    blob_sha256 TEXT NOT NULL,
    replica_id TEXT NOT NULL,
    byte_length INTEGER NOT NULL DEFAULT 0 CHECK (byte_length >= 0),
    advertised_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (blob_sha256, replica_id)
);

-- This replica's own intent about an object, kept separate from what the
-- protocol says exists. `pinned` is the user asking for it regardless of size;
-- `requested` is a one-off. `last_reason` records why the most recent pass did
-- not produce bytes, so "not downloaded" and "no peer has it" and "the source
-- sent something else" are three distinguishable states rather than one.
CREATE TABLE IF NOT EXISTS sync_blob_materialization (
    blob_sha256 TEXT PRIMARY KEY,
    pinned INTEGER NOT NULL DEFAULT 0 CHECK (pinned IN (0, 1)),
    requested INTEGER NOT NULL DEFAULT 0 CHECK (requested IN (0, 1)),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    staged_bytes INTEGER NOT NULL DEFAULT 0 CHECK (staged_bytes >= 0),
    last_reason TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS sync_blob_materialization_wanted_idx
    ON sync_blob_materialization(pinned, requested, blob_sha256);

-- The resource operation now names everything a receiver needs to decide
-- whether to fetch the bytes and how to verify them when it does: the exact
-- object hash it already carried, plus the byte length, the content type, the
-- chunk count, and the manifest digest. Go maintains sync_blob_manifests for
-- every local blob, so the trigger reads rather than computes.
DROP TRIGGER IF EXISTS sync_capture_resources_insert;
CREATE TRIGGER sync_capture_resources_insert
AFTER INSERT ON resources
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    INSERT INTO sync_journal_capture VALUES(
        'record.create', 'resource', NEW.id,
        json_object(
            'collection_id', NEW.collection_id, 'blob_sha256', NEW.blob_sha256,
            'filename', COALESCE(NEW.filename, ''), 'mime_type', NEW.mime_type,
            'metadata_json', NEW.metadata_json,
            'blob_length', COALESCE((SELECT b.size_bytes FROM blobs b WHERE b.sha256 = NEW.blob_sha256), 0),
            'blob_mime_type', COALESCE((SELECT b.mime_type FROM blobs b WHERE b.sha256 = NEW.blob_sha256), ''),
            'chunk_count', COALESCE((SELECT m.chunk_count FROM sync_blob_manifests m WHERE m.blob_sha256 = NEW.blob_sha256), 1),
            'manifest_sha256', COALESCE((SELECT m.manifest_sha256 FROM sync_blob_manifests m WHERE m.blob_sha256 = NEW.blob_sha256), '')
        )
    );
END;

DROP TRIGGER IF EXISTS sync_capture_resources_update;
CREATE TRIGGER sync_capture_resources_update
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
            'metadata_json', NEW.metadata_json,
            'blob_length', COALESCE((SELECT b.size_bytes FROM blobs b WHERE b.sha256 = NEW.blob_sha256), 0),
            'blob_mime_type', COALESCE((SELECT b.mime_type FROM blobs b WHERE b.sha256 = NEW.blob_sha256), ''),
            'chunk_count', COALESCE((SELECT m.chunk_count FROM sync_blob_manifests m WHERE m.blob_sha256 = NEW.blob_sha256), 1),
            'manifest_sha256', COALESCE((SELECT m.manifest_sha256 FROM sync_blob_manifests m WHERE m.blob_sha256 = NEW.blob_sha256), '')
        )
    );
END;

PRAGMA user_version = 23;
