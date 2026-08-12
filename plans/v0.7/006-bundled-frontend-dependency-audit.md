# v0.7 maintenance — bundled frontend dependency audit

Status: complete on 2026-08-12.

Model: GPT-5 (exact variant not exposed).

## Goal and scope

Run the user-requested regular npm advisory/fix pass before testing the bundled
offline frontend. Apply only compatible lockfile updates with `npm audit fix`;
do not use `--force`, change direct dependency ranges, fetch runtime assets from
a CDN, or begin v0.7 G4.

## Result

The baseline audit reported three transitive packages: high-severity `nanoid`
3.3.16, moderate-severity `postcss` 8.5.19, and high-severity `undici` 7.28.0.
The non-forced fix updated them to 3.3.18, 8.5.26, and 7.29.0 respectively.
It also reconciled the lockfile root package version with the existing
`web/package.json` version 0.6.0. No direct dependency range changed.

A clean reinstall reports zero vulnerabilities. The regular local workflow is
now audit, compatible fix, lockfile review, clean reinstall, re-audit, then
typecheck/tests/build. CI and release packaging use the committed lockfile and
fail on a non-clean audit without mutating it. The application remains fully
bundled for offline use.

## Validation evidence

- baseline `cd web && npm audit --json` (1 moderate, 2 high)
- `cd web && npm audit fix` (3 packages changed, 0 vulnerabilities)
- `cd web && npm ci` (0 vulnerabilities)
- `cd web && npm audit --json` (0 vulnerabilities)
- `cd web && npm run typecheck`
- `cd web && npm test -- --run` (15 files, 155 tests)
- `cd web && npm run build`
- `bash scripts/run_offline_assets_check.sh` (no third-party requests, remote
  scripts/styles, or CSP violations; KaTeX rendered locally)

Repository-wide checks, release packaging, and final ZIP verification are
recorded in the completion log and handoff report. G4 remains unapproved.
