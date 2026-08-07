# Release Checklist

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

Outstanding before the candidate is accepted:

- [ ] **E11 — the GUI cannot find its own web files outside the checkout root,
      and says the wrong thing when it cannot.** `web/dist` is resolved relative
      to the process working directory with no flag and no config option, so
      running `bin/notrios` from `bin/` renders `web_ui_not_built` inside the
      window — a message telling the reader to build assets that are already
      built. Also: the notebook dropdown has no border, `make serve`/`doctor`/
      `seed-help` are undocumented, and `web/README.md` still refers to
      `cmd/notesd`. See `PLAN.md`.

Post-candidate fixes, complete:

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
