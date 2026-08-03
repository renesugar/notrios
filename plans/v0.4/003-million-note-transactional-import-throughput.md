# v0.4 J3 — Million-note transactional import throughput

Status: complete

Date: 2026-08-02

Model: GPT-5 (Codex)

## Implemented slice

- Added a bounded canonical Store batch API. One SQLite transaction applies
  documents, revisions, FTS5 rows, same-batch links, provenance, tag/resource
  relations, projection outbox jobs, item fingerprints, and the durable
  checkpoint. Fault-injection coverage proves rollback leaves neither partial
  documents nor an advanced checkpoint.
- Split Joplin note work into a canonical pass and final bounded link pass so
  references to targets created in later batches resolve without new document
  revisions.
- Replaced retained note/note-tag inventories with a temporary indexed SQLite
  manifest, a lean routing parser, bounded insert transactions, deterministic
  order-independent source fingerprints, and keyset pages after the one resume
  offset lookup. Temporary directories are removed on close.
- Kept notebook import-state writes within the configured Store batch limit;
  the real 781-notebook corpus exposed and now covers this boundary.
- Added aggregate SQLite import metrics and private-safe full-import profiling:
  hardware, Go/runtime memory, process RSS, database/page/PRAGMA settings,
  canonical counts, interruption/resume/no-op durations, throughput, search,
  and source immutability.
- Removed a correlated global note-count subquery from the bounded document-tag
  membership API. On the real 3 GB database, the old 500-document query took
  54.32 seconds and the corrected query took 0.42 seconds. User-facing
  count-bearing tag list APIs are unchanged.
- Fingerprint- and source-mapping-verified no-ops skip identical provenance and
  item-state rewrites while the Store still verifies canonical documents and
  commits the checkpoint atomically.

## Real-corpus evidence

`performance/v0.4-j3/recipe-full-import.json` contains aggregate-only evidence
for 1,237,553 source items and 382,206 notes, with a 500-item batch size:

- interrupted after exactly 500 committed notes and resumed successfully;
- 382,206 documents, revisions, FTS rows, and provenance rows;
- 842,813 canonical tag relations;
- 765 canonical batches and 765 final link batches;
- search-ready with five sampled hits;
- all 382,206 notes unchanged on the complete no-op and revision count stable;
- 7m19s interruption pass, 32m22s resume, 18m17s no-op, 58m11s total;
- 3,101,188,096-byte SQLite database and 838,699,456-byte temporary manifest;
- 2,983,720 KiB process peak RSS and 3,037,322,568 bytes Go system memory.

The final standalone dry run used 2,809,924 KiB peak RSS versus J2's 3,769,552
KiB whole-inventory baseline. Evidence contains no source paths, filenames,
titles, bodies, resources, warning text, or databases. Source directory
metadata was unchanged.

## Synthetic evidence

All final 100/10k/100k tiers run planning, real interruption/resume, complete
canonical writes, final links, and a revision-stable no-op. The 100k tier
completed planning in 46.0s, canonical import in 5m32s, and no-op in 3m08s,
with 1,000 canonical plus 1,000 link batches and 174,960 KiB peak RSS.

## Correctness coverage

- Store transaction atomicity across revision/FTS/link/provenance/tag/resource/
  outbox/state/checkpoint changes and rollback;
- cross-batch link resolution;
- checkpoint resume without replay;
- manifest ordering, keyset lookup, relation index, and cleanup;
- lean/full parser routing parity and order-independent/count-sensitive source
  fingerprints;
- more than 500 notebooks with bounded state writes;
- no-op metadata skipping without revisions or state/source corruption;
- aggregate canonical metrics and SQLite settings.

## Validation evidence

- `go vet ./...`
- `go test ./...` in a loopback-capable environment
- focused J3 Store/Joplin/projection tests
- generated 100/10k/100k transactional/no-op profiles
- private-safe recipe dry-run and complete full-import profiles
- required-file/scaffold, frontend, docs, smoke, performance-smoke, packaging,
  and release-ZIP verification gates recorded in `agent/ATTEMPT_LOG.jsonl`

The Joplin RAW import and SQLite FTS5 service skills guided the bounded
inventory, canonical-store, search-readiness, and evidence design. Q1 is the
next incomplete plan task and requires user approval.
