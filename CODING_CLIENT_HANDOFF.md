# Coding Client Handoff

This handoff applies to any coding agent or client continuing this project (Codex, Claude, aider, swival.dev, etc. — formerly `CODEX_HANDOFF.md`). The repository is designed so an agent can continue from repository files alone.

Current phase: v0.1, v0.2, and v0.3 are complete. H1–H11 are archived under
`plans/v0.3/`. v0.4 J1 (Joplin RAW physical-line/canonical-title parsing), J2
(real-export correctness and bounded relationship planning), J3
(million-item transactional throughput), Q1 (bounded boolean/category search),
P1 (shared selection/privacy planning), P2 (native archive-v2 format and
identity verification), P3 (native archive-v2 streaming export), and P3a
(archive-v2 large-library container revision) are complete. **P3b** (packed
object layout) is the next task: a full backup of the real 382,206-note corpus
wrote 382,407 loose objects in 48m26s, about 131 objects per second, because
throughput tracks one fsync per object rather than bytes — and v0.7 sync would
pay one transport round trip per object. P3b and all remaining
archive/publishing product tasks require user approval. See the revised
`PLAN.md`.

## First files to read

1. `AGENTS.md`
2. `README.md`
3. `PLAN.md`
4. `ROADMAP.md`
5. `CODING_CLIENT_HANDOFF.md`
6. `SYSTEM_ARCHITECTURE.md`
7. `API_SPEC.md`
8. `DATABASE_SCHEMA.md`
9. `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`, `SEARCH_QUERY_LANGUAGE.md`,
   `RECOLL_INTEGRATION.md`, `SYNCHRONIZATION.md`, `DOCS_SITE.md`
10. `TESTING_POLICY.md`, `ENVIRONMENT_SETUP.md`, `CONTEXT_MAP.md`
11. `agent/PLAN_STATUS.md`, `agent/ATTEMPT_LOG.jsonl`, `agent/MODEL_LOG.jsonl`
12. Release/history context when needed: `plans/mvp/MVP_RELEASE_REPORT.md`, `RELEASE_CHECKLIST.md`, `SECURITY_REVIEW.md`, `PACKAGING.md`

## Current state

The completed v0.1 MVP supports:

- `notriosd` local HTTP service;
- SQLite database creation, migration bootstrap, and default managed collection bootstrap;
- document create/read/update/patch/soft-delete;
- revision list/read/restore;
- SQLite FTS5 search over current non-deleted notes;
- content-addressed resource upload, metadata, content streaming, attachment, detachment, and safe delete;
- Markdown link/backlink parsing and graph slices;
- React/Vite UI using `md-editor-rt` with document/resource link routing;
- dependency-free MCP endpoint at `/mcp` (read-only default; editor writes);
- `notriosctl import joplin-raw` and `notriosctl import obsidian`;
- generated-dataset smoke/performance tests;
- release packaging and ZIP verification scripts.

The completed v0.2 redesign added notebooks/tags/search notebooks, source
provenance and threads, query language, optional Recoll, five importers, native
archive v1 interchange, Wails v2 GUI, and docs site. v0.3 added the
remote-media policy/scan/quarantine/localization surfaces, exact duplicate and
unreferenced resource reports, per-notebook resource usage, and an optional
review-only perceptual hook that is inert by default. H6 added schema-v8
resource retention state, configurable local retention, dry-run-first CLI
garbage collection, a read-only REST report, explicit confirmation for
permanent REST deletion, and a future sync-aware retention gate. Archive v1 is
not a full backup; P2 defines and verifies the v0.4 native archive-v2
full-snapshot/container layer reused by v0.7 sync, while P3/P4 export/restore
remain unimplemented. H7 added schema-v9 keyset
indexes and query-bound cursors for chronological/relevance, route-bound
notebook/Trash paging, live GET search, and bounded immutable snapshots for
optional FTS5/Recoll merging. Reproducible 10k/100k/500k evidence lives under
`performance/v0.3-h7/`. H8 added schema-v10 importer checkpoints and
fingerprints, exact optional source bundles, Joplin nested notebooks and stable
real tags, bounded batch lookups, dry-run/config parity, stable resource
refresh, interruption/resume, and generated 100/10k/100k evidence under
`performance/v0.3-h8/`. H9 applies the same generic state model to Obsidian:
nested vault notebooks and collision renames, exact Markdown/frontmatter and
non-Markdown source capture, alias/relative/embed/heading/block
canonicalization, stable resource refresh, resume/dry-run parity, and generated
100/10k/100k/500k evidence under `performance/v0.3-h9/`. H10 added schema-v11
durable projection retry/backoff, bounded drains, exact missing/stale/orphan
reconciliation, hardened cancellable Recoll processes/output, stable
deduplicated per-hit engine attribution, status/UI observability, and real
100k native Recoll evidence under `performance/v0.3-h10/`. H11 added the
cross-cutting maintenance guide (also reseeded into Help), reconciled living
specs and feature status, bumped product metadata to v0.3.0, completed the
release-candidate gates, and drafted the v0.4 plan. The 2026-08-02 follow-up
selected an external `movenotes-v3` archive bridge instead of duplicating
Obsidian/Quartz/Hugo publishing, implemented bounded boolean/category search,
and completed the J1–J3 Joplin prerequisite slices plus Q1. See
`agent/PLAN_STATUS.md`.

## Validation commands

Run these before committing any task:

```bash
go vet ./... && go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build && npm test
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
bash scripts/run_joplin_import_profile.sh 100 /tmp/notrios-joplin.json
bash scripts/run_real_joplin_profile.sh <label> <raw-export-dir> /tmp/notrios-joplin-real.json
bash scripts/run_obsidian_import_profile.sh 100 /tmp/notrios-obsidian.json
bash scripts/run_recoll_hardening_profile.sh 100 /tmp/notrios-recoll.json
```

GUI-affecting tasks also build with `make gui`; layout changes additionally run `scripts/verify_layout_resize.py` under Xvfb/Openbox (see `TESTING_POLICY.md`).

## Git workflow and state (reviewed 2026-07-26)

`main` takes reviewed merges; active work happens on `develop`. Commit each completed working-state slice; pushing to GitHub is the **user's step**.

Current handoff facts (verify again before acting):

- `origin` is configured (`https://github.com/renesugar/notrios.git`).
- `develop` review base was `26b0925`; it has no configured upstream.
- local `main` was `265ef4e` tracking `origin/main`.
- The v0.3 commits are local only. The user explicitly prohibited a GitHub
  push for this session.

Commit-message convention: each agent ends commit messages with its own `Co-Authored-By:` trailer, and appends its model to `agent/MODEL_LOG.jsonl` at session start (see `AGENTS.md`).

## Important constraints to preserve

- SQLite is the canonical managed-note database.
- Recoll is a derived, optional, external search sidecar — never canonical storage, never linked/vendored (GPL; see `RECOLL_INTEGRATION.md`).
- Project code must remain compatible with an MIT or Apache-2.0 license.
- MCP must not expose raw SQL or arbitrary filesystem operations.
- Imported Markdown and downloaded resources are untrusted; preview HTML must be sanitized.
- Remote media localization must go through media policy and quarantine checks.
- Exact SHA-256 is the only deduplication identity. Perceptual hashes are
  review suggestions only and no algorithm ships by default.
- Resource garbage collection must remain dry-run first, transactionally
  recheck references on apply, and pass future synchronization acknowledgement
  policy through `store.RetentionGate`.
- Every task must leave the repo in a working state; archive completed plans under `plans/`.
- Unbounded paging uses keysets/snapshots, not hidden offsets.
- Sync must follow `SYNCHRONIZATION.md`: canonical local stores, immutable
  operations/objects, REST/folder/rclone transports, and acknowledgement-gated
  retention. `rclone sync` is not the merge algorithm.

## Local environment notes

Facts about the development machine that no other document records:

- `./data/` in the repo root holds a throwaway development database (`notes.sqlite` plus `assets/`, `quarantine/`, `projections/`, `search-index/`) created by ad-hoc service and GUI smoke runs. It is gitignored and safe to delete; the service recreates it on startup.
- `web/dist/` is gitignored; run `cd web && npm ci && npm run build` after a fresh clone (CI and `scripts/package_release.sh` build it too).
- `scripts/verify_layout_resize.py` needs Python `playwright` plus `xdotool`, `Xvfb`, and `openbox` (all installed system-wide here, but **no Python venv is committed** — create one with `python3 -m venv … && pip install playwright`; it can drive the system `google-chrome`, so no browser download is required).
- `npm test` under Node 22 prints a harmless `ExperimentalWarning: localStorage` — the real polyfill lives in `web/src/test/setup.ts` (explained in its comments).
- MCP write-tool tests enable the editor profile by setting `s.config.MCP.DefaultProfile = "editor"` directly on the server struct — a test-only shortcut (see `internal/httpapi/remote_media_test.go`).
- The GUI before/after screenshots from the v0.2 conformance pass live in `/home/renes/prompts/` (outside the repo, intentionally uncommitted — transient browser-automation output is never committed).
- All verification servers, Xvfb displays, and browser sessions from prior agent sessions are stopped; session scratchpads lived under `/tmp` and are disposable.

## Environment limitations inherited from the scaffold

The scaffold was created in a restricted container. Still-open consequences:

1. The SQLite store uses a small local cgo adapter over system `libsqlite3`; the long-term driver choice is open.
2. The MCP adapter is dependency-free; the official MCP Go SDK can replace it later without changing tool semantics.
3. J1 matches canonical Joplin first-line titles and CR/LF-only metadata
   parsing, including OCR controls. J2 validates both supplied real exports and
   makes relationship planning proportional to parsed links. J3 adds bounded
   atomic canonical/checkpoint batches, an indexed temporary manifest, final
   links, and private-safe complete recipe-corpus evidence under
   `performance/v0.4-j3/`. Never commit private datasets or content-bearing
   evidence.
4. Q1 uses one bounded AST for uppercase `OR`, implicit `AND`, prefix
   negation, grouping, phrases, fields, and `category:`/`notebook:` aliases.
   SQLite is exact for every expression; supported shapes compile to Recoll
   with live parity tests, and unsupported sidecar shapes fall back explicitly
   to canonical SQLite rather than being approximated. Generated evidence is
   under `performance/v0.4-q1/`.
5. P1 provides one read-only Store/REST/MCP planner for recursive
   notebook/tag/query/explicit-ID selection. Target policies classify reachable
   resources and internal/private/broken links, hash source-bundle keys, strip
   paths/private metadata from API output, cap visible details, and bind the
   complete manifest to SHA-256. Generated 100k evidence is under
   `performance/v0.4-p1/`.
6. P2 adds schema-v12 stable logical database and per-writable-copy replica
   identities plus a separate read-only archive-v2 verifier. The manifest-last
   format uses strict typed JSONL records and immutable SHA-256 body/resource/
   source-bundle objects; schema/capability/MIME/size/count/path/depth and
   cross-reference checks complete before future restore writes. Synthetic
   golden/adversarial fixtures live under `internal/archivev2/testdata/`.

(The formerly open "no browser testing" limitation is resolved: the GUI is browser-verified via Playwright, vitest/RTL covers the workspace, and `scripts/verify_layout_resize.py` covers native window resizing.)
