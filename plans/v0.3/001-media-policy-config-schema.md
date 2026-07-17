# v0.3 Task H1 — Media-policy configuration and schema v7

Status: **completed** (2026-07-16). Model: Claude Fable 5 (claude-fable-5).

## Goal

Land the foundation for remote-media hardening: a typed `remote_media` policy
configuration and the schema v7 media-policy tables, reported through
`/api/v1/status`, without implementing any scanning or downloading yet.

## What was built

### Configuration (`internal/config`)

- `RemoteMediaConfig` parsed from the `remote_media` section already
  documented in `config/config.example.yaml`: `default_action`
  (allow/block/review; invalid values fall back to the safe `review`),
  `allow_private_networks`, `max_redirects`, `fetch_timeout_seconds`,
  `blocked_schemes`, `blocked_domains`/`allowed_domains`/`review_domains`,
  `max_bytes.<class>` (human-readable sizes: "20MB", "1GB"), and
  `quarantine_dir`.
- The tiny YAML-subset parser gained dash-list items (first configured item
  replaces the compiled default list) and flow-style inline lists
  (`["file", "ftp"]`); `max_bytes` subsection scalars parse byte sizes.
- Restrictive defaults per `SECURITY_AND_MEDIA_POLICY.md`: action `review`
  (report, never auto-download), private networks blocked, `file`/`data`/
  `javascript`/`ftp` schemes blocked, image 20MB / video 200MB / pdf 100MB.
- `EnsureDirectories` creates the quarantine directory.
- `config.example.yaml` gained `fetch_timeout_seconds`, `quarantine_dir`,
  and `review_domains`.

### Schema v7 (`internal/store`, `migrations/0001_initial.sql`)

- New tables (CREATE IF NOT EXISTS in the migration file):
  `media_domain_rules` (unique case-insensitive pattern; action CHECK
  allow/block/review), `media_hash_rules` (algo+hash PK; kind CHECK
  exact/perceptual; action CHECK block/review), `resource_hashes`
  (blob_sha256+algo PK, storage slot for future perceptual hashes).
- `ensureSchemaV7` shim widens the existing `media_policy_decisions` table
  with quarantine-state columns (`status`, `content_type`, `size_bytes`,
  `quarantine_path`, `updated_at`) plus document/URL indexes, tolerant of
  re-runs; `PRAGMA user_version = 7`.
- Both migration copies (`migrations/` and `internal/store/migrations/`)
  stay identical.

### Status reporting

- `api.StatusResponse.MediaPolicy` (`media_policy` JSON block): default
  action, network limits, per-list rule counts, size caps, quarantine dir.
- Wired in `httpapi.handleStatus`; documented in `api/openapi.yaml`,
  `API_SPEC.md`, `docs/service.md` (full `remote_media.*` config reference
  replacing the "reserved" row), and `DATABASE_SCHEMA.md` (v7 section).

## Validation evidence

- `go vet ./...`, `go test ./...` — all 14 packages pass; new tests:
  `TestDefaultRemoteMediaPolicy`, `TestLoadRemoteMediaPolicy`,
  `TestLoadRemoteMediaInvalidValuesKeepSafeDefaults`,
  `TestLoadExampleConfigRemoteMedia`, `TestSchemaV7MediaPolicyTables`
  (fresh bootstrap, column widening, CHECK constraints, idempotent
  re-bootstrap), `TestSchemaV7UpgradeFromV6` (file-backed v6→v7 upgrade);
  status test asserts the `media_policy` block and schema version 7.
- `python3 scripts/check_required_files.py`, `bash scripts/validate-scaffold.sh`,
  `bash scripts/mvp_smoke.sh` — pass.
- `cd web && npm run typecheck && npm test` — 33 tests pass (web unchanged).
- Live check: `notriosd` with the example config reports
  `media_policy: {default_action: review, blocked_schemes: 3, allowed_domains: 4,
  blocked_domains: 1, max_bytes: {image: 20971520, …}}` and
  `database_info.schema_version: 7`.

## Follow-up tasks

- H2 consumes the policy for the remote-media scan endpoint (per-URL
  decisions, no downloads) and the read-only `scan_remote_media` MCP tool.
- H3 implements the quarantine pipeline against `quarantine_dir` and the
  `media_policy_decisions` quarantine-state columns.
- DB-stored `media_domain_rules`/`media_hash_rules` management surfaces
  (they exist but are unpopulated; config lists are the only rule source
  until a later task adds rule CRUD).
