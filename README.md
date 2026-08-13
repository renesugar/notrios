# Notrios

Notrios (formerly "Notes Companion") is a local-first note-taking, search, import, and publishing system for very large Markdown and document collections.
It combines a Go REST/MCP service (`notriosd`), a built-in GUI, SQLite/FTS5-backed canonical storage, content-addressed resources, optional Recoll-derived search/extraction, and support for third-party native clients (C++/Qt, Go/Wails, Rust/Tauri) over the same API. A versioned no-GUI C ABI is planned before 1.0, followed by an independent post-1.0 Flutter client; mobile delivery does not depend exclusively on Wails.

The v0.1 through v0.6 milestones are complete. v0.7 G0-G6 are complete: threat
and workload evidence, pure-Go binary-delta feasibility, bounded envelope and
resource design, isolated runtime profiles, the local journal, and bounded
state-vector admission, and deterministic metadata/tree convergence. G7 is next and unapproved — see
[`PLAN.md`](PLAN.md) and
[`ROADMAP.md`](ROADMAP.md). The repository is
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

User documentation lives under [`docs/`](docs/index.md) and is published as a GitHub Pages site with PageFind search (`bash scripts/build_docs_site.sh` builds it locally). `notriosctl seed-help` mirrors the same content into the app's built-in read-only Help notebook for offline use.

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
  transport will carry, using only Go standard-library cryptography.
  Transport and peer authentication are not implemented yet.
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
