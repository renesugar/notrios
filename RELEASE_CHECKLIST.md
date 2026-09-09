# Release Checklist

**How to keep this document current is in [`AGENTS.md`](AGENTS.md)** — under "Keeping the reference documents current".

## v0.8.0 — installation, configuration, shared core, and portability

H0-H29 are implemented on `develop`. v0.8 makes **no external write at all**:
the first GitHub push, the `develop`-to-`main` pull request and the branch
synchronization open v0.9, because the condition that would have brought them
forward -- native runners for Windows and macOS -- never fired once H7 moved to
post-v1.0. So the owner steps below are shorter than every milestone before it,
and what stands in their place is a source snapshot on this machine.

Release-candidate gates:

- [x] Product version reports `0.8.0`; the canonical schema stays at **v27**,
      because no v0.8 item added a migration and incrementing one for packaging
      would claim a data change that did not happen.
- [x] `go vet ./...`, `go test ./...`, and `make validate`, which now also runs
      the H10 Wails v3 spike record and the H11 Android acceptance record
      offline.
- [x] The C ABI's twelve exported symbols, no SQLite export, no dynamic SQLite,
      and the C host acceptance test -- on the desktop build and again on the
      Android cross-build.
- [x] Installed paths on the XDG contract, the `install`/`uninstall`/`purge`
      lifecycle with verified backup, and an Ubuntu package installable without
      Go, Node, Wails or a compiler.
- [x] The native credential store selected per platform, with no silent fall
      back to plaintext and no secret bytes in a purge backup or in evidence.
- [x] Documentation regenerated from source: the CLI reference, the features
      page, both journey catalogues, the atlas and the plan's own progress log.
- [x] The GUI control inventory re-crawled at 89 controls, 82 of them
      addressable, and every capability the interface offers has a captured
      journey.

Not claimed by v0.8, and refused by a gate where one exists:

- Windows and macOS installers, deferred to post-v1.0 with the hardware they
  need. No row is marked passed for a platform this machine cannot execute.
- Any Android product. H11 accepted the shared core on one x86_64 emulator;
  arm64 is built and never run, and the record is refused if it stops saying so.
- A Wails v3 migration. H10 recommends one after v3 releases; production still
  builds and runs on v2.13.0.
- A tag, a GitHub Release, a public installer, or a signing or notarisation
  claim.

Repository-owner steps (deferred to v0.9 rather than to the end of v0.8):

- [ ] Push `develop`, open the `develop`-to-`main` pull request, and merge with
      a merge commit after review.
- [ ] Bring the merge back into `develop`, then require `main` to be an ancestor
      of it with a zero content diff.
- [ ] Perform any separately authorised evidence-reserve/ISO/media work.

## v0.7.0 — native synchronization

G0-G20 are implemented on `develop`. G20 produces a local source release
candidate; repository-owner publishing remains a separate authorization.

Release-candidate gates:

- [x] Product version reports `0.7.0`; fresh and upgraded databases converge on
      schema v27 after migrations v19-v27.
- [x] `go vet ./...`, `go test ./...`, focused three-peer convergence,
      directory/REST process, catch-up/reset/retention, lazy-resource,
      conflict/fault, abuse, and upgrade suites.
- [x] Required-file, scaffold, OpenAPI/route/MCP parity, documentation-anchor,
      executable-example, GUI-journey, Hugo/Ledger, offline, and Help-seed
      checks.
- [x] `npm audit` before and after `npm ci`, frontend typecheck/tests, and
      production build.
- [x] Frozen aggregate-only G8/G14e/G17 evidence revalidated without rereading
      the private corpus: 382,206 equivalent notes, attachment-bearing lazy
      resources, physical snapshot/catch-up, and retention cost.
- [x] Exact dependency inventories pass offline for every Go module and npm
      lockfile package; all licenses remain compatible with repository policy.
- [x] The standard security scan has complete coverage, every validated finding
      is dispositioned and remediated, the one static bypass-correction cycle is
      closed, and the derived structural-hardening portfolio validates.
- [x] Upgrade/rollback instructions and disaster-recovery boundaries are
      reconciled with schema v27 and the physical/semantic backup contracts.
- [x] `scripts/package_release.sh` and `scripts/check_release_zip.py` produce a
      versioned source ZIP containing `web/dist/`; its byte count, entries, and
      SHA-256 are recorded and the exact ZIP is copied to the evidence directory.

What v0.7 adds beyond v0.6:

- protocol-1.0 record synchronization with durable state vectors, HLC merge,
  revision conflicts, lazy resources, signed/encrypted artifacts, catch-up,
  retention, repair, and retirement across directory and REST carriers;
- authenticated per-peer REST sync and a loopback-only Sync Center, without
  turning peer credentials into user login or exposing ordinary REST/MCP/UI;
- compatible physical whole-library snapshot/catch-up plus portable semantic
  archive-v2, with a published strict consumer contract;
- a frozen shared-core/C-ABI portability handoff and reproducible documentation
  anchors, executable examples, GUI journeys, and Hugo/Ledger site;
- evidence preservation, release acceptance, and G20 hardening of listener,
  browser/body, carrier-root, and legacy-archive boundaries.

Repository-owner publishing steps (not performed by G20):

- [ ] Review and merge/fast-forward `develop` into `main`.
- [ ] Push reviewed branches and confirm CI/Pages.
- [ ] Tag and push `v0.7.0` only after accepting the release candidate.
- [ ] Perform any separately authorized evidence-reserve/ISO/media work.

## v0.6.0 — MCP and automation expansion

The v0.6 implementation is complete on `develop`. As in earlier milestones, the
reproducible local release-candidate gates are separated from repository-owner
publishing actions.

Release-candidate gates (run for F7):

- [x] Product version reports `0.6.0`; schema bootstraps/upgrades to v18.
- [x] `go vet ./...` and `go test ./...`.
- [x] Required-file, scaffold, OpenAPI parse, migration-copy, and plan-loop
      checks.
- [x] Web lockfile install, typecheck, tests, production build, and GUI build.
- [x] Documentation site build and deterministic Help-notebook seed/reseed.
- [x] End-to-end REST/MCP smoke and generated store performance smoke.
- [x] Offline-assets check: no third-party requests, no CSP violations, math
      rendered with the network unavailable.
- [x] Every registered MCP tool is named in `docs/api/mcp.md`, and every REST
      route in `server.go` appears in `api/openapi.yaml` — both checked
      mechanically in F7 rather than by reading.
- [x] Every v0.6 `ROADMAP.md` bullet reconciled against the code. Ten shipped;
      one shipped **half** and the other half was refused on the evidence, which
      is recorded in the bullet rather than quietly marked done.
- [x] Source release ZIP produced by `scripts/package_release.sh`, independently
      checked by `scripts/check_release_zip.py`, and SHA-256 recorded in the
      F7 handoff.

What v0.6 adds beyond v0.5:

- notebook targeting: the sidebar selection is the creation target, the editor
  toolbar re-files the open note, and `notriosctl notes move` closes the CLI gap
  (F0);
- batch organizer transactions over an explicit note list, bounded at 500,
  atomic or best-effort, with per-item outcomes and a schema-v17 idempotency
  ledger (F1);
- four cumulative MCP tool scopes — search-only, read-only, editor, organizer —
  enforced at the call site rather than only in the tool listing (F2);
- MCP read coverage decided surface by surface, bounded resource reads, and HTTP
  `Range` in REST (F3);
- note templates with a closed placeholder vocabulary and one-pass substitution,
  and tasks computed on read with block-model identity (F4);
- graph views that stay readable at scale: a local graph, a hubs report written
  as a read-only note in a new builtin Reports notebook, and CSV node/edge
  export — plus `store.IsReadOnlyNotebook` and the fix for trashed notes
  inflating the graph report (F5);
- a job control plane for long imports and exports, with cooperative
  cancellation, a derived `interrupted` state, and a documented exit-code
  contract instead of a scheduler (F6).

Three defects found by F7's reconciliation and fixed in it:

- `POST /api/v1/batch` applied `trash`, `add_tags`, and `remove_tags` to notes in
  a read-only notebook, while the single-note routes answered 403 for the same
  notes. `move` and `duplicate` were guarded and the rest were not, which is
  what a per-operation check produces; the guard now runs once before the
  dispatch.
- `POST/DELETE /api/v1/documents/{id}/tags/{tag}` had no read-only guard at all,
  and a tag outlives a reseed because `note_tags` is keyed by a stable document
  ID.
- Six REST surfaces had neither an MCP tool nor a recorded reason, despite F3
  claiming every surface was decided. Each is now decided; `tag_note` and
  `untag_note` were added at `editor`, because tagging one note previously
  required `organizer`.

Repository-owner publishing steps (not performed by F7):

- [ ] Review and merge/fast-forward `develop` into `main`.
- [ ] Push the reviewed branches; confirm CI and the GitHub Pages docs workflow.
- [ ] Tag and push `v0.6.0` only after the owner accepts the release candidate.
- [ ] The v0.5.0 tag is still outstanding; tag it from the same merge if it has
      not been published yet.

## v0.5.0 — better editing, blocks, and graph UX

The v0.5 implementation is complete on `develop`. As in earlier milestones, the
reproducible local release-candidate gates are separated from repository-owner
publishing actions.

Release-candidate gates (run for E9):

- [x] Product version reports `0.5.0`; schema bootstraps/upgrades to v16.
- [x] `go vet ./...` and `go test ./...`.
- [x] Required-file, scaffold, OpenAPI parse, migration-copy, and plan-loop
      checks.
- [x] Web lockfile install, typecheck, tests, production build, and GUI build.
- [x] Documentation site build and deterministic Help-notebook seed/reseed.
- [x] End-to-end REST/MCP smoke and generated store performance smoke.
- [x] Offline-assets check: no third-party requests, no CSP violations, math
      rendered with the network unavailable.
- [x] Committed evidence for every slice that added a measurable surface, under
      `performance/v0.5-e1/`, `-e1a/`, `-e2/`, `-e4/`, `-e5/`, `-e6/`, `-e6a/`.
      Slices that added no new unbounded surface (E1b, E3, E6b, E7, E8) carry a
      written reason in their archived plan instead.
- [x] Source release ZIP produced by `scripts/package_release.sh`, independently
      checked by `scripts/check_release_zip.py`, and SHA-256 recorded in the
      E9 handoff.
- [x] ZIP contains `web/dist/` and excludes `.git/`, `web/node_modules/`,
      runtime `data/` **at any depth**, SQLite databases, and generated build
      directories. E9 found that both the packaging exclusion and the checker
      were anchored at the archive root, so test-created directories such as
      `internal/service/data/` and `cmd/notriosctl/data/` had been shipping in
      every ZIP; both now match at any depth, and the strengthened checker was
      confirmed to fail on the previously-built archive.

What v0.5 adds beyond v0.4:

- addressable blocks and heading anchors — schema v14/v15, content-derived block
  identity, `GET /api/v1/documents/{id}/blocks`, and anchor resolution for
  `document://` and `notrios://` links;
- a read-only workspace lint over eleven checks, and a dry-run-first `fix` for
  the mechanically repairable subset;
- bounded graph traversal, shortest paths from both ends, and an orphan/isolate/
  hub report;
- editor link intelligence — inline note search on `[[`, broken-link underlines
  that follow their text, Ctrl-click to follow a link, and a whole-note list;
- an offline-first frontend: KaTeX, highlight.js, and cropper bundled, echarts
  and prettier off, and a Content-Security-Policy served with the UI;
- HTML table paste normalization that refuses far more than it converts;
- embedded `note-query` blocks parsed server-side by the same query parser as
  every other search surface, and inert in a publication;
- trash-first delete/restore in the GUI, a notebook-deletion preview, and
  hierarchical tag rename whose dry run is a rolled-back apply.

Known release boundaries:

- **Templates, task extraction, and a graph *view* did not ship.** All three
  were v0.5 roadmap bullets; the first two never entered `PLAN.md` and the third
  was scoped to data only (E4 is named "traversal, paths, and visualization
  data"). All three moved to v0.6 rather than being left ambiguous.
- Bulk and batch organizer operations remain v0.6. v0.5's organizer is
  deliberately single-object; a hierarchical tag rename touches many rows
  because a hierarchy is one thing, not because it is a batch.
- Tag rename has no GUI surface — it is Store/REST/CLI only.
- MCP does not expose lint, fix, graph, blocks, query blocks, tag rename,
  notebook deletion, GC, archive operations, or publication. That is a standing
  decision, recorded in `docs/api/mcp.md`, not a gap.
- The Wails webview is not measured by the editor and offline-asset harnesses:
  WebKitGTK does not speak the DevTools Protocol they use. It loads the
  identical bundle through the same handler, so the code path is the one
  measured; the engine is not.
- Every v0.4 boundary still applies: source-only, Ubuntu-only, single-user,
  local, and unauthenticated; a publication handoff is a projection, not a
  backup; `--pack` archives need a reader that understands `objects.pack.v1`;
  stable-link handler registration is Ubuntu/XDG only; Recoll/Xapian remain
  optional user-installed external GPL processes.

Post-candidate fixes, complete:

- [x] **E12 — a read-only title you can still read.** The note title used
      `disabled` rather than `readOnly` on Help and trashed notes. Both refuse
      edits; only `disabled` removes the control from the tab order, so a title
      longer than the box could not be focused, scrolled with the keyboard,
      selected, or copied. Archived as
      `plans/v0.5/016-readonly-title-control.md`.

- [x] **E11 — finding the GUI's own web files.** `web/dist` was resolved
      relative to the process working directory with no flag and no config
      option, so `bin/notrios` run from `bin/` rendered `web_ui_not_built`
      inside the window — telling the reader to rebuild assets that were
      already built. It now searches `--web-dir`, `server.web_dir`, the working
      directory, and the executable's own directory and its parent; the GUI
      refuses to start when it finds nothing and names every directory tried.
      Also: a border and caret on the notebook control (and its position moved
      to lead the toolbar, since it was behind horizontal scroll), all
      seventeen `make` targets documented, and `web/README.md` rewritten.
      Archived as `plans/v0.5/015-finding-the-web-interface.md`.

- [x] **E10 — the editor toolbar: layout, and which actions belong in it.** Two
      defects from the same session of Trash testing. (1) The toolbar was one
      wrapping row, so the "In the Trash" chip never left the title's row and
      the buttons wrapped raggedly from 520 px down to the editor pane's 280 px
      minimum; it is now a title row plus an action row that stacks as a unit,
      verified by measurement across the same widths. (2) "New note" showed for
      every open note including read-only Help and Trash notes; it moved to the
      search pane, where it also names the notebook a draft will be filed in.
      No version change. Archived as
      `plans/v0.5/014-editor-toolbar-and-notebook-targeting.md`.

Also landed alongside it (v0.6 F0, not a v0.5 gate): notebook targeting — the
sidebar selection is the creation target, a notebook control in the editor
toolbar re-files the open note, and `notriosctl notes move` closes the CLI gap.
Before it the GUI created every note in "Notes" and could not move one.

Repository-owner publishing steps (not performed by E9):

- [ ] Review and merge/fast-forward `develop` into `main`.
- [ ] Push the reviewed branches; confirm CI and the GitHub Pages docs workflow.
- [ ] Tag and push `v0.5.0` only after the owner accepts the release candidate
      and E10 has landed.

## v0.4.0 — portable data, publishing, and stable references

The v0.4 implementation is complete on `develop`. As with v0.3, the
reproducible local release-candidate gates are separated from repository-owner
publishing actions.

Release-candidate gates (run for P8):

- [x] Product version reports `0.4.0`; schema bootstraps/upgrades to v13.
- [x] `go vet ./...` and `go test ./...`.
- [x] Required-file, scaffold, OpenAPI parse, migration-copy, and plan-loop
      checks.
- [x] Web lockfile install, typecheck, tests, production build, and GUI build.
- [x] Documentation site build and deterministic Help-notebook seed/reseed
      covering the two new pages (stable links, publishing).
- [x] End-to-end REST/MCP smoke and generated store performance smoke.
- [x] Committed evidence for every implemented slice under
      `performance/v0.4-j2/` through `performance/v0.4-p7/`.
- [x] Source release ZIP produced by `scripts/package_release.sh`, independently
      checked by `scripts/check_release_zip.py`, and SHA-256 recorded in the
      P8 handoff.
- [x] ZIP contains `web/dist/` and excludes `.git/`, `web/node_modules/`,
      runtime `data/`, SQLite databases, and generated build directories.

What v0.4 adds beyond v0.3:

- native archive v2 as a real backup format — streaming export at
  382,206-note scale under a loose or optional packed object layout,
  read-only verification, and restore under mandatory
  replace/adopt/merge/fork intent;
- Joplin RAW correctness and million-note transactional import throughput;
- bounded boolean search with the `category:` alias;
- the shared read-only selection/privacy planner;
- stable external `notrios://` links with an explicit local profile registry
  and an Ubuntu protocol handler;
- reviewed publication profiles emitting a sanitized subset handoff.

Known release boundaries:

- A publication handoff is a projection, not a backup: current notes only, with
  links to withheld or unresolved targets rewritten. Notrios does not build a
  site, and no external tool is yet verified to consume the handoff — the
  `movenotes-v3` compatibility bridge is deferred to v0.7 (see
  `PROJECT_DECISIONS.md` decision 16).
- `--pack` archives require a reader that understands `objects.pack.v1`; loose
  remains the default.
- An interrupted packed export restarts rather than resuming.
- Stable-link OS handler registration is Ubuntu/XDG only.
- Notrios remains source-only, Ubuntu-only, single-user/local, and
  unauthenticated. Do not expose the service directly to an untrusted network.
- Recoll/Xapian remain optional user-installed external GPL processes.
- Native archive v1 is query-scoped interchange, not a full backup; archive v2
  is the backup format.

Repository-owner publishing steps (not performed by P8):

- [ ] Review and merge/fast-forward `develop` into `main`.
- [ ] Push the reviewed branches; confirm CI and the GitHub Pages docs workflow.
- [ ] Tag and push `v0.4.0` only after the owner accepts the release candidate.

## v0.3.0 — import, resource, media, and scale hardening

The v0.3 implementation is complete on `develop`. This section separates the
reproducible local release-candidate gates from repository-owner publishing
actions.

Release-candidate gates (run for H11):

- [x] Product version reports `0.3.0`; schema bootstraps/upgrades to v11.
- [x] `go vet ./...` and `go test ./...`.
- [x] Required-file, scaffold, OpenAPI parse, migration-copy, and plan-loop
      checks.
- [x] Web lockfile install, typecheck, tests, production build, and GUI build.
- [x] Documentation site build and deterministic Help-notebook seed/reseed.
- [x] End-to-end REST/MCP smoke and generated store performance smoke.
- [x] 100-note Joplin RAW, Obsidian, and native Recoll profiles; committed
      scale evidence remains under `performance/v0.3-h7/` through
      `performance/v0.3-h10/`.
- [x] Source release ZIP produced by `scripts/package_release.sh`, independently
      checked by `scripts/check_release_zip.py`, and SHA-256 recorded in the
      H11 handoff.
- [x] ZIP contains `web/dist/` and excludes `.git/`, `web/node_modules/`,
      runtime `data/`, SQLite databases, and generated build directories.

Known release boundaries:

- Recoll/Xapian remain optional user-installed external GPL processes and are
  not linked, vendored, or redistributed.
- Joplin/Obsidian correctness and scale evidence uses generated fixtures.
  Before migrating irreplaceable notes, run a dry run and validate against a
  private backup; never add private exports to this repository.
- Notrios remains source-only, Ubuntu-only, single-user/local, and unauthenticated.
  Do not expose the service directly to an untrusted network.
- Native archive v1 is query-scoped interchange, not a full backup. Back up
  the stopped SQLite database and asset store as documented.

Repository-owner publishing steps (not performed by H11):

- [ ] Review and merge/fast-forward `develop` into `main`.
- [ ] Push the reviewed branches; confirm CI and the GitHub Pages docs workflow.
- [ ] Protect `main` and require the desired review/check policy.
- [ ] Tag and push `v0.3.0` only after the owner accepts the release candidate.

## v0.2 — first public GitHub push (task R16)

The v0.2 Notrios redesign is complete; the repository is ready for its first
push to `https://github.com/renesugar/notrios`.

Already done (task R16):

- [x] License: Apache-2.0 (`LICENSE`); `web/package.json` declares it too.
- [x] Go dependency licenses audited with go-licenses under the GUI build
      tags: MIT / BSD-2 / BSD-3 / Apache-2.0 only — all Apache-2.0 compatible.
- [x] npm production licenses audited with license-checker: MIT / ISC /
      Apache-2.0 / BSD / Python-2.0, plus MPL-2.0 from `lightningcss` (an
      unmodified Vite build-time tool; MPL's file-level copyleft does not
      affect Apache-2.0 distribution of this repository).
- [x] GPL boundary: Recoll/Xapian are user-installed external processes only;
      nothing GPL is linked, vendored, or redistributed
      (`RECOLL_INTEGRATION.md`).
- [x] CI (`.github/workflows/ci.yml`): go vet + tests + scaffold checks (with
      libsqlite3-dev), web typecheck/build, end-to-end smoke + performance
      smoke, and a GUI compile check with the Wails/WebKit build tags.
- [x] Docs site workflow (`.github/workflows/docs.yml`): GitHub Pages with
      PageFind search. Enable Pages (Source: GitHub Actions) after the push.

Push sequence (run by the repository owner):

```bash
# from the repo root; main currently holds the pre-redesign baseline
git checkout main
git merge --ff-only develop   # or merge/rebase per your preference
git remote add origin https://github.com/renesugar/notrios.git
git branch -M main
git push -u origin main
git push origin develop
```

After the push:

- [ ] Protect `main` (require CI, review before merge).
- [ ] Enable GitHub Pages: Settings → Pages → Source: GitHub Actions.
- [ ] Confirm the `docs` workflow deployed and PageFind search works.
- [ ] Optionally tag: `git tag -a v0.2.0 -m "Notrios v0.2 redesign" && git push origin v0.2.0`.

Local validation before tagging any release:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
bash scripts/package_release.sh /tmp/notrios-v0.2.0.zip
python3 scripts/check_release_zip.py /tmp/notrios-v0.2.0.zip
```

## v0.1.0 MVP (historical)

- [x] Completed and archived under `plans/v0.1/`; see `plans/mvp/MVP_RELEASE_REPORT.md`.
