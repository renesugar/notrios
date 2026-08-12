-- Schema v20: persist the compatibility contract of explicitly configured G5
-- admission peers and refuse local sequence exhaustion before SQLite can
-- promote an overflowing integer to REAL. This still adds no transport,
-- cryptographic enrollment, or canonical merge behavior.

CREATE TABLE IF NOT EXISTS sync_peer_compatibility (
    replica_id TEXT PRIMARY KEY REFERENCES sync_replicas(replica_id) ON DELETE CASCADE,
    protocol_major INTEGER NOT NULL CHECK (protocol_major > 0),
    protocol_min_minor INTEGER NOT NULL CHECK (protocol_min_minor >= 0),
    protocol_max_minor INTEGER NOT NULL CHECK (protocol_max_minor >= protocol_min_minor),
    schema_version INTEGER NOT NULL CHECK (schema_version > 0),
    min_compatible_schema INTEGER NOT NULL CHECK (min_compatible_schema > 0),
    max_compatible_schema INTEGER NOT NULL CHECK (max_compatible_schema >= min_compatible_schema),
    required_capabilities_json TEXT NOT NULL CHECK (json_valid(required_capabilities_json)),
    optional_capabilities_json TEXT NOT NULL CHECK (json_valid(optional_capabilities_json)),
    configured_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER IF NOT EXISTS sync_journal_sequence_exhaustion
BEFORE INSERT ON sync_journal_capture
WHEN EXISTS (
    SELECT 1 FROM sync_local_journal
     WHERE singleton = 1 AND last_sequence >= 9223372036854775806
)
BEGIN
    SELECT RAISE(ABORT, 'sync sequence exhausted');
END;

PRAGMA user_version = 20;
