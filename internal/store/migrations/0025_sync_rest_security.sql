-- v0.7 G13: the REST security foundation.
--
-- Two things move into the database here, and both are deliberate.
--
-- Peer *public* signing keys become canonical database state rather than
-- entries in a local key file. They are public, so nothing secret moves; what
-- is gained is that enrolment and revocation are transactional, auditable, and
-- visible to every process at once. A key file read by a daemon and rewritten
-- by a CLI is a race with a security outcome.
--
-- Pairing invitations are durable for the same reason: single use has to
-- survive a restart, and "used already" must be decided by one transaction
-- rather than by whichever process answered first.

CREATE TABLE IF NOT EXISTS sync_peer_keys (
    signer_key_id TEXT PRIMARY KEY,
    replica_id TEXT NOT NULL,
    public_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    enrolled_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TEXT,
    reason TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS sync_peer_keys_replica_idx
    ON sync_peer_keys(replica_id, status);

-- One-use, short-lived pairing invitations.
--
-- The secret itself is never stored: only its SHA-256, so a stolen database
-- does not yield a usable invitation, and an expired or consumed row is
-- evidence rather than a credential. `id` is derived from the secret so a
-- presenter can be looked up without sending the secret itself.
CREATE TABLE IF NOT EXISTS sync_pairing_invitations (
    id TEXT PRIMARY KEY,
    secret_sha256 TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'consumed', 'revoked')),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TEXT NOT NULL,
    consumed_at TEXT,
    consumer_replica_id TEXT,
    label TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS sync_pairing_invitations_status_idx
    ON sync_pairing_invitations(status, expires_at);

PRAGMA user_version = 25;
