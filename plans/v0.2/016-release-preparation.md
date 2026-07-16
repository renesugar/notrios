# v0.2 Task R16 — GitHub release preparation

Completed: 2026-07-15. Model: Claude Fable 5 (claude-fable-5). This closes the v0.2 Notrios redesign plan; the actual push to GitHub is the repository owner's step.

## Changes

- **License audit.** Go dependencies (audited with go-licenses under the full GUI build tags, so the Wails tree is included): MIT, BSD-2-Clause, BSD-3-Clause, and Apache-2.0 only — all compatible with the project's Apache-2.0 license. npm production dependencies (license-checker): MIT/ISC/Apache/BSD/Python-2.0 plus MPL-2.0 from `lightningcss`, an unmodified Vite build-time tool whose file-level copyleft does not affect distribution of this repository. `web/package.json` now declares `"license": "Apache-2.0"` (it previously reported as UNLICENSED). The GPL boundary for Recoll/Xapian (external processes only) is documented in `RECOLL_INTEGRATION.md`.
- **CI.** `.github/workflows/ci.yml` expanded from a single go-test job to: `go` (vet, tests, required-files, scaffold validation, with libsqlite3-dev installed), `web` (npm ci, typecheck, build with lockfile caching), `smoke` (built UI + `mvp_smoke.sh` + `run_performance_smoke.sh`), and `gui-build` (compiles `cmd/notrios` with the `gui desktop production webkit2_41` tags against libgtk-3-dev/libwebkit2gtk-4.1-dev). Go version pinned to 1.25 matching go.mod.
- **RELEASE_CHECKLIST.md** rewritten: the audited/ready state, the exact first-push sequence for `github.com/renesugar/notrios`, post-push steps (branch protection, enabling GitHub Pages for the docs workflow), and the validation set for tagging.
- **README final pass**: documentation section added, project status updated to "v0.2 complete", stale draft-plan references removed.
- `PLAN.md` marked complete; per `AGENTS.md`, the next plan should be drafted from `ROADMAP.md` (v0.3) with user approval.

## Validation

`go test ./...`, scaffold checks, `mvp_smoke.sh`, web build, and both workflow files YAML-validated. All passing.
