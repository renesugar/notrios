# v0.7 G8 — resource metadata, lazy materialization, chunks, and integrity

Status: **complete**, 2026-08-13. Product remains 0.6.0; the canonical schema is
now **v23**. Implemented by Claude Opus 5 (`claude-opus-5`) under Claude Code,
after explicit user approval naming G8.

## Goal

Converge attachment references immediately while downloading bytes only when
policy or user action needs them.

## What landed

**A blob row may exist without a file.** `blobs.availability` is `local` or
`unavailable`, and a trigger refuses — in both directions — any row whose
availability contradicts whether it has a storage path. That is what lets a
synchronized note reference an attachment this replica has not downloaded,
without the placeholder bytes the boundary forbids. The rule is a trigger rather
than a CHECK because SQLite cannot add a CHECK to an existing table, and a rule
that applied only to databases created after v23 would be the one least likely
to hold.

**`internal/syncassets`** owns the transfer shape and the policy, transport- and
storage-neutral like the G5, G6, and G7 cores: G2's whole-below-1-MiB and 1-MiB
fixed chunks, the 16,384-chunk and 16 GiB ceilings, manifests with per-chunk
hashes and a content-addressed digest, and the eager/pinned/lazy decision.

**Schema v23** adds `sync_blob_manifests`, `sync_blob_chunks` with fetch
progress, `sync_blob_sources`, and `sync_blob_materialization`. The resource
capture trigger now names the blob's length, content type, chunk count, and
manifest digest alongside the hash it already carried.

**Materialization** fetches through an `ObjectProvider` that G11 and G14 will
implement, stages verified chunks under `<asset root>/staging/`, resumes from
what it holds, and installs through the ordinary blob write path so the object
is hashed, sniffed, and placed by exactly the code an upload uses.

**Pin, request, and availability** are the user-facing surface:
`PinResource`, `RequestResource`, and `ResourceAvailabilityFor`, plus
`ErrResourceUnavailable` on the read path so a UI can say "not downloaded yet"
instead of showing a missing file.

## Decisions taken, and why

**The eager threshold is the chunking threshold.** The resolved decision said
"the G2 report supplies the threshold", and G2 supplies one: 1 MiB. Reusing it
means there is a single number to explain, and it is already the number that
decides how an object travels.

**The manifest is verified against a digest the operation carried.** A source
that supplied its own chunk hashes would be choosing what it is later checked
against. The digest travels in the bounded payload; the manifest is fetched
separately and refused if it does not match, before a single chunk is requested.

**MIME refusal is narrow on purpose.** `http.DetectContentType` is inconclusive
for most formats, so demanding equality would reject ordinary attachments rather
than hostile ones. A PDF advertised as a PNG is refused; an OpenDocument file
that sniffs as opaque bytes is not.

**An export now refuses rather than omitting.** A library holding unmaterialized
objects cannot produce a faithful archive-v2 container, so `OpenBlobContent`
returns `ErrResourceUnavailable` naming the object instead of `ErrNotFound`.
This is new behavior introduced by G8 and is the honest reading of "full
snapshot".

**Collection takes the transfer state with it.** Removing an unmaterialized blob
also removes its manifest, chunks, sources, materialization row, and staged
files, so a later admission of the same bytes does not believe it has already
fetched them. Its empty storage path no longer produces an "unsafe storage path"
warning, because an object nobody downloaded is an ordinary state.

## Two defects found during implementation

1. **A test fixture corrupted every first chunk.** `tamperingProvider` used a
   bare `corruptChunk int`, whose zero value is a real ordinal, so three resume
   tests failed for a reason that had nothing to do with resume. The field now
   has its own flag. It is recorded here because the failure looked exactly like
   a resume defect and was not one.
2. **Installing an object read it a second time.** `upsertLocalBlobLocked`
   rebuilt the manifest from the file it had just assembled from chunks whose
   hashes were already verified. Passing the manifest through removed a full
   pass over the data and halved the 16 MiB row: 16.6 s to 8.0 s.

## Validation

Feature evidence is `performance/v0.7-g8/` (`materialization-results.json`,
`README.md`, `FINDINGS.md`, `validate_evidence.py`), produced by real replicas
running the production store.

- Four object sizes around G2's thresholds. Admission time is flat across a 256×
  size range (79–106 ms), the note is readable before any attachment byte moves
  in every case, the policy split lands exactly on 1 MiB in both directions,
  every object reconstructs byte for byte, and an interrupted transfer refetches
  only the segments it did not already hold.
- `internal/syncassets`: chunk plans that tile exactly at every boundary
  including empty, one byte, the threshold, one byte above, an exact multiple, a
  ragged tail, and the 16 GiB ceiling; manifest build, validate, and per-chunk
  verify; seven manifest-invalidity mutations; digest binding of every field and
  independence from enumeration order; derived whole-object manifests; the
  policy table including pin-beats-budget; and the MIME rule in both directions.
- `internal/store`: metadata arriving before bytes with no placeholder anywhere;
  materialization of whole, chunked, exact-multiple, and empty objects; four
  hostile sources each with its recorded reason; MIME mismatch refused while an
  unrecognizable honest attachment transfers; resume after interruption; a
  damaged staged chunk refetched; policy and explicit request; the lazy policy;
  shared bytes not fetched twice; restart; bounded passes; provider-only access;
  the v23 backfill; the structural blob guard in both directions; and export and
  collection behavior for unavailable objects.

Repository validation, audit first as `AGENTS.md` requires:

```text
cd web && npm audit                                  0 vulnerabilities
cd web && npm ci && npm audit                        0 vulnerabilities
cd web && npm run typecheck                          pass
cd web && npm test -- --run                          15 files, 155 tests
cd web && npm run build                              pass
bash scripts/run_offline_assets_check.sh             no third-party request
go vet ./...                                         pass
go test ./...                                        pass
python3 scripts/check_required_files.py              70 files
python3 scripts/check_plan_loops.py                  pass
bash scripts/validate-scaffold.sh                    pass
bash scripts/build_docs_site.sh                      15 pages
bash scripts/mvp_smoke.sh                            pass
bash scripts/run_performance_smoke.sh                pass
python3 performance/v0.7-g8/validate_evidence.py     4 sizes
python3 performance/v0.7-g7/validate_evidence.py     4 intervals
git diff --check                                     clean
scripts/package_release.sh + check_release_zip.py    verified
```

## Out of scope, and still owned elsewhere

No carrier or wire codec, no cryptography or signatures (G9), no snapshot
catch-up (G10), no transport (G11–G14), and no REST, MCP, or UI surface for
availability, pinning, or fetching (G15/G16). Remote-media policy is unchanged:
a URL is still not a synced resource until ordinary quarantine admits it, and
materialization talks only to the provider it is handed — asserted by test.
Resource deltas were considered and not implemented: G1a's benefit case rests on
a named immutable parent, and Notrios resources have no such relationship, so
whole and chunked transfer are what G8 ships. Retention and tombstone collection
remain G17's. G9 remains unapproved.
