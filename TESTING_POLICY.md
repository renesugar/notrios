# Testing Policy

## Definition of done

A task is done only when:

- The code or document change is complete for the promised scope.
- Relevant tests pass or missing tests are explicitly documented.
- `agent/PLAN_STATUS.md` and `agent/ATTEMPT_LOG.jsonl` are updated.
- The repository remains in a working state.

## Test layers

### Unit tests

- Go service packages: business logic, parsers, media policy, cursor encoding, link parsing.
- TypeScript UI: API client, routing helpers, link/resource URI handling.

### Integration tests

- SQLite migrations.
- Document CRUD + revisions + FTS5 updates.
- Resource upload/download and reference counting.
- Importer fixtures.
- Joplin RAW hierarchy/tag/source-bundle, dry-run parity, changed-resource,
  conflict, fingerprint, and interruption/resume fixtures. Canonical fixtures
  use first-line titles and cover CR/LF-only physical splitting, OCR control
  characters, UTF-8 BOM/invalid input, duplicate/future property keys, and
  delimiter whitespace.
- REST/MCP service-layer parity.

### UI tests

Planned tools:

- Playwright for browser UI smoke tests.
- xvfb on Linux CI where needed.

Minimum UI flows:

- Create note.
- Edit and save note.
- Preview note.
- Click internal document link.
- Upload/download resource.
- Resource report invariants: exact duplicates may span collections; a blob is
  unreferenced only when none of its logical resources has any document
  reference; trashed-note references remain protective; notebook usage counts
  current notes only.
- Perceptual-hook invariants: the default is inert, hashes are reused per exact
  blob/algorithm, `block` rules are rejected, and near matches never collapse
  distinct SHA-256 blobs.
- Garbage-collection invariants: no reference (including Trash) may be crossed;
  dry run never mutates; apply rechecks state; purged resources use their
  longer window; shared blobs survive while any logical resource remains; a
  custom retention gate can defer otherwise-expired candidates.
- Search and open result.

Layout-resize testing rule: never verify window-resize behavior through
Playwright's emulated viewport (`set_viewport_size`) or by eyeballing
screenshots — the emulated canvas redraws cleanly while the real window
never changes, hiding integration bugs. `scripts/verify_layout_resize.py`
is the reference harness: it launches a headed Chromium with no viewport
emulation under Xvfb/Openbox, resizes the actual X11 window with
`xdotool windowsize`, and asserts the pane geometry (bounding rects fill
the window; editor and preview split equally after a resize) from the DOM.

### Performance tests

v0.3 H7 generated datasets cover:

- 10k notes.
- 100k notes.
- 500k note-like short documents.
- 1M links.
- large resource directory with deduplication.

Each profile records hardware/OS/SQLite version, database and index sizes,
query plan, elapsed distribution, and peak RSS for:

- first and next All Notes pages;
- a 90th-percentile-deep traversal (keyset, never a fabricated giant offset);
- selective/nonselective FTS queries and notebook/tag filters;
- importer inventory/write batches and native archive streaming;
- first/next merged FTS5/Recoll pages when Recoll is installed.

Ordinary local first/next pages target p95 below 100 ms on the recorded
reference machine. Memory, subprocess output, and rendered rows must remain
proportional to the page/batch limit. This target catches regressions but is not
a machine-independent product guarantee.

H7's executable harness is `scripts/run_large_library_profile.sh`; its committed
10k/100k/500k JSON evidence, including the precise environment and query plans,
lives under `performance/v0.3-h7/`. The ordinary-page gate is asserted by the
test. Recoll-specific measurements remain conditional on Recoll being installed
and are part of H10 sidecar hardening.

H8/J3's executable harness is `scripts/run_joplin_import_profile.sh`. It accepts
100, 10k, or 100k generated Joplin notes with nested folders, tags, and
resources. Every tier runs the same bounded dry-run planner, interrupts a real
canonical import at a durable note-batch boundary, resumes it, completes a
final link pass, and verifies a revision-stable full no-op. Reports record dry
run/import/no-op duration, canonical/link batch counts, temporary manifest
size, environment, Go memory, and process peak RSS. Exact unknown/reordered-
property and CRLF-byte preservation is covered separately by the focused
importer fixture.

v0.4 J2 adds `scripts/run_real_joplin_profile.sh` for read-only real-export
profiles over the recipe Joplin/Obsidian pair and the attachment-bearing Joplin
archive. Committed evidence under `performance/v0.4-j2/` contains
only aggregate counts, timings, sizes, warnings, and redacted environment
facts—never note titles, bodies, source paths, resources, or databases. J2
measures the production dry-run relationship planner against actual links,
including peak RSS and a no-source-write check. J3 adds
`scripts/run_full_joplin_import_profile.sh` for aggregate-only complete
transactional interruption/resume, search readiness, final links, SQLite
settings/size, and revision-stable no-op evidence. The recipe evidence under
`performance/v0.4-j3/` covers 1,237,553 source items and 382,206 canonical
notes; private paths, titles, bodies, resources, and databases are excluded.

v0.4 Q1 extends `scripts/run_large_library_profile.sh` with correctness and
timing checks for boolean `OR`, grouped field negation, and the recursive
`category:` alias. Aggregate 10k evidence under `performance/v0.4-q1/` records
the established ordinary-page gate plus representative expression metrics.
The intentionally nonselective pure-negation metric is reported separately and
is not represented as an ordinary-page latency guarantee.

v0.4 P1 extends the same generated profile with a complete content-free
full-archive selection dry run. The 100k tier asserts selected document,
reachable resource, and link-classification counts; capped visible details;
and a full manifest digest. Evidence under `performance/v0.4-p1/` records
elapsed time and whole-process peak RSS. Focused fixtures separately cover
recursive notebooks, `any`/`all` tag/query/explicit-ID selectors, secure
publication defaults, private/broken links, source-bundle key hashing,
metadata decisions, deterministic replay, read-only canonical state, and
REST/MCP output parity/caps.

H9's `scripts/run_obsidian_import_profile.sh` uses 100/10k/100k/500k tiers for
generated vaults with nested folders, aliases, relative links,
embeds, heading/block anchors, unknown frontmatter, exact source-bundle
accounting, and local assets. The 100 and 10k tiers inject a durable
interruption and verify resumed dry-run parity; 100k and 500k are bounded
inventory/dry-run profiles. Focused fixtures recover original CRLF Markdown and
binary bytes exactly and cover conflict renames, richer graph edges, stable
asset refresh, Trash, and idempotence.

H10's `scripts/run_recoll_hardening_profile.sh` generates 100k Markdown
projection files and exercises the installed Recoll index/query binaries,
exact bounded result slices, and incremental deletion/addition convergence.
The scale tier uses Recoll's internal plain-text extraction to isolate native
index/result behavior; the ordinary live integration test separately verifies
the production Notrios frontmatter handler and field/range searches. The same
profile records missing/stale/orphan repair followed by a zero-drift
reconciliation. Evidence lives under `performance/v0.3-h10/`.

SQLite's OFFSET cost grows linearly with skipped rows. In an ideal local
1,000,000-row covering-index probe during the 2026-07 plan review, offsets
10k/50k/100k/200k/500k/900k took approximately
0.01/0.02/0.03/0.06/0.13/0.43 seconds while equivalent keyset pages rounded
below 0.01 seconds. The exact crossover depends on joins, sort, cache, storage,
and hardware, so the architectural rule is: use keysets for any unbounded
collection, not “switch after N total notes.”

### Native archive v2 export

v0.4 P3 export fixtures build a complete canonical database (nested notebooks,
tags, a shared resource, cross-boundary links, provenance with private source
metadata, an exact source bundle, a trashed note, and two notes with identical
bodies) and assert that a full archive verifies, carries complete revision
history and the trashed note, deduplicates identical bodies to one object, and
preserves private metadata. Subset fixtures assert the scoped notebook/tag set,
omitted search notebooks, blanked private source `metadata_json`, and
`target_excluded` links whose targets the selection excluded. Further fixtures
cover byte-identical manifests across repeated exports, binding the dry-run
`PlanSelection` digest, an interrupted export verifying as incomplete, a
resumed export reusing published objects and pruning unlisted ones, refusal of
foreign destinations and existing complete archives, refusal of
publication-handoff and content-rewriting link actions, the object budget, and
small record chunking.

`scripts/run_archive_export_profile.sh` drives generated
100/1,000/5,000/100,000-note export, verify, resume, and subset tiers. The
100,000-note tier exists because the pre-P3a container could not reach it.
Evidence under `performance/v0.4-p3/` and `performance/v0.4-p3a/` records
aggregate timings, object/byte counts, manifest size, and whole-process peak
RSS — never note titles, bodies, resources, or local paths.

P3a additionally exports and verifies the real 382,206-note Joplin RAW recipe
corpus. That evidence is aggregate only and no private corpus, database, or
archive is committed.

### Native archive v2 container

v0.4 P3a fixtures cover the container revision that made a real library
archivable. `TestManifestSizeIsIndependentOfArchiveSize` exports 2,000 notes
and asserts the manifest stays under 32 KiB while the object count exceeds the
note count — the inline form needed roughly 1.3 MB at that size and stopped
near 6,500 objects. Further fixtures assert bounded index chunking, the
two-level `ab/cd` fanout, globally sorted unique index entries across chunks,
and that corrupting an index chunk breaks the checksum chain.

Spool fixtures cover the external-sort merge directly: balanced joins,
missing declarations, duplicate declarations, unreferenced blobs, blob
references with the wrong byte length, and global key ordering across buckets.

The golden fixture is produced by a generator that does not use the exporter,
and a test asserts the committed fixture matches that generator byte for byte,
so the verifier is never checked against an archive its own writer produced.

### Native archive v2 packed layout

P3b fixtures cover the optional pack layout: a packed export verifies and
collapses file count, a packed archive declares `objects.pack.v1` while a loose
one does not, using packs without declaring the capability is rejected,
corrupting a pack fails the checksum, pack trailers describe exactly the
objects the index places in packs, and manifest byte totals equal real on-disk
object size. The real-corpus A/B lives under `performance/v0.4-p3b/`.

### Native archive v2 verify and restore

P4 fixtures cover the restore path: a canonical round trip, resource bytes with
their relations/ordinals/anchors, refusal to write anything when verification
fails, mandatory intent, `full_archive` carrying unreferenced resources, and
source bundles staying out of the blob store. Packed-layout fixtures assert
layout equivalence by re-exported object set, resource and bundle bytes read out
of pack slices, corruption refusal, the pack handle-cache contract, and an
archive spanning more packs than the cache holds — which fails 5 times out of 5
without its fix.

Crash/fault injection covers each of the six stages that commit canonical state.
Every injected fault must fail the restore and leave a durable `restore_state`
marker; `adopt`/`merge`/`fork` must refuse a marked library and `replace` must
recover it into a library identical to a clean restore. Further fixtures cover
objects that change between verification and use, and archives written before
`record_counts` became optional.

Resource and source-bundle coverage at scale is mandatory and neither recipe
corpus provides it: the attachment-bearing Joplin RAW archive is the only corpus
carrying resources and exact source bundles. Its aggregate-only evidence lives
under `performance/v0.4-p4/` — counts, hashes, timings, and sizes, never note
content, resource bytes, or local paths.

### Native archive v2 admission

Archive-v2 tests start from a complete synthetic golden directory containing
every canonical record type plus body/resource/source-bundle blobs. Mutated
copies must reject absent manifests, missing/corrupt objects, unsupported
version/schema/required capabilities, commit/count/reference drift, traversal,
symlinks, extra files, unsafe source paths, invalid MIME, duplicate/unknown JSON
fields, and count/path/JSON/notebook depth overflow. Verification is read-only
and completes before any future restore Store transaction. Migration tests also
prove database/replica identity stability and explicit replica rotation.

### Stable external links (v0.4 P5)

Parser fixtures cover the documented shape, anchors, case-insensitive
scheme/authority with case-sensitive identifiers, the foreign-scheme versus
malformed distinction, and rejection of traversal, percent-escapes, embedded
newlines, query strings, unsupported routes, and every length bound.

Registry fixtures cover the round trip and its `0600` permissions, upsert and
remove, unambiguous resolution, ambiguity carrying every candidate in stable
order, the rule that a preferred profile cannot redirect a link into a database
it does not hold, invalid profiles, and refusal of a corrupt registry rather
than partial application.

Store fixtures cover the four resolution statuses, that no local state is
reported for a foreign-database link, link-graph classification of `notrios://`
targets inside note bodies, and that links keep resolving after the database
file is copied to a new path — the property the whole link type exists for.

CLI fixtures build the real binary and assert the exit-code contract an OS
protocol handler depends on (0 opened, 1 unresolvable, 2 malformed), including
a filesystem-level database clone that must produce `ambiguous_database` with
both candidates rather than a choice. Desktop-entry fixtures assert it claims
`x-scheme-handler/notrios` and nothing else.

Web fixtures cover client-side parsing and rejection, deep-link hash parsing,
preview routing of `notrios://` anchors, and the wrong-database message.

Live service and CLI behaviour is recorded in `performance/v0.4-p5/`.

### Publication handoff (v0.4 P7)

Export fixtures reuse the P3 canonical database, whose public note links to an
included note, a withheld note, an unresolved target, an external URL, and an
embedded resource. They assert that withheld and broken links become plain text
or a redaction placeholder while included and external links survive; that the
withheld note appears nowhere in the archive, including in link records and
their context excerpts; that retained links' byte offsets are shifted onto the
published body; that only current revisions, no provenance, no source bundles,
and no saved searches are published; that revision metadata is stripped; and
that the result verifies as an ordinary archive whose report says
`full_backup: false`.

The rewriter is tested directly for the cases where it must refuse: a span past
the end of the body, a span that is not a link, a span whose target has moved,
inverted and negative spans, and overlapping spans. A skipped span leaves the
body unchanged and warns.

Profile fixtures cover the round trip and its `0600` permissions, refusal of
`full_archive` and of an empty selection, refusal of a profile that would
publish Trash, and refusal of a corrupt profile file rather than partial
application. A CLI fixture builds the real binary and asserts the
review-then-publish contract end to end: no digest and a stale digest both
refuse and write nothing, the reviewed digest publishes, and a note joining the
selection after the review invalidates it.

Evidence: `performance/v0.4-p7/`.

### Note blocks (v0.5 E1)

Parser fixtures cover the five block kinds, unclosed fences, tables, the
per-document bound, and determinism. The identity decision is tested directly:
moving a block keeps its ID, editing its text mints a new one, the same text in
two notes is two blocks, identical blocks in one note are disambiguated by
occurrence, a heading and a paragraph reading the same are distinct, and CRLF or
trailing-whitespace changes do not disturb identity.

Store fixtures assert that blocks are rebuilt in the same transaction as the
save, that purging a note removes them, that an authored `^marker` outranks the
derived ID and survives an edit that the derived ID does not, that backlink
counts work for either spelling of an anchor, and that `RebuildDocumentBlocks`
writes no revision. REST fixtures assert the listing, its 404, that no block
text is returned, and that a stale anchor is reported distinctly while still
naming the note.

The generated scale profile adds block metrics at 10k/100k/500k: the real save
path over a bounded sample, and anchor/listing lookups against a synthetically
filled block table so index behaviour is measured at real row counts rather
than at sample size. Evidence: `performance/v0.5-e1/`.

### Heading anchors (v0.5 E1a)

Parser fixtures cover slug derivation — punctuation, Unicode, snake case,
repeated spaces, length bound, and headings that slug to nothing — plus
occurrence disambiguation and the fact that renaming a heading changes both its
slug and its block ID. Store fixtures assert that only headings carry slugs,
that a slug and the heading's text resolve to the same block, that a renamed
heading stops resolving, that precedence is marker before block ID before slug,
and that backlink counts fold both spellings onto one heading. A migration
fixture takes a v14 database to v15 without losing block rows, without inventing
slugs for rows it did not parse, and fills them in on rebuild. REST fixtures
cover the exposed slug and the `stale_anchor` a renamed heading produces; a CLI
fixture builds the real binary and checks `link --anchor` by slug, by heading
text, and by marker, that an unresolvable anchor is refused rather than printed,
and that the anchored link opens end to end.

One fixture records real Markdown behaviour rather than assumed behaviour: a
space ends an unquoted URL, so `[x](document://…#Install & Setup)` truncates at
the space and the heading-text spelling belongs in a wikilink. That is why a
stable link carries the slug.

E1b covers scheme-scoped decoding: scheme recognition and its rejections,
decoding of spaces/`%25`/UTF-8/lowercase hex, invalid escapes left literal, `+`
preserved, identifiers still refusing escapes while anchors accept them, and a
byte-for-byte round trip. Store fixtures assert that an escaped anchor resolves
through a URI-schemed link while the same bytes in a bare Markdown anchor do
not — with lint reporting exactly the bare one — that a decoded URI anchor
counts as a backlink, and that a heading containing a real percent sign resolves
both by text and as `%25`.

### Workspace lint (v0.5 E2)

One fixture carries a single instance of every detectable problem — including a
heading anchor that no longer resolves — alongside healthy content, and asserts both that each check finds its own problem and that
the healthy note and referenced resource appear in no check. Further fixtures
assert that findings carry no note content, that lint is deterministic and
writes no revision, that the detail cap hides examples without changing counts
or the digest, that check selection and limits are validated, and that a trashed
note stops being linted.

Two fixture details record real behaviour rather than assumed behaviour: the
parser records `![alt](x)` as an `embed` rather than an `image`, and
`CreateDocument` substitutes "Untitled" for a blank title — so the missing-title
fixture produces the state the way an importer or an interrupted restore would.

REST fixtures assert the report, its validation, the absence of content in the
response, and that no write method is routed. A CLI fixture builds the real
binary and asserts the exit-code contract: 0 clean, 1 with findings, quiet mode
silent, and check selection narrowing the result.

The generated scale profile runs a full lint at each tier, records per-check
timings, and asserts the digest does not change with the detail cap. Those
timings are what showed six separate scans of `document_links` dominating the
cost, which the single shared scan removed. Evidence: `performance/v0.5-e2/`.

### Workspace fix (v0.5 E3)

Store fixtures assert that planning changes nothing, that the plan carries the
exact `before`/`after` and the revision it was computed from, that applying
writes an ordinary revision and leaves the link pointing where it always did,
and that a note edited since the plan fails with a conflict while its content
survives untouched. Further fixtures cover the missing precondition, alt text
being opt-in and derived from the resource filename, wikilinks and unresolved
links being left alone, Help and trashed notes being skipped, request
validation, and the span-refusal rules — moved text, spans past the end,
inverted, negative.

A CLI fixture builds the real binary and asserts that dry run is the default and
reports no applied results, that `--apply` repairs and reports per note, that a
second run finds nothing because the repair is idempotent, that the repaired
note keeps its blocks, and that an unknown kind is refused.

One fixture records real Markdown behaviour again: `[x](Kitchen Plan)` truncates
at the space, so title-resolved Markdown links are the space-free ones — the
same rule that decided E1a's slug form.

### Sync model and transport tests (planned v0.7)

- Property/model tests shuffle, duplicate, replay, drop, and eventually deliver
  operations across at least three replicas and assert convergence.
- Crash injection covers canonical transaction, blob/chunk, envelope, manifest,
  acknowledgement, and retention boundaries.
- REST and folder/rclone adapters replay identical golden protocol transcripts.
- Test clock skew, cloned replica IDs, schema/protocol/database mismatch,
  missing/corrupt/truncated objects, offline-horizon full resync, peer
  retirement, delete/restore/purge, concurrent body edits, and notebook cycles.
- Mobile profiles measure maximum envelope/pending/object sizes and foreground
  responsiveness on a real Android device before release.

### Security tests

- SSRF-blocking for remote media.
- Domain stop-list matching.
- Oversized download rejection.
- MIME sniffing mismatch.
- HTML sanitization for Markdown preview.
- MCP result-size limits.

## Current validation commands

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
```

Frontend validation after dependencies are installed:

```bash
cd web
npm install
npm run typecheck
npm run build
```


## MVP release validation

Task 10 adds release-candidate checks beyond ordinary unit tests:

```bash
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
bash scripts/package_release.sh /tmp/notrios-v0.1.0-mvp.zip
python3 scripts/check_release_zip.py /tmp/notrios-v0.1.0-mvp.zip
```

The generated-dataset smoke test exercises document creation, FTS5 search, link graph resolution, graph slices, resource creation, and resource attachment on a synthetic dataset. The benchmark is intentionally small enough to run on developer machines; it is not a replacement for future hundreds-of-thousands-of-notes performance testing.
