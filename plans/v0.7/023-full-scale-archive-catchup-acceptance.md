# v0.7 G14e — Full-scale archive/catch-up acceptance and format freeze — complete

Status: **complete**

Date: 2026-08-23

Model: GPT-5 (exact serving variant unavailable)

## Goal and boundaries

Run the frozen G14b comparison boundaries through production G14c/G14d code,
fix only defects in that approved contract, and freeze the compatible
whole-library default before jobs, UI, retention, or compatibility work depends
on it. Evidence is aggregate-only. No private database/archive/object,
cloud-provider rerun, exFAT measurement, physical-mobile result, public release,
new format, automatic restore, REST/MCP path, schema, compressor, or dependency
is claimed.

## Accepted format contract

- Compatible same-schema whole-library backup/catch-up defaults to
  `sqlite-image+packed-assets.v1`: a sanitized SQLite Online Backup image plus
  deterministic bounded USTAR external packs and a manifest-last strict
  verifier.
- Encrypted REST/directory catch-up carries that physical snapshot through
  deterministic sequential USTAR and fixed-frame NBK1. The wrapper is not the
  trust boundary; the physical manifest/database/pack verifier is.
- Semantic archive-v2 remains the portable record-level contract for subset,
  merge, fork, publication/interchange, incompatible schema, and fallback.
  Current and previous packed archives both verify and restore with the current
  reader. No archive-v3 was introduced.
- G19 must expose this split: portable consumers identify/refuse the physical
  capability rather than reinterpreting it as semantic archive-v2.

## Production defects exposed and fixed

1. Synchronous full-snapshot creation inherited the ordinary 30-second HTTP
   client/server deadline. Only the authenticated, explicitly permitted backup
   operation now receives the existing bounded two-hour G14 stage deadline;
   ordinary API requests remain short.
2. The downloader requested 16 MiB ranges while the generic authenticated
   client truncated every response at 8 MiB. Signed Range replies now have a
   separate 16 MiB ceiling; ordinary responses remain capped at 8 MiB.
3. Admission invoked global metadata, revision, and attachment reconcilers for
   every admitted operation. A body-only edit consequently loaded/rebuilt the
   382k-record metadata projection and exceeded 3 GiB RSS. Admission now selects
   only the record-family reconcilers the new contiguous batch touched; a
   document operation containing only `current_revision_id` stays with the
   bounded revision reconciler.
4. Each bounded directory publication call compared the whole durable prefix
   again, making many-call resume quadratic. A bound `Directory` validates the
   prefix once per process, remembers the exact synced length, and hashes the
   completed staging file in full before publication. A restarted process
   revalidates its durable prefix once. Carrier mutation still causes refusal/
   restart and cannot admit different bytes.

Focused unit/integration tests cover operation-specific deadlines, response
ceilings, scoped record-family reconciliation, once-per-process prefix
validation including restart, small production catch-up, and the privacy/non-
JSON harness regressions found during the full run.

## Full-scale evidence

`performance/v0.7-g14e/FULL_SCALE_ACCEPTANCE.json` contains 19 independently
validated aggregate phases:

- exact equivalence of the 382,206-document Joplin and Obsidian canonical views;
- attachment workload counts of 103,349 documents, 758 logical resources, 731
  blobs, and 111,330 preserved source-bundle items;
- current create/verify/restore and previous verify/restore for packed semantic
  archive-v2;
- first, verify, unchanged, and exact restore for recipe and attachment physical
  snapshots;
- full production REST and directory resumable encrypted catch-up, strict
  verification, emergency replacement, replica rotation, and post-snapshot
  convergence;
- full-data integrity checks for the frozen Restic/Borg canonical/raw reference
  repositories.

Every native stage stayed below 7,200 seconds. Native receiver stages stayed
below 256 MiB and sender stages below 512 MiB. Packed semantic output was 46
files; physical recipe output was 2 files and attachment output 8, so no
transport tree scales per object. The final optimized catch-up completed in
2,948.669 seconds at 210,010,112 bytes peak RSS; its 382,209-document private
working state and 382,211 revisions matched host/restored canonical fingerprints
exactly after post-vector replay. The raw Restic data check reached
2,256,838,656 bytes RSS and remains a comparison result explaining why the raw
repository shape was not selected, not a native receiver gate.

The committed validator refuses private paths/slugs and any 64-hex private
fingerprint. Claims explicitly record that exFAT, a cloud-provider rerun,
Android/physical mobile, and public data were not measured or published.

## Repository validation

- `npm audit`; `npm ci`; post-install `npm audit`: 0 vulnerabilities.
- `npm run typecheck`; `npm test -- --run`: 15 files / 155 tests;
  `npm run build`: pass.
- `go test ./...`: pass, including loopback HTTP integrations and G14e tests.
- `go vet ./...`: pass.
- `python3 performance/v0.7-g14e/validate_evidence.py`: 19 aggregate phases and
  frozen production format validated.
- Seven harness unit tests, Python compilation, shell syntax, gofmt, and
  `git diff --check`: pass.
- Required-file and scaffold validation: 70 files, pass.
- Documentation build/Pagefind: 15 pages, pass.
- Real-service MVP smoke, generated store/import performance smoke, and offline
  asset/CSP check: pass; no third-party request or CSP violation.

All private corpora, artifacts, fingerprints, repositories, paths, and logs
remain in the external evidence workspace. The repository contains only the
privacy-validated aggregate JSON and reproducible harness/tool sources.

**Outcome (2026-08-23).** G14e is complete. The compatible whole-library
physical snapshot/catch-up default is frozen without weakening or replacing
semantic archive-v2. G15 is next and separately approval-gated.
