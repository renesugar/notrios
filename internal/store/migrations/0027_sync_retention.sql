-- Schema v27: explicit peer retirement and acknowledgement/snapshot-gated
-- retention. Verified snapshot availability is local operational evidence;
-- retirement decisions, collection floors, and death-certificate identity are
-- durable protocol state.

CREATE TABLE IF NOT EXISTS sync_peer_retirements (
    replica_id TEXT PRIMARY KEY,
    decision_replica_id TEXT NOT NULL,
    decision_sequence INTEGER NOT NULL CHECK (decision_sequence > 0),
    signer_key_id TEXT NOT NULL,
    signature TEXT NOT NULL CHECK (length(signature) > 0 AND length(signature) <= 2048),
    reason TEXT NOT NULL DEFAULT '',
    retired_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sync_verified_snapshots (
    snapshot_id TEXT PRIMARY KEY,
    commit_sha256 TEXT NOT NULL CHECK (length(commit_sha256) = 64),
    created_at TEXT NOT NULL,
    verified_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sync_verified_snapshot_vectors (
    snapshot_id TEXT NOT NULL REFERENCES sync_verified_snapshots(snapshot_id) ON DELETE CASCADE,
    replica_id TEXT NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence >= 0),
    PRIMARY KEY (snapshot_id, replica_id)
);

CREATE INDEX IF NOT EXISTS sync_verified_snapshots_verified_idx
    ON sync_verified_snapshots(verified_at DESC, snapshot_id);

CREATE TABLE IF NOT EXISTS sync_retention_floors (
    replica_id TEXT PRIMARY KEY,
    sequence INTEGER NOT NULL CHECK (sequence >= 0),
    snapshot_id TEXT NOT NULL,
    advanced_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sync_tombstone_payloads (
    document_id TEXT PRIMARY KEY,
    purge_replica_id TEXT NOT NULL,
    purge_sequence INTEGER NOT NULL CHECK (purge_sequence > 0),
    purged_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    collected_at TEXT
);

-- Compaction may remove a revision's creation operation after every active
-- peer and one verified snapshot cover it. Its protocol order remains needed
-- for deterministic future merges.
CREATE TABLE IF NOT EXISTS sync_retained_revision_orders (
    revision_id TEXT PRIMARY KEY,
    hlc_wall_ms INTEGER NOT NULL,
    hlc_logical INTEGER NOT NULL,
    replica_id TEXT NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence > 0)
);

-- Metadata checkpoints need the minimum permanent-delete identity even after
-- the operation carrying it is compacted and the document payload collected.
CREATE TABLE IF NOT EXISTS sync_metadata_baseline_deaths (
    document_id TEXT PRIMARY KEY,
    signer_replica_id TEXT NOT NULL,
    signer_sequence INTEGER NOT NULL CHECK (signer_sequence > 0),
    signature TEXT NOT NULL CHECK (length(signature) > 0 AND length(signature) <= 2048),
    hlc_wall_ms INTEGER NOT NULL,
    hlc_logical INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS sync_tombstone_payloads_pending_idx
    ON sync_tombstone_payloads(collected_at, purged_at, document_id);

PRAGMA user_version = 27;
