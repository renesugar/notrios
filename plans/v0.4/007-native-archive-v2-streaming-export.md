# v0.4 P3 — Native archive v2 streaming export

Status: complete on 2026-08-04.

Model: Claude Opus 5 (Claude Code).

## Scope

P3 implements the manifest-last writer for the P2 format: one transactionally
consistent snapshot of the canonical store, selected by the shared P1 planner,
streamed into immutable SHA-256 objects and committed by publishing
`manifest.json` last.

P3 does not restore or import archives (P4), add stable `notrios://` routing
(P5), touch `movenotes-v3` (P6), or implement publication profiles (P7).
`publication_handoff` exports are refused with an explicit message because that
target's `plain_text`/`redact` link policy rewrites note bodies, which is a
publication projection rather than a restore-fidelity archive.

## Implementation

### Store export surface

- `internal/store/export.go` defines `ExportReader`, `SelectionResolution`,
  `SourceBundleKey`, and the canonical export row types.
  `internal/store/sqlite_export.go` implements them.
- The interface is deliberately **not** part of `Store`. It exposes complete
  selected identity sets, raw source-bundle keys, and blob content streams —
  none of which may be reachable from a REST or MCP adapter.
- `WithReadSnapshot` wraps the whole export in one deferred SQLite read
  transaction, which is what the manifest's
  `consistency: "sqlite_read_transaction"` claims. The caller must own the store
  handle exclusively; the CLI opens its own.
- Every read is identity-scoped and bounded to 400-ID batches (100 for
  four-column source-bundle keys). Revisions and links are streamed through a
  visitor so note bodies are never buffered a batch at a time.
- `ResolveSelection` reuses `planSelectionLocked`, so a dry-run `PlanSelection`
  and an export of the same request produce the same `manifest_sha256`. The
  planner keeps its capped REST/MCP detail arrays; only the in-process
  resolution carries the complete document/resource/bundle identity sets.

### Writer

- `internal/archivev2/export.go` owns object publication, record chunking,
  manifest construction, pruning, and post-publication verification.
- Objects are content-addressed, so identical bodies (or any repeated bytes)
  are stored exactly once and reported as `deduplicated_objects`.
- Bodies, resource blobs, and source-bundle bytes stream through a 64 KiB
  buffer into a staged temporary file that is fsynced and renamed into its final
  `objects/sha256/<ab>/<hash>` path.
- Records are LF-terminated typed JSONL chunks bounded by both
  `RecordsPerObject` and `MaxRecordObjectBytes`; one over-long record fails with
  an explicit size error rather than producing an unreadable object.
- Record emission order is documents → revisions → tag memberships → resource
  relations → links → provenance → resources → source bundles → notebooks →
  search notebooks → tags → collections. Container records come last so the run
  can emit exactly the notebooks, tags, and collections the archived data
  actually references, in one pass with no pre-scan.

### Privacy and scope decisions applied while encoding

- `full_archive` (default, no selector, Trash included) is the only mode
  reported as `full_backup`. It keeps complete revision history, trashed notes,
  provenance including private `metadata_json`, exact source bundles, every
  notebook and tag, and the builtin search notebooks.
- `subset_transfer` requires a selector, replaces private source
  `metadata_json` with `{}` while keeping provenance identity, exports only
  notebooks reachable from selected notes plus their ancestors and only tags
  those notes use, and omits query-backed search notebooks because their
  queries describe the whole library.
- A link whose target document or resource fell outside the selection is
  encoded with the target cleared and `resolution_status: target_excluded`, and
  counted in the report. No record ever names an object the archive omits.
- Raw source and item keys are hashed; source-bundle paths are validated
  bounded relative paths. No local filesystem path enters the archive.

### Interruption, resume, and commit

- Temporary object and manifest bytes live in a sibling
  `<destination>.staging` directory, so the archive directory never holds a
  temporary file. The staging directory is removed when the command finishes.
- The destination must contain only `objects/` and `manifest.json`; a foreign
  directory is refused. An existing manifest requires `--overwrite`, and is
  removed *before* objects are rewritten so a partially rewritten archive
  cannot claim completeness.
- An interrupted export leaves no manifest, so `VerifyDirectory` reports
  `incomplete archive`. Re-running reuses every already-published object of the
  right size, prunes objects the new manifest does not list, republishes the
  manifest, and fsyncs the archive directory.
- Unless `--no-verify` is passed, the writer runs the full read-only P2
  verification after publishing. If it fails, the manifest is removed so the
  archive is never left looking complete.

## Evidence

`scripts/run_archive_export_profile.sh` drives generated 100/1,000/5,000-note
tiers. Aggregate-only evidence is under `performance/v0.4-p3/`:

| Tier | Objects | Archive bytes | Export | Verify | Resume | Subset | Peak RSS |
|---|---|---|---|---|---|---|---|
| 100 | 102 | 160 KB | 0.083 s | 0.103 s | 0.077 s | 0.015 s | 16 MiB |
| 1,000 | 1,011 | 1.6 MB | 0.643 s | 1.333 s | 0.783 s | 0.130 s | 24 MiB |
| 5,000 | 5,052 | 7.9 MB | 3.210 s | 5.130 s | 2.956 s | 0.461 s | 46 MiB |

Every tier rewrote zero bytes on resume and reported stable record counts
across runs. Focused fixtures cover full/subset semantics, byte-identical
repeat manifests, plan-digest binding, incompleteness after interruption,
resume plus stale-object pruning, destination refusals, refused publication and
content-rewriting modes, the object budget, and small record chunking.

## Recorded format limitation

Each revision, resource, and source bundle is one object, and the manifest
lists every object inline at roughly 645 bytes per descriptor. The 4 MiB
manifest bound therefore binds before the nominal 10,000-object limit and caps
an archive near 6,500 objects — about 6,400 single-revision notes. Archive v2
cannot archive the supplied 382,206-note corpora at all.

This slice deliberately did **not** widen the limits. Raising `MaxObjects`
alone fails because the inline inventory would exceed the manifest bound; the
fix is a format revision that moves the object inventory into its own
checksummed object, which needs review. The exporter enforces the documented
bounds and fails with an explicit object-budget error before publishing any
manifest, so an over-budget archive can never appear complete.

Recorded as `agent/OPEN_QUESTIONS.md` question 17 and in
`NATIVE_ARCHIVE_V2.md`, and resolved on 2026-08-04 as plan task **P3a**, which
is sequenced ahead of P4.

## Validation

- focused archive-v2 export/verify/determinism/interruption/refusal fixtures;
- generated 100/1,000/5,000-note export, verify, resume, and subset profiles;
- `go vet ./...` and `go test ./...`;
- required-file and scaffold checks;
- web typecheck/tests/build and the docs/Help site build;
- MVP and generated performance smoke checks;
- a real CLI export of an imported Obsidian vault, full and subset;
- verified release ZIP copied to `/home/renes/evidence/notrios`.

## Next task

P3a, the archive-v2 large-library container revision, requires explicit user
approval. It is sequenced ahead of P4 because a restore path built against the
current in-memory verifier would have to be rewritten, and ahead of P6 because
that task pins the container for an external consumer.
