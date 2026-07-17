# MVP Task 10 Report — Release Hardening

## Completed

- Added generated-dataset store smoke test and benchmark.
- Added `scripts/mvp_smoke.sh` for end-to-end REST/MCP/resource smoke testing.
- Added `scripts/run_performance_smoke.sh` for generated-dataset smoke and benchmark runs.
- Added `scripts/package_release.sh` for repeatable source ZIP packaging.
- Added `scripts/check_release_zip.py` to catch missing `web/dist` and accidental `node_modules`/runtime-data inclusion.
- Added `PACKAGING.md`, `SECURITY_REVIEW.md`, `RELEASE_CHECKLIST.md`, and `MVP_RELEASE_REPORT.md`.
- Hardened resource-content download headers with `X-Content-Type-Options: nosniff` and safer `Content-Disposition` filename formatting.
- Added HTTP test coverage for resource download headers.
- Archived the final MVP task under `plans/v0.1/015-mvp-release-hardening.md`.
- Added a draft next plan under `plans/v0.2/001-import-resource-media-hardening.md` and promoted it to `PLAN.md` as not-started pending user approval.

## Validation

The release-hardening pass should be considered complete only when these pass:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
bash scripts/package_release.sh /tmp/notes-companion-v0.1.0-mvp-check.zip
python3 scripts/check_release_zip.py /tmp/notes-companion-v0.1.0-mvp-check.zip
```

## Notes for Codex

The official MCP SDK, a maintained SQLite driver, Playwright browser automation, sist2, and Quartz were not installed in this container. In a normal development environment, evaluate replacing the MVP adapters/stubs with those production dependencies before expanding the feature set.
