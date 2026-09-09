-- Schema v22: immutable note revision objects, optional named-base transfer
-- deltas, and durable typed body conflicts. G9 still authenticates operation
-- bytes; these tables add no transport and no public sync surface.
--
-- The three columns this migration depends on are added by Go before this file
-- runs, because ALTER TABLE has no IF NOT EXISTS and a partially upgraded
-- development database must be able to resume.

-- `document_revisions` had no index at all before v22: every "the revisions of
-- this note" query was a full table scan, which was survivable only because
-- nothing asked per row. G7 asks per row — the parent chain, the merge base,
-- and a delta's base all look a revision up by document and order — so the
-- index is a correctness-adjacent requirement rather than a tuning choice.
-- `ensureSchemaV22` creates it in Go before the backfill runs, because the
-- backfill is the first thing that needs it.
CREATE INDEX IF NOT EXISTS document_revisions_document_created_idx
    ON document_revisions(document_id, created_at);

-- A transfer delta is an optimization, never canonical state and never the
-- only way to recover a body. The publisher keeps the complete body in
-- document_revisions, so a receiver that lacks the named base can still obtain
-- the whole object. Delta bytes are stored base64-encoded because the journal
-- payload that carries them is JSON, and one representation is easier to
-- verify than two.
CREATE TABLE IF NOT EXISTS sync_revision_deltas (
    revision_id TEXT PRIMARY KEY,
    base_revision_id TEXT NOT NULL,
    format TEXT NOT NULL,
    base_sha256 TEXT NOT NULL,
    base_length INTEGER NOT NULL CHECK (base_length >= 0),
    result_sha256 TEXT NOT NULL,
    result_length INTEGER NOT NULL CHECK (result_length >= 0),
    delta_base64 TEXT NOT NULL,
    delta_length INTEGER NOT NULL CHECK (delta_length >= 0 AND delta_length <= 8388608),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- The transient hand-off from Go to the capture trigger. Go writes one row
-- immediately before inserting a revision when it has measured an exact inline
-- payload or generated a beneficial delta; the trigger consumes and deletes
-- it. When no row exists the trigger falls back to inlining a small body, so a
-- code path that knows nothing about G7 still journals a usable object.
CREATE TABLE IF NOT EXISTS sync_revision_transfer (
    revision_id TEXT PRIMARY KEY,
    inline_body TEXT,
    delta_json TEXT CHECK (delta_json IS NULL OR json_valid(delta_json))
);

-- A revision this replica knows exists but whose bytes it does not hold. The
-- revision is not inserted into document_revisions, because a row there would
-- claim a body. `reason` records why: the object was too large to travel
-- inline, its delta named a base this replica does not have, or its delta
-- failed verification and was refused.
CREATE TABLE IF NOT EXISTS sync_revision_pending_bodies (
    revision_id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    body_mime_type TEXT NOT NULL DEFAULT 'text/markdown',
    message TEXT NOT NULL DEFAULT '',
    parent_revision_ids TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(parent_revision_ids)),
    content_sha256 TEXT NOT NULL,
    content_length INTEGER NOT NULL CHECK (content_length >= 0),
    reason TEXT NOT NULL CHECK (reason IN ('oversize', 'missing_base', 'unverified')),
    author_replica_id TEXT NOT NULL,
    author_sequence INTEGER NOT NULL CHECK (author_sequence > 0),
    recorded_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS sync_revision_pending_bodies_document_idx
    ON sync_revision_pending_bodies(document_id, revision_id);

-- A durable, visible disagreement about one document's body. It is attached to
-- the document and its revision graph rather than being a second note, so it
-- cannot invent a title or a notebook and cannot leak into search, publication,
-- or the graph report.
--
-- The two conflicting revisions are stored in sorted order rather than as
-- "mine" and "theirs": each replica calls a different one local, and a conflict
-- with two identities would be reported twice and resolved once. The UI names
-- the sides by comparing each revision's authoring replica with its own.
CREATE TABLE IF NOT EXISTS sync_document_conflicts (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL,
    base_revision_id TEXT NOT NULL,
    revision_a TEXT NOT NULL,
    revision_b TEXT NOT NULL,
    kind TEXT NOT NULL,
    region_count INTEGER NOT NULL DEFAULT 0 CHECK (region_count >= 0),
    regions_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(regions_json)),
    regions_truncated INTEGER NOT NULL DEFAULT 0 CHECK (regions_truncated IN (0, 1)),
    detected_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (revision_a < revision_b)
);

CREATE INDEX IF NOT EXISTS sync_document_conflicts_document_idx
    ON sync_document_conflicts(document_id, id);

-- Every revision that enters the journal must name its exact content. A
-- revision without one could not be verified after transfer, could not be a
-- delta base, and could not be found as a merge ancestor, so it is refused at
-- the point it would be created rather than discovered later as a gap.
DROP TRIGGER IF EXISTS sync_capture_revisions_insert;
CREATE TRIGGER sync_capture_revisions_insert
AFTER INSERT ON document_revisions
WHEN EXISTS (SELECT 1 FROM sync_local_journal WHERE singleton = 1)
BEGIN
    SELECT CASE
        WHEN NEW.content_sha256 = '' OR length(NEW.content_sha256) <> 64 OR NEW.content_length < 0
        THEN RAISE(ABORT, 'sync revision requires an exact content hash and length')
    END;

    INSERT INTO sync_journal_capture VALUES(
        'revision.create', 'revision', NEW.id,
        json_object(
            'document_id', NEW.document_id,
            'title', NEW.title,
            'body_mime_type', NEW.body_mime_type,
            'message', COALESCE(NEW.message, ''),
            'parents', json(NEW.parent_revision_ids),
            'content_sha256', NEW.content_sha256,
            'content_length', NEW.content_length,
            'body', CASE
                WHEN EXISTS (SELECT 1 FROM sync_revision_transfer t WHERE t.revision_id = NEW.id)
                    THEN (SELECT t.inline_body FROM sync_revision_transfer t WHERE t.revision_id = NEW.id)
                -- Without a Go-measured payload the fallback inlines only a
                -- small body. The cap is 65,536 raw bytes because the worst
                -- case JSON escaping of UTF-8 text is six bytes per byte
                -- (\u00XX for a control character), and 6 × 65,536 stays under
                -- G2's 512 KiB payload ceiling for any input whatsoever. G1
                -- measured every corpus p99 below 9 KiB, so the fallback covers
                -- the overwhelming majority of real notes.
                WHEN NEW.content_length <= 65536 THEN NEW.body
                ELSE NULL
            END,
            'delta', json(COALESCE(
                (SELECT t.delta_json FROM sync_revision_transfer t WHERE t.revision_id = NEW.id),
                'null'))
        )
    );

    DELETE FROM sync_revision_transfer WHERE revision_id = NEW.id;
END;

PRAGMA user_version = 22;
