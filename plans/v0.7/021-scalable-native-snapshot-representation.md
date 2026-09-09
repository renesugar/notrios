# v0.7 G14c — Scalable native snapshot representation — complete

Status: **complete**

Date: 2026-08-22

Model: GPT-5 (exact serving variant unavailable)

## Goal and boundaries

Implement G14b's selected compatible whole-library physical representation
without weakening semantic archive-v2 or widening network/filesystem APIs.
This slice owns production creation and read-only admission only. It does not
replace the G14 catch-up producer, wrap or transport an artifact, make an
emergency backup, install a database, rotate a replica, rebuild derived state,
or replay incremental operations; those are G14d.

## Implementation

- `internal/store/sqlite_snapshot.go` uses SQLite Online Backup to create one
  consistent image, enables secure deletion while clearing reviewed local and
  transient tables, then vacuums copied freelist pages; it retains
  canonical/admitted history, inspects schema,
  identity, vector and floors, runs `PRAGMA integrity_check`, and pages the
  external object inventory 1,000 rows at a time.
- `internal/snapshotimage` defines required
  `sqlite-image+packed-assets.v1` plus semantic archive-v2 fallback capability.
  It writes deterministic uncompressed USTAR packs that close at 256 MiB
  payload or 65,536 entries; a single larger object is declared oversized.
  Packs publish through private partials, fsync, full verification and atomic
  rename. Only a verified final pack is resumable; the SQLite image restarts.
  `manifest.json` is the sole completion marker and publishes last.
- Admission strictly checks the manifest/capability, exact schema 25 and
  Notrios application ID, database/pack hashes and lengths, SQLite integrity,
  database identity, vector/floors, cleared local state, canonical USTAR
  headers, content-addressed paths, every object hash, and exact agreement
  between database-declared local objects and pack entries. It never extracts
  or installs.
- `notriosctl snapshot create|verify` is local-filesystem only and tracked by
  the existing job control plane. No REST/MCP path was added.
- `performance/v0.7-g14c/STATE_REVIEW.md` is the exhaustive schema-v25 state
  classification. Recoll and secret key files remain outside the image.

## Compatibility and dependencies

Loose and packed archive-v2 readers/export/restore remain unchanged and tested.
Physical and semantic readers reject the other format rather than silently
reinterpreting it. An incompatible physical image directs the operator to
semantic archive-v2 and is not migrated in place.

No Go/npm module, compressor, native library, or tool dependency was added.
The implementation uses the existing system SQLite library and Go standard
library. The inventory is in
`performance/v0.7-g14c/LICENSE_INVENTORY.md`.

## Validation

- `npm audit`; `npm ci`; `npm audit`: 0 vulnerabilities.
- `npm run typecheck`; `npm test -- --run`: 15 files / 155 tests; `npm run build`.
- `go test ./...`: pass outside the filesystem sandbox (localhost tests need
  ephemeral sockets).
- `bash scripts/validate-scaffold.sh`: pass outside the sandbox; all 70
  required files present.
- `go vet ./...`: pass.
- `bash scripts/build_docs_site.sh`: pass, 15 pages indexed.
- `bash scripts/mvp_smoke.sh`: pass outside the sandbox (loopback bind needed).
- `bash scripts/run_performance_smoke.sh`: pass.
- Independent physical golden built without the production `Create` path.
- Loose archive-v2/current physical reader matrix.
- Deterministic pack hashes, oversized-object declaration, corrupt database,
  truncated pack, malicious expansion size, path traversal, symlink, exact
  schema/capability, manifest-last fault points, and verified-pack resume.
- `bash scripts/run_snapshot_image_profile.sh
  /tmp/notrios-g14c-profile-vacuum.json`: generated 100,000 documents;
  61,181,952-byte image; 18.212 s create; 8.231 s verify; 1,458,288-byte
  reported heap; 23,547,904-byte peak RSS; bounded-memory gate passed.
- Production CLI create/verify smoke passed on a new schema-v25 store.

All generated databases/packages remained under `/tmp`. Committed evidence is
aggregate-only and contains no private content, path, filename, hash, database,
or asset.

**Outcome (2026-08-22).** G14c is complete. The selected physical snapshot is
a production versioned local representation with bounded shape, strict
admission, deterministic packs, and explicit interruption rules. G14d is next
and approval-gated; it must integrate encryption/transport and prove emergency
backup plus crash-safe cutover/replay before this representation becomes the
sync catch-up path.
