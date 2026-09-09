# v0.7 G14d — Scalable restore, emergency backup, and synchronization catch-up — complete

Status: **complete**

Date: 2026-08-22

Model: GPT-5 (exact serving variant unavailable)

## Goal and boundaries

Integrate G14c's selected compatible physical representation into encrypted
whole-library catch-up and explicit crash-safe replacement without weakening
semantic archive-v2. This slice does not add automatic destructive restore,
filesystem paths to REST/MCP, a scheduler, schema migration, a second physical
format, or a physical-mobile performance claim.

## Transport and admission

- The REST producer creates `sqlite-image+packed-assets.v1` directly from the
  store, then streams it through deterministic sequential USTAR and the existing
  1 MiB NBK1 AES-GCM frames. There is no one-ZIP-entry-per-content-object tree
  and no seekable multi-gigabyte buffer.
- The authorized offer binds format, snapshot/commit/source identity, vector,
  ciphertext length/hash, and opaque requester-owned artifact ID. The consumer
  range-resumes to a durable file, decrypts sequentially, extracts under a fixed
  local staging root, runs the strict G14c verifier, and compares authenticated
  offer metadata before reporting installable state.
- The directory carrier gains bounded resumable publish/download primitives for
  those same sealed bytes, including prefix verification before publisher
  resume. Ordinary small-artifact memory limits are not used for bulk snapshot
  bytes.
- The generic wrapper changes from stored ZIP to deterministic USTAR only. It
  remains an untrusted transport envelope; the physical verifier remains the
  trust boundary. Semantic loose/packed archive-v2 behavior is unchanged.

## Restore and recovery

- `notriosctl snapshot restore --intent replace|adopt` is stopped-service and
  local-only. Replace requires the same database ID; adopt is explicit.
  Merge/fork and incompatible schema use semantic archive-v2.
- Before the first destructive rename, the coordinator creates and completely
  verifies an emergency G14c snapshot of the current database/assets.
- An owner-only adjacent plan persists every state from initialization through
  emergency backup, staging, prepared activation, startup block, asset/database
  moves, installed verification, and completion. Ordinary database open refuses
  while the blocker exists. Re-running the same command rolls forward; changed
  input, intent, or targets are refused.
- Activation occurs on the private staged copy. It retires the copied allocator,
  mints a fresh writable replica, retains the source as a peer, installs the
  snapshot vector and catch-up floors without acknowledging a peer, preserves
  unavailable-resource declarations, and queues every document for external
  index projection. Same-schema SQLite FTS/link/block state is retained but
  non-authoritative; Recoll is never included.
- The installed identity is verified through the coordinator-only open seam
  before startup is unblocked. The verified emergency snapshot remains. The
  completed plan is archived under a unique adjacent name so a later explicit
  restore can start.

## Compatibility and dependencies

No schema, Go/npm module, compressor, native library, vendored code, REST/MCP
path surface, or archive-v2 format changed. The implementation uses the existing
SQLite adapter and Go standard-library tar, hashing, JSON, filesystem, and
cryptography support. Secret keys remain in owner-only key files and are not in
the snapshot.

## Validation

- `npm audit`; `npm ci`; post-install `npm audit`: 0 vulnerabilities.
- `npm run typecheck`; `npm test -- --run`: 15 files / 155 tests;
  `npm run build`.
- `go test ./...`: pass outside the filesystem sandbox because loopback HTTP
  tests require ephemeral sockets.
- `go vet ./...`, plan/required-file checks, documentation build, scaffold,
  MVP, and performance smoke gates: pass.
- Physical restore success verifies the emergency snapshot retains the old
  library, the installed database has the snapshot database ID and a fresh
  replica, and the journal is active.
- Fault injection covers `emergency_verified`, `assets_staged`, `prepared`,
  `cutover_blocked`, `previous_assets_moved`, `assets_installed`,
  `previous_database_moved`, `installed`, and `verified_installed`; startup is
  blocked after the destructive boundary and the same command resumes.
- REST coverage includes interruption/resume, wrong key, tamper, requester
  ownership, bounded frame memory, physical offer/manifest agreement, installed
  restore, and ordinary post-snapshot incremental convergence.
- A sparse destination proves an exact HTTP `Range` resume above 3 GiB without
  materializing preceding bytes. Directory coverage publishes, interrupts,
  resumes, downloads, and compares identical 2.7 MiB bytes through the bulk
  path.
- `bash scripts/run_snapshot_restore_profile.sh
  /tmp/notrios-g14d-restore-100000.json`: generated 100,000 documents; 13.080 s
  restore; 60,557,003-byte snapshot; 62,138,059-byte estimated peak durable
  workspace for the generated target; 2,570,736-byte live heap at report time;
  24,788,992-byte process peak RSS; 100,000 derived documents queued; bounded
  gates passed.
- The Android-emulator follow-up is explicit in
  `performance/v0.7-g14d/ANDROID_EMULATOR_CHECKLIST.md`; no emulator or physical
  device result is claimed.

All generated databases, snapshots, decrypted artifacts, and keys remained
under `/tmp`. Committed evidence is aggregate-only and contains no private
content, path, filename, content hash, database, resource byte, or key.

**Outcome (2026-08-22).** G14d is complete. A large compatible physical snapshot
can be requested, produced, range-resumed, decrypted, strictly verified,
installed under explicit intent with a verified emergency copy, and followed by
ordinary incremental synchronization. Interrupted replacement has a typed,
durable, startup-blocking roll-forward state. G14e is next and approval-gated;
it must repeat the frozen full-corpus matrix before this format is frozen and
G15 can begin.
