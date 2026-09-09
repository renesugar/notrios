-- v0.7 G15: durable, bounded sync work layered on the v0.6 job records.
--
-- This is intentionally not a general scheduler. There are no priorities,
-- dependencies, arbitrary commands, cron expressions, or payload locations.
-- A row can only extend a closed sync job kind, and target_id is an opaque
-- application digest rather than a directory, URL, credential, or key.

CREATE TABLE IF NOT EXISTS sync_jobs (
    job_id TEXT PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
    actor TEXT NOT NULL CHECK (actor IN ('cli', 'rest', 'mcp', 'service')),
    target_id TEXT NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 8 CHECK (max_attempts BETWEEN 1 AND 32),
    next_attempt_at TEXT,
    retry_code TEXT NOT NULL DEFAULT '',
    byte_budget INTEGER NOT NULL CHECK (byte_budget > 0),
    bytes_used INTEGER NOT NULL DEFAULT 0 CHECK (bytes_used >= 0),
    checkpoint TEXT NOT NULL DEFAULT '{}',
    lease_owner TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS sync_jobs_target_idx
    ON sync_jobs(target_id, next_attempt_at, job_id);

CREATE TABLE IF NOT EXISTS sync_job_audit (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    details_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS sync_job_audit_job_idx
    ON sync_job_audit(job_id, created_at, id);

PRAGMA user_version = 26;
