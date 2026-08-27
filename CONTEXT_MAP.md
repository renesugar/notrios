# Context Map

This file is the codebase atlas. Update it whenever major files or directories are added.

## Root documents

- `README.md` — project overview and quick start.
- `PLAN.md` — the **active v0.7 native synchronization plan**, divided into
  independently approvable G0-G20 slices plus the blocking G14a-G14e archive-
  scalability sequence, the blocking G17a-G17b external evidence-preservation
  sequence, and the newly planned G18a-G18g documentation-integrity/Hugo-Ledger
  sequence. G0-G18 are complete; the G17b verifier gates every future GitHub
  push. G18a-G18b are complete; G18c is next but unapproved. The
  current product remains 0.6.0 at schema v27; v0.6 is archived under
  `plans/v0.6/`; v0.5 and v0.4 are archived under their version directories.
- `plans/scaffold/SCAFFOLD_CREATION_PLAN.md` — process for creating/refining this scaffold.
- `ROADMAP.md` — product roadmap and future features.
- `AGENTS.md` — coding-agent instructions (`CLAUDE.md` points here).
- `CODING_CLIENT_HANDOFF.md` — compressed project state for any coding agent (formerly `CODEX_HANDOFF.md`).
- `SYSTEM_ARCHITECTURE.md` — architectural blueprint.
- `API_SPEC.md` — REST/MCP contract notes.
- `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md` — notebooks, tags, search notebooks, Trash/Help semantics.
- `SEARCH_QUERY_LANGUAGE.md` — user query language and backend translation.
- `RECOLL_INTEGRATION.md` — Recoll sidecar design and licensing boundary.
- `SYNCHRONIZATION.md` — planned v0.7 database/replica identities,
  state-vector/change-log and revision-merge rules, lazy resources,
  archive/envelope security, ephemeral-directory and REST transports, snapshot
  catch-up, retention, library decision, and validation.
- `internal/syncjobs/` — G15's closed durable sync-outbox worker and
  directory/REST carrier target adapter; explicit work only, no cadence.
- `internal/httpapi/sync_ui.go`, `internal/store/sync_ui.go`, and
  `web/src/components/SyncCenter.tsx` — G16's loopback/native human facade:
  redacted aggregate state, setup/discovery/pairing and separate snapshot
  permission, conflict comparison/two-parent resolution, lazy resource intent,
  repair visibility, catch-up/reset preparation, and responsive UI.
- `internal/syncbackup/portable.go` — NPB1 password wrapper around the verified
  physical NBK1 payload; Argon2id wraps a fresh payload key and inspection is
  authenticated, bounded, private-staging-only, and non-installing.
- `cmd/notrios/gui_wails.go` — native-only directory chooser binding for the
  local service mode; GUI-only remote mode deliberately receives no path picker.
- `cmd/notriosctl/sync_ui_e2e_test.go` and `performance/v0.7-g16/` — G16's real
  two-daemon acceptance flow and content-free desktop/mobile browser evidence.
- `internal/store/sync_retention.go`, migration `0027_sync_retention.sql`, and
  `performance/v0.7-g17/` — G17's signed peer retirement, snapshot/acknowledgement
  floors, death/tombstone compaction, resource-GC gate, repair plan, safety
  matrix, and generated full-corpus-scale retention-cost evidence.
- `EVIDENCE_PRESERVATION.md` — G17a-G17b's human-readable canonical-manifest,
  OpenPGP, RFC 3161 checkpoint, deterministic ISO, offline-verification, outer-
  catalog, and custody contract plus issued-volume summary.
- `evidence/` — G17b's public key/TSA trust material, artifact signatures,
  canonical manifest and checkpoint, final and superseded outer catalog seals,
  JSON schemas, offline verifier, 14-case refusal harness/results, custody
  template, and future burn/read-back runbook. No private key or passphrase is
  stored here.
- `scripts/g17b_evidence.py` and `scripts/verify_evidence_pre_push.sh` — the
  production assembly/sealing tool and mandatory host-side tracked/source/
  reserve/ancestry verification gate.
- `performance/v0.7-g17a/` — G17a's privacy-safe aggregate inventory,
  provenance and recursive-scope findings, TSA assessment, generated-only
  canonical/signature/timestamp/ISO prototype, tests, and evidence validator.
- `performance/v0.7-g18/` — G18's 109-operation service/HTTP audit, finite
  19-capability platform matrix, ABI-major-1 lifecycle/ownership/error/cancel/
  stream contract, Android SQLite/cgo and modernc/C comparison evidence, API 35
  x86_64 emulator runtime/reboot/shared-library evidence, exact Mermaid
  enablement gate, unit tests, and source validator.
- `performance/v0.7-g18a/` — G18a's frozen 15-page/199-section claim-surface
  inventory, declaration-anchor rules, compatibility directive grammar,
  labelled contradiction calibration set, Go probe, and evidence validator.
- `performance/v0.7-g18b/` — G18b's pinned investigation-only Hugo/Ledger
  prototype, route/anchor/Pagefind and request contract, three-option delivery
  comparison, license/provenance records, build/validator, unit tests, and
  executable browser smoke. Production site generation remains unchanged.
- `performance/v0.7-g0/` — completed G0 threat model, normative protocol
  glossary, thirty misuse/control traces, primary-source dependency/license/
  platform validation, and its structural evidence validator.
- `performance/v0.7-g1/` — completed G1 aggregate-only corpus profiles,
  deterministic delta/merge/hostile-patch workload, exact reconstruction and
  CPU/RSS evidence, superseded upstream merge-candidate probe, findings, and
  validators and the now-superseded external merge-package probe.
- `performance/v0.7-g1a/` — completed pure-Go Subversion-style matcher and
  constrained VCDIFF/private comparison prototypes, exact text/binary
  benchmarks, hostile/golden/fuzz vectors, external interoperability evidence,
  provenance, findings, and structural validator. It is investigation code,
  not a production sync package.
- `performance/v0.7-g2/` — completed aggregate-only envelope/codec/resource
  bounds harness and evidence, NCB1 investigation specification, hostile-limit
  probes, fixed-chunk/pack recommendations, and separate emulator/physical
  mobile checklists. It is investigation code, not a production sync package.
- `performance/v0.7-g4/` — reproducible 100k import A/B for the schema-v19
  local journal, including exact sequence count, elapsed throughput, and
  database-byte overhead.
- `performance/v0.7-g5/` — G5 compatibility/bounds table and reproducible
  three-replica model plus durable SQLite convergence validation map.
- `internal/syncstate/` — transport/storage-neutral protocol-1.0 handshake,
  bounded state-vector comparison, deterministic missing-range planner, strict
  normalized operation model, and randomized three-replica property tests.
- `internal/synccatchup/` and `performance/v0.7-g10/` — schema-v24 snapshot
  catch-up: signed requests, explicit source permission, competing-offer
  selection, the durable reset state machine, catch-up floors, and password
  wrapping.
- `internal/synccarrier/` and `performance/v0.7-g11/` — the ephemeral
  shared-directory carrier: the transport-neutral `Carrier` surface, the blinded
  folder layout, the exchange `Round`, discovery that reports without enrolling,
  and the carrier-backed `ObjectProvider`. No schema change.
- `performance/v0.7-g12/` — the conformance run that puts that carrier on a real
  provider: measured visibility delay, eight protocol phases through a mounted
  Google Drive folder and a drive passed between peers, and the enforced
  refusal of every destructive rclone verb. No production code.
- `internal/syncauth/` and `performance/v0.7-g13/` — the peer principal: signed
  requests rather than bearer tokens, the replay cache and rate limiters, the
  short-lived single-use pairing code and its proofs, the signing client, and
  the generated authentication/authorization matrix. Schema v25 holds enrolled
  peer public keys and pairing invitations.
- `internal/httpapi/sync.go` — the three peer routes and the middleware that
  guards them; a peer credential reaches this surface and no ordinary route.
- `internal/syncrest/`, `internal/syncbackup/`, `internal/httpapi/sync_data.go`,
  and `performance/v0.7-g14/` — the REST data plane: G11's `Carrier` implemented
  over the signing client so both transports run one protocol, the carrier
  routes with `Range`, and the original resumable encrypted archive-v2 snapshot
  baseline. Its loose-object ZIP overhead triggered G14a-G14e; G14d has replaced
  that production backup payload and wrapper, while the historical evidence
  remains the baseline.
- `performance/v0.7-g14a/` and
  `scripts/run_archive_scalability_benchmark.sh` — the generated 10k/100k,
  aggregate-only calibration contract and resumable 11-adapter harness.
- `performance/v0.7-g14b/` and
  `scripts/run_archive_full_corpus_benchmark.sh` — the completed private-
  corpus investigation harness, 57 sanitized phase rows, option-B findings,
  and selected SQLite-image capability contract. Private inputs, detailed
  phase rows, repositories, paths, hashes, and logs remain external.
- `performance/v0.7-g14c/` and `performance/v0.7-g14d/` — production physical
  snapshot creation/admission state review, generated 100k create/restore
  evidence, recovery state machine, and Android-emulator follow-up checklist.
- `performance/v0.7-g14e/` and
  `scripts/run_archive_production_acceptance.sh` — the 19-phase resumable
  production full-scale matrix, privacy-sanitized format freeze, current/
  previous semantic reader coverage, physical first/unchanged/restore evidence,
  REST/directory catch-up, and frozen Restic/Borg integrity checks. Private
  corpora, artifacts, fingerprints, repositories, paths, and logs remain under
  the external evidence workspace.
- `internal/synckeys/` — the warned `0600` development secret provider holding
  one group key per epoch and this replica's signing key. Since G13 it holds
  secrets only: which peers are trusted is database state. v0.8 owns the
  platform store.
- `cmd/notriosctl/sync.go` — `notriosctl sync init|bundle|pair|status|discover|once`,
  the only way to run an exchange; every round is explicit, and there is no
  watcher or scheduler (G15).
- `internal/syncwire/` and `performance/v0.7-g9/` — the canonical NCB1/NEV1
  encoding, deterministic gzip, and the encrypted, signed NAR1 artifact every
  transport carries. No schema change and no dependency.
- `internal/syncassets/` and `performance/v0.7-g8/` — schema-v23 attachment
  metadata that converges before its bytes, chunk manifests, bounded verified
  materialization, and eager/pinned/lazy policy.
- `internal/syncbody/`, `internal/syncdelta/`, and `performance/v0.7-g7/` —
  schema-v22 note revision objects, bounded named-base transfer deltas, the
  line-first three-way merge with word-region refinement, and durable typed
  body conflicts.
- `internal/syncmerge/` and `performance/v0.7-g6/` — schema-v21 deterministic
  HLC/field/membership/lifecycle/tree convergence core and its exhaustive,
  randomized, and SQLite evidence map.
- `internal/profiles/runtime.go` — G3 named runtime-profile creation, redacted
  views, registry/config/database startup binding, path/port/replica collision
  detection, and explicit copied-database adopt/fork handling.
- `plans/v0.7/` — archived completed v0.7 slices through G18 plus the planning
  amendments that inserted G1a, the G14a-G14e scalability sequence, the
  G17a-G17b evidence-preservation/ISO sequence, the G18a-G18g documentation
  sequence, and the bundled-dependency maintenance pass.
- `plans/v0.7/027-evidence-preservation-iso-plan-amendment.md` — the planning
  record that corrects the hash/signature/RFC3161/custody claims, records the
  aggregate external inventory, and inserts blocking G17a-G17b before G18 and
  any GitHub push. It authorizes no key, network, ISO, push, or burn operation.
- `plans/v0.7/028-evidence-preservation-contract.md` — G17a's completed
  investigation record, including the selected signed-checkpoint topology,
  measured curated scope/capacity, rejected recursive-private scope, prototype
  results, and the three design questions later resolved by archive 029.
- `plans/v0.7/029-evidence-handling-decisions.md` — the post-G17a resolution of
  curated scope, exact OpenPGP fingerprints, DigiCert/Sectigo order, supplied-
  tool reference accuracy, and the later-satisfied operational approval gate.
- `plans/v0.7/030-evidence-seals-iso-reserve.md` — G17b's completed curated
  backfill, exact-key signatures, DigiCert RFC 3161 evidence, deterministic
  immutable `NTR-EV-0001`, finite outer-catalog closure, refusal matrix, and
  validation record.
- `plans/v0.7/031-shared-core-ffi-portability-handoff.md` — G18's completed
  application-facade audit and pre-1.0 C-ABI/platform/Mermaid handoff to v0.8
  and the independent post-1.0 Flutter client.
- `plans/v0.7/033-modernc-sqlite-evaluation.md` — G18's follow-up evaluation of
  modernc/cznic for mobile, desktop, and Web and the two-candidate v0.8 H0 gate.
- `plans/v0.7/034-android-emulator-modernc-runtime.md` — G18's API 35 x86_64
  emulator runtime amendment for the disposable modernc candidate and its
  explicit remaining H0 boundaries.
- `FLUTTER_GO_CLIENT.md` — verified Flutter/Dart FFI and Go build-mode facts,
  the pre-1.0 framework-neutral application facade/C ABI contract, post-1.0
  Flutter client split, memory/stream ownership, platform limits, and the fact
  that current-GUI Mermaid support is disabled pending v0.8 evidence.
- `NATIVE_ARCHIVE_V2.md` — v2 identity, manifest-last object/record contract,
  compatibility/limits, explicit restore intent, verification rules, P3 export
  staging/resume semantics, the P3a index-chunk container, the optional P3b
  packed layout, and the P4 restore contract.
- `DOCS_SITE.md` — current Marked/Pagefind GitHub Pages site and Help notebook,
  plus the approved-plan boundary for G18a-G18g source anchors, executable
  examples/GUI journeys, generated freshness, advisory prose review, and a
  pinned Hugo/Ledger migration.
- `CODING_STANDARDS.md` — coding style and guardrails.
- `TESTING_POLICY.md` — definition of done and testing layers.
- `ENVIRONMENT_SETUP.md` — development setup.
- `PROMPT.md` — initial agent prompt.

## Code directories

- `cmd/notriosd/` — service daemon entry point.
- `cmd/notriosctl/` — CLI/admin/import command entry point.
- `internal/importers/joplinraw/` — hardened Joplin RAW importer: canonical
  first-line titles, CR/LF-only physical parsing (including OCR controls), one
  ordered future/duplicate-property parse, deterministic inventory, nested
  notebooks, real tags, exact optional source bundles, fingerprints, bounded
  batches, checkpoints/resume, and dry-run/config planning.
- `internal/importers/obsidian/` — hardened deterministic vault importer with
  nested notebooks, exact optional source bundles, bounded checkpoints/resume,
  dry-run/config planning, and canonical alias/embed/anchor resolution.
- `internal/api/` — shared API request/response models.
- `internal/httpapi/` — REST HTTP adapter for status, documents, revisions, resources, links, graph slices, and staged future routes.
- `internal/store/` — SQLite-backed persistence, schema-v19 local replication
  journal, schema-v20 bounded admission, schema-v21 deterministic metadata, schema-v22 revision-object body convergence, and schema-v23 lazy attachment materialization
  application (`sync_journal.go`, `sync_admission.go`, `sync_metadata.go`, and
  migrations `0019`-`0021`), document
  CRUD, revision history, soft delete, restore, FTS5 search, resource
  storage/reference reports (`sqlite_resource_reports.go`), retention-aware GC
  (`sqlite_gc.go`), importer batch/checkpoint/source-bundle state
  (`sqlite_imports.go`), archive export/restore, logical database/replica
  identity, note blocks, lint/fix, graph traversal/reports, editor links,
  query blocks, organizer operations, and notebooks/tags/search-notebooks/Trash.
- `internal/archivev2/` — native archive-v2 manifest/record model
  (`format.go`), out-of-manifest object index and fanout layout (`index.go`),
  identity-intent planner, bounded streaming verifier (`verify.go`), bounded
  external-sort spool and merge join (`spool.go`), the optional packed object
  layout (`pack.go`), the manifest-last streaming exporter (`export.go`), the
  verify-first restore reader (`restore.go`), the publication projection
  (`publish.go`), a generator-built synthetic golden fixture, and generated
  scale profiles.
- `internal/snapshotimage/`, `internal/store/sqlite_snapshot.go`, and
  `internal/store/sync_snapshot_activate.go` — G14c/G14d's
  exact-schema SQLite Online Backup image, deterministic bounded external
  packs, strict manifest-last verifier, table-state sanitization, verified
  emergency backup, durable physical install, replica rotation/floors, derived
  rebuild selection, and generated 100k evidence.
- `internal/syncbackup/`, `internal/syncrest/backup.go`, and
  `internal/synccarrier/snapshot.go` — the sequential deterministic USTAR plus
  NBK1 physical snapshot transport and bounded resumable REST/directory byte
  paths. The physical verifier, not USTAR, is the trust boundary.
- `internal/markdownlinks/` — conservative MVP Markdown/Obsidian/app-URI link extractor.
- `internal/markdownblocks/` — deterministic block splitter behind schema-v14
  addressable blocks and schema-v15 heading slugs; block identity is
  content-derived and scoped to the document, and `Slugify` is the shared
  heading-anchor normalization.
- `internal/stablelink/` — strict parser/formatter for the external
  `notrios://databases/{database_id}/documents/{document_id}` link, with typed
  rejections (foreign scheme, malformed, over-limit, unsupported route).
- `internal/profiles/` — the explicit local registry mapping a logical database
  ID to a database on this machine plus G3 runtime-profile config/identity and
  process-isolation validation; stable links resolve only unambiguous matches
  and report every candidate otherwise.
- `internal/publish/` — saved publication profiles (selection and privacy
  decisions only, never an output path or a command) plus the reviewed-plan gate
  that publishing must satisfy.
- `internal/version/` — version constants.
- `migrations/` — SQLite schema migrations.
- `web/` — React/Vite built-in UI scaffold. `src/useLinkIntelligence.ts` and
  `src/components/LinkIntelligence.tsx` hold the v0.5 E5 link picker and buffer
  link check; both debounce, cancel superseded requests, and degrade to nothing
  when the service is unreachable. `src/editor-extensions.ts` and
  `src/editor-offsets.ts` add the E6 in-editor half — `[[` autocomplete, broken-
  link decorations, and Ctrl-click — through the CodeMirror 6 hooks
  `md-editor-rt` already exposes, plus the UTF-8-byte to UTF-16-index conversion
  the service boundary needs. `src/editor-assets.ts` supplies KaTeX,
  highlight.js, and cropper locally so nothing is fetched from a CDN (v0.5 E6a),
  and explicitly sets `noMermaid: true` until the planned offline/security-tested
  v0.8 enablement.
  `src/html-table-markdown.ts` converts a pasted HTML table into a Markdown pipe
  table and refuses anything a pipe table cannot hold (v0.5 E6b).
  `src/note-query.ts` renders embedded ```note-query blocks after the note has
  already rendered, writing every value as text (v0.5 E7). `src/organizer.ts`
  holds the notebook-deletion confirmation text, built from the service's own
  preview rather than a guess (v0.5 E8).

## Agent support

- `agent/PLAN_STATUS.md` — current task and working-state notes.
- `agent/ATTEMPT_LOG.jsonl` — append-only attempt history.
- `agent/MODEL_LOG.jsonl` — model/session tracking.
- `agent/LOOP_DETECTION.md` — stalled-task detection and resolution.
- `plans/` — archived completed plans by version/milestone.
- `skills/` — agent skills using the `SKILL.md` format.
- `prompts/` — reusable agent prompts.

## API and configuration

- `api/openapi.yaml` — initial REST OpenAPI skeleton.
- `config/config.example.yaml` — example service configuration and media policy.

## Scripts

- `scripts/check_required_files.py` — verifies required scaffold files exist.
- `scripts/validate-scaffold.sh` — runs current scaffold validation checks.
- `scripts/run_editor_profile.sh` + `scripts/measure_editor.mjs` — v0.5 E6
  editor first-paint and keystroke-latency profile, driving real headless Chrome
  over the DevTools Protocol with a client written against Node's built-in
  WebSocket (no new dependency).
- `scripts/run_offline_assets_check.sh` + `scripts/check_offline_assets.mjs` —
  v0.5 E6a guard: fails if the built-in UI issues any cross-origin request,
  injects a remote script or stylesheet, violates the CSP, or fails to render
  math with every CDN blocked.

## Added design docs

- `FEATURE_MATRIX.md` — milestone and ownership map.
- `UI_DESIGN.md` — web UI/editor decisions.
- `PUBLISHING_POLICY.md` — Quartz/static publishing rules.
- `SELECTION_AND_PRIVACY_PLANNER.md` — live P1 typed selection, target privacy
  defaults, manifest/report shapes, content boundary, and scale limits.
- `docs/stable-links.md` — user guide for `notrios://` links, the profile
  registry, `notriosctl link`/`open`/`profile`, and the desktop handler.
- `docs/publishing.md` — user guide for publication profiles, the
  review-then-publish gate, and what a publication withholds.
- `VERSIONING_AND_SYNC_POLICY.md` — revision/checkpoint policy and summary of
  the synchronization invariants.
- `WORKSPACE_MAINTENANCE.md` — query/lint/outline/block features.
- `plans/scaffold/SCAFFOLD_REVIEW_REPORT.md` — Step 2 repair summary.

## Step 3 additions

- `DATABASE_SCHEMA.md` explains the target SQLite schema and why search sidecars (now Recoll) remain derived indexes.
- `api/openapi.yaml` contains the REST contract for collections, documents,
  resources, revisions, links, remote media, graph, selection planning, and jobs.
  Remote-media scan/localization, selection/privacy dry runs, batches, and job
  watch/cancel are live; G3 local runtime profiles are CLI/config-only, while
  remote job starts, REST profile management, and sync remain staged.
- `api/mcp-tools.md` defines MCP tool scopes and the implemented bounded tools. `internal/httpapi/mcp.go` contains the current dependency-free adapter mounted at `/mcp`.
- `internal/api/types.go` mirrors the current REST DTO shapes.
- `internal/httpapi/server.go` plus its route-specific files expose live
  SQLite-backed document/search/selection, batch, and job watch/cancel routes;
  path-taking job starts remain deliberately absent.

## MVP Task 1 additions

- `internal/config/` — small dependency-free config loader for the documented YAML subset and directory bootstrap helper.
- `internal/store.StoreStatus` — database driver/path/state/schema-version reporting for status output.
- `plans/mvp/MVP_TASK1_REPORT.md` — service persistence foundation completion report.

## MVP Task 2 additions

- `internal/store` now exposes update, soft-delete, revision list/read, and revision restore operations with optimistic concurrency.
- `internal/httpapi` implements `PUT`/`PATCH`/`DELETE` document routes plus revision list/read/restore.
- `web/src/api.ts` and `web/src/App.tsx` can save a new revision for the currently opened note.
- `plans/mvp/MVP_TASK2_REPORT.md` records this task.

## MVP Task 4 additions

- `internal/store` now streams resource bytes into the configured asset store and deduplicates exact blobs by SHA-256.
- REST implements resource metadata/content and document-resource attach/list/detach routes.
- `web/src/App.tsx` can upload/list/download attached resources.
- H5 adds read-only exact-duplicate/unreferenced/per-notebook usage reports
  through Store, REST, and `notriosctl`, plus an optional
  `PerceptualHashHook`; no perceptual algorithm is installed by default.
- H6 adds schema-v8 unreferenced timestamps/reasons, retention config,
  `RetentionGate`, dry-run/apply GC in `notriosctl`, and a read-only REST GC
  report. Permanent REST deletion requires object-specific confirmation.
- `plans/mvp/MVP_TASK4_REPORT.md` records this task.

## MVP Task 5 additions

- `internal/markdownlinks` extracts common Markdown links/images, Obsidian wikilinks/embeds, app URIs, external URLs, heading anchors, and block anchors.
- `internal/store` rebuilds `document_links` rows transactionally on document create/update/restore and clears outgoing links on soft delete.
- `internal/httpapi` implements `GET /api/v1/documents/{document_id}/links` and `POST /api/v1/graph`.
- `web/src/api.ts` and `web/src/App.tsx` can list and display outgoing links/backlinks for the opened note.
- `plans/mvp/MVP_TASK5_REPORT.md` records this task.


## MVP Task 6 UI files

- `web/src/App.tsx`: REST-backed Markdown UI with `md-editor-rt`, preview normalization, app URI routing, resource upload, link/backlink/resource sidebars.
- `web/src/styles.css`: app layout plus editor, link-list, resource-list, and text-button styling.
- `internal/httpapi/server.go`: now serves `web/dist` for the built-in UI when the production build exists.
- `plans/mvp/MVP_TASK6_REPORT.md`: implementation notes and deliberate limitations for the UI MVP.

## MVP Task 8 importer files

- `internal/importers/joplinraw/joplinraw.go` owns RAW parsing/body/link helpers and the JSON report contract; `scalable.go` owns deterministic inventory, conflict planning, bounded phases, fingerprints, exact bundles, and resume.
- `internal/importers/joplinraw/joplinraw_test.go` covers hierarchy, real tags and renames, exact RAW bytes/property order, dry-run parity, resource refresh, conflicts, bounded batches, and resume; `profile_test.go` drives generated 100/10k/100k profiles.
- `cmd/notriosctl/main.go` now includes `notriosctl import joplin-raw`.
- Store create requests support optional preferred IDs so importers can create deterministic source-derived document/resource IDs.


## MVP Task 9 importer files

- `internal/importers/obsidian/obsidian.go` owns deterministic one-pass vault
  inventory, nested-folder conflict planning, bounded fingerprint/checkpoint
  phases, exact source bundles, canonical alias/relative/embed/anchor
  resolution, resource refresh, and link rebuild.
- `internal/importers/obsidian/obsidian_test.go` covers exact Markdown/binary
  recovery, hierarchy, conflicts/renames, dry-run parity, interruption/resume,
  resource refresh, richer graph edges, Trash, search, and idempotence;
  `profile_test.go` drives generated 100/10k/100k/500k tiers.
- `cmd/notriosctl/main.go` exposes Obsidian batch, source preservation,
  dry-run config, resume progress, and media-localization flags.
- `store.RebuildDocumentLinks` lets batch importers refresh link resolution after all target documents/resources exist without creating extra revisions.


## Release hardening files

- `PACKAGING.md` — release ZIP contents, exclusions, and package commands.
- `SECURITY_REVIEW.md` — v0.1 security review for resource downloads, preview sanitization, MCP, and importers.
- `RELEASE_CHECKLIST.md` — pre-tag checklist for `v0.1.0-mvp`.
- `plans/mvp/MVP_RELEASE_REPORT.md` — completed MVP capability summary and deferred features.
- `scripts/mvp_smoke.sh` — end-to-end local REST/MCP/resource smoke test.
- `scripts/run_performance_smoke.sh` — generated-dataset smoke and benchmark wrapper.
- `scripts/run_large_library_profile.sh` — reproducible H7
  10k/100k/500k keyset/search/resource profile driver.
- `scripts/run_joplin_import_profile.sh` — reproducible H8 generated
  100/10k/100k dry-run plus interrupted/resumed import profile.
- `scripts/run_obsidian_import_profile.sh` — reproducible H9 generated
  100/10k/100k/500k dry-run plus interrupted/resumed vault profile.
- `scripts/run_archive_export_profile.sh` — reproducible P3 generated
  100/1,000/5,000-note archive-v2 export, verify, resume, and subset profile.
- `performance/v0.4-p3/` — committed aggregate archive-v2 export timings,
  object/byte counts, peak RSS, and object-budget usage.
- `performance/v0.3-h7/` — committed environment, query-plan, latency, size,
  and peak-RSS evidence from the H7 scale runs.
- `performance/v0.3-h8/` — committed Joplin import duration, batch/resume, and
  memory evidence for the H8 synthetic profiles.
- `scripts/package_release.sh` — validates, builds UI, creates source ZIP, and verifies contents.
- `scripts/check_release_zip.py` — catches missing `web/dist`, accidental `web/node_modules`, and runtime data in ZIPs.

## v0.2 Notrios redesign additions (task R1)

- Git repository initialized: `main` = pre-redesign baseline, `develop` = active work.
- `CODEX_HANDOFF.md` renamed to `CODING_CLIENT_HANDOFF.md`; `CLAUDE.md` added pointing to `AGENTS.md`.
- `skills/codex-handoff/` renamed to `skills/agent-handoff/`; `prompts/start_codex_from_handoff.md` renamed to `prompts/start_agent_from_handoff.md`.
- New design docs: `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`, `SEARCH_QUERY_LANGUAGE.md`, `RECOLL_INTEGRATION.md`, `DOCS_SITE.md`.
- Living design docs rebranded to Notrios and switched from sist2 to Recoll; historical reports (`SCAFFOLD_STEP*`, `MVP_TASK*`, `plans/`) intentionally keep old names as records.

## v0.2 task R3 additions (schema v5)

- `migrations/0001_initial.sql` (and the embedded copy under `internal/store/migrations/`) now creates `notebooks`, `tags`, `note_tags`, and `search_notebooks`, adds `documents.notebook_id`, and sets `PRAGMA user_version = 5`; `ensureSchemaV5` upgrades v4 databases and bootstrap backfills existing notes into the default notebook.
- `internal/store/sqlite_notebooks.go` — notebook CRUD (nested, emoji, case-insensitive sibling-unique names, cycle-safe moves, recursive delete-to-trash), tag add/remove/list with live note counts, search-notebook lifecycle with builtin protection, and trash list/restore/purge.
- `internal/store/notebooks_test.go` — coverage for bootstrap builtins, naming rules, nesting, membership/move, recursive delete, trash/restore/purge, tag counts, sidebar ordering, and the v4→v5 upgrade path.

## v0.2 task R4 additions (schema v6)

- `migrations/0001_initial.sql` (both copies) adds `document_sources` (source system, external ID, author/author_id, thread_id/reply_to, source URL, published_at + derived published_ts) and sets `PRAGMA user_version = 6`.
- `internal/store/sqlite_sources.go` — `SetDocumentSource` upsert (works for trashed notes so importers can backfill), `GetDocumentSource`, `FindDocumentBySource` (importer idempotency), `ListThreadDocuments` (chronological thread recovery, trashed notes excluded); `PurgeDocument` now refuses externally-sourced notes.
- `internal/importers/joplinraw` and `internal/importers/obsidian` record provenance rows on every import run (re-running an import backfills existing notes).
- `internal/store/sources_test.go` plus importer-test assertions cover upsert/lookup, thread ordering, purge protection, and importer provenance.

## v0.2 task R6 additions (query language)

- `internal/query/` — bounded backend-neutral expression parser for implicit
  AND, uppercase OR, prefix negation, parentheses, phrases, `category:`/
  `notebook:`, tags, provenance/time fields, literal unknown-colon fallback,
  URLs/hyphens/emoji, and Trash scope.
- `internal/store/sqlite_query.go` — compiles positive text trees to FTS5 with
  relevance keysets and mixed/negated trees to exact parameterized SQL
  predicates with chronological keysets; recursive case-insensitive notebook
  expansion, All-notes alias semantics, emoji fallback, and canonical AST
  cursor binding are shared by every search surface.

## v0.2 task R7 additions (Recoll sidecar)

- `internal/projection/` — outbox-driven Markdown+front-matter filesystem
  projection with bounded draining, durable retry scheduling, exact
  missing/stale/orphan reconciliation, and atomic repair writes.
- `internal/recoll/` — external-process Recoll sidecar: generated config (fields prefixes, `publishedts` range slot, `underscoreasletter`), the embedded from-scratch `notrios_md_handler.py` front-matter handler, `recollindex`/`recollq` invocation, query compilation, and result parsing. GPL boundary: binaries are user-installed and never linked or vendored.
- `internal/httpapi/sidecar_search.go` — merges sidecar-only hits into search results behind the existing API; FTS5 stays authoritative and sidecar failures degrade gracefully.
- `internal/service/` — activates optional Recoll: startup reconciliation and
  index, 30-second bounded drain, 10-minute reconciliation, runtime status,
  cancellation, and stable deduplicated merged search.

## v0.2 task R9 additions (Twitter/X importer)

- `internal/importers/twitter/` — extracted-archive parser (`window.YTD` wrappers, tweets.js/tweet.js, account.js, tweets_media), in-reply-to thread recovery, t.co URL expansion, media-as-resources, hashtag tags, "Twitter" notebook, provenance rows, trashed-note non-resurrection, dry run.
- `cmd/notriosctl` — `import twitter` subcommand.
- `testdata/schemas/` — genson-derived JSON Schemas for the Twitter archive formats (synthetic samples only).

## v0.2 tasks R10/R11 additions (conversation importers)

- `internal/importers/chatgpt/` — ChatGPT `conversations.json` importer (mapping tree, current-node main path, system/tool skip).
- `internal/importers/claude/` — Claude `conversations.json` importer (flat chat_messages, content blocks).
- `cmd/notriosctl` — `import chatgpt` and `import claude` subcommands.
- `testdata/schemas/` — genson-derived schemas for both export formats.

## v0.2 task R12 additions (native archive)

- `internal/archive/` — query-scoped export (notebook paths + emoji preserved, tags, resource bytes), dry-run conflict analysis with rename-suggestion `import-config.json`, validated rename-on-import (refuses names colliding with source-bound notebooks before writing), idempotent plain-note import.
- `cmd/notriosctl` — `export archive` and `import archive` subcommands.

## v0.2 task R13 additions (Wails GUI shell)

- `internal/service/` — shared startup (directories, store, HTTP handler, Recoll sidecar loop) used by `notriosd` and `notrios`.
- `cmd/notrios/` — GUI executable: default (GUI + local service), `-no-gui`, `-gui-only -remote <url>`; `gui_wails.go` (build-tagged Wails app whose asset server routes all webview requests through the service handler or a remote reverse proxy) and `gui_stub.go` (helpful error without the tags). Build with `make gui`.
- `web/src/App.tsx` + `api.ts` + `styles.css` — Notrios sidebar layout: search notebooks first/last anchoring, nested notebook tree with emoji, tags with counts, startup "All notes" with cursor-based Load more, Help-menu event hook.

## v0.2 task R14 additions (GUI themes)

- `web/src/themes.ts` — theme token sets (builtin Light/Dark), custom-theme persistence, per-mode theme selection, applyTheme.
- `web/src/styles.css` — colors tokenized into CSS custom properties; theme-panel styles.
- `web/src/App.tsx` — header light/dark toggle and theme settings panel (create/edit/delete custom themes, assign per mode).

## v0.2 task R15 additions (docs + Help notebook)

- `docs/` — user documentation (published to GitHub Pages and seeded into the Help notebook).
- `scripts/build_docs_site.sh` — Markdown → HTML with a shared template plus a PageFind static search index.
- `.github/workflows/docs.yml` — GitHub Pages deployment for the docs site.
- `internal/helpdocs/` + `notriosctl seed-help` — deterministic Help-notebook seeding (create/update/remove).
- Help notes are read-only at the REST/MCP layers (`guardHelpNote`).

## Documentation completeness pass (2026-07-16)

- New user docs: `docs/installation.md` (build/install guide),
  `docs/troubleshooting.md`, and `docs/operations.md` (remote media,
  resources/GC, resumable imports, Recoll status); all pages are in the site
  nav and deterministic Help seed.
- Rewritten: `ENVIRONMENT_SETUP.md` (contributor guide incl. cleanup/precheck workflow), `PACKAGING.md`, `docs/cli.md` (full subcommand reference), `docs/service.md` (config reference + backup/restore), expanded `docs/import-export.md` (per-source workflows incl. the detailed Joplin RAW procedure) and API guides (curl examples, placeholder-endpoint labeling).
- `notriosctl doctor` performs real diagnostics; importers gained trashed-note re-import guards (Joplin/Obsidian); SQLite opens with a 5s busy timeout; Makefile has full build/clean/precheck targets.
