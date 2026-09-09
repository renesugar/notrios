# v0.4 P3a — Archive-v2 large-library container revision

Status: complete on 2026-08-04.

Model: Claude Opus 5 (Claude Code).

## Why

P3 shipped a correct manifest-last writer and then measured what the P2
container could actually hold. `manifest.json` listed every object inline at
roughly 645 bytes per descriptor, so the 4 MiB manifest bound — not the
nominal 10,000-object limit — capped an archive near **6,500 objects, about
6,400 single-revision notes**.

The supplied test corpora hold 382,206 notes each (`recipe_joplin` and
`recipe_vault`), and the Joplin RAW export holds 1,237,553 source items. J3
already imports that corpus, so export and verification were roughly 60× short
of the data the project is tested against: archive v2 could not archive the
real test data at all.

This slice resolves `agent/OPEN_QUESTIONS.md` question 17. It is sequenced
ahead of P4 because a restore path built against the old in-memory verifier
would have been rewritten, and ahead of P6 because that task pins the
container for an external consumer.

## Format changes

- **The object inventory left the manifest.** `manifest.json` now carries
  `index` (the index chunks, in entry order) and `totals` (aggregate object,
  byte, record-chunk, and blob counts). Index chunks are LF-terminated
  `application/vnd.notrios.archive-v2-index+jsonl` objects whose entries are
  globally sorted by object hash across chunks.
- **The checksum chain is unbroken.** The manifest commit digest binds each
  chunk hash; each chunk binds the hash of every object it names. Corrupting a
  chunk breaks its own hash, and rehashing it breaks the manifest.
- **Index chunks are named by the manifest, not by the index**, so the
  inventory never lists itself.
- **Discriminated `location`.** Every entry declares its layout. `fanout` is
  the only one this build writes, so a packed layout can arrive behind an
  optional capability rather than a second breaking revision.
- **Two-level fanout** (`objects/sha256/ab/cd/<hash>`). One level would put a
  million-note archive's objects into 256 directories of tens of thousands of
  entries; two keeps them near 25, and it matches the asset store. A path's
  lexical order is its hash's, which is what lets pruning and file counting be
  merges rather than set lookups.
- **`objects.index.v1` is a required capability**, so no reader can mistake an
  index-form archive for the inline form.
- **Limits re-derived from a 1,000,000-note target**: 8,000,000 objects,
  1,000 index chunks of 10,000 entries, 64,000,000 records, 1,000,000
  notebooks. The manifest bound stayed at 4 MiB because it no longer scales
  with the archive.

The index form is the only v2 form. No archive exists outside this repository
and P6 has not pinned anything, so the fixtures were regenerated instead of
carrying an inline-inventory compatibility path.

## Bounded writer

Nothing proportional to the archive stays on the heap.

- **No object table.** The published object tree *is* the deduplication index:
  an object exists exactly when its content-addressed file exists. The old
  `map[string]Object` would have cost hundreds of megabytes at real scale.
- **External-sorted index.** Entries stream into a 256-bucket spool keyed by
  the object hash — the same first byte that selects the fanout directory — so
  replaying buckets in order yields globally sorted entries while holding one
  bucket. Duplicate hashes collapse during the replay and are reported as
  `deduplicated_objects`; a hash appearing with a different kind or size is an
  error.
- **Merge-based pruning.** The sorted expected-path list is merged against the
  sorted object tree, so a resumed export with a different selection still
  removes stale objects without a path set.

## Bounded verifier

`verificationState` previously held twelve maps of full record structs; at the
recipe corpus's record count that is gigabytes.

- Only the two genuinely bounded sets stay in memory: collections
  (`MaxCollections`) and the notebook tree (`MaxNotebooks`, needed for cycle
  and depth checks).
- Every record identity and object hash goes to a declaration spool or a
  reference spool, merge-joined per bucket. The join reports references with no
  declaration, duplicate declarations, and blob objects nothing references.
- **Composite keys fold consistency checks into the join.** A revision key
  carries its document ID, so a current-revision pointer at the wrong document
  finds no declaration. A blob key carries its exact byte length, so a
  size disagreement fails the same way.
- **"No extra files" is proven by counting.** Every declared object is opened
  during the index scan, so an equal file count leaves no room for an unlisted
  one — no path set required.
- Records are read once: the index scan verifies object bytes and decodes
  records in the same pass, emitting both spools together.

## Evidence

See `performance/v0.4-p3a/`.

## Validation

- focused container fixtures: manifest size independent of archive size, index
  chunking, fanout layout, sorted unique entries, index-chunk corruption;
- focused spool fixtures: balanced join, missing declaration, duplicate
  declaration, unreferenced blob, wrong blob length, global key ordering;
- regenerated golden fixture plus the generator-equality test, and every P2
  adversarial fixture updated to the index form;
- the complete P3 export fixture suite unchanged in intent;
- generated 100/1,000/5,000/100,000-note export, verify, resume, and subset
  profiles;
- a real 382,206-note Joplin RAW corpus import, export, and verification;
- `go vet ./...` and `go test ./...`;
- required-file, scaffold, web typecheck/test/build, docs site, MVP smoke, and
  generated performance smoke checks;
- verified release ZIP copied to `/home/renes/evidence/notrios`.

## Layout follow-up

The measurement this slice produced answered the layout question it left open.
The real corpus wrote 382,407 loose objects in 48m26s — about 131 objects per
second for only 1.14 GB — because throughput tracks one
`create + write + fsync + rename` per object rather than bytes, and v0.7 sync
would additionally pay one transport round trip per object.

`agent/OPEN_QUESTIONS.md` question 18 is therefore resolved in favor of a
packed layout, scheduled as plan task **P3b**. The discriminated `location`
this slice introduced is what lets that arrive as an optional capability
rather than another format break.

## Next task

P3b, the packed object layout, requires explicit user approval. P4 follows it
and must reuse the spooled verification rather than reintroducing in-memory
record sets.
