# Notrios

Notrios is a local-first note-taking, search, import and publishing system for
very large Markdown and document collections. It was formerly called "Notes
Companion".

**What it is made of.** A Go REST/MCP service (`notriosd`), a desktop GUI, a
command-line tool (`notriosctl`), and SQLite/FTS5 canonical storage with
content-addressed resources. Search and text extraction can optionally use a
Recoll sidecar. Third-party native clients (C++/Qt, Go/Wails, Rust/Tauri) work
against the same API.

**Where it runs.** Ubuntu Linux, which is the only tested platform. There is a
`.deb` package a person can install without Go, Node or a compiler; Windows and
macOS installers are post-1.0, when there is hardware to test them on.

**What is planned before 1.0.** A versioned no-GUI C ABI, so a client can link
the core rather than speak to the service. An independent Flutter client comes
after 1.0; mobile delivery does not depend on Wails.

**Where to start reading.** [Quick start](#quick-start) to build and run it;
[`docs/index.md`](docs/index.md) for the user documentation;
[Project status](#project-status) for where the work is;
[`ROADMAP.md`](ROADMAP.md) for where it is going. The per-milestone history that
used to live in this paragraph is in Project status, one row per milestone.

**If you are a coding agent,** read [`AGENTS.md`](AGENTS.md) first — it carries
the reading order, the authorizations that are and are not in force, and, under
"Keeping the reference documents current", the rules for updating this file and
the other reference documents. The repository is structured so an agent can
resume safely after usage limits or a model change.

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

User documentation lives under [`docs/`](docs/index.md) as Markdown you can
read in the repository, and is published at
[notrios.com](https://notrios.com) as a pinned Hugo/Ledger site with offline
Pagefind search. `notriosctl seed-help` mirrors the same raw bytes into the
app's built-in read-only Help notebook, so the documentation works with no
network at all.

### Reading the site locally

```bash
make docs                                        # builds _site/ (first run installs Pagefind)
python3 -m http.server 8000 --directory _site    # then open http://127.0.0.1:8000
```

`make docs` writes `_site/` and nothing else in the tree; both `_site/` and
`docs-site/node_modules/` are git-ignored. It needs the pinned toolchain the
build script checks for — Hugo Extended 0.164.0, Node 26.3.0, Pagefind 1.5.2 —
and refuses rather than producing a site built with something else.

**`_site/` is not rebuilt by `make validate`.** Nothing in the ordinary loop
refreshes it, so a checkout can hold months-old rendered documentation while
every gate is green. `make docs-stale` says whether it has fallen behind
`docs/`; `make docs` fixes it.

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

Everything also runs from source: `go run ./cmd/notriosd -config config/config.example.yaml` and `go run ./cmd/notriosctl <command>`. `make help` lists every target.

### Installing, and removing

These are the targets that touch files outside the checkout, so they are listed
together. Full detail, including the packaged install, is in
[docs/installation.md](docs/installation.md).

```bash
make install                 # copy into $HOME/.local by default; records a manifest
make install-dry-run         # print every path it would write, write nothing
make uninstall               # remove only what that manifest recorded installing
make purge                   # uninstall AND delete this user's Notrios data
DRYRUN=1 make purge          # print the whole plan and stop; asks nothing
```

`make install` writes a `MANIFEST.json` and `make uninstall` removes only what
that manifest recorded, so a file you edited is kept and reported rather than
deleted. Pass `prefix=/some/where` to both.

**`make purge` is the one that destroys things you made.** It deletes your
notes, configuration, state and cache after writing a verified backup beside
your state root, and it asks first. `make uninstall` never touches your data;
`make purge` is uninstall plus your data. A packaged install has no Makefile, so
the same deletion is `notriosctl purge`. Both are documented in
[docs/installation.md](docs/installation.md).

### Cleaning up

These two only ever touch the checkout, and never your notes, configuration or
installed files:

```bash
make clean                   # build output, and every __pycache__/.pyc in the tree
make clobber                 # clean, plus web/node_modules/ and docs-site/node_modules/
make precheck                # fail if the tree has uncommitted or ignored-but-tracked files
```

`clean` removes what a build produces — `bin/`, `dist/`, `_site/`, `web/dist/`,
`.playwright-mcp/`, the coverage files, and every `__pycache__` directory and
`.pyc` file in the tree — so the next build remakes it. `clobber` additionally
removes the installed npm dependencies, so the next build has to run `npm ci`
again; use it when a dependency tree is suspect, not routinely.
Neither reads or writes `~/.config/notrios`, `~/.local/share/notrios` or
anything `make install` wrote: deleting those is `make purge` and nothing else.

There are no official prebuilt binaries yet; Notrios is built from source on
Ubuntu Linux, the only tested platform.

## Configuration

**[docs/configuration.md](docs/configuration.md) is the reference** — what each
section is for, which roots a purge treats differently, and a generated table of
all 53 settable keys with their types and defaults.

Notrios runs with no configuration file at all; everything has a default.
`notriosd` takes `-config <path>`, and without one it loads
`config/config.example.yaml` from a source checkout if that file exists and
compiled defaults otherwise. `-addr` and `-db` override the file.

`notriosctl config show` prints what is actually in effect, which is the command
to run before editing anything; `notriosctl paths` shows the resolved storage
roots and how they were resolved, and `notriosctl doctor` checks they exist and
that the database opens. `/api/v1/status` reports the same roots plus
database/schema state, capability and search limits, and optional Recoll
availability, backlog, sync/index and reconciliation state.

## Project status

Where the project is, generated from the slice ledger by
`go run ./cmd/docplan --write` and gated, so it cannot be the paragraph that
drifts. The rules for keeping the reference documents current are in
[`AGENTS.md`](AGENTS.md).

<!-- notrios:generated:readme:status:begin -->
**Current milestone: v1.0.** 14 items: 4 complete, 1 in progress, 9 not started, 0 deferred. Item by item, with what each one proved and what it left owed, in [`PLAN.md`](PLAN.md).

**Product version: 0.8.0**, which is what every binary reports and what a release is tagged with. It is not the milestone number: the version is bumped when the release is cut, which is the last item in the plan.

**Archived milestones:** v0.1, v0.2, v0.3, v0.4, v0.5, v0.6, v0.7, v0.8, v0.8e, v0.9 — one directory each under [`plans/`](plans/), holding the plan as it stood when the milestone closed.
<!-- notrios:generated:readme:status:end -->

What each milestone *meant* is below, in prose, because that is not derivable
from a ledger. `plans/scaffold` and `plans/mvp` are records of how the
repository started rather than numbered milestones, so the generated list above
does not name them.

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
  sanitized subset handoff. The deferred P6 external contract is now completed
  by v0.7 G19 with published schemas, deterministic goldens, and bounded
  compatibility admission; no MoveNotes consumer currently exists.
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
- **v0.6 (0.6.0) — MCP and automation expansion.** Notebook targeting
  in the GUI and CLI (F0), bounded batch organizer transactions with an
  idempotency ledger (F1), four cumulative MCP tool scopes enforced at the call
  site (F2), MCP read coverage with bounded resource reads and HTTP `Range`
  (F3), note templates and task extraction (F4), graph views that stay readable
  at scale — a local graph, a hubs report written as a read-only note, and CSV
  export (F5), a job control plane for long imports and exports (F6), and the
  documentation and release wrap-up (F7). Archived under `plans/v0.6/`. See
  [`PLAN.md`](PLAN.md) for the next milestone and [`ROADMAP.md`](ROADMAP.md) for
  the sequence.
- **v0.7 (0.7.0) — native synchronization.** G0-G2 completed the threat,
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
  password-protected backup review without silently restoring anything. G17
  adds signed retirement, acknowledgement/snapshot-gated retention, repair,
  and evidence preservation. G18 freezes the shared-core/C-ABI portability
  handoff and then makes documentation anchors, examples, GUI journeys, the
  Hugo/Ledger site, and advisory review reproducible. G19 publishes the strict
  archive-v2 consumer contract. G20 reconciles the full system, hardens remote
  admission and untrusted filesystem roots, validates schema v27 upgrade and
  recovery, and ships the 0.7.0 source release candidate. All G0-G20 slices are
  archived under `plans/v0.7/`.
- **v0.8 — installation, configuration, shared core, and portability.** H0-H1 established the application facade, the frozen version-1
  C ABI and one hidden SQLite engine. H3-H6 centralised installed paths on the
  XDG contract, added a reviewed `install`/`uninstall`/`purge` lifecycle and an
  Ubuntu package a person can install without Go, Node or a compiler; Windows
  and macOS moved to post-v1.0 with the hardware they need. H9 chose the native
  credential store and refuses a silent fall back to plaintext. H11 accepted the
  shared core on an Android emulator, and H10's spike recommends migrating the
  desktop to Wails v3 once it releases. The rest closed a gap the milestone kept
  finding: a capability reachable from one adapter and not another. Search,
  reading, discovery, tagging, attachments and batch operations came to the
  command line; the interface gained names for its controls, multi-select, and a
  journey for every capability it offers. The schema is unchanged at v27. All
  H0-H29 slices are archived under `plans/v0.8/`.
- **v0.8e — the evidence reserve.** A separate short milestone rather than a
  slice of v0.8, because it is the one thing that cannot be done retroactively:
  four chained, signed and RFC 3161 timestamped volumes (`NTR-EV-0001` to
  `NTR-EV-0004`) over an append-only outer catalog, and a host-side verifier
  that gates every push. Archived under `plans/v0.8e/`.
- **v0.9 — release-candidate hardening.** I1 put the repository on GitHub and
  reconciled `develop` with `main`. I3 promoted the Ubuntu installer through
  clean containers, I4 hardened the destructive lifecycle and fixed a profile
  race, I5 settled signing, notarization and timestamping policy, and I6
  generated and verified the release evidence set — SBOM, provenance,
  checksums — cross-checked against syft and cdxgen. I7 soaked the service,
  drilled recovery, and froze the support matrix; I8 froze the 1.0
  compatibility surfaces by re-deriving them rather than restating them. I9
  rewrote the operational documentation for a reader who has only the package,
  and I10 moved the documentation site to `notrios.com`. I2 — the Wails v3
  desktop migration — is **deferred**, on the record, because Wails v3 has not
  released. Archived under `plans/v0.9/`.
- **v1.0 — feature-complete local product.** In progress; the generated block
  at the top of this section counts it, and [`PLAN.md`](PLAN.md) says what each
  item proved and what it left owed.
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
make docgen                   # deterministic user/API documentation freshness
make g18f-validate            # model-free generated/advisory evidence checks
make g18g-validate            # pinned Hugo/Ledger source/site evidence
make smoke                    # end-to-end REST/MCP smoke test
bash scripts/package_release.sh    # validated source ZIP into dist/
```

`make validate` is the one to run before a commit. The others are narrower and
slower: `g18g-validate` rebuilds the documentation site, and
`package_release.sh` runs everything above plus the npm installs.

Two reporting targets fail nothing and answer a question:

```bash
make docs-stale               # has _site/ fallen behind docs/?
make bytecode                 # is there __pycache__/.pyc left in the tree?
```

### Performance profiles

```bash
bash scripts/run_performance_smoke.sh
bash scripts/run_large_library_profile.sh 100000 /tmp/notrios-profile.json
bash scripts/run_joplin_import_profile.sh 100000 /tmp/notrios-joplin-profile.json
bash scripts/run_real_joplin_profile.sh <label> <raw-export-dir> /tmp/notrios-joplin-real.json
bash scripts/run_obsidian_import_profile.sh 100000 /tmp/notrios-obsidian-profile.json
bash scripts/run_recoll_hardening_profile.sh 100000 /tmp/notrios-recoll-profile.json
```

The scale argument is not free-form. The collection profile accepts `10000`,
`100000` or `500000`; Joplin accepts `100`, `10000` or `100000`; Obsidian also
accepts `500000`; Recoll accepts `100` or `100000`.

### Where the evidence is

<!-- notrios:generated:readme:evidence:begin -->
77 evidence directories under [`performance/`](performance/), generated by `go run ./cmd/docplan --write`. Each holds the record for one plan item: what was measured, how, and what was not.

| Milestone | Evidence directories | For example |
|---|---|---|
| v0.3 | 4 | `performance/v0.3-h10/` |
| v0.4 | 10 | `performance/v0.4-j2/` |
| v0.5 | 7 | `performance/v0.5-e1/` |
| v0.7 | 33 | `performance/v0.7-g0/` |
| v0.8 | 13 | `performance/v0.8-h0/` |
| v0.8e | 1 | `performance/v0.8e/` |
| v0.9 | 7 | `performance/v0.9/` |
| v1.0 | 2 | `performance/v1.0/` |
<!-- notrios:generated:readme:evidence:end -->

A slice that added no new unbounded surface carries no profile of its own and
says so in its archived plan — v0.5's E1b, E3, E6b, E7 and E8 are the worked
example of that.

### Before tagging a release

Read [`PACKAGING.md`](PACKAGING.md), [`SECURITY_REVIEW.md`](SECURITY_REVIEW.md)
and [`RELEASE_CHECKLIST.md`](RELEASE_CHECKLIST.md).
