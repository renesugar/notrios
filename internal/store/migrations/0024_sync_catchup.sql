-- Schema v24: the durable snapshot catch-up state machine.
--
-- A catch-up is large, slow, and interruptible by design, so where it has got
-- to is a row rather than a variable. Nothing here moves bytes or writes
-- canonical state; it records what was asked, who answered, and how far the
-- process got.

-- Enrollment is not permission. Answering a backup request means producing and
-- handing over a complete copy of the library, so a peer is allowed to be a
-- snapshot source explicitly and separately.
CREATE TABLE IF NOT EXISTS sync_catchup_permissions (
    replica_id TEXT PRIMARY KEY,
    permitted_source INTEGER NOT NULL DEFAULT 0 CHECK (permitted_source IN (0, 1)),
    granted_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sync_catchup_sessions (
    id TEXT PRIMARY KEY,
    role TEXT NOT NULL CHECK (role IN ('requester', 'responder')),
    state TEXT NOT NULL,
    database_id TEXT NOT NULL,
    peer_replica_id TEXT NOT NULL DEFAULT '',
    request_nonce TEXT NOT NULL DEFAULT '',
    snapshot_id TEXT NOT NULL DEFAULT '',
    -- The vector the snapshot was taken at. It is the whole reason a restored
    -- replica can ask only for what came later instead of starting again.
    snapshot_vector_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(snapshot_vector_json)),
    archive_sha256 TEXT NOT NULL DEFAULT '',
    archive_length INTEGER NOT NULL DEFAULT 0 CHECK (archive_length >= 0),
    received_bytes INTEGER NOT NULL DEFAULT 0 CHECK (received_bytes >= 0),
    wrapping_mode TEXT NOT NULL DEFAULT 'peer-key' CHECK (wrapping_mode IN ('peer-key', 'password')),
    -- Restore intent is mandatory and never inferred: G10's boundary is that
    -- there is no automatic destructive restore.
    intent TEXT NOT NULL DEFAULT '',
    expires_at TEXT NOT NULL DEFAULT '',
    last_reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- What a restored snapshot already contains, per replica. G5 admits an
-- operation only when its immediate predecessor is present, which a replica
-- built from a snapshot does not have and must not have: the snapshot *is* that
-- history, in canonical form. A floor says "everything at or below this
-- sequence arrived as canonical state", so the operation after it is contiguous
-- and anything below it is already contained.
CREATE TABLE IF NOT EXISTS sync_catchup_floors (
    replica_id TEXT PRIMARY KEY,
    sequence INTEGER NOT NULL CHECK (sequence >= 0),
    snapshot_id TEXT NOT NULL DEFAULT '',
    established_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS sync_catchup_sessions_state_idx
    ON sync_catchup_sessions(state, updated_at);

PRAGMA user_version = 24;
