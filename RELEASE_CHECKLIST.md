# Release Checklist

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
