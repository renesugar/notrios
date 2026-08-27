# Notrios

Notrios (formerly "Notes Companion") is a local-first note-taking, search, import, and publishing system for very large Markdown and document collections.
It combines a Go REST/MCP service (`notriosd`), a built-in GUI, SQLite/FTS5-backed canonical storage, content-addressed resources, optional Recoll-derived search/extraction, and support for third-party native clients (C++/Qt, Go/Wails, Rust/Tauri) over the same API. A versioned no-GUI C ABI is planned before 1.0, followed by an independent post-1.0 Flutter client; mobile delivery does not depend exclusively on Wails.

The v0.1 through v0.6 milestones are complete. v0.7 G0-G18 are complete through
the authenticated REST sync data plane and resumable encrypted snapshot
download. The blocking G14a-G14e archive-scalability sequence is complete:
G14b selected and G14c implemented a
same-schema SQLite image plus bounded packed assets for full backup/catch-up
while retaining semantic archive-v2; G14d integrated crash-safe restore and
incremental catch-up, and G14e passed/froze the production full-scale contract.
G15 adds the schema-v26 durable sync outbox, bounded retries/cancellation, and
explicit-scope REST/MCP control. G16 adds the responsive local Sync Center,
native directory chooser, explicit pairing/snapshot permission, lazy-resource
and three-way conflict workflows, and non-persistent password backup review.
G17 advances schema v27 with signed peer retirement, a configurable 90-day
history horizon, acknowledgement plus re-verified-snapshot collection floors,
permanent death identity, sync-aware resource GC, and explicit snapshot catch-
up below the floor. The blocking G17a-G17b preservation sequence is complete:
G17a proved the contract, and G17b sealed 81 curated artifacts, issued and
independently verified immutable reserve volume `NTR-EV-0001`, and committed
the finite outer catalog without rewriting original handoffs. Its host-side
verifier now gates every future GitHub push. G18 completed the finite 19-
capability platform matrix, 109-operation facade audit, ABI-major-1 ownership/
error/cancel/stream proposal, Android SQLite/cgo finding, and exact Mermaid
enablement gate without implementing or claiming those future products. Its
follow-up now makes pinned modernc/libc a measured H0 candidate beside the C
amalgamation control while preserving Go database ownership; neither is
selected. A later API 35 x86_64 emulator run passed the disposable candidate's
FTS5/JSON/WAL/integrity, reboot-persistence, and shared-library-load pre-gates;
Android arm64 and the production Notrios ABI/store workload remain open. G18a
completed the documentation-anchor investigation. G18b then selected a pinned
minimal Ledger source snapshot, preserved `.html`/fragment routes, and static
Pagefind through a repository-owned prototype without switching production.
G18c now provides the deterministic cross-language `docaudit` graph, a typed
claim registry, and an honest 351-unit coverage baseline. G18d and the remaining
documentation/Hugo-Ledger work are next and unapproved. GitHub
push and physical disc burning remain unauthorized — see [`PLAN.md`](PLAN.md),
[`ROADMAP.md`](ROADMAP.md), and
[`plans/v0.7/034-android-emulator-modernc-runtime.md`](plans/v0.7/034-android-emulator-modernc-runtime.md). The repository is
structured so a coding agent can resume safely after usage limits or model
changes.

## Technology choices

- **Service:** Go, standard library first, SQLite/FTS5 canonical storage. Binaries: `notriosd` (service) and `notriosctl` (CLI); Go module `github.com/renesugar/notrios`.
- **MCP:** dependency-free JSON-RPC adapter at `/mcp`, with the `read-only` tool
  scope by default and `editor`-scope writes with revision preconditions; replace the
  adapter with the official Go SDK later without changing tool semantics.
- **Built-in GUI:** Go + Wails v2 (`cmd/notrios`, built with `make gui`) with
  `-no-gui` and `-gui-only` modes; the React + Vite frontend runs inside the
  webview and in the browser. Wails v3/mobile is a later measured option, while
  the roadmap also carries a shared Go C ABI and post-1.0 Flutter client.
- **Search:** SQLite FTS5 for managed notes; Recoll as an optional derived sidecar for field/front-matter search, OCR-style extraction, and arbitrary files (see `RECOLL_INTEGRATION.md` and `SEARCH_QUERY_LANGUAGE.md`).
- **Data model:** nested notebooks with emoji icons, tags, query-backed search
  notebooks (All notes/Trash), a default Notes notebook, read-only Help and
  Reports notebooks, and G6's protected Recovered repair target — see
  `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`.
- **Import sources:** Joplin RAW, Obsidian vaults, Twitter/X archives (thread-preserving), ChatGPT exports, Claude exports.
- **Resource store:** content-addressed assets, exact-hash/reference reports,
  and optional review-only perceptual-hash hooks (no algorithm ships by
  default), with dry-run-first retention-aware garbage collection.
- **Publishing/docs:** the shared read-only selection/privacy planner is live;
  the future native-archive subset handoff goes to the external `movenotes-v3`
  pipeline, which owns Obsidian/Quartz and Hugo/Ledger+Bluge projections.
  Project documentation ships as a GitHub
  Pages site with PageFind search (`DOCS_SITE.md`).

## Documentation

User documentation lives under [`docs/`](docs/index.md) and is published as a GitHub Pages site with PageFind search (`bash scripts/build_docs_site.sh` builds it locally). `notriosctl seed-help` mirrors the same content into the app's built-in read-only Help notebook for offline use. G18a froze the source-anchor contract, G18b selected/proved the pinned Hugo/Ledger integration under `performance/v0.7-g18b/`, and G18c implemented `make docaudit`; production still uses the current builder. G18d-G18g remain approval-gated for executed examples/journeys, generation, advisory review, and the eventual site switch.

## Quick start

Requirements (Ubuntu-tested): Go 1.25+, `build-essential pkg-config libsqlite3-dev`, and Node 22 for the web UI. Full details, GUI prerequisites, and local installation: [docs/installation.md](docs/installation.md).

```bash
git clone https://github.com/renesugar/notrios.git
cd notrios
make build web              # bin/notriosd, bin/notriosctl, web/dist/
./bin/notriosd -config config/config.example.yaml
# open http://127.0.0.1:8080  (REST: /api/v1, MCP: /mcp)
```

Desktop GUI (needs `libgtk-3-dev libwebkit2gtk-4.1-dev`):

```bash
make gui && ./bin/notrios
```

Everything also runs from source: `go run ./cmd/notriosd -config config/config.example.yaml` and `go run ./cmd/notriosctl <command>`. `make help` lists all build, test, docs, and cleanup targets. There are no official prebuilt binaries yet; Notrios is built from source on Ubuntu Linux (the only tested platform).

## Configuration

`notriosd` now loads runtime configuration from `-config <path>`. When no path is supplied, it loads `config/config.example.yaml` from a source checkout if that file exists; otherwise it uses compiled local-development defaults. The `-addr` and `-db` flags remain available as explicit overrides.

On startup, the service creates the configured data directory, SQLite database
parent directory, asset store, projection directory, and search-sidecar index
directory (`search_sidecar.index_dir`). `/api/v1/status` reports the active
storage roots, database/schema state, capability/search limits, and optional
Recoll availability, backlog, sync/index, and reconciliation state.

## Project status

- v0.1 MVP: complete (see `plans/mvp/MVP_RELEASE_REPORT.md`).
- v0.2 Notrios redesign: **complete** (all 16 tasks; see `plans/v0.2/`) — notebooks/tags/search notebooks, source provenance with conversation threads, the query language, the optional Recoll sidecar, five importers, query-scoped export/import, the Wails GUI with themes, and the documentation site.
- v0.3 import/resource/media/large-library hardening: **complete** (tasks
  H1–H11; see `plans/v0.3/`) — safe remote-media localization, resource
  reports/retention, keyset pagination, hardened resumable Joplin/Obsidian
  imports, and a convergent observable Recoll sidecar.
- v0.4 portable data, publishing, and stable references: **complete**
  (see `plans/v0.4/`) — bounded boolean search, the shared selection/privacy
  planner, native archive v2 as a real backup format (streaming export at
  382,206-note scale under a loose or optional packed layout, read-only
  verification, and restore under mandatory replace/adopt/merge/fork intent),
  portable `notrios://` links with an explicit local profile registry and an
  Ubuntu protocol handler, and reviewed publication profiles that emit a
  sanitized subset handoff. P6, the movenotes-v3 compatibility bridge, is
  deferred to v0.7.
- **v0.5 (0.5.0) — better editing, blocks, and graph UX.** Addressable
  blocks (E1), heading anchors in stable links (E1a), scheme-scoped anchor
  decoding (E1b), workspace lint (E2), workspace fix (E3), bounded graph
  traversal with shortest paths and an orphan/hub report (E4), editor link
  intelligence (E5), the CodeMirror decision (E6 — stay), offline-first frontend
  assets (E6a), HTML table paste normalization (E6b), embedded query blocks
  (E7), organizer UX with trash-first delete/restore in the GUI and hierarchical
  tag rename (E8), and the documentation and release wrap-up (E9). All thirteen
  slices are archived under `plans/v0.5/`, including a copy of the plan itself
  at `plans/v0.5/000-v0.5-plan.md`.
- **v0.6 (current, 0.6.0) — MCP and automation expansion.** Notebook targeting
  in the GUI and CLI (F0), bounded batch organizer transactions with an
  idempotency ledger (F1), four cumulative MCP tool scopes enforced at the call
  site (F2), MCP read coverage with bounded resource reads and HTTP `Range`
  (F3), note templates and task extraction (F4), graph views that stay readable
  at scale — a local graph, a hubs report written as a read-only note, and CSV
  export (F5), a job control plane for long imports and exports (F6), and the
  documentation and release wrap-up (F7). Archived under `plans/v0.6/`. See
  [`PLAN.md`](PLAN.md) for the next milestone and [`ROADMAP.md`](ROADMAP.md) for
  the sequence.
- **v0.7 (in progress) — native synchronization.** G0-G2 completed the threat,
  revision/delta, pure-Go VCDIFF, envelope, and resource-bound investigations.
  G3 adds named owner-only runtime profiles, explicit database/replica binding,
  isolated paths and loopback ports, copied-database adopt/fork checks, and a
  verified two-daemon working state. G4 adds schema-v19 explicit enrollment and
  snapshot boundaries plus an atomic local operation journal over canonical
  writes. G5 adds the transport-neutral protocol-1.0 compatibility handshake,
  bounded state vectors/missing-range planning, durable dependency queues,
  exact replay handling, and atomic contiguous acknowledgement advancement in
  schema v20. G6 advances schema v21 with durable HLC order, sparse field and
  membership registers, trash/restore/death-certificate state, and deterministic
  notebook orphan/cycle/name repair applied atomically to canonical metadata.
  G7 advances schema v22 with immutable revision objects, optional named-base
  VCDIFF transfer deltas that are verified before any canonical write, a bounded
  line-first three-way merge with word-region refinement, and durable typed
  body conflicts that keep both variants. G8 advances schema v23 with
  attachment metadata that converges before its bytes, chunk manifests,
  resumable verified materialization, and eager/pinned/lazy policy. G9 adds the
  canonical envelope codec and the encrypted, signed protocol artifact every
  transport will carry, using only Go standard-library cryptography. G10 adds
  schema-v24 snapshot catch-up: signed backup requests, explicit snapshot-source
  permission, one-of-many offer selection, the durable reset state machine, and
  the catch-up floor that lets a restored replica resume incremental admission.
  G11 adds the ephemeral shared-directory carrier and `notriosctl sync`, G12
  proves it against a mapped cloud folder and a drive passed between peers, and
  G13 advances schema v25 with the first authenticated surface: a peer principal
  proved by a signature over each request, authorizing `/api/v1/sync/...` for one
  database and nothing else, paired by a short-lived single-use code. G14
  implements the REST data plane and resumable encrypted snapshot download.
  Its 100/500-note evidence exposed about 25% stored-ZIP overhead from loose
  per-object entries. G14a has now built and calibrated the resumable benchmark
  at 10k/100k. G14b's full-corpus evidence selected a consistent same-schema
  SQLite image plus bounded packed assets for whole-library backup/catch-up,
  while retaining semantic archive-v2. G14c now supplies the production local
  `snapshot create|verify` representation and admission boundary. G14d now
  replaces the loose-object stored-ZIP producer with sequential USTAR plus the
  existing authenticated frames, adds resumable REST/directory bulk transfer,
  a verified emergency snapshot and durable crash-safe `snapshot restore`, a
  fresh replica identity/floor, and post-snapshot replay. G14e then passed 19
  privacy-sanitized full-scale phases, including current/previous semantic
  readers, first/unchanged physical snapshots, attachment/source-bundle round
  trips, REST/directory restore, scoped replay, and frozen Restic/Borg checks.
  `sqlite-image+packed-assets.v1` is now the compatible whole-library default;
  semantic archive-v2 remains portable. G15 then adds durable per-target sync
  jobs, checkpoint-safe cancellation/retry, byte budgets, and explicit-scope
  REST/MCP control without adding cron or a workflow scheduler. G16 then adds a
  loopback/native-only Sync Center for setup, pairing, explicit catch-up
  permission, job state, lazy attachments, conflicts, repair reports, and
  password-protected backup review without silently restoring anything. G17 is
  next and approval-gated.
- Agent progress/attempt tracking: [`agent/PLAN_STATUS.md`](agent/PLAN_STATUS.md), [`agent/ATTEMPT_LOG.jsonl`](agent/ATTEMPT_LOG.jsonl), and [`agent/MODEL_LOG.jsonl`](agent/MODEL_LOG.jsonl).

## Contributing

Development happens on `develop`; `main` takes reviewed merges. [`ENVIRONMENT_SETUP.md`](ENVIRONMENT_SETUP.md) covers the contributor environment, everyday `make` targets, validation, and the cleanup/pre-checkin workflow (`make clean`, `make precheck`).

## License

Notrios is licensed under the [Apache License 2.0](LICENSE). All code and dependencies must remain Apache-2.0 compatible; GPL tools (Recoll, Xapian) are only ever invoked as external user-installed processes.

## Notrios redesign design documents

- `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md` — notebooks, tags, search notebooks, Trash/Help semantics.
- `SEARCH_QUERY_LANGUAGE.md` — user query language and backend translation rules.
- `RECOLL_INTEGRATION.md` — Recoll sidecar design (replaces sist2) and licensing boundary.
- `DOCS_SITE.md` — GitHub Pages documentation site with PageFind and the Help notebook.
- `SYNCHRONIZATION.md` — planned state-vector/change-log merge algorithm,
  ephemeral-directory and REST transports, lazy resources, secure catch-up,
  retention, and backup/restore relationships.
- `NATIVE_ARCHIVE_V2.md` — implemented v2 identity, immutable-object,
  manifest-last, compatibility, restore-intent, and verification contract.
- `CODING_CLIENT_HANDOFF.md` — agent handoff (formerly `CODEX_HANDOFF.md`).

## Historical design and report documents

The scaffold-era and v0.1-MVP reports (`plans/scaffold/SCAFFOLD_*.md`, `plans/mvp/MVP_TASK*_REPORT.md`, `plans/mvp/MVP_RELEASE_REPORT.md`) are retained as historical records of how the project was built; they intentionally keep the old "Notes Companion" naming and superseded design decisions. Living design documents: `FEATURE_MATRIX.md`, `UI_DESIGN.md`, `PUBLISHING_POLICY.md`, `VERSIONING_AND_SYNC_POLICY.md`, `WORKSPACE_MAINTENANCE.md`, plus the redesign documents listed above.

## Importing your notes

`notriosctl` imports Joplin RAW exports, Obsidian vaults, Twitter/X archives, ChatGPT exports, and Claude exports, and round-trips a native archive format — each with a `--dry-run` mode and an idempotent re-run story. Joplin and Obsidian restore nested notebooks, resume durable bounded batches, emit conflict-rename plans, and can retain exact optional source bundles; Obsidian also canonicalizes aliases, relative links, embeds, headings, and block references without discarding original vault bytes. See [docs/import-export.md](docs/import-export.md) and [docs/cli.md](docs/cli.md).

## Validation

```bash
make validate                 # tests + repository checks
cd web && npm audit           # advisory gate for the bundled offline modules
make docaudit                 # documentation/source/claim graph and grades
make smoke                    # end-to-end REST/MCP smoke test
bash scripts/run_performance_smoke.sh
bash scripts/run_large_library_profile.sh 100000 /tmp/notrios-profile.json
bash scripts/run_joplin_import_profile.sh 100000 /tmp/notrios-joplin-profile.json
bash scripts/run_real_joplin_profile.sh <label> <raw-export-dir> /tmp/notrios-joplin-real.json
bash scripts/run_obsidian_import_profile.sh 100000 /tmp/notrios-obsidian-profile.json
bash scripts/run_recoll_hardening_profile.sh 100000 /tmp/notrios-recoll-profile.json
bash scripts/package_release.sh    # validated source ZIP into dist/
```

The collection scale profile accepts `10000`, `100000`, or `500000`; Joplin
accepts `100`, `10000`, or `100000`; Obsidian also accepts `500000`; Recoll
accepts `100` or `100000`. Committed reference evidence is under
`performance/v0.3-h7/` through `performance/v0.3-h10/` and
`performance/v0.4-j2/` through `performance/v0.4-p7/`; the v0.5 profiles are
under `performance/v0.5-e1/`, `performance/v0.5-e1a/`, `performance/v0.5-e2/`,
`performance/v0.5-e4/`, `performance/v0.5-e5/`, `performance/v0.5-e6/`, and
`performance/v0.5-e6a/`. Slices that added no new unbounded surface (E1b, E3,
E6b, E7, E8) carry no profile of their own and say so in their archived plan.
See `PACKAGING.md`,
`SECURITY_REVIEW.md`, and `RELEASE_CHECKLIST.md` before tagging a release.
Archive-v2 evidence, including the attachment-bearing round trip, is under
`performance/v0.4-p3/` through `performance/v0.4-p4/`.
