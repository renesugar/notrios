# v0.7 planning amendment — archive scalability before further synchronization — complete

Status: complete on 2026-08-15.

Model: GPT-5 (Codex; exact model variant not exposed).

## Goal and boundary

Insert an evidence-first, independently resumable archive-scalability sequence
before G15. This is a documentation/planning slice. It changes no production
code, schema, dependency, archive default, encryption, REST route, or sync
behavior, and it does not authorize G14a.

## Why the existing evidence is not enough

P3b proved that packed archive-v2 collapses a 382,206-note archive from 382,447
files to 46, with a 1.29× export/verify speedup. P4 proved both layouts restore
the attachment-bearing corpus and recorded a final packed export of 8m22s,
verify of 3m46s, and restore of 12m40s. Those results establish correctness and
file-count value.

G14 nevertheless produces catch-up snapshots by exporting **loose** archive-v2,
adding every archive file as a stored ZIP entry, and sealing the ZIP in
authenticated frames. At 100 and 500 notes, sealed bytes were about 25% larger
than the archive directory. The frame cost is only 28 bytes per MiB; the rest
is entry metadata. No run measures packed archive-v2 through production
catch-up at 382,206 notes, and no equivalent comparison exists against a
consistent SQLite snapshot, restic, or borg.

## Plan change

`PLAN.md` and `ROADMAP.md` now insert five approval-gated slices before G15:

1. **G14a — benchmark contract and resumable harness.** Build aggregate-only,
   phase-checkpointed adapters and prove them on generated calibration tiers.
2. **G14b — full-corpus baseline and selection.** Run the supplied equivalent
   recipe Joplin/Obsidian pair and attachment-bearing Joplin export. Compare
   loose/packed semantic archive-v2, the complete catch-up wrapper, stopped and
   Online Backup API SQLite candidates with packed external assets, restic, and
   borg. Select packed semantic archive-v2, a compatible SQLite-image
   capability, or a further repository investigation.
3. **G14c — production representation.** Implement only the selected and
   recorded design with compatibility, deterministic/bounded decoding, and
   previous-reader evidence.
4. **G14d — restore and catch-up.** Integrate emergency backup, explicit
   destructive intent, identity rotation, resumable transfer, verified cutover,
   catch-up floor, and later incremental replay without an object-per-note
   transport tree.
5. **G14e — full-scale acceptance and freeze.** Repeat the real-corpus matrix
   with production code, reconcile documentation, and freeze the capability
   before G15/G19 depend on it.

Each slice stops after validation, commit, verified release ZIP copied to
`/home/renes/evidence/notrios`, and user approval for the next slice.

## Candidate facts checked before planning

- SQLCipher accurately describes itself as SQLite with strong 256-bit AES
  database encryption. That protects a database image; it does not by itself
  bind Notrios' external asset/source-bundle files, express semantic subset or
  merge, sign a peer snapshot, rotate replica identity, or define archive
  capability compatibility. `mutecomm/go-sqlcipher` also documents SQLCipher
  3/4 incompatibility and leaves major-version migration to the application.
- SQLite's Online Backup API copies a live database into a consistent
  destination snapshot and can release its read lock between page batches.
  Copying only the main file while WAL writes continue remains invalid. A
  stopped, checkpointed database copy is a separate candidate and must still
  include referenced assets.
- restic uses content-defined chunks, authenticated/encrypted blobs, pack
  files, and rebuildable indexes; Borg uses a transactional append-only segment
  log, content-defined chunks, and authoritative key-to-segment indexes. Those
  designs add incremental deduplication, integrity, compression, and general
  filesystem history. They still recreate the source filesystem's individual
  files during an ordinary restore, which is the high-directory-cardinality
  cost Notrios may avoid by restoring canonical database state and packed
  assets.
- Bluge is an Apache-2.0 Go full-text indexing library. It is suitable for
  searchable derived metadata, not as the authoritative transactional map from
  cryptographic object IDs to pack offsets/refcounts. A Go Borg port is a
  separate product/research project and is not required to resolve Notrios'
  snapshot format.
- `klauspost/compress` supplies pure-Go zstd and optimized deflate and `pgzip`
  supplies parallel gzip. They are admissible benchmark candidates by license,
  not selected dependencies; G14b evidence and G14c's dependency review decide
  whether any is needed.

Primary references checked 2026-08-15:

- <https://www.zetetic.net/sqlcipher/>
- <https://www.sqlite.org/backup.html>
- <https://sqlite.org/howtocorrupt.html>
- <https://restic.readthedocs.io/en/latest/100_references.html>
- <https://borgbackup.readthedocs.io/en/stable/internals/data-structures.html>
- <https://github.com/blugelabs/bluge>
- <https://github.com/mutecomm/go-sqlcipher>
- <https://github.com/klauspost/compress>
- <https://github.com/klauspost/pgzip>

## Local aggregate facts

Read-only inventory on 2026-08-15 found 1,619,759 files across the recipe
Joplin and Obsidian inputs and 112,093 files in the attachment-bearing Joplin
input. The reference source and `/media/renes/HD2` report an ext-family
filesystem; `/home/renes/GoogleDrive` reports FUSE. Installed comparison tools
are borg 1.2.8, restic 0.16.4, and SQLite 3.45.1. These are environment facts,
not performance results. No private name, path, content, hash, database,
archive, resource, competitor repository, or cloud content is committed.

exFAT is no longer mounted for testing. G14a/G14b therefore require bounded
directory shape and record the prior failure motivation, but they may not claim
measured exFAT performance.

## Documents reconciled

- `PLAN.md`, `ROADMAP.md`, `CODING_CLIENT_HANDOFF.md`, `agent/PLAN_STATUS.md`;
- `README.md`, `CONTEXT_MAP.md`, `FEATURE_MATRIX.md`;
- `NATIVE_ARCHIVE_V2.md`, `SYNCHRONIZATION.md`, `TESTING_POLICY.md`.

## Validation

Passed on 2026-08-15:

- `cd web && npm audit` before install and after `npm ci`: zero vulnerabilities
  (the sandboxed refresh lost DNS once; the approved network retry passed);
- `npm run typecheck`, 15 Vitest files / 155 tests, and `npm run build`;
- `go test ./...`;
- `python3 scripts/check_plan_loops.py` and
  `python3 scripts/check_required_files.py` (70 required files);
- `bash scripts/validate-scaffold.sh`;
- `bash scripts/build_docs_site.sh` (15 pages);
- `git diff --check` and cross-document next-item/status searches.

Release packaging is produced after the coherent planning commit and verified
with the repository checker before handoff.

## Outcome

**Outcome (2026-08-15).** The physical full-snapshot representation is now an
explicit measured decision rather than an inherited default. G14a is next and
unapproved; G15-G20 are blocked through G14e. No implementation choice was made
cheaply after the fact: G14a records non-blocking measurement defaults, G14b
owns the candidate decision, and G14c repeats that unresolved decision as
blocking until the evidence amends it.
