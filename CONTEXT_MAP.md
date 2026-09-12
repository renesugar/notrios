# Context Map

This file is the codebase atlas: where things live, and what each thing is for.

**How to keep this document current is in [`AGENTS.md`](AGENTS.md)** — under "Keeping the reference documents current".

It is organised by location rather than by when something arrived. It was
organised the other way until v0.8 H20 — twenty-two sections named after the
task that added them, the newest dated 2026-07-16 — and that is why eleven
packages went unrecorded: adding to it meant choosing a section by date, and the
right date section is always the one that does not exist yet. What arrived when
is in the archived plans under `plans/`, which is its home.

Two parts of this file are generated and one is checked:

- the root-document list below comes from `docs/docrules/DOCUMENTS.json`, the
  same registry that decides what each document is the home for, so the atlas
  and the inventory cannot disagree about which documents exist;
- `go test ./internal/docrules/` fails when a package under `internal/` or
  `cmd/` has no entry here. `CODING_STANDARDS.md` carried that as a sentence and
  nothing enforced it. The check found five more packages missing than a grep
  for their names did.

## Root documents

<!-- notrios:generated:atlas:documents:begin -->
Every root document, from `docs/docrules/DOCUMENTS.json`. What each is the home for, and the rules for
keeping it current, are in `AGENTS.md`.

- `AGENTS.md` — how a coding agent works here, including how every other document is kept current
- `CLAUDE.md` — one line telling Claude where the real instructions are
- `PLAN.md` — the active plan: what each item is for, and what happened
- `ROADMAP.md` — what a version means, as a feature inventory and planning source
- `README.md` — what Notrios is and what it can do today, for somebody who has not seen it
- `CODING_CLIENT_HANDOFF.md` — the compressed current state and the recommended next task, for an agent starting cold
- `CONTEXT_MAP.md` — where things live: the root documents, the packages, and what each is for
- `SYSTEM_ARCHITECTURE.md` — how the parts fit together and why they are separated that way
- `API_SPEC.md` — the REST and MCP contract in prose: what each surface is for and what it refuses
- `DATABASE_SCHEMA.md` — why the schema is shaped the way it is, table by table
- `UI_DESIGN.md` — what the built-in GUI is meant to be and how it is built
- `FEATURE_MATRIX.md` — triage: whether a feature is in scope, which milestone owns it, and how far it got
- `WORKSPACE_MAINTENANCE.md` — the networked-notes maintenance features and which of them Notrios took
- `DOCS_SITE.md` — how documentation is authored once and consumed twice, as a site and as the Help notebook
- `ENVIRONMENT_SETUP.md` — how a contributor gets a machine that can build, test and run this
- `PACKAGING.md` — what a release artifact contains, how it is produced, and how it is checked
- `RELEASE_CHECKLIST.md` — the gates a version must pass, kept per version
- `TESTING_POLICY.md` — the definition of done and what counts as evidence for it
- `CODING_STANDARDS.md` — how code is written here, and where this project departs from the Google style guides
- `PUBLISHING_POLICY.md` — what a publication may contain and what it must strip
- `VERSIONING_AND_SYNC_POLICY.md` — what is canonical, how revisions are kept, and what may never become the store of record
- `EVIDENCE_PRESERVATION.md` — the preservation contract: what is sealed, by whom, and how it is verified
- `PROJECT_DECISIONS.md` — decisions that have been accepted, and the reason each was accepted
- `PROMPT.md` — the cold-start prompt handed to an agent that has no history
- `SECURITY_REVIEW.md` — what the security posture was at a named release, and what was still open

Feature contracts, which describe one capability rather than the project:

- `FLUTTER_GO_CLIENT.md` — feature contract: the shared-core and client handoff
- `IMPORT_EXPORT_POLICY.md` — feature contract: import, export and publishing behaviour
- `NATIVE_ARCHIVE_V2.md` — feature contract: the archive-v2 format and identity rules
- `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md` — feature contract: the notebook and tag model
- `RECOLL_INTEGRATION.md` — feature contract: the derived Recoll sidecar
- `SEARCH_QUERY_LANGUAGE.md` — feature contract: the user-facing query language
- `SECURITY_AND_MEDIA_POLICY.md` — feature contract: remote-media localization
- `SELECTION_AND_PRIVACY_PLANNER.md` — feature contract: the selection and privacy planner
- `SYNCHRONIZATION.md` — feature contract: the synchronization architecture
<!-- notrios:generated:atlas:documents:end -->

## Commands

- `cmd/docaudit/`, `internal/docaudit/`, `scripts/docaudit_ts.mjs`, and
  `docs/docaudit/registry.json` — G18c's repository-only directive/parser,
  Go and TypeScript declaration resolver, typed claim/check registry,
  executable-fence and GUI-journey accounting, and four-grade topic report.
- `cmd/doccheck/`, `internal/doccheck/`, and `scripts/doccheck_ts.mjs` — G18f's
  loopback-only local advisory client, bounded claim-blind Go/TypeScript source
  view, decomposed verdict recording, and exact closed-fixture action matcher.
- `cmd/docgen/`, `internal/docgen/`, and `docs/docgen/templates.json` — G18f's
  explicit user/API placement templates, closed finite-registry adapters, and
  in-memory committed-Markdown freshness gate.
- `cmd/docjourney/` and `internal/docjourney/` — the GUI journey manifest and
  its source and test anchors, for the documented steps a browser can reach.
- `cmd/docplan/`, `internal/docplan/`, and `docs/docplan/PLAN_SLICES.json` —
  the plan's slice ledger, the generated progress log in `PLAN.md`, and the
  generated status sentence in `ROADMAP.md`.
- `cmd/docrules/`, `internal/docrules/`, and `docs/docrules/DOCUMENTS.json` —
  the root-document inventory: what each document is the home for, the pointer
  each carries to `AGENTS.md`, and the check that a new root document declares
  itself.
- `cmd/notrios/` — GUI executable: default (GUI + local service), `-no-gui`, `-gui-only -remote <url>`; `gui_wails.go` (build-tagged Wails app whose asset server routes all webview requests through the service handler or a remote reverse proxy) and `gui_stub.go` (helpful error without the tags). Build with `make gui`.
- `cmd/notrios/gui_wails.go` — native-only directory chooser binding for the
  local service mode; GUI-only remote mode deliberately receives no path picker.
- `cmd/notriosctl` — `import twitter` subcommand.
- `cmd/notriosctl` — `import chatgpt` and `import claude` subcommands.
- `cmd/notriosctl` — `export archive` and `import archive` subcommands.
- `cmd/notriosctl/` — CLI/admin/import command entry point.
- `cmd/notriosctl/main.go` now includes `notriosctl import joplin-raw`.
- `cmd/notriosctl/main.go` exposes Obsidian batch, source preservation,
  dry-run config, resume progress, and media-localization flags.
- `cmd/notriosctl/sync.go` — `notriosctl sync init|bundle|pair|status|discover|once`,
  the only way to run an exchange; every round is explicit, and there is no
  watcher or scheduler (G15).
- `cmd/notriosctl/sync_ui_e2e_test.go` and `performance/v0.7-g16/` — G16's real
  two-daemon acceptance flow and content-free desktop/mobile browser evidence.
- `cmd/notriosd/` — service daemon entry point.
- `cmd/notrioslib/`, `internal/abi/`, and `cmd/notrioslib/hosttest/` — H0's
  shared C ABI: the versioned no-GUI library, handle and session ownership,
  status codes, dispatch, and a C host test that links it.

## Packages

- `internal/api/` — shared API request/response models.
- `internal/api/types.go` mirrors the current REST DTO shapes.
- `internal/application/` — the transport-neutral application facade H0 chose as
  the owner of the application contract, in preference to narrowing
  `internal/service`: that package imports `net/http`, so an operation expressed
  there inherits an HTTP shape whether it wants one or not. The C ABI in
  `cmd/notrioslib/` is the second consumer that is not HTTP.
- `internal/archive/` — query-scoped export (notebook paths + emoji preserved, tags, resource bytes), dry-run conflict analysis with rename-suggestion `import-config.json`, validated rename-on-import (refuses names colliding with source-bound notebooks before writing), idempotent plain-note import.
- `internal/archivev2/` — native archive-v2 manifest/record model
  (`format.go`), out-of-manifest object index and fanout layout (`index.go`),
  identity-intent planner, bounded streaming verifier (`verify.go`), bounded
  external-sort spool and merge join (`spool.go`), the optional packed object
  layout (`pack.go`), the manifest-last streaming exporter (`export.go`), the
  verify-first restore reader (`restore.go`), the publication projection
  (`publish.go`), a generator-built synthetic golden fixture, and generated
  scale profiles, plus bounded declaration-only compatibility classification.
- `internal/clispec/` + `internal/clispec/commands.json` — the command line's one
  description of itself: every command with its purpose, usage form and notes,
  embedded in the binary. `printHelp` renders it, `docs/cli.md` is generated from
  it, and `internal/docgen` reads it for the command-line surface inventory.
  `cmd/notriosctl/commands_test.go` walks the dispatcher and fails when the two
  disagree; `flags_test.go` does the same for each command's flags.
- `internal/config/` — small dependency-free config loader for the documented YAML subset and directory bootstrap helper.
- `internal/credentials/` — where a secret lives, and the refusal to guess: a
  configured native store that is unavailable is an error, never a silent
  fallback to plaintext.
- `internal/doccompare/` — what the pages say against what the surfaces offer.
  It reports differences and does not decide which to close.
- `internal/docexec/` and `internal/docjourneys/` — G18d/H15's executed
  documentation: a documented command is run against a real library and its
  postcondition checked, so a copyable example that no longer works fails a
  build rather than a reader.
- `internal/docfeatures/` and `docs/docfeatures/FEATURES.json` — H18's feature
  registry, and the coverage ratchet that asks whether every surface the code
  offers is claimed by a feature a reader could find.
- `internal/helpdocs/` + `notriosctl seed-help` — deterministic Help-notebook seeding (create/update/remove).
- `internal/httpapi` implements `PUT`/`PATCH`/`DELETE` document routes plus revision list/read/restore.
- `internal/httpapi` implements `GET /api/v1/documents/{document_id}/links` and `POST /api/v1/graph`.
- `internal/httpapi/` — REST HTTP adapter for status, documents, revisions, resources, links, graph slices, and staged future routes.
- `internal/httpapi/server.go` plus its route-specific files expose live
  SQLite-backed document/search/selection, batch, and job watch/cancel routes;
  path-taking job starts remain deliberately absent.
- `internal/httpapi/server.go`: now serves `web/dist` for the built-in UI when the production build exists.
- `internal/httpapi/sidecar_search.go` — merges sidecar-only hits into search results behind the existing API; FTS5 stays authoritative and sidecar failures degrade gracefully.
- `internal/httpapi/sync.go` — the three peer routes and the middleware that
  guards them; a peer credential reaches this surface and no ordinary route.
- `internal/httpapi/sync_ui.go`, `internal/store/sync_ui.go`, and
  `web/src/components/SyncCenter.tsx` — G16's loopback/native human facade:
  redacted aggregate state, setup/discovery/pairing and separate snapshot
  permission, conflict comparison/two-parent resolution, lazy resource intent,
  repair visibility, catch-up/reset preparation, and responsive UI.
- `internal/importers/chatgpt/` — ChatGPT `conversations.json` importer (mapping tree, current-node main path, system/tool skip).
- `internal/importers/claude/` — Claude `conversations.json` importer (flat chat_messages, content blocks).
- `internal/importers/joplinraw` and `internal/importers/obsidian` record provenance rows on every import run (re-running an import backfills existing notes).
- `internal/importers/joplinraw/` — hardened Joplin RAW importer: canonical
  first-line titles, CR/LF-only physical parsing (including OCR controls), one
  ordered future/duplicate-property parse, deterministic inventory, nested
  notebooks, real tags, exact optional source bundles, fingerprints, bounded
  batches, checkpoints/resume, and dry-run/config planning.
- `internal/importers/joplinraw/joplinraw.go` owns RAW parsing/body/link helpers and the JSON report contract; `scalable.go` owns deterministic inventory, conflict planning, bounded phases, fingerprints, exact bundles, and resume.
- `internal/importers/joplinraw/joplinraw_test.go` covers hierarchy, real tags and renames, exact RAW bytes/property order, dry-run parity, resource refresh, conflicts, bounded batches, and resume; `profile_test.go` drives generated 100/10k/100k profiles.
- `internal/importers/obsidian/` — hardened deterministic vault importer with
  nested notebooks, exact optional source bundles, bounded checkpoints/resume,
  dry-run/config planning, and canonical alias/embed/anchor resolution.
- `internal/importers/obsidian/obsidian.go` owns deterministic one-pass vault
  inventory, nested-folder conflict planning, bounded fingerprint/checkpoint
  phases, exact source bundles, canonical alias/relative/embed/anchor
  resolution, resource refresh, and link rebuild.
- `internal/importers/obsidian/obsidian_test.go` covers exact Markdown/binary
  recovery, hierarchy, conflicts/renames, dry-run parity, interruption/resume,
  resource refresh, richer graph edges, Trash, search, and idempotence;
  `profile_test.go` drives generated 100/10k/100k/500k tiers.
- `internal/importers/twitter/` — extracted-archive parser (`window.YTD` wrappers, tweets.js/tweet.js, account.js, tweets_media), in-reply-to thread recovery, t.co URL expansion, media-as-resources, hashtag tags, "Twitter" notebook, provenance rows, trashed-note non-resurrection, dry run.
- `internal/jobs/` — a durable job record tied to one long-running operation, with
  two cancellation mechanisms because the operations differ: importers stop at a
  committed batch boundary, and the rest are cancelled through the context.
- `internal/localize/` — remote-media localization behind the domain policy,
  quarantine, exact-hash and dry-run rules in `SECURITY_AND_MEDIA_POLICY.md`.
- `internal/markdownblocks/` — deterministic block splitter behind schema-v14
  addressable blocks and schema-v15 heading slugs; block identity is
  content-derived and scoped to the document, and `Slugify` is the shared
  heading-anchor normalization.
- `internal/markdownlinks` extracts common Markdown links/images, Obsidian wikilinks/embeds, app URIs, external URLs, heading anchors, and block anchors.
- `internal/markdownlinks/` — conservative MVP Markdown/Obsidian/app-URI link extractor.
- `internal/media/` — the remote-media policy engine of
  `SECURITY_AND_MEDIA_POLICY.md`. The static half evaluates URL policy and scans
  Markdown and performs no network I/O, not even DNS; quarantine and
  localization re-apply the same policy to every redirect hop and to resolved
  addresses at connect time.
- `internal/migrate/` — moving a library between machines and layouts by
  copying files. It never opens the source database, because migrating a
  library must leave the library the user already had.
- `internal/paths/` — the single answer to "where does this file go?". H3 found
  the question answered in twenty-five places and the interesting defects were
  all disagreements between two of them rather than mistakes inside any one.
- `internal/purge/` — the decision procedure for "may Notrios delete this
  path?", and the backup that must succeed before it does. A port of H3's
  reference oracle, made because the safeguards lived in `scripts/lifecycle.py`
  and a packaged installation ships no Python, so the user most in need of them
  could not reach them. Two implementations of a deletion rule is the worst
  possible duplication, so `oracle_test.go` drives H3's own thirty fixtures and
  fails if the two ever disagree.
- `internal/profiles/` — the explicit local registry mapping a logical database
  ID to a database on this machine plus G3 runtime-profile config/identity and
  process-isolation validation; stable links resolve only unambiguous matches
  and report every candidate otherwise.
- `internal/profiles/runtime.go` — G3 named runtime-profile creation, redacted
  views, registry/config/database startup binding, path/port/replica collision
  detection, and explicit copied-database adopt/fork handling.
- `internal/projection/` — outbox-driven Markdown+front-matter filesystem
  projection with bounded draining, durable retry scheduling, exact
  missing/stale/orphan reconciliation, and atomic repair writes.
- `internal/publish/` — saved publication profiles (selection and privacy
  decisions only, never an output path or a command) plus the reviewed-plan gate
  that publishing must satisfy.
- `internal/query/` — bounded backend-neutral expression parser for implicit
  AND, uppercase OR, prefix negation, parentheses, phrases, `category:`/
  `notebook:`, tags, provenance/time fields, literal unknown-colon fallback,
  URLs/hyphens/emoji, and Trash scope.
- `internal/recoll/` — external-process Recoll sidecar: generated config (fields prefixes, `publishedts` range slot, `underscoreasletter`), the embedded from-scratch `notrios_md_handler.py` front-matter handler, `recollindex`/`recollq` invocation, query compilation, and result parsing. GPL boundary: binaries are user-installed and never linked or vendored.
- `internal/service/` — activates optional Recoll: startup reconciliation and
  index, 30-second bounded drain, 10-minute reconciliation, runtime status,
  cancellation, and stable deduplicated merged search.
- `internal/service/` — shared startup (directories, store, HTTP handler, Recoll sidecar loop) used by `notriosd` and `notrios`.
- `internal/snapshotimage/`, `internal/store/sqlite_snapshot.go`, and
  `internal/store/sync_snapshot_activate.go` — G14c/G14d's
  exact-schema SQLite Online Backup image, deterministic bounded external
  packs, strict manifest-last verifier, table-state sanitization, verified
  emergency backup, durable physical install, replica rotation/floors, derived
  rebuild selection, and generated 100k evidence.
- `internal/stablelink/` — strict parser/formatter for the external
  `notrios://databases/{database_id}/documents/{document_id}` link, with typed
  rejections (foreign scheme, malformed, over-limit, unsupported route).
- `internal/store` now exposes update, soft-delete, revision list/read, and revision restore operations with optimistic concurrency.
- `internal/store` now streams resource bytes into the configured asset store and deduplicates exact blobs by SHA-256.
- `internal/store` rebuilds `document_links` rows transactionally on document create/update/restore and clears outgoing links on soft delete.
- `internal/store.StoreStatus` — database driver/path/state/schema-version reporting for status output.
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
- `internal/store/notebooks_test.go` — coverage for bootstrap builtins, naming rules, nesting, membership/move, recursive delete, trash/restore/purge, tag counts, sidebar ordering, and the v4→v5 upgrade path.
- `internal/store/sources_test.go` plus importer-test assertions cover upsert/lookup, thread ordering, purge protection, and importer provenance.
- `internal/store/sqlite_notebooks.go` — notebook CRUD (nested, emoji, case-insensitive sibling-unique names, cycle-safe moves, recursive delete-to-trash), tag add/remove/list with live note counts, search-notebook lifecycle with builtin protection, and trash list/restore/purge.
- `internal/store/sqlite_query.go` — compiles positive text trees to FTS5 with
  relevance keysets and mixed/negated trees to exact parameterized SQL
  predicates with chronological keysets; recursive case-insensitive notebook
  expansion, All-notes alias semantics, emoji fallback, and canonical AST
  cursor binding are shared by every search surface.
- `internal/store/sqlite_sources.go` — `SetDocumentSource` upsert (works for trashed notes so importers can backfill), `GetDocumentSource`, `FindDocumentBySource` (importer idempotency), `ListThreadDocuments` (chronological thread recovery, trashed notes excluded); `PurgeDocument` now refuses externally-sourced notes.
- `internal/store/sync_retention.go`, migration `0027_sync_retention.sql`, and
  `performance/v0.7-g17/` — G17's signed peer retirement, snapshot/acknowledgement
  floors, death/tombstone compaction, resource-GC gate, repair plan, safety
  matrix, and generated full-corpus-scale retention-cost evidence.
- `internal/syncassets/` and `performance/v0.7-g8/` — schema-v23 attachment
  metadata that converges before its bytes, chunk manifests, bounded verified
  materialization, and eager/pinned/lazy policy.
- `internal/syncauth/` and `performance/v0.7-g13/` — the peer principal: signed
  requests rather than bearer tokens, the replay cache and rate limiters, the
  short-lived single-use pairing code and its proofs, the signing client, and
  the generated authentication/authorization matrix. Schema v25 holds enrolled
  peer public keys and pairing invitations.
- `internal/syncbackup/`, `internal/syncrest/backup.go`, and
  `internal/synccarrier/snapshot.go` — the sequential deterministic USTAR plus
  NBK1 physical snapshot transport and bounded resumable REST/directory byte
  paths. The physical verifier, not USTAR, is the trust boundary.
- `internal/syncbackup/portable.go` — NPB1 password wrapper around the verified
  physical NBK1 payload; Argon2id wraps a fresh payload key and inspection is
  authenticated, bounded, private-staging-only, and non-installing.
- `internal/syncbody/`, `internal/syncdelta/`, and `performance/v0.7-g7/` —
  schema-v22 note revision objects, bounded named-base transfer deltas, the
  line-first three-way merge with word-region refinement, and durable typed
  body conflicts.
- `internal/synccarrier/` and `performance/v0.7-g11/` — the ephemeral
  shared-directory carrier: the transport-neutral `Carrier` surface, the blinded
  folder layout, the exchange `Round`, discovery that reports without enrolling,
  and the carrier-backed `ObjectProvider`. No schema change.
- `internal/synccatchup/` and `performance/v0.7-g10/` — schema-v24 snapshot
  catch-up: signed requests, explicit source permission, competing-offer
  selection, the durable reset state machine, catch-up floors, and password
  wrapping.
- `internal/syncjobs/` — G15's closed durable sync-outbox worker and
  directory/REST carrier target adapter; explicit work only, no cadence.
- `internal/synckeys/` — the warned `0600` development secret provider holding
  one group key per epoch and this replica's signing key. Since G13 it holds
  secrets only: which peers are trusted is database state. v0.8 owns the
  platform store.
- `internal/syncmerge/` and `performance/v0.7-g6/` — schema-v21 deterministic
  HLC/field/membership/lifecycle/tree convergence core and its exhaustive,
  randomized, and SQLite evidence map.
- `internal/syncrest/`, `internal/syncbackup/`, `internal/httpapi/sync_data.go`,
  and `performance/v0.7-g14/` — the REST data plane: G11's `Carrier` implemented
  over the signing client so both transports run one protocol, the carrier
  routes with `Range`, and the original resumable encrypted archive-v2 snapshot
  baseline. Its loose-object ZIP overhead triggered G14a-G14e; G14d has replaced
  that production backup payload and wrapper, while the historical evidence
  remains the baseline.
- `internal/syncstate/` — transport/storage-neutral protocol-1.0 handshake,
  bounded state-vector comparison, deterministic missing-range planner, strict
  normalized operation model, and randomized three-replica property tests.
- `internal/syncwire/` and `performance/v0.7-g9/` — the canonical NCB1/NEV1
  encoding, deterministic gzip, and the encrypted, signed NAR1 artifact every
  transport carries. No schema change and no dependency.
- `internal/version/` — version constants.

## The web interface

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
- `web/src/App.tsx` can upload/list/download attached resources.
- `web/src/App.tsx`: REST-backed Markdown UI with `md-editor-rt`, preview normalization, app URI routing, resource upload, link/backlink/resource sidebars.
- `web/src/App.tsx` + `api.ts` + `styles.css` — Notrios sidebar layout: search notebooks first/last anchoring, nested notebook tree with emoji, tags with counts, startup "All notes" with cursor-based Load more, Help-menu event hook.
- `web/src/App.tsx` — header light/dark toggle and theme settings panel (create/edit/delete custom themes, assign per mode).
- `web/src/api.ts` and `web/src/App.tsx` can save a new revision for the currently opened note.
- `web/src/api.ts` and `web/src/App.tsx` can list and display outgoing links/backlinks for the opened note.
- `web/src/draft.ts` and `cmd/notrios/gui_window_state.go` — the unsaved-draft
  store behind explicit save, and the window state that lets the desktop app
  ask before closing over unsaved work.
- `web/src/styles.css`: app layout plus editor, link-list, resource-list, and text-button styling.
- `web/src/styles.css` — colors tokenized into CSS custom properties; theme-panel styles.
- `web/src/themes.ts` — theme token sets (builtin Light/Dark), custom-theme persistence, per-mode theme selection, applyTheme.

## Documentation and the site

- `.github/workflows/docs.yml` — GitHub Pages deployment for the docs site.
- `docs-site/`, `scripts/build_docs_site.sh`, and `.github/workflows/docs.yml` —
  G18g's production Hugo/Ledger source, exact Apache-2.0 theme provenance,
  pinned local Pagefind dependency, temporary raw-doc staging, `/notrios/`
  route adapters, offline policy, and Pages build.
- `docs-site/` + `scripts/build_docs_site.sh` — pinned Hugo/Ledger source and
  temporary raw-Markdown staging followed by a local Pagefind static index.
- `docs/` — user documentation (published to GitHub Pages and seeded into the Help notebook).
- `docs/publishing.md` — user guide for publication profiles, the
  review-then-publish gate, and what a publication withholds.
- `docs/stable-links.md` — user guide for `notrios://` links, the profile
  registry, `notriosctl link`/`open`/`profile`, and the desktop handler.

## Scripts and build

- `scripts/check_agent_usage.py`, `scripts/agent_usage_preflight.sh`, and
  `skills/agent-usage-preflight/` — model-free Codex rolling-window probe,
  cache-only Claude probe, adaptive operation reserve, and durable-boundary
  workflow used before long local profiles. CI tests parsing but never queries
  a developer account.
- `scripts/check_release_zip.py` — catches missing `web/dist`, accidental `web/node_modules`, and runtime data in ZIPs.
- `scripts/check_required_files.py` — verifies required scaffold files exist.
- `scripts/g17b_evidence.py` and `scripts/verify_evidence_pre_push.sh` — the
  production assembly/sealing tool and mandatory host-side tracked/source/
  reserve/ancestry verification gate.
- `scripts/mvp_smoke.sh` — end-to-end local REST/MCP/resource smoke test.
- `scripts/package_release.sh` — validates, builds UI, creates source ZIP, and verifies contents.
- `scripts/run_archive_export_profile.sh` — reproducible P3 generated
  100/1,000/5,000-note archive-v2 export, verify, resume, and subset profile.
- `scripts/run_editor_profile.sh` + `scripts/measure_editor.mjs` — v0.5 E6
  editor first-paint and keystroke-latency profile, driving real headless Chrome
  over the DevTools Protocol with a client written against Node's built-in
  WebSocket (no new dependency).
- `scripts/run_joplin_import_profile.sh` — reproducible H8 generated
  100/10k/100k dry-run plus interrupted/resumed import profile.
- `scripts/run_large_library_profile.sh` — reproducible H7
  10k/100k/500k keyset/search/resource profile driver.
- `scripts/run_obsidian_import_profile.sh` — reproducible H9 generated
  100/10k/100k/500k dry-run plus interrupted/resumed vault profile.
- `scripts/run_offline_assets_check.sh` + `scripts/check_offline_assets.mjs` —
  v0.5 E6a guard: fails if the built-in UI issues any cross-origin request,
  injects a remote script or stylesheet, violates the CSP, or fails to render
  math with every CDN blocked.
- `scripts/run_performance_smoke.sh` — generated-dataset smoke and benchmark wrapper.
- `scripts/validate-scaffold.sh` — runs current scaffold validation checks.

## API, configuration and schema

- `api/mcp-tools.md` defines MCP tool scopes and the implemented bounded tools. `internal/httpapi/mcp.go` contains the current dependency-free adapter mounted at `/mcp`.
- `api/openapi.yaml` — initial REST OpenAPI skeleton.
- `api/openapi.yaml` contains the REST contract for collections, documents,
  resources, revisions, links, remote media, graph, selection planning, and jobs.
  Remote-media scan/localization, selection/privacy dry runs, batches, and job
  watch/cancel are live; G3 local runtime profiles are CLI/config-only, while
  remote job starts, REST profile management, and sync remain staged.
- `config/config.example.yaml` — example service configuration and media policy.
- `contracts/archive-v2/` — G19's machine-readable capability/limit registry,
  strict Draft 2020-12 schemas, complete independently generated loose/packed/
  schema-27 goldens, narrow refusal probes, and reader matrix.
- `migrations/` — SQLite schema migrations.
- `migrations/0001_initial.sql` (and the embedded copy under `internal/store/migrations/`) now creates `notebooks`, `tags`, `note_tags`, and `search_notebooks`, adds `documents.notebook_id`, and sets `PRAGMA user_version = 5`; `ensureSchemaV5` upgrades v4 databases and bootstrap backfills existing notes into the default notebook.
- `migrations/0001_initial.sql` (both copies) adds `document_sources` (source system, external ID, author/author_id, thread_id/reply_to, source URL, published_at + derived published_ts) and sets `PRAGMA user_version = 6`.

## Evidence and measurements

- `evidence/` — G17b's public key/TSA trust material, artifact signatures,
  canonical manifest and checkpoint, final and superseded outer catalog seals,
  JSON schemas, offline verifier, 14-case refusal harness/results, custody
  template, and future burn/read-back runbook. No private key or passphrase is
  stored here.
- `performance/v0.3-h7/` — committed environment, query-plan, latency, size,
  and peak-RSS evidence from the H7 scale runs.
- `performance/v0.3-h8/` — committed Joplin import duration, batch/resume, and
  memory evidence for the H8 synthetic profiles.
- `performance/v0.4-p3/` — committed aggregate archive-v2 export timings,
  object/byte counts, peak RSS, and object-budget usage.
- `performance/v0.7-g0/` — completed G0 threat model, normative protocol
  glossary, thirty misuse/control traces, primary-source dependency/license/
  platform validation, and its structural evidence validator.
- `performance/v0.7-g1/` — completed G1 aggregate-only corpus profiles,
  deterministic delta/merge/hostile-patch workload, exact reconstruction and
  CPU/RSS evidence, superseded upstream merge-candidate probe, findings, and
  validators and the now-superseded external merge-package probe.
- `performance/v0.7-g12/` — the conformance run that puts that carrier on a real
  provider: measured visibility delay, eight protocol phases through a mounted
  Google Drive folder and a drive passed between peers, and the enforced
  refusal of every destructive rclone verb. No production code.
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
  executable browser smoke.
- `performance/v0.7-g18c/` — the exact 351-unit report, 20-case Go mutation
  matrix, eight-case TypeScript resolver coverage, and freshness validator.
- `performance/v0.7-g18f/` — deterministic hashes/counts, the repeated local
  Qwen advisory report, human dispositions, mutation matrix, and model-free
  validator. The low calibration score is evidence, not a build oracle.
- `performance/v0.7-g18g/` and `internal/helpdocs/g18g_docs_test.go` — G18g's
  exact-theme, route/fragment, search-scope, semantic rendering, offline,
  measurement, mutation, rendered-browser, and raw Help equivalence gates.
- `performance/v0.7-g19/` — external MoveNotes-consumer absence audit and G19
  validation evidence; no external repository was modified.
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

## Plans and agent support

- `agent/ATTEMPT_LOG.jsonl` — append-only attempt history.
- `agent/LOOP_DETECTION.md` — stalled-task detection and resolution.
- `agent/MODEL_LOG.jsonl` — model/session tracking.
- `agent/PLAN_STATUS.md` — current task and working-state notes.
- `plans/` — archived completed plans by version/milestone.
- `plans/mvp/MVP_RELEASE_REPORT.md` — completed MVP capability summary and deferred features.
- `plans/mvp/MVP_TASK1_REPORT.md` — service persistence foundation completion report.
- `plans/mvp/MVP_TASK2_REPORT.md` records this task.
- `plans/mvp/MVP_TASK4_REPORT.md` records this task.
- `plans/mvp/MVP_TASK5_REPORT.md` records this task.
- `plans/mvp/MVP_TASK6_REPORT.md`: implementation notes and deliberate limitations for the UI MVP.
- `plans/scaffold/SCAFFOLD_CREATION_PLAN.md` — process for creating/refining this scaffold.
- `plans/scaffold/SCAFFOLD_REVIEW_REPORT.md` — Step 2 repair summary.
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
- `plans/v0.8/001-installation-delivery-plan-amendment.md` — primary-source
  checked planning record for XDG/native paths, safe Make lifecycle, Ubuntu-
  priority installers, GitHub-native Windows/macOS evidence, delayed PR/branch
  synchronization, and the v0.8-v1.0 end-user delivery sequence. It implements
  no product behavior.
- `plans/v0.8/002-application-facade-c-abi-sqlite-ownership-investigation.md`
  and `performance/v0.8-h0/` — H0's selected application owner, exact SQLite
  3.53.4 provenance/update policy, ABI-major-1 lifecycle/ownership form,
  desktop/API-35 store and cost matrix, arm64 build-only artifacts,
  cross-engine checkpointed round-trip, single-engine probe, limitations, and
  validator.
- `prompts/` — reusable agent prompts.
- `skills/` — agent skills using the `SKILL.md` format.
- `skills/codex-handoff/` renamed to `skills/agent-handoff/`; `prompts/start_codex_from_handoff.md` renamed to `prompts/start_agent_from_handoff.md`.

## Assets and test data

- `testdata/schemas/` — genson-derived JSON Schemas for the Twitter archive formats (synthetic samples only).
- `testdata/schemas/` — genson-derived schemas for both export formats.
