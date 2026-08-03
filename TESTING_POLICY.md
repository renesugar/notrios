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

H8's executable harness is `scripts/run_joplin_import_profile.sh`. It accepts
100, 10k, or 100k generated Joplin notes with nested folders, tags, and
resources. Every tier runs the same bounded dry-run planner; the 100 and 10k
tiers interrupt a real import at a durable note-batch boundary and resume it.
The 100k tier profiles the complete inventory/dry-run path without pretending
the current per-document canonical write API is a hardware-independent bulk
throughput gate. Reports record elapsed time, batch count, environment, and Go
memory. Exact unknown/reordered-property and CRLF-byte preservation is covered
separately by the focused importer fixture.

v0.4 J2 adds `scripts/run_real_joplin_profile.sh` for read-only real-export
profiles over the recipe Joplin/Obsidian pair and the attachment-bearing Joplin
archive. Committed evidence under `performance/v0.4-j2/` contains
only aggregate counts, timings, sizes, warnings, and redacted environment
facts—never note titles, bodies, source paths, resources, or databases. J2
measures the production dry-run relationship planner against actual links,
including peak RSS and a no-source-write check. J3 measures complete
transactional import, interruption/resume, and no-op re-import at the
million-note tier.

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
