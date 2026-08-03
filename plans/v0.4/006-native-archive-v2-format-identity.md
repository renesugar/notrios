# v0.4 P2 — Native archive v2 format and identity contract

Status: complete on 2026-08-03.

Model: GPT-5.

## Scope

P2 defines the lossless native archive-v2 identity, manifest, immutable object,
typed record, compatibility, limit, MIME, completion, and restore-intent
contracts. It implements complete read-only fixture verification before any
future canonical restore write.

P2 does not export a live database (P3), restore/import archive records (P4),
add REST/MCP filesystem tools, implement stable external-link routing (P5), or
modify `movenotes-v3` (P6).

## Implementation

- Schema v12 adds the single canonical `database_identity` row. Random
  `database_id` identifies the logical universe; random `replica_id` identifies
  one writable copy. Bootstrap/reopen preserve both and the internal Store
  rotation operation changes only the replica ID.
- `internal/archivev2` defines strict manifest/snapshot/compatibility/object/
  count types, all twelve canonical record envelopes, canonical MIME/blob
  references, a semantic manifest commit SHA-256, and deterministic object
  ordering.
- The manifest binds P1's complete selection digest, source database and
  replica identity, collection scope, SQLite read-transaction consistency,
  source/reader schema range, and sorted required/optional capabilities.
- Objects are immutable regular files at exact
  `objects/sha256/<prefix>/<hash>` paths. LF-terminated typed JSONL chunks
  contain records; separate blob objects hold revision bodies, resources, and
  exact source-bundle bytes. FTS5/Recoll/outbox/cache data is excluded.
- Restore identity planning has no default. Replace/adopt require a full
  archive and preserve its database universe; merge keeps the target universe;
  fork requires an explicit distinct new ID. Every resulting writable copy
  must mint a replica ID.
- `VerifyDirectory` treats `manifest.json` as the manifest-last completion
  marker, strictly decodes JSON, enforces size/count/path/JSON/notebook depth
  bounds, verifies every byte/hash/size/MIME, reconciles decoded counts and
  record references, and rejects unreferenced blobs before returning success.

## Golden and adversarial evidence

`internal/archivev2/testdata/golden-minimal/` is a committed complete synthetic
archive with one record of every type and three referenced blobs. It contains
no private data. Tests mutate copies and reject:

- absent/corrupt manifests and manifest commit drift;
- missing/corrupt/unlisted objects, extra files, symlinks, and traversal;
- unsupported archive versions, schema ranges, and required capabilities;
- invalid MIME, record/manifest counts, current-revision and other references;
- unknown/duplicate JSON fields, unknown record types, unsafe bundle paths,
  and configured count/depth overflow.

P4 may replace in-memory reference sets with an indexed verification spool for
very large archives; it must preserve the same admission semantics and remain
complete before its first canonical transaction.

## Validation

- focused schema-v12 bootstrap/reopen/v11-upgrade/replica-rotation tests;
- focused valid and hostile archive-v2 fixture/identity tests;
- `go vet ./...` and `go test ./...`, including loopback media and live Recoll;
- required-file, scaffold, migration-copy, JSON/fixture, and diff checks;
- web typecheck/tests/build and 13-page docs/Help build;
- MVP and generated performance smoke checks;
- verified release ZIP copied to `/home/renes/evidence/notrios`.

## Next task

P3, native archive-v2 streaming export, requires explicit user approval.
